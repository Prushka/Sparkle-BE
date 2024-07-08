package main

import (
	"Sparkle/config"
	"Sparkle/discord"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
)

func RandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func copyFile(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer func(source *os.File) {
		err := source.Close()
		if err != nil {
			discord.Errorf("error closing file: %v", err)
		}
	}(source)

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer func(destination *os.File) {
		err := destination.Close()
		if err != nil {
			discord.Errorf("error closing file: %v", err)
		}
	}(destination)
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

func PrintAsJson(v interface{}) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		discord.Errorf(err.Error())
	}
	fmt.Println(string(b))
}

func calculateFileSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			discord.Errorf("error closing file: %v", err)
		}
	}(file)
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	checksum := hex.EncodeToString(hash.Sum(nil))
	return checksum, nil
}

func FormatSecondsToTime(seconds float64) string {
	// HH:MM
	minutes := int(seconds / 60)
	seconds = seconds - float64(minutes*60)
	return fmt.Sprintf("%02d:%02d", minutes, int(seconds))
}

func InputJoin(args ...string) string {
	return filepath.Join(config.TheConfig.Input, filepath.Join(args...))
}

func OutputJoin(args ...string) string {
	return filepath.Join(config.TheConfig.Output, filepath.Join(args...))
}
