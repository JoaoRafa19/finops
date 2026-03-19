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

type AccountFormState struct {
	AccountID      int64
	Name           string
	Type           string
	Currency       string
	OpeningBalance string
	OpeningDate    string
	ErrorMessage   string
	Inline         bool
	IsEdit         bool
}

func (s AccountFormState) ContainerID() string {
	if s.IsEdit {
		return fmt.Sprintf("account-%d", s.AccountID)
	}

	return "account-create-form"
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

func buildAccountOptions(accounts []store.Account) []SelectOption {
	options := make([]SelectOption, len(accounts))
	for i, acc := range accounts {
		options[i] = SelectOption{
			Value: fmt.Sprintf("%d", acc.ID),
			Label: acc.Name,
		}
	}
	return options
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

func accountFormTarget(form AccountFormState) string {
	if form.IsEdit {
		return "closest li"
	}

	return "#accounts-panel"
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
	return "Salvar alterações"
}

func accountFormClass(inline bool) string {
	if inline {
		return "grid gap-4 md:grid-cols-2"
	}
	return "mt-6 grid gap-4 md:grid-cols-2"
}

func NewCreateAccountFormState() AccountFormState {
	return AccountFormState{}
}

func NewCreateAccountFormStateFromValues(name, accountType, currency, openingBalance, openingDate, errorMessage string) AccountFormState {
	return AccountFormState{
		Name:           name,
		Type:           accountType,
		Currency:       currency,
		OpeningBalance: openingBalance,
		OpeningDate:    openingDate,
		ErrorMessage:   errorMessage,
	}

}

func NewEditAccountFormState(account store.Account, errorMessage string) AccountFormState {
	form := AccountFormState{
		AccountID:      account.ID,
		Name:           account.Name,
		Type:           account.Type,
		Currency:       account.Currency,
		OpeningBalance: fmt.Sprintf("%.2f", account.OpeningBalance),
		ErrorMessage:   errorMessage,
		Inline:         true,
		IsEdit:         true,
	}

	if account.OpeningDate.Valid {
		form.OpeningDate = account.OpeningDate.Time.Format("2006-01-02")
	}

	return form
}

func NewEditAccountFormStateFromValues(accountID int64, name, accountType, currency, openingBalance, openingDate, errorMessage string) AccountFormState {
	return AccountFormState{
		AccountID:      accountID,
		Name:           name,
		Type:           accountType,
		Currency:       currency,
		OpeningBalance: openingBalance,
		OpeningDate:    openingDate,
		ErrorMessage:   errorMessage,
		Inline:         true,
		IsEdit:         true,
	}
}
