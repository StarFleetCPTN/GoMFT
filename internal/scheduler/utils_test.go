package scheduler

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestProcessOutputPattern(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name             string
		pattern          string
		originalFilename string
		wantPatternRegex string // Use regex for date matching
	}{
		{
			name:             "Simple filename and extension",
			pattern:          "${filename}_processed.${ext}",
			originalFilename: "myfile.txt",
			wantPatternRegex: `^myfile_processed\.txt$`,
		},
		{
			name:             "Filename without extension",
			pattern:          "${filename}_backup",
			originalFilename: "important_data",
			wantPatternRegex: `^important_data_backup$`,
		},
		{
			name:             "Date formatting YYYYMMDD",
			pattern:          "${filename}_${date:20060102}.${ext}",
			originalFilename: "report.csv",
			wantPatternRegex: fmt.Sprintf(`^report_%s\.csv$`, now.Format("20060102")),
		},
		{
			name:             "Date formatting with time",
			pattern:          "${date:2006-01-02_150405}_${filename}.${ext}",
			originalFilename: "image.jpg",
			wantPatternRegex: fmt.Sprintf(`^%s_image\.jpg$`, now.Format("2006-01-02_150405")),
		},
		{
			name:             "Combined date, filename, extension",
			pattern:          "archive/${date:2006/01}/${filename}_${date:1504}.${ext}",
			originalFilename: "document.pdf",
			wantPatternRegex: fmt.Sprintf(`^archive/%s/document_%s\.pdf$`, now.Format("2006/01"), now.Format("1504")),
		},
		{
			name:             "No variables",
			pattern:          "fixed_output.dat",
			originalFilename: "input.bin",
			wantPatternRegex: `^fixed_output\.dat$`,
		},
		{
			name:             "Filename with multiple dots",
			pattern:          "${filename}.${ext}",
			originalFilename: "archive.tar.gz",
			wantPatternRegex: `^archive\.tar\.gz$`, // Ext should be .gz
		},
		{
			name:             "Pattern with only date",
			pattern:          "${date:2006}_backup",
			originalFilename: "data.zip",
			wantPatternRegex: fmt.Sprintf(`^%s_backup$`, now.Format("2006")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure the fix in utils.go (TrimPrefix) is present for this test to pass
			got := ProcessOutputPattern(tt.pattern, tt.originalFilename)
			matched, err := regexp.MatchString(tt.wantPatternRegex, got)
			if err != nil {
				t.Fatalf("Invalid regex pattern %q: %v", tt.wantPatternRegex, err)
			}
			if !matched {
				t.Errorf("ProcessOutputPattern(%q, %q) = %q, want match for regex %q", tt.pattern, tt.originalFilename, got, tt.wantPatternRegex)
			}
		})
	}
}

// Rewritten TestCreateRcloneFilterFile using parsing instead of regex matching
func TestCreateRcloneFilterFile(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name                string
		pattern             string
		wantReplacementRule string // Expected replacement part for the main rule
		wantFallbackRule    string // Expected replacement part for the fallback rule
		wantErr             bool
	}{
		{
			name:                "Simple rename with date",
			pattern:             "${date:20060102}_${filename}.${ext}",
			wantReplacementRule: fmt.Sprintf("%s_{1}.{2}", now.Format("20060102")),
			wantFallbackRule:    fmt.Sprintf("%s_{1}.", now.Format("20060102")),
			wantErr:             false,
		},
		{
			name:                "Filename only",
			pattern:             "prefix_${filename}",
			wantReplacementRule: "prefix_{1}",
			wantFallbackRule:    "prefix_{1}",
			wantErr:             false,
		},
		{
			name:                "Extension only (unlikely but test)",
			pattern:             "file.${ext}",
			wantReplacementRule: "file.{2}",
			wantFallbackRule:    "file.", // Fallback has no {2}
			wantErr:             false,
		},
		{
			name:                "Complex pattern with slashes",
			pattern:             "processed/${date:2006/01}/${filename}_backup.${ext}",
			wantReplacementRule: fmt.Sprintf("processed/%s/{1}_backup.{2}", now.Format("2006/01")),
			wantFallbackRule:    fmt.Sprintf("processed/%s/{1}_backup.", now.Format("2006/01")),
			wantErr:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath, err := createRcloneFilterFile(tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Fatalf("createRcloneFilterFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return // Expected error, test passed
			}
			defer os.Remove(filePath) // Clean up the temp file

			contentBytes, readErr := os.ReadFile(filePath)
			if readErr != nil {
				t.Fatalf("Failed to read created filter file %q: %v", filePath, readErr)
			}
			content := string(contentBytes)
			lines := strings.Split(strings.TrimSpace(content), "\n")

			if len(lines) != 2 {
				t.Fatalf("Expected 2 lines in filter file, got %d. Content:\n%s", len(lines), content)
			}

			// Define the expected source patterns literally
			expectedSourcePattern1 := `(.*)(\..+)$`
			expectedSourcePattern2 := `([^.]+)$`

			// Validate first rule (with extension)
			parts1 := strings.Fields(lines[0])
			if len(parts1) != 3 || parts1[0] != "--" || parts1[1] != expectedSourcePattern1 || parts1[2] != tt.wantReplacementRule {
				t.Errorf("Rule 1 mismatch:\n Got: %q\n Want: -- %s %s", lines[0], expectedSourcePattern1, tt.wantReplacementRule)
			}

			// Validate second rule (fallback without extension)
			parts2 := strings.Fields(lines[1])
			if len(parts2) != 3 || parts2[0] != "--" || parts2[1] != expectedSourcePattern2 || parts2[2] != tt.wantFallbackRule {
				t.Errorf("Rule 2 (fallback) mismatch:\n Got: %q\n Want: -- %s %s", lines[1], expectedSourcePattern2, tt.wantFallbackRule)
			}
		})
	}
}
