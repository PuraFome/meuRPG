# MeuRpg

This project was generated using [Angular CLI](https://github.com/angular-cli) version 21.2.19.

## Backend Server

A simple NestJS 11 backend has been created under `server/` implementing Google OAuth 2.0 login with PKCE (S256), bearer token sessions (SHA256-hashed in CockroachDB) and LGPD-compliant account deletion.

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
Unit tests for auth flows are planned under `src/auth/auth.controller.spec.ts`.