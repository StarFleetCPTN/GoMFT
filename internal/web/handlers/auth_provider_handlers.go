package handlers

// Authentication Provider handlers for the GoMFT application
import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/starfleetcptn/gomft/components"
	"github.com/starfleetcptn/gomft/internal/db"
)

// AuthProvidersPage renders the page listing all authentication providers
func (h *Handlers) AuthProvidersPage(c *gin.Context) {
	// Get user from context
	userID := c.GetUint("userID")
	if userID == 0 {
		h.HandleUnauthorized(c)
		return
	}

	// Get all auth providers
	providers, err := h.DB.GetAllAuthProviders(c.Request.Context())
	if err != nil {
		h.HandleServerError(c, err)
		return
	}

	components.AuthProviders(c.Request.Context(), providers).Render(c, c.Writer)

}

// NewAuthProviderPage renders the form to create a new authentication provider
func (h *Handlers) NewAuthProviderPage(c *gin.Context) {
	// Get user from context
	userID := c.GetUint("userID")
	if userID == 0 {
		h.HandleUnauthorized(c)
		return
	}

	// Create an empty auth provider for the form
	provider := &db.AuthProvider{}

	// Render the auth provider form for creating a new provider
	components.AuthProviderForm(c.Request.Context(), provider, true).Render(c, c.Writer)
}

// EditAuthProviderPage renders the form to edit an existing authentication provider
func (h *Handlers) EditAuthProviderPage(c *gin.Context) {
	// Get user from context
	userID := c.GetUint("userID")
	if userID == 0 {
		h.HandleUnauthorized(c)
		return
	}

	providerID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		h.HandleBadRequest(c, "Invalid Provider ID", "The provider ID is not valid")
		return
	}

	// Get the auth provider
	provider, err := h.DB.GetAuthProviderByID(c.Request.Context(), uint(providerID))
	if err != nil {
		h.HandleBadRequest(c, "Provider Not Found", "The authentication provider could not be found")
		return
	}

	// Render the auth provider form for editing
	components.AuthProviderForm(c.Request.Context(), provider, false).Render(c, c.Writer)
}

// HandleCreateAuthProvider handles the form submission to create a new authentication provider
func (h *Handlers) HandleCreateAuthProvider(c *gin.Context) {
	// Get user from context
	userID := c.GetUint("userID")
	if userID == 0 {
		h.HandleUnauthorized(c)
		return
	}

	if err := c.Request.ParseForm(); err != nil {
		h.HandleBadRequest(c, "Invalid Form Data", "Could not parse form data")
		return
	}

	provider := &db.AuthProvider{
		Name:         c.PostForm("name"),
		Type:         db.ProviderType(c.PostForm("type")),
		ProviderURL:  c.PostForm("provider_url"),
		ClientID:     c.PostForm("client_id"),
		ClientSecret: c.PostForm("client_secret"),
		RedirectURL:  c.PostForm("redirect_url"),
		Scopes:       c.PostForm("scopes"),
		Description:  c.PostForm("description"),
		IconURL:      c.PostForm("icon_url"),
		Enabled:      c.PostForm("enabled") == "on",
	}

	// Process config values based on provider type
	config := make(map[string]interface{})
	switch provider.Type {
	case db.ProviderTypeAuthentik:
		if tenant := c.PostForm("authentik_tenant"); tenant != "" {
			config["tenant_id"] = tenant
		}
	case db.ProviderTypeOIDC:
		if discoveryURL := c.PostForm("oidc_discovery_url"); discoveryURL != "" {
			config["discovery_url"] = discoveryURL
		}
	case db.ProviderTypeSAML:
		if metadataURL := c.PostForm("saml_metadata_url"); metadataURL != "" {
			config["metadata_url"] = metadataURL
		}
	}

	// Set the config
	if len(config) > 0 {
		configJSON, err := json.Marshal(config)
		if err != nil {
			h.HandleBadRequest(c, "Invalid Configuration", "Could not process provider configuration")
			return
		}
		provider.Config = string(configJSON)
	}

	// Process attribute mappings
	attrMapping := map[string]string{
		"username": c.PostForm("attr_username"),
		"email":    c.PostForm("attr_email"),
		"name":     c.PostForm("attr_name"),
		"groups":   c.PostForm("attr_groups"),
	}

	// Remove empty mappings
	for k, v := range attrMapping {
		if v == "" {
			delete(attrMapping, k)
		}
	}

	if len(attrMapping) > 0 {
		mappingJSON, err := json.Marshal(attrMapping)
		if err != nil {
			h.HandleBadRequest(c, "Invalid Attribute Mapping", "Could not process attribute mappings")
			return
		}
		provider.AttributeMapping = string(mappingJSON)
	}

	// Create the provider
	if err := h.DB.CreateAuthProvider(c.Request.Context(), provider); err != nil {
		h.HandleServerError(c, err)
		return
	}

	log.Printf("[Audit] User %d created auth provider %d of type %s with name '%s'",
		userID, provider.ID, provider.Type, provider.Name)

	// Set success flash and redirect
	c.SetCookie("flash_message", fmt.Sprintf("Authentication provider '%s' created successfully", provider.Name),
		3600, "/", "", false, true)
	c.SetCookie("flash_type", "success", 3600, "/", "", false, true)

	c.Redirect(http.StatusFound, "/admin/settings/auth-providers")
}

// HandleUpdateAuthProvider handles the form submission to update an existing authentication provider
func (h *Handlers) HandleUpdateAuthProvider(c *gin.Context) {
	// Get user from context
	userID := c.GetUint("userID")
	if userID == 0 {
		h.HandleUnauthorized(c)
		return
	}

	providerID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		h.HandleBadRequest(c, "Invalid Provider ID", "The provider ID is not valid")
		return
	}

	// Get the existing provider
	existingProvider, err := h.DB.GetAuthProviderByID(c.Request.Context(), uint(providerID))
	if err != nil {
		h.HandleBadRequest(c, "Provider Not Found", "The authentication provider could not be found")
		return
	}

	if err := c.Request.ParseForm(); err != nil {
		h.HandleBadRequest(c, "Invalid Form Data", "Could not parse form data")
		return
	}

	// Update the provider with form data
	existingProvider.Name = c.PostForm("name")
	existingProvider.ProviderURL = c.PostForm("provider_url")
	existingProvider.ClientID = c.PostForm("client_id")
	existingProvider.IconURL = c.PostForm("icon_url")

	// Only update client secret if provided
	if clientSecret := c.PostForm("client_secret"); clientSecret != "" {
		existingProvider.ClientSecret = clientSecret
	}

	existingProvider.RedirectURL = c.PostForm("redirect_url")
	existingProvider.Scopes = c.PostForm("scopes")
	existingProvider.Description = c.PostForm("description")
	existingProvider.Enabled = c.PostForm("enabled") == "on"

	// Update the type if changed
	if providerType := c.PostForm("type"); providerType != "" {
		existingProvider.Type = db.ProviderType(providerType)
	}

	// Process config values based on provider type
	config := make(map[string]interface{})
	switch existingProvider.Type {
	case db.ProviderTypeAuthentik:
		if tenant := c.PostForm("authentik_tenant"); tenant != "" {
			config["tenant_id"] = tenant
		}
	case db.ProviderTypeOIDC:
		if discoveryURL := c.PostForm("oidc_discovery_url"); discoveryURL != "" {
			config["discovery_url"] = discoveryURL
		}
	case db.ProviderTypeSAML:
		if metadataURL := c.PostForm("saml_metadata_url"); metadataURL != "" {
			config["metadata_url"] = metadataURL
		}
	}

	// Set the config
	if len(config) > 0 {
		configJSON, err := json.Marshal(config)
		if err != nil {
			h.HandleBadRequest(c, "Invalid Configuration", "Could not process provider configuration")
			return
		}
		existingProvider.Config = string(configJSON)
	}

	// Process attribute mappings
	attrMapping := map[string]string{
		"username": c.PostForm("attr_username"),
		"email":    c.PostForm("attr_email"),
		"name":     c.PostForm("attr_name"),
		"groups":   c.PostForm("attr_groups"),
	}

	// Remove empty mappings
	for k, v := range attrMapping {
		if v == "" {
			delete(attrMapping, k)
		}
	}

	if len(attrMapping) > 0 {
		mappingJSON, err := json.Marshal(attrMapping)
		if err != nil {
			h.HandleBadRequest(c, "Invalid Attribute Mapping", "Could not process attribute mappings")
			return
		}
		existingProvider.AttributeMapping = string(mappingJSON)
	}

	// Update the provider
	if err := h.DB.UpdateAuthProvider(c.Request.Context(), existingProvider); err != nil {
		h.HandleServerError(c, err)
		return
	}

	log.Printf("[Audit] User %d updated auth provider %d of type %s with name '%s'",
		userID, existingProvider.ID, existingProvider.Type, existingProvider.Name)

	// Set success flash and redirect
	c.SetCookie("flash_message", fmt.Sprintf("Authentication provider '%s' updated successfully", existingProvider.Name),
		3600, "/", "", false, true)
	c.SetCookie("flash_type", "success", 3600, "/", "", false, true)

	c.Redirect(http.StatusFound, "/admin/settings/auth-providers")
}

// HandleDeleteAuthProvider handles the request to delete an authentication provider
func (h *Handlers) HandleDeleteAuthProvider(c *gin.Context) {
	// Get user from context
	userID := c.GetUint("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	providerID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid provider ID",
		})
		return
	}

	// Get the provider to be deleted
	provider, err := h.DB.GetAuthProviderByID(c.Request.Context(), uint(providerID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Authentication provider not found",
		})
		return
	}

	// Check if there are any user identities associated with this provider
	count, err := h.DB.CountExternalUserIdentitiesByProviderID(c.Request.Context(), uint(providerID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to check if provider is in use",
		})
		return
	}

	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Cannot delete provider '%s' because it has %d associated user identities", provider.Name, count),
		})
		return
	}

	// Delete the provider
	if err := h.DB.DeleteAuthProvider(c.Request.Context(), uint(providerID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete authentication provider",
		})
		return
	}

	log.Printf("[Audit] User %d deleted auth provider %d of type %s with name '%s'",
		userID, provider.ID, provider.Type, provider.Name)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Authentication provider '%s' deleted successfully", provider.Name),
	})
}

// HandleTestAuthProviderConnection tests the connection to an authentication provider
func (h *Handlers) HandleTestAuthProviderConnection(c *gin.Context) {
	// Get user from context
	userID := c.GetUint("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	providerID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid provider ID",
		})
		return
	}

	// Get the provider
	provider, err := h.DB.GetAuthProviderByID(c.Request.Context(), uint(providerID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Authentication provider not found",
		})
		return
	}

	// Attempt to test the connection based on provider type
	var testResult error
	switch provider.Type {
	case db.ProviderTypeAuthentik:
		testResult = h.testAuthentikConnection(provider)
	case db.ProviderTypeOIDC:
		testResult = h.testOIDCConnection(provider)
	case db.ProviderTypeSAML:
		testResult = h.testSAMLConnection(provider)
	case db.ProviderTypeOAuth2:
		testResult = h.testOAuth2Connection(provider)
	default:
		testResult = fmt.Errorf("unsupported provider type: %s", provider.Type)
	}

	if testResult != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Connection test failed: %s", testResult.Error()),
		})
		return
	}

	log.Printf("[Audit] User %d successfully tested connection to auth provider %d of type %s",
		userID, provider.ID, provider.Type)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Connection test successful",
	})
}

// Test methods for different provider types
func (h *Handlers) testAuthentikConnection(provider *db.AuthProvider) error {
	// TODO: Implement actual test logic for Authentik
	return nil
}

func (h *Handlers) testOIDCConnection(provider *db.AuthProvider) error {
	// TODO: Implement actual test logic for OIDC
	return nil
}

func (h *Handlers) testSAMLConnection(provider *db.AuthProvider) error {
	// TODO: Implement actual test logic for SAML
	return nil
}

func (h *Handlers) testOAuth2Connection(provider *db.AuthProvider) error {
	// TODO: Implement actual test logic for OAuth2
	return nil
}
