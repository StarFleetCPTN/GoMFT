package providers

import (
	"context"
	"strings"
	"testing"

	"github.com/starfleetcptn/gomft/components/providers/common"
	"github.com/starfleetcptn/gomft/components/providers/destination"
	"github.com/starfleetcptn/gomft/components/providers/source"
	"github.com/stretchr/testify/assert"
)

// Test that both source and destination providers can be rendered together with common components
func TestProvidersIntegration(t *testing.T) {
	// Create context for test
	ctx := context.Background()
	assert := assert.New(t)

	// Test rendering common components
	{
		var buf strings.Builder
		err := common.NameField().Render(ctx, &buf)
		assert.NoError(err, "Failed to render NameField")
		html := buf.String()
		assert.Contains(html, `<label for="name"`)
	}

	// Test rendering source components
	{
		var buf strings.Builder
		err := source.LocalSourceForm().Render(ctx, &buf)
		assert.NoError(err, "Failed to render LocalSourceForm")
		html := buf.String()
		assert.Contains(html, `<label for="source_path"`)
	}

	// Test rendering destination components
	{
		var buf strings.Builder
		err := destination.LocalDestinationForm().Render(ctx, &buf)
		assert.NoError(err, "Failed to render LocalDestinationForm")
		html := buf.String()
		assert.Contains(html, `<label for="destination_path"`)
	}
}

// Test that all source providers are available
func TestSourceProviders(t *testing.T) {
	// Create context for test
	ctx := context.Background()
	assert := assert.New(t)

	// Test each source provider can be rendered
	providers := []struct {
		name     string
		template func() (string, error)
	}{
		{"LocalSourceForm", func() (string, error) {
			var buf strings.Builder
			err := source.LocalSourceForm().Render(ctx, &buf)
			return buf.String(), err
		}},
		{"SFTPSourceForm", func() (string, error) {
			var buf strings.Builder
			err := source.SFTPSourceForm().Render(ctx, &buf)
			return buf.String(), err
		}},
		{"S3SourceForm", func() (string, error) {
			var buf strings.Builder
			err := source.S3SourceForm().Render(ctx, &buf)
			return buf.String(), err
		}},
		{"FTPSourceForm", func() (string, error) {
			var buf strings.Builder
			err := source.FTPSourceForm().Render(ctx, &buf)
			return buf.String(), err
		}},
		{"SMBSourceForm", func() (string, error) {
			var buf strings.Builder
			err := source.SMBSourceForm().Render(ctx, &buf)
			return buf.String(), err
		}},
		{"WebDAVSourceForm", func() (string, error) {
			var buf strings.Builder
			err := source.WebDAVSourceForm().Render(ctx, &buf)
			return buf.String(), err
		}},
	}

	for _, provider := range providers {
		t.Run(provider.name, func(t *testing.T) {
			html, err := provider.template()
			assert.NoError(err, "Failed to render "+provider.name)
			assert.NotEmpty(html, provider.name+" rendered empty HTML")
		})
	}
}

// Test that all destination providers are available
func TestDestinationProviders(t *testing.T) {
	// Create context for test
	ctx := context.Background()
	assert := assert.New(t)

	// Test each destination provider can be rendered
	providers := []struct {
		name     string
		template func() (string, error)
	}{
		{"LocalDestinationForm", func() (string, error) {
			var buf strings.Builder
			err := destination.LocalDestinationForm().Render(ctx, &buf)
			return buf.String(), err
		}},
		{"SFTPDestinationForm", func() (string, error) {
			var buf strings.Builder
			err := destination.SFTPDestinationForm().Render(ctx, &buf)
			return buf.String(), err
		}},
		{"S3DestinationForm", func() (string, error) {
			var buf strings.Builder
			err := destination.S3DestinationForm().Render(ctx, &buf)
			return buf.String(), err
		}},
		{"FTPDestinationForm", func() (string, error) {
			var buf strings.Builder
			err := destination.FTPDestinationForm().Render(ctx, &buf)
			return buf.String(), err
		}},
		{"SMBDestinationForm", func() (string, error) {
			var buf strings.Builder
			err := destination.SMBDestinationForm().Render(ctx, &buf)
			return buf.String(), err
		}},
		{"WebDAVDestinationForm", func() (string, error) {
			var buf strings.Builder
			err := destination.WebDAVDestinationForm().Render(ctx, &buf)
			return buf.String(), err
		}},
	}

	for _, provider := range providers {
		t.Run(provider.name, func(t *testing.T) {
			html, err := provider.template()
			assert.NoError(err, "Failed to render "+provider.name)
			assert.NotEmpty(html, provider.name+" rendered empty HTML")
		})
	}
}

// Test the complete configuration wizard flow
func TestConfigurationWizard(t *testing.T) {
	// Create context for test
	ctx := context.Background()
	assert := assert.New(t)

	// First test common configuration fields
	var buf strings.Builder
	err := common.NameField().Render(ctx, &buf)
	assert.NoError(err, "Failed to render name field")
	nameField := buf.String()
	assert.Contains(nameField, `<input type="text" name="name" id="name"`)

	// Test source selection
	buf.Reset()
	err = common.SourceSelection().Render(ctx, &buf)
	assert.NoError(err, "Failed to render source selection")
	sourceSelection := buf.String()
	assert.Contains(sourceSelection, `<select id="source_type" name="source_type"`)

	// Test specific source form (local example)
	buf.Reset()
	err = source.LocalSourceForm().Render(ctx, &buf)
	assert.NoError(err, "Failed to render local source form")
	localSource := buf.String()
	assert.Contains(localSource, `<input type="text" name="source_path" id="source_path"`)

	// Test destination selection
	buf.Reset()
	err = common.DestinationSelection().Render(ctx, &buf)
	assert.NoError(err, "Failed to render destination selection")
	destinationSelection := buf.String()
	assert.Contains(destinationSelection, `<select id="destination_type" name="destination_type"`)

	// Test specific destination form (S3 example)
	buf.Reset()
	err = destination.S3DestinationForm().Render(ctx, &buf)
	assert.NoError(err, "Failed to render S3 destination form")
	s3Destination := buf.String()
	assert.Contains(s3Destination, `<input type="text" name="dest_bucket" id="dest_bucket"`)

	// Test advanced options
	buf.Reset()
	err = common.ArchiveOptions().Render(ctx, &buf)
	assert.NoError(err, "Failed to render archive options")
	archiveOptions := buf.String()
	assert.Contains(archiveOptions, `Enable archiving`)

	buf.Reset()
	err = common.FilePatternFields().Render(ctx, &buf)
	assert.NoError(err, "Failed to render file pattern fields")
	filePatterns := buf.String()
	assert.Contains(filePatterns, `<input type="text" name="file_pattern" id="file_pattern"`)

	// All essential components for the configuration wizard are present and renderable
}

// Test that provider forms have proper conditional logic
func TestProviderFormConditionals(t *testing.T) {
	// Create context for test
	ctx := context.Background()
	assert := assert.New(t)

	// Test SFTP Source form conditionals (password vs key file)
	{
		var buf strings.Builder
		err := source.SFTPSourceForm().Render(ctx, &buf)
		assert.NoError(err, "Failed to render SFTP source form")
		html := buf.String()

		// Should have auth type selection
		assert.Contains(html, `<select id="source_auth_type" name="source_auth_type"`)

		// Should have password field that's conditionally shown
		assert.Contains(html, `x-show="sourceAuthType === 'password'"`)
		assert.Contains(html, `<input type="password" name="source_password"`)

		// Should have key file field that's conditionally shown
		assert.Contains(html, `x-show="sourceAuthType === 'key_file'"`)
		assert.Contains(html, `<input type="text" name="source_key_file"`)
	}

	// Test S3 Source form conditionals
	{
		var buf strings.Builder
		err := source.S3SourceForm().Render(ctx, &buf)
		assert.NoError(err, "Failed to render S3 source form")
		html := buf.String()

		// Should have optional endpoint field
		assert.Contains(html, `Custom Endpoint`)
		assert.Contains(html, `<input type="text" name="source_endpoint"`)

		// Should have both required and optional fields
		assert.Contains(html, `<input type="text" name="source_bucket" id="source_bucket" required`)
		assert.Contains(html, `<input type="text" name="source_region" id="source_region"`)
	}

	// Test advanced options show/hide behavior
	{
		var buf strings.Builder
		err := common.ArchiveOptions().Render(ctx, &buf)
		assert.NoError(err, "Failed to render archive options")
		html := buf.String()

		// Archive path should only show when archive is enabled
		assert.Contains(html, `x-show="archiveEnabled"`)
		assert.Contains(html, `<input type="text" name="archive_path" id="archive_path"`)

		// Toggle behavior
		assert.Contains(html, `x-model="archiveEnabled"`)
		assert.Contains(html, `<input type="checkbox"`)
	}
}

// Test for accessibility attributes in provider forms
func TestProviderFormsAccessibility(t *testing.T) {
	// Create context for test
	ctx := context.Background()
	assert := assert.New(t)

	// Test source form for accessibility
	{
		var buf strings.Builder
		err := source.LocalSourceForm().Render(ctx, &buf)
		assert.NoError(err, "Failed to render local source form")
		html := buf.String()

		// Should have labels with proper for attributes
		assert.Contains(html, `<label for="source_path"`)

		// Should have input with id matching label's for attribute
		assert.Contains(html, `<input type="text" name="source_path" id="source_path"`)

		// Should have aria attributes
		assert.Contains(html, `aria-label`)
		assert.Contains(html, `aria-describedby`)
	}

	// Test destination form for accessibility
	{
		var buf strings.Builder
		err := destination.LocalDestinationForm().Render(ctx, &buf)
		assert.NoError(err, "Failed to render local destination form")
		html := buf.String()

		// Should have labels with proper for attributes
		assert.Contains(html, `<label for="destination_path"`)

		// Should have input with id matching label's for attribute
		assert.Contains(html, `<input type="text" name="destination_path" id="destination_path"`)

		// Should have aria attributes
		assert.Contains(html, `aria-label`)
		assert.Contains(html, `aria-describedby`)
	}
}

// Test dynamic form rendering based on provider selection
func TestDynamicFormRendering(t *testing.T) {
	// Create context for test
	ctx := context.Background()
	assert := assert.New(t)

	// Test source selection dynamic rendering
	{
		var buf strings.Builder
		err := common.SourceSelection().Render(ctx, &buf)
		assert.NoError(err, "Failed to render source selection")
		html := buf.String()

		// Should have x-model for binding selected value
		assert.Contains(html, `x-model="sourceType"`)

		// Should have template for dynamic rendering
		assert.Contains(html, `x-show="sourceType === 'local'"`)
		assert.Contains(html, `x-show="sourceType === 'sftp'"`)
		assert.Contains(html, `x-show="sourceType === 'ftp'"`)
		assert.Contains(html, `x-show="sourceType === 's3'"`)
	}

	// Test destination selection dynamic rendering
	{
		var buf strings.Builder
		err := common.DestinationSelection().Render(ctx, &buf)
		assert.NoError(err, "Failed to render destination selection")
		html := buf.String()

		// Should have x-model for binding selected value
		assert.Contains(html, `x-model="destinationType"`)

		// Should have template for dynamic rendering
		assert.Contains(html, `x-show="destinationType === 'local'"`)
		assert.Contains(html, `x-show="destinationType === 'sftp'"`)
		assert.Contains(html, `x-show="destinationType === 'ftp'"`)
		assert.Contains(html, `x-show="destinationType === 's3'"`)
	}

	// Test for proper Alpine.js initialization
	{
		var buf strings.Builder
		err := source.LocalSourceForm().Render(ctx, &buf)
		assert.NoError(err)
		html := buf.String()

		// Should initialize Alpine.js data properly
		assert.Contains(html, `x-data=`)

		// Should have state variables
		assert.Contains(html, `sourceType:`)
		assert.Contains(html, `sourcePath:`)
	}

	// Test that wizard has a submission handler
	{
		var buf strings.Builder
		// Here we'd render the full form container if available
		// Using source selection as a proxy
		err := common.SourceSelection().Render(ctx, &buf)
		assert.NoError(err)
		html := buf.String()

		// Should have form tag with action/method
		assert.Contains(html, `<form`)
		assert.Contains(html, `method="POST"`)

		// Should have submit button
		assert.Contains(html, `type="submit"`)
		assert.Contains(html, `Save Configuration`)
	}
}
