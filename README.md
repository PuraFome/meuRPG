# MeuRPG

O MeuRPG é onde uma mesa de D&D 5e prepara e joga suas campanhas. O mestre prepara o mundo e conduz a sessão ao vivo; o jogador entra por convite, acompanha a ficha e age no RP e no combate com o que as regras permitem. Mais contexto em [docs/produto/visao.md](docs/produto/visao.md).

O MeuRPG novo está sendo construído do zero: um backend em Go (monólito modular, API em Protobuf + Connect, rodando no Cloud Run em São Paulo) e um novo app Angular em `web/`, servido pelo mesmo servidor. Decidido em 29/09/2026: não há migração gradual. O app antigo — Angular em `src/` (GitHub Pages) e NestJS em `server/` (Render) — está descontinuado. O `src/` fica no repositório só como referência, até sair num PR à parte. O `server/` (NestJS) **será removido do repositório** — decidido pelo Samuel em 29/09/2026 —, porque o backend novo em Go cobre sozinho todas as histórias do MVP; até lá, a seção abaixo continua documentando o que existe. Detalhes em [docs/arquitetura.md](docs/arquitetura.md) e o estado de cada etapa em [docs/roadmap.md](docs/roadmap.md).

- Documentação completa: [docs/README.md](docs/README.md)
- Como rodar, testar e abrir um PR: [CONTRIBUTING.md](CONTRIBUTING.md)

## Como começar rápido

Stack nova (Go + Angular em `web/`, na mesma origem), com CockroachDB local no Docker:

```bash
make up      # sobe CockroachDB, aplica as migrations e sobe o app (API + Angular) em localhost:8080
make test    # testes do backend
make down    # derruba tudo
```

Só o frontend novo, em modo dev (`web/`), apontando para uma API já rodando em `localhost:8080` via `make up` ou `make run`:

```bash
cd web
npm ci --ignore-scripts   # nunca roda scripts de instalação de terceiros
npm start
```

Todos os comandos estão em `make help` e no [CONTRIBUTING.md](CONTRIBUTING.md). Os comandos do app antigo (descontinuado, só para consulta) estão na seção abaixo.

## Conteúdo de regras e licença

As regras vêm do System Reference Document 5.1 (SRD 5.1), sob a licença Creative Commons Attribution 4.0. O app é compatível com a quinta edição ("5E compatible") e não usa nenhuma marca da editora. A atribuição que a licença exige, com o texto exato, está no [NOTICE](NOTICE) e na página "Créditos" do app:

> This work includes material taken from the System Reference Document 5.1 ("SRD 5.1") by Wizards of the Coast LLC and available at https://dnd.wizards.com/resources/systems-reference-document. The SRD 5.1 is licensed under the Creative Commons Attribution 4.0 International License available at https://creativecommons.org/licenses/by/4.0/legalcode.

- O conteúdo fica embutido no binário, em `backend/internal/rules/srd51`, gerado a partir do 5e-database (MIT) num commit fixado. Nada é buscado em runtime.
- Os nomes em português e os efeitos estruturados são nossos. As descrições do SRD ficam em inglês por enquanto.
- Nenhum texto de livro fora do SRD entra no repositório: o que a mesa usar de outros livros ela cadastra com as próprias palavras.
- Como o motor funciona: [Arquitetura → Módulo rules](docs/arquitetura.md#módulo-rules-regras-como-dados). Como atualizar o SRD: [CONTRIBUTING.md](CONTRIBUTING.md#conteúdo-de-regras-srd).

## App antigo (descontinuado)

O `src/` (Angular) fica só como referência até sair do repositório num PR à parte. O `server/` (NestJS) será removido do repositório (decidido pelo Samuel em 29/09/2026): o backend novo em Go passa a cobrir sozinho todas as histórias do MVP, então não faz sentido manter os dois. A subseção "Backend (NestJS)" abaixo documenta o que existe até essa remoção.

### Frontend (Angular)

This project was generated using [Angular CLI](https://github.com/angular-cli) version 21.2.19.

```bash
npm install
npm start
```

### Backend (NestJS)

Um simples backend NestJS 11 foi criado em `server/`, implementando login Google OAuth 2.0 com PKCE (S256), sessões com bearer token (hash SHA256 no CockroachDB) e exclusão de conta conforme a LGPD.

#### Features
- Google OAuth 2.0 Authorization Code + PKCE flow
- Bearer token session management (SHA256 hashed in CockroachDB)
- Account deletion with cascading deletes (LGPD Art. 16)
- CockroachDB connection via `pg` driver
- Docker multi-stage build + docker-compose
- `.env.example` with all required variables
- API endpoints:
  - `GET /api/auth/google` - Initiate Google OAuth login
  - `GET /api/auth/callback` - Google OAuth callback handler
  - `POST /api/auth/logout` - Logout and invalidate session
  - `DELETE /api/auth/account` - LGPD-compliant account deletion

> **Atenção (29/09/2026):** `POST /api/auth/logout` e `DELETE /api/auth/account` são chamados pelo frontend, mas **não existem** no NestJS. Hoje o logout só limpa o navegador (o token continua válido no servidor até expirar), e "Excluir conta" falha. Ver [Privacidade](docs/privacidade.md).
  - `GET /api/me` - Get current user profile
  - `GET /api/health` - Health check

#### Configuration
Copy `.env.example` to `.env` and fill in:
- `DATABASE_URL` - CockroachDB connection string
- `GOOGLE_CLIENT_ID` - Google OAuth client ID
- `GOOGLE_CLIENT_SECRET` - Google OAuth client secret
- `API_PUBLIC_URL` - Public URL for OAuth redirects
- `FRONTEND_ORIGIN` - Frontend origin for CORS
- `API_PORT` - Server port (default: 3000)
- `NODE_ENV` - Environment (development/production)

#### Implementation
The backend mirrors the structure of the Minha-Agenda server, using:
- NestJS 11 with `@nestjs/common`, `@nestjs/config`, `@nestjs/terminus`
- `pg` driver for CockroachDB (PostgreSQL wire protocol compatible)
- `google-auth-library` for JWT verification
- Bearer tokens format: `ma_` + 32 random bytes base64url
- Only collects `sub`, `email`, `name` from Google (data minimization)
- Handshake TTL: 10 minutes; Session TTL: 30 days

#### Docker
```bash
cd server
docker-compose up --build
```

#### Tests
Unit tests for auth flows are planned under `src/auth/auth.controller.spec.ts`.

## Ver também

- [docs/README.md](docs/README.md): índice de toda a documentação.
- [CONTRIBUTING.md](CONTRIBUTING.md): ambiente de dev, `make`, branches, PR e CI.
- [docs/produto/visao.md](docs/produto/visao.md), [docs/produto/glossario.md](docs/produto/glossario.md), [docs/produto/regras.md](docs/produto/regras.md), [docs/produto/historias.md](docs/produto/historias.md)
- [docs/arquitetura.md](docs/arquitetura.md), [docs/dados.md](docs/dados.md), [docs/roadmap.md](docs/roadmap.md), [docs/operacao.md](docs/operacao.md)
