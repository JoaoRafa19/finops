package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ClassificationExample struct {
	Description string
	Category    string
}

type AIService interface {
	SuggestCategory(ctx context.Context, description, direction string, categories []string, examples []ClassificationExample) (string, error)
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
		client:  &http.Client{Timeout: 90 * time.Second},
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

func (s *OllamaAIService) SuggestCategory(ctx context.Context, description, direction string, categories []string, examples []ClassificationExample) (string, error) {
	dirLabel := "despesa"
	if direction == "credit" {
		dirLabel = "receita"
	}

	catList := strings.Join(categories, ", ")

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
Se nenhuma for adequada, responda com: Sem categoria`, examplesBlock, description, dirLabel, catList)

	body, err := json.Marshal(ollamaRequest{
		Model:  s.model,
		Prompt: prompt,
		Stream: false,
	})
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

	suggestion := strings.TrimSpace(out.Response)
	suggestion = strings.Trim(suggestion, `"'`)
	return suggestion, nil
}

// NoopAIService retorna "Sem categoria" quando Ollama não está configurado.
type NoopAIService struct{}

func (n *NoopAIService) SuggestCategory(_ context.Context, _, _ string, _ []string, _ []ClassificationExample) (string, error) {
	return "Sem categoria", nil
}
