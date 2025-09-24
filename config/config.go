package config

import (
	"strings"
	"time"

	"github.com/caarlos0/env"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	Debug bool `env:"DEBUG" envDefault:"false"`

	MatchEverything bool `env:"MATCH_EVERYTHING" envDefault:"false"`
	ReverseOrder    bool `env:"REVERSE_ORDER" envDefault:"false"`

	Output                 string `env:"OUTPUT" envDefault:"./output"`
	Input                  string `env:"INPUT" envDefault:"./input"`
	Ffmpeg                 string `env:"FFMPEG" envDefault:"ffmpeg"`
	Ffprobe                string `env:"FFPROBE" envDefault:"ffprobe"`
	MKVExtract             string `env:"MKVEXTRACT" envDefault:"mkvextract"`
	HandbrakeCli           string `env:"HANDBRAKE_CLI" envDefault:"HandBrakeCLI"`
	ConstantQuality        string `env:"CONSTANT_QUALITY" envDefault:"18"`
	VideoExt               string `env:"VIDEO_EXT" envDefault:"mp4"`
	Host                   string `env:"HOST" envDefault:"http://localhost"`
	Encoder                string `env:"ENCODER" envDefault:"av1"`
	AudioKbps              int    `env:"AUDIO_KBPS" envDefault:"144"`
	Av1Encoder             string `env:"SVT_AV1_ENCODER" envDefault:"svt_av1_10bit"` // svt_av1_10bit,nvenc_av1_10bit
	Av1Preset              string `env:"AV1_PRESET" envDefault:"4"`                  // 4,slowest
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

	EnableEncode      bool `env:"ENABLE_ENCODE" envDefault:"true"`
	EnableSprite      bool `env:"ENABLE_SPRITE" envDefault:"true"`
	EnableLowPriority bool `env:"ENABLE_LOW_PRIORITY" envDefault:"true"`
	EnableCleanup     bool `env:"ENABLE_CLEANUP" envDefault:"true"`

	DiscordName         string   `env:"DISCORD_NAME" envDefault:"Encoding"`
	DiscordWebhookError string   `env:"DISCORD_WEBHOOK_ERROR" envDefault:""`
	DiscordWebhookInfo  string   `env:"DISCORD_WEBHOOK_INFO" envDefault:""`
	DiscordWebhookChat  string   `env:"DISCORD_WEBHOOK_CHAT" envDefault:""`
	EncodeListFile      string   `env:"ENCODE_LIST_FILE" envDefault:"encode_list.json"`
	ShowDirs            []string `env:"SHOW_DIR" envDefault:""`
	MovieDirs           []string `env:"MOVIE_DIR" envDefault:""`

	ScanConfigInterval time.Duration `env:"SCAN_CONFIG_INTERVAL" envDefault:"2h"`
	ScanInputInterval  time.Duration `env:"SCAN_INPUT_INTERVAL" envDefault:"4h"`

	PurgeCacheUrl            string   `env:"PURGE_CACHE_URL" envDefault:""`
	NoHistory                bool     `env:"NO_HISTORY" envDefault:"false"`
	OpenAI                   []string `env:"OPENAI" envDefault:""`
	Gemini                   []string `env:"GEMINI" envDefault:""`
	AiProvider               string   `env:"AI_PROVIDER" envDefault:"gemini"` // gemini or openai
	OpenAIModel              string   `env:"OPENAI_MODEL" envDefault:"gpt-5"`
	OpenAIUrl                string   `env:"OPENAI_URL" envDefault:""`
	GeminiModel              string   `env:"GEMINI_MODEL" envDefault:"gemini-2.5-pro"`
	TranslationLanguages     []string `env:"TRANSLATION_LANGUAGES" envDefault:"SIMPLIFIED Chinese/chi,Turkish/tur"` // Turkish/tur,Spanish/spa
	TranslationSubtitleTypes []string `env:"TRANSLATION_SUBTITLE_TYPES" envDefault:"ass"`
	TranslationBatchLength   int      `env:"TRANSLATION_BATCH_LENGTH" envDefault:"36000"`
	TranslationAttempts      int      `env:"TRANSLATION_ATTEMPTS" envDefault:"3"`
	TranslationInputLanguage []string `env:"TRANSLATION_INPUT_LANGUAGE" envDefault:"eng"`

	SleepAfterExhausted time.Duration `env:"SLEEP_ATTEMPTS" envDefault:"2h"`
	DelayBeforeNextSend time.Duration `env:"DELAY_BEFORE_NEXT_SEND" envDefault:"2m"` // one request every 2 minutes

	OverSeerrURL     string `env:"OVERSEERR_URL" envDefault:"http://localhost"`
	OverSeerrAPI     string `env:"OVERSEERR_API" envDefault:""`
	OverSeerrUserIds []int  `env:"OVERSEERR_USER_IDS" envDefault:""`

	OCRVLMUrl    string   `env:"OCRVLM_URL" envDefault:"http://192.168.1.244:11434/v1"` // ollama or openai compatible
	OCRVLMModels []string `env:"OCRVLM_MODEL" envDefault:"gemma3:27b,mistral-small3.2:24b,llama4:16x17b"`
	OCRVLMVotes  int      `env:"OCRVLM_VOTES" envDefault:"2"`
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
	if TheConfig.Debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug mode enabled")
	}
}
