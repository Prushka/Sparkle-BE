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

func TestCorrection(t *testing.T) {
	fBytes, err := os.ReadFile("3-eng.ass")
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	headers, _, err := sanitizeInputASS(string(fBytes))
	if err != nil {
		t.Fatalf("Failed to sanitize input: %v", err)
	}

	fmt.Println(strings.Join(correctTimestamps(headers, `
Dialogue: 0,0:01:08.26,0:01:09.55,Default,,0,0,0,,Prepare a transfusion!
Dialogue: 0,0:01:09.55,0:01:10.80,Default,,0,0,0,,Can you hear me?
Dialogue: 0,0:01:10.80,0:01:12.56,Default,,0,0,0,,Do you know your name?
Dialogue: 0,0:01:12.56,0:01:13.76,Default,,0,0,0,,Can you speak?
Dialogue: 0,0:01:13.76,0:01:16.68,Default,,0,0,0,,{\i1}Huh? Am I dying?
Dialogue: 0,0:01:16.68,0:01:20.10,Default,,0,0,0,,{\i1}Oh well, my life was basically over anyway.
Dialogue: 0,0:01:21.94,0:01:25.40,Default,,0,0,0,,{\i1}I wish I could've at least lost my virginity.
Dialogue: 0,0:01:26.53,0:01:30.16,Default,,0,0,0,,Epabuhia! Tauwka seio kwa! Pahail za!
Dialogue: 0,0:01:30.16,0:01:33.16,Default,,0,0,0,,Ivin kav Zenith! Pahail za!
Dialogue: 0,0:01:36.58,0:01:38.00,Default,,0,0,0,,{\i1}Ow, my eyes!
Dialogue: 0,0:01:38.58,0:01:40.37,Default,,0,0,0,,{\i1}Am I still alive?
Dialogue: 0,0:01:42.38,0:01:46.59,Default,,0,0,0,,{\an8}Zevaiah bazalfben kifu zivaikah kwav!
Dialogue: 0,0:01:43.17,0:01:45.21,Default,,0,0,0,,{\i1}Hey, who's this guy?
Dialogue: 0,0:01:47.51,0:01:48.84,Default,,0,0,0,,{\i1}Seriously?
Dialogue: 0,0:01:48.84,0:01:50.97,Default,,0,0,0,,{\i1}I'm 100-plus kilos, and he lifted me like...
Dialogue: 0,0:01:50.05,0:01:54.22,Default,,0,0,0,,{\an8}Bukay aiha naiboo!
Dialogue: 0,0:01:50.97,0:01:52.18,Default,,0,0,0,,{\i1}Whoa! What the hell?!
Dialogue: 0,0:01:52.18,0:01:54.22,Default,,0,0,0,,{\i1}Stop! That's gross!`,
		`
Dialogue: 0,0:01:08.26,0:01:09.55,Default,,0,0,0,,Prepare a transfusion!
Dialogue: 0,0:01:09.55,0:01:10.80,Default,,0,0,0,,Can you hear me?
Dialogue: 0,0:01:10.80,0:01:12.56,Default,,0,0,0,,Do you know your name?
Dialogue: 0,0:01:12.56,0:01:13.76,Default,,0,0,0,,Can you speak?
Dialogue: 0,0:01:13.76,0:01:16.68,Default,,0,0,0,,{\i1}Huh? Am I dying?
Dialogue: 0,0:01:16.68,0:01:20.10,Default,,0,0,0,,{\i1}Oh well, my life was basically over anyway.
Dialogue: 0,0:01:21.94,0:01:5.40,Default,,0,0,0,,{\i1}I wish I could've at least lost my virginity.
Dialogue: 0,0:01:26.53,0:01:30.16,Default,,0,0,0,,Epabuhia! Tauwka seio kwa! Pahail za!
Dialogue: 0,0:01:30.16,0:01:33.16,Default,,0,0,0,,Ivin kav Zenith! Pahail za!
Dialogue: 0,0:01:36.58,0:01:38.00,Default,,0,0,0,,{\i1}Ow, my eyes!
Dialogue: 0,0:01:38.58,0:01:40.37,Default,,0,0,0,,{\i1}Am I still alive?
Dialogue: 0,0:42.38,0:01:46.59,Default,,0,0,0,,{\an8}Zevaiah bazalfben kifu zivaikah kwav!
Dialogue: 0,0:01:43.17,0:01:45.21,Default,,0,0,0,,{\i1}Hey, who's this guy?
Dialogue: 0,0:01:47.51,0:01:48.84,Default,,0,0,0,,{\i1}Seriously?
Dialogue: 0,0:01:48.84,0:01:50.97,Default,,0,0,0,,{\i1}I'm 100-plus kilos, and he lifted me like...
Dialogue: 0,0:01:50.05,0:01:54.22,Default,,0,0,0,,{\an8}Bukay aiha naiboo!
Dialogue: 0,0:01:50.97,0:01:52.18,Default,,0,0,0,,{\i1}Whoa! What the hell?!
Dialogue: 0,0:01:52.18,0:01:54.2,Default,,0,0,0,,{\i1}Stop! That's gross!`), "\n"))
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
