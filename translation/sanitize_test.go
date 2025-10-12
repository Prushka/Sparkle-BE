package translation

import (
	"Sparkle/discord"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitization(t *testing.T) {
	fBytes, err := os.ReadFile("5-eng.ass")
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	sub, err := sanitizeInputASS(string(fBytes))
	if err != nil {
		t.Fatalf("Failed to sanitize input: %v", err)
	}

	if err := os.WriteFile("output_distilled.ass", []byte(strings.Join(sub.distilledDialogues, "\n")), 0644); err != nil {
		t.Fatalf("Failed to write sanitized file: %v", err)
	}
	if err := os.WriteFile("output_headers.ass", []byte(strings.Join(sub.headers, "\n")), 0644); err != nil {
		t.Fatalf("Failed to write sanitized file: %v", err)
	}

}

func TestProcess(t *testing.T) {

	fBytes, err := os.ReadFile("5-eng.ass")
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	sub, err := sanitizeInputASS(string(fBytes))
	if err != nil {
		t.Fatalf("Failed to sanitize input: %v", err)
	}
	inputsPairs := splitByCharacters(sub.distilledDialogues, 29000)
	text, err := sub.process(inputsPairs[0], []string{
		"0:00:03.34,0:00:04.43,From my room,",
		"0:00:04.63,0:00:07.13,I can see the neighbor's backyard.",
		"0:00:07.34,0:00:09.13,It doesn't get a lot of sunlight...",
		"0:00:28.65,0:00:32.24,Rina-chan from next door is \na few years younger than I am,",
		"0:00:32.45,0:00:34.04,but apparently she's been sick all her life",
		"0:00:34.24,0:00:6.03,and never leaves the house.",
	})
	if err != nil {
		t.Fatalf("Failed to process: %v", err)
	}
	fmt.Println(text)
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
