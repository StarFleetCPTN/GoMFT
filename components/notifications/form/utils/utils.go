package utils

// Contains checks if a string is in a slice.
func Contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// BoolToString converts bool to string for HTML attributes.
// Returns "true" for true, "" for false (useful for checked attribute).
func BoolToString(b bool) string {
	if b {
		return "true"
	}
	return ""
}

// GetNotificationFormTitle returns the appropriate title for the form.
func GetNotificationFormTitle(isNew bool) string {
	if isNew {
		return "Add Notification Service"
	}
	return "Edit Notification Service"
}
