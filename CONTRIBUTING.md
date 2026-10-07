# Contribuindo com o MeuRPG

Todo código entra por PR para `PuraFome/meuRPG`, com o CI verde e a aprovação do Samuel. Nada vai direto para a `main`. Até o MVP, a aprovação fica para depois: com o CI verde, quem abriu o PR faz o squash merge, e o PR continua sendo aberto para registrar o que mudou (decidido em 30/09/2026).

## Ambiente local

Ferramentas: Go 1.27, buf, sqlc 1.31.1, goose, golangci-lint, Docker e Node 22. No Mac, todas instalam pelo Homebrew. O sqlc é opcional: o `make sqlc` roda a versão certa sozinho.

| Comando | O que faz |
| --- | --- |
| `make up` | Sobe o CockroachDB (um nó só), o devidp (provedor OIDC de desenvolvimento) e o backend com Docker Compose (`deploy/local/compose.yaml`); serve o app em `http://localhost:8080`, servidor e API na mesma origem, com o login funcionando (ver [Login local com o devidp](#login-local-com-o-devidp)) e as imagens da galeria num volume (ver [Imagens da galeria](#imagens-da-galeria)). O log sai em `LOG_LEVEL=debug` (cada requisição, RPC, stream e evento de jogo, com `request_id`); `LOG_LEVEL=info make up` baixa o nível (ver [Arquitetura](docs/arquitetura.md#os-logs)). |
| `make up LOCAL_STACK=native` | O mesmo ambiente **sem Docker**: o devidp e a API rodam como processos no Mac, contra o CockroachDB nativo, nas mesmas portas e com o mesmo login. `make down`, `make logs` e `make e2e` aceitam o mesmo `LOCAL_STACK=native`. Ver [Tudo nativo](#tudo-nativo-mac-opcional). |
| `make run` | Roda o backend direto no terminal, apontando para o banco do `make up`. |
| `make db-native-start` / `make db-native-stop` | Liga e desliga um CockroachDB rodando direto no Mac, fora do Docker, para o `LOCAL_DB=native`. Ver [CockroachDB nativo](#cockroachdb-nativo-mac-opcional). |
| `make db-test-start` / `make db-test-stop` | Liga e desliga o CockroachDB **dos testes**, também direto no Mac, na porta 26258 (com `TEST_DB_PORT=26259`, um segundo, ao lado), com o banco **na memória** (no máximo 2 GiB, que somem ao desligar). É o melhor lugar para os testes de integração: as mudanças de esquema, de que os bancos de teste são feitos, levam um quarto do tempo, e o banco de desenvolvimento (26257) não se enche de bancos de teste. Ver [Os bancos dos testes de integração](#os-bancos-dos-testes-de-integração). |
| `make proto` | Gera o código Go **e** o TypeScript a partir dos `.proto` (`backend/gen` e `web/src/gen`). Instala as dependências do `web/` sozinho, se faltarem. |
| `make sqlc` | Gera o código Go das queries SQL (`backend/internal/<módulo>/<módulo>db`) com o sqlc 1.31.1. Ver [Queries com sqlc](#queries-com-sqlc). |
| `make lint` | Roda `buf lint` e `golangci-lint`. |
| `make test` | Roda `go test -race` em todo o backend. |
| `MEURPG_TEST_DATABASE_URL='postgresql://root@localhost:26258/defaultdb?sslmode=disable' make test` | Roda os testes de integração (migrations, transações, login, campanhas, convites, personagens, sessões de jogo, a sessão ao vivo, com o stream, a galeria, os mapas, as cenas de RP, as pistas e as anotações dos jogadores, o palco e os retratos dos NPCs, o combate, os destaques do combate, o XP, a subida de nível guiada, as opções das cenas, o resumo da sessão, as criaturas do SRD, o bestiário e o NPC feito de uma criatura, as camadas dos mapas, as portas, as armadilhas e o que elas fazem em jogo, os tesouros, a luz carregada, a névoa de guerra por jogador, o movimento em círculo, o salto e a cobertura do combate, as criaturas do personagem e o que a ficha pode convocar, a Forma Selvagem e os olhos do familiar, as classes e subclasses da mesa (os padrões da tabela de 20 níveis, o menu de efeitos, o campo de cada recusa, a multiclasse na criação, a subida de nível e o combate com o conjurador de um terço, e as frases de "A classe mudou"), as regras da mesa fora do combate: os PV da subida de nível, os jeitos de fazer atributos e os 4d6 guardados, a névoa dos mapas novos e a mudança do modo de XP; a calibração da grade; o mapa de uma masmorra gerada, com a prévia, a lista das salas, a cena na sala e o "Redesenhar"; os quebra-cabeças: as quatro tabelas, as jogadas ao mesmo tempo, o "Ao resolver", o enigma, a sequência (tocada passo a passo) e a cifra, as dicas por teste de perícia (dado do app e físico), a informação dividida e o "Ao errar" (a armadilha, as tentativas e os limites, contados com exatidão com respostas ao mesmo tempo); o conteúdo da mesa: as escritas do mestre, a revisão e o cache ao vivo, "A classe mudou" e a entrada arquivada como escolha nova; as imagens geradas por IA, com o gerador falso, e as feitas de um mapa: a vista dos jogadores (a união do que os personagens veem agora, o NPC escondido e a porta secreta como parede), o mapa com textura completado e recortado de volta e o "Usar como imagem do mapa"; o combate sem grade, o teatro da mente: o modo, o movimento por número, o ataque de oportunidade que o mestre oferece e o que não existe sem mapa; e a lista de magias dos jogadores, o antecedente "Outro" e a magia da mesa em combate; os interruptores "Opções para os jogadores": o que está desligado nunca chega a um jogador, a escolha nova recusada, a revisão e o cache, a dica `content_changed` e os interruptores ao mesmo tempo; os monstros no combate, "Pôr no combate": os nomes, os PV médios ou rolados, a iniciativa de cada um, o que o jogador lê, o crítico, o XP pelo ND e o teatro da mente; e as regras da mesa dentro do combate: o crítico (dados dobrados ou o máximo mais uma rolagem, de arma, de magia, de criatura e de NPC, com dado do app e com dado físico, lido na transação de cada rolagem) e os testes contra a morte que só o dono e o mestre veem, no combatente, no registro, no stream, nos destaques e no resumo; e o gerador de tesouro: o nível do grupo, quem pode pedir, "Pôr no mapa" (o ponto escondido, a chave de idempotência, o que o jogador nunca recebe) e o ouro que vira XP numa campanha por ouro; e o montador de encontros: a conta contra o grupo, o gerador por semente, o "Trocar", o encontro guardado num ponto de batalha e o "Começar este combate", com os monstros entrando no início do combate, também no teatro da mente) contra o CockroachDB dos testes (`make db-test-start`); a porta 26257 também serve, a do `make up` ou do `make db-native-start`. Sem a variável, eles são pulados. |
| `make migrate` | Aplica as migrations do goose no banco local. |
| `make e2e` | Sobe o ambiente local (como o `make up`), roda os testes Playwright de `e2e/` contra ele e mostra onde está o relatório. O ambiente continua de pé; `make down` derruba. Ver [Testes ponta a ponta](#testes-ponta-a-ponta-playwright). |
| `make down` | Derruba o ambiente local (`docker compose down`). |
| `npm start` | Sobe o Angular antigo (`src/`), descontinuado — mantido só como referência (ver [App antigo](docs/app-antigo.md)). |
| `make web-install` | Instala as dependências do `web/`: `npm ci --ignore-scripts` (nunca roda scripts de instalação de terceiros). Se for adicionar ou atualizar uma dependência, use `npm install` com o Corepack ativado (`corepack enable`, uma vez só): o `web/package.json` fixa `npm@11.20.0` porque o `npm` de série (10.x) trava ao resolver o grafo de peer dependencies do Vitest 4.1; `npm ci` não tem esse problema e funciona com qualquer um dos dois. |
| `make web-test` | Roda os testes do Angular (`cd web && npm test`). Os arquivos de teste não são isolados uns dos outros (o construtor do Angular roda o Vitest com `isolate: false`), então `web/src/test-setup.ts` desfaz todo `vi.stubGlobal` ao fim de cada teste, dá a cada teste um `scrollIntoView` novo (o jsdom não tem, e os componentes chamam) e devolve os timers de verdade (um relógio falso não passa para outro arquivo); e `web/src/test-providers.ts` desliga as animações do Material: nenhum teste depende da ordem dos arquivos nem espera tempo de verdade. |
| `make web-build` | Builda o Angular para produção (`cd web && npm run build`). |
| `cd web && npm start` | Sobe o Angular sozinho, em modo dev, com `proxy.conf.json` encaminhando as rotas da API (`/meurpg.*`, `/auth`, `/images`, `/uploads`, `/healthz`, `/readyz`) para `localhost:8080`. |
| `WEB_DIR=../web/dist/web/browser PORT=8090 go run -C backend ./cmd/api` | Sobe só a API do jeito que ela roda em produção — servindo o build do Angular, com os headers de cache e o CSP de verdade — sem Docker nem banco. Rode `cd web && npm run build` antes. Sem `DATABASE_URL`, o login fica desligado e o `IdentityService` responde `unavailable`, mas a tela pública e o `SystemService.GetServerInfo` funcionam normalmente. Útil para conferir o CSP no navegador sem subir o Docker; a porta 8090 não conflita com o `make up`. |

### CockroachDB nativo (Mac, opcional)

No Mac, todo container roda dentro de uma máquina virtual Linux (a do Docker Desktop ou do Rancher Desktop), e o CockroachDB é de longe a parte mais pesada do ambiente: a máquina virtual chega a pedir mais de 10 GB de memória, e os testes de integração ficam lentos. Rodar o banco direto no Mac resolve isso. É opcional: o padrão, e o que o CI usa, continua sendo o banco no Docker.

1. Instale a mesma versão que o CI e o `compose.yaml` usam (26.2.7), pela tap oficial da Cockroach Labs. O Homebrew pede para confiar na fórmula antes:

   ```bash
   brew tap cockroachdb/tap
   brew trust --formula cockroachdb/tap/cockroach@26.2
   brew trust --formula cockroachdb/tap/cockroach   # o link carrega esta também
   brew install cockroachdb/tap/cockroach@26.2
   brew link cockroachdb/tap/cockroach@26.2
   ```

   Não use `brew services start`: ele deixa o banco subindo junto com o Mac. O `make` liga e desliga quando você precisa.
2. `make db-native-start` sobe o banco em `localhost:26257` (o console fica em `http://localhost:8081`), com os dados em `~/.meurpg/cockroach`, fora do repositório (todas as worktrees usam o mesmo banco), e cria o banco `meurpg`. Se já estiver de pé, não faz nada. `make db-native-stop` desliga.
3. Com `LOCAL_DB=native`, o `make up`, o `make e2e`, o `make down` e o `make logs` usam esse banco: o `deploy/local/compose.native-db.yaml` deixa o container do CockroachDB de fora e aponta a API e as migrations para `host.docker.internal:26257`. Dá para fixar no terminal com `export LOCAL_DB=native`, ou passar a cada comando: `make up LOCAL_DB=native`.
4. Os testes de integração podem usar esse banco (`MEURPG_TEST_DATABASE_URL='postgresql://root@localhost:26257/defaultdb?sslmode=disable' make test`), mas o lugar melhor para eles é o CockroachDB dos testes, também nativo, na memória e na porta 26258: `make db-test-start` e `MEURPG_TEST_DATABASE_URL='postgresql://root@localhost:26258/defaultdb?sslmode=disable' make test` (ver [Os bancos dos testes de integração](#os-bancos-dos-testes-de-integração)). Na máquina do Vinicius, com o banco nativo no disco, os testes do `campaigns` caíram de 729 s para 99 s, e os do `play`, de 407 s para 60 s, em relação ao banco no Docker.

O banco do container e o nativo usam a mesma porta: desligue um antes de ligar o outro (`make down` derruba o do container). Os dados deles são separados. Com o banco fora do Docker, dá para diminuir a memória da máquina virtual nas configurações do Docker Desktop ou do Rancher Desktop.

### Tudo nativo (Mac, opcional)

Com `LOCAL_STACK=native`, o ambiente local roda **sem Docker**: o devidp e a API são processos no Mac, contra o CockroachDB nativo (`make db-native-start`), e a máquina virtual do Docker pode ficar desligada. No Mac, essa máquina virtual reserva vários GB de memória; sem ela, sobra memória para os bancos de teste na memória (ver [abaixo](#os-bancos-dos-testes-de-integração)). O CI continua usando o Docker, e o `LOCAL_STACK=docker` continua sendo o padrão.

```bash
make db-native-start
make up LOCAL_STACK=native     # compila, migra, sobe o devidp e a API; o app em http://localhost:8080
make logs LOCAL_STACK=native   # os logs dos dois processos
make e2e LOCAL_STACK=native    # sobe (se precisar) e roda os testes Playwright
make down LOCAL_STACK=native   # para os dois processos
```

- **O que o `make up` faz** (`deploy/local/native.sh`): compila a API, o `migrate` e o devidp, refaz o build do Angular só se algum arquivo de `web/` mudou desde o último, aplica as migrations no banco `meurpg` e sobe o devidp e a API em segundo plano, com as mesmas variáveis do `compose.yaml`. Cada processo grava o PID e o log em `~/.meurpg/run/`, e o `make down` para exatamente esses PIDs. As imagens da galeria ficam em `~/.meurpg/images`, separadas do volume do Docker.
- **O issuer é o mesmo, `http://idp.localhost:9090`.** O resolvedor do macOS manda qualquer nome `*.localhost` para o próprio computador (RFC 6761), e o Go usa esse resolvedor por padrão no Mac. O resolvedor só do Go (`GODEBUG=netdns=go`) e o Linux não fazem isso; neles, use o Docker.
- **As portas 8080 e 9090 precisam estar livres.** O Rancher Desktop continua encaminhando as portas de um container mesmo depois do `make down`, até ser fechado: feche-o, ou suba o ambiente nativo em outras portas.
- **Um segundo ambiente, ao lado do primeiro:** `API_PORT=8180 IDP_PORT=9190 DB_NAME=meurpg_2 deploy/local/native.sh up` sobe outro, com outras portas, outro banco (criado se não existe), outra pasta de PIDs e logs (`~/.meurpg/run-8180`) e outra de imagens. Os testes Playwright vão para ele com `E2E_BASE_URL=http://localhost:8180 E2E_IDP_ORIGIN=http://idp.localhost:9190`; como o banco é outro, as contas de teste dos dois ambientes não se misturam. Para parar: `API_PORT=8180 deploy/local/native.sh down`.

### Os bancos dos testes de integração

Dá para ter **mais de um banco de teste na memória** ao mesmo tempo, cada um numa porta (`make db-test-start TEST_DB_PORT=26259`; o console fica na porta seguinte à do primeiro, 8083), com a própria memória (até 2 GiB cada). Duas execuções dos testes de integração, cada uma apontando o `MEURPG_TEST_DATABASE_URL` para um, nunca esperam uma pela outra. Cada um custa uns 2,5 GB de memória no Mac; com a máquina virtual do Docker desligada ([Tudo nativo](#tudo-nativo-mac-opcional)), dois cabem num Mac de 16 GB.

Os testes de integração **reaproveitam bancos**: cada pacote cria uns poucos bancos de teste e, entre um teste e outro, só os esvazia (`backend/internal/platform/dbtest`). Criar um banco com todas as tabelas leva uns 5 segundos no CockroachDB (cada tabela é uma mudança de esquema), e apagá-lo mais 1,5; esvaziá-lo com `DELETE` leva uns 10 milissegundos. Medido em 04/10/2026: os testes do `progression` caíram de 88 para 20 segundos.

- As migrations rodam uma vez só, num banco-modelo, `meurpg_tpl_<hash das migrations>`. Uma migration nova ou mudada gera outro. Os modelos de migrations antigas ficam no servidor de teste, vazios, e podem ser apagados (`DROP DATABASE meurpg_tpl_... CASCADE`).
- Um teste que precisa de banco pega um livre do pacote (ou cria um, copiando as tabelas do modelo) e o devolve quando termina; o próximo teste apaga todas as linhas antes de usar (das tabelas filhas para as mães). Testes em paralelo nunca dividem um banco: o pacote cria tantos quantos rodam ao mesmo tempo.
- Cada pacote que usa banco tem um `TestMain` que chama `dbtest.Main(m)`: no fim da execução, ele apaga os bancos que o pacote criou. Um pacote novo com testes de integração precisa dele, ou os bancos ficam para trás.
- Se uma execução é interrompida no meio, os bancos dela ficam no servidor (`meurpg_<pacote>_test_...`) e podem ser apagados à mão. Com o `make db-test-start`, desligar o banco dos testes apaga tudo.
- As migrations em si continuam testadas do zero pelo pacote `migrations`.
- Todo pool de teste tem **uma** conexão (`MaxConns = 1`) e um rastreador de `Acquire`. O teste em que uma leitura passa pelo pool de dentro de um `db.InTx` falha na hora, com a pilha, e o `Acquire` é cancelado. Uma espera de mais de 90 s por uma conexão (o caso que a pilha não enxerga) derruba o binário de testes do pacote inteiro, com a pilha de quem espera. Assim, a suíte de cada pacote é também a prova de que nenhuma escrita pega uma segunda conexão.
  - Um teste que corre transações umas contra as outras pede um pool maior no começo: `dbtest.PoolSize(t, n)`, com `n` pelo menos o número de corredores. Com uma conexão elas rodariam uma de cada vez, e o teste passaria sem provar nada. Funciona com `t.Parallel`.
  - Um teste que precisa segurar uma transação aberta enquanto o serviço trabalha usa `dbtest.SideConnection(t, pool).Begin(...)` (não `db.InTx` em volta de chamadas do serviço: o rastreador as acusaria).
  - `MEURPG_TEST_POOL_MAX_CONNS=8` vale para o processo inteiro, e serve para listar todas as violações de uma vez. Em produção o pool abre no máximo 10 conexões (ver [Operação](docs/operacao.md#o-pool-de-conexões)).
- No CI, o CockroachDB do job `go-db` também guarda os dados na memória e desliga o que só serve a um banco de longa duração (estatísticas automáticas, junção de faixas vazias, histórico de jobs), como o `make db-test-start`. Os pacotes rodam ao mesmo tempo contra ele, cada um com até 25 minutos (`go test -timeout 25m`, dentro dos 30 do job).

## Login local com um provedor OIDC

O login do mestre funciona com qualquer provedor OpenID Connect: o Google em produção e, na sua máquina, um provedor OIDC local (ou um provedor de verdade com um client de teste). O backend só precisa destas variáveis:

| Variável | Obrigatória | O que é |
| --- | --- | --- |
| `OIDC_ISSUER` | Sim, para ligar o login | O issuer do provedor, igual ao campo `issuer` do `/.well-known/openid-configuration` dele. Precisa ser `https`; `http` só vale em `localhost`, num nome `*.localhost` ou num IP de loopback. |
| `OIDC_CLIENT_ID` | Sim | O client ID do MeuRPG no provedor. |
| `OIDC_CLIENT_SECRET` | Sim | O client secret. É segredo: nunca vai para o repositório, para um issue ou para o log (o backend mostra `[REDACTED]`). |
| `OIDC_REDIRECT_URL` | Sim | A URL pública do backend mais `/auth/callback`. Na sua máquina, `http://localhost:8080/auth/callback`. Cadastre no provedor exatamente igual. |
| `OIDC_CA_FILE` | Não | Arquivo PEM com o certificado de um provedor local com certificado autoassinado. |
| `OIDC_MAX_AGE` | Não | Uma duração, como `1h`. Se definida, vai como `max_age`: o provedor pede a senha de novo quando o último login nele é mais antigo que isso. Use só com provedor que documenta `max_age`: `1h` com o devidp (é o que o `compose.yaml` usa); **não defina com o Google**, que não documenta `max_age`. Quem garante a reautenticação pelo menos a cada 30 dias (NIST SP 800-63B-4, AAL1) é a nossa sessão no servidor, que nunca passa de 30 dias, e não o `max_age`. |

Sem `OIDC_ISSUER` ou sem `DATABASE_URL`, o backend sobe do mesmo jeito, avisa no log que o login está desligado, e `/auth/login` responde 503.

Para testar o login na sua máquina:

1. No provedor, crie um client confidencial com Authorization Code, PKCE S256, os escopos `openid email` e a redirect URL `http://localhost:8080/auth/callback`.
2. Suba só o banco e as migrations, deixando a porta 8080 livre: `docker compose -f deploy/local/compose.yaml up -d cockroach migrate`.
3. Rode o backend com as variáveis, por exemplo `OIDC_ISSUER=... OIDC_CLIENT_ID=... OIDC_CLIENT_SECRET=... OIDC_REDIRECT_URL=http://localhost:8080/auth/callback make run`. O log de início diz `sign-in is enabled`.
4. Abra `http://localhost:8080/auth/login?return_to=/` no Chrome ou no Firefox. O Safari não aceita cookie `Secure` em `http://localhost`.
5. Para ver quem está logado, abra no mesmo navegador `http://localhost:8080/meurpg.identity.v1.IdentityService/GetMe?connect=v1&encoding=json&message=%7B%7D`.

Chamadas Connect por `curl` agora precisam do header `-H 'Connect-Protocol-Version: 1'` (proteção contra CSRF, ver [Arquitetura](docs/arquitetura.md#csrf)).

Testes do login:

- `make test` roda tudo com um provedor OIDC falso, dentro do próprio teste. Não precisa de rede.
- Com `MEURPG_TEST_DATABASE_URL` apontando para um CockroachDB (por exemplo `postgresql://root@localhost:26257/defaultdb?sslmode=disable`), os mesmos testes também rodam contra o banco.
- Com as variáveis `MEURPG_TEST_OIDC_*` (a lista está no comentário de `TestRealProviderSignIn`, em `backend/internal/identity`), um teste faz o login de verdade num provedor OIDC local, sem navegador.

## Login local com o devidp

O `make up` já sobe o login pronto: o serviço `idp` do `compose.yaml` roda o **devidp** (`backend/cmd/devidp`), um provedor OpenID Connect mínimo, só para a sua máquina e para o CI, no lugar do Google.

1. `make up`.
2. Abra `http://localhost:8080/auth/login?return_to=/` no Chrome. O Safari não aceita cookie `Secure` em `http://localhost`, e o Firefox ainda não foi testado com o devidp.
3. O navegador vai para `http://idp.localhost:9090`, que lista os usuários de teste. Um clique e você volta logado.

| Usuário de teste | `sub` | E-mail | Para quê |
| --- | --- | --- | --- |
| Mestre Teste | `devidp-mestre` | `mestre@example.com` (verificado) | O mestre dos testes. |
| Jogador Teste | `devidp-jogador` | `jogador@example.com` (verificado) | Uma segunda pessoa, para os testes com jogador. |
| E-mail Não Verificado | `devidp-nao-verificado` | `nao-verificado@example.com` (**não** verificado) | Conferir que um e-mail não verificado não é guardado. |

O `sub` é fixo, então cada usuário de teste cai sempre na mesma conta do banco local. Não há senha: qualquer um que alcance o devidp entra como qualquer usuário de teste. Por isso ele **nunca** vai para produção:

- a imagem de produção (`backend/Dockerfile`) só compila `cmd/api` e `cmd/migrate`; o devidp tem imagem própria (`deploy/local/devidp.Dockerfile`), usada só pelo `compose.yaml`, e o CI confere que a imagem de produção não tem o binário;
- ele se recusa a subir se o issuer não estiver num host de loopback (`localhost`, `*.localhost`, `127.0.0.1`, `::1`), ou se estiver no Cloud Run (`K_SERVICE` definido); um teste cobre essa trava;
- ao subir, ele imprime um aviso grande no log.

A configuração vem de flags ou de variáveis de ambiente: `DEVIDP_ISSUER`, `DEVIDP_LISTEN`, `DEVIDP_CLIENT_ID`, `DEVIDP_CLIENT_SECRET` e `DEVIDP_REDIRECT_URIS` (a lista completa e os padrões estão no comentário de `backend/cmd/devidp/main.go`). Para rodar fora do Docker, com as portas 8080 e 9090 livres: `cd backend && go run ./cmd/devidp` num terminal e, em outro, o backend com `OIDC_ISSUER=http://localhost:9090 OIDC_CLIENT_ID=meurpg-local OIDC_CLIENT_SECRET=meurpg-local-secret OIDC_REDIRECT_URL=http://localhost:8080/auth/callback make run` (com a porta 8080 livre).

O devidp também respeita `max_age` e `prompt=login|none`: ele guarda o próprio login num cookie `devidp_session`, então um segundo login no mesmo navegador, dentro de `max_age`, volta direto, sem a lista. O provedor em si fica em `backend/internal/identity/oidctest` e é o mesmo que os testes em Go usam.

**Por que `idp.localhost`.** O issuer precisa ser a mesma string para o navegador e para o container da API, porque o backend confere o `iss` do ID token. O navegador resolve qualquer `*.localhost` para `127.0.0.1` sozinho (RFC 6761) e chega ao devidp pela porta publicada; o container da API resolve o mesmo nome pelo alias de rede do Docker. Detalhes em [Arquitetura](docs/arquitetura.md#testes-e-o-provedor-de-desenvolvimento).

## Imagens da galeria

As imagens que o mestre envia (MR-019) ficam numa pasta, a do `BLOB_DIR`. O `make up` já liga: o `compose.yaml` monta o volume `images` em `/var/lib/meurpg/images`, e ele sobrevive ao `make down` (`docker volume rm meurpg-local_images` apaga as imagens).

| Variável | Obrigatória | O que é |
| --- | --- | --- |
| `BLOB_DIR` | Não | A pasta onde a API guarda as imagens e as miniaturas. Sem ela, as imagens ficam desligadas: `POST /uploads/images`, `GET /images/...` e o `GalleryService` respondem `503`/`unavailable`, o log de início avisa, e o resto do app funciona. Uma pasta sem permissão de escrita impede a API de subir. |

Com `make run`, a API roda no terminal sem imagens; para ligar, `BLOB_DIR=/tmp/meurpg-images make run` (a pasta é criada se não existir). Os testes usam uma pasta temporária cada um.

Para testar o envio com `curl`:

1. Faça login no Chrome em `http://localhost:8080` como "Mestre Teste" e crie uma campanha.
2. Copie o valor do cookie `__Host-meurpg_session` (DevTools → Application → Cookies → `http://localhost:8080`) e o ID da campanha (o que vem depois de `/campanhas/` na URL da página dela). O cookie é a sua sessão: não o cole em lugar nenhum.
3. Envie. Os campos vão nesta ordem, `campaign_id` e depois `file`, e o `-F` do `curl` respeita a ordem:

   ```bash
   SESSION='valor do cookie'
   curl -s -X POST http://localhost:8080/uploads/images \
     -b "__Host-meurpg_session=$SESSION" \
     -F campaign_id=<id da campanha> -F file=@mapa.png
   ```

   A resposta é `201` com a imagem em JSON (`id`, `url`, `thumbnailUrl`...). Um erro vem como `{"code": "...", "reason": "...", "message": "..."}`; os `reason` estão no comentário do `GalleryService` (`proto/meurpg/maps/v1/gallery.proto`) e em [Arquitetura](docs/arquitetura.md#os-erros-do-envio).
4. Baixe: `curl -s -b "__Host-meurpg_session=$SESSION" http://localhost:8080/images/<id> -o imagem` (ou `/images/<id>/thumb`, a miniatura).

O envio não é Connect, então não precisa do `Connect-Protocol-Version`. A proteção contra CSRF (`http.CrossOriginProtection`) julga só os headers que o navegador põe sozinho, `Sec-Fetch-Site` e `Origin`: o `curl` não manda nenhum dos dois, e passa. Com `-H 'Sec-Fetch-Site: cross-site'` ou `-H 'Origin: https://outro.site'`, a resposta é `403`, como seria para uma página de outro site. No app, o navegador manda `Sec-Fetch-Site: same-origin`, que passa.

## Imagens geradas por IA

A geração de imagens (MR-039, RN-28) chama a API do Gemini. Sem chave, fica desligada e o resto do app funciona.

| Variável | Obrigatória | O que é |
| --- | --- | --- |
| `GEMINI_API_KEY` | Não | A chave do Google AI Studio. **Segredo:** nunca vai para o repositório, para um teste, para o log ou para uma mensagem de erro (o backend a mostra como `[REDACTED]`). Sem ela, o log de início diz que a geração está desligada e as chamadas respondem `failed_precondition` com o motivo `OFF` |
| `GEMINI_IMAGE_MODEL` | Não | O modelo; padrão `gemini-3.1-flash-image` |
| `IMAGE_GENERATOR` | Não | `fake` usa o gerador falso (`backend/internal/maps/images/gen`): sem chave e sem rede, devolve um PNG determinístico da proporção pedida. É o que o `make up` (Docker e nativo) usa. Um `[recusa]`, `[vazio]`, `[erro]` ou `[lento]` no texto do pedido faz o falso recusar, não devolver imagem, falhar ou demorar. O servidor se recusa a subir com `fake` no Cloud Run |
| `IMAGE_MONTHLY_LIMIT` | Não | Imagens por campanha por mês; padrão 20 (ver [Operação](docs/operacao.md#imagens-geradas-a-api-do-gemini)) |
| `IMAGE_DAILY_LIMIT` | Não | Imagens do servidor todo por dia de Brasília; padrão 100. É o teto da conta do Gemini (ver [Operação](docs/operacao.md#limites-de-abuso)) |

Os testes usam sempre o gerador falso. O teste com o modelo de verdade, `TestRealGemini` (em `backend/internal/maps/images/gen`), gera uma imagem pequena e só roda com `MEURPG_TEST_GEMINI_API_KEY` definida na sua máquina (e, se quiser, `MEURPG_TEST_GEMINI_MODEL`): sem ela, é pulado, e o CI nunca a define. As medidas de memória (`MEURPG_MEASURE=1`) estão em [Operação](docs/operacao.md#imagens-geradas-a-api-do-gemini). Os testes de integração do módulo: `go test -race -run 'TestMR039|TestRN28|TestRN10_AGeneratedImage' ./internal/maps/`, contra o banco dos testes.

## Limites de abuso

O backend limita a taxa de requisições (por IP e por usuário, em memória) e o que uma conta cria. Os padrões servem a uma mesa e não atrapalham o desenvolvimento; as variáveis são para subir ou descer um teto.

| Variável | Obrigatória | O que é |
| --- | --- | --- |
| `MAX_CAMPAIGNS_PER_USER` | Não | Campanhas de que uma conta pode ser mestre; padrão 10 (RN-30) |
| `CAMPAIGN_CREATORS` | Não | E-mails verificados, separados por vírgula, que podem criar campanhas; vazia, qualquer um cria. No `make up`, o "Mestre Teste" é `mestre@example.com` |
| `RATE_LIMIT_MULTIPLIER` | Não | Multiplica todos os limites de taxa; padrão 1. O `make up` (Docker e nativo) usa 10, porque a suíte e2e manda as requisições de várias contas do mesmo endereço e divide algumas contas entre os workers |

Os números, as respostas (`429` com `Retry-After`, `resource_exhausted`) e a conta de cada limite estão em [Arquitetura](docs/arquitetura.md#limites-de-abuso) e em [Operação](docs/operacao.md#limites-de-abuso). Os testes dos limitadores usam um relógio falso (`go test ./internal/platform/ratelimit`); o teto de campanhas e o de imagens do dia rodam contra o banco, como o resto (`TestRN30_*` em `campaigns`, `TestMR039_TheServersDailyCap` e `TestRateLimitsOfTheImageRoutes` em `maps`).

## Testes ponta a ponta (Playwright)

Os testes de aceite pela tela ficam em `e2e/`, um projeto Playwright em TypeScript, com `package.json` próprio e versões fixas.

- `make e2e` sobe o ambiente local, instala as dependências de `e2e/` se faltarem (`npm ci --ignore-scripts`), roda os testes e mostra onde está o relatório HTML (`cd e2e && npx playwright show-report`).
- Na sua máquina, os testes usam o **Google Chrome instalado** (`channel: 'chrome'`), sem baixar navegador. No CI, usam o Chromium que o `npx playwright install chromium` baixa, na versão presa pelo `package-lock.json`. `E2E_BROWSER_CHANNEL` troca isso (vazio = o Chromium do Playwright, que precisa de `npx playwright install chromium` antes).
- O login passa pelo devidp, clicando no usuário de teste como uma pessoa faria. As RPCs vão com `page.request`, que usa os mesmos cookies da página, e com o header `Connect-Protocol-Version: 1`.
- A suíte loga de verdade só onde logar é o próprio critério de aceite: `login.spec.ts`, `ui.spec.ts` e a metade "visitante sem sessão" de `invite.spec.ts`. Todo o resto reaproveita uma sessão já pronta: o projeto `setup` (`e2e/tests/auth.setup.ts`), que roda antes de tudo (`playwright.config.ts`: o projeto `chrome` depende dele), loga uma vez como "Mestre Teste", uma como "Jogador Teste" e uma como "E-mail Não Verificado" (o segundo jogador dos testes que precisam de duas pessoas na mesa, como os da névoa de guerra) e grava o `storageState` de cada um em `e2e/.auth/` (no `.gitignore`: é sessão, é segredo). Os outros testes usam `newSignedInContext(browser, 'Jogador Teste')` (ou `test.use({ storageState: authStatePath(...) })` para a `page` padrão do arquivo) em vez de logar nas vezes deles — ver `support.ts`. Isso existe porque `/auth/login` tem um limite de 20 por cliente a cada 3 segundos (`backend/internal/identity/login.go`, `loginRateLimit`): logando toda vez, a suíte inteira passava desse limite sozinha, mesmo numa máquina tranquila; com o reaproveitamento, uma rodada completa faz uns 17 logins de verdade, contra os mais de 30 de antes.
  - Um teste que desloga (`SignOut`) nunca reaproveita sessão: desloga a sessão de verdade no servidor, o que quebraria qualquer outro teste que estivesse reaproveitando a mesma.
  - `browser.newContext()` sem `storageState` explícito herda o `storageState` que o arquivo configurou por `test.use()`, se houver — por isso um contexto que precisa começar deslogado (o "visitante" de `invite.spec.ts`) fica num arquivo sem `test.use()` nenhum, com todo contexto explícito sobre o próprio estado.
  - Nenhum teste define o nome de exibição das contas compartilhadas: `ui.spec.ts` espera "Minha conta" (o texto padrão de `user-menu.ts` sem nome definido), que só continua certo enquanto nenhum outro teste passar por `/perfil` nessas contas.
- Cada teste que prova um critério de aceite leva a tag da história ou da regra, como `@MR-001`. `npx playwright test --grep @MR-001` roda só os dela.
- O `a11y.spec.ts` passa o [axe](https://github.com/dequelabs/axe-core) (`@axe-core/playwright`) nas telas principais, no tema claro e no escuro, no desktop e no celular, e falha em qualquer violação séria ou crítica das regras WCAG 2.1 A e AA. Na mesma tela, roda as conferências de alinhamento do `layout.ts`: ícone na altura das próprias palavras, conteúdo de botão no meio dele, nada por cima de um ícone, palavras longe da borda de um cartão de rádio (ver [Design](docs/design.md#como-uma-tela-é-feita)). `npx playwright test --grep @a11y` roda só ele.
- O `ui.spec.ts` faz o login pela tela, clicando em "Entrar" e em "Sair". Os outros testes de login começam direto em `/auth/login?return_to=/`, que é mais rápido e mantém o foco no servidor.

Para adicionar ou atualizar uma dependência de `e2e/`: `cd e2e && npm install <pacote>@<versão>`. O `e2e/.npmrc` já impede scripts de instalação e grava a versão exata.

## Telas: desenho e revisão

Toda tela nova, ou mudança visível numa tela, segue o [design](docs/design.md) do app, a "ficha de papel", e passa por cinco passos:

1. **Brief:** a tarefa da tela, quem usa, onde, os dados reais e todos os estados (vazio, carregando, erro, travado, pendente).
2. **Desenho** no Claude Design, com o sistema visual do app, a 390px e a 1280px, aprovado antes do código. Correção pequena dentro do sistema não precisa de desenho.
3. **Código com os tokens** (`--mr-*` em `web/src/styles.scss`) e as peças comuns (`web/src/styles/_ui.scss`), nunca uma cor escrita à mão.
4. **Revisão pela tela:** prints a 390, 768 e 1280px, no tema claro e no escuro, em cada estado, conferidos com o desenho, e cada peça olhada de perto (2x): alinhamento não aparece num print reduzido. O PR leva os prints de antes e depois e a checklist de telas preenchida.
5. **Conferência automática:** a tela entra no `a11y.spec.ts`, que passa o axe e as conferências de alinhamento do `layout.ts`, sem nenhuma falha.

### As medidas da névoa

A névoa de guerra (MR-036) calcula, por jogador, o que o personagem vê. O `go test -run '^$' -bench FilteredMap -benchmem ./internal/maps` (em `backend/`) mede só a parte de cálculo de uma leitura filtrada de um jogador (a visão, os estados por quadrado e as camadas filtradas), sem o banco: bem abaixo de 1 ms com a cena e as visões no cache (a leitura de sempre), e também sem o cache nos mapas medidos. O `TestFogGetMapTiming` mede o `GetMap` inteiro, com o banco, e mostra os tempos com `-v`. Medido em 04/10/2026 (Apple M1 Pro):

| Mapa | Com cache (leitura de sempre) | Sem cache (depois de um movimento) |
| --- | --- | --- |
| A caverna (24 × 16, uma tocha, 6 jogadores) | 3,5 µs, 1 KB | 9,4 µs, 1,5 KB |
| 60 × 40, 6 jogadores e 3 luzes | 34 µs, 5,8 KB | 166 µs, 8,5 KB |

Memória (`TestSceneMemory -v`): uma cena compilada de 200 × 400 ocupa cerca de 430 kB e cada visão, 85 kB. O cache guarda 8 cenas com até 24 visões cada: no máximo 19 MB, dentro do orçamento de 512 MiB.

### As medidas dos tiles da névoa

O jogador de um mapa com névoa recebe a imagem em peças montadas no servidor (MR-036, RN-10; [Arquitetura](docs/arquitetura.md#as-peças-da-imagem-por-jogador-etapa-9-fatia-95)). Os números abaixo são de um Apple M1 Pro, imagens de teste (um gradiente com textura; uma foto de verdade comprime pior, e as peças ficam maiores); o servidor de produção tem 1 vCPU, então conte com uns 2 a 3 vezes isso no tempo.

| O que | Medida |
| --- | --- |
| Primeira peça da caverna (240 × 160 px, 24 × 16 quadrados) | Cerca de 2 ms; uma peça de 2 kB; 1 MB de memória |
| Primeira peça de um envio de 4.096 × 2.048 px (64 colunas) | Cerca de 150 ms (decodificar e encolher quadrado por quadrado, a maior parte) e 11 ms de renderização; a cópia de trabalho fica com 2.048 × 1.024 = 8,4 MB; pico de 46 MB de memória durante a decodificação |
| Primeira peça de um envio de 8.192 × 4.096 px | Cerca de 460 ms; a cópia de trabalho também tem 8,4 MB; pico de 146 MB (a imagem decodificada, 4 bytes por pixel: um envio pode ter até 40 megapixels, uns 175 MB) |
| Uma peça pronta, da cópia de trabalho (`BenchmarkTileWarm`, uma peça toda vista de 4.096 × 2.048, PNG) | 10 ms, 2,5 MB de alocação |
| A mesma num mapa JPEG (`BenchmarkTileWarmJPEG`, com a conta dos blocos) | 22 ms, 4,2 MB de alocação |
| Uma peça fria (`BenchmarkTileCold`: decodificar o PNG, encolher e renderizar) | 128 ms, 44 MB de alocação |
| Um pedido de uma peça que já está no cache, pela rota inteira (`TestTileRequestTiming`, com o HTTP do próprio teste e o banco em memória) | 6,7 ms |
| Um `304` (`If-None-Match`), pela rota inteira | 4,8 ms: não renderiza nem decodifica nada; é o custo das três leituras do banco que conferem a sessão, a participação e se o jogador vê o mapa |

**O pior caso da memória**, contra o `GOMEMLIMIT` de 400 MiB (e os 512 MiB da instância), com um envio de 40 megapixels em PNG de 8 bits (o maior que o servidor guarda):

| Parte | Memória |
| --- | --- |
| Uma imagem decodificada na vaga do envio (`processing`: uma por vez no servidor, sem coincidir com outra): a primeira peça da névoa (a imagem decodificada, o arquivo de até 10 MiB e a cópia que `io.ReadAll` faz), o envio e a referência para a IA (MR-039). Medido com 40 megapixels em PNG de 8 bits: a primeira peça uns 195 MB; o envio (`images.Process`) **178 MiB** mais o arquivo lido e a cópia dele (até uns 20 MB), uns **200 MB**; encolher a referência 172 MiB (o arquivo é lido uma vez, num buffer do tamanho exato). **O número do envio estava subestimado e foi medido e corrigido nesta mudança:** media-se 230 MiB, porque a miniatura e a referência usavam o espaço de trabalho do reescalonador do tamanho da imagem (77 MB e 163 MB); agora escalam em faixas de 32 linhas (uns 8 MB). A imagem de 40 megapixels em JPEG: 78 MiB. **O recorte do mapa com textura** (`images.CropFit`, MR-039), que usa a mesma vaga: recusa uma resposta do modelo de mais de 4.096 px de lado ou que, decodificada com a cópia de trabalho da saída, passe de 178 MiB; o pior caso aceito (uma resposta de 4.096 × 4.096 para uma imagem de mapa de 16 megapixels) mediu **151 MiB** vivos (`TestCropFitMemory`), dentro dos uns 200 MB da vaga e sem somar com eles | até uns **200 MB**, passageiros |
| As cópias de trabalho: 2 mapas de até 2.048 × 2.048 (16,8 MB no pior caso de proporção) | até 34 MB |
| O cache de peças prontas | 32 MiB (33,5 MB), contados em bytes |
| As cenas e visões da névoa por jogador (`TestSceneMemory`) | até 19 MB |
| As respostas da geração de imagens (MR-039): **1 chamada ao modelo por vez** (`maxGenerating`), uma resposta de até 8 MiB, lida direto para uma estrutura (medido: cerca de 2,5 vezes a resposta, uns 20 MiB no teto; uma imagem de 1K tem uns 2 MB, e na prática uns 5 MiB), e as imagens encolhidas do pedido, até uns 6 MiB | até uns 26 MB |
| Soma | 200 + 34 + 33,5 + 19 + 26 = **uns 313 MB**, mais uns 50 MB do resto do servidor (**uns 363 MB**): dentro dos 400 MiB do `GOMEMLIMIT` e dos 512 MiB da instância. Com 2 chamadas ao mesmo tempo seriam uns 389 MB, e por isso é 1 |

Uma imagem cuja decodificação passaria de 192 MiB (um PNG de 16 bits de mais de 24 megapixels, de antes de o envio guardar só 8 bits por canal) é recusada na primeira peça, com `503`; um JPEG de 40 megapixels decodifica em uns 60 MB.

Para medir de novo: `MEURPG_MEASURE=1 go test -run 'TestTileMemory|TestTileRequestTiming' -v ./internal/maps` (este com o banco de teste), e, para a geração de imagens, `MEURPG_MEASURE=1 go test -run 'TestShrinkMemory|TestMeasureMemory' -v ./internal/maps/images/...` e `go test -run '^$' -bench 'Tile' -benchmem ./internal/maps` (em `backend/`, sem `-race`: o detector multiplica o tempo e a memória).

### Os desenhos das imagens geradas de um mapa

O desenho que vai ao modelo (`backend/internal/maps/refimg`) e o recorte do resultado (`images.CropFit`) são puros: `go test ./internal/maps/refimg ./internal/maps/images` roda a conta da proporção (qual das 10 do modelo, o preenchimento com rocha e o recorte de volta), o preto onde ninguém vê, os discos das criaturas e o recorte em poucos segundos, sem banco. As medidas do maior mapa (200 × 400) estão em [Operação](docs/operacao.md#imagens-geradas-a-api-do-gemini): `MEURPG_MEASURE=1 go test -run 'TestMeasureTheBiggestMap' -v ./internal/maps/refimg ./internal/maps` (o desenho, sem banco), `-bench BiggestMap -benchmem ./internal/maps/refimg` e `MEURPG_MEASURE=1 go test -run TestCropFitMemory -v ./internal/maps/images` (o recorte do pior caso, uns 150 MiB e alguns segundos).

### O editor do mapa no navegador

Pintar num mapa grande não pode travar a tela: o `PaintedLayers` decodifica a grade inteira uma vez por quadro de animação (`requestAnimationFrame`), não a cada quadrado que o ponteiro atravessa, e os lotes vão ao servidor fora do caminho do desenho (ver [Arquitetura](docs/arquitetura.md#o-editor-do-mapa-etapa-9-fatia-912)). Medido em 05/10/2026 no Chrome, num Apple M1 Pro, com um mapa de 200 × 400 quadrados (uma imagem de 1.200 × 2.400 px e a grade de 200 colunas, o maior que o servidor aceita) e o pincel de 3 × 3: um arrasto de 240 eventos do ponteiro de um lado ao outro do mapa, durante 5,5 s, deu **335 quadros com média de 16,7 ms (60 quadros por segundo), p95 de 16,7 ms e máximo de 16,8 ms**: nenhum quadro perdido. O arrasto foi guiado pelo Playwright, que manda um evento por ida e volta ao navegador; um mouse de verdade manda mais eventos por quadro, e eles se juntam no mesmo quadro. Para medir de novo: um teste Playwright que cria o mapa, entra em "Pintar", escolhe a Parede e o pincel 3 × 3, e registra a duração de cada `requestAnimationFrame` enquanto o `page.mouse` arrasta.

### As medidas do mapa de uma masmorra

Criar o mapa de uma masmorra gerada (MR-010, [Arquitetura](docs/arquitetura.md#o-mapa-de-uma-masmorra-gerada-etapa-10-fatia-106d)) gera, desenha, codifica, guarda os dois arquivos e escreve as linhas. Medido em 06/10/2026 num Apple M1 Pro (a máquina estava ocupada; sem `-race`), com as opções padrão e a semente 11:

| Masmorra | Imagem | A chamada `CreateDungeonMap` inteira | Só o desenho, o PNG, a miniatura e os arquivos (`BenchmarkDungeonImage`) |
| --- | --- | --- | --- |
| 121 × 121 (a maior da página), 134 salas | 2.904 × 2.904 px (24 por quadrado), PNG de 59 KiB, miniatura de 65 KiB | 140 ms | 114 ms, 12 MB alocados |
| 199 × 399 (o maior que a API aceita), 500 salas | 3.980 × 7.980 px (20 por quadrado, 31,8 megapixels), PNG de 223 KiB, miniatura de 130 KiB | 379 ms | 502 ms, 36 MB alocados |

A prévia (`PreviewDungeon`) leva 3 ms em 121 × 121 e 10 ms em 199 × 399. O desenho puro (`go test -run '^$' -bench Render ./internal/maps/dungeonimg`) leva 48 ms e 197 ms; o resto é a codificação do PNG e a miniatura. A miniatura de uma imagem de paleta é uma média por caixas feita em uma passagem (`images.thumbnailPaletted`): o redutor genérico lia a paleta pixel a pixel e levava mais de 2 s. Para medir de novo: `bash dbtest.sh go test -run TestDungeonMapCreationTiming -v ./internal/maps/` (em `backend/`, sem `-race`, que multiplica o tempo) e `go test -run '^$' -bench DungeonImage -benchmem ./internal/maps/`.

## Queries com sqlc

O SQL de cada módulo fica em `backend/internal/<módulo>/queries.sql`, e o sqlc gera os métodos Go tipados num pacote ao lado (`identitydb`, `campaignsdb`, `charactersdb`, `playdb`, `mapsdb`). O schema que o sqlc usa são as próprias migrations do goose, então não existe uma segunda cópia do schema para manter igual. A configuração está em `backend/sqlc.yaml`.

Para mudar uma query ou criar uma:

1. Escreva o SQL em `queries.sql`, com um comentário `-- name: NomeDaQuery :one` (ou `:many`, `:exec`, `:execrows`) em cima.
2. Rode `make sqlc` e faça commit do código gerado junto. O CI gera de novo e falha se aparecer diferença.
3. Escrita passa por `db.InTx`, que repete a transação no erro `40001` do CockroachDB: `s.queries.WithTx(tx).NomeDaQuery(...)`.
   - **Dentro do `InTx`, só `WithTx(tx)`.** Nenhuma leitura pelo pool (`s.queries.X`, `s.pool`, ou uma chamada a outro módulo que leia pelo pool): ela pega uma segunda conexão enquanto a transação segura a primeira, e poucas requisições assim travam o pool inteiro até o contexto acabar (PR #120). Uma chamada entre módulos que lê recebe o `tx` (primeiro argumento depois do `ctx`; `nil` fora de transação); o que não pode ficar dentro da transação é lido antes dela. Ver [Arquitetura → Transações e o pool de conexões](docs/arquitetura.md#transações-e-o-pool-de-conexões). O `dbtest` dá a todo teste um pool de **uma** conexão e falha o teste que fizer isso, com a pilha.

O sqlc fica preso na versão **1.31.1**, porque a versão vai escrita em cada arquivo gerado. O `make sqlc` roda essa versão exata com `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1`, sem instalar nada; a primeira vez leva cerca de um minuto para compilar. O `sqlc` do Homebrew na mesma versão gera o mesmo resultado.

O sqlc lê as migrations com o parser do PostgreSQL. Por isso, migration nova usa SQL que o PostgreSQL e o CockroachDB aceitam: índice numa migration própria, com `CREATE INDEX IF NOT EXISTS`, nunca uma linha `INDEX` dentro do `CREATE TABLE`; coluna coberta com `INCLUDE` (o `STORING` do CockroachDB). Opções do CockroachDB em `WITH (...)`, como o TTL por linha, funcionam. Toda migration precisa poder rodar duas vezes: o teste `TestMigrationsAreSafeToRerun` confere. Todo `ADD COLUMN` na tabela `characters` precisa pôr de novo a expressão de TTL da `00020` no fim, como a `00122` faz, porque o CockroachDB reescreve a expressão ao acrescentar uma coluna e, sem isso, a segunda execução deixa a tabela diferente.

## Conteúdo de regras (SRD)

O módulo `rules` é puro: os testes dele rodam sem banco, sem Docker e sem rede, em cerca de um segundo. É o ciclo rápido para mexer no motor, nos efeitos ou nos nomes.

| Comando (em `backend/`) | O que faz |
| --- | --- |
| `go test ./internal/rules/...` | Roda os testes do motor, do snapshot do SRD e das fórmulas. |
| `go test ./internal/rules -run Golden -update` | Regrava `testdata/golden/pensantus.json` depois de uma mudança intencional nos números. Revise o diff antes do commit. |
| `go test ./internal/rules/formula -run '^$' -fuzz FuzzCompileFormula -fuzztime 30s` | Fuzz das fórmulas: nenhuma entrada pode travar ou derrubar o motor. O CI não roda o fuzz; rode ao mexer em `formula/`. |
| `go test ./internal/rules -run '^$' -bench Derive -benchmem` | Mede um `Derive`, que roda a cada leitura de ficha. |
| `go test ./internal/rules/vision -run '^$' -bench . -benchmem` | Mede a névoa de guerra: a caverna do desenho, um mapa de 60 × 40 e o maior (200 × 400), no escuro e iluminado, com borda e pilares e com paredes esparsas. O orçamento: a caverna bem abaixo de 1 ms, o 60 × 40 abaixo de 10 ms e o 200 × 400 abaixo de 300 ms. |
| `go test ./internal/rules -run 'TestWith\|TestLevelUpSweepTable\|TestDeriveTable\|TestSpellDetailsOfTable\|TestTable\|TestThirdCaster\|TestAlwaysPrepared\|TestRaceBonuses\|TestArchived\|TestMissingTable\|TestRealistic\|TestEffectMenu\|TestClassRefusalsNameTheirField\|TestChangedClassSentences\|TestChangeSentence\|TestStrayEffect\|TestThirdCasterUnder\|TestLevelUpSubclassOffers'` | O conteúdo da mesa (`Content.With`, ver [Arquitetura](docs/arquitetura.md#o-conteúdo-da-mesa-contentwith-mr-025-rn-23-adr-0018-fatia-101b)): as recusas (uma linha por regra), o SRD que não muda, dois conteúdos que não se enxergam, o `With` em paralelo (use `-race`), o conjurador de um terço, a varredura de subida de nível sobre classes da mesa de cada tipo de conjuração, em multiclasse também, o golden de um personagem da mesa, os padrões da tabela de 20 níveis, o menu de efeitos, o campo de cada recusa e as frases de "A classe mudou". Sem banco. |
| `go test ./internal/rules -run TableCharacterGolden -update` | Regrava `testdata/golden/table-character.json` (o personagem da mesa nos níveis 1, 5 e 11). Revise o diff antes do commit. |
| `go test -run '^$' -bench BenchmarkWith -benchmem ./internal/rules` | Mede um `With` com 300 entradas (o orçamento da ADR-0018: uns 25 ms e 4 MB) e um `With` vazio. Rode em `backend/`, sem `-race`. `MEURPG_MEASURE=1 go test ./internal/rules -run TestWithMemory -v` mede a memória que um conteúdo da mesa mantém, e `MEURPG_MEASURE=1 go test ./internal/rules -run TestWithAtTheBudgets -v` confere o tempo de um conteúdo nos limites (abaixo de 50 ms; só com a variável, porque numa máquina ocupada ou no CI um teste de relógio falha à toa). |
| `go test ./internal/rules/puzzle` | As contas dos quebra-cabeças (MR-038): a comparação do enigma e da cifra sem maiúsculas, acentos nem pontuação, a cifra (deslocamento e palavra-chave) de ida e volta, a conferência da sequência e o toque passo a passo, o mínimo de toques de "Apagar as luzes" em GF(2) contra a força bruta (3 × 3 e 4 × 4), os casos do núcleo (4 × 4 e 5 × 5), os começos nunca resolvidos de 3 a 7, a fechadura que dá a volta, a busca dos pilares com as ligações e a semente que dá sempre o mesmo começo. `-bench .` mede o mínimo de 7 × 7 e a busca de 6 pilares. Sem banco. |
| `go test ./internal/rules -run 'Treasure\|Hoard\|Families\|MagicItemValues'` | O gerador de tesouro (MR-044, ver [Arquitetura](docs/arquitetura.md#o-gerador-de-tesouro-mr-044-etapa-10-fatia-1010b)): as faixas de nível, o determinismo, o tesouro fixo de uma semente (`TestTreasureGolden`), as propriedades de milhares de tesouros (nunca um artefato, todo item existe e tem nome em português, os totais batem, o limite de peças), os valores dos itens (a metade do consumível, o pergaminho inteiro, o artefato sem preço) e o carregamento recusando tabela fora de forma. Sem banco. |
| `go test ./internal/rules -run '^$' -fuzz FuzzGenerateTreasure -fuzztime 30s` | Fuzz do gerador de tesouro: nenhuma semente, nível ou modo pode dar pânico, e todo tesouro aceito passa nas propriedades. O CI não roda o fuzz; rode ao mexer nas tabelas ou no gerador. `-bench GenerateTreasure -benchmem` mede um covil do nível 17 (uns 21 µs, 40 alocações). |
| `go test ./internal/rules/grid ./internal/rules/vision` | A geometria do mapa (grade, camadas, linha reta, custo do movimento, alcance, cobertura) e a visão: puros, conferidos contra os números da caverna dos desenhos da Etapa 9. Rodam sem banco. |
| `go test ./internal/rules/dungeon` | O gerador de masmorras (MR-010): propriedades sobre muitas opções (todos os invariantes da especificação), os hashes de ouro, o vetor do gerador de números, a independência dos fluxos e os casos de borda. Puro, sem banco, uns 4 segundos (uns 32 com `-race`); `DUNGEON_SWEEP=1` roda ainda uma varredura de 20 000 combinações. |
| `go test ./internal/rules/dungeon -run Golden -update` | Regrava `testdata/golden.json` (um hash da grade por caso) depois de uma mudança intencional na saída do gerador. Mudar a saída pede subir `Version` em `types.go`; revise o diff antes do commit. |
| `go test ./internal/rules/dungeon -run '^$' -fuzz FuzzOptions -fuzztime 60s` | Fuzz das opções do gerador: nenhuma entrada pode dar pânico, e toda saída aceita passa em todos os invariantes. O CI não roda o fuzz; rode ao mexer no gerador. |
| `go test ./internal/rules/dungeon -run '^$' -bench 'Generate|Worst|PromptOf' -benchmem` | Mede o gerador de 31 × 31 até 199 × 399, os piores casos construídos (`Worst`) e o `PromptOf` no pior caso (`PromptOf`). O orçamento: o maior abaixo de 50 ms (teto de 250 ms) e o `PromptOf` abaixo de 20 ms. Os testes só conferem esses tempos com `MEURPG_MEASURE=1` e sem `-race`, numa máquina tranquila: numa máquina ocupada, ou no CI, um teste de relógio falha à toa. |
| `go test ./internal/rules/encounter` | A conta do montador de encontros (MR-043): o orçamento do grupo pela tabela do SRD 5.2.1, a faixa (abaixo de "Baixa" ainda é "Baixa"; acima de "Alta" é permitido), o ND máximo (o menor nível mais 3), o gerador (determinístico por semente, nunca acima do orçamento nem com ND acima do teto, gasta o que dá, um líder e um grupo de uma ou duas criaturas, sobre muitas sementes, orçamentos e tipos) e as trocas pelo mesmo XP. Puro, sem banco. `go test ./internal/rules -run 'Encounter'` roda a tabela carregada (`effects/encounter_budget.json`) e os números do desenho E10-09 sobre o SRD de verdade. |
| `go test ./internal/rules/encounter -run '^$' -fuzz FuzzGenerate -fuzztime 30s` | Fuzz do gerador de encontros: nenhuma entrada dá pânico, o custo nunca passa do orçamento e a mesma semente dá o mesmo encontro. O CI não roda o fuzz; rode ao mexer no gerador. |
| `go test ./internal/rules/encounter -run '^$' -bench . -benchmem` e `go test ./internal/rules -run '^$' -bench GenerateEncounter -benchmem` | O `GenerateEncounter` mede o gerador sobre as 334 criaturas do SRD de verdade, com a medida do resultado: cerca de 1,1 ms e 2,6 MB por encontro (Apple M1 Pro, 06/10/2026); o `BenchmarkGenerate`, do pacote `encounter`, usa um conjunto de teste de 334 criaturas e é só para comparar mudanças. |

O conteúdo fica em `backend/internal/rules/srd51`:

- `data/` é gerado pelo `cmd/srdimport` a partir do 5e-database (`packages/5e-database/src/2014/en` do repositório `5e-bits/5e-srd-api`), num commit fixado. **Nunca edite à mão:** o `TestSnapshot` compara cada arquivo com o sha256 do `manifest.json`. O diff desses arquivos aparece recolhido no PR (`.gitattributes`); revise o importador e o `manifest.json`. Os arquivos de entrada são 20, e o `5e-SRD-Monsters.json` (as 334 criaturas, MR-037) gera o `monsters.json`, de uns 740 kB (97 kB comprimido); o importador recusa um total diferente de 334. O `5e-SRD-Spells.json` também dá a área de cada magia (`area_of_effect`: a forma e o tamanho em pés, 88 das 319 magias), que vai para `spells.json` como `area_type` e `area_size_ft`; o importador recusa uma forma que não é cone, cubo, cilindro, linha ou esfera e um tamanho que não é múltiplo de 5, e o carregamento (`rules`) confere de novo. Com a área estruturada, o `characters` não lê o texto do SRD para saber se uma magia pega uma área; o texto fica só de reserva, para as magias sem ela (ver [Arquitetura](docs/arquitetura.md#os-alvos-das-magias-e-a-página-magias-etapa-10-fatia-102)). O vigésimo, o `5e-SRD-Magic-Items.json` (os 362 itens mágicos do SRD 5.1, MR-044), gera o `magic-items.json`: por item, a chave (`item:<índice>`), o nome do SRD, a categoria, a raridade (`common`, `uncommon`, `rare`, `very_rare`, `legendary`, `artifact` ou `varies`), se pede sintonização (e de quem, em inglês, como no SRD), as variantes e o texto do SRD em inglês. O importador recusa: um total diferente de 362 (239 itens e famílias, 123 variantes); uma categoria ou uma raridade que não conhece; uma entrada sem texto ou repetida; uma família que lista uma variante que não existe ou que não é variante; uma variante em duas famílias, sem família, com variantes ou com raridade `varies`; e uma raridade `varies` sem variantes. O carregamento (`rules`) confere tudo de novo no que está em `data/`. Entra no mesmo commit fixado, então só o `manifest.json` e o `magic-items.json` mudam no `data/`.
- `effects/` é escrito à mão: os efeitos de cada feature e traço (`<classe>.json`, `races.json`, `backgrounds.json`); os nomes em português (`names_pt.json`), com os 362 itens mágicos como `item:<índice>` e o rótulo de cada restrição de sintonização como `attunement:<restrição>` (o `TestMagicItemNamesPT` falha se faltar um); os itens mágicos de uso único (`consumables.json`: as categorias e os itens, que o carregamento confere contra os itens); e a revisão (`revision.json`).

### Atualizar o SRD para um commit novo

1. Baixe os JSON no commit novo, numa pasta fora do repositório:

   ```bash
   git clone --filter=blob:none --no-checkout https://github.com/5e-bits/5e-srd-api.git /tmp/5e-srd-api
   git -C /tmp/5e-srd-api sparse-checkout set --no-cone packages/5e-database/src/2014/en
   git -C /tmp/5e-srd-api checkout <commit>
   ```

2. Em `backend/cmd/srdimport/main.go`, troque `sourceCommit` e a tabela `inputHashes` (`shasum -a 256 /tmp/5e-srd-api/packages/5e-database/src/2014/en/5e-SRD-*.json`). O importador recusa qualquer arquivo com outro sha256.
3. Gere o snapshot: `cd backend && go run ./cmd/srdimport -src /tmp/5e-srd-api/packages/5e-database/src/2014/en`.
4. Troque o commit no `NOTICE` (o `TestSnapshot` confere).
5. Rode `go test ./internal/rules/...`. O `TestReferences` mostra referências quebradas, o `TestEffectsCoverLevels1To5` e o `TestNamesPT` mostram o que ficou sem efeito ou sem nome, e o golden mostra os números que mudaram.

### Mudar um efeito ou um nome

- Uma versão de conteúdo nunca muda no lugar (ADR-0008). Depois de editar qualquer arquivo de `effects/`, o `TestSnapshot` falha e diz o `revision` e o `sha256` novos para pôr em `effects/revision.json`; o `content_version` passa de `srd51@<commit>+fx.<n>` para `fx.<n+1>`. O mesmo vale para uma mudança em `data/` (o `TestSnapshot` só confere o hash de `effects/`, então a revisão sobe à mão, com o mesmo `sha256`): a área das magias (a fatia 10.2) trouxe a `fx.14`, as correções do ataque múltiplo das criaturas (a fatia 10.9b), a `fx.15`, a tabela de orçamento dos encontros (a fatia 10.9c), a `fx.16`, e os valores dos itens e as tabelas do tesouro (a fatia 10.10b), a `fx.17`.
- Os tipos de efeito são fechados e o carregamento recusa o resto. As fórmulas só usam `level()`, `classLevel("wizard")`, `mod("int")`, `score("int")`, `prof()`, `armor()`, `shield()`, `floor`, `ceil`, `min` e `max` (ver [Arquitetura → Módulo rules](docs/arquitetura.md#módulo-rules-regras-como-dados)).
- `effects/advancement.json` guarda as tabelas do SRD de experiência: o XP de cada nível (1 a 20) e o XP de cada nível de desafio (0 a 30). O carregamento recusa uma tabela fora do formato, e mudar qualquer número segue a regra da revisão acima.
- `effects/encounter_budget.json` guarda a tabela "XP Budget per Character" do SRD 5.2.1 (as regras de 2024, CC BY 4.0, p. 201): o XP por personagem de cada nível (1 a 20) nas faixas baixa, moderada e alta, com a fonte no arquivo. O carregamento recusa um nível que falta ou fora de ordem, uma linha que não cresce de baixa para moderada para alta, um nível que custa menos que o anterior em alguma faixa e um campo desconhecido, e o arquivo entra na regra da revisão acima (a `fx.16` o trouxe). É uma das três tabelas do SRD 5.2.1 que o app usa (as outras são os jeitos de fazer atributos e os valores dos itens mágicos por raridade), cada uma creditada no `NOTICE` e rotulada "SRD 5.2.1 (regras de 2024)" na tela.
- `effects/standard_actions.json` guarda as dez ações que todo personagem tem (Atacar, Disparada...); os nomes em português dos recursos ficam em `names_pt.json` como `resource:<nome>`. Os dois entram na regra da revisão acima.
- `effects/spells.json` guarda o que as magias que leem PV fazem (Sono, Borrifo de Cores, Palavra de Poder: Atordoar e Matar, Poupar os Moribundos, Cura Completa), em quatro tipos fechados: `hp_pool`, `hp_threshold`, `zero_hp_target` e `flat_heal` (ver [Arquitetura → Magias que leem PV](docs/arquitetura.md#magias-que-leem-pv)). O carregamento recusa uma magia que não é do SRD, um tipo desconhecido, um campo que o tipo não usa e uma condição que não existe, e o arquivo entra na regra da revisão acima. Uma magia nova desse tipo também precisa de uma conta em `rules/combat/hpspells.go` e do teste dela. Magia fora do SRD 5.1 (como Dobre pelos Mortos) nunca entra. O tipo `ignores_cover` (sem outros campos) marca a magia cujo teste de resistência não recebe bônus de cobertura, como a Chama Sagrada (SRD 5.1; ver [Movimento no combate](docs/arquitetura.md#movimento-no-combate-etapa-9-fatia-96)); não lê PV, e `Content.IgnoresCover` o responde.
- O mesmo arquivo tem o quinto tipo, `summon`, das magias que convocam criaturas (Encontrar Familiar, Animar os Mortos, Conjurar Animais; Encontrar Montaria fica de fora, pergunta 74). Ou ele lista as criaturas (`creatures`, com `count`, `count_per_level` e o que cada uma pode fazer, `attack`: `none`, `reaction` ou `full`, e `unlocks` para uma feature, como o Pacto da Corrente, que soma formas e dá a todas o `attack` do desbloqueio), ou dá faixas de ND de um tipo (`type`, `options` com `count` e `max_cr`, `multiplier_at_level`). O carregamento recusa uma criatura que não existe (e uma ficha de criatura com contagem de ataque menor que 1, ação de Ataque Múltiplo que não existe ou armadura que não é equipamento), um ND que não é do SRD, um campo desconhecido e um tipo que não é de criatura. O tempo de conjuração, o ritual e a concentração vêm da magia, não do arquivo. Uma magia nova desse tipo precisa do teste dela em `rules/summon_test.go`.
- `wild_shape` é o tipo de efeito da Forma Selvagem do druida, em `effects/druid.json`, nos três níveis (2, 4 e 8): `max_cr` (um ND do SRD, como `"1/4"`), `no_fly` e `no_swim`. O carregamento recusa um `max_cr` que não é ND, qualquer outro campo no efeito, e `max_cr`, `no_fly` ou `no_swim` em outro tipo. Os nomes das criaturas (`monster:<índice>`, 334) ficam em `effects/names_pt.json`, e o `TestNamesPT` confere os 334. Nenhuma criatura de outro livro entra. Os nomes em português dos ataques das criaturas que um personagem pode ter (as feras até ND 2, os mortos-vivos de Animar os Mortos e as formas do familiar) são `attack:<nome do SRD em minúsculas, com hífens>` (`attack:bite` é "Mordida"): o servidor os põe em `Attack.name_pt` e em `CreatureAction.name_pt`, o `TestAttackNamesPT` confere que nenhum ficou sem nome e o carregamento recusa um `attack:` que nenhuma criatura usa; o texto do ataque (`notes`) continua o do SRD, em inglês. Uma mudança aqui segue a regra da revisão acima (revisão 11 trouxe os 23 nomes; a 13 trouxe os que faltavam, e o `TestAttackNamesPT` agora confere todos os ataques das 334 criaturas, porque o "Criar NPC" do bestiário os copia para a ficha).
- `effects/traps.json` guarda as oito armadilhas de exemplo do SRD (Fosso simples, Fosso escondido, Agulha envenenada, Dardos envenenados, Teto que desaba, Estátua que cospe fogo, Rede que cai, Esfera rolante), com as tabelas de gravidade do SRD (CD e bônus de ataque; dano por nível). Cada armadilha tem `kind`, `description_pt` (nas nossas palavras), a CD para notar (`notice_dc`, opcional) e para achar (`find_dc`; o SRD só dá a CD para notar no Fosso simples, no Teto que desaba, na Estátua e na Rede, então a de achar é nossa e leva `find_dc_ours: true`), o `trigger` (`enter` ou `manual`), o `area_size` (1 a 4), os `targets` (`area` ou `manual`) e as partes do efeito: um `attack`, `damage` que sempre acerta, `conditions` que sempre pegam (os fossos deixam Derrubado) e um `save` (habilidade, CD, `applies_to`, o que acontece ao falhar e `on_pass`: `half` ou `none`). O carregamento recusa campo desconhecido, condição ou tipo de dano que não existe, resistência sem habilidade, "metade" sem dano ao falhar, fosso cuja queda não é 1d6 por 10 ft e armadilha sem nome em `names_pt.json` (`trap:<chave>`), e o arquivo entra na regra da revisão acima. Os números vêm do `5e-SRD-Rules.json` do commit fixado (seção `sample-traps`); só entram armadilhas do SRD.
- `effects/spell_targets.json` guarda o alvo de algumas magias do SRD onde a área estruturada do banco ou o texto erra (`creature`, `creatures` com número, `area`, `self`, `none` e `label_pt`): toda chave é uma magia do SRD, os tipos são fechados, o texto é nosso, e o arquivo entra na regra da revisão acima. As outras magias seguem a área estruturada e, sem ela, o texto (ver [Arquitetura](docs/arquitetura.md#os-alvos-das-magias-e-a-página-magias-etapa-10-fatia-102)).
- `effects/lights.json` guarda as luzes do SRD em pés (`bright_ft` e, a mais, `dim_ft`): Vela, Tocha, Lâmpada, Lanterna coberta, a magia Luz, Chama Contínua e Luz do Dia, com os nomes em `names_pt.json` (`light:<chave>`). A lanterna de foco (um cone) fica de fora: o app só acende círculos. O carregamento recusa raio negativo ou nulo, raio que não é múltiplo de 5 ft, e luz sem nome ou sem duração. O arquivo entra na regra da revisão acima.
- `effects/magic_item_values.json` guarda os valores dos itens mágicos em PO, a tabela "Magic Item Rarities and Values" do SRD 5.2.1 (p. 205, CC BY 4.0, regras de 2024), com a fonte: comum 100, incomum 400, raro 4.000, muito raro 40.000 e lendário 200.000 (o artefato não tem preço), e o divisor do consumível (2). É a única parte do SRD 5.2.1 que o tesouro usa. O carregamento recusa uma raridade sem valor, valores que não sobem, um valor para o artefato e um divisor menor que 1, e o arquivo entra na regra da revisão acima (revisão 16). O Pergaminho de magia não é dividido (decisão de engenharia de 06/10/2026); o código que aplica isso é `Content.MagicItemValue`.
- `effects/treasure.json` guarda **as nossas tabelas** do gerador de tesouro (MR-044): os degraus de valor das gemas (6 PO e três vezes o anterior, até 4.400 PO) e das obras de arte (a mesma razão deslocada meio degrau, de 9 a 6.600 PO), cada um com nomes em português que nós escrevemos (pedras de verdade, agrupadas por quão fáceis de achar; obras de arte inventadas por nós), e, por faixa de nível do grupo (1 a 4, 5 a 10, 11 a 16 e 17 a 20), as moedas do tesouro individual e do de covil (tipo, rolagem como `2d6*100`, chance) e, no covil, quantas gemas, obras de arte e itens mágicos, com os pesos dos degraus e das raridades. O SRD 5.1 não tem tabelas de tesouro aleatório, então os números e os nomes são nossos, sem tabela de livro nem de outro site (o repositório é público). O carregamento recusa faixa que deixa um nível de fora, rolagem que não se lê, moeda, degrau ou raridade que não existe, o artefato e o `varies`, nome repetido, um degrau acima de 10.000 PO, um nome com mais de 60 caracteres, um individual sem moedas, um sorteio com degraus e raridades juntos, e um tesouro que passe de 12 gemas, 6 obras de arte, 6 itens ou de 1.000.000 PO de ouro, e uma raridade sem item; o arquivo entra na regra da revisão acima. As metas de ouro e as escalas estão em [Arquitetura](docs/arquitetura.md#o-gerador-de-tesouro-mr-044-etapa-10-fatia-1010b). Mudar uma tabela ou um nome de item muda o tesouro de toda semente (o `TestTreasureGolden` avisa), então a revisão sobe: uma semente só vale dentro de uma versão do conteúdo.
- `effects/corrections.json` corrige números que o snapshot do 5e-database traz errados em relação às tabelas e ao texto do SRD 5.1: as invocações conhecidas do Bruxo (`corrections`) e o número de ataques da Ação de Atacar de quatro criaturas (`creature_corrections`, tipo fechado: criatura com Multiattack e campo `attacks_per_action`, com a fonte; fx.15). O `TestMultiattackCountsFollowTheSRDText` lê o texto do Multiattack de toda criatura e confere o número do motor, com uma lista explícita das cinco que o texto não deixa ler (cada uma com o motivo). O carregador aplica a correção por cima das linhas da tabela da classe, recusa classe, campo ou nível desconhecido, e o arquivo entra na regra da revisão acima. `data/` nunca se edita à mão.
- Nomes e textos em `effects/` são nossos, em português. Nenhum texto de livro fora do SRD entra aqui: o repositório é público.

## Branches

Crie uma branch a partir da `main`: `feat/`, `fix/`, `docs/` ou `chore/` e um nome curto. Quem não tem acesso de escrita trabalha num fork, como `vfraga/meuRPG`.

## Commits

Commits em inglês no padrão [Conventional Commits](https://www.conventionalcommits.org/) (proposta, ainda não é regra fechada):

```
feat(characters): lock sheet on first session
```

## Do código ao merge

1. Crie a branch (ver "Branches" acima).
2. Escreva o código, os testes e a documentação no mesmo PR.
3. Faça os commits em Conventional Commits.
4. Abra o PR citando a história (`MR-006`) e a regra (`RN-01`) que ele cumpre. Use o [modelo de PR](.github/pull_request_template.md).
5. O CI precisa ficar verde. Depois o Samuel revisa e faz o squash merge. Até o MVP, quem abriu o PR faz o squash merge assim que o CI fica verde, sem esperar a revisão.

Fluxo resumido: fork, se for o caso → PR para `PuraFome/meuRPG` → CI verde → revisão do Samuel → squash merge.

**Trem de merge.** PRs prontos ao mesmo tempo podem entrar como um trem: cada branch parte da anterior, e os conflitos se resolvem uma vez só. Os PRs do trem entram com merge commit (`gh pr merge N --merge`), não com squash, porque as fatias acrescentam nos mesmos lugares (`e2e/tests/a11y.spec.ts`, `docs/design.md`) e um squash faria o PR seguinte conflitar de novo. Nunca force-push. Decidido pelo Vinicius em 06/10/2026 e usado nos PRs #173 a #177. Um PR sozinho continua entrando com squash.

## O que o CI confere

| Job | Verificações |
| --- | --- |
| backend | `buf lint`, `buf format` e `buf breaking`; código gerado igual ao dos `.proto` e ao das queries (`make sqlc`); `golangci-lint`; `go test -race`; `govulncheck` (dependências com falhas conhecidas); build da imagem Docker. |
| web | `npm ci --ignore-scripts` em `web/`, testes e build do Angular. |
| backend (`go-db`) | `go test -race` com `MEURPG_TEST_DATABASE_URL` apontando para um CockroachDB de verdade (a mesma imagem, presa pelo mesmo digest, do `compose.yaml`), então os testes de integração rodam em vez de serem pulados. |
| e2e | Em quatro máquinas ao mesmo tempo (os testes de acessibilidade, `@a11y`, que são os longos, em duas, e o resto em outras duas, com `--grep`, `--grep-invert` e `--shard`), sobe o ambiente local com `docker compose up --build` (com o `deploy/local/compose.ci.yaml`, que deixa o banco na memória), confere que a imagem de produção não tem o devidp (numa delas) e roda os testes Playwright de `e2e/` no Chromium. Se falhar, mostra os logs do ambiente e guarda o relatório do Playwright daquela parte como artifact por 7 dias (`playwright-report-0` a `-3`). Mudança só em documentação (`docs/`, arquivos `.md`) não roda esse job. |

Toda action do GitHub fica presa pelo SHA do commit, não pela tag. Quem controla uma action consegue mover uma tag para um código malicioso, mas não consegue mudar um SHA. A máquina de cada job também fica presa na versão do Ubuntu (`runs-on: ubuntu-26.04`), nunca em `ubuntu-latest`, que muda de versão sozinha: o GitHub passa o `ubuntu-latest` para o Ubuntu 26.04 entre 19/10 e 19/11/2026, e o CI já roda nele desde 07/10/2026.

O Dependabot (`.github/dependabot.yml`) abre toda semana os PRs que mantêm essas travas em dia: as actions (o SHA e o comentário com a versão), os módulos Go, os pacotes npm do `web/` e do `e2e/` e as imagens base dos Dockerfiles. Versões menores e correções chegam juntas, num PR por grupo. Não chegam pelo Dependabot, e são feitas à mão:

- uma major do Angular, do TypeScript ou do vitest do `web/`, que vem com o `ng update` e as migrações dele (o builder do Angular aceita uma major de cada vez);
- uma major do `@types/node`, que acompanha a versão do Node em que o código roda (22, no CI e no `backend/Dockerfile`);
- uma versão nova do Node ou do Go nas imagens, que muda junto com o CI e o `go.mod`;
- a imagem do CockroachDB, que o `deploy/local/compose.yaml` e o job `go-db` prendem pelo mesmo digest.
- a versão do Ubuntu das máquinas do CI (`runs-on`), que muda junto em todos os jobs, testada num PR antes.

O `package.json` da raiz é o do [app antigo](docs/app-antigo.md) (`src/`, descontinuado) e não recebe atualização.

## Tipos de teste

| Tipo | Ferramenta | Cobre |
| --- | --- | --- |
| Unitário | `go test`, com tabelas de casos | As contas do módulo `rules` e cada regra de negócio isolada. |
| Integração | `go test` + CockroachDB (no Docker ou, no Mac, nativo) | Queries do sqlc, migrations, a repetição no erro `40001` e quem pode fazer o quê. |
| Ponta a ponta | Playwright + um provedor OIDC local | Os critérios de aceite, pela tela, como o usuário faria. |
| Acessibilidade | axe (`@axe-core/playwright`) no Playwright | Cada tela principal, no tema claro e no escuro: nenhuma violação séria ou crítica de WCAG 2.1 A e AA. |

**Teste de aceite.** Cada critério de aceite de uma história (`docs/produto/historias.md`) vira um teste automático desse tipo: `go test` para a regra que roda no servidor, Playwright para o que aparece na tela. Uma história só está pronta quando os testes dela passam. Não há teste de caracterização do app antigo — ele é descontinuado, e o sistema novo só precisa provar os próprios critérios.

## Como manter a documentação

A documentação muda no mesmo PR que o código. Um PR que muda comportamento sem mudar o documento correspondente volta na revisão.

| Mudança no PR | Atualize |
| --- | --- |
| Regra nova ou regra mudada | `docs/produto/regras.md` (um RN novo) e a história afetada |
| História nova ou prioridade mudada | `docs/produto/historias.md` |
| Tela ou fluxo novo | Critérios de aceite e o teste Playwright deles; o desenho aprovado e os prints no PR (ver [Telas](#telas-desenho-e-revisão)) |
| Cor, fonte, espaço ou componente visual | `docs/design.md` e o sistema visual no Claude Design |
| Tabela ou coluna | Migration e `docs/dados.md`, com o diagrama |
| Serviço ou mensagem da API | Comentários no `.proto`; a referência é gerada sozinha |
| Módulo novo, serviço externo ou infraestrutura | `docs/arquitetura.md` e, se for difícil de desfazer, um ADR |
| Deploy, segredo, alerta ou custo | `docs/operacao.md` |
| Comando ou ferramenta nova | `README.md` ou `CONTRIBUTING.md` |
| Termo novo | `docs/produto/glossario.md` |
| Dado pessoal, log, cookie, imagem ou fornecedor novo | `docs/privacidade.md` (inventário e operadores) e o checklist de privacidade no PR |
| Dependência, fonte, ícone ou conteúdo de terceiros novo | Licença compatível com a Apache 2.0; o `NOTICE`, a licença em `third_party/licenses/` e a página "Créditos" do app, quando a licença pede atribuição (ver [Licença](#licença)) |

Como escrever:

- A primeira frase de cada seção é a resposta. O contexto vem depois.
- Frases curtas, em português, com termos de software em inglês.
- Fluxo, estados ou relações viram diagrama Mermaid. Itens comparados viram tabela.
- Um assunto por arquivo. Os outros apontam por link, sem copiar.
- Todo número tem unidade e fonte. O que ninguém sabe ainda vira pergunta em aberto (`docs/produto/perguntas-em-aberto.md`), não chute.

Modelo de história novo (`docs/produto/historias.md`):

```markdown
## MR-0xx: Título curto

**Como** papel, **quero** o que a história entrega,
**para** o motivo.

- Prioridade: MVP | MVP (pré-requisito) | Depois | Existia no app antigo
- Regras: RN-xx ou —
- Módulos: nome dos módulos envolvidos

### Critérios de aceite
- **Dado** ... **quando** ... **então** ...

### Dúvidas
- ...
```

Modelo de ADR (para o repositório privado de ADRs) e mais contexto sobre por que cada documento existe: ver [docs/README.md](docs/README.md).

## Licença

O MeuRPG é distribuído sob a [Apache License 2.0](LICENSE). Todo PR entra sob a mesma licença (seção 5 da Apache 2.0), então só envie código que você escreveu ou que tem licença compatível.

Código sob licença não compatível (por exemplo, CC BY-NC) nunca é copiado: se uma ideia vale, uma pessoa (ou um agente) escreve uma especificação do comportamento, com as nossas palavras, e outra implementa só a partir dela (sala limpa).

Conteúdo de terceiros mantém a própria licença, e o [NOTICE](NOTICE) lista cada um: o SRD 5.1 (CC BY 4.0), os dados do 5e-database (MIT), as fontes (OFL 1.1) e os ícones (Apache 2.0). Uma dependência ou um conteúdo novo que peça atribuição entra no `NOTICE`, com a licença inteira em `third_party/licenses/` e uma linha na página "Créditos" do app.

## Guias de uso

Os guias de uso (guia do mestre, guia do jogador e perguntas frequentes) começam quando as telas do MVP estabilizarem, num site de documentação separado do código.

## Ver também

- [docs/README.md](docs/README.md): índice de toda a documentação.
- [README.md](README.md): visão geral do projeto.
