package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// processMkvFile runs the ffprobe command on a given .mkv file.
func processMkvFile(path string) {

	// Prepare the ffprobe command
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "s", "-show_entries", "stream=codec_name", "-of", "csv=p=0", path)

	// Execute the command and capture the output
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error running ffprobe on %s: %v\n", path, err)
		return
	}

	// Print the codec name if any output is received
	if strings.Contains(string(output), "image") || strings.Contains(string(output), "dvd") || strings.Contains(string(output), "vob") {
		fmt.Printf("Subtitle codec for %s: %s\n", path, strings.TrimSpace(string(output)))
	}
}

func main() {
	rootDir := "R:\\Managed-Videos\\Anime"

	// Walk the directory recursively
	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Check if the entry is a file and has a .mkv extension
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".mkv") {
			processMkvFile(path)
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Error walking the path %q: %v\n", rootDir, err)
	}
}
