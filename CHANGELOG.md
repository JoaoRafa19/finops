# Changelog

Todas as mudanças relevantes do FinOps são registradas neste arquivo.

O formato segue [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/) e o projeto
adota [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.1.0-beta] — 2026-07-01

Primeira release beta. Sistema publicado em https://finops-app.com.br.

### Adicionado

**Autenticação e conta**
- Cadastro por e-mail e senha, login, logout, sessões em Redis com CSRF token.
- Recuperação de senha por e-mail (token uso único, TTL 1h).
- Verificação de e-mail obrigatória no primeiro acesso (token TTL 24h, gate por middleware).
- Reenvio manual de e-mail de confirmação.
- Rate limiting por IP nos endpoints públicos (login, signup, forgot, reset, verify).

**Core financeiro**
- Contas bancárias: cadastro, edição, exclusão com cascade das transações.
- Categorias: cadastro, edição inline em `/profile`, exclusão.
- Transações: cadastro, edição, exclusão, listagem com paginação.
- Transferências entre contas (duas transações vinculadas).
- Importação de extratos OFX e CSV.

**Relatórios**
- Gastos por categoria (barra horizontal).
- Receitas vs. despesas mensais (tabela + gráfico).
- Evolução do saldo consolidado.
- Filtros de período (last 24h, 7d, 15d, 30d, mês atual, ano atual, custom).
- Filtro por descrição (busca), categoria e direção nas transações.

**Dashboard (home)**
- Cards de saldo total, receitas, despesas, economia.
- Gráficos de linha (saldo), pizza (distribuição de gastos), barra (fluxo de caixa).
- Insights automáticos (variação mês a mês, categoria dominante).
- Ações rápidas: registrar transação, transferir, importar, nova conta, nova categoria.

**IA / Assistente**
- Chat com histórico persistente por usuário (via `chat_messages`).
- Function calling para dados financeiros: `get_summary`, `get_spending_by_category`, `get_recent_transactions`, `list_accounts`.
- Classificação automática de transações via tool calling + embeddings semânticos (cosine similarity, SQL puro sem pgvector).
- Widget de chat redimensionável, dicas de uso em modal, tamanho de fonte configurável em `/profile`.

**Perfil e configurações**
- Página `/profile` com informações da conta, categorias (CRUD inline), contas bancárias (exclusão com cascade), preferências do chat.
- Link direto para reset de senha.

**Onboarding**
- Tour guiado (driver.js) nas primeiras visitas — saldo, contas, transações, relatórios, IA.
- Criação automática de workspace no primeiro login.

**Navegação (SPA)**
- Navegação HTMX com slide-transition entre páginas (mantém sidenav sem full reload).
- Sidenav mobile com hamburger + overlay para telas &lt; 640px.
- Barra global de progresso HTMX no topo.

### Segurança

- Middleware CSRF com validação por sessão + Origin check para POSTs sem sessão.
- Rate limiting Redis-backed por IP com fixed window.
- Cookie de sessão HTTP-only, Secure em produção, SameSite=Lax.
- bcrypt para senhas.
- Tokens (reset, verify) gerados com `crypto/rand`, comparação em tempo constante para CSRF.

### UX / Acessibilidade

- `aria-label` em todos os botões-só-de-ícone (categorias, contas, transações, chat, modais).
- Empty states com CTA (accounts, categorias).
- Medidor visual de força de senha no signup.
- Charts com `aria-label` descritivos.
- Páginas estilizadas para 404, 500 (via `PanicRecover`) e 429.
- Feedback de loading global durante requests HTMX.
- Modais bloqueados (`TransactionModalBlocked`, `TransferModalBlocked`) exibem CTA para criar pré-requisito.

### Infraestrutura

- Migrations embarcadas no binário (`internal/store/migrate.go`) — auto-aplicadas no boot quando `MIGRATE_ON_START=true`. Compatível com o tracker do `tern` para uso local.
- Deploy em Render (Postgres + Key Value + Web Service).
- Envio de e-mail via Resend HTTPS API (fallback SMTP e Noop também suportados).
- Dockerfile Go 1.25 (também usado o buildpack nativo Go do Render).
- Índices em `transactions(workspace_id, posted_on DESC)`, `(workspace_id, category_id)`, e parcial em `category_id IS NULL`.

### Observabilidade

- Logs estruturados em JSON via `log/slog`, request ID por requisição.
- Middleware de logging com status, duração e path.
- Log de bootstrap defensivo: `db_connect_attempt`, `redis_connect_attempt`, `email_service_*`, `migrate_on_start_*`.

### Comandos utilitários

- `cmd/create-user` — cria usuário admin fora do fluxo web.
- `cmd/backfill-embeddings` — gera embeddings para transações já classificadas (idempotente).

### Testes

- Cobertura em `middleware` (auth, CSRF, ratelimit, errorpage), `auth service` (Redis), `transactions controller`, `observability`.
- `go test ./...` verde em cada release.

### Conhecidos / limitações

- Free tier do Render bloqueia porta SMTP 587 — projeto usa Resend por padrão para contornar.
- Embeddings desabilitados em produção sem provedor OpenAI-compatible configurado (Groq não expõe `/v1/embeddings`).
- Backup automático depende do plano Postgres do Render.
- Rate limit é fixed window (não sliding) — ataques podem staggerar entre janelas.
- Migração de senha do BD no free tier exige ALTER USER manual e reconfiguração da env var.
