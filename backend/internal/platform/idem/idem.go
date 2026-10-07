// Package idem is the idempotency key of the create RPCs: how a key is checked, how it
// is scoped, and how the request it came with is hashed. The rest is each module's own
// column and unique index (map_points.create_key was the first), so this package holds
// only what every create does the same way.
//
// A key makes a create safe to retry. The app makes one per action and sends it again
// when the answer is lost: a second call with the same key and the same request returns
// what the first one made and creates nothing, and the same key with another request is
// refused. The key is optional on the RPCs that existed before it did: with none, the
// call is not deduplicated, as it never was.
package idem

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// MaxKeyLength is the longest key, in characters.
const MaxKeyLength = 64

// keyField is the name of the request field that holds the key.
const keyField protoreflect.Name = "idempotency_key"

// Clean checks a key: 1 to 64 characters of valid UTF-8, no control characters. An empty
// key is not an error: it means the caller asked for no deduplication, and Clean returns
// "" for it.
func Clean(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", nil
	}
	if utf8.RuneCountInString(key) > MaxKeyLength || !utf8.ValidString(key) || strings.IndexFunc(key, unicode.IsControl) >= 0 {
		return "", connect.NewError(connect.CodeInvalidArgument, errors.New("idempotency_key must be 1 to 64 characters"))
	}
	return key, nil
}

// Scope is the key as it is stored: the owner of the key (a campaign's ID, or a campaign
// and a user) and a colon in front, so one key is unique in its scope and two scopes can
// reuse the same string. It returns nil for no key, which the columns store as NULL.
func Scope(scope, key string) *string {
	if key == "" {
		return nil
	}
	scoped := scope + ":" + key
	return &scoped
}

// Hash is the hash of a whole request, minus its key: what is kept next to the key, so a
// retry is told from another request that reused the key. The message is cloned, so the
// caller's is never touched; the encoding is deterministic, so the same request always
// hashes the same.
func Hash(msg proto.Message) *string {
	clone := proto.Clone(msg)
	if fd := clone.ProtoReflect().Descriptor().Fields().ByName(keyField); fd != nil {
		clone.ProtoReflect().Clear(fd)
	}
	b, err := proto.MarshalOptions{Deterministic: true}.Marshal(clone)
	if err != nil {
		// A request that was decoded from the wire encodes again; this cannot happen.
		panic("idem: encode a request: " + err.Error())
	}
	sum := sha256.Sum256(append([]byte(string(clone.ProtoReflect().Descriptor().FullName())+"\x00"), b...))
	hash := hex.EncodeToString(sum[:])
	return &hash
}

// ErrReused is the answer to a key used before for another request.
func ErrReused() error {
	return connect.NewError(connect.CodeInvalidArgument, errors.New("idempotency_key was used for another change"))
}

// SameRequest reports whether the hash kept with a stored row is the one of this request:
// nil when it is, ErrReused when it is not (or when the row was made without one).
func SameRequest(stored *string, hash *string) error {
	if stored == nil || hash == nil || *stored != *hash {
		return ErrReused()
	}
	return nil
}

// Create makes the row of a create, or finds the one an earlier call with the same key made.
// It is the whole dedupe of a create, to be called inside the create's transaction (db.InTx),
// where get and insert both go through the transaction (WithTx):
//
//   - with no key (scoped is nil) it only calls insert;
//   - with a key it first calls get, which returns the row of the key or pgx.ErrNoRows, and a
//     row that is there is the answer when its hash is the request's (replayed is true) and
//     ErrReused when it is not;
//   - otherwise it calls insert, which must return pgx.ErrNoRows when the key's unique index
//     already holds a row (INSERT ... ON CONFLICT DO NOTHING RETURNING *): another call with
//     the same key won the race, and its row is read and checked the same way.
//
// hashOf reads the request hash kept in a row.
func Create[T any](
	ctx context.Context,
	scoped, hash *string,
	get func(ctx context.Context, key *string) (T, error),
	hashOf func(T) *string,
	insert func() (T, error),
) (row T, replayed bool, err error) {
	if scoped == nil {
		row, err = insert()
		return row, false, err
	}
	same := func(prior T) (T, bool, error) {
		if err := SameRequest(hashOf(prior), hash); err != nil {
			var zero T
			return zero, false, err
		}
		return prior, true, nil
	}
	prior, err := get(ctx, scoped)
	switch {
	case err == nil:
		return same(prior)
	case !errors.Is(err, pgx.ErrNoRows):
		return row, false, err
	}
	row, err = insert()
	if errors.Is(err, pgx.ErrNoRows) {
		// Another call with the same key won the race: its row is ours.
		if prior, err = get(ctx, scoped); err != nil {
			return row, false, err
		}
		return same(prior)
	}
	return row, false, err
}
