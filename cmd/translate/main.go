package main

import (
	"Sparkle/ai"
	"Sparkle/config"
	"Sparkle/translation"
	"fmt"
)

func main() {
	config.Configure()
	ai.Init()
	media := "Junji Ito Collection - S01E04 - Collection No. 034 - Shiver + Collection No. 060 - House of Puppets Bluray-1080p"
	err := translation.Translate(
		fmt.Sprintf("%s.mkv", media),
		"qQLjp",
		fmt.Sprintf(`R:\\Managed-Videos\\Anime\\Junji Ito Collection\\Season 1\\%s.mkv`, media),
		fmt.Sprintf(`R:\\Managed-Videos\\Anime\\Junji Ito Collection\\Season 1\\%s.chi.ass`, media),
		"SIMPLIFIED Chinese/chi",
		"ass",
		false,
	)
	if err != nil {
		panic(err)
	}
}

// gemma3:27b (3000 tokens)

// llama3.3:70b
// qwen3:235b
// qwen3:30b
// llama4:16x17b
// deepseek-r1:70b
// gpt-oss:20b
// gpt-oss:120b
