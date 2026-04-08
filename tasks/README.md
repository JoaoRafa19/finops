# Tasks

Estrutura simples para controlar o trabalho do projeto dentro do repositório.

## Fluxo

1. Criar uma tarefa em `tasks/backlog/` usando o template de `tasks/_templates/task.md`.
2. Quando começar a executar, mover o arquivo para `tasks/in-progress/`.
3. Se travar por dependência, decisão ou bug externo, mover para `tasks/blocked/`.
4. Ao concluir, mover para `tasks/done/`.

## Convenção de nome

Use nomes curtos e sequenciais:

- `001-fechar-transferencias.md`
- `002-corrigir-modal-categoria.md`
- `003-testar-home-controller.md`

## Prioridade sugerida

Prefixe o título do arquivo ou o campo `Prioridade` com:

- `P0` para item crítico do fluxo atual
- `P1` para próximo item importante
- `P2` para melhoria útil mas não bloqueante

## Regra prática

- Cada arquivo deve representar uma tarefa pequena e executável.
- Se uma tarefa crescer demais, quebre em novas tarefas menores no backlog.
- Não use este diretório para ideias vagas; use somente itens acionáveis.
