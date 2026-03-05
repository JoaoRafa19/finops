# finops

## Roadmap
```mermaid
gantt
  title Roadmap 8–12 semanas (exemplo)
  dateFormat  YYYY-MM-DD
  axisFormat  %d/%m

  section Base do monólito
  Setup repo, docker-compose, migrações base        :a1, 2026-02-26, 10d
  Autenticação local + sessões + CSRF               :a2, 2026-03-03, 7d

  section Core financeiro
  Contas + transações + transferência               :b1, 2026-03-10, 10d
  Categorias + relatórios básicos                   :b2, 2026-03-17, 10d

  section Importação e conciliação
  Import CSV + templates de mapeamento              :c1, 2026-03-24, 7d
  Import OFX + staging + dedupe                     :c2, 2026-03-31, 10d
  Conciliação manual + matching automático básico   :c3, 2026-04-07, 7d

  section IA local
  Integração Ollama (generate + schema)             :d1, 2026-04-14, 7d
  Embeddings + persistência + busca                 :d2, 2026-04-21, 7d

  section Qualidade e deploy
  Testes (unit/integration/E2E)                     :e1, 2026-04-28, 10d
  Render deploy (Blueprint) + observabilidade       :e2, 2026-05-08, 10d
```

Critérios de aceitação por marco (amostra):
- **Base**: subir app + DB via compose; `templ generate --watch` funcionando; login OK com cookie seguro e CSRF em POSTs. 
- **Core**: criar transação e ver refletir no saldo; transferência não conta em relatórios.  
- **Import**: importar OFX/CSV e deduplicar por `FITID`/hash; staging revisável. 
- **IA**: categorização retorna JSON válido via schema (sem texto extra) e é auditada; embeddings gerados via `/api/embed`.
- **Deploy**: `render.yaml` provisiona web+DB; backups planejados; logs sem dados sensíveis.