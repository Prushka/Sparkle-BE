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
	ConstantQuality        string `env:"CONSTANT_QUALITY" envDefault:"22"`
	VideoExt               string `env:"VIDEO_EXT" envDefault:"mp4"`
	DiscordUserName        string `env:"DISCORD_USER_NAME" envDefault:"Sparkle"`
	DiscordWebhook         string `env:"DISCORD_WEBHOOK" envDefault:""`
	Host                   string `env:"HOST" envDefault:"http://localhost"`
	Encoder                string `env:"ENCODER" envDefault:"av1,hevc,h264-8bit"`
	Av1Encoder             string `env:"SVT_AV1_ENCODER" envDefault:"svt_av1_10bit"`
	Av1Preset              string `env:"AV1_PRESET" envDefault:"6"`
	HevcEncoder            string `env:"HEVC_ENCODER" envDefault:"nvenc_h265_10bit"`
	HevcPreset             string `env:"HEVC_PRESET" envDefault:"slowest"`
	H26410BitEncoder       string `env:"H264_ENCODER" envDefault:"x264_10bit"`
	H26410BitPreset        string `env:"H264_PRESET" envDefault:"slow"`
	H2648BitEncoder        string `env:"H264_ENCODER" envDefault:"x264"`
	H2648BitPreset         string `env:"H264_PRESET" envDefault:"slow"`
	H2648BitProfile        string `env:"H264_PROFILE" envDefault:"baseline"`
	H2648BitTune           string `env:"H264_TUNE" envDefault:"fastdecode"`
	ThumbnailHeight        int    `env:"THUMBNAIL_HEIGHT" envDefault:"320"`
	ThumbnailInterval      int    `env:"THUMBNAIL_INTERVAL" envDefault:"2"`
	ThumbnailChunkInterval int    `env:"THUMBNAIL_CHUNK_INTERVAL" envDefault:"1152"`

	EnableEncode               bool `env:"ENABLE_ENCODE" envDefault:"true"`
	EnableSprite               bool `env:"ENABLE_SPRITE" envDefault:"true"`
	EnableAudioExtraction      bool `env:"ENABLE_AUDIO_EXTRACTION" envDefault:"true"`
	EnableAttachmentExtraction bool `env:"ENABLE_ATTACHMENT_EXTRACTION" envDefault:"true"`
	RemoveOnSuccess            bool `env:"REMOVE_ON_SUCCESS" envDefault:"true"`
}

var TheConfig = &Config{}

func configure() {
	err := env.Parse(TheConfig)
	if err != nil {
		log.Fatalf("error parsing config: %v", err)
	}
}
