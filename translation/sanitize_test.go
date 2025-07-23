package translation

import (
	"os"
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
