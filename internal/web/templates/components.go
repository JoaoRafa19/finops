package templates

import (
	"finops/internal/store"
	"fmt"
)

type SelectOption struct {
	Value    string
	Label    string
	Selected bool
}

type AccountDTO struct {
	TotalBalance float64
	Accounts     []AccountItemDTO
}

type AccountItemDTO struct {
	ID             int64
	Name           string
	Type           string
	Currency       string
	CurrentBalance float64
}

type TransactionFormState struct {
	Description  string
	Amount       string
	AccountID    string
	CategoryID   string
	PostedOn     string
	Direction    string
	ErrorMessage string
}

type TransferFormState struct {
	FromAccountID string
	ToAccountID   string
	Amount        string
	PostedOn      string
	ErrorMessage  string
}

func NewTransactionFormState() TransactionFormState {
	return TransactionFormState{}
}

func NewTransactionFormStateFromValues(description, amount, accountID, categoryID, postedOn, direction, errorMessage string) TransactionFormState {
	return TransactionFormState{
		Description:  description,
		Amount:       amount,
		AccountID:    accountID,
		PostedOn:     postedOn,
		Direction:    direction,
		CategoryID:   categoryID,
		ErrorMessage: errorMessage,
	}
}

func NewTransferFormState() TransferFormState {
	return TransferFormState{}
}

func NewTransferFormStateFromValues(fromAccountID, toAccountID, amount, postedOn, errorMessage string) TransferFormState {
	return TransferFormState{
		FromAccountID: fromAccountID,
		ToAccountID:   toAccountID,
		Amount:        amount,
		PostedOn:      postedOn,
		ErrorMessage:  errorMessage,
	}
}

func buildTransactionDirectionOptions(selected string) []SelectOption {
	return []SelectOption{
		{Value: "", Label: "Selecione", Selected: selected == ""},
		{Value: "credit", Label: "Entrada", Selected: selected == "credit"},
		{Value: "debit", Label: "Saida", Selected: selected == "debit"},
	}
}

func buildCategoryOptions(categories []store.Category, selected string) []SelectOption {
	options := make([]SelectOption, 0, len(categories)+1)
	options = append(options, SelectOption{
		Value:    "",
		Label:    "Sem categoria",
		Selected: true,
	})

	for _, cat := range categories {
		value := fmt.Sprintf("%d", cat.ID)
		options = append(options, SelectOption{
			Value:    fmt.Sprintf("%d", cat.ID),
			Label:    cat.Name,
			Selected: value == selected,
		})
	}

	return options
}

func buildAccountOptions(accounts []store.Account, selected string) []SelectOption {
	options := make([]SelectOption, len(accounts))
	for i, acc := range accounts {
		value := fmt.Sprintf("%d", acc.ID)
		options[i] = SelectOption{
			Value:    fmt.Sprintf("%d", acc.ID),
			Label:    acc.Name,
			Selected: value == selected,
		}
	}

	return options
}

type AccountFormState struct {
	AccountID      int64
	Name           string
	Type           string
	Currency       string
	CurrentBalance string
	ErrorMessage   string
	IsEdit         bool
}

type CategoryFormState struct {
	Name         string
	Kind         string
	ErrorMessage string
	Oob          bool
}

func buildCategoryKindOptions(selected string) []SelectOption {
	return []SelectOption{
		{Value: "", Label: "Selecione", Selected: selected == ""},
		{Value: "expense", Label: "Despesa", Selected: selected == "expense"},
		{Value: "income", Label: "Receita", Selected: selected == "income"},
		{Value: "transfer", Label: "Transferencia", Selected: selected == "transfer"},
	}
}

func categoryKindLabel(kind string) string {
	switch kind {
	case "expense":
		return "Despesa"
	case "income":
		return "Receita"
	case "transfer":
		return "Transferencia"
	default:
		return kind
	}
}

func NewCategoryFormState() CategoryFormState {
	return CategoryFormState{}
}

func NewCategoryFormStateFromValues(name, kind, errorMessage string) CategoryFormState {
	return CategoryFormState{
		Name:         name,
		Kind:         kind,
		ErrorMessage: errorMessage,
	}
}

func primaryButtonClass(fullWidth bool) string {
	className := "inline-flex items-center justify-center rounded-xl bg-finops-900 px-4 py-3 text-sm font-extrabold uppercase tracking-[0.16em] text-white shadow-lg shadow-finops-900/20 transition hover:bg-finops-800 focus:outline-none focus:ring-4 focus:ring-finops-400/25"
	if fullWidth {
		return className + " w-full"
	}

	return className + " px-5"
}

func buildAccountTypeOptions(selected string) []SelectOption {
	return []SelectOption{
		{Value: "", Label: "Selecione", Selected: selected == ""},
		{Value: "checking", Label: "Conta corrente", Selected: selected == "checking"},
		{Value: "savings", Label: "Poupanca", Selected: selected == "savings"},
		{Value: "cash", Label: "Dinheiro", Selected: selected == "cash"},
		{Value: "credit_card", Label: "Cartao", Selected: selected == "credit_card"},
		{Value: "investment", Label: "Investimento", Selected: selected == "investment"},
	}
}

func accountFormSubmitLabel(form AccountFormState) string {
	if !form.IsEdit {
		return "Cadastrar conta"
	}

	return "Atualizar"
}

func accountFormAction(form AccountFormState) string {
	if !form.IsEdit {
		return "/accounts"
	}

	return fmt.Sprintf("/accounts/%d", form.AccountID)
}

func accountFormTitle(form AccountFormState) string {
	if !form.IsEdit {
		return "Cadastrar nova conta"
	}

	return "Editar conta"
}

func accountFormDescription(form AccountFormState) string {
	if !form.IsEdit {
		return "Cadastre a primeira conta para iniciar lancamentos e transferencias."
	}

	return "Salvar alteracoes"
}

func NewCreateAccountFormState() AccountFormState {
	return AccountFormState{
		Currency:       "BRL",
		CurrentBalance: "0.00",
	}
}

func NewCreateAccountFormStateFromValues(name, accountType, currency, currentBalance, errorMessage string) AccountFormState {
	if currency == "" {
		currency = "BRL"
	}
	if currentBalance == "" {
		currentBalance = "0.00"
	}

	return AccountFormState{
		Name:           name,
		Type:           accountType,
		Currency:       currency,
		CurrentBalance: currentBalance,
		ErrorMessage:   errorMessage,
	}
}

func NewEditAccountFormState(account store.Account, currentBalance float64, errorMessage string) AccountFormState {
	return AccountFormState{
		AccountID:      account.ID,
		Name:           account.Name,
		Type:           account.Type,
		Currency:       account.Currency,
		CurrentBalance: fmt.Sprintf("%.2f", currentBalance),
		ErrorMessage:   errorMessage,
		IsEdit:         true,
	}
}

func NewEditAccountFormStateFromValues(accountID int64, name, accountType, currency, currentBalance, errorMessage string) AccountFormState {
	if currentBalance == "" {
		currentBalance = "0.00"
	}

	return AccountFormState{
		AccountID:      accountID,
		Name:           name,
		Type:           accountType,
		Currency:       currency,
		CurrentBalance: currentBalance,
		ErrorMessage:   errorMessage,
		IsEdit:         true,
	}
}
