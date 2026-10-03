# Contribuindo com o MeuRPG

Todo código entra por PR para `PuraFome/meuRPG`, com o CI verde e a aprovação do Samuel. Nada vai direto para a `main`. Até o MVP, a aprovação fica para depois: com o CI verde, quem abriu o PR faz o squash merge, e o PR continua sendo aberto para registrar o que mudou (decidido em 30/09/2026).

Os comandos e o CI abaixo passam a existir quando a Etapa 1 (ver [roadmap](docs/roadmap.md)) for integrada.

## Ambiente local

Ferramentas: Go 1.27, buf, sqlc 1.31.1, goose, golangci-lint, Docker e Node 22. No Mac, todas instalam pelo Homebrew. O sqlc é opcional: o `make sqlc` roda a versão certa sozinho.

| Comando | O que faz |
| --- | --- |
| `make up` | Sobe o CockroachDB (um nó só), o devidp (provedor OIDC de desenvolvimento) e o backend com Docker Compose (`deploy/local/compose.yaml`); serve o app em `http://localhost:8080`, servidor e API na mesma origem, com o login funcionando (ver [Login local com o devidp](#login-local-com-o-devidp)) e as imagens da galeria num volume (ver [Imagens da galeria](#imagens-da-galeria)). |
| `make run` | Roda o backend direto no terminal, apontando para o banco do `make up`. |
| `make db-native-start` / `make db-native-stop` | Liga e desliga um CockroachDB rodando direto no Mac, fora do Docker, para o `LOCAL_DB=native` e os testes de integração. Ver [CockroachDB nativo](#cockroachdb-nativo-mac-opcional). |
| `make proto` | Gera o código Go **e** o TypeScript a partir dos `.proto` (`backend/gen` e `web/src/gen`). Instala as dependências do `web/` sozinho, se faltarem. |
| `make sqlc` | Gera o código Go das queries SQL (`backend/internal/<módulo>/<módulo>db`) com o sqlc 1.31.1. Ver [Queries com sqlc](#queries-com-sqlc). |
| `make lint` | Roda `buf lint` e `golangci-lint`. |
| `make test` | Roda `go test -race` em todo o backend. |
| `MEURPG_TEST_DATABASE_URL='postgresql://root@localhost:26257/defaultdb?sslmode=disable' make test` | Roda os testes de integração (migrations, transações, login, campanhas, convites, personagens, sessões de jogo, a sessão ao vivo, com o stream, a galeria, os mapas, as cenas de RP, as pistas e as anotações dos jogadores, o combate e o XP) contra o CockroachDB do `make up`. Sem a variável, eles são pulados. |
| `make migrate` | Aplica as migrations do goose no banco local. |
| `make e2e` | Sobe o ambiente local (como o `make up`), roda os testes Playwright de `e2e/` contra ele e mostra onde está o relatório. O ambiente continua de pé; `make down` derruba. Ver [Testes ponta a ponta](#testes-ponta-a-ponta-playwright). |
| `make down` | Derruba o ambiente local (`docker compose down`). |
| `npm start` | Sobe o Angular antigo (`src/`), descontinuado — mantido só como referência (ver [App antigo](docs/app-antigo.md)). |
| `make web-install` | Instala as dependências do `web/`: `npm ci --ignore-scripts` (nunca roda scripts de instalação de terceiros). Se for adicionar ou atualizar uma dependência, use `npm install` com o Corepack ativado (`corepack enable`, uma vez só): o `web/package.json` fixa `npm@11.20.0` porque o `npm` de série (10.x) trava ao resolver o grafo de peer dependencies do Vitest 4.1; `npm ci` não tem esse problema e funciona com qualquer um dos dois. |
| `make web-test` | Roda os testes do Angular (`cd web && npm test`). |
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
4. Os testes de integração usam o mesmo endereço de antes, então nada muda: `MEURPG_TEST_DATABASE_URL='postgresql://root@localhost:26257/defaultdb?sslmode=disable' make test`. Na máquina do Vinicius, os testes do `campaigns` caíram de 729 s para 99 s, e os do `play`, de 407 s para 60 s.

O banco do container e o nativo usam a mesma porta: desligue um antes de ligar o outro (`make down` derruba o do container). Os dados deles são separados. Com o banco fora do Docker, dá para diminuir a memória da máquina virtual nas configurações do Docker Desktop ou do Rancher Desktop.

### Os bancos dos testes de integração

Cada teste de integração ganha um banco novo, apagado no fim (`backend/internal/platform/dbtest`). Rodar as migrations do zero em cada um levava uns 25 segundos no CockroachDB, e o CI passou do limite de 10 minutos. Então as migrations rodam uma vez só, num banco-modelo, e cada teste copia as tabelas dele, em uns 3 segundos.

- O banco-modelo se chama `meurpg_tpl_<hash das migrations>`. Uma migration nova ou mudada gera outro.
- Os modelos de migrations antigas ficam no servidor de teste. Estão vazios e podem ser apagados (`DROP DATABASE meurpg_tpl_... CASCADE`).
- As migrations em si continuam testadas do zero pelo pacote `migrations`.

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

## Testes ponta a ponta (Playwright)

Os testes de aceite pela tela ficam em `e2e/`, um projeto Playwright em TypeScript, com `package.json` próprio e versões fixas.

- `make e2e` sobe o ambiente local, instala as dependências de `e2e/` se faltarem (`npm ci --ignore-scripts`), roda os testes e mostra onde está o relatório HTML (`cd e2e && npx playwright show-report`).
- Na sua máquina, os testes usam o **Google Chrome instalado** (`channel: 'chrome'`), sem baixar navegador. No CI, usam o Chromium que o `npx playwright install chromium` baixa, na versão presa pelo `package-lock.json`. `E2E_BROWSER_CHANNEL` troca isso (vazio = o Chromium do Playwright, que precisa de `npx playwright install chromium` antes).
- O login passa pelo devidp, clicando no usuário de teste como uma pessoa faria. As RPCs vão com `page.request`, que usa os mesmos cookies da página, e com o header `Connect-Protocol-Version: 1`.
- A suíte loga de verdade só onde logar é o próprio critério de aceite: `login.spec.ts`, `ui.spec.ts` e a metade "visitante sem sessão" de `invite.spec.ts`. Todo o resto reaproveita uma sessão já pronta: o projeto `setup` (`e2e/tests/auth.setup.ts`), que roda antes de tudo (`playwright.config.ts`: o projeto `chrome` depende dele), loga uma vez como "Mestre Teste" e uma vez como "Jogador Teste" e grava o `storageState` de cada um em `e2e/.auth/` (no `.gitignore`: é sessão, é segredo). Os outros testes usam `newSignedInContext(browser, 'Jogador Teste')` (ou `test.use({ storageState: authStatePath(...) })` para a `page` padrão do arquivo) em vez de logar nas vezes deles — ver `support.ts`. Isso existe porque `/auth/login` tem um limite de 20 por cliente a cada 3 segundos (`backend/internal/identity/login.go`, `loginRateLimit`): logando toda vez, a suíte inteira passava desse limite sozinha, mesmo numa máquina tranquila; com o reaproveitamento, uma rodada completa faz uns 16 logins de verdade, contra os mais de 30 de antes.
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

## Queries com sqlc

O SQL de cada módulo fica em `backend/internal/<módulo>/queries.sql`, e o sqlc gera os métodos Go tipados num pacote ao lado (`identitydb`, `campaignsdb`, `charactersdb`, `playdb`, `mapsdb`). O schema que o sqlc usa são as próprias migrations do goose, então não existe uma segunda cópia do schema para manter igual. A configuração está em `backend/sqlc.yaml`.

Para mudar uma query ou criar uma:

1. Escreva o SQL em `queries.sql`, com um comentário `-- name: NomeDaQuery :one` (ou `:many`, `:exec`, `:execrows`) em cima.
2. Rode `make sqlc` e faça commit do código gerado junto. O CI gera de novo e falha se aparecer diferença.
3. Escrita passa por `db.InTx`, que repete a transação no erro `40001` do CockroachDB: `s.queries.WithTx(tx).NomeDaQuery(...)`.

O sqlc fica preso na versão **1.31.1**, porque a versão vai escrita em cada arquivo gerado. O `make sqlc` roda essa versão exata com `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1`, sem instalar nada; a primeira vez leva cerca de um minuto para compilar. O `sqlc` do Homebrew na mesma versão gera o mesmo resultado.

O sqlc lê as migrations com o parser do PostgreSQL. Por isso, migration nova usa SQL que o PostgreSQL e o CockroachDB aceitam: índice numa migration própria, com `CREATE INDEX IF NOT EXISTS`, nunca uma linha `INDEX` dentro do `CREATE TABLE`; coluna coberta com `INCLUDE` (o `STORING` do CockroachDB). Opções do CockroachDB em `WITH (...)`, como o TTL por linha, funcionam. Toda migration precisa poder rodar duas vezes: o teste `TestMigrationsAreSafeToRerun` confere.

## Conteúdo de regras (SRD)

O módulo `rules` é puro: os testes dele rodam sem banco, sem Docker e sem rede, em cerca de um segundo. É o ciclo rápido para mexer no motor, nos efeitos ou nos nomes.

| Comando (em `backend/`) | O que faz |
| --- | --- |
| `go test ./internal/rules/...` | Roda os testes do motor, do snapshot do SRD e das fórmulas. |
| `go test ./internal/rules -run Golden -update` | Regrava `testdata/golden/pensantus.json` depois de uma mudança intencional nos números. Revise o diff antes do commit. |
| `go test ./internal/rules/formula -run '^$' -fuzz FuzzCompileFormula -fuzztime 30s` | Fuzz das fórmulas: nenhuma entrada pode travar ou derrubar o motor. O CI não roda o fuzz; rode ao mexer em `formula/`. |
| `go test ./internal/rules -run '^$' -bench Derive -benchmem` | Mede um `Derive`, que roda a cada leitura de ficha. |

O conteúdo fica em `backend/internal/rules/srd51`:

- `data/` é gerado pelo `cmd/srdimport` a partir do 5e-database (`packages/5e-database/src/2014/en` do repositório `5e-bits/5e-srd-api`), num commit fixado. **Nunca edite à mão:** o `TestSnapshot` compara cada arquivo com o sha256 do `manifest.json`. O diff desses arquivos aparece recolhido no PR (`.gitattributes`); revise o importador e o `manifest.json`.
- `effects/` é escrito à mão: os efeitos de cada feature e traço (`<classe>.json`, `races.json`, `backgrounds.json`), os nomes em português (`names_pt.json`) e a revisão (`revision.json`).

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

- Uma versão de conteúdo nunca muda no lugar (ADR-0008). Depois de editar qualquer arquivo de `effects/`, o `TestSnapshot` falha e diz o `revision` e o `sha256` novos para pôr em `effects/revision.json`; o `content_version` passa de `srd51@<commit>+fx.<n>` para `fx.<n+1>`.
- Os tipos de efeito são fechados e o carregamento recusa o resto. As fórmulas só usam `level()`, `classLevel("wizard")`, `mod("int")`, `score("int")`, `prof()`, `armor()`, `shield()`, `floor`, `ceil`, `min` e `max` (ver [Arquitetura → Módulo rules](docs/arquitetura.md#módulo-rules-regras-como-dados)).
- `effects/advancement.json` guarda as tabelas do SRD de experiência: o XP de cada nível (1 a 20) e o XP de cada nível de desafio (0 a 30). O carregamento recusa uma tabela fora do formato, e mudar qualquer número segue a regra da revisão acima.
- `effects/standard_actions.json` guarda as dez ações que todo personagem tem (Atacar, Disparada...); os nomes em português dos recursos ficam em `names_pt.json` como `resource:<nome>`. Os dois entram na regra da revisão acima.
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

## O que o CI confere

| Job | Verificações |
| --- | --- |
| backend | `buf lint`, `buf format` e `buf breaking`; código gerado igual ao dos `.proto` e ao das queries (`make sqlc`); `golangci-lint`; `go test -race`; `govulncheck` (dependências com falhas conhecidas); build da imagem Docker. |
| web | `npm ci --ignore-scripts` em `web/`, testes e build do Angular. |
| backend (`go-db`) | `go test -race` com `MEURPG_TEST_DATABASE_URL` apontando para um CockroachDB de verdade (a mesma imagem, presa pelo mesmo digest, do `compose.yaml`), então os testes de integração rodam em vez de serem pulados. |
| e2e | Sobe o ambiente local com `docker compose up --build`, confere que a imagem de produção não tem o devidp e roda os testes Playwright de `e2e/` no Chromium. Se falhar, mostra os logs do ambiente e guarda o relatório do Playwright como artifact por 7 dias. Mudança só em documentação (`docs/`, arquivos `.md`) não roda esse job. |

Toda action do GitHub fica presa pelo SHA do commit, não pela tag. Quem controla uma action consegue mover uma tag para um código malicioso, mas não consegue mudar um SHA.

O Dependabot (`.github/dependabot.yml`) abre toda semana os PRs que mantêm essas travas em dia: as actions (o SHA e o comentário com a versão), os módulos Go, os pacotes npm do `web/` e do `e2e/` e as imagens base dos Dockerfiles. Versões menores e correções chegam juntas, num PR por grupo. Não chegam pelo Dependabot, e são feitas à mão:

- uma major do Angular, do TypeScript ou do vitest do `web/`, que vem com o `ng update` e as migrações dele (o builder do Angular aceita uma major de cada vez);
- uma major do `@types/node`, que acompanha a versão do Node em que o código roda (22, no CI e no `backend/Dockerfile`);
- uma versão nova do Node ou do Go nas imagens, que muda junto com o CI e o `go.mod`;
- a imagem do CockroachDB, que o `deploy/local/compose.yaml` e o job `go-db` prendem pelo mesmo digest.

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
