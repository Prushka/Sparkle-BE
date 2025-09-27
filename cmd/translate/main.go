package main

import (
	"Sparkle/ai"
	"Sparkle/config"
	"Sparkle/discord"
	"Sparkle/translation"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
)

func main() {
	config.Configure()
	ai.Init()

	bench("Spanish", "spa", "sp", "gemma3:27b")
	bench("Spanish", "spa", "sp", "qwen3:30b")
	bench("Spanish", "spa", "sp", "deepseek-r1:70b")
	bench("Spanish", "spa", "sp", "gpt-oss:20b")
	bench("Spanish", "spa", "sp", "gpt-oss:120b")
	bench("Spanish", "spa", "sp", "llama3.3:70b")
	bench("Spanish", "spa", "sp", "llama4:16x17b")

	bench("Turkish", "tur", "tr", "gemma3:27b")
	bench("Turkish", "tur", "tr", "qwen3:30b")
	bench("Turkish", "tur", "tr", "deepseek-r1:70b")
	bench("Turkish", "tur", "tr", "gpt-oss:20b")
	bench("Turkish", "tur", "tr", "gpt-oss:120b")
	bench("Turkish", "tur", "tr", "llama3.3:70b")
	bench("Turkish", "tur", "tr", "llama4:16x17b")

	bench("SIMPLIFIED Chinese", "chi", "ci", "gemma3:27b")
	bench("SIMPLIFIED Chinese", "chi", "ci", "qwen3:30b")
	bench("SIMPLIFIED Chinese", "chi", "ci", "deepseek-r1:70b")
	bench("SIMPLIFIED Chinese", "chi", "ci", "gpt-oss:20b")
	bench("SIMPLIFIED Chinese", "chi", "ci", "gpt-oss:120b")
	bench("SIMPLIFIED Chinese", "chi", "ci", "llama3.3:70b")
	bench("SIMPLIFIED Chinese", "chi", "ci", "llama4:16x17b")

	bench("Russian", "rus", "rs", "gemma3:27b")
	bench("Russian", "rus", "rs", "qwen3:30b")
	bench("Russian", "rus", "rs", "deepseek-r1:70b")
	bench("Russian", "rus", "rs", "gpt-oss:20b")
	bench("Russian", "rus", "rs", "gpt-oss:120b")
	bench("Russian", "rus", "rs", "llama3.3:70b")
	bench("Russian", "rus", "rs", "llama4:16x17b")
}

func bench(language, languageCode, id, m string) {
	discord.Infof("%s, %s", language, languageCode)
	media := "Junji Ito Collection - S01E04 - Collection No. 034 - Shiver + Collection No. 060 - House of Puppets Bluray-1080p"
	config.TheConfig.OpenAIModel = m
	model := strings.ReplaceAll(config.TheConfig.OpenAIModel, ":", "-")
	_, err := translation.Translate(
		fmt.Sprintf("%s.mkv", media),
		"qQLjp",
		fmt.Sprintf(`R:\\Managed-Videos\\Anime\\Junji Ito Collection\\Season 1\\%s.mkv`, media),
		fmt.Sprintf(`qQLjp\\%s.%s.ass`, model, fmt.Sprintf("test-%s-{attempts}-{duration}", id)),
		fmt.Sprintf("%s/%s", language, languageCode),
		false,
	)
	if err != nil {
		log.Errorf("error: %v", err)
	}
}

// gemma3:27b (1200 tokens, 2 history count) (turkish missing lines)
// qwen3:235b (most natural, slow, thinking tokens)

// qwen3:30b (less natural, thinking tokens)
// gpt-oss:20b
// gpt-oss:120b
// llama3.3:70b (slow, less natural)

// llama4:16x17b (slow, contains comments)
// deepseek-r1:70b (contains comments)
