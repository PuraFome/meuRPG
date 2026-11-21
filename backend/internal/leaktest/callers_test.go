package leaktest

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
)

// callerVariant is one of the five requests the probe sends to a procedure, as each caller.
type callerVariant struct {
	name  string
	empty bool // every field empty: no campaign exists
	build func(w *world, p *person, method protoreflect.MethodDescriptor, other string) *dynamicpb.Message
}

var callerVariants = []callerVariant{
	{"the probe's request", false, func(w *world, p *person, m protoreflect.MethodDescriptor, _ string) *dynamicpb.Message {
		return w.probeRequest(p, m)
	}},
	{"every field empty", true, func(_ *world, _ *person, m protoreflect.MethodDescriptor, _ string) *dynamicpb.Message {
		return dynamicpb.NewMessage(m.Input())
	}},
	{"only campaign_id", false, func(w *world, _ *person, m protoreflect.MethodDescriptor, _ string) *dynamicpb.Message {
		return setStrings(dynamicpb.NewMessage(m.Input()), func(name string) (string, bool) { return w.campaign, name == "campaign_id" })
	}},
	{"an idempotency key that is not a UUID", false, func(w *world, p *person, m protoreflect.MethodDescriptor, _ string) *dynamicpb.Message {
		return setStrings(w.probeRequest(p, m), func(name string) (string, bool) { return "not-a-uuid", name == "idempotency_key" })
	}},
	{"ids of another campaign", false, func(w *world, p *person, m protoreflect.MethodDescriptor, other string) *dynamicpb.Message {
		return setStrings(w.probeRequest(p, m), func(name string) (string, bool) {
			return other, name != "campaign_id" && strings.HasSuffix(name, "_id")
		})
	}},
}

// setStrings sets every string field (not a list) that pick accepts, to the value it gives.
func setStrings(m *dynamicpb.Message, pick func(name string) (string, bool)) *dynamicpb.Message {
	fields := m.Descriptor().Fields()
	for i := range fields.Len() {
		fd := fields.Get(i)
		if fd.Kind() != protoreflect.StringKind || fd.IsList() {
			continue
		}
		if v, ok := pick(string(fd.Name())); ok {
			m.Set(fd, protoreflect.ValueOfString(v))
		}
	}
	return m
}

// secondCampaign creates a campaign whose master is the stranger: the "other campaign" of
// the ids that the probe sends in place of the fixture's.
func (w *world) secondCampaign(t *testing.T) string {
	t.Helper()
	res := must(w.stranger.campaigns.CreateCampaign(t.Context(), rq(&campaignsv1.CreateCampaignRequest{Name: "Other", XpMode: campaignsv1.XpMode_XP_MODE_MILESTONES})))
	return res.GetCampaign().GetId()
}

// errorOf is the Connect error code and message of a failed call, empty for a successful one.
func (r reply) errorOf() (code, message string) {
	if r.ok() {
		return "", ""
	}
	var e struct{ Code, Message string }
	_ = json.Unmarshal(r.body, &e)
	return e.Code, e.Message
}

// wantCode is the code that p must be answered with by a procedure of class c, and whether
// the probe requires one (see checkNotReads).
func (w *world) wantCode(procedure string, c class, p, anon *person, v callerVariant) (string, bool) {
	switch {
	case p == anon:
		return "unauthenticated", true
	case c == signedInCall:
		return "", false
	case p == w.pending && authz.PendingMayCall(procedure):
		return "", false
	case p == w.pending || p == w.stranger || v.empty:
		return "not_found", true
	case c == masterWrite:
		return "permission_denied", true
	case c == masterWriteHidden:
		return "not_found", true
	}
	return "", false // a player's own action: see checkNotReads
}

// checkCodes checks the code of every answer to one request, and that the pending member
// gets the stranger's code and message.
func checkCodes(t *testing.T, w *world, anon *person, procedure string, c classified, v callerVariant, replies map[*person]reply) {
	t.Helper()
	name := strings.TrimPrefix(procedure, "/meurpg.")
	for p, r := range replies {
		want, required := w.wantCode(procedure, c.class, p, anon, v)
		if got, _ := r.errorOf(); required && got != want {
			t.Errorf("%s (%s): %s was answered %q, want %q (%s): %s", name, v.name, p.name, got, want, c.why, shorten(r.body))
		}
	}
	if c.class == signedInCall || authz.PendingMayCall(procedure) {
		return
	}
	pc, pm := replies[w.pending].errorOf()
	sc, sm := replies[w.stranger].errorOf()
	if pc != sc || pm != sm {
		t.Errorf("%s (%s): the pending member was answered %s %q, the stranger %s %q", name, v.name, pc, pm, sc, sm)
	}
}
