# App antigo (descontinuado)

O app antigo, o Angular em `src/` e o NestJS em `server/`, está descontinuado desde 29/09/2026 e não recebe mais mudanças. O MeuRPG novo (`backend/` e `web/`) foi construído do zero e cobre sozinho todas as histórias do MVP; não há migração gradual (ver [Roadmap](roadmap.md)).

Esta página guarda, só para consulta, o que o README dizia sobre o app antigo, até o código sair do repositório.

## O que acontece com cada parte

| Parte | Onde rodava | O que acontece |
| --- | --- | --- |
| `src/` (Angular) | GitHub Pages | Fica no repositório só como referência, até sair num PR à parte, depois do `server/`. |
| `server/` (NestJS) | Render | **Será removido do repositório**, decidido pelo Samuel em 29/09/2026: o backend novo em Go cobre sozinho todas as histórias do MVP, então não faz sentido manter os dois. Esta página já registra o que ele fazia, então a remoção não perde o contexto. |
| O banco do app antigo | CockroachDB | Só os personagens são importados, uma vez, para o banco novo; depois o banco é descomissionado (ver [Modelo de dados](dados.md) e [Privacidade](privacidade.md#o-banco-do-app-antigo)). |

As duas remoções e a importação são tarefas de limpeza da Etapa 8, sem história própria (ver [Roadmap](roadmap.md)).

Outros arquivos do app antigo, também só para consulta:

- `package.json`, `angular.json` e os `tsconfig*.json` da raiz: o projeto do Angular antigo. O Dependabot não atualiza esses pacotes (ver [CONTRIBUTING](../CONTRIBUTING.md#o-que-o-ci-confere)).
- `learnings.md`, na raiz: anotações da construção do Angular antigo.
- [code-quality.md](code-quality.md): o relatório de qualidade do Angular antigo.

## Frontend (Angular)

Gerado com o [Angular CLI](https://github.com/angular/angular-cli) 21.2.19.

```bash
npm install
npm start
```

## Backend (NestJS)

Um backend NestJS 11 simples, em `server/`, com login Google OAuth 2.0 com PKCE (S256), sessões com bearer token (hash SHA-256 no CockroachDB) e exclusão de conta conforme a LGPD.

### Features

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
  - `GET /api/me` - Get current user profile
  - `GET /api/health` - Health check

> **Atenção (29/09/2026):** `POST /api/auth/logout` e `DELETE /api/auth/account` são chamados pelo frontend, mas **não existem** no NestJS. Hoje o logout só limpa o navegador (o token continua válido no servidor até expirar), e "Excluir conta" falha. Ver [Privacidade](privacidade.md).

### Configuration

Copy `.env.example` to `.env` and fill in:

- `DATABASE_URL` - CockroachDB connection string
- `GOOGLE_CLIENT_ID` - Google OAuth client ID
- `GOOGLE_CLIENT_SECRET` - Google OAuth client secret
- `API_PUBLIC_URL` - Public URL for OAuth redirects
- `FRONTEND_ORIGIN` - Frontend origin for CORS
- `API_PORT` - Server port (default: 3000)
- `NODE_ENV` - Environment (development/production)

### Implementation

The backend mirrors the structure of the Minha-Agenda server, using:

- NestJS 11 with `@nestjs/common`, `@nestjs/config`, `@nestjs/terminus`
- `pg` driver for CockroachDB (PostgreSQL wire protocol compatible)
- `google-auth-library` for JWT verification
- Bearer tokens format: `ma_` + 32 random bytes base64url
- Only collects `sub`, `email`, `name` from Google (data minimization)
- Handshake TTL: 10 minutes; Session TTL: 30 days

### Docker

```bash
cd server
docker-compose up --build
```

### Tests

Os testes do login (`src/auth/auth.controller.spec.ts`) ficaram planejados e nunca foram escritos. Existem só os dos personagens: `src/characters/characters.controller.spec.ts` e `src/db/characters.repository.spec.ts`.

## Ver também

- [README](../README.md): o MeuRPG novo.
- [Arquitetura](arquitetura.md): como o sistema novo é montado.
