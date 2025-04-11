package handlers

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/starfleetcptn/gomft/components"
	"github.com/starfleetcptn/gomft/internal/db"
	"github.com/starfleetcptn/gomft/internal/email"
	"github.com/starfleetcptn/gomft/internal/scheduler"
)

// WebSocketClients maintains the set of active WebSocket clients
var WebSocketClients = make(map[*websocket.Conn]bool)

// WebSocketClientsMutex protects the WebSocketClients map
var WebSocketClientsMutex = &sync.Mutex{}

// WebSocketClientWriteMutexes maintains individual write mutexes for each client
var WebSocketClientWriteMutexes = make(map[*websocket.Conn]*sync.Mutex)

// LogChannel is used to send log entries to all WebSocket clients
var LogChannel = make(chan components.LogEntry, 512)

// Handlers contains all the dependencies needed by the handlers
type Handlers struct {
	DB        *db.DB
	Scheduler scheduler.SchedulerInterface
	JWTSecret string
	StartTime time.Time
	DBPath    string
	BackupDir string
	LogsDir   string
	Email     *email.Service
}

// NewHandlers creates a new Handlers instance
func NewHandlers(database *db.DB, scheduler scheduler.SchedulerInterface, jwtSecret string, dbPath string, backupDir string, logsDir string, emailService *email.Service) *Handlers {
	h := &Handlers{
		DB:        database,
		Scheduler: scheduler,
		JWTSecret: jwtSecret,
		StartTime: time.Now(),
		DBPath:    dbPath,
		BackupDir: backupDir,
		LogsDir:   logsDir,
		Email:     emailService,
	}

	// Start the WebSocket log broadcaster
	StartLogBroadcaster()

	return h
}

// StartLogBroadcaster starts a goroutine that broadcasts logs to all connected WebSocket clients
func StartLogBroadcaster() {
	go func() {
		fmt.Fprintln(os.Stderr, "[DEBUG-BROADCASTER-V4] Broadcaster goroutine started.")
		for {
			logEntry := <-LogChannel // Wait for a log entry

			WebSocketClientsMutex.Lock()
			clientsToSend := make(map[*websocket.Conn]*sync.Mutex)
			for client, mutex := range WebSocketClientWriteMutexes {
				if _, exists := WebSocketClients[client]; exists {
					clientsToSend[client] = mutex
				}
			}
			WebSocketClientsMutex.Unlock()

			if len(clientsToSend) == 0 {
				continue // Skip if no clients
			}

			fmt.Fprintf(os.Stderr, "[DEBUG-BROADCASTER-V4] Received log. Broadcasting to %d clients. Level='%s', Src='%s'\n",
				len(clientsToSend), logEntry.Level, logEntry.Source)

			var wg sync.WaitGroup
			for client, mutex := range clientsToSend {
				wg.Add(1)
				go func(c *websocket.Conn, m *sync.Mutex, entry components.LogEntry) {
					defer wg.Done()

					fmt.Fprintf(os.Stderr, "[DEBUG-BROADCASTER-V4] Attempting send to client %v\n", c.RemoteAddr())

					// Lock only for this specific client's write
					m.Lock()
					// Set a deadline for the write operation
					deadline := time.Now().Add(5 * time.Second) // 5-second deadline
					err := c.SetWriteDeadline(deadline)
					if err != nil {
						fmt.Fprintf(os.Stderr, "[DEBUG-BROADCASTER-V4] Error setting write deadline for client %v: %v\n", c.RemoteAddr(), err)
						// Don't unlock yet, proceed to cleanup
					} else {
						err = c.WriteJSON(entry)
					}
					m.Unlock() // Unlock after write attempt (or deadline error)

					if err != nil {
						fmt.Fprintf(os.Stderr, "[DEBUG-BROADCASTER-V4] Error writing to client %v: %v. Initiating removal.\n", c.RemoteAddr(), err)
						WebSocketClientsMutex.Lock()
						if _, stillExists := WebSocketClients[c]; stillExists {
							delete(WebSocketClients, c)
							delete(WebSocketClientWriteMutexes, c)
							fmt.Fprintf(os.Stderr, "[DEBUG-BROADCASTER-V4] Removed client %v from maps.\n", c.RemoteAddr())
						} else {
							fmt.Fprintf(os.Stderr, "[DEBUG-BROADCASTER-V4] Client %v already removed by another process.\n", c.RemoteAddr())
						}
						WebSocketClientsMutex.Unlock()
						c.Close() // Close the connection outside the lock
					} else {
						fmt.Fprintf(os.Stderr, "[DEBUG-BROADCASTER-V4] Successfully sent to client %v\n", c.RemoteAddr())
					}
				}(client, mutex, logEntry)
			}
			wg.Wait() // Wait for all sends in this batch to complete or fail
		}
	}()
}

// BroadcastLog sends a log entry to all connected WebSocket clients
func (h *Handlers) BroadcastLog(level, message, source string) {
	// NOTE: Level prefix parsing is now handled in logger.go/parseLogEntry

	// Create log entry with UTC timestamp for consistency
	logEntry := components.LogEntry{
		Timestamp: time.Now().UTC(),
		Level:     level,
		Message:   message,
		Source:    source,
	}

	// Get the current number of clients (avoid logging in case of recursive issues)
	numClients := 0
	WebSocketClientsMutex.Lock()
	numClients = len(WebSocketClients)
	WebSocketClientsMutex.Unlock()

	// Only attempt to write to channel if there are clients
	if numClients > 0 {
		// *** DEBUG: Print channel send attempt ***
		fmt.Fprintf(os.Stderr, "[DEBUG-BROADCASTLOG-V4] Attempting to send to LogChannel: Level='%s', Source='%s'\n", level, source)

		// Try to send the log entry to the channel with a timeout
		select {
		case LogChannel <- logEntry:
			// Successfully sent
			fmt.Fprintf(os.Stderr, "[DEBUG-BROADCASTLOG-V4] Successfully sent to LogChannel.\n")
		case <-time.After(100 * time.Millisecond):
			// Channel is full or blocked, log and continue
			fmt.Fprintf(os.Stderr, "[DEBUG-BROADCASTLOG-V4] Log channel timeout, discarding log entry: %s\n", message)
		}
	} else {
		fmt.Fprintf(os.Stderr, "[DEBUG-BROADCASTLOG-V4] No clients connected, skipping send to LogChannel.\n")
	}
}
