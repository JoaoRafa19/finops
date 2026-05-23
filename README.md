# finops



## Roadmap
```mermaid
gantt
  title Roadmap evolutivo do FinOps
  dateFormat  YYYY-MM-DD
  axisFormat  %d/%m

  section Base do monolito
  Setup repo, docker-compose e migracoes base      :a1, 2026-02-26, 8d
  Autenticacao local, sessoes e CSRF               :a2, 2026-03-05, 8d
  Request ID e middleware base                     :a3, 2026-03-11, 4d

  section Core financeiro
  Onboarding e workspace unico MVP                 :b1, 2026-03-15, 5d
  Contas, saldo atual e edicao por modal           :b2, 2026-03-20, 12d
  Transacoes manuais e listagem recente            :b3, 2026-04-01, 10d
  Ajustes HTMX na home e formularios               :b4, 2026-04-07, 7d
  Transferencias entre contas                      :b5, 2026-04-14, 10d
  Categorias e testes do core financeiro           :b6, 2026-04-21, 7d

  section Depois do core
  Relatorios basicos e visao consolidada           :c1, 2026-04-28, 10d
  Importacao CSV e OFX                             :c2, 2026-05-08, 12d
  Observabilidade e health checks                  :c3, 2026-05-20, 8d
  IA local e automacoes                            :c4, 2026-05-30, 14d
```

## Estado atual

- Base do monólito concluída: app sobe com Go + Postgres + Redis, autenticação por sessão e proteção CSRF já estão implementadas.
- Onboarding do workspace MVP concluído: usuário autenticado sem workspace é direcionado para criação inicial.
- Core financeiro em andamento: contas já possuem criação, listagem e edição; transações manuais já possuem cadastro e listagem recente na home.
- O foco atual é fechar o slice `contas -> transações -> transferências` antes de abrir novas frentes.

## Próximos passos

- Fechar UX de contas na home: garantir que saldo atual e edição por modal fiquem consistentes.
- Fechar UX de transações: preservar estado do formulário em erro e manter a listagem recente consistente.
- Implementar transferências entre contas como duas transações vinculadas.
- Consolidar testes dos controllers e fluxos HTMX do core financeiro.

## Critérios de aceitação do bloco atual

- **Contas**: criar e editar conta pela home, com retorno consistente via modal e saldo atual correto.
- **Transações**: registrar transação manual e ver o efeito refletido na home.
- **Transferências**: movimentar valor entre duas contas sem distorcer o saldo consolidado.
- **Qualidade**: `go test ./...` passando ao final de cada bloco.
