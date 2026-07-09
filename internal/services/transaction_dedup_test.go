package service

import "testing"

func TestSimilarDescription(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"Uber", "uber", true},                              // case
		{"Uber *trip 123", "UBER TRIP", true},               // contido + pontuação
		{"Mercado Extra", "Extra Mercado", true},            // ordem
		{"iFood restaurante", "iFood", true},                // contido
		{"Padaria Pão Quente", "Posto Shell", false},        // diferentes
		{"Netflix", "Spotify", false},                       // diferentes
		{"Compra cartão 1234", "Compra cartão 9999", false}, // 2/3 tokens → jaccard 0.5? checar
	}
	for _, c := range cases {
		if got := similarDescription(c.a, c.b); got != c.want {
			t.Errorf("similarDescription(%q,%q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestSimilarDescriptionEmpty(t *testing.T) {
	if !similarDescription("", "") {
		t.Error("descrições vazias iguais devem ser similares")
	}
	if similarDescription("algo", "") {
		t.Error("uma vazia e outra não não são similares")
	}
}
