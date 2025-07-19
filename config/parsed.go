package config

import "fmt"

const systemMessageWEBVTT = `You are an intelligent WEBVTT subtitle translator.
Input: WEBVTT containing subtitles in one foreign language.
Task:
1. Preserve every original timing cue exactly.
2. Replace each subtitle line with a context‑aware and natural %s translation, except for lines or phrases with intentionally untranslated content.
3. Do NOT omit any lines. Translate every single line from start to end.
Output: A single, valid, sanitized WEBVTT as plain text and nothing else, no extra notes, no markdown, formatted correctly and identically to the input except that subtitle text is now in %s.`

const systemMessageASS = `You are an intelligent .ass (SubStation Alpha) subtitle translator.
Input: .ass containing subtitles in one foreign language.
Task:
1. Reproduce every non‑dialogue element—timing cues, style definitions, headers, and all other formatting—exactly as it appears in the original file.
2. Replace each dialogue line text with a fluent, context‑aware %s translation, except for segments that are intentionally left untranslated.
3. Do NOT omit any lines. Translate every single dialogue line from start to end.
4. If the input is only a fragment of an .ass file, translate ONLY that fragment; do not add missing headers, footers, or any other content.
Output: A single, valid .ass as plain text—no markdown, notes, or comments—identical in structure to the input, with dialogue text now in %s.`

const (
	WEBVTT = iota // 0
	ASS           // 1
)

func GetSystemMessage(translationLanguage string, whichOne int) string {
	var msg string
	if whichOne == ASS {
		msg = systemMessageASS
	} else if whichOne == WEBVTT {
		msg = systemMessageWEBVTT
	} else {
		panic(fmt.Errorf("unknown subtitle type: %d", whichOne))
	}
	return fmt.Sprintf(msg, translationLanguage, translationLanguage)
}
