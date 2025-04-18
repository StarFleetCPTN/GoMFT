package components

type RcloneImportPreview struct {
	Remotes []RcloneRemotePreview
	Error   string
}

type RcloneRemotePreview struct {
	Name   string
	Type   string
	Fields map[string]string
	Import bool // Should import
}
