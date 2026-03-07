# Implementation Plan

1. Autenticação base concluída
Breve descrição: o projeto já possui login, logout, sessão em Redis, proteção CSRF e middleware para rotas privadas.

2. Postgres é o banco oficial da aplicação
Breve descrição: apesar de documentos antigos citarem MySQL, a implementação atual usa `pgx`, migrations em Postgres e queries compatíveis com Postgres.

3. Próxima fase ativa do roadmap é o Core financeiro
Breve descrição: após concluir a base de autenticação, a evolução atual está focada em `contas -> transações -> transferências`.

4. Contas são a primeira feature do Core
Breve descrição: antes de lançar transações, o sistema precisa permitir listar e criar contas, pois transações dependem de uma conta existente.

5. Workspace único por usuário no MVP
Breve descrição: por enquanto, cada usuário terá apenas um workspace efetivo, evitando a complexidade de multi-workspace nesta fase.

6. Ausência de workspace é tratada como onboarding
Breve descrição: quando o usuário autenticado ainda não possui workspace, o sistema não deve falhar; deve redirecionar para um fluxo inicial de criação.

7. Onboarding tem formulário simples de criação de workspace
Breve descrição: a tela `/onboarding` pede apenas o nome do workspace; o campo é opcional e usa um valor padrão quando vier vazio.

8. Nome padrão do workspace é `Meu Workspace`
Breve descrição: se o usuário não informar nome no onboarding, o backend cria o workspace com esse valor padrão.

9. Moeda padrão inicial é `BRL`
Breve descrição: tanto no onboarding quanto no fluxo inicial de contas, o comportamento assumido até agora usa `BRL` como default.

10. Após criar o workspace, o usuário volta para a home
Breve descrição: o onboarding não precisa criar conta junto; após criar o workspace, o usuário retorna para `/`.

11. Home com workspace sem contas deve mostrar estado vazio
Breve descrição: a homepage deve renderizar normalmente quando não existirem contas e oferecer a criação da primeira conta, em vez de tratar isso como erro.

12. Primeira conta será cadastrada pela própria home
Breve descrição: o fluxo escolhido é usar a home como ponto de entrada para o cadastro inicial de conta após o onboarding.

13. Implementação incremental com validação a cada etapa
Breve descrição: as mudanças estão sendo feitas funcionalidade por funcionalidade, com revisão do estado atual antes de seguir para o próximo bloco.
