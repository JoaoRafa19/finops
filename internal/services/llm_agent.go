package service

import "context"

type LLMAgent interface {
	Query(ctx context.Context, user int64, question string) (string, error)
}
