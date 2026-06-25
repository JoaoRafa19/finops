package service

import (
	"context"
	"encoding/json"
	"fmt"
"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

type ClassificationExample struct {
	Description string
	Category    string
}

type BulkClassifyInput struct {
	ID          int64
	Description string
	Direction   string
}

type AIService interface {
	SuggestCategory(ctx context.Context, description, direction string, categories []string, examples []ClassificationExample) (string, error)
	SuggestCategoryBulk(ctx context.Context, txs []BulkClassifyInput, categories []string, examples []ClassificationExample) (map[int64]string, error)
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

func (s *OpenAICompatibleAIService) chat(ctx context.Context, systemPrompt, userMsg string, maxTokens int) (string, error) {
	ctx2, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	resp, err := s.client.CreateChatCompletion(ctx2, openai.ChatCompletionRequest{
		Model: s.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userMsg},
		},
		MaxTokens: maxTokens,
	})
	if err != nil {
		return "", fmt.Errorf("LLM indisponível: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("LLM retornou resposta vazia")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

func (s *OpenAICompatibleAIService) Generate(ctx context.Context, prompt string) (string, error) {
	return s.chat(ctx, "", prompt, 1024)
}

func (s *OpenAICompatibleAIService) SuggestCategory(ctx context.Context, description, direction string, categories []string, examples []ClassificationExample) (string, error) {
	dirLabel := "despesa"
	if direction == "credit" {
		dirLabel = "receita"
	}

	var examplesBlock string
	if len(examples) > 0 {
		var sb strings.Builder
		sb.WriteString("Histórico de classificações deste usuário:\n")
		for _, ex := range examples {
			fmt.Fprintf(&sb, "- \"%s\" → %s\n", ex.Description, ex.Category)
		}
		examplesBlock = sb.String() + "\n"
	}

	system := "Você é um assistente financeiro que classifica transações em categorias. Responda APENAS com o nome exato de uma das categorias, sem explicações."
	user := fmt.Sprintf(`%sClassifique a transação abaixo:
Descrição: %s
Tipo: %s
Categorias disponíveis: %s

Se nenhuma categoria for adequada, responda: Sem categoria`, examplesBlock, description, dirLabel, strings.Join(categories, ", "))

	raw, err := s.chat(ctx, system, user, 50)
	if err != nil {
		return "", err
	}
	return strings.Trim(raw, `"'`), nil
}

func (s *OpenAICompatibleAIService) SuggestCategoryBulk(ctx context.Context, txs []BulkClassifyInput, categories []string, examples []ClassificationExample) (map[int64]string, error) {
	if len(txs) == 0 {
		return map[int64]string{}, nil
	}

	var sb strings.Builder
	if len(examples) > 0 {
		sb.WriteString("Histórico de classificações deste usuário:\n")
		for _, ex := range examples {
			fmt.Fprintf(&sb, "- \"%s\" → %s\n", ex.Description, ex.Category)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Classifique as transações abaixo. Use apenas as categorias listadas.\n\nTransações:\n")
	for _, tx := range txs {
		dirLabel := "despesa"
		if tx.Direction == "credit" {
			dirLabel = "receita"
		}
		fmt.Fprintf(&sb, "- ID:%d | %s | %s\n", tx.ID, tx.Description, dirLabel)
	}
	fmt.Fprintf(&sb, "\nCategorias disponíveis: %s\n\n", strings.Join(categories, ", "))
	sb.WriteString("Responda APENAS com um array JSON, sem texto adicional. Formato exato:\n")
	sb.WriteString(`[{"id": 123, "category": "NomeDaCategoria"}, ...]`)

	system := "Você é um assistente de classificação financeira. Responda apenas com JSON válido."
	raw, err := s.chat(ctx, system, sb.String(), 1024)
	if err != nil {
		return nil, err
	}
	return parseBulkResponse(raw), nil
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

// NoopAIService retorna "Sem categoria" quando IA não está configurada.
type NoopAIService struct{}

func (n *NoopAIService) SuggestCategory(_ context.Context, _, _ string, _ []string, _ []ClassificationExample) (string, error) {
	return "Sem categoria", nil
}

func (n *NoopAIService) SuggestCategoryBulk(_ context.Context, txs []BulkClassifyInput, _ []string, _ []ClassificationExample) (map[int64]string, error) {
	result := make(map[int64]string, len(txs))
	for _, tx := range txs {
		result[tx.ID] = "Sem categoria"
	}
	return result, nil
}

func (n *NoopAIService) Generate(_ context.Context, _ string) (string, error) {
	return "Serviço de IA não configurado.", nil
}
