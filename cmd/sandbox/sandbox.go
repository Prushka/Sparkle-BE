package main

import (
	"Sparkle/config"
	"Sparkle/discord"
)

func main() {
	config.Configure()
	discord.Infof("%d", len(config.TheConfig.TranslationLanguages))
}
