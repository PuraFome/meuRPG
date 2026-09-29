# Contribuindo com o MeuRPG

Todo código entra por PR para `PuraFome/meuRPG`, com o CI verde e a aprovação do Samuel. Nada vai direto para a `main`.

Os comandos e o CI abaixo passam a existir quando a Etapa 1 (ver [roadmap](docs/roadmap.md)) for integrada.

## Ambiente local

Ferramentas: Go 1.27, buf, sqlc, goose, golangci-lint, Docker e Node 22. No Mac, todas instalam pelo Homebrew.

| Comando | O que faz |
| --- | --- |
| `make up` | Sobe o CockroachDB (um nó só) e o backend com Docker Compose (`deploy/local/compose.yaml`); serve o app em `http://localhost:8080`, servidor e API na mesma origem. |
| `make run` | Roda o backend direto no terminal, apontando para o banco do `make up`. |
| `make proto` | Gera o código Go **e** o TypeScript a partir dos `.proto` (`backend/gen` e `web/src/gen`). Instala as dependências do `web/` sozinho, se faltarem. |
| `make lint` | Roda `buf lint` e `golangci-lint`. |
| `make test` | Roda `go test -race` em todo o backend. |
| `MEURPG_TEST_DATABASE_URL='postgresql://root@localhost:26257/defaultdb?sslmode=disable' make test` | Roda os testes de integração (migrations, transações) contra o CockroachDB do `make up`. Sem a variável, eles são pulados. |
| `make migrate` | Aplica as migrations do goose no banco local. |
| `make down` | Derruba o ambiente local (`docker compose down`). |
| `npm start` | Sobe o Angular antigo (`src/`), descontinuado — mantido só como referência. |
| `make web-install` | Instala as dependências do `web/`: `npm ci --ignore-scripts` (nunca roda scripts de instalação de terceiros). Se for adicionar ou atualizar uma dependência, use `npm install` com o Corepack ativado (`corepack enable`, uma vez só): o `web/package.json` fixa `npm@11.20.0` porque o `npm` de série (10.x) trava ao resolver o grafo de peer dependencies do Vitest 4.1; `npm ci` não tem esse problema e funciona com qualquer um dos dois. |
| `make web-test` | Roda os testes do Angular (`cd web && npm test`). |
| `make web-build` | Builda o Angular para produção (`cd web && npm run build`). |
| `cd web && npm start` | Sobe o Angular sozinho, em modo dev, com `proxy.conf.json` encaminhando as rotas da API (`/meurpg.*`, `/auth`, `/healthz`, `/readyz`) para `localhost:8080`. |

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
5. O CI precisa ficar verde. Depois o Samuel revisa e faz o squash merge.

Fluxo resumido: fork, se for o caso → PR para `PuraFome/meuRPG` → CI verde → revisão do Samuel → squash merge.

## O que o CI confere

| Job | Verificações |
| --- | --- |
| backend | `buf lint`, `buf format` e `buf breaking`; código gerado igual ao dos `.proto`; `golangci-lint`; `go test -race`; `govulncheck` (dependências com falhas conhecidas); build da imagem Docker. |
| web | `npm ci --ignore-scripts` em `web/`, testes e build do Angular. |

Toda action do GitHub fica presa pelo SHA do commit, não pela tag. Quem controla uma action consegue mover uma tag para um código malicioso, mas não consegue mudar um SHA.

## Tipos de teste

| Tipo | Ferramenta | Cobre |
| --- | --- | --- |
| Unitário | `go test`, com tabelas de casos | As contas do módulo `rules` e cada regra de negócio isolada. |
| Integração | `go test` + CockroachDB no Docker | Queries do sqlc, migrations, a repetição no erro `40001` e quem pode fazer o quê. |
| Ponta a ponta | Playwright + um provedor OIDC local | Os critérios de aceite, pela tela, como o usuário faria. |

**Teste de aceite.** Cada critério de aceite de uma história (`docs/produto/historias.md`) vira um teste automático desse tipo: `go test` para a regra que roda no servidor, Playwright para o que aparece na tela. Uma história só está pronta quando os testes dela passam. Não há teste de caracterização do app antigo — ele é descontinuado, e o sistema novo só precisa provar os próprios critérios.

## Como manter a documentação

A documentação muda no mesmo PR que o código. Um PR que muda comportamento sem mudar o documento correspondente volta na revisão.

| Mudança no PR | Atualize |
| --- | --- |
| Regra nova ou regra mudada | `docs/produto/regras.md` (um RN novo) e a história afetada |
| História nova ou prioridade mudada | `docs/produto/historias.md` |
| Tela ou fluxo novo | Critérios de aceite e o teste Playwright deles |
| Tabela ou coluna | Migration e `docs/dados.md`, com o diagrama |
| Serviço ou mensagem da API | Comentários no `.proto`; a referência é gerada sozinha |
| Módulo novo, serviço externo ou infraestrutura | `docs/arquitetura.md` e, se for difícil de desfazer, um ADR |
| Deploy, segredo, alerta ou custo | `docs/operacao.md` |
| Comando ou ferramenta nova | `README.md` ou `CONTRIBUTING.md` |
| Termo novo | `docs/produto/glossario.md` |
| Dado pessoal, log, cookie, imagem ou fornecedor novo | `docs/privacidade.md` (inventário e operadores) e o checklist de privacidade no PR |

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

## Guias de uso

Os guias de uso (guia do mestre, guia do jogador e perguntas frequentes) começam quando as telas do MVP estabilizarem, num site de documentação separado do código.

## Ver também

- [docs/README.md](docs/README.md): índice de toda a documentação.
- [README.md](README.md): visão geral do projeto.
