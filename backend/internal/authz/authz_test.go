package authz

import (
	"context"
	"errors"
	"log/slog"
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
	outsider  = "user-outsider"
)

// fakeSource is a MembershipSource backed by a map, which counts lookups.
type fakeSource struct {
	mu      sync.Mutex
	roles   map[[2]string]Role // {campaignID, userID}
	err     error              // returned instead, when set
	lookups int
}

func newFakeSource() *fakeSource {
	return &fakeSource{roles: map[[2]string]Role{
		{campaignA, master}: RoleMaster,
		{campaignA, player}: RolePlayer,
		{campaignB, player}: RoleMaster, // the same user can be master elsewhere (RN-05)
	}}
}

func (f *fakeSource) CampaignRole(_ context.Context, campaignID, userID string) (Role, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lookups++
	if f.err != nil {
		return "", f.err
	}
	role, ok := f.roles[[2]string{campaignID, userID}]
	if !ok {
		return "", ErrNotMember
	}
	return role, nil
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
