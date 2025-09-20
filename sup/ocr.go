package sup

import (
	"Sparkle/config"
	"Sparkle/discord"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	log "github.com/sirupsen/logrus"
)

const (
	systemPrompt = `You are a precision OCR subtitle extractor.
Input: A single (PGS) subtitle image.
Task:
1.  Perform OCR on the input image to identify all text.
2.  Transcribe the text with 100% accuracy, matching the source exactly WITHOUT modification.
3.  Preserve the original structure, preserve all line breaks.
4.  Wrap any italic text in <i>...</i> tags.
Output: A plain text transcription of the subtitle. There should be no markdown, no comments, and no content other than the text extracted from the image.`

	temperature = 0.1
)

type ImageSubtitle struct {
	Image     image.Image
	StartTime time.Duration
	EndTime   time.Duration
}

func OCR(imgSubs []ImageSubtitle) (SRTSubtitles, error) {
	var (
		totalPromptTokens     int64
		totalCompletionTokens int64
	)
	txtSubs := make(SRTSubtitles, len(imgSubs))
	defer func() {
		discord.Infof("%s model tokens used: prompt=%d, completion=%d", config.TheConfig.OCRVLMModel, totalPromptTokens, totalCompletionTokens)
	}()
	for index, pg := range imgSubs {
		text, promptTokens, completionTokens, err := ExtractText(pg.Image)
		if err != nil {
			return nil, fmt.Errorf("failed to extract text from image #%d: %s", index+1, err)
		}
		totalPromptTokens += promptTokens
		totalCompletionTokens += completionTokens

		log.Debugf("#%d %s --> %s %s", index+1, pg.StartTime, pg.EndTime, text)
		txtSubs[index] = SRTSubtitle{
			Start: SRTTimestamp(pg.StartTime),
			End:   SRTTimestamp(pg.EndTime),
			Text:  text,
		}
	}
	return txtSubs, nil
}

func ExtractText(img image.Image) (text string, promptTokens, completionTokens int64, err error) {
	encodedImage, err := encodeImageToDataURL(img)
	if err != nil {
		err = fmt.Errorf("failed to encode image: %w", err)
		return
	}
	chatCompletion, err := oaiClient.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
		Model: config.TheConfig.OCRVLMModel,
		Temperature: param.Opt[float64]{
			Value: temperature,
		},
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			{
				OfUser: &openai.ChatCompletionUserMessageParam{
					Content: openai.ChatCompletionUserMessageParamContentUnion{
						OfArrayOfContentParts: []openai.ChatCompletionContentPartUnionParam{
							{
								OfImageURL: &openai.ChatCompletionContentPartImageParam{
									ImageURL: openai.ChatCompletionContentPartImageImageURLParam{
										URL: encodedImage,
									},
								},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		err = fmt.Errorf("failed to get OCR chat completion: %w", err)
		return
	}
	if chatCompletion == nil || len(chatCompletion.Choices) == 0 {
		err = fmt.Errorf("no choices returned from OCR chat completion")
		return
	}
	text = chatCompletion.Choices[0].Message.Content
	promptTokens = chatCompletion.Usage.PromptTokens
	completionTokens = chatCompletion.Usage.CompletionTokens
	return
}

func encodeImageToDataURL(image image.Image) (string, error) {
	var data bytes.Buffer
	err := png.Encode(&data, image)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(data.Bytes())), nil
}
