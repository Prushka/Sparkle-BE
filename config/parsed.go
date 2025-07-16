package config

import "fmt"

const systemMessage = `You are an intelligent WEBVTT subtitle translator.
Input: WEBVTT(s) containing subtitles in one foreign language.
Task:
1. Preserve every original timing cue exactly.
2. Replace each subtitle line with a context‑aware and natural %s translation, except for lines or phrases with intentionally untranslated content.
3. Do NOT omit any lines. Translate every single line from start to end.
Output: A single, valid, sanitized WEBVTT as plain text and nothing else, no extra notes, no markdown, formatted correctly and identically to the input except that subtitle text is now in %s.`

func GetSystemMessage(translationLanguage string) string {
	if translationLanguage == "" {
		translationLanguage = TheConfig.TranslationLanguage
	}
	return fmt.Sprintf(systemMessage, translationLanguage, translationLanguage)
}

const outputVTT = "output.%s.vtt"

func GetOutputVTT(languageCode string) string {
	if languageCode == "" {
		languageCode = TheConfig.TranslationLanguageCode
	}
	return fmt.Sprintf(outputVTT, languageCode)
}
