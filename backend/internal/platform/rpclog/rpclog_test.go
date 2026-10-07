package rpclog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	progressionv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/progression/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/logging"
)

const campaign = "3f2b8c1e-5d4a-4e6b-9c7d-1a2b3c4d5e6f"

// rig serves one unary and one streaming procedure through the interceptor,
// the way cmd/api mounts it: inside a request that already has log fields.
type rig struct {
	logs   *bytes.Buffer
	server *httptest.Server
}

func newRig(t *testing.T, unary func(context.Context) error, stream func(context.Context, *connect.ServerStream[playv1.WatchGameSessionResponse]) error) *rig {
	t.Helper()
	logs := &bytes.Buffer{}
	logger := logging.New(logs, slog.LevelDebug)
	opts := connect.WithInterceptors(Interceptor(logger))

	mux := http.NewServeMux()
	mux.Handle("/test.Svc/Unary", connect.NewUnaryHandler("/test.Svc/Unary",
		func(ctx context.Context, _ *connect.Request[campaignsv1.GetCampaignRequest]) (*connect.Response[campaignsv1.GetCampaignRequest], error) {
			// What the session interceptor does inside the call.
			logging.SetUserID(ctx, "99999999-9999-9999-9999-999999999999")
			return connect.NewResponse(&campaignsv1.GetCampaignRequest{}), unary(ctx)
		}, opts))
	mux.Handle("/test.Svc/Stream", connect.NewServerStreamHandler("/test.Svc/Stream",
		func(ctx context.Context, _ *connect.Request[playv1.WatchGameSessionRequest], s *connect.ServerStream[playv1.WatchGameSessionResponse]) error {
			return stream(ctx, s)
		}, opts))

	// The HTTP layer that gives each request its log fields (httpserver does
	// this in production).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := logging.WithRequest(r.Context(), "0123456789abcdef0123456789abcdef", "")
		mux.ServeHTTP(w, r.WithContext(ctx))
	}))
	t.Cleanup(srv.Close)
	return &rig{logs: logs, server: srv}
}

func (r *rig) lines(t *testing.T) []map[string]any {
	t.Helper()
	var out []map[string]any
	for l := range bytes.SplitSeq(bytes.TrimSpace(r.logs.Bytes()), []byte("\n")) {
		if len(l) == 0 {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(l, &m); err != nil {
			t.Fatalf("bad line %q: %v", l, err)
		}
		out = append(out, m)
	}
	return out
}

func callUnary(t *testing.T, r *rig, campaignID string) error {
	t.Helper()
	c := connect.NewClient[campaignsv1.GetCampaignRequest, campaignsv1.GetCampaignRequest](r.server.Client(), r.server.URL+"/test.Svc/Unary")
	_, err := c.CallUnary(t.Context(), connect.NewRequest(&campaignsv1.GetCampaignRequest{CampaignId: campaignID}))
	return err
}

func TestUnaryLevelsFollowTheCode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err   error
		code  string
		level string
	}{
		{nil, "ok", "DEBUG"},
		{connect.NewError(connect.CodeNotFound, errors.New("no such thing")), "not_found", "DEBUG"},
		{connect.NewError(connect.CodeInvalidArgument, errors.New("bad")), "invalid_argument", "DEBUG"},
		{connect.NewError(connect.CodeFailedPrecondition, errors.New("no")), "failed_precondition", "DEBUG"},
		{connect.NewError(connect.CodeInternal, errors.New("boom")), "internal", "ERROR"},
		{connect.NewError(connect.CodeUnavailable, errors.New("db down")), "unavailable", "ERROR"},
		{connect.NewError(connect.CodeDeadlineExceeded, errors.New("slow")), "deadline_exceeded", "ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, func(context.Context) error { return tt.err }, nil)
			_ = callUnary(t, r, campaign)

			line := r.lines(t)[0]
			if line["message"] != "rpc" || line["code"] != tt.code || line["severity"] != tt.level {
				t.Errorf("line = %v, want code %s at %s", line, tt.code, tt.level)
			}
			if line["procedure"] != "/test.Svc/Unary" || line["stream"] != false {
				t.Errorf("procedure/stream = %v/%v", line["procedure"], line["stream"])
			}
			// The campaign came from the request; the user from inside the call.
			if line["campaign_id"] != campaign || line["user_id"] != "99999999-9999-9999-9999-999999999999" || line["request_id"] != "0123456789abcdef0123456789abcdef" {
				t.Errorf("context fields missing: %v", line)
			}
			if _, ok := line["duration_ms"].(float64); !ok {
				t.Errorf("duration_ms = %v", line["duration_ms"])
			}
			if tt.err == nil {
				if _, has := line["error"]; has {
					t.Errorf("an ok call has no error key: %v", line)
				}
			}
		})
	}
}

func TestCampaignFromTheRequestMustBeAUUID(t *testing.T) {
	t.Parallel()
	r := newRig(t, func(context.Context) error { return nil }, nil)
	_ = callUnary(t, r, "not a uuid\nwith text")
	if _, has := r.lines(t)[0]["campaign_id"]; has {
		t.Error("a client-chosen string reached the log as campaign_id")
	}
}

func TestReasonAndErrorText(t *testing.T) {
	t.Parallel()
	failure := connect.NewError(connect.CodeFailedPrecondition, errors.New("there is no award to undo"))
	detail, err := connect.NewErrorDetail(&progressionv1.XPBlocked{Reason: progressionv1.XPBlockedReason_XP_BLOCKED_REASON_NOTHING_TO_UNDO})
	if err != nil {
		t.Fatal(err)
	}
	failure.AddDetail(detail)
	r := newRig(t, func(context.Context) error { return failure }, nil)

	callErr := callUnary(t, r, campaign)
	line := r.lines(t)[0]
	if line["reason"] != "XP_BLOCKED_REASON_NOTHING_TO_UNDO" {
		t.Errorf("reason = %v", line["reason"])
	}
	if line["error"] != "there is no award to undo" {
		t.Errorf("error = %v", line["error"])
	}
	if !strings.Contains(callErr.Error(), "there is no award to undo") {
		t.Errorf("the client lost the message: %v", callErr)
	}
}

// The server's own error text (a driver message, a path) goes to the log and
// never to the client.
func TestInternalErrorTextIsLoggedNotSent(t *testing.T) {
	t.Parallel()
	const secret = "pq: connection to 10.0.0.7:26257 refused"
	r := newRig(t, func(context.Context) error { return errors.New(secret) }, nil)

	callErr := callUnary(t, r, campaign)

	line := r.lines(t)[0]
	if line["error"] != secret || line["code"] != "internal" || line["severity"] != "ERROR" {
		t.Errorf("line = %v", line)
	}
	if callErr == nil || strings.Contains(callErr.Error(), "10.0.0.7") || connect.CodeOf(callErr) != connect.CodeInternal {
		t.Errorf("the client got %v, want a bare internal error", callErr)
	}
}

func TestStreamLines(t *testing.T) {
	t.Parallel()
	r := newRig(t, nil, func(_ context.Context, s *connect.ServerStream[playv1.WatchGameSessionResponse]) error {
		for range 3 {
			if err := s.Send(&playv1.WatchGameSessionResponse{}); err != nil {
				return err
			}
		}
		return nil // the handler ends it while the client is still there
	})
	c := connect.NewClient[playv1.WatchGameSessionRequest, playv1.WatchGameSessionResponse](r.server.Client(), r.server.URL+"/test.Svc/Stream")
	stream, err := c.CallServerStream(t.Context(), connect.NewRequest(&playv1.WatchGameSessionRequest{CampaignId: campaign}))
	if err != nil {
		t.Fatal(err)
	}
	for stream.Receive() {
	}
	_ = stream.Close()

	lines := r.lines(t)
	if len(lines) != 3 {
		t.Fatalf("want opened, closed and rpc lines, got %v", lines)
	}
	opened, closed, rpc := lines[0], lines[1], lines[2]
	if opened["message"] != "stream opened" || opened["campaign_id"] != campaign || opened["procedure"] != "/test.Svc/Stream" {
		t.Errorf("opened = %v", opened)
	}
	if closed["message"] != "stream closed" || closed["messages"] != float64(3) || closed["close_reason"] != "server_ended" || closed["campaign_id"] != campaign {
		t.Errorf("closed = %v", closed)
	}
	if rpc["message"] != "rpc" || rpc["stream"] != true || rpc["code"] != "ok" {
		t.Errorf("rpc = %v", rpc)
	}
}

func TestStreamCloseReasons(t *testing.T) {
	t.Parallel()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	tests := []struct {
		name       string
		ctx        context.Context
		err        error
		sendFailed bool
		want       string
	}{
		{"client left", canceled, nil, false, "client_gone"},
		{"send failed", context.Background(), nil, true, "client_gone"},
		{"server ended it", context.Background(), nil, false, "server_ended"},
		{"membership removed", context.Background(), connect.NewError(connect.CodeNotFound, errors.New("x")), false, "access_lost"},
		{"session ended", context.Background(), connect.NewError(connect.CodeUnauthenticated, errors.New("x")), false, "access_lost"},
		{"fell behind", context.Background(), connect.NewError(connect.CodeUnavailable, errors.New("x")), false, "error"},
	}
	for _, tt := range tests {
		if got := closeReasonOf(tt.ctx, tt.err, tt.sendFailed); got != tt.want {
			t.Errorf("%s: %s, want %s", tt.name, got, tt.want)
		}
	}
}
