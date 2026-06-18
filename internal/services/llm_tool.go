package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"finops/internal/utils"
)

type financialTool struct {
	schema  ollamaTool
	handler func(ctx context.Context, args json.RawMessage) (string, error)
}

type ollamaTool struct {
	Type     string       `json:"type"`
	Function ollamaToolFn `json:"function"`
}

type ollamaToolFn struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

func FinancialTools(reportSvc ReportsService, accountSvc AccountService, userID int64) []financialTool {
	return []financialTool{
		accountBalancesTool(accountSvc, userID),
		spendingByCategoryTool(reportSvc, userID),
		monthlyComparisonTool(reportSvc, userID),
		balanceHistoryTool(reportSvc, userID),
		listTransactionsTool(reportSvc, userID),
	}
}

func accountBalancesTool(accountSvc AccountService, userID int64) financialTool {
	return financialTool{
		schema: ollamaTool{
			Type: "function",
			Function: ollamaToolFn{
				Name:        "get_account_balances",
				Description: "Retorna o saldo atual de todas as contas do usuário.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
			},
		},
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

func spendingByCategoryTool(reportSvc ReportsService, userID int64) financialTool {
	return financialTool{
		schema: ollamaTool{
			Type: "function",
			Function: ollamaToolFn{
				Name:        "get_spending_by_category",
				Description: "Retorna os gastos (despesas) agrupados por categoria em um período.",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"from":{"type":"string","description":"Data inicial no formato YYYY-MM-DD"},
						"to":{"type":"string","description":"Data final no formato YYYY-MM-DD"}
					},
					"required":["from","to"]
				}`),
			},
		},
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

func monthlyComparisonTool(reportSvc ReportsService, userID int64) financialTool {
	return financialTool{
		schema: ollamaTool{
			Type: "function",
			Function: ollamaToolFn{
				Name:        "get_monthly_comparison",
				Description: "Retorna comparativo mensal de receitas e despesas em um período.",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"from":{"type":"string","description":"Data inicial no formato YYYY-MM-DD"},
						"to":{"type":"string","description":"Data final no formato YYYY-MM-DD"}
					},
					"required":["from","to"]
				}`),
			},
		},
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
			var sb strings.Builder
			months := [...]string{"Jan", "Fev", "Mar", "Abr", "Mai", "Jun", "Jul", "Ago", "Set", "Out", "Nov", "Dez"}
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
		schema: ollamaTool{
			Type: "function",
			Function: ollamaToolFn{
				Name:        "get_balance_history",
				Description: "Retorna a evolução do saldo patrimonial mês a mês em um período.",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"from":{"type":"string","description":"Data inicial no formato YYYY-MM-DD"},
						"to":{"type":"string","description":"Data final no formato YYYY-MM-DD"}
					},
					"required":["from","to"]
				}`),
			},
		},
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
			var sb strings.Builder
			months := [...]string{"Jan", "Fev", "Mar", "Abr", "Mai", "Jun", "Jul", "Ago", "Set", "Out", "Nov", "Dez"}
			fmt.Fprintf(&sb, "Evolução do saldo (%s a %s):\n", from.Format("02/01/2006"), to.Format("02/01/2006"))
			for _, p := range points {
				month := fmt.Sprintf("%s/%d", months[p.Month.Month()-1], p.Month.Year())
				fmt.Fprintf(&sb, "- %s: %s\n", month, utils.FormatMoney(p.Balance))
			}
			return sb.String(), nil
		},
	}
}

func listTransactionsTool(reportSvc ReportsService, userID int64) financialTool {
	return financialTool{
		schema: ollamaTool{
			Type: "function",
			Function: ollamaToolFn{
				Name:        "list_transactions",
				Description: "Lista transações com filtros opcionais de período, tipo (debit/credit) e limite.",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"from":{"type":"string","description":"Data inicial YYYY-MM-DD (opcional)"},
						"to":{"type":"string","description":"Data final YYYY-MM-DD (opcional)"},
						"direction":{"type":"string","enum":["debit","credit"],"description":"debit=despesa, credit=receita (opcional)"},
						"limit":{"type":"integer","description":"Número de resultados (padrão 10, máximo 50)"}
					}
				}`),
			},
		},
		handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct {
				From      string `json:"from"`
				To        string `json:"to"`
				Direction string `json:"direction"`
				Limit     int32  `json:"limit"`
			}
			_ = json.Unmarshal(args, &p)

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
