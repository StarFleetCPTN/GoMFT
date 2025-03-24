package db

import (
	"database/sql"
	"encoding/json"
	"time"
)

// ProviderType represents the type of authentication provider
type ProviderType string

const (
	// ProviderTypeAuthentik represents an Authentik authentication provider
	ProviderTypeAuthentik ProviderType = "authentik"

	// ProviderTypeOIDC represents an OpenID Connect authentication provider
	ProviderTypeOIDC ProviderType = "oidc"

	// ProviderTypeSAML represents a SAML authentication provider
	ProviderTypeSAML ProviderType = "saml"

	// ProviderTypeOAuth2 represents an OAuth2 authentication provider
	ProviderTypeOAuth2 ProviderType = "oauth2"
)

// AuthProvider represents an external authentication provider configuration
type AuthProvider struct {
	ID               uint         `gorm:"primarykey" json:"id"`
	Name             string       `gorm:"not null" json:"name"`
	Type             ProviderType `gorm:"not null" json:"type"`
	Enabled          bool         `gorm:"default:true" json:"enabled"`
	Description      string       `json:"description"`
	ProviderURL      string       `json:"provider_url"`
	ClientID         string       `json:"client_id"`
	ClientSecret     string       `json:"-"` // Not returned in JSON responses
	RedirectURL      string       `json:"redirect_url"`
	Scopes           string       `json:"scopes"`
	AttributeMapping string       `json:"attribute_mapping"`
	Config           string       `json:"-"`        // Stores provider-specific configuration
	IconURL          string       `json:"icon_url"` // URL to provider icon
	SuccessfulLogins int          `json:"successful_logins"`
	LastUsed         sql.NullTime `json:"last_used"`
	CreatedAt        time.Time    `json:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at"`

	// Unmarshalled config
	configData map[string]interface{} `gorm:"-" json:"-"`
}

// GetConfig returns the unmarshalled configuration data
func (p *AuthProvider) GetConfig() (map[string]interface{}, error) {
	if p.configData == nil && p.Config != "" {
		err := json.Unmarshal([]byte(p.Config), &p.configData)
		if err != nil {
			return nil, err
		}
	}

	if p.configData == nil {
		p.configData = make(map[string]interface{})
	}

	return p.configData, nil
}

// SetConfig sets the configuration data and marshals it to JSON
func (p *AuthProvider) SetConfig(data map[string]interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	p.Config = string(jsonData)
	p.configData = data
	return nil
}

// ExternalUserIdentity represents a user identity from an external authentication provider
type ExternalUserIdentity struct {
	ID           uint         `gorm:"primarykey" json:"id"`
	UserID       uint         `gorm:"not null" json:"user_id"`
	ProviderID   uint         `gorm:"not null" json:"provider_id"`
	ProviderType ProviderType `gorm:"not null" json:"provider_type"`
	ExternalID   string       `gorm:"not null" json:"external_id"`
	Email        string       `gorm:"not null" json:"email"`
	Username     string       `json:"username"`
	DisplayName  string       `json:"display_name"`
	Groups       string       `json:"groups"` // JSON array of groups
	LastLogin    sql.NullTime `json:"last_login"`
	ProviderData string       `json:"-"` // Raw data from provider
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`

	// Foreign key relationships
	User     User         `gorm:"foreignKey:UserID" json:"-"`
	Provider AuthProvider `gorm:"foreignKey:ProviderID" json:"-"`

	// Unmarshalled provider data
	providerDataObj map[string]interface{} `gorm:"-" json:"-"`
}

// GetProviderData returns the unmarshalled provider data
func (e *ExternalUserIdentity) GetProviderData() (map[string]interface{}, error) {
	if e.providerDataObj == nil && e.ProviderData != "" {
		err := json.Unmarshal([]byte(e.ProviderData), &e.providerDataObj)
		if err != nil {
			return nil, err
		}
	}

	if e.providerDataObj == nil {
		e.providerDataObj = make(map[string]interface{})
	}

	return e.providerDataObj, nil
}

// SetProviderData sets the provider data and marshals it to JSON
func (e *ExternalUserIdentity) SetProviderData(data map[string]interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	e.ProviderData = string(jsonData)
	e.providerDataObj = data
	return nil
}

// GetGroups returns the unmarshalled groups
func (e *ExternalUserIdentity) GetGroups() ([]string, error) {
	var groups []string
	if e.Groups == "" {
		return groups, nil
	}

	err := json.Unmarshal([]byte(e.Groups), &groups)
	if err != nil {
		return nil, err
	}

	return groups, nil
}

// SetGroups sets the groups and marshals them to JSON
func (e *ExternalUserIdentity) SetGroups(groups []string) error {
	jsonData, err := json.Marshal(groups)
	if err != nil {
		return err
	}

	e.Groups = string(jsonData)
	return nil
}
