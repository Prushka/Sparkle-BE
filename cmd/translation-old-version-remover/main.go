package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func processFile(path string) error {
	// Define the date range
	startDate, err := time.Parse("2006-01-02", "2025-01-01")
	if err != nil {
		return fmt.Errorf("failed to parse start date: %w", err)
	}
	endDate, err := time.Parse("2006-01-02", "2025-10-06")
	if err != nil {
		return fmt.Errorf("failed to parse end date: %w", err)
	}

	// Get file info
	fileInfo, err := os.Stat(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", path)
	}
	if err != nil {
		return fmt.Errorf("failed to get file info for %s: %w", path, err)
	}

	// Get file creation time (or modification time as a fallback)
	// Note: Go's os.Stat doesn't directly expose creation time.
	// For Windows, it's possible to get it via syscall.
	// For Unix-like systems, os.Stat().ModTime() is typically the modification time.
	// For simplicity and cross-platform compatibility, we'll use ModTime() here
	// as a proxy if creation time isn't readily available, or you might need platform-specific logic.
	fileTime := fileInfo.ModTime()

	// Check if the file's time is within the specified range
	if fileTime.After(startDate) && fileTime.Before(endDate) {
		fmt.Printf("File %s created/modified at %s\n",
			path, fileTime.Format("2006-01-02"))
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("failed to remove file %s: %w", path, err)
		}
		fmt.Printf("File %s removed successfully.\n", path)
	}

	return nil
}

func main() {
	rootDir := "R:\\Managed-Videos\\TV-Shows"

	// Walk the directory recursively
	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Check if the entry is a file and has a .mkv extension
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".ass") {
			if err := processFile(path); err != nil {
				panic(err)
			}
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Error walking the path %q: %v\n", rootDir, err)
	}
}
