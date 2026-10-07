package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/PuraFome/meuRPG/backend/gen/meurpg/system/v1/systemv1connect"
	"github.com/PuraFome/meuRPG/backend/internal/identity"
	"github.com/PuraFome/meuRPG/backend/internal/platform/config"
	"github.com/PuraFome/meuRPG/backend/internal/platform/ratelimit"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/system"
)

// publicProcedures are the procedures that answer a caller with no session,
// on purpose. Anything not listed here must answer "unauthenticated": a new
// RPC that forgets its gate fails TestEveryProcedureRefusesAnonymousCalls,
// and adding it here is a decision to justify in review.
var publicProcedures = map[string]string{
	// The version and commit, which the login page shows and the probes read.
	"/meurpg.system.v1.SystemService/GetServerInfo": "server info is public",
}

// buildHandlerSet mounts every service the way run() does: the same wiring
// (wireModules), the same Connect options (connectOptions) and the same mounts
// (mountModules), over a pool that never connects (pgxpool connects lazily).
// A call with no session must be turned away before any query, so a gate that
// is missing shows up as an error that is not "unauthenticated".
func buildHandlerSet(t *testing.T) http.Handler {
	t.Helper()
	ctx := t.Context()
	logger := slog.New(slog.DiscardHandler)

	pool, err := pgxpool.New(ctx, "postgresql://root@127.0.0.1:1/none?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	content, err := rules.LoadSRD()
	must(t, err)

	// No image store and no image generator: like a server without BLOB_DIR
	// or GEMINI_API_KEY, whose image RPCs still refuse an anonymous caller.
	m, err := wireModules(logger, pool, content, nil, nil, wireOptions{MonthlyImages: 20})
	must(t, err)
	t.Cleanup(m.play.Close)

	// The provider does not exist: discovery fails fast (connection refused)
	// and is retried on the next sign-in, which this test never makes.
	identityService, err := identity.New(ctx, identity.Config{
		OIDC: config.OIDC{
			IssuerURL: "http://localhost:1", ClientID: "test", ClientSecret: "test",
			RedirectURL: "http://localhost:8080/auth/callback",
		},
		Store: m.users, Logger: logger,
	})
	must(t, err)

	mux := http.NewServeMux()
	opts := connectOptions(logger)
	mux.Handle(systemv1connect.NewSystemServiceHandler(system.NewService(version, commit), opts...))
	mountModules(mux.Handle, m, identityService, opts, ratelimit.NewPolicy(1).RPC, logger)
	return mux
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// TestEveryProcedureRefusesAnonymousCalls calls every RPC of every service
// linked into the binary, enumerated from the generated descriptors, with no
// session cookie. Each must answer "unauthenticated", except the ones in
// publicProcedures. A new RPC is covered the day it is generated.
func TestEveryProcedureRefusesAnonymousCalls(t *testing.T) {
	handler := buildHandlerSet(t)

	seen := map[string]bool{}
	protoregistry.GlobalFiles.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		if !strings.HasPrefix(string(file.Package()), "meurpg.") {
			return true
		}
		for i := range file.Services().Len() {
			service := file.Services().Get(i)
			for j := range service.Methods().Len() {
				method := service.Methods().Get(j)
				procedure := "/" + string(service.FullName()) + "/" + string(method.Name())
				seen[procedure] = true
				t.Run(procedure, func(t *testing.T) {
					got := anonymousCode(t, handler, procedure, method.IsStreamingServer())
					if reason, public := publicProcedures[procedure]; public {
						if got == "unauthenticated" {
							t.Errorf("%s is listed as public (%s) but refuses anonymous calls: drop it from publicProcedures", procedure, reason)
						}
						return
					}
					if got != "unauthenticated" {
						t.Errorf("an anonymous call to %s answered %q, want unauthenticated; "+
							"a new RPC needs its session gate (or an entry in publicProcedures, with the reason)", procedure, got)
					}
				})
			}
		}
		return true
	})
	for procedure := range publicProcedures {
		if !seen[procedure] {
			t.Errorf("publicProcedures lists %s, which no service has", procedure)
		}
	}
	if len(seen) < 100 {
		t.Errorf("only %d procedures found; the services are no longer all linked in", len(seen))
	}
}

// anonymousCode makes the call with an empty request and no cookie and
// returns the Connect error code, or "ok" when it succeeded.
func anonymousCode(t *testing.T, handler http.Handler, procedure string, serverStream bool) string {
	t.Helper()
	contentType, body := "application/json", []byte("{}")
	if serverStream {
		// A server stream is only Connect's streaming flavor: each message
		// is an envelope (flag byte, 4-byte length, payload).
		contentType = "application/connect+json"
		body = append([]byte{0, 0, 0, 0, 2}, "{}"...)
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, procedure, bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Connect-Protocol-Version", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	raw, _ := io.ReadAll(rec.Result().Body)
	if serverStream {
		// The error, if any, is in the last envelope (the end-of-stream message).
		var last []byte
		for len(raw) >= 5 {
			n := int(binary.BigEndian.Uint32(raw[1:5]))
			if len(raw) < 5+n {
				break
			}
			last, raw = raw[5:5+n], raw[5+n:]
		}
		raw = last
		var end struct {
			Error *struct{ Code string } `json:"error"`
		}
		if err := json.Unmarshal(raw, &end); err != nil {
			t.Fatalf("%s: unreadable end of stream %q: %v", procedure, raw, err)
		}
		if end.Error == nil {
			return "ok"
		}
		return end.Error.Code
	}
	if rec.Code == http.StatusOK {
		return "ok"
	}
	var e struct{ Code string }
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatalf("%s: status %d, unreadable body %q: %v", procedure, rec.Code, raw, err)
	}
	return e.Code
}
