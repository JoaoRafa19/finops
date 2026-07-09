package service

import "testing"

func TestExtractInlineToolCall(t *testing.T) {
	tools := []financialTool{
		{schema: tool("stage_transaction", "", params(nil))},
		{schema: tool("get_account_balances", "", params(nil))},
	}

	cases := []struct {
		in       string
		wantName string
		wantOK   bool
	}{
		{`<tool_call>{"name": "stage_transaction", "arguments": {"amount": 50}}</tool_call>`, "stage_transaction", true},
		{" Ronaldo\n{\"name\": \"stage_transaction\", \"arguments\": {\"description\": \"livro\"}}", "stage_transaction", true},
		{`rored\n{"name":"get_account_balances","arguments":{}}`, "get_account_balances", true},
		{"Olá! Seu saldo é R$100.", "", false},                        // texto normal, não vira tool
		{`{"name":"ferramenta_inexistente","arguments":{}}`, "", false}, // nome desconhecido
	}
	for _, c := range cases {
		name, _, ok := extractInlineToolCall(c.in, tools)
		if ok != c.wantOK || name != c.wantName {
			t.Errorf("extractInlineToolCall(%q) = (%q,%v), quero (%q,%v)", c.in, name, ok, c.wantName, c.wantOK)
		}
	}
}
