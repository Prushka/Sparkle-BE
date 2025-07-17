package translation

import (
	"os"
	"testing"
)

func TestSanitization(t *testing.T) {
	// read from output.vtt
	fBytes, err := os.ReadFile("output.vtt")
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	// write to output_sanitized.vtt
	sanitized := sanitizeInputVTT(string(fBytes))
	if err := os.WriteFile("output_sanitized.vtt", []byte(sanitized), 0644); err != nil {
		t.Fatalf("Failed to write sanitized file: %v", err)
	}

}
