package service

import (
	"context"
	"encoding/json"
)

type Tool struct {
	Name        string
	Description string
	Schema      json.RawMessage
	Handler     func(ctx context.Context, args json.RawMessage) (any, error)
}

func FinancialTool(svc ReportsService, userID int64) []Tool {
	return []Tool{
		{Name: "get_spending_by_category", Description: "get the spending amount filtered by category name"},
	}
}
