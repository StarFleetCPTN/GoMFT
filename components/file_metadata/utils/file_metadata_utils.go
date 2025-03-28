package utils

import "fmt"

// GetStatusBadgeClass returns the appropriate CSS class for a file status badge
func GetStatusBadgeClass(status string) string {
	switch status {
	case "processed":
		return "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300"
	case "archived":
		return "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300"
	case "deleted":
		return "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-300"
	case "archived_and_deleted":
		return "bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-300"
	case "error":
		return "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-300"
	default:
		return "bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300"
	}
}

// FormatFileSize formats a file size in bytes to a human-readable string
func FormatFileSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.2f KB", float64(size)/1024)
	} else if size < 1024*1024*1024 {
		return fmt.Sprintf("%.2f MB", float64(size)/(1024*1024))
	} else {
		return fmt.Sprintf("%.2f GB", float64(size)/(1024*1024*1024))
	}
}
