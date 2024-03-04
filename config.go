package main

import (
	"github.com/caarlos0/env"
	log "github.com/sirupsen/logrus"
)

const (
	EncodingMode = "encoding"
	RESTMode     = "rest"
)

type Config struct {
	Mode string `env:"MODE" envDefault:"rest"`
}

var TheConfig *Config = &Config{}

func configure() {

	err := env.Parse(TheConfig)

	if err != nil {
		log.Fatalf("error parsing config: %v", err)
	}
}
