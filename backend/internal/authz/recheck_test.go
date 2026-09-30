package authz

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect"
)

// recheckingCaller is a fakeCaller that also implements SessionRechecker:
// sessionErr is what RecheckSession answers.
type recheckingCaller struct {
	fakeCaller
	sessionErr atomic.Pointer[error]
	rechecks   atomic.Int32
}

func (c *recheckingCaller) RecheckSession(context.Context) error {
	c.rechecks.Add(1)
	if err := c.sessionErr.Load(); err != nil {
		return *err
	}
	return nil
}

func (c *recheckingCaller) setSessionErr(err error) { c.sessionErr.Store(&err) }

// streamContext is the context a streaming handler sees, with the memo the
// interceptor gives the whole stream.
func streamContext(t *testing.T, caller Caller, source MembershipSource) context.Context {
	t.Helper()
	i := Interceptor(caller, source, slog.New(slog.DiscardHandler)).(*interceptor)
	return i.withMemo(t.Context(), "/meurpg.play.v1.PlayService/WatchGameSession")
}

// TestRecheckCampaignMemberReadsAgain: the memo would keep answering "member"
// for the whole stream; the recheck reads the database again, so a removal
// ends the stream with not_found (ADR-0005).
func TestRecheckCampaignMemberReadsAgain(t *testing.T) {
	t.Parallel()
	source := newFakeSource()
	caller := &recheckingCaller{fakeCaller: fakeCaller{player}}
	ctx := streamContext(t, caller, source)

	if _, err := RequireCampaignMember(ctx, campaignA); err != nil {
		t.Fatalf("RequireCampaignMember() error = %v", err)
	}
	source.mu.Lock()
	delete(source.members, [2]string{campaignA, player})
	source.mu.Unlock()

	// The memo still says "member": that is why streams must recheck.
	if _, err := RequireCampaignMember(ctx, campaignA); err != nil {
		t.Fatalf("memoized RequireCampaignMember() error = %v, want the memo's answer", err)
	}
	before := source.lookupCount()
	_, err := RecheckCampaignMember(ctx, campaignA)
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("RecheckCampaignMember() after removal error = %v, want not_found", err)
	}
	if source.lookupCount() != before+1 {
		t.Errorf("RecheckCampaignMember() read the database %d times, want 1", source.lookupCount()-before)
	}
	if caller.rechecks.Load() != 1 {
		t.Errorf("RecheckSession() called %d times, want 1", caller.rechecks.Load())
	}
}

func TestRecheckCampaignMember(t *testing.T) {
	t.Parallel()
	signedOut := connect.NewError(connect.CodeUnauthenticated, errors.New("sign in to continue"))
	tests := []struct {
		name       string
		user       string
		sessionErr error
		sourceErr  error
		want       connect.Code // 0 means allowed
	}{
		{"member", player, nil, nil, 0},
		{"the session was revoked", player, signedOut, nil, connect.CodeUnauthenticated},
		{"the session store is down", player, errors.New("database is down"), nil, connect.CodeUnavailable},
		{"pending member (RN-15)", pending, nil, nil, connect.CodeNotFound},
		{"outsider", outsider, nil, nil, connect.CodeNotFound},
		{"the membership store is down", player, nil, errors.New("database is down"), connect.CodeUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			source := newFakeSource()
			source.err = tt.sourceErr
			caller := &recheckingCaller{fakeCaller: fakeCaller{tt.user}}
			if tt.sessionErr != nil {
				caller.setSessionErr(tt.sessionErr)
			}
			m, err := RecheckCampaignMember(streamContext(t, caller, source), campaignA)
			switch {
			case tt.want == 0 && err != nil:
				t.Errorf("error = %v, want allowed", err)
			case tt.want == 0 && (m.CampaignID != campaignA || m.UserID != tt.user):
				t.Errorf("membership = %+v", m)
			case tt.want != 0 && connect.CodeOf(err) != tt.want:
				t.Errorf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

// TestRecheckCampaignMemberFailsClosed: without a Caller that can recheck the
// session, or without the interceptor, the stream must not go on.
func TestRecheckCampaignMemberFailsClosed(t *testing.T) {
	t.Parallel()
	ctx := streamContext(t, fakeCaller{player}, newFakeSource())
	if _, err := RecheckCampaignMember(ctx, campaignA); connect.CodeOf(err) != connect.CodeInternal {
		t.Errorf("without a SessionRechecker: error = %v, want internal", err)
	}
	if _, err := RecheckCampaignMember(t.Context(), campaignA); connect.CodeOf(err) != connect.CodeInternal {
		t.Errorf("without the interceptor: error = %v, want internal", err)
	}
}
