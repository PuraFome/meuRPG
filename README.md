# MeuRPG

O MeuRPG é onde uma mesa de D&D 5e prepara e joga suas campanhas. O mestre prepara o mundo e conduz a sessão ao vivo; o jogador entra por convite, acompanha a ficha e age no RP e no combate com o que as regras permitem. Mais contexto em [docs/produto/visao.md](docs/produto/visao.md).

O projeto está em migração. Hoje o frontend é em Angular e o backend em NestJS, no Render. O backend novo é um monólito modular em Go, com API em Protobuf + Connect, rodando no Cloud Run em São Paulo — migrando módulo por módulo, com o app atual sempre no ar. Detalhes em [docs/arquitetura.md](docs/arquitetura.md) e o estado de cada etapa em [docs/roadmap.md](docs/roadmap.md).

- Documentação completa: [docs/README.md](docs/README.md)
- Como rodar, testar e abrir um PR: [CONTRIBUTING.md](CONTRIBUTING.md)

## Como começar rápido

Frontend (Angular), como hoje:

```bash
npm install
npm start
```

Backend atual (NestJS), com Docker:

```bash
cd server
docker-compose up --build
```

Backend novo (Go), com CockroachDB local no Docker:

```bash
make up      # sobe CockroachDB, aplica as migrations e sobe a API em localhost:8080
make test    # testes do backend
make down    # derruba tudo
```

Todos os comandos estão em `make help` e no [CONTRIBUTING.md](CONTRIBUTING.md).

## Stack atual (durante a migração)

### Frontend (Angular)

This project was generated using [Angular CLI](https://github.com/angular-cli) version 21.2.19.

### Backend atual (NestJS)

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
