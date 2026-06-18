package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"github.com/redis/go-redis/v9"
)

const chatHistoryKeyFmt = "finops:chat:hist:%d"
const maxChatHistory = 40
const chatHistoryTTL = 7 * 24 * time.Hour
const maxToolLoops = 8

// ChatMessage é usado para exibir o histórico na UI.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatService interface {
	Ask(ctx context.Context, userID int64, question string) (string, error)
	History(ctx context.Context, userID int64) ([]ChatMessage, error)
}

type OllamaAgentChatService struct {
	client      *openai.Client
	model       string
	rdb         *redis.Client
	accountSvc  AccountService
	reportSvc   ReportsService
	categorySvc CategoryService
}

func NewOllamaAgentChatService(
	baseURL, model string,
	rdb *redis.Client,
	accountSvc AccountService,
	reportSvc ReportsService,
	categorySvc CategoryService,
) ChatService {
	cfg := openai.DefaultConfig("ollama")
	cfg.BaseURL = strings.TrimRight(baseURL, "/") + "/v1"
	return &OllamaAgentChatService{
		client:      openai.NewClientWithConfig(cfg),
		model:       model,
		rdb:         rdb,
		accountSvc:  accountSvc,
		reportSvc:   reportSvc,
		categorySvc: categorySvc,
	}
}

func (s *OllamaAgentChatService) histKey(userID int64) string {
	return fmt.Sprintf(chatHistoryKeyFmt, userID)
}

func (s *OllamaAgentChatService) loadMsgs(ctx context.Context, userID int64) ([]openai.ChatCompletionMessage, error) {
	raw, err := s.rdb.Get(ctx, s.histKey(userID)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var msgs []openai.ChatCompletionMessage
	if err := json.Unmarshal(raw, &msgs); err != nil {
		return nil, err
	}
	return msgs, nil
}

func (s *OllamaAgentChatService) saveMsgs(ctx context.Context, userID int64, msgs []openai.ChatCompletionMessage) {
	raw, err := json.Marshal(msgs)
	if err != nil {
		return
	}
	_ = s.rdb.Set(ctx, s.histKey(userID), raw, chatHistoryTTL).Err()
}

// History retorna apenas mensagens de usuário e assistente para exibição na UI.
func (s *OllamaAgentChatService) History(ctx context.Context, userID int64) ([]ChatMessage, error) {
	msgs, err := s.loadMsgs(ctx, userID)
	if err != nil {
		return nil, err
	}
	var out []ChatMessage
	for _, m := range msgs {
		if (m.Role == openai.ChatMessageRoleUser || m.Role == openai.ChatMessageRoleAssistant) && m.Content != "" {
			out = append(out, ChatMessage{Role: m.Role, Content: m.Content})
		}
	}
	return out, nil
}

func (s *OllamaAgentChatService) Ask(ctx context.Context, userID int64, question string) (string, error) {
	logger := slog.Default()
	history, _ := s.loadMsgs(ctx, userID)
	tools := FinancialTools(s.reportSvc, s.accountSvc, s.categorySvc, userID)

	// Constrói schema de tools para a API
	openaiTools := make([]openai.Tool, len(tools))
	for i, t := range tools {
		openaiTools[i] = t.schema
	}

	systemMsg := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: buildSystemPrompt(time.Now()),
	}

	messages := make([]openai.ChatCompletionMessage, 0, 1+len(history)+1)
	messages = append(messages, systemMsg)
	messages = append(messages, history...)
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: question,
	})

	var finalAnswer string
	for i := 0; i < maxToolLoops; i++ {
		resp, err := s.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:    s.model,
			Messages: messages,
			Tools:    openaiTools,
		})
		if err != nil {
			return "", fmt.Errorf("erro ao chamar modelo: %w", err)
		}

		if len(resp.Choices) == 0 {
			break
		}

		msg := resp.Choices[0].Message
		messages = append(messages, msg)

		if len(msg.ToolCalls) == 0 {
			finalAnswer = msg.Content
			logger.Info("chat_final_answer", "iteration", i, "answer", finalAnswer)
			break
		}

		for _, tc := range msg.ToolCalls {
			logger.Info("chat_tool_call", "iteration", i, "tool", tc.Function.Name, "args", tc.Function.Arguments)
			result := executeTool(ctx, tc.Function.Name, json.RawMessage(tc.Function.Arguments), tools)
			logger.Info("chat_tool_result", "tool", tc.Function.Name, "result", result)

			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	if finalAnswer == "" {
		finalAnswer = "Não consegui processar sua pergunta. Tente reformulá-la."
	}

	// Persiste histórico sem a system message
	stored := messages[1:]
	if len(stored) > maxChatHistory {
		stored = stored[len(stored)-maxChatHistory:]
	}
	s.saveMsgs(ctx, userID, stored)

	return finalAnswer, nil
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

func buildSystemPrompt(now time.Time) string {
	d := func(days int) string { return now.AddDate(0, 0, -days).Format("2006-01-02") }
	m := func(months int) string { return now.AddDate(0, -months, 0).Format("2006-01-02") }
	today := now.Format("2006-01-02")
	firstDayOfMonth := now.Format("2006-01") + "-01"

	return fmt.Sprintf(`Você é um assistente financeiro pessoal chamado Finops.
Responda SEMPRE em português.

REGRA OBRIGATÓRIA: Antes de responder qualquer pergunta sobre dados financeiros, você DEVE chamar uma ou mais ferramentas para buscar os dados reais. NUNCA invente ou estime valores financeiros.

═══════════════════════════════════════════
REFERÊNCIAS DE DATA
═══════════════════════════════════════════
- Hoje: %s
- Início deste mês: %s
- 7 dias atrás: %s
- 10 dias atrás: %s
- 15 dias atrás: %s
- 30 dias atrás: %s
- 1 mês atrás: %s
- 3 meses atrás: %s
- 6 meses atrás: %s
- 12 meses atrás: %s

═══════════════════════════════════════════
GUIA DE FERRAMENTAS
═══════════════════════════════════════════
- get_account_balances → saldos atuais das contas
- get_categories → lista todas as categorias com seu tipo; use antes de classificar gastos
- get_spending_by_category(from, to) → total de gastos por categoria em um período
- get_spending_trend(from, to) → gastos por categoria MÊS A MÊS; use para calcular médias e tendências
- get_top_expenses(from, to, limit) → maiores despesas individuais ordenadas por valor
- get_monthly_comparison(from, to) → receitas vs despesas totais por mês
- get_balance_history(from, to) → evolução do patrimônio por mês
- list_transactions(from, to, direction, limit) → listagem de transações recentes

═══════════════════════════════════════════
PROTOCOLO PARA PERGUNTAS ANALÍTICAS
═══════════════════════════════════════════
Quando o usuário perguntar sobre corte de gastos, economia, o que é supérfluo ou essencial, siga ESTES PASSOS EM SEQUÊNCIA:

PASSO 1 → Chamar get_spending_trend com from="%s" to="%s" (últimos 3 meses)
  Objetivo: identificar quais categorias têm gastos consistentemente altos mês a mês.

PASSO 2 → Chamar get_top_expenses com from="%s" to="%s" limit=15
  Objetivo: identificar despesas pontuais de alto valor que distorcem os totais.

PASSO 3 → Chamar get_categories
  Objetivo: conhecer os nomes exatos das categorias cadastradas pelo usuário.

PASSO 4 → Classificar internamente as categorias em:
  ESSENCIAIS: moradia (Aluguel, Condomínio, Luz, Água, Gás, Internet), saúde (Saúde, Farmácia, Médico, Plano de Saúde), alimentação básica (Mercado, Supermercado), transporte necessário (Transporte, Combustível, Passagem)
  SUPÉRFLUOS/DISCRICIONÁRIOS: lazer (Lazer, Entretenimento, Cinema, Shows, Streaming), alimentação fora (Restaurante, Delivery, Ifood, Bar), compras não essenciais (Compras, Vestuário além do básico), assinaturas, viagens

PASSO 5 → Se o usuário mencionou uma meta (ex: "economizar R$600/mês"):
  Somar os gastos médios mensais das categorias discricionárias.
  Identificar quais cortar ou reduzir para atingir a meta.
  Ser específico: "Se você reduzir [categoria] de R$X para R$Y, economiza R$Z/mês."

PASSO 6 → Apresentar a resposta organizada em seções:
  1. Visão geral dos seus gastos nos últimos 3 meses
  2. Gastos essenciais (não cortar)
  3. Onde você pode economizar (com valores concretos)
  4. Resumo: quanto pode poupar no total

═══════════════════════════════════════════
EXEMPLOS RÁPIDOS
═══════════════════════════════════════════
- "quanto gastei esta semana" → get_spending_by_category(from="%s", to="%s")
- "meu saldo atual" → get_account_balances
- "onde posso cortar gastos" → seguir PROTOCOLO ANALÍTICO (3 chamadas em sequência)
- "quais meus gastos essenciais" → get_categories + get_spending_trend(3 meses)`,
		today, firstDayOfMonth,
		d(7), d(10), d(15), d(30),
		m(1), m(3), m(6), m(12),
		m(3), today, m(3), today,
		d(7), today,
	)
}
