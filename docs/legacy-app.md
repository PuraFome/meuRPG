# Legacy app (discontinued)

The old app, the Angular app in `src/` and the NestJS server in `server/`, is discontinued and gets no more changes. The current MeuRPG (`backend/` and `web/`) was built from scratch and covers every MVP story on its own; there is no gradual migration (see [Roadmap](roadmap.md)).

This page keeps, for reference only, what the README used to say about the old app, until the code leaves the repository.

## What happens to each part

| Part | Where it ran | What happens |
| --- | --- | --- |
| `src/` (Angular) | GitHub Pages | Stays in the repository as a reference only, until it is removed in a separate PR, after `server/`. |
| `server/` (NestJS) | Render | **Will be removed from the repository.** The new Go backend covers every MVP story on its own, so keeping both makes no sense. This page records what the server did, so removing it loses no context. |
| The old app's database | CockroachDB | Only the characters are imported, once, into the new database; then the old database is decommissioned (see [Data model](data.md) and [Privacy](privacy.md#the-legacy-app-database)). |

The two removals and the import are cleanup tasks of Etapa 11, with no story of their own (see [Roadmap](roadmap.md)).

Other files of the old app, also kept for reference:

- `package.json`, `angular.json` and the `tsconfig*.json` files at the root: the old Angular project. Dependabot does not update these packages (see [CONTRIBUTING](../CONTRIBUTING.md#what-ci-checks)).
- `learnings.md`, at the root: notes from building the old Angular app.
- [legacy-code-quality.md](archive/legacy-code-quality.md): the quality report of the old Angular app (archived).

## Frontend (Angular)

Generated with the [Angular CLI](https://github.com/angular/angular-cli) 21.2.19.

```bash
npm install
npm start
```

## Backend (NestJS)

A simple NestJS 11 backend in `server/`, with Google OAuth 2.0 sign-in with PKCE (S256), bearer-token sessions (SHA-256 hash in CockroachDB) and account deletion under the LGPD.

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

> **Warning:** the frontend calls `POST /api/auth/logout` and `DELETE /api/auth/account`, but they **do not exist** in the NestJS server. Today sign-out only clears the browser (the token stays valid on the server until it expires), and "Excluir conta" fails. See [Privacy](privacy.md).

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

The sign-in tests (`src/auth/auth.controller.spec.ts`) were planned and never written. Only the characters have tests: `src/characters/characters.controller.spec.ts` and `src/db/characters.repository.spec.ts`.

## See also

- [README](../README.md): the current MeuRPG.
- [Architecture](architecture.md): how the current system is built.
