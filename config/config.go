package config

import (
	"github.com/caarlos0/env"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

type Config struct {
	Output                 string `env:"OUTPUT" envDefault:"./output"`
	Input                  string `env:"INPUT" envDefault:"./input"`
	Ffmpeg                 string `env:"FFMPEG" envDefault:"ffmpeg"`
	Ffprobe                string `env:"FFPROBE" envDefault:"ffprobe"`
	HandbrakeCli           string `env:"HANDBRAKE_CLI" envDefault:"./HandBrakeCLI"`
	ConstantQuality        string `env:"CONSTANT_QUALITY" envDefault:"21"`
	VideoExt               string `env:"VIDEO_EXT" envDefault:"mp4"`
	Host                   string `env:"HOST" envDefault:"http://localhost"`
	Encoder                string `env:"ENCODER" envDefault:"av1"`
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
	EnableLowPriority          bool `env:"ENABLE_LOW_PRIORITY" envDefault:"true"`
	EnableCleanup              bool `env:"ENABLE_CLEANUP" envDefault:"true"`

	DiscordName         string   `env:"DISCORD_NAME" envDefault:"Encoding"`
	DiscordWebhookError string   `env:"DISCORD_WEBHOOK_ERROR" envDefault:""`
	DiscordWebhookInfo  string   `env:"DISCORD_WEBHOOK_INFO" envDefault:""`
	DiscordWebhookChat  string   `env:"DISCORD_WEBHOOK_CHAT" envDefault:""`
	EncodeListFile      string   `env:"ENCODE_LIST_FILE" envDefault:"encode_list.json"`
	ShowDirs            []string `env:"SHOW_DIR" envDefault:""`
	MovieDirs           []string `env:"MOVIE_DIR" envDefault:""`

	ScanConfigInterval time.Duration `env:"SCAN_CONFIG_INTERVAL" envDefault:"10m"`
	ScanInputInterval  time.Duration `env:"SCAN_INPUT_INTERVAL" envDefault:"2h"`

	PurgeCacheUrl            string   `env:"PURGE_CACHE_URL" envDefault:""`
	OpenAI                   string   `env:"OPENAI" envDefault:""`
	Gemini                   string   `env:"GEMINI" envDefault:""`
	AiProvider               string   `env:"AI_PROVIDER" envDefault:"gemini"`
	OpenAIModel              string   `env:"OPENAI_MODEL" envDefault:"o4-mini"`
	GeminiModel              string   `env:"GEMINI_MODEL" envDefault:"gemini-2.5-pro"`
	TranslationLanguages     []string `env:"TRANSLATION_LANGUAGES" envDefault:"SIMPLIFIED Chinese;chi,Turkish;tur"` // Turkish;tur,Spanish;spa
	KeepTranslationAttempt   bool     `env:"KEEP_TRANSLATION_ATTEMPT" envDefault:"true"`
	TranslationOutputCutoff  float64  `env:"TRANSLATION_OUTPUT_CUTOFF" envDefault:"0.98"`
	TranslationSubtitleTypes []string `env:"TRANSLATION_SUBTITLE_TYPES" envDefault:"ass"`
	TranslationBatchLength   int      `env:"TRANSLATION_BATCH_LENGTH" envDefault:"36000"`
	TranslationAttempts      int      `env:"TRANSLATION_ATTEMPTS" envDefault:"3"`
	TranslationInputLanguage []string `env:"TRANSLATION_INPUT_LANGUAGE" envDefault:"jpn,eng"`
}

var TheConfig = &Config{}

var gitHash, gitVersion string

func Configure() {
	err := env.Parse(TheConfig)
	if err != nil {
		log.Fatalf("error parsing config: %v", err)
	}

	for i, t := range TheConfig.TranslationSubtitleTypes {
		TheConfig.TranslationSubtitleTypes[i] = strings.ToLower(t)
	}
	log.Infof("Running: %s, %s", gitVersion, gitHash)
}
