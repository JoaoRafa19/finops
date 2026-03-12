# finops



## Roadmap
```mermaid
gantt
  title Roadmap evolutivo do FinOps
  dateFormat  YYYY-MM-DD
  axisFormat  %d/%m

  section Base do monólito
  Setup repo, docker-compose, migrações base        :a1, 2026-02-26, 10d
  Autenticação local + sessões + CSRF               :a2, 2026-03-03, 7d
  Request ID middleware + logs estruturados         :a3, 2026-03-06, 4d

  section Core financeiro
  Contas + transações + transferência               :b1, 2026-03-10, 10d
  Categorias + relatórios básicos                   :b2, 2026-03-17, 10d
  Sistema de regras automáticas de categorização    :b3, 2026-03-22, 6d

  section Importação e conciliação
  Import CSV + templates de mapeamento              :c1, 2026-03-24, 7d
  Import OFX + staging + dedupe                     :c2, 2026-03-31, 10d
  Conciliação manual + matching automático básico   :c3, 2026-04-07, 7d
  Worker system para processamento assíncrono       :c4, 2026-04-10, 6d

  section Infraestrutura avançada
  Redis cache (dashboard + relatórios)              :d1, 2026-04-14, 6d
  Rate limiting + segurança API                     :d2, 2026-04-18, 4d
  Métricas Prometheus + health checks               :d3, 2026-04-20, 5d

  section IA local
  Integração Ollama (generate + schema)             :e1, 2026-04-24, 7d
  Embeddings + persistência + busca                 :e2, 2026-05-01, 7d
  Categorização automática com IA                   :e3, 2026-05-05, 5d

  section IA avançada e insights
  Busca semântica de transações                     :f1, 2026-05-10, 6d
  Copilot financeiro (perguntas em linguagem natural):f2, 2026-05-16, 7d
  Forecast financeiro e insights automáticos        :f3, 2026-05-23, 7d

  section Qualidade e deploy
  Testes (unit/integration/E2E)                     :g1, 2026-05-30, 10d
  Observabilidade completa (logs + metrics + trace) :g2, 2026-06-08, 7d
  Render deploy (Blueprint) + CI/CD                 :g3, 2026-06-15, 7d
```

Critérios de aceitação por marco (amostra):
- **Base**: subir app + DB via compose; `templ generate --watch` funcionando; login OK com cookie seguro e CSRF em POSTs. 
- **Core**: criar transação e ver refletir no saldo; transferência não conta em relatórios.  
- **Import**: importar OFX/CSV e deduplicar por `FITID`/hash; staging revisável. 
- **IA**: categorização retorna JSON válido via schema (sem texto extra) e é auditada; embeddings gerados via `/api/embed`.
- **Deploy**: `render.yaml` provisiona web+DB; backups planejados; logs sem dados sensíveis.