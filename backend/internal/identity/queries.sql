-- name: InsertLoginState :exec
INSERT INTO oidc_login_states
    (state_hash, code_verifier, nonce, return_to, created_at, expires_at, intent_kind, intent_data)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: TakeLoginState :one
-- Reads and deletes in one statement, so two callbacks racing with the same
-- state cannot both get it.
DELETE FROM oidc_login_states
WHERE state_hash = $1
RETURNING code_verifier, nonce, return_to, created_at, expires_at, intent_kind, intent_data;

-- name: UpdateIdentityEmail :one
UPDATE user_identities SET email = $3
WHERE issuer = $1 AND subject = $2
RETURNING user_id;

-- name: InsertUser :one
INSERT INTO users DEFAULT VALUES RETURNING id;

-- name: InsertIdentity :exec
INSERT INTO user_identities (issuer, subject, user_id, email)
VALUES ($1, $2, $3, $4);

-- name: InsertSession :one
INSERT INTO auth_sessions (token_hash, user_id, created_at, expires_at, auth_time)
VALUES ($1, $2, $3, $4, $5)
RETURNING id;

-- name: LookupSession :one
SELECT id, user_id, created_at, expires_at
FROM auth_sessions
WHERE token_hash = $1 AND expires_at > sqlc.arg(now);

-- name: SessionIsActive :one
-- A long-lived stream checks its session again by ID (RecheckSession): it
-- is still there (no sign-out, no revocation) and has not expired.
SELECT EXISTS (
    SELECT 1 FROM auth_sessions WHERE id = $1 AND expires_at > sqlc.arg(now)
);

-- name: DeleteSession :exec
DELETE FROM auth_sessions WHERE id = $1;

-- name: DeleteUserSessions :exec
DELETE FROM auth_sessions WHERE user_id = $1;

-- name: GetDisplayName :one
SELECT display_name FROM users WHERE id = $1;

-- name: SetDisplayName :execrows
UPDATE users SET display_name = $2 WHERE id = $1;

-- name: ListVerifiedEmails :many
-- The e-mails the provider vouched for (an unverified one is never stored).
-- campaigns reads them for the allow-list of who may create campaigns (RN-30).
SELECT email::TEXT AS email
FROM user_identities
WHERE user_id = $1 AND email IS NOT NULL;

-- name: ListDisplayNames :many
-- Users without a display name are left out.
SELECT id, display_name::TEXT AS display_name
FROM users
WHERE id = ANY(sqlc.arg(ids)::UUID[]) AND display_name IS NOT NULL;
