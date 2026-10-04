<h1 align="center">MeuRPG</h1>

<p align="center">
  Onde uma mesa de D&amp;D 5e prepara e joga as suas campanhas.<br>
  O mestre conduz, o jogador acompanha a ficha, e as regras fazem as contas.
</p>

<p align="center">
  <a href="https://github.com/PuraFome/meuRPG/actions/workflows/backend.yml?query=branch%3Amain"><img alt="backend" src="https://github.com/PuraFome/meuRPG/actions/workflows/backend.yml/badge.svg?branch=main"></a>
  <a href="https://github.com/PuraFome/meuRPG/actions/workflows/web.yml?query=branch%3Amain"><img alt="web" src="https://github.com/PuraFome/meuRPG/actions/workflows/web.yml/badge.svg?branch=main"></a>
  <a href="https://github.com/PuraFome/meuRPG/actions/workflows/e2e.yml?query=branch%3Amain"><img alt="e2e" src="https://github.com/PuraFome/meuRPG/actions/workflows/e2e.yml/badge.svg?branch=main"></a>
</p>

<p align="center">
  <a href="backend/go.mod"><img alt="Go" src="https://img.shields.io/github/go-mod/go-version/PuraFome/meuRPG?filename=backend%2Fgo.mod&amp;logo=go&amp;logoColor=white&amp;label=Go"></a>
  <a href="web/package.json"><img alt="Angular" src="https://img.shields.io/github/package-json/dependency-version/PuraFome/meuRPG/%40angular%2Fcore?filename=web%2Fpackage.json&amp;logo=angular&amp;logoColor=white&amp;label=Angular&amp;color=DD0031"></a>
  <a href="proto/"><img alt="API: Connect + Protobuf" src="https://img.shields.io/badge/API-Connect%20%2B%20Protobuf-4B32C3"></a>
  <a href="docs/dados.md"><img alt="Banco: CockroachDB" src="https://img.shields.io/badge/banco-CockroachDB-6933FF?logo=cockroachlabs&amp;logoColor=white"></a>
  <a href="docs/operacao.md"><img alt="Infra: Cloud Run em São Paulo" src="https://img.shields.io/badge/infra-Cloud%20Run%20%C2%B7%20S%C3%A3o%20Paulo-4285F4?logo=googlecloud&amp;logoColor=white"></a>
</p>

<p align="center">
  <a href="e2e/tests/a11y.spec.ts"><img alt="Acessibilidade: axe, WCAG 2.1 AA" src="https://img.shields.io/badge/a11y-axe%20%C2%B7%20WCAG%202.1%20AA-1F6FEB"></a>
  <a href=".github/dependabot.yml"><img alt="Dependabot ativo" src="https://img.shields.io/badge/Dependabot-ativo-025E8C?logo=dependabot&amp;logoColor=white"></a>
  <a href="https://www.conventionalcommits.org/pt-br/v1.0.0/"><img alt="Conventional Commits" src="https://img.shields.io/badge/Conventional%20Commits-1.0.0-FE5196?logo=conventionalcommits&amp;logoColor=white"></a>
  <a href="NOTICE"><img alt="Regras: SRD 5.1, CC BY 4.0" src="https://img.shields.io/badge/regras-SRD%205.1%20%C2%B7%20CC%20BY%204.0-555555"></a>
  <a href="LICENSE"><img alt="Licença: Apache 2.0" src="https://img.shields.io/github/license/PuraFome/meuRPG?label=licen%C3%A7a&amp;color=D22128"></a>
  <a href="https://github.com/PuraFome/meuRPG/commits/main"><img alt="Último commit" src="https://img.shields.io/github/last-commit/PuraFome/meuRPG/main?label=%C3%BAltimo%20commit"></a>
</p>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/img/ficha-desktop-escuro.webp">
    <img alt="A ficha do Pensantus, Mago 3, travada na sessão: os medalhões de atributo, o escudo da CA, os pontos de vida e o aviso de armadura sem proficiência" src="docs/img/ficha-desktop-claro.webp" width="900">
  </picture>
</p>

## O que é

O mestre prepara o mundo e conduz a sessão ao vivo; o jogador entra por convite, acompanha a ficha e age no RP e no combate com o que as regras permitem. Mais contexto em [Visão do produto](docs/produto/visao.md).

O MeuRPG está sendo reconstruído do zero: um servidor em Go que serve o app Angular e a API na mesma origem, com o CockroachDB, feito para rodar no Cloud Run, em São Paulo. O primeiro deploy vem depois. O app antigo está descontinuado e fica documentado em [App antigo](docs/app-antigo.md).

## O que já funciona

As Etapas 1 a 8 do [roadmap](docs/roadmap.md) estão na `main`:

- **Login** por OpenID Connect (o Google em produção), com sessão de até 30 dias no servidor.
- **Campanhas e convites.** O mestre cria a campanha e gera convites com validade e número de usos. Quem recebe o link entra, logado ou fazendo login no caminho, e o convite pode pedir a aprovação do mestre.
- **Personagens.** A ficha no formato da ficha oficial, calculada pelo motor de regras a partir do SRD 5.1: modificadores, perícias, CA, PV, magias e os avisos de regra, como armadura sem proficiência. O mestre cria NPCs numa ficha curta, com iniciativa e ataques. Na criação, os atributos e os PV podem ser rolados no app, e cada magia tem a descrição completa.
- **Sessão.** Iniciar a sessão trava as fichas dos jogadores; a história do personagem tem uma trava própria.
- **Sessão ao vivo.** Quando o mestre inicia a sessão, quem joga na campanha vê o aviso com o link, em qualquer página do app. Na página da sessão, os PV, os espaços de magia e os dados de vida do personagem mudam na tela do jogador quando o mestre corrige, sem recarregar.
- **Galeria.** O mestre envia as imagens da campanha, e o servidor tira os metadados, como o GPS da foto, antes de guardar.
- **Mapas sem spoiler.** O mestre cria mapas a partir da galeria, com pontos de interesse (batalha, submapa e cena de RP) e os tokens dos personagens, e revela cada coisa na hora certa: o que está escondido nunca sai do servidor para o jogador. Na sessão, o mestre escolhe o mapa atual e move os tokens, e a mesa vê ao vivo. Ele também pode mostrar aos jogadores uma imagem da galeria.
- **Documento da campanha.** As anotações do mestre, em Markdown, com imagens da galeria e links para os mapas e as fichas. Só o mestre vê.
- **Combate.** O mestre põe a grade de 1,5 m no mapa e inicia o combate: iniciativa, ordem dos turnos e movimento, sem o jogador ver quem está escondido nem os números dos inimigos. Na sua vez, o jogador vê o que as regras deixam fazer com a ação, a ação bônus, a reação e o movimento, e ataca, conjura, usa as habilidades de classe e as reações, com o dado do app ou o físico. O mestre aplica o dano, desfaz a última ação, marca as condições e confirma a morte de quem falha três vezes no teste contra a morte. O registro conta a luta, e cada um vê só o que pode ver. Quem tem a mesma iniciativa joga o mesmo turno (o turno conjunto), as distâncias saem em metros e em quadrados, e o fim do combate mostra os destaques.
- **Cenas de RP.** O mestre põe as ações num ponto de cena do mapa (uma perícia, um teste de atributo ou uma salvaguarda, com CD se quiser) e abre a cena na sessão. Cada jogador vê as ações com o próprio bônus e rola no app ou digita o dado físico. O mestre escolhe, por cena, se os jogadores veem a CD e se passaram, e quantas tentativas cada um tem. Ele escreve as pistas e os ganchos do ponto, revela uma pista a quem escolher e põe os NPCs em cena, com o retrato, como num visual novel.
- **Anotações.** O jogador guarda as próprias anotações e as pistas que recebeu, na sessão e na ficha. O mestre não lê.
- **XP.** O mestre dá XP no fim do combate, por ouro ou avulso, ou registra um marco, conforme o modo da campanha, e desfaz o último. Toda a campanha vê o histórico, e a ficha mostra "Pode subir de nível". Na campanha por marcos, o mestre escreve os marcos antes. Quando pode, o jogador sobe o nível pela própria ficha, passo a passo, acrescentando só o que o nível dá, e o mestre vê o que mudou.
- **Resumo da sessão.** Quando o mestre encerra a sessão, todos veem os destaques, e o mestre vê também a tabela de cada jogador.
- **Imprimir o mapa.** O mestre imprime um mapa com a grade na escala da mesa, escolhendo o tamanho do quadrado e o papel.

Agora vem a Etapa 9, o combate e o mapa a fundo (movimento em círculo, armadilhas, névoa de guerra, criaturas do personagem e os tesouros no mapa, com o XP por ouro). O MVP termina na Etapa 10, quando a mesa joga a primeira sessão inteira pelo app; a 10 traz o conteúdo e a geração (regras da mesa, quebra-cabeças, gerador de masmorras e imagens geradas por IA). Ver [Roadmap](docs/roadmap.md).

## O visual: a ficha de papel

As telas imitam a ficha oficial de D&D 5e, na mesma ordem e com as mesmas formas: os medalhões de atributo, o escudo da CA, os pontos de proficiência. O número de jogo é a coisa mais visível da tela. O tema claro e o escuro seguem o sistema operacional, com contraste AA nos dois, e cada tela funciona no celular, na mesa de jogo.

<p align="center">
  <img alt="A campanha Mirathel no celular, pelo mestre, no tema claro: a sessão em andamento, o Pensantus travado e os NPCs" src="docs/img/campanha-celular-claro.webp" width="300">
  &nbsp;&nbsp;
  <img alt="A ficha do Pensantus no celular, no tema escuro" src="docs/img/ficha-celular-escuro.webp" width="300">
</p>

Os princípios, os tokens de cor e fonte, os componentes e como uma tela nova é desenhada e revisada estão em [Design](docs/design.md).

## Como funciona

| Parte | O que usa |
| --- | --- |
| Backend | Go, num monólito modular (`identity`, `authz`, `campaigns`, `rules`, `characters`, `play`), que também é o BFF do app |
| API | Protobuf + [Connect](https://connectrpc.com), em `proto/`; o cliente TypeScript do `web/` é gerado dos mesmos `.proto` |
| Banco | CockroachDB, com as queries no sqlc e as migrations no goose |
| Web | Angular 21 (standalone, zoneless) com Angular Material, no visual da [ficha de papel](docs/design.md) |
| Login | OpenID Connect com PKCE; o navegador só recebe um cookie `__Host-` com uma sessão opaca |
| Regras | Um motor puro sobre o SRD 5.1, com os efeitos em JSON e as fórmulas num sandbox |
| Infra | Cloud Run e CockroachDB no Google Cloud, em São Paulo |

O desenho completo, com os diagramas, está em [Arquitetura](docs/arquitetura.md).

## Como rodar

Precisa de Docker, Go 1.27, Node 22 e `make`. As ferramentas que só geram código ou rodam lint (buf, sqlc, golangci-lint) estão no [CONTRIBUTING](CONTRIBUTING.md#ambiente-local).

```bash
make up      # CockroachDB, migrations, devidp e o app em http://localhost:8080
make test    # testes do backend (go test -race)
make e2e     # testes pela tela (Playwright) e de acessibilidade (axe)
make down    # derruba tudo
```

Abra `http://localhost:8080` no Chrome e clique em "Entrar": o login vai para o **devidp**, um provedor de teste que só existe na sua máquina e no CI, e um clique em "Mestre Teste" volta logado. O Safari não aceita cookie `Secure` em `http://localhost`.

Para mexer só nas telas, com recarga automática, deixe o `make up` rodando e suba o Angular em modo dev, em `http://localhost:4200`:

```bash
cd web
npm ci --ignore-scripts   # nunca roda scripts de instalação de terceiros
npm start
```

Todos os comandos estão em `make help` e no [CONTRIBUTING](CONTRIBUTING.md).

## Estrutura do repositório

| Pasta | O que tem |
| --- | --- |
| `backend/` | O servidor Go: `cmd/` (a API, as migrations, o devidp e o importador do SRD), `internal/<módulo>` e `migrations/` |
| `proto/` | Os contratos da API (`meurpg/<módulo>/v1/*.proto`) |
| `web/` | O app Angular |
| `e2e/` | Os testes Playwright de cada critério de aceite e o `a11y.spec.ts` |
| `deploy/local/` | O Docker Compose do ambiente local |
| `docs/` | A documentação, em português, com os diagramas em Mermaid |
| `.github/` | Os workflows do CI, o Dependabot e o modelo de PR |
| `src/`, `server/` | O [app antigo](docs/app-antigo.md), descontinuado |

## Qualidade

Todo código entra por PR, com o CI verde. O que cada job confere está em [CONTRIBUTING](CONTRIBUTING.md#o-que-o-ci-confere).

- **Cada critério de aceite vira um teste:** `go test` para a regra que roda no servidor, Playwright para o que aparece na tela, marcado com a história (`@MR-001`).
- **Acessibilidade:** o [axe](https://github.com/dequelabs/axe-core) passa nas telas principais, no tema claro e no escuro, no computador e no celular, e o CI falha em qualquer violação séria ou crítica das regras WCAG 2.1 A e AA.
- **Contrato e código gerado:** `buf lint`, `buf format` e `buf breaking` nos `.proto`; o CI gera de novo o código do buf e do sqlc e falha se aparecer diferença.
- **Banco de verdade nos testes:** os testes de integração rodam contra o CockroachDB, na mesma imagem do ambiente local.
- **Tudo com versão presa:** as actions pelo SHA do commit, as imagens pelo digest, os pacotes npm na versão exata e instalados sem scripts. O Dependabot abre toda semana os PRs que mantêm isso em dia, e o `govulncheck` confere as dependências Go.
- **Privacidade:** a regra mais restritiva entre a LGPD e o GDPR, dado a dado, em [Privacidade](docs/privacidade.md). O app não guarda nada no `localStorage` nem no `sessionStorage`.

## Documentação

| Documento | Responde |
| --- | --- |
| [docs/README.md](docs/README.md) | O índice de toda a documentação |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Como rodar, testar e abrir um PR |
| [Visão](docs/produto/visao.md), [Histórias](docs/produto/historias.md), [Regras](docs/produto/regras.md), [Glossário](docs/produto/glossario.md) | O que o app faz, para quem, e como o jogo se comporta nele |
| [Roadmap](docs/roadmap.md) | A ordem das etapas até o MVP e o que já está na `main` |
| [Arquitetura](docs/arquitetura.md), [Dados](docs/dados.md) | Como o sistema é montado, onde roda, as tabelas |
| [Design](docs/design.md) | O visual do app e como uma tela é desenhada e revisada |
| [Privacidade](docs/privacidade.md), [Operação](docs/operacao.md) | Os dados pessoais que guardamos; deploy, segredos e custos |

## Conteúdo de regras e licença

As regras vêm do System Reference Document 5.1 (SRD 5.1), sob a licença Creative Commons Attribution 4.0. O app é compatível com a quinta edição ("5E compatible") e não usa nenhuma marca da editora. A atribuição que a licença exige, com o texto exato, está no [NOTICE](NOTICE) e na página "Créditos" do app:

> This work includes material taken from the System Reference Document 5.1 ("SRD 5.1") by Wizards of the Coast LLC and available at https://dnd.wizards.com/resources/systems-reference-document. The SRD 5.1 is licensed under the Creative Commons Attribution 4.0 International License available at https://creativecommons.org/licenses/by/4.0/legalcode.

- O conteúdo fica embutido no binário, em `backend/internal/rules/srd51`, gerado a partir do 5e-database (MIT) num commit fixado. Nada é buscado em runtime.
- Os nomes em português e os efeitos estruturados são nossos. As descrições do SRD ficam em inglês por enquanto.
- Nenhum texto de livro fora do SRD entra no repositório: o que a mesa usar de outros livros ela cadastra com as próprias palavras.
- Como o motor funciona: [Arquitetura → Módulo rules](docs/arquitetura.md#módulo-rules-regras-como-dados). Como atualizar o SRD: [CONTRIBUTING.md](CONTRIBUTING.md#conteúdo-de-regras-srd).

As fontes (Alegreya e Alegreya Sans, OFL 1.1) e os ícones (Material Symbols, Apache 2.0) também estão no [NOTICE](NOTICE), com as licenças em [`third_party/licenses/`](third_party/licenses/).

O MeuRPG é distribuído sob a [Apache License 2.0](LICENSE). O conteúdo de terceiros listado no [NOTICE](NOTICE) mantém a própria licença: o SRD 5.1 (CC BY 4.0), os dados do 5e-database (MIT), as fontes (OFL 1.1) e os ícones (Apache 2.0). Quem redistribui o MeuRPG, com ou sem mudanças, leva junto o `LICENSE` e o `NOTICE`.

## App antigo (descontinuado)

O Angular em `src/` e o NestJS em `server/` são o app antigo. Não recebem mudanças, e os dois saem do repositório na Etapa 11. O que eles faziam e como rodavam está em [App antigo](docs/app-antigo.md).
