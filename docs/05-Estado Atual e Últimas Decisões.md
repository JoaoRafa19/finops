# Estado Atual e Últimas Decisões

Data de referência: 2026-03-10

## Onde a aplicação está agora

O projeto já concluiu a base do monólito com autenticação local, sessão em Redis, cookie seguro e proteção CSRF. A aplicação também já possui:

- página de login;
- fluxo de logout;
- checagem de autenticação por middleware;
- criação de workspace no onboarding;
- home autenticada;
- listagem de contas por usuário/workspace.

Neste momento, a aplicação está na transição entre a base de autenticação e o início do core financeiro. O foco ativo é o fluxo:

`workspace -> home -> primeira conta -> contas -> transações`

Ou seja: a fundação de acesso e contexto do usuário já existe, e o próximo bloco funcional é consolidar o cadastro da primeira conta para então avançar para transações.

## Últimas decisões tomadas

### 1. Postgres é o banco oficial da aplicação

Mesmo que documentos antigos mencionem MySQL, a implementação atual usa `pgx`, migrations em Postgres e queries geradas por `sqlc` voltadas para Postgres.

### 2. Autenticação escolhida: sessão stateful em Redis

Foi decidido usar sessão tradicional armazenada no Redis, com:

- cookie HttpOnly contendo apenas o ID da sessão;
- invalidação explícita no logout;
- proteção CSRF via synchronizer token;
- middleware de sessão antes do middleware de autorização.

Essa decisão está formalizada no ADR `0001`.

### 3. Modelo de autorização atual: rotas públicas e privadas no mesmo monólito

O roteamento separa:

- rotas públicas, como `/login`;
- rotas privadas, protegidas por `AuthRequired`, como `/`, `/onboarding` e `/logout`.

Essa decisão está alinhada com a organização atual em controllers, services e middleware.

### 4. Cada usuário terá um workspace único no MVP

Para reduzir complexidade nesta fase, a aplicação está assumindo um workspace efetivo por usuário. Ainda não existe fluxo multi-workspace no produto.

### 5. Ausência de workspace é tratada como etapa de onboarding

Se o usuário autenticado ainda não possui workspace, o comportamento esperado é conduzi-lo ao onboarding, e não gerar erro de aplicação.

### 6. O onboarding foi simplificado

O onboarding cria apenas o workspace inicial. Ele não cria conta automaticamente. Isso mantém o fluxo inicial menor e separa responsabilidades:

- onboarding cria o contexto do usuário;
- home passa a ser o ponto de entrada da primeira conta.

### 7. Defaults iniciais foram definidos para acelerar o MVP

As convenções atuais são:

- moeda padrão inicial: `BRL`;
- nome padrão do workspace quando vazio: um valor default no backend.

Observação: há diferença entre o plano/documentação e a implementação atual sobre o nome default do workspace. O plano menciona `Meu Workspace`, mas o service atual usa `MyWorkspace`. Isso precisa ser alinhado.

### 8. A home deve suportar estado vazio

A home não deve falhar quando o workspace existir mas ainda não houver contas. A decisão é renderizar um estado vazio útil e oferecer ali o cadastro da primeira conta.

### 9. O core financeiro começa por contas

Antes de transações, o sistema precisa permitir:

- criar conta;
- listar contas do workspace;
- definir moeda, tipo e saldo inicial.

Essa dependência já orienta a estrutura da home e do `AccountService`.

## Estado funcional por área

### Autenticação

Já implementado:

- login;
- logout;
- sessão com Redis;
- cookie de sessão;
- CSRF;
- middleware de autenticação;
- carregamento de sessão no contexto.

### Workspace e onboarding

Já implementado:

- verificação se o usuário possui workspace;
- tela de onboarding;
- criação do workspace inicial;
- redirecionamento de volta para a home após criação.

### Home

Já implementado:

- home autenticada;
- leitura da sessão do usuário;
- listagem de contas vinculadas ao workspace;
- template com estado vazio e formulário da primeira conta.

### Contas

Parcialmente implementado:

- `AccountService.ListByUser` já existe;
- `AccountService.Create` já existe no service;
- template da home já possui formulário `POST /accounts`;
- `AccountController` foi criado, mas ainda está incompleto;
- a rota de criação de conta ainda não está conectada no router.

## Ponto exato do desenvolvimento

O sistema já consegue levar o usuário até aqui:

1. login;
2. autenticação por sessão;
3. criação do workspace no onboarding;
4. acesso à home autenticada.

O ponto atual de desenvolvimento é concluir o passo seguinte:

5. cadastro da primeira conta a partir da home.

Depois disso, o próximo avanço natural é a camada de transações, já com contas existentes no workspace.

## Pendências e desalinhamentos identificados

### Fluxo da home vs ausência de workspace

Hoje o `HomeController` depende de `ListByUser`, e quando isso falha com `sql.ErrNoRows` ele redireciona para `/onboarding`. Na prática, esse erro está sendo usado para inferir ausência de workspace. Funciona como heurística, mas o ideal é a home consultar workspace de forma explícita antes de decidir o fluxo.

### Controller de contas ainda não finalizado

Existe um `AccountController` novo no repositório, mas ele ainda não implementa a action de criação da conta e ainda não foi plugado ao roteamento privado.

### Inconsistência no default do nome do workspace

Há divergência entre documentação e código:

- documentação/planejamento: `Meu Workspace`;
- implementação atual em service: `MyWorkspace`.

### Documento de autorização ainda está mais amplo do que o código atual

O ADR `0002` descreve uma visão mais extensa de autorização e comportamento HTTP. A aplicação real já segue parte dessa direção, mas o estado implementado ainda está mais enxuto que o desenho completo.

## Resumo executivo

O projeto saiu da fase de infraestrutura de autenticação e já entrou no começo do domínio financeiro. O estado atual da aplicação é: usuário autentica, cria o workspace e chega à home; a etapa imediatamente em curso é finalizar o cadastro da primeira conta e conectar esse fluxo ao router/controller. A partir daí, o sistema fica pronto para evoluir para transações.
