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
	Opusenc       string `env:"OPUSENC" envDefault:"opusenc"`
	HandbrakeCli  string `env:"HANDBRAKE_CLI" envDefault:"./HandBrakeCLI"`
	Av1Preset     string `env:"AV1_PRESET" envDefault:"6"`
	Av1Quality    string `env:"AV1_QUALITY" envDefault:"21"`
	SubtitleExt   string `env:"SUBTITLE_EXT" envDefault:".vtt"`
	VideoExt      string `env:"VIDEO_EXT" envDefault:".mp4"`
}

var TheConfig = &Config{}

func configure() {

	err := env.Parse(TheConfig)

	if err != nil {
		log.Fatalf("error parsing config: %v", err)
	}
}
