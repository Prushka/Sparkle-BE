package main

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"math/rand"
)

func RandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func PrintAsJson(v interface{}) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Error(err)
	}
	fmt.Println(string(b))
}
