# Decisões arquiteturais atuais (MVC + Services + roteamento API/Web)

Data: 2026-03-03

## Objetivo

Registrar as decisões de arquitetura adotadas na implementação atual para manter consistência entre código e documentação.

## Decisões

1. Padrão base: monólito modular por domínio
- Model: `internal/store` (queries SQL + código gerado por `sqlc`).
- Service: `internal/services` (regras de negócio e orquestração de integrações).
- HTTP por domínio: `internal/modules/<dominio>/{web|api}`.
- Composição transversal: `internal/web` (router, middleware) e `internal/app` (bootstrap).

2. Regra de responsabilidade
- Controller não acessa SQL diretamente.
- Service contém regra de negócio.
- Model/persistência não conhece HTTP.

3. Estrutura HTTP por domínio e por interface
- `internal/modules/*/web`: handlers de páginas e fluxos HTMX/templ.
- `internal/modules/*/api`: handlers de endpoints JSON.

4. Roteamento separado por contexto
- Router raiz: `internal/web/router.go`.
- Router de API: `internal/web/api_router.go`.
- Router de páginas: `internal/web/page_router.go`.

5. Prefixos de rota
- API em `/api/*` (ex.: `GET /api/health`).
- Páginas sem prefixo (ex.: `GET /`).

6. Middleware e renderização compartilhados
- Middlewares ficam em `internal/web/middleware`.
- Renderização HTML compartilhada fica em `internal/web/render`.
- Middleware global atual: request ID, logging, sessão e CSRF.

7. Simplificações adotadas
- Mantida uma camada `services` direta, sem abstrações extras entre service e store neste estágio.
- O frontend continua server-rendered com `templ`/HTMX dentro do monólito.
- A API segue separada por prefixo e por pacote, sem exigir deploy separado.

## Estrutura de pastas (recorte)

```text
internal/
  app/
  modules/
    auth/
    accounts/
    health/
    home/
    onboarding/
    transactions/
  services/
  store/
  web/
    middleware/
    render/
    api_router.go
    page_router.go
    router.go
```

## Próximas regras para novas funcionalidades

- Toda nova regra de negócio deve nascer em `internal/services`.
- Todo novo endpoint deve ser adicionado no router correto:
  - API -> módulo em `internal/modules/<dominio>/api` e registro em `api_router.go`
  - Página/HTMX -> módulo em `internal/modules/<dominio>/web` e registro em `page_router.go`
- Toda consulta SQL nova deve ser adicionada em `internal/store/queries` e gerada com `sqlc`.
