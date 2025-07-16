package ai

import (
	"context"
)

type Translator interface {
	StartChat(ctx context.Context, systemInstruction string) error
	Send(ctx context.Context, input string) (Result, error)
	SendWithRetry(ctx context.Context, input string, pass func(result Result) bool, attempts int) (Result, error)
}

type Result interface {
	Usage() interface{}
	Text() string
}
