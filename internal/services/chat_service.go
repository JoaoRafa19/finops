package service

import (
	"context"
	"encoding/json"
	"errors"
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
	Clear(ctx context.Context, userID int64) error
}

type OllamaAgentChatService struct {
	client      *openai.Client
	model       string
	rdb         *redis.Client
	accountSvc     AccountService
	reportSvc      ReportsService
	categorySvc    CategoryService
	projectionSvc  ProjectionService
	transactionSvc TransactionService
}

func NewOllamaAgentChatService(
	baseURL, apiKey, model string,
	rdb *redis.Client,
	accountSvc AccountService,
	reportSvc ReportsService,
	categorySvc CategoryService,
	projectionSvc ProjectionService,
	transactionSvc TransactionService,
) ChatService {
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = strings.TrimRight(baseURL, "/") + "/v1"
	return &OllamaAgentChatService{
		client:         openai.NewClientWithConfig(cfg),
		model:          model,
		rdb:            rdb,
		accountSvc:     accountSvc,
		reportSvc:      reportSvc,
		categorySvc:    categorySvc,
		projectionSvc:  projectionSvc,
		transactionSvc: transactionSvc,
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

	// Atalho determinístico de confirmação. Modelos pequenos (qwen2.5:7b,
	// llama-8b) tendem a re-encenar o resumo em vez de chamar commit_transaction
	// quando o usuário responde "sim". Se há uma transação staged e a mensagem é
	// uma confirmação/negação curta, resolvemos sem depender do modelo.
	if s.pendingTxExists(ctx, userID) {
		if isAffirmation(question) {
			result, err := s.commitTransactionTool(userID).handler(ctx, nil)
			if err != nil {
				return "", err
			}
			s.appendHistory(ctx, userID, question, result)
			return result, nil
		}
		if isNegation(question) {
			s.rdb.Del(ctx, s.pendingTxKey(userID))
			const msg = "Ok, cancelei o lançamento. Nada foi gravado."
			s.appendHistory(ctx, userID, question, msg)
			return msg, nil
		}
	}

	history, _ := s.loadMsgs(ctx, userID)
	tools := FinancialTools(s.reportSvc, s.accountSvc, s.categorySvc, s.projectionSvc, userID)
	tools = append(tools, s.transactionWriteTools(userID)...)

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
		resp, err := s.completionWithRetry(ctx, openai.ChatCompletionRequest{
			Model:    s.model,
			Messages: messages,
			Tools:    openaiTools,
		})
		if err != nil {
			if isRateLimit(err) {
				return "", ErrRateLimited
			}
			return "", fmt.Errorf("erro ao chamar modelo: %w", err)
		}

		if len(resp.Choices) == 0 {
			break
		}

		msg := resp.Choices[0].Message

		// Fallback: modelos locais (qwen2.5 via Ollama) às vezes emitem a tool-call
		// como texto em vez de tool_calls estruturado. Extrai e sintetiza um
		// tool_call válido para não perder a ação (stage/commit).
		if len(msg.ToolCalls) == 0 {
			if name, args, ok := extractInlineToolCall(msg.Content, tools); ok {
				logger.Info("chat_inline_tool_call_recovered", "tool", name)
				msg.ToolCalls = []openai.ToolCall{{
					ID:       "inline_" + name,
					Type:     openai.ToolTypeFunction,
					Function: openai.FunctionCall{Name: name, Arguments: string(args)},
				}}
				msg.Content = ""
			}
		}

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

	// Persiste apenas as falas de usuário e as respostas finais do assistente.
	// Os tool_calls e tool-results (JSON grande) NÃO vão para o histórico: incham
	// o contexto e estouram o limite de tokens do provedor. O modelo rebusca via
	// tools quando precisar. ToolCalls é zerado para o histórico continuar válido.
	var stored []openai.ChatCompletionMessage
	for _, m := range messages[1:] {
		switch m.Role {
		case openai.ChatMessageRoleUser:
			stored = append(stored, m)
		case openai.ChatMessageRoleAssistant:
			if strings.TrimSpace(m.Content) != "" {
				stored = append(stored, openai.ChatCompletionMessage{Role: m.Role, Content: m.Content})
			}
		}
	}
	if len(stored) > maxChatHistory {
		stored = stored[len(stored)-maxChatHistory:]
	}
	s.saveMsgs(ctx, userID, stored)

	return finalAnswer, nil
}

// Clear apaga o histórico do chat e qualquer transação pendente do usuário.
func (s *OllamaAgentChatService) Clear(ctx context.Context, userID int64) error {
	return s.rdb.Del(ctx, s.histKey(userID), s.pendingTxKey(userID)).Err()
}

// pendingTxExists diz se há uma transação staged aguardando confirmação.
func (s *OllamaAgentChatService) pendingTxExists(ctx context.Context, userID int64) bool {
	n, err := s.rdb.Exists(ctx, s.pendingTxKey(userID)).Result()
	return err == nil && n > 0
}

// appendHistory grava um par pergunta/resposta no histórico (mesmo formato
// enxuto do Ask: só user + assistant, sem tool messages).
func (s *OllamaAgentChatService) appendHistory(ctx context.Context, userID int64, question, answer string) {
	msgs, _ := s.loadMsgs(ctx, userID)
	msgs = append(msgs,
		openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: question},
		openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: answer},
	)
	if len(msgs) > maxChatHistory {
		msgs = msgs[len(msgs)-maxChatHistory:]
	}
	s.saveMsgs(ctx, userID, msgs)
}

func normalizeConfirm(s string) string {
	return strings.Trim(strings.ToLower(strings.TrimSpace(s)), " .!¡\n\t")
}

// isAffirmation reconhece confirmações curtas para gravar a transação staged.
func isAffirmation(s string) bool {
	switch normalizeConfirm(s) {
	case "sim", "s", "confirmo", "confirma", "confirmar", "confirmado", "pode",
		"pode gravar", "pode salvar", "pode confirmar", "grava", "gravar",
		"salva", "salvar", "isso", "isso mesmo", "ok", "okay", "claro",
		"com certeza", "yes", "y":
		return true
	}
	return false
}

// isNegation reconhece cancelamentos curtos da transação staged.
func isNegation(s string) bool {
	switch normalizeConfirm(s) {
	case "não", "nao", "n", "cancela", "cancelar", "cancelado", "não gravar",
		"nao gravar", "deixa", "deixa pra lá", "esquece", "no":
		return true
	}
	return false
}

// ErrRateLimited sinaliza que o provedor de LLM recusou por excesso de requisições
// (429). O controller mostra uma mensagem específica ao usuário.
var ErrRateLimited = errors.New("limite de requisições do modelo atingido")

// isRateLimit detecta o 429 do provedor (Groq/OpenAI-compatible).
func isRateLimit(err error) bool {
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		return apiErr.HTTPStatusCode == 429
	}
	return err != nil && strings.Contains(err.Error(), "429")
}

// isBadFunctionCall detecta o 400 do Groq quando o llama gera uma tool-call
// malformada. É NÃO-determinístico: re-amostrar quase sempre resolve.
func isBadFunctionCall(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Failed to call a function")
}

// completionWithRetry re-tenta em dois casos: 429 (throughput do Groq — espera
// backoff) e o 400 "Failed to call a function" (tool-call malformada do llama —
// re-amostra na hora, sem espera).
func (s *OllamaAgentChatService) completionWithRetry(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	const maxAttempts = 4
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		resp, err := s.client.CreateChatCompletion(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		switch {
		case isRateLimit(err):
			select {
			case <-ctx.Done():
				return resp, ctx.Err()
			case <-time.After(time.Duration(attempt+1) * 2 * time.Second):
			}
		case isBadFunctionCall(err):
			// re-amostra imediatamente (pausa mínima para variar o sampling)
			select {
			case <-ctx.Done():
				return resp, ctx.Err()
			case <-time.After(200 * time.Millisecond):
			}
		default:
			return resp, err
		}
	}
	return openai.ChatCompletionResponse{}, lastErr
}

// extractInlineToolCall recupera uma tool-call que o modelo emitiu como TEXTO
// (ex.: `<tool_call>{"name":"stage_transaction","arguments":{...}}</tool_call>`
// ou com lixo antes), quando o provedor não a devolveu como tool_calls
// estruturado. Só aceita se o nome bater com uma tool conhecida.
func extractInlineToolCall(content string, tools []financialTool) (string, json.RawMessage, bool) {
	if content == "" {
		return "", nil, false
	}
	known := map[string]bool{}
	for _, t := range tools {
		known[t.schema.Function.Name] = true
	}
	c := strings.ReplaceAll(content, "<tool_call>", "")
	c = strings.ReplaceAll(c, "</tool_call>", "")

	for start := strings.IndexByte(c, '{'); start >= 0; {
		var m struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		// Decode lê um único objeto e ignora o que vier depois (lixo/tags).
		if err := json.NewDecoder(strings.NewReader(c[start:])).Decode(&m); err == nil && known[m.Name] {
			args := m.Arguments
			if len(args) == 0 || string(args) == "null" {
				args = json.RawMessage("{}")
			}
			return m.Name, args, true
		}
		next := strings.IndexByte(c[start+1:], '{')
		if next < 0 {
			break
		}
		start += 1 + next
	}
	return "", nil, false
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

	// Prompt enxuto de propósito: prompts longos sobrecarregam modelos pequenos
	// (qwen2.5:7b via Ollama) e quebram o tool-calling, além de estourar a cota de
	// tokens do Groq. Mantém só o essencial para o agente escolher e usar as tools.
	return fmt.Sprintf(`Você é o Finops, assistente financeiro pessoal. Responda SEMPRE em português, de forma curta e direta.

Antes de responder qualquer pergunta sobre dados financeiros, chame as ferramentas para buscar dados reais. NUNCA invente ou estime valores.

Datas de referência (YYYY-MM-DD): hoje=%s, início do mês=%s, 7 dias atrás=%s, 30 dias atrás=%s, 3 meses atrás=%s, 12 meses atrás=%s.

Ferramentas:
- get_account_balances(): saldos das contas
- get_categories(): categorias e seus tipos
- get_spending_by_category(from,to): gastos por categoria no período
- get_spending_trend(from,to): gastos por categoria mês a mês (médias/tendências)
- get_top_expenses(from,to,limit): maiores despesas
- get_monthly_comparison(from,to): receitas vs despesas por mês
- get_balance_history(from,to): evolução do patrimônio
- list_transactions(from,to,direction,limit): transações
- get_cash_flow_projection(from,to): projeção de fluxo de caixa
- stage_transaction(description,amount,direction,date?,account?,category?): PREPARA um lançamento (não grava)
- commit_transaction(): grava o lançamento preparado

Para registrar um gasto/receita (ex.: "gastei 50 no mercado ontem", "recebi 3000 de salário"):
1. Chame stage_transaction (valor positivo; direction=debit para gasto, credit para receita; date em YYYY-MM-DD, default hoje).
2. Copie EXATAMENTE o resumo que ela devolver e peça confirmação. Nunca grave sem confirmar.
3. Quando o usuário confirmar, chame commit_transaction(). NUNCA chame stage_transaction duas vezes para o mesmo lançamento nem repita o resumo.
Se faltar a conta ou houver duplicata, avise e pergunte — não invente.`,
		today, firstDayOfMonth, d(7), d(30), m(3), m(12),
	)
}
