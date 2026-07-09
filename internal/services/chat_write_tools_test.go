package service

import (
	"testing"

	"finops/internal/store"
)

func TestResolveAccount(t *testing.T) {
	accounts := []store.Account{
		{ID: 1, Name: "Nubank"},
		{ID: 2, Name: "Itaú Corrente"},
	}

	// nome vazio com múltiplas contas → ambíguo
	if _, ok := resolveAccount(accounts, ""); ok {
		t.Error("nome vazio com 2 contas deveria ser ambíguo")
	}
	// nome vazio com 1 conta → resolve
	if a, ok := resolveAccount(accounts[:1], ""); !ok || a.ID != 1 {
		t.Error("nome vazio com 1 conta deveria resolver")
	}
	// match exato normalizado
	if a, ok := resolveAccount(accounts, "nubank"); !ok || a.ID != 1 {
		t.Errorf("'nubank' deveria casar com Nubank, got ok=%v", ok)
	}
	// contains parcial
	if a, ok := resolveAccount(accounts, "itaú"); !ok || a.ID != 2 {
		t.Errorf("'itaú' deveria casar com Itaú Corrente, got ok=%v id=%v", ok, a.ID)
	}
	// inexistente
	if _, ok := resolveAccount(accounts, "bradesco"); ok {
		t.Error("'bradesco' não deveria casar")
	}
}

func TestResolveCategory(t *testing.T) {
	cats := []store.Category{
		{ID: 10, Name: "Alimentação"},
		{ID: 20, Name: "Transporte"},
	}
	if id, _, ok := resolveCategory(cats, "alimentação"); !ok || id != 10 {
		t.Errorf("deveria casar Alimentação, got ok=%v id=%v", ok, id)
	}
	if id, _, ok := resolveCategory(cats, "transporte"); !ok || id != 20 {
		t.Errorf("deveria casar Transporte, got ok=%v id=%v", ok, id)
	}
	if _, _, ok := resolveCategory(cats, "lazer"); ok {
		t.Error("'lazer' não deveria casar")
	}
}
