package service

import (
	"context"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

type EmbeddingService interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

type openAIEmbeddingService struct {
	client *openai.Client
	model  string
}

func NewEmbeddingService(baseURL, apiKey, model string) EmbeddingService {
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = strings.TrimRight(baseURL, "/") + "/v1"
	return &openAIEmbeddingService{
		client: openai.NewClientWithConfig(cfg),
		model:  model,
	}
}

func (s *openAIEmbeddingService) Embed(ctx context.Context, text string) ([]float32, error) {
	resp, err := s.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: []string{text},
		Model: openai.EmbeddingModel(s.model),
	})
	if err != nil {
		return nil, fmt.Errorf("embedding API: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("embedding API retornou resposta vazia")
	}
	return resp.Data[0].Embedding, nil
}

type noopEmbeddingService struct{}

func NewNoopEmbeddingService() EmbeddingService { return &noopEmbeddingService{} }

func (n *noopEmbeddingService) Embed(_ context.Context, _ string) ([]float32, error) {
	return nil, fmt.Errorf("serviço de embeddings não configurado")
}
