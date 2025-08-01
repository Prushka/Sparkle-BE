package translation

import (
	"Sparkle/discord"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitization(t *testing.T) {
	// read from output.vtt
	fBytes, err := os.ReadFile("3-eng.ass")
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	// write to output_sanitized.vtt
	headers, sanitized, err := sanitizeInputASS(string(fBytes))
	if err != nil {
		t.Fatalf("Failed to sanitize input: %v", err)
	}

	if err := os.WriteFile("output_sanitized.ass", []byte(sanitized), 0644); err != nil {
		t.Fatalf("Failed to write sanitized file: %v", err)
	}
	if err := os.WriteFile("output_headers.ass", []byte(headers), 0644); err != nil {
		t.Fatalf("Failed to write sanitized file: %v", err)
	}
	if err := os.WriteFile("output.ass", []byte(sanitizeOutputASS(headers, sanitized)), 0644); err != nil {
		t.Fatalf("Failed to write sanitized file: %v", err)
	}

}

func TestPrintMalformedASS(t *testing.T) {
	err := ProcessFiles("/Volumes/media/Managed-Videos/")
	if err != nil {
		discord.Errorf("%v", err)
	}
}

func ProcessFiles(dir string) error {
	// Read the directory
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	// Iterate through the files in the directory
	for _, file := range files {
		// Get the full file path
		filePath := filepath.Join(dir, file.Name())

		// If the file is a directory, recursively process it
		if file.IsDir() {
			if err := ProcessFiles(filePath); err != nil {
				return err
			}
			continue
		}

		// If the file has a .ass extension, validate it
		if strings.HasSuffix(file.Name(), ".ass") {
			err = isASSFileValid(filePath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
