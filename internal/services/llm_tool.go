package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"finops/internal/store"
	"finops/internal/utils"
)

type financialTool struct {
	schema  openai.Tool
	handler func(ctx context.Context, args json.RawMessage) (string, error)
}

// params é um helper para construir JSON Schema de parâmetros de tools.
func params(properties map[string]any, required ...string) json.RawMessage {
	if properties == nil {
		properties = map[string]any{}
	}
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	raw, _ := json.Marshal(schema)
	return raw
}

func prop(typ, description string) map[string]string {
	return map[string]string{"type": typ, "description": description}
}

func tool(name, description string, parameters json.RawMessage) openai.Tool {
	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        name,
			Description: description,
			Parameters:  parameters,
		},
	}
}

// FinancialTools retorna as ferramentas disponíveis para o agente de chat.
func FinancialTools(reportSvc ReportsService, accountSvc AccountService, categorySvc CategoryService, projectionSvc ProjectionService, userID int64) []financialTool {
	return []financialTool{
		accountBalancesTool(accountSvc, userID),
		categoriesTool(categorySvc, userID),
		spendingByCategoryTool(reportSvc, userID),
		spendingTrendTool(reportSvc, userID),
		topExpensesTool(reportSvc, userID),
		monthlyComparisonTool(reportSvc, userID),
		balanceHistoryTool(reportSvc, userID),
		listTransactionsTool(reportSvc, userID),
		projectionForecastTool(projectionSvc, userID),
	}
}

func projectionForecastTool(projectionSvc ProjectionService, userID int64) financialTool {
	return financialTool{
		schema: tool("get_cash_flow_projection",
			"Projeta o fluxo de caixa FUTURO mês a mês (entradas, saídas fixas, sobra e caixa acumulado) a partir das premissas e compromissos cadastrados (parcelamentos, assinaturas, recebíveis). Use para perguntas sobre meses futuros: quanto vai sobrar, quando a fatura alivia, meses no vermelho.",
			params(map[string]any{
				"from": prop("string", "Mês inicial no formato YYYY-MM (opcional; default: mês atual)"),
				"to":   prop("string", "Mês final no formato YYYY-MM (opcional; default: 11 meses após o inicial)"),
			}),
		),
		handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct {
				From string `json:"from"`
				To   string `json:"to"`
			}
			_ = json.Unmarshal(args, &p)
			now := time.Now()
			from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
			if t, err := time.Parse("2006-01", p.From); err == nil {
				from = t
			}
			to := from.AddDate(0, 11, 0)
			if t, err := time.Parse("2006-01", p.To); err == nil && !t.Before(from) {
				to = t
			}
			res, err := projectionSvc.Forecast(ctx, userID, from, to)
			if err != nil {
				return "", err
			}
			if len(res.Months) == 0 {
				return "Nenhum mês no período.", nil
			}
			months := [...]string{"Jan", "Fev", "Mar", "Abr", "Mai", "Jun", "Jul", "Ago", "Set", "Out", "Nov", "Dez"}
			mName := func(t time.Time) string { return fmt.Sprintf("%s/%d", months[t.Month()-1], t.Year()) }
			var sb strings.Builder
			fmt.Fprintf(&sb, "Projeção de fluxo de caixa (%s a %s):\n", mName(from), mName(to))
			for _, m := range res.Months {
				fmt.Fprintf(&sb, "- %s: entradas %s | saídas %s | sobra %s | caixa acumulado %s\n",
					mName(m.Month), utils.FormatMoney(m.Income), utils.FormatMoney(m.FixedOut+m.VariableOut),
					utils.FormatMoney(m.Net), utils.FormatMoney(m.Cumulative))
			}
			fmt.Fprintf(&sb, "Resumo: sobra média %s | pior mês %s (%s) | melhor mês %s (%s) | caixa final %s | %d mês(es) no vermelho\n",
				utils.FormatMoney(res.Summary.AvgNet),
				mName(res.Summary.WorstMonth), utils.FormatMoney(res.Summary.WorstNet),
				mName(res.Summary.BestMonth), utils.FormatMoney(res.Summary.BestNet),
				utils.FormatMoney(res.Summary.FinalCash), res.Summary.MonthsNetNegative)
			if len(res.Milestones) > 0 {
				sb.WriteString("Marcos:\n")
				for _, ms := range res.Milestones {
					fmt.Fprintf(&sb, "- %s: %s\n", mName(ms.Month), ms.Label)
				}
			}
			return sb.String(), nil
		},
	}
}

// ClassificationTools retorna as ferramentas disponíveis para o agente de classificação.
func ClassificationTools(embSvc EmbeddingService, rawDB *sql.DB, queries *store.Queries, userID int64) []financialTool {
	return []financialTool{
		searchSimilarClassificationsTool(embSvc, rawDB, queries, userID),
	}
}

func accountBalancesTool(accountSvc AccountService, userID int64) financialTool {
	return financialTool{
		schema: tool("get_account_balances", "Retorna o saldo atual de todas as contas do usuário.", params(nil)),
		handler: func(ctx context.Context, _ json.RawMessage) (string, error) {
			accounts, err := accountSvc.ListSummariesByUser(ctx, userID)
			if err != nil {
				return "", err
			}
			if len(accounts) == 0 {
				return "Nenhuma conta cadastrada.", nil
			}
			var sb strings.Builder
			var total float64
			for _, acc := range accounts {
				total += acc.CurrentBalance
				fmt.Fprintf(&sb, "- %s (%s): %s\n", acc.Name, acc.Type, utils.FormatMoney(acc.CurrentBalance))
			}
			fmt.Fprintf(&sb, "Saldo total: %s", utils.FormatMoney(total))
			return sb.String(), nil
		},
	}
}

func categoriesTool(categorySvc CategoryService, userID int64) financialTool {
	return financialTool{
		schema: tool("get_categories",
			"Retorna todas as categorias cadastradas com seu tipo (expense=despesa, income=receita). Use antes de classificar gastos como essenciais ou supérfluos.",
			params(nil),
		),
		handler: func(ctx context.Context, _ json.RawMessage) (string, error) {
			cats, err := categorySvc.GetCategories(ctx, userID)
			if err != nil {
				return "", err
			}
			if len(cats) == 0 {
				return "Nenhuma categoria cadastrada.", nil
			}
			var sb strings.Builder
			sb.WriteString("Categorias cadastradas:\n")
			for _, c := range cats {
				fmt.Fprintf(&sb, "- %s (%s)\n", c.Name, c.Kind)
			}
			return sb.String(), nil
		},
	}
}

type periodArgs struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func parsePeriod(args json.RawMessage) (time.Time, time.Time, error) {
	var p periodArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return time.Time{}, time.Time{}, err
	}
	from, err := time.Parse("2006-01-02", p.From)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("data 'from' inválida: %w", err)
	}
	to, err := time.Parse("2006-01-02", p.To)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("data 'to' inválida: %w", err)
	}
	return from, to, nil
}

var periodParams = params(map[string]any{
	"from": prop("string", "Data inicial no formato YYYY-MM-DD"),
	"to":   prop("string", "Data final no formato YYYY-MM-DD"),
}, "from", "to")

func spendingByCategoryTool(reportSvc ReportsService, userID int64) financialTool {
	return financialTool{
		schema: tool("get_spending_by_category",
			"Retorna os gastos (despesas) agrupados por categoria em um período.",
			periodParams,
		),
		handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			from, to, err := parsePeriod(args)
			if err != nil {
				return "", err
			}
			rows, err := reportSvc.SpendingByCategory(ctx, userID, from, to)
			if err != nil {
				return "", err
			}
			if len(rows) == 0 {
				return "Nenhum gasto encontrado no período.", nil
			}
			var sb strings.Builder
			fmt.Fprintf(&sb, "Gastos por categoria (%s a %s):\n", from.Format("02/01/2006"), to.Format("02/01/2006"))
			for _, r := range rows {
				fmt.Fprintf(&sb, "- %s: %s\n", r.CategoryName, utils.FormatMoney(r.Total))
			}
			return sb.String(), nil
		},
	}
}

func spendingTrendTool(reportSvc ReportsService, userID int64) financialTool {
	return financialTool{
		schema: tool("get_spending_trend",
			"Retorna gastos por categoria mês a mês em um período. Use para calcular médias mensais por categoria e identificar tendências de gasto ao longo de 3-6 meses.",
			periodParams,
		),
		handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			from, to, err := parsePeriod(args)
			if err != nil {
				return "", err
			}
			rows, err := reportSvc.SpendingTrend(ctx, userID, from, to)
			if err != nil {
				return "", err
			}
			if len(rows) == 0 {
				return "Nenhum dado encontrado no período.", nil
			}
			months := [...]string{"Jan", "Fev", "Mar", "Abr", "Mai", "Jun", "Jul", "Ago", "Set", "Out", "Nov", "Dez"}
			var sb strings.Builder
			fmt.Fprintf(&sb, "Gastos por categoria e mês (%s a %s):\n", from.Format("02/01/2006"), to.Format("02/01/2006"))
			for _, r := range rows {
				month := fmt.Sprintf("%s/%d", months[r.Month.Month()-1], r.Month.Year())
				fmt.Fprintf(&sb, "- %s | %s: %s\n", month, r.CategoryName, utils.FormatMoney(r.Total))
			}
			return sb.String(), nil
		},
	}
}

func topExpensesTool(reportSvc ReportsService, userID int64) financialTool {
	return financialTool{
		schema: tool("get_top_expenses",
			"Retorna as maiores despesas individuais ordenadas por valor. Use para identificar gastos pontuais de alto valor.",
			params(map[string]any{
				"from":  prop("string", "Data inicial YYYY-MM-DD"),
				"to":    prop("string", "Data final YYYY-MM-DD"),
				"limit": prop("integer", "Número de resultados (padrão 10, máximo 20)"),
			}, "from", "to"),
		),
		handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct {
				Limit int32 `json:"limit"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("argumentos inválidos: %w", err)
			}
			from, to, err := parsePeriod(args)
			if err != nil {
				return "", err
			}
			limit := int32(10)
			if p.Limit > 0 && p.Limit <= 20 {
				limit = p.Limit
			}
			items, err := reportSvc.TopExpenses(ctx, userID, from, to, limit)
			if err != nil {
				return "", err
			}
			if len(items) == 0 {
				return "Nenhuma despesa encontrada no período.", nil
			}
			var sb strings.Builder
			fmt.Fprintf(&sb, "Maiores despesas (%s a %s):\n", from.Format("02/01/2006"), to.Format("02/01/2006"))
			for i, item := range items {
				fmt.Fprintf(&sb, "%d. %s | %s | %s | %s\n",
					i+1,
					item.PostedOn.Format("02/01/2006"),
					item.Description,
					utils.FormatMoney(item.Amount),
					item.CategoryName,
				)
			}
			return sb.String(), nil
		},
	}
}

func monthlyComparisonTool(reportSvc ReportsService, userID int64) financialTool {
	return financialTool{
		schema: tool("get_monthly_comparison",
			"Retorna comparativo mensal de receitas e despesas em um período.",
			periodParams,
		),
		handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			from, to, err := parsePeriod(args)
			if err != nil {
				return "", err
			}
			rows, err := reportSvc.MonthlyComparison(ctx, userID, from, to)
			if err != nil {
				return "", err
			}
			if len(rows) == 0 {
				return "Nenhum dado encontrado no período.", nil
			}
			months := [...]string{"Jan", "Fev", "Mar", "Abr", "Mai", "Jun", "Jul", "Ago", "Set", "Out", "Nov", "Dez"}
			var sb strings.Builder
			fmt.Fprintf(&sb, "Comparativo mensal (%s a %s):\n", from.Format("02/01/2006"), to.Format("02/01/2006"))
			for _, r := range rows {
				month := fmt.Sprintf("%s/%d", months[r.Month.Month()-1], r.Month.Year())
				saldo := r.Income - r.Expenses
				fmt.Fprintf(&sb, "- %s: receitas %s | despesas %s | saldo %s\n",
					month, utils.FormatMoney(r.Income), utils.FormatMoney(r.Expenses), utils.FormatMoney(saldo))
			}
			return sb.String(), nil
		},
	}
}

func balanceHistoryTool(reportSvc ReportsService, userID int64) financialTool {
	return financialTool{
		schema: tool("get_balance_history",
			"Retorna a evolução do saldo patrimonial mês a mês em um período.",
			periodParams,
		),
		handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			from, to, err := parsePeriod(args)
			if err != nil {
				return "", err
			}
			points, err := reportSvc.BalancedHistory(ctx, userID, from, to)
			if err != nil {
				return "", err
			}
			if len(points) == 0 {
				return "Nenhum dado encontrado no período.", nil
			}
			months := [...]string{"Jan", "Fev", "Mar", "Abr", "Mai", "Jun", "Jul", "Ago", "Set", "Out", "Nov", "Dez"}
			var sb strings.Builder
			fmt.Fprintf(&sb, "Evolução do saldo (%s a %s):\n", from.Format("02/01/2006"), to.Format("02/01/2006"))
			for _, p := range points {
				month := fmt.Sprintf("%s/%d", months[p.Month.Month()-1], p.Month.Year())
				fmt.Fprintf(&sb, "- %s: %s\n", month, utils.FormatMoney(p.Balance))
			}
			return sb.String(), nil
		},
	}
}

func searchSimilarClassificationsTool(embSvc EmbeddingService, rawDB *sql.DB, queries *store.Queries, userID int64) financialTool {
	return financialTool{
		schema: tool("search_similar_classifications",
			"Busca transações já classificadas semanticamente semelhantes à descrição fornecida. Use para descobrir como transações parecidas foram categorizadas no passado.",
			params(map[string]any{
				"query": prop("string", "Descrição ou palavras-chave da transação a ser pesquisada"),
				"limit": prop("integer", "Número de resultados (padrão 5, máximo 10)"),
			}, "query"),
		),
		handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct {
				Query string `json:"query"`
				Limit int    `json:"limit"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("argumentos inválidos: %w", err)
			}
			if p.Limit <= 0 || p.Limit > 10 {
				p.Limit = 5
			}

			ws, err := queries.GetWorkSpaceByOwnerUserID(ctx, userID)
			if err != nil {
				return "", err
			}

			emb, err := embSvc.Embed(ctx, p.Query)
			if err != nil {
				return "Serviço de embeddings indisponível.", nil
			}

			results, err := store.SearchClassificationEmbeddings(ctx, rawDB, ws.ID, emb, p.Limit)
			if err != nil {
				return "", err
			}
			if len(results) == 0 {
				return "Nenhuma classificação similar encontrada.", nil
			}

			var sb strings.Builder
			sb.WriteString("Classificações similares encontradas:\n")
			for _, r := range results {
				fmt.Fprintf(&sb, "- \"%s\" → %s\n", r.Description, r.Category)
			}
			return sb.String(), nil
		},
	}
}

func listTransactionsTool(reportSvc ReportsService, userID int64) financialTool {
	return financialTool{
		schema: tool("list_transactions",
			"Lista transações com filtros opcionais de período, tipo (debit/credit) e limite.",
			params(map[string]any{
				"from":      prop("string", "Data inicial YYYY-MM-DD (opcional)"),
				"to":        prop("string", "Data final YYYY-MM-DD (opcional)"),
				"direction": map[string]any{"type": "string", "enum": []string{"debit", "credit"}, "description": "debit=despesa, credit=receita (opcional)"},
				"limit":     prop("integer", "Número de resultados (padrão 10, máximo 50)"),
			}),
		),
		handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct {
				From      string `json:"from"`
				To        string `json:"to"`
				Direction string `json:"direction"`
				Limit     int32  `json:"limit"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("argumentos inválidos: %w", err)
			}

			filter := TransactionFilter{Limit: 10}
			if p.Limit > 0 && p.Limit <= 50 {
				filter.Limit = p.Limit
			}
			if p.Direction != "" {
				filter.Direction = &p.Direction
			}
			if p.From != "" {
				if t, err := time.Parse("2006-01-02", p.From); err == nil {
					filter.FromDate = &t
				}
			}
			if p.To != "" {
				if t, err := time.Parse("2006-01-02", p.To); err == nil {
					filter.ToDate = &t
				}
			}

			txs, err := reportSvc.ListFiltered(ctx, userID, filter)
			if err != nil {
				return "", err
			}
			if len(txs) == 0 {
				return "Nenhuma transação encontrada.", nil
			}
			var sb strings.Builder
			sb.WriteString("Transações:\n")
			for _, tx := range txs {
				sign := "-"
				if tx.Direction == "credit" {
					sign = "+"
				}
				cat := tx.Category
				if cat == "" {
					cat = "Sem categoria"
				}
				fmt.Fprintf(&sb, "- %s | %s | %s%s | %s | %s\n",
					tx.PostedOn.Format("02/01/2006"),
					tx.Description,
					sign, utils.FormatMoney(tx.Amount),
					cat,
					tx.AccountName,
				)
			}
			return sb.String(), nil
		},
	}
}
