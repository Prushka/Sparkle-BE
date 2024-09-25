package main

import (
	"bytes"
	"crypto/md5"
	"crypto/sha512"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"path/filepath"
)

var totalDeleted int
var totalDeletedSize int64

type DupFile struct {
	Path   string
	Size   int64
	MD5    string
	SHA512 string
}

func dedup(root string) {
	duplicates, err := findDuplicates(root)
	if err != nil {
		log.Errorf("Error finding duplicates: %v\n", err)
		os.Exit(1)
	}

	for key, files := range duplicates {
		if len(files) > 1 {
			log.Infof("Duplicate files: %s", key)
			for _, file := range files {
				log.Info(file)
			}
			if err := DeleteDuplicateFiles(files); err != nil {
				log.Errorf("Error deleting duplicate files: %v", err)
			}
		}
		log.Info()
	}
	log.Infof("Total deleted: %d files, %d GB", totalDeleted, totalDeletedSize>>30)
}

func DeleteDuplicateFiles(files []DupFile) error {
	if len(files) <= 1 {
		return nil
	}

	firstHash, err := computeSHA512(files[0].Path)
	if err != nil {
		return err
	}
	for _, dupFile := range files[1:] {
		hash, err := computeSHA512(dupFile.Path)
		if err != nil {
			return err
		}
		if !bytes.Equal(hash, firstHash) {
			log.Errorf("Files %s and %s have different content", files[0], dupFile)
			return nil
		}
	}
	for _, dupFile := range files[1:] {
		//if err := os.Remove(dupFile.Path); err != nil {
		//	return err
		//}
		log.Infof("Deleted: %s", dupFile.Path)
		totalDeleted++
		totalDeletedSize += dupFile.Size
	}

	return nil
}

func findDuplicates(root string) (map[string][]DupFile, error) {
	sizeMap := make(map[int64][]string)

	// First pass: Walk the directory tree, group files by size
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Can't access the path, skip it
			return nil
		}
		if !info.Mode().IsRegular() {
			// Not a regular file, skip it
			return nil
		}

		size := info.Size()
		sizeMap[size] = append(sizeMap[size], path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Second pass: For files with the same size, compute MD5 hash
	hashMap := make(map[string][]DupFile) // map from hash to list of file paths

	for size, files := range sizeMap {
		if len(files) < 2 {
			// Only one file of this size, cannot be duplicate
			continue
		}

		// For files of the same size, compute their MD5 hash
		for _, file := range files {
			hash, err := computeMD5(file)
			if err != nil {
				log.Errorf("Error computing MD5 for file %s: %v", file, err)
				continue
			}
			hashMap[hash] = append(hashMap[hash], DupFile{Path: file, Size: size, MD5: hash})
		}
	}

	return hashMap, nil
}

func computeMD5(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log.Errorf("Error closing file %s: %v", f.Name(), err)
		}
	}(f)

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func computeSHA512(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log.Errorf("Error closing file %s: %v", f.Name(), err)
		}
	}(f)

	h := sha512.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}
