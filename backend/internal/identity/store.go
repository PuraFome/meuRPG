package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
)

// ErrNotFound is returned by a Store when the row does not exist or has
// expired.
var ErrNotFound = errors.New("identity: not found")

// Store persists what the identity module needs. PostgresStore is the real
// one; tests use an in-memory fake.
type Store interface {
	// SaveLoginState records a sign-in that has just started.
	SaveLoginState(ctx context.Context, state LoginState) error
	// TakeLoginState deletes and returns the login state with this hash, so
	// it can be used only once. It returns ErrNotFound when there is no such
	// state or when it expired before now.
	TakeLoginState(ctx context.Context, stateHash []byte, now time.Time) (LoginState, error)

	// UpsertUser returns the account linked to the identity, creating both
	// on the first sign-in, and refreshes the stored e-mail.
	UpsertUser(ctx context.Context, identity ExternalIdentity) (userID string, err error)

	// CreateSession stores a new session.
	CreateSession(ctx context.Context, session NewSession) (Session, error)
	// LookupSession returns the session with this token hash, or
	// ErrNotFound if there is none or it expired before now.
	LookupSession(ctx context.Context, tokenHash []byte, now time.Time) (Session, error)
	// RevokeSession deletes one session. Deleting a missing one is not an
	// error.
	RevokeSession(ctx context.Context, sessionID string) error
	// RevokeUserSessions deletes every session of a user: "sign out
	// everywhere", or a suspected account takeover.
	RevokeUserSessions(ctx context.Context, userID string) error
}

// LoginState is a sign-in in progress (table oidc_login_states).
type LoginState struct {
	StateHash    []byte
	CodeVerifier string
	Nonce        string
	ReturnTo     string
	CreatedAt    time.Time
	ExpiresAt    time.Time
}

// ExternalIdentity is a verified identity at an OpenID Connect provider.
type ExternalIdentity struct {
	Issuer  string
	Subject string
	// Email is a verified e-mail, or empty. An empty value clears the one
	// stored before, so the column always mirrors the latest sign-in.
	Email string
}

// NewSession is a session about to be created.
type NewSession struct {
	TokenHash []byte
	UserID    string
	CreatedAt time.Time
	ExpiresAt time.Time
	// AuthTime is the provider's auth_time; zero when unknown.
	AuthTime time.Time
}

// PostgresStore is the Store backed by CockroachDB.
//
// Statements that stand alone run without an explicit transaction:
// CockroachDB retries a single-statement (implicit) transaction by itself
// when it hits a conflict. Only UpsertUser, which needs several statements
// to agree, goes through db.InTx.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore returns a Store that uses pool.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

var _ Store = (*PostgresStore)(nil)

// SaveLoginState implements Store.
func (s *PostgresStore) SaveLoginState(ctx context.Context, st LoginState) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO oidc_login_states
			(state_hash, code_verifier, nonce, return_to, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		st.StateHash, st.CodeVerifier, st.Nonce, st.ReturnTo, st.CreatedAt, st.ExpiresAt)
	if err != nil {
		return fmt.Errorf("insert login state: %w", err)
	}
	return nil
}

// TakeLoginState implements Store. DELETE ... RETURNING reads and removes
// the row in one statement, so two callbacks racing with the same state
// cannot both get it.
func (s *PostgresStore) TakeLoginState(ctx context.Context, stateHash []byte, now time.Time) (LoginState, error) {
	st := LoginState{StateHash: stateHash}
	err := s.pool.QueryRow(ctx, `
		DELETE FROM oidc_login_states
		WHERE state_hash = $1
		RETURNING code_verifier, nonce, return_to, created_at, expires_at`,
		stateHash,
	).Scan(&st.CodeVerifier, &st.Nonce, &st.ReturnTo, &st.CreatedAt, &st.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return LoginState{}, ErrNotFound
	}
	if err != nil {
		return LoginState{}, fmt.Errorf("take login state: %w", err)
	}
	// Expired rows linger until the TTL job runs; they are deleted above
	// all the same.
	if !now.Before(st.ExpiresAt) {
		return LoginState{}, ErrNotFound
	}
	return st, nil
}

// UpsertUser implements Store.
//
// Two first sign-ins of the same person at the same moment both see "no
// identity yet" and both try to create one. CockroachDB aborts one of them
// with a serialization error (40001); InTx runs it again, and the second
// attempt finds the other's row, with no stray users row left behind
// (TestPostgresStoreUpsertUserRace).
func (s *PostgresStore) UpsertUser(ctx context.Context, id ExternalIdentity) (string, error) {
	var email *string
	if id.Email != "" {
		email = &id.Email
	}

	var userID string
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		// Returning user: refresh the e-mail and we are done. This runs on
		// every sign-in, so a changed or unverified e-mail is updated or
		// cleared right away.
		err := tx.QueryRow(ctx, `
			UPDATE user_identities SET email = $3
			WHERE issuer = $1 AND subject = $2
			RETURNING user_id`,
			id.Issuer, id.Subject, email,
		).Scan(&userID)
		if err == nil {
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("update identity: %w", err)
		}

		// First sign-in: create the account and link the identity to it.
		if err := tx.QueryRow(ctx, `INSERT INTO users DEFAULT VALUES RETURNING id`).Scan(&userID); err != nil {
			return fmt.Errorf("insert user: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_identities (issuer, subject, user_id, email)
			VALUES ($1, $2, $3, $4)`,
			id.Issuer, id.Subject, userID, email,
		); err != nil {
			return fmt.Errorf("insert identity: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return userID, nil
}

// CreateSession implements Store.
func (s *PostgresStore) CreateSession(ctx context.Context, ns NewSession) (Session, error) {
	var authTime *time.Time
	if !ns.AuthTime.IsZero() {
		authTime = &ns.AuthTime
	}
	session := Session{UserID: ns.UserID, CreatedAt: ns.CreatedAt, ExpiresAt: ns.ExpiresAt}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO auth_sessions (token_hash, user_id, created_at, expires_at, auth_time)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`,
		ns.TokenHash, ns.UserID, ns.CreatedAt, ns.ExpiresAt, authTime,
	).Scan(&session.ID)
	if err != nil {
		return Session{}, fmt.Errorf("insert session: %w", err)
	}
	return session, nil
}

// LookupSession implements Store.
func (s *PostgresStore) LookupSession(ctx context.Context, tokenHash []byte, now time.Time) (Session, error) {
	var session Session
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, created_at, expires_at
		FROM auth_sessions
		WHERE token_hash = $1 AND expires_at > $2`,
		tokenHash, now,
	).Scan(&session.ID, &session.UserID, &session.CreatedAt, &session.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("look up session: %w", err)
	}
	return session, nil
}

// RevokeSession implements Store.
func (s *PostgresStore) RevokeSession(ctx context.Context, sessionID string) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM auth_sessions WHERE id = $1`, sessionID); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// RevokeUserSessions implements Store.
func (s *PostgresStore) RevokeUserSessions(ctx context.Context, userID string) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM auth_sessions WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("delete user sessions: %w", err)
	}
	return nil
}
