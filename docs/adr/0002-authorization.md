ADR: Autenticação por Sessão com Redis em Monolito Go (HTMX, Templ, Tailwind)
Contexto

Estamos desenvolvendo o FinOps, um sistema web monolítico escrito em Go, com interface dinâmica fornecida por HTMX, Templ (template engine) e Tailwind CSS no front-end. O sistema requer autenticação de usuários e proteção contra ataques CSRF. Desejamos uma solução de autenticação baseada em sessões tradicionais (stateful), em vez de tokens JWT, para maximizar o controle sobre sessões ativas (permitindo, por exemplo, invalidação imediata de sessões no logout ou em casos suspeitos). Também precisamos suportar a funcionalidade "lembrar-me" (sessões persistentes mais longas opcionalmente), e assegurar que a implementação funcione adequadamente com o modelo de interação do HTMX (submissões parciais de formulário, atualizações inline, etc.).

Resumindo, o contexto exige: (a) sessões de usuário armazenadas no servidor (usando Redis como store, dado que o aplicativo pode ser escalado horizontalmente), (b) cookies de sessão seguros no cliente para vincular solicitações à sessão armazenada, (c) proteção CSRF robusta, e (d) integração suave com HTMX para experiência dinâmica (ex.: validação inline e redirecionamentos adequados). Além disso, precisamos definir claramente os endpoints de login/logout e os contratos HTTP (códigos de status, formato de respostas de erro) para padronizar o comportamento do sistema ao autenticar e desautenticar usuários.

Decisão

Em resumo: Adotaremos autenticação via sessões stateful armazenadas no Redis, identificadas por um cookie seguro no cliente. Definimos endpoints HTTP para login/logout conforme descrito abaixo, utilizando tokens CSRF do tipo synchronizer para proteger requisições POST. A ordem dos middlewares garantirá logging, carregamento da sessão, validação CSRF e verificação de autenticação. O cookie de sessão terá flags de segurança apropriadas. Implementaremos também a lógica para lidar com requisições HTMX (AJAX) de forma elegante (por exemplo, retornando fragmentos HTML ou cabeçalhos especiais em vez de redirecionamentos completos, quando necessário). Os pontos-chave da decisão são:

Endpoints HTTP:

GET /login: retorna a página de formulário de login (HTML) com código 200.

POST /login: recebe credenciais e tenta autenticar. Em caso de sucesso, cria uma nova sessão no Redis e retorna um 302 Redirect para a página inicial ou destino apropriado. Em caso de falha (credenciais inválidas), retorna 401 Unauthorized (se for requisição AJAX/HTMX ou API, com um JSON contendo mensagem de erro) ou realiza um redirect de volta para /login com um flash message de erro (no caso de navegação normal). Em ambos os casos, a resposta de erro não expõe detalhes sensíveis (mensagem genérica como "usuário ou senha inválidos").

POST /logout: invalida a sessão ativa do usuário. No server, remove/invalida a sessão no Redis e, no client, expira o cookie de sessão. Em seguida, responde com 302 Redirect para a página de login (ou página pública) indicando que a sessão foi encerrada. (Para requisição HTMX, podemos retornar um cabeçalho HX-Redirect em vez de 302 tradicional, conforme detalhado abaixo.)

Contrato de Sessão (Redis):
Cada sessão autenticada será armazenada no Redis como uma entrada, usando a chave prefixada sess:{session_id}. O {session_id} será um ID de sessão aleatório criptograficamente seguro gerado no login. Os dados armazenados no valor associado incluirão pelo menos: user_id (identificador do usuário autenticado), email (do usuário, se necessário para exibir em UI ou logs), csrf_token (token aleatório para proteção CSRF, descrito adiante), expires_at (timestamp de expiração) e remember_me (booleano indicando se a sessão é do tipo "lembrar-me"). Podemos armazenar esses dados como um hash ou JSON. Nenhuma informação sensível além disso será armazenada no cliente – o cookie conterá apenas o ID de sessão, funcionando como um ponteiro para os dados reais no Redis.

A TTL padrão das chaves de sessão no Redis será de 30 minutos (para sessões normais não “lembradas”), o que significa que, se o usuário ficar inativo por 30 min, a sessão expira. Caso o usuário faça uma nova requisição antes disso, podemos optar por renovar o TTL (sliding expiration) para melhorar a UX (sessão permanece ativa enquanto o usuário estiver usando). Essa abordagem de expiração deslizante é comum em apps de consumo para evitar logouts frequentes durante atividade contínua, enquanto expiração absoluta (não renovada) de 30 min poderia ser usada em cenários de segurança mais alta.

Se o usuário marcar "Lembrar-me", a sessão terá uma TTL estendida de 7 dias. Nesse caso, além de armazenar remember_me=true e definir expires_at apropriadamente, também será enviado um cookie persistente com expiração em 7 dias no navegador. Assim, o usuário pode fechar o browser e reabrir dentro desse período sem precisar logar novamente. (Sem "lembrar-me", o cookie não terá um expiry persistente, tornando-se um cookie de sessão que expira ao fechar o navegador, conforme comportamento padrão). Vale notar que 7 dias é uma configuração arbitrária de conveniência; valores comuns variam de alguns dias a semanas, cabendo à política de segurança do produto balancear conveniência vs. risco. Escolhemos 7 dias como um meio-termo razoável.

Política de Cookies (Sessão):
O cookie de sessão se chamará finops_session. Será enviado ao cliente no Set-Cookie quando a sessão for criada (login bem-sucedido) e também atualizado/renovado conforme necessário (por exemplo, se implementar sliding expiration, poderíamos atualizar a expiração do cookie a cada resposta). As configurações de segurança do cookie incluem:

HttpOnly: habilitado, impedindo acesso via JavaScript no front-end (mitigando roubo de sessão via XSS).

Secure: habilitado em produção, para garantir que o cookie só seja transmitido sobre conexões HTTPS. (Em ambientes de desenvolvimento sem TLS, esta flag pode ser desabilitada para teste, mas em produção é mandatória).

SameSite=Lax: o atributo SameSite será Lax, o que significa que o cookie não será enviado em requisições de contextos cross-site típicos (como requisições POST de formulários de outros sites ou em <iframe>), porém ainda será enviado em navegações top-level GET (ex.: se o usuário clicar em um link de um email para nosso site, o cookie seguirá). O objetivo é mitigar CSRF sem prejudicar casos legítimos de navegação. Nota: SameSite=Lax fornece proteção parcial contra CSRF e já é padrão na maioria dos navegadores; mesmo assim, não confiamos apenas nisso e implementamos tokens CSRF para defesa em profundidade.

Path=/: o cookie é válido para toda a aplicação (raiz do domínio da aplicação).

Domain: não será definido (assim, padrão é host atual apenas), evitando vazar para subdomínios indesejados.

Expires/Max-Age: para sessões "lembrar-me", será definido Expires (ou Max-Age) de 7 dias no futuro, tornando o cookie persistente até essa data. Para sessões padrão, não definiremos expires (cookie de sessão) ou definiremos um Max-Age curto alinhado com a TTL (embora geralmente, sem "lembrar-me", o cookie pode ficar sem expiry e apenas o Redis TTL controla a validade de sessão).

Remoção no logout: no logout, o servidor enviará um Set-Cookie com o mesmo nome finops_session e flags, porém com Max-Age=0 (ou data de expiração no passado) para instruir o navegador a apagar o cookie imediatamente. Isso garante que o identificador de sessão não permaneça armazenado no browser após logoff. Em paralelo, a sessão correspondente será removida do Redis. Essa dupla medida (invalidar no cliente e no servidor) é crucial: logout deve remover o cookie no cliente e invalidar a sessão no servidor, para evitar que um atacante reutilize um cookie de sessão capturado após logout.

Estratégia CSRF:
Adotaremos o Synchronizer Token Pattern (token anti-CSRF baseado em sessão). Ao efetuar login (ou criar uma nova sessão), o servidor gera um token CSRF aleatório (criptograficamente seguro, p.ex. 128 bits) e o armazena na sessão (campo csrf_token). Esse mesmo token será inserido em formulários HTML gerados pelo servidor como um campo hidden (por convenção, chamaremos de _csrf no form). Para requisições feitas via HTMX ou AJAX, o token deve ser enviado em um cabeçalho personalizado, como X-CSRF-Token. Em nosso caso, podemos inserir um meta-tag no HTML ou usar um atributo hx-headers global para que HTMX envie automaticamente o token em cada request.

O middleware CSRF guard validará, para cada requisição state-changing (métodos POST, PUT, DELETE), se o token presente no request corresponde ao token armazenado na sessão do usuário: ele buscará o token no header X-CSRF-Token (ou no corpo, caso form tradicional) e comparará com o valor dentro da sessão. Se estiver ausente ou não bater, a requisição é bloqueada com 403 Forbidden imediatamente, pois assume-se uma possível tentativa de CSRF. (Nota: 403 é apropriado aqui já que o usuário até pode estar autenticado, mas a requisição não é autorizada devido à falha na verificação de origem.) Em caso de falha, retornaremos um JSON de erro ou uma página de erro genérica conforme o contexto (p.ex., para HTMX, podemos retornar um snippet ou um erro error específico para manipulação). Logs de segurança também serão gerados para facilitar monitoramento de possíveis ataques CSRF. O token CSRF terá validade atrelada à sessão (podemos mantê-lo constante por sessão para simplicidade; alternativamente poderíamos regenerá-lo a cada login ou até a cada requisição para máxima segurança, mas isso complicaria UX em caso de formulário aberto em outra aba - manter um por sessão é um bom compromisso). Importante: como camada extra, mantemos o cookie de sessão com SameSite=Lax conforme citado, o que reduz exposição a CSRF em navegadores modernos, mas conforme recomendações do OWASP, sempre combinamos tokens anti-CSRF junto com SameSite para melhor proteção.

Ordem de Middlewares:
O servidor Go aplicará middlewares na seguinte ordem para cada requisição:

Logging – primeiro, um middleware de logging genérico para registro de requisição (método, path, status, tempo, etc.). Sem impacto no estado de autenticação, pode vir no topo.

Session Loader – em seguida, um middleware que lê o cookie finops_session da requisição. Se presente, extrai o ID de sessão e consulta o Redis (GET sess:<id>). Se encontrar uma sessão válida, carrega os dados (user_id, email, csrf_token, etc.) no contexto da requisição (por exemplo, anexando a um struct no request.Context). Se o cookie estiver ausente ou a sessão não existir/expirada, esse passo simplesmente não anexa credenciais (o que será interpretado posteriormente como "usuário não autenticado"). Este middleware também pode renovar o TTL da chave no Redis se usarmos expiração deslizante (i.e., cada acesso bem-sucedido ao sistema renova a validade por mais 30 min).

CSRF Guard – depois de a sessão (se existente) estar carregada, o middleware CSRF verifica as requisições conforme descrito acima. Ele deve rodar antes de qualquer processamento que modifique estado no servidor. No caso de falha (token inválido), retorna 403 imediatamente, evitando que o handler da rota seja executado. É importante vir após o loader de sessão, pois necessita do token armazenado.

Auth Required – por fim, um middleware de autorização/autenticação para rotas privadas. Nas rotas marcadas como protegidas (e.g., qualquer URL fora login/static), este middleware checa se no contexto da requisição existe um usuário autenticado (dados de sessão carregados). Se não houver, ele intercepta a requisição. O comportamento pode ser: redirecionar para /login com HTTP 302 (se for uma navegação de página) ou retornar um 401 Unauthorized (se for uma chamada XHR/HTMX) indicando que login é necessário. (Podemos distinguir pelo cabeçalho HX-Request do HTMX ou pelo Accept header se espera JSON). Assim garantimos que áreas privadas não são acessíveis sem login. Para rotas públicas ou o próprio /login, esse middleware não bloqueia. Também podemos incluir aqui lógica de authorization (checagem de roles/permissões) se aplicável no futuro – por ora, assume-se que ou o usuário tem acesso ou não, já que é um sistema interno monolítico simples.

(Observação: a ordem acima é alinhada com práticas comuns; por exemplo, frameworks web como Django recomendam carregar sessão antes de verificar CSRF, caso contrário a validação CSRF não consegue ler o token do armazenamento. Da mesma forma, a checagem de autenticação deve vir após garantir que a requisição é legítima do ponto de vista CSRF.)

Integração com HTMX (UX dinâmico):
Como usamos HTMX para experiências dinâmicas (submissão de formulários sem recarregar a página, atualizações parciais de conteúdo etc.), a implementação deve tratar certos fluxos de forma diferenciada para melhorar a UX:

No caso do POST /login, se a requisição vier de um form HTMX (detectável via header HX-Request: true), em vez de retornar um 302 (que o HTMX por padrão não segue corretamente para reload completo), optaremos por retornar a resposta com código 200 e um header especial HX-Redirect: /url-destino. O HTMX, ao ver esse header, fará um redirecionamento completo do browser para a URL destino (efetivamente logando o usuário e carregando a página inicial autenticada). Isso evita comportamentos estranhos onde o HTMX poderia tentar injetar o HTML da página redirecionada em um fragmento. Em outras palavras, para chamadas HTMX usamos HX-Redirect ao invés de 302 para navegá-las corretamente.

Ainda no login via HTMX, em caso de erro de autenticação, podemos retornar o próprio formulário de login (ou um trecho dele) já renderizado com uma mensagem de erro (por exemplo, um alerta inline) e status 401 ou 200. Como o HTMX está esperando um fragmento, ele pode substituir a parte do DOM correspondente pelo conteúdo de erro sem recarregar toda a página – oferecendo validação inline. Alternativamente, poderíamos retornar um pequeno JSON com a mensagem de erro e usar um handler JS, mas mantendo em HTML aproveitamos o HTMX para o swap automático.

Para requisições subsequentes feitas via HTMX (por exemplo, um POST em alguma funcionalidade interna autenticada), precisamos garantir que o token CSRF esteja sendo enviado. Como mencionado, utilizaremos hx-headers no <body> ou meta tag para inserir X-CSRF-Token em todas requisições HTMX automaticamente, de modo que nosso middleware CSRF continue funcionando sem exigir JS manual.

Se a sessão expirar enquanto o usuário está numa página e eles fizerem uma requisição HTMX, o middleware Auth Required retornará provavelmente um 401. O HTMX ao receber 401 não redirecionará por conta própria; podemos então adicionar um comportamento global no front (por exemplo, um handler htmx:onError ou onUnauthorized custom) para capturar 401 e recarregar a página de login. Outra abordagem é, no middleware Auth, se detectar HX-Request, retornar um HX-Redirect: /login (forçando redirecionamento login).

Em suma, estamos tomando cuidado para que as respostas em cenário HTMX sejam adequadas: headers especiais (HX-*) e fragmentos HTML em vez de suposições de full page reload, garantindo que a experiência seja fluida.

Testes (Critérios de Aceitação): Definiremos testes automatizados (por exemplo, testes de integração usando HTTP client fake) para cobrir os cenários críticos de autenticação e segurança:

Login com credenciais válidas – deve retornar 302 redirect para página autenticada, setar o cookie de sessão corretamente e criar entrada no Redis (TTL correspondente). Verificar que, após o login, acessar uma rota protegida retorna 200 (autorizado).

Login com credenciais inválidas – deve retornar 401 (no caso de chamada API/AJAX) ou redirect de volta para /login com mensagem de erro (no caso de chamada normal). Garantir que nenhum cookie de sessão válido é setado e que nenhuma sessão no Redis é criada.

Acesso não autenticado a rota privada – simulando um usuário que não está logado e tenta acessar, por exemplo, /app/dashboard. Esperado: ou um redirect 302 para /login (fluxo web) ou um 401 Unauthorized (se for uma chamada XHR/API), garantindo que não é possível acessar conteúdo sem login.

Requisição POST sem CSRF token – (ex: omitir token ou token errado em um formulário simuladamente enviado de outro domínio). Esperado: o middleware CSRF bloqueia e retorna 403 Forbidden, e nenhuma mudança de estado ocorre.

Logout – após realizar logout, a sessão deve ser removida/inválida no Redis e o cookie de sessão deve desaparecer do cliente. Tentativas de reutilizar o cookie antigo (via ferramentas ou script) não devem conseguir acessar áreas autenticadas (o middleware de sessão não encontrará a sessão, levando a redirecionamento/401). Também testar que após logout o usuário é redirecionado adequadamente para a página de login e que uma eventual chamada autenticada via XHR retorna erro após logout.

Sessão expirada – simular passagem do tempo além do TTL e verificar que o acesso subsequente é tratado como não autenticado (redirect/401), e se "lembrar-me" estiver habilitado, verificar que antes do TTL de 7 dias expirar o usuário permanece logado mesmo fechando e reabrindo navegador (persistência).

Esses testes asseguram o cumprimento dos requisitos de segurança: que somente usuários autenticados acessam conteúdo privado, que tokens CSRF são mandatórios para operações sensíveis, e que logout e expiração realmente invalidam o acesso. Também garantem que a integração com HTMX não quebrou esses fluxos (ex.: verificando que em chamadas HTMX os cabeçalhos/códigos estão corretos).

Justificativa Técnica

Por que sessões server-side e Redis? Optamos por sessões stateful armazenadas no servidor (em Redis) em vez de tokens JWT stateless principalmente para ter maior controle e segurança sobre sessões de usuários. Em aplicações como esta (monolítica, possivelmente interna ou com dados sensíveis), é valioso poder invalidar imediatamente uma sessão (por logout explícito ou eventos de segurança) simplesmente removendo a entrada no Redis – todos os pedidos subsequentes com aquele cookie tornam-se inválidos instantaneamente. Com JWT puramente no cliente, isso seria impossível sem mecanismos adicionais, já que o token permaneceria válido até expirar. O Redis atua como um armazenador central, permitindo que múltiplas instâncias da aplicação compartilhem estado de sessão. Cada sessão é um registro chave-valor (ou hash) pequeno, e mesmo centenas de milhares de sessões são triviais para o Redis manejar. Além disso, Redis oferece expiração automática de keys, simplificando a limpeza de sessões expiradas sem esforço manual.

A decisão de usar um store externo (Redis) em vez de memória local também vem da necessidade de escalar a aplicação: se rodarmos múltiplos processos/containers, todos podem acessar o mesmo Redis para validar sessões. Essa abordagem é bem estabelecida – frameworks web frequentemente usam Redis ou bancos similares para session store distribuído; a chave sess:<id> prefixada segue convenção comum (ex.: Express.js default). Em suma, sessions + Redis nos dão confiabilidade e o trade-off aceitável é a manutenção desse serviço extra.

TTL 30min vs "lembrar-me" 7d: Escolhemos 30 minutos como tempo de inatividade após o qual sessões normais expiram por motivos de segurança – é um valor tradicional que mitiga riscos caso o usuário esqueça de logout em computador compartilhado ou em caso de roubo de cookie, limitando janela de uso indevido. Outros sistemas seguem janela semelhante (15-30 min inatividade para sistemas bancários, por ex.). Entretanto, para conveniência, oferecemos "lembrar-me" que estende a sessão até 7 dias (persistente). 7 dias foi considerado um equilíbrio entre conveniência e exposição ao risco; não usamos um período mais longo (30+ dias) pois os dados do FinOps podem ser sensíveis e queremos limitar a longevidade de um cookie de sessão no cliente. Vale mencionar que qualquer duração é arbitrária e deve refletir requisitos de negócio e segurança – implementações variam de dias a semanas. Nossa justificativa técnica é que usuários confiam essa decisão explicitamente via checkbox; se marcado, assumem responsabilidade de manter dispositivo seguro (e nós garantimos via flags seguras do cookie). Também notamos que navegadores modernos possuem políticas de expiração automática para cookies sem uso (Chrome, por ex., limpa cookies Lax não usados por alguns meses), mas 7 dias fica bem dentro disso. Em resumo, 30 min idle aumenta segurança base, 7 dias opt-in oferece usabilidade com riscos mitigados (via flags de cookie e possibilidade de revogação server-side).

Cookie HttpOnly, Secure, SameSite: São configurações de segurança padrão hoje e adotá-las tem justificativas claras: HttpOnly previne scripts maliciosos (XSS) de roubarem a identificação de sessão; Secure garante que nunca transmitiremos o cookie em conexões não criptografadas, evitando vazamento por intermediários; SameSite=Lax, embora não substitua proteção CSRF ativa, dificulta ataques CSRF triviais porque o navegador não enviará o cookie em contextos cross-site comuns (form posts invisíveis, etc.). Decidimos por Lax em vez de Strict porque Strict poderia prejudicar a UX em cenários legítimos (e.g., usuário clicar em link externo para retornar ao app e não ter sua sessão enviada, o que efetivamente o deslogaria em certos fluxos). Lax oferece a maioria das proteções necessárias contra métodos "perigosos" (POST/PUT) mantendo cookies em navegações GET de topo. Em conjunto com tokens CSRF, consideramos isso suficiente. Adicionalmente, definiremos o flag Secure condicionalmente – ambientes de dev local sem HTTPS não funcionariam com Secure, mas isso será documentado; em produção é indispensável, pois cookies de sessão sem Secure poderiam ser capturados se qualquer chamada acidentalmente ocorrer via HTTP.

Proteção CSRF (Synchronizer Token): Seguir o padrão de token sincronizado foi uma decisão natural porque nosso aplicativo mantém estado de sessão. Como indicado pelo OWASP, aplicações stateful devem usar tokens únicos por sessão para prevenir CSRF. Avaliamos outras abordagens como "Double Submit Cookie" ou verificar headers como Origin, mas tokens sincronizados oferecem robustez e simplicidade de verificação no backend sem dependência exclusiva de comportamentos de navegador. Geramos um token secreto guardado no servidor e exigimos que cada requisição de alteração o mande de volta – isso efetivamente vincula o navegador do usuário à ação esperada, impedindo sites terceiros de forjarem requisições válidas. A implementação é direta e existem bibliotecas caso precisássemos. Optamos por implementá-la manualmente para aprendizado/controle, mas seguiremos as diretrizes OWASP: tokens devem ser criptograficamente aleatórios e únicos por sessão, nunca enviados em URLs ou logs (somente em corpo/headers). Também consideramos que o SameSite cookie já mitiga boa parte de ataques CSRF comuns, mas não é perfeito (p.ex., não protege contra formulários GET indevidos, compatibilidade retro, ou contra login CSRF). Por isso, combinamos ambas defesas, e de fato, uma resposta no Security StackExchange reforça que só SameSite não basta e tokens anti-CSRF continuam recomendados para total segurança. Em síntese, a justificativa é reduzir ao mínimo a superfície para CSRF – um atacante teria que roubar o token via XSS (que outras medidas já dificultam) para conseguir enviar uma requisição válida, tornando o ataque altamente improvável.

Middlewares e organização: A sequência escolhida visa garantir que cada responsabilidade ocorra no momento certo. Por que logging primeiro? Assim registramos até mesmo requisições que possam ser bloqueadas mais tarde por CSRF ou auth; e também porque ele é transversal e independente. Session loader precisa vir antes de qualquer decisão de acesso ou CSRF porque tanto a verificação CSRF quanto a de autenticação dependem de conhecer o usuário/tokens. Colocá-lo antes também nos permite, eventualmente, registrar últimas atividades ou implementar sliding expiration (atualizando TTL a cada uso) antes de prosseguir. CSRF antes de auth-check porque mesmo se não autenticado, não há risco em não ter token (podemos simplesmente ignorar para requests públicos); mas se autenticado, queremos barrar requisições suspeitas antes de executar lógica de negócio. Algumas frameworks colocam CSRF depois de auth – porém, isso geralmente é quando assumem que apenas usuários logados terão sessões e tokens; no nosso caso, podemos validar CSRF sempre que um token e sessão existirem. A ordem Auth Required no final garante que a requisição só chega no handler se passou por todos os filtros de segurança. Essa estrutura modular melhora a manutenibilidade e teste – podemos testar cada middleware isoladamente. Por exemplo, simular uma requisição sem token para o CSRF middleware e esperar bloqueio, ou simular uma rota com auth required sem sessão e ver se redireciona.

A integração do HTMX exigiu algumas adaptações técnicas: aprendemos pelos próprios docs/issues do HTMX que ele não segue redirecionamentos 302 da forma tradicional para navegadores. A justificativa para usar HX-Redirect header é justamente contornar essa limitação – é a solução recomendada pela comunidade HTMX para cenários de pós-login. Isso mantém a aplicação consistente tanto para navegadores full quanto para requisições HTMX. O mesmo vale para validação inline: ao retornar HTML parcial com erro, evitamos ter que codificar lógica duplicada no front-end para exibir mensagens – reutilizamos nossos templates no servidor e o HTMX injeta no DOM. Essa abordagem "server-driven UI" condiz com a escolha por HTMX e Templ, nos dando interatividade sem escrever JavaScript customizado. Do ponto de vista arquitetural, suportar HTMX significa tratar as rotas praticamente como APIs RESTful retornando fragmentos HTML/JSON dependendo do header – isso foi levado em conta no design dos handlers (por ex., POST /login precisa distinguir modo normal vs HTMX via headers).

Trade-offs e Consequências: A decisão traz algumas trocas a considerar:

Complexidade vs Simplicidade: Sessions stateful requerem infraestrutura (Redis) e um pouco mais de lógica do lado servidor (gerenciar TTLs, armazenamento). Em contrapartida, oferecem a já mencionada capacidade de invalidação imediata e armazenamento seguro de dados extras (como flags de MFA, timestamp, etc.) que podemos usar para avançar requisitos futuros. Tokens JWT seriam mais simples de distribuir (stateless), mas complicariam revogação e aumentariam a responsabilidade do cliente em armazená-los com segurança. Para nosso cenário, privilegiamos controle sobre simplicidade.

Escalabilidade: A solução escala bem horizontalmente enquanto o Redis comportar carga (que, para sessões, normalmente é leve – ~100k sessões = 100k pequenas keys). Adicionamos, porém, um ponto de falha: se o Redis ficar indisponível, logins não funcionarão e possivelmente nenhuma requisição autenticada validará. Mitigamos isso escolhendo Redis (que é estável) e podendo configurar alta disponibilidade, mas é um ponto a monitorar. Em contrapartida, JWT não teria ponto central – mas como nosso app já requer banco de dados etc., um Redis não é um custo muito alto.

Segurança: A abordagem é robusta em segurança: armazenamento server-side significa que mesmo que um cookie vaze, podemos invalidá-lo no servidor. Também nos permite monitorar sessões (poderíamos registrar IP de origem, device, etc., na sessão e invalidar se algo suspeito). O trade-off é que precisamos ter total atenção em proteger o cookie e o Redis. Os flags Secure/HttpOnly/SameSite e CSRF tokens cuidam do lado cliente. No servidor, assegurar que as comunicações com Redis são internas ou seguras e que IDs de sessão são aleatórios o bastante (para não haver predição) são considerações vitais. Também há um trade-off em armazenar dados como email na sessão: pode haver inconsistência se o usuário atualizar seu email no sistema – a sessão poderia ter um valor antigo até re-login. Aqui decidimos aceitar essa pequena inconsistência em prol de não realizar consultas extras a todo momento; em casos críticos, poderíamos sincronizar certos campos ou apenas armazenar user_id e consultar detalhes on-demand. Novamente, é uma escolha de caching vs consistência.

Experiência do usuário: Com a implementação "lembrar-me", facilitamos a vida do usuário, mas prolongamos a presença de um token de acesso no cliente. Se o dispositivo for comprometido, uma sessão válida de até 7 dias poderia ser usada indevidamente. No entanto, se o usuário não optar por "lembrar-me", a segurança é maior (cookie de sessão morre no browser ao fechar). Documentaremos essa diferença para os usuários. Além disso, sessões curtas (30 min) significam que sem "lembrar-me" usuários terão que re-logar com certa frequência, o que pode ser inconveniente – é uma troca deliberada por segurança. Acreditamos que dar a opção explicita (checkbox) torna isso aceitável.

Integração HTMX: A decisão de suportar HTMX sem separar APIs REST e páginas nos força a tratar variações de resposta no mesmo endpoint, aumentando complexidade no código dos handlers (ifs para HX-Request, etc.). É um trade-off: poderíamos ter construído APIs JSON separadas, mas optamos por uma solução unificada para evitar duplicação de lógica. Isso exige teste cuidadoso para não introduzir bugs (por ex., esquecer de enviar HX-Redirect poderia quebrar login via HTMX). No ADR consideramos que essa complexidade extra é justificada pela melhoria de UX e pela filosofia server-driven do projeto. Manteremos exemplos claros e possivelmente helpers utilitários no código para lidar com essas diferenças (e.g., uma função util respondLoginSuccess(w, r, destURL) que internamente checa se HTMX ou não e decide enviar 302 ou HX-Redirect).

Manutenção e Evolução: Registrar essa decisão em um ADR nos ajuda a no futuro reavaliar se necessário. Por exemplo, se um dia o FinOps virar vários microsserviços, poderemos considerar migrar para JWT ou outro método – mas então teremos este registro para entender o contexto em que a escolha atual foi feita (monolito, necessidade de controle imediato, etc.). Da mesma forma, se bugs de sessão surgirem, saberemos quais invariantes esperar (ex: "session loader always sets context if cookie valid" – útil para debug).

Resumindo, acreditamos que os benefícios (segurança, controle, compatibilidade com nossa stack) superam os custos (infra adicional, leve incremento de complexidade). A decisão está alinhada com práticas recomendadas para aplicações web seguras: sessões curtas renováveis, cookies protegidos, tokens anti-CSRF e teste rigoroso de fluxo de autenticação.

Consequências e Trade-offs

Consequências Positivas: Esta decisão reforça a segurança do FinOps:

Imediata Revogação de Acesso: Podemos invalidar sessões instantaneamente (logout ou outras políticas), atendendo requisitos de segurança corporativa (ex.: remover acesso de usuários comprometidos).

Defesa em Camadas: Com token CSRF + SameSite + HttpOnly cookies, cobrimos múltiplos vetores de ataque (CSRF e XSS). Também, ao não carregar nada além de um ID no cookie, mesmo que fosse interceptado, o atacante não obtém informações sensíveis diretamente.

Escalabilidade Moderada: A aplicação pode ser dimensionada para múltiplos servidores sem sticky sessions, pois o Redis centraliza o estado. A limpeza automática de sessões expiradas pelo Redis poupa desenvolvedores de construir tarefas de limpeza.

Melhor UX Opcional: O "lembrar-me" fornece conveniência solicitada pelos usuários, e a integração HTMX nos dá uma interface mais responsiva (menos reloads de página), sem comprometer os padrões de segurança (já que adaptamos tokens e respostas adequadamente).

Consequências Negativas/Pontos de Atenção:

Dependência do Redis: Introduzimos um componente externo – se o Redis falhar, logins e validações falham. Precisaremos monitorá-lo e talvez implementar um mecanismo de fallback ou fila de reconexão. Também há pequena latência adicionada em cada request (consulta ao Redis), mas sendo O(1) e Redis in-memory, isso é normalmente insignificante.

Gerenciamento de Sessões: Com sessões longas (7 dias), aumenta a necessidade de monitorar possíveis session hijacking. Podemos mitigar implementando detecção de dispositivo/UA e alertando usuário se um login ocorrer de contexto muito diferente, ou invalidando todas sessões em logout global. Não implementaremos isso inicialmente, mas manter no radar.

Stale Data: Armazenar email na sessão significa que se o usuário alterar seu email, a sessão atual pode continuar mostrando o antigo até renovar. Aceitamos isso dado o escopo (FinOps talvez não permita o próprio usuário mudar email com frequência, e mesmo se, um logout/login resolve; alternativamente, poderíamos atualizar a sessão no momento da mudança).

Complexidade de Código: Os handlers de login/logout e middleware CSRF/auth têm várias ramificações (para HTMX vs normal, JSON vs HTML). Isso requer disciplina para manter e testar. Equívocos podem introduzir bugs onde, por exemplo, uma chamada HTMX não autenticada não redirecione corretamente. Para mitigar, definiremos claramente essas ramificações e adicionaremos testes simulando ambos os modos.

Cargas de Testes: A necessidade de testar muitos cenários (login ok, erro, csrf fail, expire, etc.) aumenta o esforço de QA, mas esse esforço é necessário dado que estamos implementando recursos de segurança – não podemos arriscar falhas silenciosas. Esses testes, entretanto, também atuam como documentação viva do contrato definido.

Requisitos de Documentação: Precisaremos instruir desenvolvedores futuros sobre como usar corretamente o token CSRF nos templates (e.g., sempre incluir {{.CSRFToken}} hidden nos forms, e para qualquer novo formulário HTMX lembrar do header) – um lapso pode introduzir vulnerabilidade. Vamos documentar essas práticas no README do projeto ou num guia de contribuidor.

No balanço, as consequências negativas são administráveis com boa engenharia (monitoramento de Redis, testes, documentação), enquanto as positivas são fundamentais para a integridade e confiança no sistema. Assim, seguimos satisfeitos com esta decisão arquitetural.

Referências

OWASP Cheat Sheet: Cross-Site Request Forgery Prevention – Recomenda uso de Synchronizer Token Pattern para apps stateful; tokens únicos por sessão, comparados no backend. Também destaca que SameSite cookies não eliminam necessidade de tokens CSRF em todos casos.

Medium (Sarah Nzeshi, Session Management Deep Dive #2) – Explica vantagens de sessões server-side com Redis. Exemplifica chave sess:<ID> no Redis contendo dados do usuário, com apenas o ID no cookie HttpOnly. Descreve validação da sessão a cada request via O(1) lookup no Redis. Discute políticas de expiração absoluta vs. deslizante (usuário ativo permanece logado) e recomenda sliding expirations para melhor UX em apps comuns.

Medium (Shubham Soni, Remember Me in Web Applications) – Discute diferenças entre sessão temporária e persistente. Recomenda sempre usar cookies com HttpOnly, Secure e SameSite (Strict/Lax), e limpar cookies no logout. Também exemplifica persistência remember-me ~7 ou 30 dias como comum.

Infosec Institute – Protecting Cookies with HttpOnly and Secure: destaca que flag Secure previne envio do cookie em conexões HTTP, e HttpOnly previne roubo via XSS (JavaScript não consegue ler o cookie). Essas flags são imperativas para cookies de sessão de autenticação.

Security StackExchange – Logout e destruição de sessão: enfatiza que um logout correto deve terminar a sessão no servidor e remover o cookie no cliente. Mantivemos isso na solução (deleção da key no Redis + cookie expirado). Outra resposta reforça que sessões no servidor devem expirar após logout, não bastando esperar o cookie expirar.

GitHub Issues / HTMX Docs: Observação de que HTMX não segue 302 por padrão, sendo preferível usar HX-Redirect header com código 200 para redirecionar em requisições AJAX. Implementamos essa nuance para compatibilidade com HTMX. Também exemplo do uso de hx-headers='{"X-CSRF-Token": "..."}' no <body> para incluir token CSRF automaticamente em todas requisições HTMX – solução adotada para facilitar integração CSRF.

Prompt para Implementação (Codex)

Desenvolva a implementação em Go de acordo com a arquitetura acima, garantindo clareza e segurança do código. Utilize as tecnologias mencionadas (HTMX no front-end, Templ templates e Redis para sessão). Implemente os seguintes requisitos:

Configuração de Sessão e Redis: Configure um client Redis para armazenar sessões. Implemente uma estrutura SessionData com campos UserID, Email, CSRFToken, ExpiresAt, RememberMe. Implemente funções para criar, buscar e deletar sessões no Redis:

CreateSession(userID, email string, remember bool) -> (sessionID string, sessionData SessionData): Gera um ID de sessão aleatório seguro, popula SessionData com userID, email, CSRFToken (token aleatório seguro), ExpiresAt (agora + TTL adequado: 30min ou 7d dependendo de remember), RememberMe (true/false). Salva no Redis com TTL correspondente. Retorna o ID para ser enviado no cookie.

GetSession(sessionID string) -> (SessionData, error): Consulta Redis pela chave sess:<sessionID>. Retorna erro se não existir (expirada ou inválida).

DeleteSession(sessionID string): Remove a chave no Redis (usa DEL).

Se usar expiração deslizante, considere atualizar o TTL ao acessar (e atualizar ExpiresAt no objeto).

Middlewares: Crie middlewares HTTP para:

LoggingMiddleware: logar cada requisição (método, URL, status, tempo).

SessionMiddleware: lê o cookie finops_session. Se presente, carrega a sessão via GetSession. Se encontrada, anexa o SessionData (ou pelo menos userID, email e CSRF) no request.Context. Se remember=true e falta pouco para expirar, você pode optar por estender TTL (não obrigatório, mas pode implementar sliding expiry). Em caso de erro (sessão inexistente), simplesmente prossiga sem sessão no contexto (não aborta resposta).

CSRFMiddleware: em cada requisição mutante (POST/PUT/DELETE), valida o token. Obtém o SessionData do contexto (se não houver e também não há cookie, pode ignorar validação pois usuário não logado não terá token – embora tais rotas possivelmente já exijam auth). Se houver SessionData, compare o token presente:

Procure o token no header X-CSRF-Token ou em um campo de formulário r.FormValue("_csrf") (trate ambos para cobrir AJAX e forms normais).

Se o token do request estiver vazio ou não bater com SessionData.CSRFToken, retorne 403 Forbidden. Escreva uma resposta adequada: no caso de Accept JSON ou HX-Request, retorne um JSON {error: "CSRF token invalid"} ou talvez um pequeno HTML informando erro; você pode decidir.

Caso o token valide ou para métodos seguros (GET, OPTIONS), continue a chain.

AuthMiddleware: proteja rotas autenticadas. Configure de modo que seja aplicado apenas em rotas privadas (por exemplo, usando um router que permite grupos ou verificando Request.URL.Path). No middleware, cheque se SessionData está no contexto (setado pelo SessionMiddleware).

Se não existir sessão no contexto: o usuário não está logado. Responda adequadamente – se detectar HX-Request ou Accept JSON, responda com status 401 e talvez {error: "unauthorized"}; caso contrário, redirecione (http.Redirect) para /login com 302. (Dica: você pode verificar r.Header.Get("HX-Request") != "" para saber se veio de HTMX, ou usar r.Header.Get("Accept") se estiver um valor JSON).

Se existir sessão, apenas prossiga (user considerado autenticado).

Handlers Endpoints:

GET /login: Se o usuário já estiver autenticado (session presente no contexto), opcionalmente pode redirecionar para home (para evitar re-login desnecessário). Caso contrário, renderize a página de login usando Templ (ou html/template). Inclua no form de login um campo hidden com name _csrf e valor = token CSRF da sessão caso haja uma sessão anônima? (Como o usuário não tem sessão ainda, não teremos token... Alternativa: poderíamos gerar um token temporário para a página de login, mas como vamos criar sessão apenas após credenciais válidas, provavelmente não protegemos o POST /login por CSRF – login form geralmente está público antes de auth. Então não precisa token CSRF no login form). Simplesmente renderize o form (campos para email e senha, e checkbox "remember me").

POST /login: Processar os dados do form de login:

Leia email, password e remember (checkbox) do request. Valide as credenciais (para fins deste prompt, você pode simular uma validação fixa, ex: aceitar user@example.com/password ou verificar em um map).

Se as credenciais forem inválidas:

Se for requisição HTMX (HX-Request presente): retorne HTML parcial contendo o form de login novamente mas com uma mensagem de erro embutida (por ex., um <div class="error">Credenciais inválidas</div> no topo). Mantenha status 401 (ou 200 – debata qual usar; 401 pode ser ok já que não será interceptado a não ser que haja um handler global – talvez melhor 200 para que HTMX substitua conteúdo, já que 401 HTMX pode acionar seu próprio mecanismo de erro; podemos usar 200). Para segurança do usuário, a mensagem deve ser genérica.

Se não HTMX (navegação normal): defina uma mensagem flash de erro (pode simular salvando em SessionData temporário ou usando redirect URL param) e faça redirect 302 de volta para /login. (Você pode simplesmente redirect e acrescentar ?error=1 na URL para simplificar, e no GET /login exibir mensagem se esse param presente).

Em ambos casos, não criar sessão.

Se as credenciais forem válidas:

Chame CreateSession para gerar uma nova sessão. Obtenha sessionID e SessionData.

Inicie o cookie de resposta: Set-Cookie: finops_session=<sessionID>; HttpOnly; Path=/; SameSite=Lax. Se remember=true, adicione Secure (se TLS) e um Expires + Max-Age de 7 dias no futuro; caso contrário, não coloque expires (cookie de sessão).

Resposta:

Se for HTMX (HX-Request): em vez de 302, retorne status 200 e envie header HX-Redirect: /dashboard (ou página inicial autenticada). Opcional: poderia retornar um pequeno JSON {redirect: "/dashboard"} e ter script, mas HX-Redirect é o mais simples.

Se for normal: faça http.Redirect(w, r, "/dashboard", http.StatusSeeOther) (302 Found). O navegador então fará GET /dashboard.

POST /logout: Invalida sessão atual.

Verifique se há sessão no contexto (se não, simplesmente redirect para /login de qualquer forma, nada a fazer).

Se sim, pegue o sessionID. Chame DeleteSession(sessionID) no Redis.

Invalide o cookie: Set-Cookie: finops_session="" com Max-Age=0; Path=/; SameSite=Lax; HttpOnly; Secure (Secure se aplicável) para remover do navegador.

Responda:

Se HTMX: pode retornar HX-Redirect: /login (forçando reload na página de login).

Se normal: redirect 302 para /login.

GET /dashboard (exemplo de rota autenticada): (Implementar uma rota qualquer para testar auth). Esse handler deve ler do contexto o SessionData (colocado pelo middleware). Por exemplo, exibir "Olá, <email>" e talvez um form de logout (um botão que faz POST /logout). Se por algum motivo chegar aqui sem SessionData (o middleware Auth deveria ter bloqueado), retorne 401 ou redirect.

HTMX considerations (global): Garanta no template base (se houver) ou nas páginas protegidas que HTMX enviará o token CSRF:

Por exemplo, inclua no <body> do layout autenticado: <body hx-headers='{"X-CSRF-Token": "{{.CSRFToken}}"}'> de forma que qualquer requisição HTMX subsequente carregue o header. Certifique-se que CSRFToken está disponível no contexto/template quando o usuário está logado. Assim, os desenvolvedores não precisam adicionar manualmente em cada chamada.

Alternativamente, se não quiser no body, ao gerar cada fragmento HTML via HTMX você pode inserir um meta tag <meta name="csrf-token" content="{{.CSRFToken}}"> e ter um script JS que pegue e coloque em cada request, mas o método hx-headers inline é preferido pela simplicidade (evita JS).

Também, pense em um mecanismo global de erro: se uma resposta retorna 401 (digamos AuthMiddleware interceptou em HTMX request), o HTMX por padrão considera erro. Podemos deixar assim e confiar que HX-Redirect será usado no lugar certo, ou implementar um handler JS global: (fora do escopo do backend, mas mencionar que o backend está preparado para enviar HX-Redirect no logout e login expirado).

Templ Templates: Use o templ (ou text/template) para separar HTML. Tenha templates para login page e dashboard.

O template do login deve ter campos para email, senha, lembrar-me. Se receber um parâmetro de erro (via Query ou context), exibe a mensagem flash.

O template do dashboard (ou home autenticada) pode mostrar email do usuário e ter um form (ou botão) de logout. O form logout pode ser um simples <form id="logoutForm" hx-post="/logout" hx-redirect="true"> ou um botão com hx-post (mas lembre de incluir CSRF nesse também! se usar hx-post diretamente num botão, precisará hx-headers ou um input).

Inclua nos forms internos um input hidden _csrf com valor do token do contexto (por exemplo, se você renderiza um form de alterar perfil, etc.). O login form público não tem token pois não há contexto de usuário ainda.

Testing Hints (not necessarily implement in code here, but structure): Write unit tests or integration tests using httptest.NewServer simulating:

Successful login sets cookie and returns correct redirect or HX-Redirect.

Failed login returns proper status and message.

Access to /dashboard without login is blocked (after login is allowed).

CSRF protection works: simulate missing token on a protected form submission and get 403.

Logout indeed deletes session (after logout, using the old cookie fails to auth).

Detalhes de implementação adicionais:

Use a router/framework de sua escolha (pode ser net/http padrão com http.ServeMux ou algo como Echo, mas dado uso de middlewares custom, net/http simples ou Alice chain é suficiente).

Manejo de contexto: provavelmente usaremos context.WithValue para passar SessionData do middleware para handlers. Defina uma key do tipo custom para evitar colisão.

Hash de senhas: Para simplificar, não implemente hashing real no prompt, mas comente que em produção usaríamos bcrypt.

Cuidado com concorrência: Redis client é seguro para uso concorrente. Ao gerar session IDs, use crypto/rand.

Tente tornar o código idiomático, claro, separando responsabilidades (middlewares separados, funções auxiliares para cookie).

Inclua comentários no código explicando passos importantes (especialmente onde há bifurcações para HTMX vs normal, e setup de cookie flags), para aumentar clareza.

Produza o código completo necessário (ou pseudocódigo estruturado) cobrindo os pontos acima. Certifique-se que segue as boas práticas de segurança (nenhum dado sensível no cookie, tokens imprevisíveis, validação rigorosa).