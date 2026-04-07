# finops



## Roadmap
```mermaid
gantt
  title Roadmap operacional do FinOps
  dateFormat  YYYY-MM-DD
  axisFormat  %d/%m

  section Base concluída
  Setup repo + compose + migrations                 :done1, 2026-02-26, 8d
  Auth local + sessões Redis + CSRF                 :done2, 2026-03-05, 8d
  Request ID + middleware base                      :done3, 2026-03-11, 4d

  section Core financeiro atual
  Onboarding + workspace único MVP                  :done4, 2026-03-15, 5d
  Contas: criação, listagem e edição                :active1, 2026-03-20, 12d
  Transações: cadastro manual + listagem recente    :active2, 2026-04-01, 10d
  Ajustes de UX home/HTMX para contas e transações  :active3, 2026-04-07, 7d

  section Próximo bloco
  Transferências entre contas                       :next1, 2026-04-14, 10d
  Categorias: fechamento do modal e fluxo HTMX      :next2, 2026-04-18, 5d
  Testes de controllers do core financeiro          :next3, 2026-04-21, 7d

  section Depois do core fechado
  Relatórios básicos e visão consolidada            :later1, 2026-04-28, 10d
  Importação CSV/OFX                                :later2, 2026-05-08, 12d
  Observabilidade e health checks                   :later3, 2026-05-20, 8d
  IA local e automações                             :later4, 2026-05-30, 14d
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
