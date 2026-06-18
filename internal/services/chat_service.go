package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const chatHistoryKeyFmt = "finops:chat:hist:%d"
const maxChatHistory = 40
const chatHistoryTTL = 7 * 24 * time.Hour
const maxToolLoops = 5

// ChatMessage é usado para exibir o histórico na UI.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatService interface {
	Ask(ctx context.Context, userID int64, question string) (string, error)
	History(ctx context.Context, userID int64) ([]ChatMessage, error)
}

// --- Tipos internos da API /api/chat do Ollama ---

type ollamaChatMsg struct {
	Role      string       `json:"role"`
	Content   string       `json:"content,omitempty"`
	ToolCalls []ollamaTC   `json:"tool_calls,omitempty"`
}

type ollamaTC struct {
	Function ollamaTCFn `json:"function"`
}

type ollamaTCFn struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ollamaChatReq struct {
	Model    string          `json:"model"`
	Messages []ollamaChatMsg `json:"messages"`
	Tools    []ollamaTool    `json:"tools,omitempty"`
	Stream   bool            `json:"stream"`
}

type ollamaChatResp struct {
	Message ollamaChatMsg `json:"message"`
	Done    bool          `json:"done"`
}

// --- Implementação ---

type OllamaAgentChatService struct {
	baseURL    string
	model      string
	client     *http.Client
	rdb        *redis.Client
	accountSvc AccountService
	reportSvc  ReportsService
}

func NewOllamaAgentChatService(
	baseURL, model string,
	rdb *redis.Client,
	accountSvc AccountService,
	reportSvc ReportsService,
) ChatService {
	return &OllamaAgentChatService{
		baseURL:    strings.TrimRight(baseURL, "/"),
		model:      model,
		client:     &http.Client{Timeout: 180 * time.Second},
		rdb:        rdb,
		accountSvc: accountSvc,
		reportSvc:  reportSvc,
	}
}

func (s *OllamaAgentChatService) histKey(userID int64) string {
	return fmt.Sprintf(chatHistoryKeyFmt, userID)
}

func (s *OllamaAgentChatService) loadMsgs(ctx context.Context, userID int64) ([]ollamaChatMsg, error) {
	raw, err := s.rdb.Get(ctx, s.histKey(userID)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var msgs []ollamaChatMsg
	if err := json.Unmarshal(raw, &msgs); err != nil {
		return nil, err
	}
	return msgs, nil
}

func (s *OllamaAgentChatService) saveMsgs(ctx context.Context, userID int64, msgs []ollamaChatMsg) {
	raw, err := json.Marshal(msgs)
	if err != nil {
		return
	}
	_ = s.rdb.Set(ctx, s.histKey(userID), raw, chatHistoryTTL).Err()
}

func (s *OllamaAgentChatService) History(ctx context.Context, userID int64) ([]ChatMessage, error) {
	msgs, err := s.loadMsgs(ctx, userID)
	if err != nil {
		return nil, err
	}
	var out []ChatMessage
	for _, m := range msgs {
		if (m.Role == "user" || m.Role == "assistant") && m.Content != "" {
			out = append(out, ChatMessage{Role: m.Role, Content: m.Content})
		}
	}
	return out, nil
}

func (s *OllamaAgentChatService) Ask(ctx context.Context, userID int64, question string) (string, error) {
	history, _ := s.loadMsgs(ctx, userID)
	tools := FinancialTools(s.reportSvc, s.accountSvc, userID)

	systemMsg := ollamaChatMsg{
		Role: "system",
		Content: fmt.Sprintf(
			"Você é um assistente financeiro pessoal chamado Finops. "+
				"Responda sempre em português, de forma clara e objetiva. "+
				"Use as ferramentas disponíveis para buscar dados reais do usuário antes de responder. "+
				"Hoje é %s.",
			time.Now().Format("02/01/2006"),
		),
	}

	messages := make([]ollamaChatMsg, 0, 1+len(history)+1)
	messages = append(messages, systemMsg)
	messages = append(messages, history...)
	messages = append(messages, ollamaChatMsg{Role: "user", Content: question})

	ollamaTools := make([]ollamaTool, len(tools))
	for i, t := range tools {
		ollamaTools[i] = t.schema
	}

	logger := slog.Default()
	var finalAnswer string
	for i := 0; i < maxToolLoops; i++ {
		resp, err := s.callChat(ctx, messages, ollamaTools)
		if err != nil {
			return "", err
		}

		messages = append(messages, resp.Message)

		if len(resp.Message.ToolCalls) == 0 {
			finalAnswer = resp.Message.Content
			logger.Info("chat_final_answer", "iteration", i, "answer", finalAnswer)
			break
		}

		for _, tc := range resp.Message.ToolCalls {
			logger.Info("chat_tool_call", "iteration", i, "tool", tc.Function.Name, "args", string(tc.Function.Arguments))
			result := executeTool(ctx, tc.Function.Name, tc.Function.Arguments, tools)
			logger.Info("chat_tool_result", "tool", tc.Function.Name, "result", result)
			messages = append(messages, ollamaChatMsg{Role: "tool", Content: result})
		}
	}

	if finalAnswer == "" {
		finalAnswer = "Não consegui processar sua pergunta. Tente reformulá-la."
	}

	stored := messages[1:] // exclui system message do histórico salvo
	if len(stored) > maxChatHistory {
		stored = stored[len(stored)-maxChatHistory:]
	}
	s.saveMsgs(ctx, userID, stored)

	return finalAnswer, nil
}

func (s *OllamaAgentChatService) callChat(ctx context.Context, messages []ollamaChatMsg, tools []ollamaTool) (ollamaChatResp, error) {
	body, err := json.Marshal(ollamaChatReq{
		Model:    s.model,
		Messages: messages,
		Tools:    tools,
		Stream:   false,
	})
	if err != nil {
		return ollamaChatResp{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return ollamaChatResp{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return ollamaChatResp{}, fmt.Errorf("ollama indisponível: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return ollamaChatResp{}, fmt.Errorf("ollama status %d: %s", resp.StatusCode, string(b))
	}

	var out ollamaChatResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ollamaChatResp{}, err
	}
	return out, nil
}

func executeTool(ctx context.Context, name string, args json.RawMessage, tools []financialTool) string {
	for _, t := range tools {
		if t.schema.Function.Name == name {
			result, err := t.handler(ctx, args)
			if err != nil {
				return fmt.Sprintf("Erro ao executar %s: %v", name, err)
			}
			return result
		}
	}
	return fmt.Sprintf("Ferramenta '%s' não encontrada.", name)
}
