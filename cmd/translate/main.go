package main

import (
	"Sparkle/ai"
	"Sparkle/config"
	"Sparkle/translation"
	"fmt"
	"strings"
)

func main() {
	config.Configure()
	ai.Init()
	media := "Junji Ito Collection - S01E04 - Collection No. 034 - Shiver + Collection No. 060 - House of Puppets Bluray-1080p"
	model := strings.ReplaceAll(config.TheConfig.OpenAIModel, ":", "-")
	err := translation.Translate(
		fmt.Sprintf("%s.mkv", media),
		"qQLjp",
		fmt.Sprintf(`R:\\Managed-Videos\\Anime\\Junji Ito Collection\\Season 1\\%s.mkv`, media),
		fmt.Sprintf(`qQLjp\\%s.%s.ass`, model, "test-sp"),
		"Spanish/spa",
		"ass",
		false,
	)
	if err != nil {
		panic(err)
	}
}

// gemma3:27b (1200 tokens, 2 history count)
// qwen3:235b (most natural, slow, thinking tokens)

// deepseek-r1:70b (contains comments)
// qwen3:30b (less natural, thinking tokens)
// gpt-oss:20b
// gpt-oss:120b

// llama3.3:70b (slow, less natural)
// llama4:16x17b (slow, contains comments)
