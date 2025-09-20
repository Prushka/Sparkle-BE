package main

import (
	"Sparkle/config"
	"Sparkle/sup"
)

func main() {
	config.Configure()
	err := sup.Convert("0-eng.sup", config.TheConfig.OCRVLMModel+".srt")
	if err != nil {
		panic(err)
	}
}
