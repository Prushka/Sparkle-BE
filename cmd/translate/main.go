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

	aisToTry := []string{
		//"gemma3:27b", // unstable line count output
		//"qwen3:30b",  // unstable line count output
		//"gpt-oss:20b", // works, slightly more attempts than 120b
		"gpt-oss:120b",
		"llama3.3:70b",
		//"llama4:16x17b", // unstable line count output
		//"deepseek-r1:70b", // way too slow
	}

	languagesToTry := [][3]string{
		{"SIMPLIFIED Chinese", "chi", "ci"},
		{"Spanish", "spa", "sp"},
		{"Turkish", "tur", "tr"},
		{"Russian", "rus", "rs"},
	}

	for _, aiModel := range aisToTry {
		for _, lang := range languagesToTry {
			bench(lang[0], lang[1], lang[2], aiModel)
		}
	}
}

func bench(language, languageCode, id, m string) {
	discord.Infof("%s, %s", language, languageCode)
	media := "Junji Ito Collection - S01E04 - Collection No. 034 - Shiver + Collection No. 060 - House of Puppets Bluray-1080p"
	config.TheConfig.AIModel = m
	model := strings.ReplaceAll(config.TheConfig.AIModel, ":", "-")
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
