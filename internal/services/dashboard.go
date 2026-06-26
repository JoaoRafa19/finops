package service

import (
	"fmt"
	"math"
	"time"
)

type DashboardSummary struct {
	TotalBalance   float64
	Income         float64
	Expenses       float64
	Savings        float64
	IncomeDelta    float64
	ExpensesDelta  float64
	HasDelta       bool
	Spending       []CategorySpend
	CashFlow       []MonthlyRow
	BalanceHistory []BalancedHistory
	TopCategories  []TopCategoryItem
	Insights       []string
}

type TopCategoryItem struct {
	Name     string
	Total    float64
	Delta    float64
	HasDelta bool
}

// BuildDashboard agrega os dados das chamadas de serviço num único DashboardSummary.
func BuildDashboard(
	totalBalance float64,
	spending []CategorySpend,
	cashflow []MonthlyRow,
	balanceHistory []BalancedHistory,
	prevSpending []CategorySpend,
	prevCashflow []MonthlyRow,
) DashboardSummary {
	var income, expenses float64
	for _, m := range cashflow {
		income += m.Income
		expenses += m.Expenses
	}

	var prevIncome, prevExpenses float64
	for _, m := range prevCashflow {
		prevIncome += m.Income
		prevExpenses += m.Expenses
	}

	hasDelta := prevIncome > 0 || prevExpenses > 0

	var incomeDelta, expensesDelta float64
	if hasDelta && prevIncome > 0 {
		incomeDelta = ((income - prevIncome) / prevIncome) * 100
	}
	if hasDelta && prevExpenses > 0 {
		expensesDelta = ((expenses - prevExpenses) / prevExpenses) * 100
	}

	prevSpendMap := make(map[string]float64, len(prevSpending))
	for _, c := range prevSpending {
		prevSpendMap[c.CategoryName] = c.Total
	}

	top := spending
	if len(top) > 5 {
		top = top[:5]
	}
	topItems := make([]TopCategoryItem, len(top))
	for i, c := range top {
		item := TopCategoryItem{Name: c.CategoryName, Total: c.Total}
		if prev, ok := prevSpendMap[c.CategoryName]; ok && prev > 0 {
			item.Delta = ((c.Total - prev) / prev) * 100
			item.HasDelta = true
		}
		topItems[i] = item
	}

	return DashboardSummary{
		TotalBalance:   totalBalance,
		Income:         income,
		Expenses:       expenses,
		Savings:        income - expenses,
		IncomeDelta:    incomeDelta,
		ExpensesDelta:  expensesDelta,
		HasDelta:       hasDelta,
		Spending:       spending,
		CashFlow:       cashflow,
		BalanceHistory: balanceHistory,
		TopCategories:  topItems,
		Insights:       dashboardInsights(income, expenses, incomeDelta, expensesDelta, hasDelta, spending),
	}
}

// PreviousPeriod calcula o período imediatamente anterior com a mesma duração.
func PreviousPeriod(from, to time.Time) (time.Time, time.Time) {
	duration := to.Sub(from)
	prevTo := from.Add(-24 * time.Hour)
	prevFrom := prevTo.Add(-duration)
	return prevFrom, prevTo
}

func dashboardInsights(income, expenses, incomeDelta, expensesDelta float64, hasDelta bool, spending []CategorySpend) []string {
	savings := income - expenses
	var out []string

	if savings < 0 {
		out = append(out, fmt.Sprintf("⚠️ Suas despesas superaram as receitas em R$ %.2f.", math.Abs(savings)))
	} else if income > 0 {
		pct := (savings / income) * 100
		out = append(out, fmt.Sprintf("✅ Você economizou %.0f%% da renda neste período.", pct))
	}

	if hasDelta {
		if expensesDelta > 20 {
			out = append(out, fmt.Sprintf("📈 Gastos aumentaram %.0f%% em relação ao período anterior.", expensesDelta))
		} else if expensesDelta < -10 {
			out = append(out, fmt.Sprintf("📉 Gastos reduziram %.0f%% em relação ao período anterior.", math.Abs(expensesDelta)))
		}
		if incomeDelta > 10 {
			out = append(out, fmt.Sprintf("💰 Receitas cresceram %.0f%% em relação ao período anterior.", incomeDelta))
		}
	}

	if len(spending) > 0 && expenses > 0 {
		top := spending[0]
		pct := (top.Total / expenses) * 100
		out = append(out, fmt.Sprintf("🏆 %s representa %.0f%% das despesas.", top.CategoryName, pct))
	}

	return out
}
