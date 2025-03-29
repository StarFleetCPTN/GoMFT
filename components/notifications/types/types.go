package types

// SettingsNotificationsData defines the data needed for the notifications list page
type SettingsNotificationsData struct {
	NotificationServices []NotificationServiceData
	SuccessMessage       string
	ErrorMessage         string
}

// NotificationServiceData defines the data for a single service in the list
type NotificationServiceData struct {
	ID              uint
	Name            string
	Type            string
	IsEnabled       bool
	Config          map[string]string // Keep for now, might refine later
	Description     string
	EventTriggers   []string
	PayloadTemplate string
	SecretKey       string
	RetryPolicy     string
	SuccessCount    int
	FailureCount    int
}

// NotificationFormData defines the data needed for the notification add/edit form
// TODO: This will be moved from notification_form.templ later
type NotificationFormData struct {
	NotificationService *struct {
		ID                      uint
		Name                    string
		Description             string
		Type                    string
		IsEnabled               bool
		EventTriggers           []string
		RetryPolicy             string
		WebhookURL              string
		Method                  string
		Headers                 string
		PayloadTemplate         string
		SecretKey               string
		PushbulletAPIKey        string
		PushbulletDeviceID      string
		PushbulletTitleTemplate string
		PushbulletBodyTemplate  string
		NtfyServer              string
		NtfyTopic               string
		NtfyPriority            string
		NtfyUsername            string
		NtfyPassword            string
		NtfyTitleTemplate       string
		NtfyMessageTemplate     string
		GotifyURL               string
		GotifyToken             string
		GotifyPriority          string
		GotifyTitleTemplate     string
		GotifyMessageTemplate   string
		PushoverAPIToken        string
		PushoverUserKey         string
		PushoverDevice          string
		PushoverPriority        string
		PushoverSound           string
		PushoverTitleTemplate   string
		PushoverMessageTemplate string
	}
	IsNew          bool
	SuccessMessage string
	ErrorMessage   string
}
