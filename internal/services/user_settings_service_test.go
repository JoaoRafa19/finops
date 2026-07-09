package service

import "testing"

func TestValidHomeMode(t *testing.T) {
	cases := map[string]bool{
		HomeModeSimple:   true,
		HomeModeAdvanced: true,
		"":               false,
		"fancy":          false,
	}
	for mode, want := range cases {
		if got := validHomeMode(mode); got != want {
			t.Errorf("validHomeMode(%q) = %v, want %v", mode, got, want)
		}
	}
}
