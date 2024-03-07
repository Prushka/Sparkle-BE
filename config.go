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
	Mode          string `env:"MODE" envDefault:"rest"`
	Redis         string `env:"REDIS" envDefault:"localhost:6379"`
	RedisPassword string `env:"REDIS_PASSWORD" envDefault:""`
	Output        string `env:"OUTPUT" envDefault:"./output"`
	Input         string `env:"INPUT" envDefault:"./input"`
	Ffmpeg        string `env:"FFMPEG" envDefault:"ffmpeg"`
	Ffprobe       string `env:"FFPROBE" envDefault:"ffprobe"`
	HandbrakeCli  string `env:"HANDBRAKE_CLI" envDefault:"./HandBrakeCLI"`
}

var TheConfig = &Config{}

func configure() {

	err := env.Parse(TheConfig)

	if err != nil {
		log.Fatalf("error parsing config: %v", err)
	}
}
