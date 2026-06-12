# finops



## Roadmap
```mermaid
gantt
  title Roadmap evolutivo do FinOps
  dateFormat  YYYY-MM-DD
  axisFormat  %d/%m

  section Base do monolito
  Setup repo, docker-compose e migracoes base      :done, a1, 2026-02-26, 8d
  Autenticacao local, sessoes e CSRF               :done, a2, 2026-03-05, 8d
  Request ID e middleware base                     :done, a3, 2026-03-11, 4d

  section Core financeiro
  Onboarding e workspace unico MVP                 :done, b1, 2026-03-15, 5d
  Contas, saldo atual e edicao por modal           :done, b2, 2026-03-20, 12d
  Transacoes manuais e listagem recente            :done, b3, 2026-04-01, 10d
  Ajustes HTMX na home e formularios               :done, b4, 2026-04-07, 7d
  Transferencias entre contas                      :done, b5, 2026-04-14, 10d
  Categorias e testes do core financeiro           :done, b6, 2026-04-21, 7d

  section Depois do core
  Importacao CSV e OFX                             :done, c2, 2026-05-08, 12d
  Relatorios basicos e visao consolidada           :active, c1, 2026-06-01, 20d
  Observabilidade e health checks                  :c3, 2026-06-21, 10d
  IA local e automacoes                            :c4, 2026-07-01, 14d
```

## Estado atual

MVP entregue. Todo o core financeiro está concluído e o foco atual é o bloco de relatórios e automações.

**Concluído:**
- Base do monólito: app sobe com Go + Postgres + Redis, autenticação por sessão e proteção CSRF.
- Onboarding do workspace MVP: usuário autenticado sem workspace é direcionado para criação inicial.
- Core financeiro completo: contas (criação, listagem, edição por modal), transações manuais, transferências entre contas e categorias.
- Importação OFX e CSV: upload, preview e confirmação de lançamentos via arquivo.

**Em andamento:**
- Relatórios básicos e visão consolidada (gastos por categoria, comparativo mensal, evolução de saldo).
- Camada LLM: tools financeiras para consulta em linguagem natural sobre os dados do workspace.

## Próximos passos

- Finalizar queries SQL de relatórios e regenerar com `make sqlc`.
- Implementar `ReportService` (gastos por categoria, comparativo mensal, histórico de saldo, listagem filtrada).
- Construir controller e templates de relatórios (4 views + chat LLM).
- Expor LLM tools para consultas em linguagem natural sobre os dados financeiros.
- Observabilidade: health checks e métricas básicas.

## Critérios de aceitação do bloco atual

- **Contas**: criar e editar conta pela home, com retorno consistente via modal e saldo atual correto.
- **Transações**: registrar transação manual e ver o efeito refletido na home.
- **Transferências**: movimentar valor entre duas contas sem distorcer o saldo consolidado.
- **Qualidade**: `go test ./...` passando ao final de cada bloco.
- **Gastos por categoria**: total agrupado por categoria em qualquer período, excluindo transferências.
- **Comparativo mensal**: receitas × despesas mês a mês.
- **Evolução de saldo**: curva de saldo acumulado com âncora correta no início do período.
- **Listagem filtrada**: paginação com filtros de conta, categoria, direção e período.
- **Chat LLM**: pergunta em linguagem natural retorna resposta baseada nos dados do workspace.
- **Qualidade**: `go test ./...` passando ao final de cada etapa.

## Env Variables
```
DB_HOST=localhost
DB_PORT=5432
DB_NAME=finops
DB_USER=finops
DB_PASSWORD=finops
```
