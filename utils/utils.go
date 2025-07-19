package utils

import (
	"Sparkle/config"
	"Sparkle/discord"
	"Sparkle/priority"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// WebvttTimeRangeRegex Matches lines like "00:00:01.000 --> 00:00:05.000", "00:01.000 --> 00:05.000"
var WebvttTimeRangeRegex = regexp.MustCompile(`^((?:\d{1,2}:){0,2}\d{1,2}\.\d{1,3})(\s*-->\s*)((?:\d{1,2}:){0,2}\d{1,2}\.\d{1,3})$`)

func IsWebVTTTimeRangeLine(input string) bool {
	return WebvttTimeRangeRegex.MatchString(input)
}

func CountVTTTimeLines(input string) int {
	lines := strings.Split(input, "\n")
	count := 0
	for _, s := range lines {
		if IsWebVTTTimeRangeLine(s) {
			count++
		}
	}
	return count
}

func PanicOnSec(a interface{}, err error) {
	if err != nil {
		panic(err)
	}
}

func GetTitleId(title string) string {
	parts := strings.Split(title, " - ")
	se := ""

	for i, part := range parts {
		matched, _ := regexp.MatchString(`S\d{2}E\d{2}`, part)
		if matched {
			se = part
			// seTitle = strings.Join(parts[i+1:], " - ")
			title = strings.Join(parts[:i], " - ")
			break
		}
	}

	titleId := regexp.MustCompile(`[^a-z0-9]`).ReplaceAllString(strings.ToLower(title), "")
	return titleId + se
}

func Run(c *exec.Cmd) error {
	if err := c.Start(); err != nil {
		return err
	}
	if config.TheConfig.EnableLowPriority {
		err := priority.LowPriority(c.Process.Pid)
		if err != nil {
			discord.Errorf("error setting priority: %v", err)
		}
	}
	return c.Wait()
}

func CombinedOutput(c *exec.Cmd) ([]byte, error) {
	if c.Stdout != nil {
		return nil, fmt.Errorf("exec: Stdout already set")
	}
	if c.Stderr != nil {
		return nil, fmt.Errorf("exec: Stderr already set")
	}
	var b bytes.Buffer
	c.Stdout = &b
	c.Stderr = &b
	err := Run(c)
	return b.Bytes(), err
}

func RunCommand(cmd *exec.Cmd) ([]byte, error) {
	out, err := CombinedOutput(cmd)
	if err != nil {
		discord.Errorf(cmd.String())
		fmt.Println(string(out))
		return out, err
	} else {
		log.Debugf("output: %s", out)
	}
	return out, err
}

func RandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func CopyFile(src, dst string) (int64, error) {
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

func AsJson(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		discord.Errorf(err.Error())
	}
	return string(b)
}

func CalculateFileSHA256(filePath string) (string, error) {
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
