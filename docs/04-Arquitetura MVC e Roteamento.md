# Decisões arquiteturais atuais (MVC + Services + roteamento API/Web)

Data: 2026-03-03

## Objetivo

Registrar as decisões de arquitetura adotadas na implementação atual para manter consistência entre código e documentação.

## Decisões

1. Padrão base: MVC simplificado
- Model: `internal/store` (queries SQL + código gerado por `sqlc`).
- Service: `internal/service` (regras de negócio e orquestração de integrações).
- Controller: `internal/controllers` (camada HTTP de request/response).

2. Regra de responsabilidade
- Controller não acessa SQL diretamente.
- Service contém regra de negócio.
- Model/persistência não conhece HTTP.

3. Estrutura de controllers por tipo de interface
- `internal/controllers/api`: controllers de endpoints de API.
- `internal/controllers/web`: controllers de páginas (HTMX/templ).

4. Roteamento separado por contexto
- Router raiz: `internal/web/router.go`.
- Router de API: `internal/web/api_router.go`.
- Router de páginas: `internal/web/page_router.go`.

5. Prefixos de rota
- API em `/api/*` (ex.: `GET /api/health`).
- Páginas sem prefixo (ex.: `GET /`).

6. Middleware HTTP
- Middlewares ficam em `internal/web/middleware`.
- Middleware global atual: logging.

7. Simplificações adotadas
- Removida a pasta `internal/httpx`.
- Removidos contratos na pasta `domain` para evitar conflito com o padrão escolhido.
- Mantida uma camada `service` direta, sem abstrações extras neste estágio.

## Estrutura de pastas (recorte)

```text
internal/
  app/
  controllers/
    api/
    web/
  service/
  store/
  web/
    middleware/
    api_router.go
    page_router.go
    router.go
```

## Próximas regras para novas funcionalidades

- Toda nova regra de negócio deve nascer em `internal/service`.
- Todo novo endpoint deve ser adicionado no router correto:
  - API -> `api_router.go`
  - Página/HTMX -> `page_router.go`
- Toda consulta SQL nova deve ser adicionada em `internal/store/queries` e gerada com `sqlc`.
