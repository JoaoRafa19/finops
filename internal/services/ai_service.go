package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

type AIService interface {
	// RunAgentLoop executa um loop agentico com as ferramentas fornecidas e retorna a resposta final em texto.
	RunAgentLoop(ctx context.Context, system, user string, tools []financialTool, maxIter int) (string, error)
	Generate(ctx context.Context, prompt string) (string, error)
}

// OpenAICompatibleAIService usa qualquer API com formato OpenAI (Groq, OpenAI, Ollama /v1, etc.)
type OpenAICompatibleAIService struct {
	client *openai.Client
	model  string
}

func NewAIService(baseURL, apiKey, model string) AIService {
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = strings.TrimRight(baseURL, "/") + "/v1"
	return &OpenAICompatibleAIService{
		client: openai.NewClientWithConfig(cfg),
		model:  model,
	}
}

func (s *OpenAICompatibleAIService) Generate(ctx context.Context, prompt string) (string, error) {
	ctx2, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	resp, err := s.client.CreateChatCompletion(ctx2, openai.ChatCompletionRequest{
		Model: s.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		MaxTokens: 1024,
	})
	if err != nil {
		return "", fmt.Errorf("LLM indisponível: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("LLM retornou resposta vazia")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

func (s *OpenAICompatibleAIService) RunAgentLoop(ctx context.Context, system, user string, tools []financialTool, maxIter int) (string, error) {
	ctx2, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	openaiTools := make([]openai.Tool, len(tools))
	for i, t := range tools {
		openaiTools[i] = t.schema
	}

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: system},
		{Role: openai.ChatMessageRoleUser, Content: user},
	}

	for i := 0; i < maxIter; i++ {
		resp, err := s.client.CreateChatCompletion(ctx2, openai.ChatCompletionRequest{
			Model:    s.model,
			Messages: messages,
			Tools:    openaiTools,
		})
		if err != nil {
			return "", fmt.Errorf("LLM indisponível: %w", err)
		}
		if len(resp.Choices) == 0 {
			break
		}

		msg := resp.Choices[0].Message
		messages = append(messages, msg)

		if len(msg.ToolCalls) == 0 {
			return strings.TrimSpace(msg.Content), nil
		}

		for _, tc := range msg.ToolCalls {
			result := executeTool(ctx2, tc.Function.Name, json.RawMessage(tc.Function.Arguments), tools)
			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	return "", fmt.Errorf("loop de agente não convergiu em %d iterações", maxIter)
}

// NoopAIService retorna "Sem categoria" quando IA não está configurada.
type NoopAIService struct{}

func (n *NoopAIService) RunAgentLoop(_ context.Context, _, _ string, _ []financialTool, _ int) (string, error) {
	return "Sem categoria", nil
}

func (n *NoopAIService) Generate(_ context.Context, _ string) (string, error) {
	return "Serviço de IA não configurado.", nil
}

func parseBulkResponse(raw string) map[int64]string {
	result := make(map[int64]string)
	jsonStr := raw
	if start := strings.Index(raw, "["); start != -1 {
		if end := strings.LastIndex(raw, "]"); end > start {
			jsonStr = raw[start : end+1]
		}
	}
	var items []struct {
		ID       int64  `json:"id"`
		Category string `json:"category"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &items); err != nil {
		return result
	}
	for _, item := range items {
		if item.ID > 0 && item.Category != "" {
			result[item.ID] = strings.TrimSpace(item.Category)
		}
	}
	return result
}
