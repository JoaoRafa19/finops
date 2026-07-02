package service

import "time"

// MockDashboard retorna dados fictícios para o tour guiado.
// Nada é persistido: ao sair do tour, o dashboard volta a refletir o banco.
func MockDashboard() DashboardSummary {
	now := time.Now()
	month := func(offset int) time.Time {
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, offset, 0)
	}

	return DashboardSummary{
		TotalBalance:  4200.00,
		Income:        7500.00,
		Expenses:      3300.00,
		Savings:       4200.00,
		IncomeDelta:   5.2,
		ExpensesDelta: -3.1,
		HasDelta:      true,
		Spending: []CategorySpend{
			{CategoryName: "Alimentação", Total: 1240.50},
			{CategoryName: "Transporte", Total: 645.00},
			{CategoryName: "Lazer", Total: 480.00},
			{CategoryName: "Assinaturas", Total: 177.80},
		},
		CashFlow: []MonthlyRow{
			{Month: month(-2), Income: 7500, Expenses: 3900},
			{Month: month(-1), Income: 7500, Expenses: 3400},
			{Month: month(0), Income: 7500, Expenses: 3300},
		},
		BalanceHistory: []BalancedHistory{
			{Month: month(-2), Balance: 1100},
			{Month: month(-1), Balance: 2600},
			{Month: month(0), Balance: 4200},
		},
		TopCategories: []TopCategoryItem{
			{Name: "Alimentação", Total: 1240.50, Delta: 8, HasDelta: true},
			{Name: "Transporte", Total: 645.00, Delta: -12, HasDelta: true},
			{Name: "Lazer", Total: 480.00, Delta: 3, HasDelta: true},
		},
		Insights: []string{
			"Você economizou R$ 4.200,00 este período — 56% da sua renda.",
			"Gastos com Transporte caíram 12% em relação ao período anterior.",
		},
	}
}
