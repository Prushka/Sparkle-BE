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
	Mode                   string `env:"MODE" envDefault:"rest"`
	Redis                  string `env:"REDIS" envDefault:"localhost:6379"`
	RedisPassword          string `env:"REDIS_PASSWORD" envDefault:""`
	Output                 string `env:"OUTPUT" envDefault:"./output"`
	Input                  string `env:"INPUT" envDefault:"./input"`
	Ffmpeg                 string `env:"FFMPEG" envDefault:"ffmpeg"`
	Ffprobe                string `env:"FFPROBE" envDefault:"ffprobe"`
	Opusenc                string `env:"OPUSENC" envDefault:"opusenc"`
	HandbrakeCli           string `env:"HANDBRAKE_CLI" envDefault:"HandBrakeCLI"`
	Av1Preset              string `env:"AV1_PRESET" envDefault:"6"`
	NvencPreset            string `env:"NVENC_PRESET" envDefault:"slower"`
	ConstantQuality        string `env:"CONSTANT_QUALITY" envDefault:"22"`
	SubtitleExt            string `env:"SUBTITLE_EXT" envDefault:".vtt"`
	VideoExt               string `env:"VIDEO_EXT" envDefault:"mp4"`
	SkipAudioExtraction    bool   `env:"SKIP_AUDIO_EXTRACTION" envDefault:"true"`
	DiscordUserName        string `env:"DISCORD_USER_NAME" envDefault:"Sparkle"`
	DiscordWebhook         string `env:"DISCORD_WEBHOOK" envDefault:""`
	Host                   string `env:"HOST" envDefault:"http://localhost"`
	Encoder                string `env:"ENCODER" envDefault:"av1,hevc"`
	Av1Encoder             string `env:"SVT_AV1_ENCODER" envDefault:"svt_av1_10bit"`
	HevcEncoder            string `env:"HEVC_ENCODER" envDefault:"nvenc_h265_10bit"`
	ThumbnailHeight        int    `env:"THUMBNAIL_HEIGHT" envDefault:"360"`
	ThumbnailInterval      int    `env:"THUMBNAIL_INTERVAL" envDefault:"2"`
	ThumbnailChunkInterval int    `env:"THUMBNAIL_CHUNK_INTERVAL" envDefault:"1152"`
}

var TheConfig = &Config{}

func configure() {
	err := env.Parse(TheConfig)
	if err != nil {
		log.Fatalf("error parsing config: %v", err)
	}
}
