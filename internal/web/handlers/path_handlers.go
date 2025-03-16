package handlers

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// HandleCheckPath validates if a given path exists and is accessible
func (h *Handlers) HandleCheckPath(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"valid": false,
			"error": "No path provided",
		})
		return
	}

	// Clean and resolve the path
	path = filepath.Clean(path)
	absPath, err := filepath.Abs(path)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"valid": false,
			"error": "Invalid path format",
		})
		return
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{
				"valid": false,
				"error": "Path does not exist",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"valid": false,
			"error": "Error accessing path: " + err.Error(),
		})
		return
	}

	// Check if it's a directory
	if !info.IsDir() {
		c.JSON(http.StatusOK, gin.H{
			"valid": false,
			"error": "Path exists but is not a directory",
		})
		return
	}

	// Check if we have read access
	testFile := filepath.Join(absPath, ".gomft_test")
	f, err := os.OpenFile(testFile, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"valid": false,
			"error": "Directory exists but is not writable",
		})
		return
	}
	f.Close()
	os.Remove(testFile)

	c.JSON(http.StatusOK, gin.H{
		"valid": true,
		"error": "",
	})
}
