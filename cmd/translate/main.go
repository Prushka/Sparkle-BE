package main

import (
	"Sparkle/ai"
	"Sparkle/config"
	"Sparkle/translation"
)

func main() {
	config.Configure()
	ai.Init()
	err := translation.Translate(
		"Junji Ito Collection - S01E01 - Collection No. 068 - Souichi's Convenient Curses + Collection No. 090 - Hellish Doll Funeral Bluray-1080p.mkv",
		"temp/qj04b",
		`R:\\Managed-Videos\\Anime\\Junji Ito Collection\\Season 1\\Junji Ito Collection - S01E01 - Collection No. 068 - Souichi's Convenient Curses + Collection No. 090 - Hellish Doll Funeral Bluray-1080p.mkv`,
		`R:\\Managed-Videos\\Anime\\Junji Ito Collection\\Season 1\\Junji Ito Collection - S01E01 - Collection No. 068 - Souichi's Convenient Curses + Collection No. 090 - Hellish Doll Funeral Bluray-1080p.chi.ass`,
		"SIMPLIFIED Chinese/chi",
		"ass",
		false,
	)
	if err != nil {
		panic(err)
	}
}
