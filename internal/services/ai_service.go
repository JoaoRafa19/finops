package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
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
}

type OllamaAIService struct {
	baseURL string
	model   string
	client  *http.Client
}

func NewOllamaAIService(baseURL, model string) AIService {
	return &OllamaAIService{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

func (s *OllamaAIService) callOllama(ctx context.Context, prompt string) (string, error) {
	body, err := json.Marshal(ollamaRequest{Model: s.model, Prompt: prompt, Stream: false})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama indisponível: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama status %d: %s", resp.StatusCode, string(b))
	}

	var out ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.Response), nil
}

func (s *OllamaAIService) SuggestCategory(ctx context.Context, description, direction string, categories []string, examples []ClassificationExample) (string, error) {
	dirLabel := "despesa"
	if direction == "credit" {
		dirLabel = "receita"
	}

	var examplesBlock string
	if len(examples) > 0 {
		var sb strings.Builder
		sb.WriteString("Histórico de classificações deste usuário:\n")
		for _, ex := range examples {
			sb.WriteString(fmt.Sprintf("- \"%s\" → %s\n", ex.Description, ex.Category))
		}
		examplesBlock = sb.String() + "\n"
	}

	prompt := fmt.Sprintf(`Você é um assistente financeiro pessoal que aprende com o histórico do usuário.

%sNova transação a classificar:
Descrição: %s
Tipo: %s
Categorias disponíveis: %s

Responda APENAS com o nome exato de uma das categorias disponíveis, sem explicações.
Se nenhuma for adequada, responda com: Sem categoria`, examplesBlock, description, dirLabel, strings.Join(categories, ", "))

	raw, err := s.callOllama(ctx, prompt)
	if err != nil {
		return "", err
	}
	return strings.Trim(raw, `"'`), nil
}

func (s *OllamaAIService) SuggestCategoryBulk(ctx context.Context, txs []BulkClassifyInput, categories []string, examples []ClassificationExample) (map[int64]string, error) {
	if len(txs) == 0 {
		return map[int64]string{}, nil
	}

	var sb strings.Builder

	sb.WriteString("Você é um assistente financeiro pessoal.\n\n")

	if len(examples) > 0 {
		sb.WriteString("Histórico de classificações deste usuário:\n")
		for _, ex := range examples {
			sb.WriteString(fmt.Sprintf("- \"%s\" → %s\n", ex.Description, ex.Category))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Classifique as transações abaixo. Use apenas as categorias listadas.\n\n")
	sb.WriteString("Transações:\n")
	for _, tx := range txs {
		dirLabel := "despesa"
		if tx.Direction == "credit" {
			dirLabel = "receita"
		}
		sb.WriteString(fmt.Sprintf("- ID:%d | %s | %s\n", tx.ID, tx.Description, dirLabel))
	}

	sb.WriteString(fmt.Sprintf("\nCategorias disponíveis: %s\n\n", strings.Join(categories, ", ")))
	sb.WriteString("Responda APENAS com um array JSON, sem texto adicional. Formato exato:\n")
	sb.WriteString(`[{"id": 123, "category": "NomeDaCategoria"}, ...]`)

	raw, err := s.callOllama(ctx, sb.String())
	if err != nil {
		return nil, err
	}

	return parseBulkResponse(raw), nil
}

var jsonArrayRe = regexp.MustCompile(`(?s)\[.*?\]`)

func parseBulkResponse(raw string) map[int64]string {
	result := make(map[int64]string)

	// Tenta extrair o array JSON mesmo que o modelo adicione texto em volta
	jsonStr := raw
	if match := jsonArrayRe.FindString(raw); match != "" {
		jsonStr = match
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

// NoopAIService retorna "Sem categoria" quando Ollama não está configurado.
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
