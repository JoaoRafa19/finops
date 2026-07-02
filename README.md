# FinOps

Gestor financeiro pessoal — monólito Go com HTMX, Postgres, Redis (KeyValue no Render) e assistente IA.

**Produção:** https://finops-app.com.br

**Release atual:** [v0.1.0-beta](https://github.com/JoaoRafa19/finops/releases/tag/v0.1.0-beta) · [changelog](./CHANGELOG.md)

## Stack

- Go 1.25 + [templ](https://templ.guide) para renderização server-side
- HTMX 2 para SPA sem SPA framework
- Tailwind (CDN) + Chart.js (CDN) na UI
- Postgres 16 (Render Managed)
- Redis / Valkey (Render Key Value) para sessões, CSRF e rate limit
- Resend para e-mail transacional
- Groq (LLM cloud) para chat e classificação
- Deploy: Render (docker + native Go)

## Estrutura

```
cmd/
  finops/              # servidor web principal
  create-user/         # utilitário de criar usuário fora do fluxo
  backfill-embeddings/ # gera embeddings para transações antigas
internal/
  app/                 # bootstrap, config, DB, Redis, runtime
  services/            # regras de negócio (auth, transações, chat, IA...)
  store/               # sqlc gerado + migrations embarcadas
    migrations/        # arquivos .sql do tern (também rodam via MIGRATE_ON_START)
  modules/*/web/       # controllers HTTP por domínio
  web/
    middleware/        # auth, CSRF, rate limit, logging, error page, panic
    templates/         # templ (.templ → *_templ.go compilado)
    router.go          # composição de middlewares
    page_router.go     # rotas HTML
    api_router.go      # rotas JSON
  observability/       # logging, request ID
  models/              # DTOs de entrada/saída de API
```

## Rodar local

Pré-requisitos: Go 1.25+, Docker, [`templ`](https://github.com/a-h/templ#installation), [`sqlc`](https://sqlc.dev/), [`tern`](https://github.com/jackc/tern).

```bash
# 1. Sobe Postgres + Redis locais
make start_db

# 2. Aplica migrations (tern lê ./internal/store/migrations/tern.conf)
make migrate

# 3. Gera código (sqlc + templ)
make gen

# 4. Roda o servidor com hot-reload de templates
make front
```

Abre `http://localhost:8080` → signup → onboarding → tour.

## Variáveis de ambiente

Mínimo para local (via `.env` na raiz):

```env
# Postgres (tern usa DB_*; app usa DATABASE_URL)
DB_HOST=localhost
DB_PORT=5432
DB_NAME=finops
DB_USER=finops
DB_PASSWORD=finops
DATABASE_URL=postgres://finops:finops@localhost:5432/finops?sslmode=disable

# Redis
REDIS_URL=redis://localhost:6379

# LLM
LLM_BASE_URL=http://localhost:11434       # Ollama local, ou Groq/OpenAI compatível
LLM_API_KEY=ollama
LLM_MODEL=qwen2.5:3b

# Embeddings (opcional; se vazio, busca semântica desabilita silenciosamente)
EMBEDDING_BASE_URL=http://localhost:11434
EMBEDDING_MODEL=nomic-embed-text

# App
APP_BASE_URL=http://localhost:8080
COOKIE_SECURE=false
LOG_LEVEL=info

# E-mail (opcional em dev — usa Noop se vazio)
RESEND_API_KEY=
RESEND_FROM=Finops <noreply@localhost>
```

Em produção o Render provisiona `DATABASE_URL` e `REDIS_URL` automaticamente. `MIGRATE_ON_START=true` faz o binário rodar migrations no boot.

## Comandos úteis

```bash
make run                       # sobe o server
make test                      # roda go test ./...
make sqlc                      # regenera sqlc a partir das .sql
make gen                       # sqlc + templ
make migrate                   # aplica migrations pendentes (tern)
make backfill-embeddings       # gera embeddings pra transações classificadas antigas
```

## Deploy

Branch `main` → auto-deploy no Render (via `render.yaml` blueprint).

Fluxo de release: feature no `dev` (ou branch própria) → PR → merge no `main` → deploy automático. Tag `vX.Y.Z` no commit da main quando estabilizar → Release no GitHub com notas do CHANGELOG.

## Arquitetura em uma página

- **Sessão** em Redis (30min sliding TTL, 7d se remember-me), cookie HTTP-only + CSRF token separado.
- **Middleware chain**: RequestID → Logging → PanicRecover → NotFoundInterceptor → SessionLoader → CSRF → mux.
- **Renderização**: templ compila `.templ` em Go puro. HTMX faz swap de fragments. SPA-like sem framework JS.
- **Estados de UI**: barra global de progresso HTMX, empty states com CTA, modais com foco automático via `<dialog>` nativo.
- **IA**: chat usa function calling contra as próprias queries; classificação usa embeddings (768 dims) + cosine similarity em `float8[]` (sem pgvector).
- **Migrations**: escritas para `tern` local; embarcadas via `//go:embed` e rodadas no boot em prod.

## Roadmap

Concluído em v0.1.0-beta — ver [CHANGELOG](./CHANGELOG.md) completo.

Próximas frentes candidatas (v0.2.0+):

- **Metas de economia** — targets por período/categoria, tracking, alertas.
- **Chat com function calling ativo** — ações no BD via chat, não só leitura.
- **Insights proativos** — cron detecta anomalias e notifica.
- **2FA/TOTP** — segundo fator opcional.
- **Security headers** — CSP, HSTS, X-Frame-Options.
- **Auditoria** — log estruturado de ações sensíveis.
- **Embeddings em prod** — provider dedicado (Voyage AI free tier ou Ollama self-hosted).

## Licença

TBD.
