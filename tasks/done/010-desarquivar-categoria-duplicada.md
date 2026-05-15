# Desarquivar categoria ao recriar nome duplicado

- ID: 010
- Status: done
- Prioridade: P1
- Tipo: backend
- Complexidade: 2
- Épico: Fechar Core Financeiro MVP
- Feature: Categorias do core financeiro

## Objetivo

Evitar erro para categorias arquivadas com o mesmo nome, reaproveitando o registro existente.

## Contexto

- Arquivos principais: `internal/services/categories_service.go`, `internal/store/queries/categories.sql`
- Dependências: constraint única `(workspace_id, name)` e campo `archived`
- Risco: duplicidade bloquear recriação de categorias já removidas da UI

## Alterações esperadas

- [x] Detectar erro `23505` na criação
- [x] Buscar categoria existente por nome no workspace
- [x] Desarquivar a categoria quando o nome duplicado estiver arquivado

## Critério de aceite

- [x] Recriar categoria arquivada volta a ativar o registro anterior
- [x] Categoria já ativa com o mesmo nome continua retornando erro

## Validação

- [x] `go test ./...`
- [ ] validação manual

## Observações

O fluxo usa `GetCategoryByWorkspaceAndName` antes de chamar `UnarchiveCategory`.
