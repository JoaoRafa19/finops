package service

import (
	"context"
	"database/sql"
	"errors"
	"finops/internal/store"
	"strconv"
	"strings"
	"time"
)

type CategorySpend struct {
	CategoryName string
	Total        float64
}

type MonthlyRow struct {
	Month    time.Time
	Income   float64
	Expenses float64
}

type BalancedHistory struct {
	Month   time.Time
	Balance float64
}

type CategoryMonthSpend struct {
	Month        time.Time
	CategoryName string
	Total        float64
}

type TopExpenseItem struct {
	AccountName  string
	CategoryName string
	PostedOn     time.Time
	Description  string
	Amount       float64
}

type TransactionFilter struct {
	AccountID   *int64
	CategoryID  *int64
	Direction   *string
	Description *string
	FromDate    *time.Time
	ToDate      *time.Time
	Limit       int32
	Offset      int32
}

type ReportsService interface {
	SpendingByCategory(ctx context.Context, userID int64, from, to time.Time) ([]CategorySpend, error)
	MonthlyComparison(ctx context.Context, userID int64, from, to time.Time) ([]MonthlyRow, error)
	BalancedHistory(ctx context.Context, userID int64, from, to time.Time) ([]BalancedHistory, error)
	ListFiltered(ctx context.Context, userID int64, filter TransactionFilter) ([]TransactionListItem, error)
	SpendingTrend(ctx context.Context, userID int64, from, to time.Time) ([]CategoryMonthSpend, error)
	TopExpenses(ctx context.Context, userID int64, from, to time.Time, limit int32) ([]TopExpenseItem, error)
}

type PGReportService struct {
	db *store.Queries
}

// BalancedHistory implements [ReportsService].
func (p *PGReportService) BalancedHistory(ctx context.Context, userID int64, from time.Time, to time.Time) ([]BalancedHistory, error) {
	ws, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	anchor, err := p.db.GetBalanceBefore(ctx, store.GetBalanceBeforeParams{
		WorkspaceID: ws.ID,
		PostedOn:    from,
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		anchor = 0
	}

	running := anchor

	data, err := p.db.GetBalanceHistoryByMonth(ctx, store.GetBalanceHistoryByMonthParams{
		WorkspaceID: ws.ID,
		PostedOn:    from,
		PostedOn_2:  to,
	})

	if err != nil {
		return nil, err
	}

	var history []BalancedHistory

	for _, i := range data {
		running += i.NetChange
		history = append(history, BalancedHistory{
			Month:   i.Month,
			Balance: running,
		})
	}

	return history, nil

}

// ListFiltered implements [ReportsService].
func (p *PGReportService) ListFiltered(ctx context.Context, userID int64, filter TransactionFilter) ([]TransactionListItem, error) {
	ws, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	accountId := sql.NullInt64{}
	if filter.AccountID != nil {
		accountId = sql.NullInt64{
			Int64: *filter.AccountID,
			Valid: true,
		}
	}

	CategoryID := sql.NullInt64{}
	if filter.CategoryID != nil {
		CategoryID = sql.NullInt64{
			Int64: *filter.CategoryID,
			Valid: true,
		}
	}

	Direction := sql.NullString{}
	if filter.Direction != nil {
		Direction = sql.NullString{
			String: *filter.Direction,
			Valid:  true,
		}
	}

	Description := sql.NullString{}
	if filter.Description != nil && *filter.Description != "" {
		Description = sql.NullString{
			String: "%" + strings.ToLower(*filter.Description) + "%",
			Valid:  true,
		}
	}

	FromDate := sql.NullTime{}
	if filter.FromDate != nil {
		FromDate = sql.NullTime{
			Time:  *filter.FromDate,
			Valid: true,
		}
	}

	ToDate := sql.NullTime{}
	if filter.ToDate != nil {
		ToDate = sql.NullTime{
			Time:  *filter.ToDate,
			Valid: true,
		}
	}

	filtered, err := p.db.ListTransactionsFiltered(ctx, store.ListTransactionsFilteredParams{
		WorkspaceID: ws.ID,
		Limit:       filter.Limit,
		Offset:      filter.Offset,
		AccountID:   accountId,
		CategoryID:  CategoryID,
		Direction:   Direction,
		Description: Description,
		FromDate:    FromDate,
		ToDate:      ToDate,
	})

	if err != nil {
		return nil, err
	}

	var transactions []TransactionListItem

	for _, t := range filtered {

		amnt, err := strconv.ParseFloat(t.Amount, 64)
		if err != nil {
			continue
		}

		transactions = append(transactions, TransactionListItem{
			ID:          t.ID,
			AccountID:   t.AccountID,
			AccountName: t.AccountName,
			PostedOn:    t.PostedOn,
			Description: t.Description,
			Amount:      amnt,
			Direction:   t.Direction,
			Currency:    t.Currency,
			Source:      t.Source,
			Category:    t.CategoryName,
		})
	}

	return transactions, err
}

// MonthlyComparison implements [ReportsService].
func (p *PGReportService) MonthlyComparison(ctx context.Context, userID int64, from time.Time, to time.Time) ([]MonthlyRow, error) {
	ws, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	data, err := p.db.GetMonthlyComparison(ctx, store.GetMonthlyComparisonParams{
		WorkspaceID: ws.ID,
		PostedOn:    from,
		PostedOn_2:  to,
	})

	if err != nil {
		return nil, err
	}

	var monthlyRow []MonthlyRow

	for _, i := range data {
		monthlyRow = append(monthlyRow, MonthlyRow{
			Month:    i.Month,
			Income:   i.Income,
			Expenses: i.Expenses,
		})
	}

	return monthlyRow, nil

}

// SpendingByCategory implements [ReportsService].
func (p *PGReportService) SpendingByCategory(ctx context.Context, userID int64, from time.Time, to time.Time) ([]CategorySpend, error) {
	ws, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	data, err := p.db.GetSpendingByCategoryForPeriod(
		ctx,
		store.GetSpendingByCategoryForPeriodParams{
			WorkspaceID: ws.ID,
			PostedOn:    from,
			PostedOn_2:  to,
		},
	)

	if err != nil {
		return nil, err
	}

	var history []CategorySpend

	for _, i := range data {
		history = append(history, CategorySpend{
			CategoryName: i.CategoryName,
			Total:        i.Total,
		})
	}

	return history, nil

}

// SpendingTrend implements [ReportsService].
func (p *PGReportService) SpendingTrend(ctx context.Context, userID int64, from, to time.Time) ([]CategoryMonthSpend, error) {
	ws, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	rows, err := p.db.GetSpendingByCategoryByMonth(ctx, store.GetSpendingByCategoryByMonthParams{
		WorkspaceID: ws.ID,
		PostedOn:    from,
		PostedOn_2:  to,
	})
	if err != nil {
		return nil, err
	}
	out := make([]CategoryMonthSpend, len(rows))
	for i, r := range rows {
		out[i] = CategoryMonthSpend{Month: r.Month, CategoryName: r.CategoryName, Total: r.Total}
	}
	return out, nil
}

// TopExpenses implements [ReportsService].
func (p *PGReportService) TopExpenses(ctx context.Context, userID int64, from, to time.Time, limit int32) ([]TopExpenseItem, error) {
	ws, err := p.db.GetWorkSpaceByOwnerUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	rows, err := p.db.GetTopExpenses(ctx, store.GetTopExpensesParams{
		WorkspaceID: ws.ID,
		PostedOn:    from,
		PostedOn_2:  to,
		Limit:       limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]TopExpenseItem, len(rows))
	for i, r := range rows {
		out[i] = TopExpenseItem{
			AccountName:  r.AccountName,
			CategoryName: r.CategoryName,
			PostedOn:     r.PostedOn,
			Description:  r.Description,
			Amount:       r.Amount,
		}
	}
	return out, nil
}

func NewPGReportService(q *store.Queries) ReportsService {
	return &PGReportService{
		db: q,
	}
}
