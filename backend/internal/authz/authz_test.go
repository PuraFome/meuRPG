package authz

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	campaignA = "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e001"
	campaignB = "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e002"
	missing   = "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e0ff"
	master    = "user-master"
	player    = "user-player"
	pending   = "user-pending" // accepted an invite that requires approval (RN-15)
	outsider  = "user-outsider"
)

// fakeMember is one row of fakeSource.
type fakeMember struct {
	role   Role
	status Status
}

// fakeSource is a MembershipSource backed by a map, which counts lookups.
type fakeSource struct {
	mu      sync.Mutex
	members map[[2]string]fakeMember // {campaignID, userID}
	err     error                    // returned instead, when set
	lookups int
}

func newFakeSource() *fakeSource {
	return &fakeSource{members: map[[2]string]fakeMember{
		{campaignA, master}:  {RoleMaster, StatusActive},
		{campaignA, player}:  {RolePlayer, StatusActive},
		{campaignA, pending}: {RolePlayer, StatusPending},
		{campaignB, player}:  {RoleMaster, StatusActive}, // the same user can be master elsewhere (RN-05)
	}}
}

func (f *fakeSource) CampaignMembership(_ context.Context, campaignID, userID string) (Role, Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lookups++
	if f.err != nil {
		return "", "", f.err
	}
	m, ok := f.members[[2]string{campaignID, userID}]
	if !ok {
		return "", "", ErrNotMember
	}
	return m.role, m.status, nil
}

func (f *fakeSource) lookupCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lookups
}

// fakeCaller is a Caller that always answers with the same user, or
// `unauthenticated` when userID is "". It stands in for the identity module,
// whose session interceptor is the only thing that sets the caller in
// production.
type fakeCaller struct{ userID string }

func (f fakeCaller) UserID(context.Context) (string, error) {
	if f.userID == "" {
		return "", connect.NewError(connect.CodeUnauthenticated, errors.New("sign in to continue"))
	}
	return f.userID, nil
}

// requestContext is the context a handler sees for a request by userID
// ("" for signed out), with the authz interceptor installed.
func requestContext(t *testing.T, source MembershipSource, userID string) context.Context {
	t.Helper()
	var got context.Context
	handler := Interceptor(fakeCaller{userID}, source, slog.New(slog.DiscardHandler)).WrapUnary(
		func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
			got = ctx
			return nil, nil
		})
	if _, err := handler(t.Context(), connect.NewRequest(&emptypb.Empty{})); err != nil {
		t.Fatalf("interceptor error = %v", err)
	}
	return got
}

// procedureContext is the context a handler of procedure sees for a
// request by userID. connect.NewRequest cannot carry a handler's Spec, so
// this builds the memo the way the interceptor does for a served request.
func procedureContext(t *testing.T, source MembershipSource, userID, procedure string) context.Context {
	t.Helper()
	i := Interceptor(fakeCaller{userID}, source, slog.New(slog.DiscardHandler)).(*interceptor)
	return i.withMemo(t.Context(), procedure)
}

func TestRequireCampaignMemberAndRole(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		user       string
		campaign   string
		wantMember connect.Code // 0 means allowed
		wantMaster connect.Code
	}{
		{"master", master, campaignA, 0, 0},
		{"player", player, campaignA, 0, connect.CodePermissionDenied},
		{"a player in one campaign is the master of another (RN-05)", player, campaignB, 0, 0},
		// A pending member is not a member yet (RN-15): the same answer as
		// for a stranger.
		{"pending member", pending, campaignA, connect.CodeNotFound, connect.CodeNotFound},
		{"non-member", outsider, campaignA, connect.CodeNotFound, connect.CodeNotFound},
		{"campaign that does not exist", master, missing, connect.CodeNotFound, connect.CodeNotFound},
		{"ID that is not a UUID", master, "mirathel", connect.CodeNotFound, connect.CodeNotFound},
		{"anonymous", "", campaignA, connect.CodeUnauthenticated, connect.CodeUnauthenticated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := requestContext(t, newFakeSource(), tt.user)

			userID, err := RequireSignedIn(ctx)
			if tt.user == "" {
				checkCode(t, "RequireSignedIn", err, connect.CodeUnauthenticated)
			} else if err != nil || userID != tt.user {
				t.Errorf("RequireSignedIn() = %q, %v; want %q", userID, err, tt.user)
			}

			m, err := RequireCampaignMember(ctx, tt.campaign)
			checkCode(t, "RequireCampaignMember", err, tt.wantMember)
			if err == nil && (m.CampaignID != tt.campaign || m.UserID != tt.user) {
				t.Errorf("RequireCampaignMember() = %+v, want campaign %s, user %s", m, tt.campaign, tt.user)
			}

			_, err = RequireCampaignRole(ctx, tt.campaign, RoleMaster)
			checkCode(t, "RequireCampaignRole(master)", err, tt.wantMaster)
		})
	}
}

// TestNotFoundDoesNotRevealExistence: a non-member gets the very same
// error for a real campaign as for a made-up one.
func TestNotFoundDoesNotRevealExistence(t *testing.T) {
	t.Parallel()
	source := newFakeSource()

	_, existing := RequireCampaignMember(requestContext(t, source, outsider), campaignA)
	_, fake := RequireCampaignMember(requestContext(t, source, outsider), missing)
	if existing == nil || fake == nil || existing.Error() != fake.Error() {
		t.Errorf("errors differ: %v vs %v", existing, fake)
	}
	for _, err := range []error{existing, fake} {
		if ce, ok := errors.AsType[*connect.Error](err); !ok || ce.Meta().Get("Cache-Control") != "no-store" {
			t.Errorf("%v lacks Cache-Control: no-store", err)
		}
	}
}

func TestMembershipIsLoadedOncePerRequest(t *testing.T) {
	t.Parallel()
	source := newFakeSource()
	ctx := requestContext(t, source, player)

	for range 3 {
		if _, err := RequireCampaignMember(ctx, campaignA); err != nil {
			t.Fatalf("RequireCampaignMember() error = %v", err)
		}
		if _, err := RequireCampaignRole(ctx, campaignA, RoleMaster); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Fatalf("RequireCampaignRole() error = %v, want permission_denied", err)
		}
		// Another spelling of the same ID is the same campaign.
		if _, err := RequireCampaignMember(ctx, strings.ToUpper(campaignA)); err != nil {
			t.Fatalf("RequireCampaignMember(upper case) error = %v", err)
		}
		// "Not a member" is remembered too.
		if _, err := RequireCampaignMember(ctx, missing); connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("RequireCampaignMember(missing) error = %v, want not_found", err)
		}
	}
	if got := source.lookupCount(); got != 2 {
		t.Errorf("lookups = %d, want 2 (one per campaign)", got)
	}

	// The next request starts with an empty memo: a membership removed in
	// between is noticed at once.
	if _, err := RequireCampaignMember(requestContext(t, source, player), campaignA); err != nil {
		t.Fatalf("RequireCampaignMember() error = %v", err)
	}
	if got := source.lookupCount(); got != 3 {
		t.Errorf("lookups after a new request = %d, want 3", got)
	}
}

func TestSourceFailureIsUnavailableAndNotRemembered(t *testing.T) {
	t.Parallel()
	source := newFakeSource()
	source.err = errors.New("database is down")
	ctx := requestContext(t, source, master)

	if _, err := RequireCampaignMember(ctx, campaignA); connect.CodeOf(err) != connect.CodeUnavailable {
		t.Fatalf("RequireCampaignMember() error = %v, want unavailable", err)
	} else if strings.Contains(err.Error(), "database is down") {
		t.Errorf("error %q leaks the internal cause to the client", err)
	}

	source.mu.Lock()
	source.err = nil
	source.mu.Unlock()
	if _, err := RequireCampaignMember(ctx, campaignA); err != nil {
		t.Errorf("RequireCampaignMember() after recovery error = %v, want the failure not to be remembered", err)
	}
}

func TestRequireFailsClosedWithoutTheInterceptor(t *testing.T) {
	t.Parallel()
	// Without the interceptor there is no caller at all, so even a request
	// that would have a session gets nowhere.
	ctx := t.Context()
	if _, err := RequireSignedIn(ctx); connect.CodeOf(err) != connect.CodeInternal {
		t.Errorf("RequireSignedIn() without Interceptor error = %v, want internal", err)
	}
	if _, err := RequireCampaignMember(ctx, campaignA); connect.CodeOf(err) != connect.CodeInternal {
		t.Errorf("RequireCampaignMember() without Interceptor error = %v, want internal", err)
	}
	if _, err := RequireCampaignRole(ctx, campaignA, RoleMaster); connect.CodeOf(err) != connect.CodeInternal {
		t.Errorf("RequireCampaignRole() without Interceptor error = %v, want internal", err)
	}
}

func checkCode(t *testing.T, call string, err error, want connect.Code) {
	t.Helper()
	switch {
	case want == 0 && err != nil:
		t.Errorf("%s error = %v, want allowed", call, err)
	case want != 0 && connect.CodeOf(err) != want:
		t.Errorf("%s error = %v, want %v", call, err, want)
	}
}

// TestMiddlewareForPlainHTTPRoutes: behind Middleware, a plain HTTP handler
// (the image routes of package maps) makes the same checks as a Connect
// handler, with one memo per request. A pending member stays out: no route
// is in pendingMayCall.
func TestMiddlewareForPlainHTTPRoutes(t *testing.T) {
	t.Parallel()
	source := newFakeSource()
	check := func(userID string, handler func(ctx context.Context) error) error {
		t.Helper()
		var err error
		mux := http.NewServeMux()
		mux.Handle("POST /uploads/images", Middleware(fakeCaller{userID}, source, slog.New(slog.DiscardHandler))(
			http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { err = handler(r.Context()) })))
		mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/uploads/images", nil))
		return err
	}
	role := func(ctx context.Context) error {
		_, err := RequireCampaignRole(ctx, campaignA, RoleMaster)
		return err
	}
	orPending := func(ctx context.Context) error {
		_, err := RequireCampaignMemberOrPending(ctx, campaignA)
		return err
	}

	checkCode(t, "master", check(master, role), 0)
	checkCode(t, "player", check(player, role), connect.CodePermissionDenied)
	checkCode(t, "outsider", check(outsider, role), connect.CodeNotFound)
	checkCode(t, "signed out", check("", role), connect.CodeUnauthenticated)
	checkCode(t, "pending", check(pending, role), connect.CodeNotFound)
	checkCode(t, "pending, asking for the allowance", check(pending, orPending), connect.CodeNotFound)

	// One lookup per request, even when the handler checks twice.
	before := source.lookupCount()
	_ = check(master, func(ctx context.Context) error {
		if err := role(ctx); err != nil {
			return err
		}
		return role(ctx)
	})
	if got := source.lookupCount() - before; got != 1 {
		t.Errorf("lookups for two checks in one request = %d, want 1", got)
	}
}
