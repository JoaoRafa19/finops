# Excluir categoria renderizada via HTMX

- ID: 009
- Status: done
- Prioridade: P1
- Tipo: frontend
- Complexidade: 2
- Épico: Fechar Core Financeiro MVP
- Feature: Categorias do core financeiro

## Objetivo

Permitir excluir uma categoria diretamente da lista já renderizada, sem recarregar a home inteira.

## Contexto

- Arquivos principais: `internal/web/templates/category_template.templ`, `internal/modules/category/web/controller.go`, `internal/web/page_router.go`
- Dependências: HTMX, sessão autenticada e service de categoria
- Risco: UX inconsistente ou item permanecer na tela após exclusão

## Alterações esperadas

- [x] Adicionar botão de excluir no card da categoria
- [x] Remover o `li` da lista com `hx-target="closest li"` e `hx-swap="outerHTML"`
- [x] Expor rota HTTP para exclusão da categoria

## Critério de aceite

- [x] Categoria pode ser excluída a partir da lista renderizada
- [x] O item some da tela sem recarregar a página inteira

## Validação

- [x] `go test ./...`
- [ ] validação manual

## Observações

O fluxo retorna erro no próprio painel quando a exclusão falha.
