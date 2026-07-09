package service

import "testing"

func TestConfirmDetection(t *testing.T) {
	aff := []string{"sim", "Sim.", " SIM! ", "pode gravar", "confirmo", "ok", "s"}
	neg := []string{"não", "nao", "cancela", "N", "esquece"}
	other := []string{"sim, mas muda a conta", "quanto gastei?", "coxinha de 9 reais"}

	for _, s := range aff {
		if !isAffirmation(s) {
			t.Errorf("isAffirmation(%q) = false, quero true", s)
		}
	}
	for _, s := range neg {
		if !isNegation(s) {
			t.Errorf("isNegation(%q) = false, quero true", s)
		}
	}
	for _, s := range other {
		if isAffirmation(s) || isNegation(s) {
			t.Errorf("%q não devia ser confirmação nem negação", s)
		}
	}
}
