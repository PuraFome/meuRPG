package characters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// Characterization of the claim, reserve and review calls (MR-049, RN-15, MR-024):
// ClaimCharacter, RevokeClaimLink, ReturnCharacterToReserve, RequestCharacterChanges,
// ResubmitCharacter, ApproveCharacter and RejectCharacter. Each test freezes who may
// call, every refusal's code, message, field and detail in the order the checks run,
// what reaches the streams, and the server's log lines, byte for byte (IDs replaced
// by names). The faults are injected through the collaborators the service is given
// (the campaigns module, the content, the display names, the combat), never by
// breaking the database.

// claimsFaults holds the errors the collaborators below return instead of answering,
// by method name, and a hook JoinAsPlayer runs once before answering.
type claimsFaults struct {
	mu       sync.Mutex
	errs     map[string]error
	inCombat bool
	onJoin   func(ctx context.Context)
	calls    []string
}

func (f *claimsFaults) set(method string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errs == nil {
		f.errs = map[string]error{}
	}
	f.errs[method] = err
}

// fault records the call and returns the error injected for it, if any.
func (f *claimsFaults) fault(method string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, method)
	return f.errs[method]
}

// called returns the methods called since the last reset.
func (f *claimsFaults) called() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := append([]string(nil), f.calls...)
	f.calls = nil
	return out
}

func (f *claimsFaults) setInCombat(in bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inCombat = in
}

func (f *claimsFaults) takeJoinHook() func(ctx context.Context) {
	f.mu.Lock()
	defer f.mu.Unlock()
	hook := f.onJoin
	f.onJoin = nil
	return hook
}

// claimsFaultMembers is the campaigns module with claimsFaults in front of it.
type claimsFaultMembers struct {
	PendingMembers
	f *claimsFaults
}

func (m claimsFaultMembers) ActivatePendingMember(ctx context.Context, tx pgx.Tx, campaignID, userID string) error {
	if err := m.f.fault("ActivatePendingMember"); err != nil {
		return err
	}
	return m.PendingMembers.ActivatePendingMember(ctx, tx, campaignID, userID)
}

func (m claimsFaultMembers) DeletePendingMember(ctx context.Context, tx pgx.Tx, campaignID, userID string) error {
	if err := m.f.fault("DeletePendingMember"); err != nil {
		return err
	}
	return m.PendingMembers.DeletePendingMember(ctx, tx, campaignID, userID)
}

func (m claimsFaultMembers) CampaignForClaim(ctx context.Context, tx pgx.Tx, campaignID, userID string) (string, bool, error) {
	if err := m.f.fault("CampaignForClaim"); err != nil {
		return "", false, err
	}
	return m.PendingMembers.CampaignForClaim(ctx, tx, campaignID, userID)
}

func (m claimsFaultMembers) JoinAsPlayer(ctx context.Context, tx pgx.Tx, campaignID, userID string) (string, error) {
	if err := m.f.fault("JoinAsPlayer"); err != nil {
		return "", err
	}
	if hook := m.f.takeJoinHook(); hook != nil {
		hook(ctx)
	}
	return m.PendingMembers.JoinAsPlayer(ctx, tx, campaignID, userID)
}

// claimsFaultContent is the rules content with claimsFaults in front of it.
type claimsFaultContent struct {
	ContentSource
	f *claimsFaults
}

func (c claimsFaultContent) ContentFor(ctx context.Context, tx pgx.Tx, campaignID string) (*rules.Content, error) {
	if err := c.f.fault("ContentFor"); err != nil {
		return nil, err
	}
	return c.ContentSource.ContentFor(ctx, tx, campaignID)
}

// claimsFaultProfiles is the display names with claimsFaults in front of them.
type claimsFaultProfiles struct {
	Profiles
	f *claimsFaults
}

func (p claimsFaultProfiles) DisplayNames(ctx context.Context, userIDs []string) (map[string]string, error) {
	if err := p.f.fault("DisplayNames"); err != nil {
		return nil, err
	}
	return p.Profiles.DisplayNames(ctx, userIDs)
}

// reserveCombatHost is the play module's side of ReturnCharacterToReserve: whether the
// character is in a combat, as claimsFaults says.
type reserveCombatHost struct {
	CreatureHost
	f *claimsFaults
}

func (r reserveCombatHost) CharacterInCombat(context.Context, pgx.Tx, string, string) (bool, error) {
	if err := r.f.fault("CharacterInCombat"); err != nil {
		return false, err
	}
	r.f.mu.Lock()
	defer r.f.mu.Unlock()
	return r.f.inCombat, nil
}

// claimsReviewTable is a campaign with a master, two active players (Caio with his own
// living character), a pending member with her pending character and a stranger, on a
// service whose collaborators take faults, whose log is read back and whose streams are
// spied on.
type claimsReviewTable struct {
	h                                *harness
	logs                             *logBuffer
	faults                           *claimsFaults
	live                             *liveSpy
	host                             *hostSpy
	master, lia, caio, eva, stranger *user
	campaign                         string
	caioPC, evaPC                    *charactersv1.Character
	labels                           map[string]string
}

func newClaimsReviewTable(t *testing.T) *claimsReviewTable {
	t.Helper()
	tb := &claimsReviewTable{logs: &logBuffer{}, faults: &claimsFaults{}, live: &liveSpy{}, host: &hostSpy{}, labels: map[string]string{}}
	tb.h = newHarnessWith(t, func(c *Config) {
		c.Logger = slog.New(slog.NewJSONHandler(tb.logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
		c.Members = claimsFaultMembers{PendingMembers: c.Members, f: tb.faults}
		c.Content = claimsFaultContent{ContentSource: c.Content, f: tb.faults}
		c.Profiles = claimsFaultProfiles{Profiles: c.Profiles, f: tb.faults}
	})
	tb.h.svc.SetLive(tb.live)
	tb.h.svc.SetReviewHost(tb.host)
	tb.master, tb.lia, tb.caio, tb.eva, tb.stranger = tb.h.newUser("Mestre"), tb.h.newUser("Lia"), tb.h.newUser("Caio"), tb.h.newUser("Eva"), tb.h.newUser("Estranho")
	tb.campaign = tb.h.newCampaign(tb.master, "Mirathel", tb.lia, tb.caio)
	tb.h.joinPending(tb.master, tb.campaign, tb.eva)
	tb.caioPC = tb.caio.create(t, tb.campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Ícaro", pensantusSheet())
	tb.evaPC = tb.eva.create(t, tb.campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Ondina", pensantusSheet())
	tb.label(tb.campaign, "<campaign>")
	tb.label(tb.caioPC.GetId(), "<Ícaro>")
	tb.label(tb.evaPC.GetId(), "<Ondina>")
	for name, u := range map[string]*user{"Mestre": tb.master, "Lia": tb.lia, "Caio": tb.caio, "Eva": tb.eva, "Estranho": tb.stranger} {
		tb.label(u.id, "<"+name+">")
	}
	return tb
}

// label names an ID in the frozen refusals and log lines.
func (tb *claimsReviewTable) label(id, name string) { tb.labels[id] = name }

// named replaces every labeled ID in s.
func (tb *claimsReviewTable) named(s string) string {
	pairs := make([]string, 0, 2*len(tb.labels))
	for id, name := range tb.labels {
		pairs = append(pairs, id, name)
	}
	return strings.NewReplacer(pairs...).Replace(s)
}

// reserved makes a reserved character as the master and labels it.
func (tb *claimsReviewTable) reserved(t *testing.T, name string) *charactersv1.Character {
	t.Helper()
	c := tb.master.reservedByMaster(t, tb.campaign, name)
	tb.label(c.GetId(), "<"+name+">")
	return c
}

// claimed makes a reserved character and has u claim it, or fails the test.
func (tb *claimsReviewTable) claimed(t *testing.T, u *user, name string) *charactersv1.Character {
	t.Helper()
	c := tb.reserved(t, name)
	if _, err := u.claim(t, tb.master.link(t, c, 7)); err != nil {
		t.Fatalf("ClaimCharacter(%s) error = %v", name, err)
	}
	return c
}

// mark is where the log is now: claimsLogLines reads what comes after it.
func (tb *claimsReviewTable) mark() int { return len(tb.logs.String()) }

// claimsLogLines renders the log lines written since mark, one per line: the level,
// the message and every attribute but the time, sorted by key, with IDs named.
func (tb *claimsReviewTable) claimsLogLines(t *testing.T, mark int) []string {
	t.Helper()
	var out []string
	for line := range strings.SplitSeq(strings.TrimSpace(tb.logs.String()[mark:]), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("log line %q: %v", line, err)
		}
		keys := make([]string, 0, len(rec))
		for k := range rec {
			if k != "time" && k != "level" && k != "msg" {
				keys = append(keys, k)
			}
		}
		slices.Sort(keys)
		parts := []string{fmt.Sprint(rec["level"]), fmt.Sprint(rec["msg"])}
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s=%v", k, rec[k]))
		}
		out = append(out, tb.named(strings.Join(parts, " ")))
	}
	return out
}

// wantClaimsLog fails the test unless the log lines since mark are want.
func (tb *claimsReviewTable) wantClaimsLog(t *testing.T, call string, mark int, want ...string) {
	t.Helper()
	got := tb.claimsLogLines(t, mark)
	if !slices.Equal(got, want) {
		t.Errorf("%s log lines:\n got %q\nwant %q", call, got, want)
	}
}

// claimsRefusal renders a refusal the way the web reads it: the code, the message, the
// InvalidField's field and the CharacterBlocked detail, IDs named. nil is "ok".
func (tb *claimsReviewTable) claimsRefusal(err error) string {
	if err == nil {
		return "ok"
	}
	ce, ok := errors.AsType[*connect.Error](err)
	if !ok {
		return "not a connect error: " + err.Error()
	}
	out := fmt.Sprintf("%s %q", ce.Code(), ce.Message())
	for _, d := range ce.Details() {
		v, err := d.Value()
		if err != nil {
			out += " undecodable-detail"
			continue
		}
		switch detail := v.(type) {
		case *charactersv1.InvalidField:
			out += " field=" + detail.GetField()
		case *charactersv1.CharacterBlocked:
			out += " blocked=" + strings.TrimPrefix(detail.GetReason().String(), "CHARACTER_BLOCKED_REASON_")
			for _, kv := range [][2]string{
				{"id", detail.GetCharacterId()},
				{"name", detail.GetCharacterName()},
				{"campaign", detail.GetCampaignId()},
				{"player", detail.GetPlayerDisplayName()},
				{"content", detail.GetContentKey()},
			} {
				if kv[1] != "" {
					out += " " + kv[0] + "=" + kv[1]
				}
			}
		default:
			out += fmt.Sprintf(" detail=%T", v)
		}
	}
	return tb.named(out)
}

// wantClaimsRefusal fails the test unless err renders as want (claimsRefusal).
func (tb *claimsReviewTable) wantClaimsRefusal(t *testing.T, call string, err error, want string) {
	t.Helper()
	if got := tb.claimsRefusal(err); got != want {
		t.Errorf("%s:\n got %s\nwant %s", call, got, want)
	}
}

// The calls of the batch as one shape, for the tables below.
func (u *user) revokeLinkOf(campaignID, characterID string) error {
	_, err := u.api.RevokeClaimLink(context.Background(), connect.NewRequest(&charactersv1.RevokeClaimLinkRequest{CampaignId: campaignID, CharacterId: characterID}))
	return err
}

func (u *user) returnToReserve(campaignID, characterID string) (*charactersv1.Character, error) {
	res, err := u.api.ReturnCharacterToReserve(context.Background(), connect.NewRequest(&charactersv1.ReturnCharacterToReserveRequest{CampaignId: campaignID, CharacterId: characterID}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetCharacter(), nil
}

func (u *user) askChanges(campaignID, characterID, reason, key string) (*charactersv1.Character, error) {
	res, err := u.api.RequestCharacterChanges(context.Background(), connect.NewRequest(&charactersv1.RequestCharacterChangesRequest{
		CampaignId: campaignID, CharacterId: characterID, Reason: reason, IdempotencyKey: key,
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetCharacter(), nil
}

func (u *user) resubmitAgain(campaignID, characterID, key string) (*charactersv1.Character, error) {
	res, err := u.api.ResubmitCharacter(context.Background(), connect.NewRequest(&charactersv1.ResubmitCharacterRequest{
		CampaignId: campaignID, CharacterId: characterID, IdempotencyKey: key,
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetCharacter(), nil
}

func (u *user) approveOf(campaignID, characterID string) (*charactersv1.Character, error) {
	res, err := u.api.ApproveCharacter(context.Background(), connect.NewRequest(&charactersv1.ApproveCharacterRequest{CampaignId: campaignID, CharacterId: characterID}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetCharacter(), nil
}

func (u *user) rejectOf(campaignID, characterID string) error {
	_, err := u.api.RejectCharacter(context.Background(), connect.NewRequest(&charactersv1.RejectCharacterRequest{CampaignId: campaignID, CharacterId: characterID}))
	return err
}

// The refusals every test below compares with.
const (
	claimsWantUnauthenticated = `unauthenticated "sign in to continue"`
	claimsWantNoCampaign      = `not_found "campaign not found"`
	claimsWantMasterOnly      = `permission_denied "only the campaign's master can do this"`
	claimsWantNoCharacter     = `not_found "character not found"`
	claimsWantUnusable        = `not_found "this claim link cannot be used"`
	claimsWantInternal        = `internal "internal error"`
	claimsWantKeyRequired     = `invalid_argument "idempotency_key is required" field=idempotency_key`
	claimsWantKeyReused       = `invalid_argument "idempotency_key was used for another change"`
)

// Who may call: every caller of each of the seven calls, with the first refusal each
// one gets, the checks' order (campaign, ID, key, reason) included.
func TestClaimsReviewFrozen_WhoMayCall(t *testing.T) {
	t.Parallel()
	tb := newClaimsReviewTable(t)
	kai := tb.claimed(t, tb.lia, "Kai")
	unknown := uuid.New().String()
	anonymous := tb.h.anonymous()

	type call struct {
		name string
		do   func(u *user, campaignID, characterID string) error
	}
	masterOnly := []call{
		{"RevokeClaimLink", func(u *user, c, id string) error { return u.revokeLinkOf(c, id) }},
		{"ReturnCharacterToReserve", func(u *user, c, id string) error { _, err := u.returnToReserve(c, id); return err }},
		{"RequestCharacterChanges", func(u *user, c, id string) error { _, err := u.askChanges(c, id, "Falta algo.", key()); return err }},
		{"ApproveCharacter", func(u *user, c, id string) error { _, err := u.approveOf(c, id); return err }},
		{"RejectCharacter", func(u *user, c, id string) error { return u.rejectOf(c, id) }},
	}
	for _, c := range masterOnly {
		for _, tc := range []struct {
			who        string
			u          *user
			campaignID string
			id         string
			want       string
		}{
			{"nobody", anonymous, tb.campaign, kai.GetId(), claimsWantUnauthenticated},
			{"a stranger", tb.stranger, tb.campaign, kai.GetId(), claimsWantNoCampaign},
			{"a pending member", tb.eva, tb.campaign, tb.evaPC.GetId(), claimsWantNoCampaign},
			{"a player", tb.lia, tb.campaign, kai.GetId(), claimsWantMasterOnly},
			{"the master, a campaign that is not a UUID", tb.master, "mirathel", kai.GetId(), claimsWantNoCampaign},
			{"the master, a character that is not a UUID", tb.master, tb.campaign, "kai", claimsWantNoCharacter},
			{"the master, an unknown character", tb.master, tb.campaign, unknown, claimsWantNoCharacter},
		} {
			tb.wantClaimsRefusal(t, c.name+" by "+tc.who, c.do(tc.u, tc.campaignID, tc.id), tc.want)
		}
	}

	// ResubmitCharacter is the owner's alone: anyone else in the campaign gets the answer
	// of a character that is not there.
	evaKey := key()
	for _, tc := range []struct {
		who        string
		u          *user
		campaignID string
		id         string
		key        string
		want       string
	}{
		{"nobody", anonymous, tb.campaign, tb.evaPC.GetId(), evaKey, claimsWantUnauthenticated},
		{"a stranger", tb.stranger, tb.campaign, tb.evaPC.GetId(), evaKey, claimsWantNoCampaign},
		{"another player", tb.lia, tb.campaign, tb.evaPC.GetId(), evaKey, claimsWantNoCharacter},
		{"the master", tb.master, tb.campaign, tb.evaPC.GetId(), evaKey, claimsWantNoCharacter},
		{"the owner, a character that is not a UUID, no key", tb.eva, tb.campaign, "ondina", "", claimsWantNoCharacter},
		{"the owner, no key", tb.eva, tb.campaign, tb.evaPC.GetId(), "", claimsWantKeyRequired},
		{"the owner, a key of spaces", tb.eva, tb.campaign, tb.evaPC.GetId(), "   ", claimsWantKeyRequired},
		{"the owner, a key of 65 characters", tb.eva, tb.campaign, tb.evaPC.GetId(), strings.Repeat("k", 65), `invalid_argument "idempotency_key must be 1 to 64 characters" field=idempotency_key`},
		{"the owner, an unknown character", tb.eva, tb.campaign, unknown, evaKey, claimsWantNoCharacter},
		{"the owner, nothing asked yet", tb.eva, tb.campaign, tb.evaPC.GetId(), evaKey, `failed_precondition "" blocked=NO_CHANGES_REQUESTED id=<Ondina>`},
	} {
		_, err := tc.u.resubmitAgain(tc.campaignID, tc.id, tc.key)
		tb.wantClaimsRefusal(t, "ResubmitCharacter by "+tc.who, err, tc.want)
	}

	// ClaimCharacter: anyone signed in; nobody else.
	_, err := anonymous.claim(t, "x")
	tb.wantClaimsRefusal(t, "ClaimCharacter by nobody", err, claimsWantUnauthenticated)
	_, err = tb.stranger.claim(t, "x")
	tb.wantClaimsRefusal(t, "ClaimCharacter by a stranger, not a token", err, claimsWantUnusable)

	// RequestCharacterChanges checks the ID, then the key, then the reason, and only then
	// reads the character: an unknown character with a bad reason is the reason's refusal.
	for _, tc := range []struct {
		name, id, reason, key, want string
	}{
		{"not a UUID, no key, no reason", "ondina", "", "", claimsWantNoCharacter},
		{"no key, no reason", tb.evaPC.GetId(), "", "", claimsWantKeyRequired},
		{"a key of 65 characters, no reason", tb.evaPC.GetId(), "", strings.Repeat("k", 65), `invalid_argument "idempotency_key must be 1 to 64 characters" field=idempotency_key`},
		{"no reason", tb.evaPC.GetId(), "", key(), `invalid_argument "reason is required" field=reason`},
		{"a reason of spaces", tb.evaPC.GetId(), " \n\t ", key(), `invalid_argument "reason is required" field=reason`},
		{"a reason of 501 characters", tb.evaPC.GetId(), strings.Repeat("ã", 501), key(), `invalid_argument "reason must be at most 500 characters" field=reason`},
		// Finding: a reason with a control character is refused with the length's
		// sentence ("must be at most 500 characters"), not with what is wrong in it.
		{"a reason with a control character", tb.evaPC.GetId(), "Falta\x07 algo.", key(), `invalid_argument "reason must be at most 500 characters" field=reason`},
		{"an unknown character with no reason", unknown, "", key(), `invalid_argument "reason is required" field=reason`},
	} {
		_, err := tb.master.askChanges(tb.campaign, tc.id, tc.reason, tc.key)
		tb.wantClaimsRefusal(t, "RequestCharacterChanges, "+tc.name, err, tc.want)
	}
	if got := tb.h.reviewRows(tb.campaign); got != 0 {
		t.Errorf("%d review rows after refused calls, want 0", got)
	}
	if got := tb.host.told(); len(got) != 0 {
		t.Errorf("the streams were told %v after refused calls, want nothing", got)
	}
}

// ClaimCharacter: the answer, the stream and the log line of a claim, and each refusal
// of a link that works for a person who may not take the character.
func TestClaimsReviewFrozen_ClaimCharacter(t *testing.T) {
	t.Parallel()
	tb := newClaimsReviewTable(t)

	kai := tb.reserved(t, "Kai")
	token := tb.master.link(t, kai, 7)
	mark := tb.mark()
	res, err := tb.lia.claim(t, token)
	if err != nil {
		t.Fatalf("ClaimCharacter() error = %v", err)
	}
	if got := tb.named(fmt.Sprintf("%s %s %s %s", res.GetCampaignId(), res.GetCampaignName(), res.GetCharacterId(), res.GetCharacterName())); got != "<campaign> Mirathel <Kai> Kai" {
		t.Errorf("ClaimCharacter() = %s, want <campaign> Mirathel <Kai> Kai", got)
	}
	tb.wantClaimsLog(t, "ClaimCharacter()", mark, "DEBUG event campaign_id=<campaign> character_id=<Kai> event=character.claimed")
	if got := tb.live.count(); got != 1 {
		t.Errorf("the party was told %d times, want 1", got)
	}

	// The master's own link: a refusal with no sentence.
	own := tb.reserved(t, "Ragna")
	ownToken := tb.master.link(t, own, 7)
	mark = tb.mark()
	_, err = tb.master.claim(t, ownToken)
	// Finding: errBlocked has no sentence for CLAIM_OWN_LINK, so the message is empty.
	tb.wantClaimsRefusal(t, "ClaimCharacter(own link)", err, `failed_precondition "" blocked=CLAIM_OWN_LINK id=<Ragna>`)

	// RN-03: a living character, and a pending one, count; Lia now has Kai.
	_, err = tb.lia.claim(t, ownToken)
	tb.wantClaimsRefusal(t, "ClaimCharacter(RN-03, a claimed character)", err,
		`failed_precondition "you already have a living character in this campaign" blocked=LIVING_CHARACTER_EXISTS id=<Kai> name=Kai campaign=<campaign>`)
	_, err = tb.eva.claim(t, ownToken)
	tb.wantClaimsRefusal(t, "ClaimCharacter(RN-03, a pending character)", err,
		`failed_precondition "you already have a living character in this campaign" blocked=LIVING_CHARACTER_EXISTS id=<Ondina> name=Ondina campaign=<campaign>`)
	tb.wantClaimsLog(t, "the refused claims", mark)
	if got := tb.live.count(); got != 1 {
		t.Errorf("the party was told %d times after refusals, want still 1", got)
	}

	// Faults of the campaigns module: the refusal and the log line of each.
	tb.faults.set("CampaignForClaim", errors.New("claims-fault-campaign"))
	mark = tb.mark()
	_, err = tb.stranger.claim(t, ownToken)
	tb.wantClaimsRefusal(t, "ClaimCharacter(CampaignForClaim fails)", err, claimsWantInternal)
	tb.wantClaimsLog(t, "ClaimCharacter(CampaignForClaim fails)", mark,
		"ERROR characters: cannot claim a character code=internal error=read the campaign of a claim link: claims-fault-campaign")
	// A campaign the module cannot find is the answer of a character that is not there
	// (PreviewClaim answers the unusable link instead).
	tb.faults.set("CampaignForClaim", pgx.ErrNoRows)
	_, err = tb.stranger.claim(t, ownToken)
	tb.wantClaimsRefusal(t, "ClaimCharacter(CampaignForClaim finds nothing)", err, claimsWantNoCharacter)
	tb.faults.set("CampaignForClaim", nil)

	tb.faults.set("JoinAsPlayer", errors.New("claims-fault-join"))
	mark = tb.mark()
	_, err = tb.stranger.claim(t, ownToken)
	tb.wantClaimsRefusal(t, "ClaimCharacter(JoinAsPlayer fails)", err, claimsWantInternal)
	tb.wantClaimsLog(t, "ClaimCharacter(JoinAsPlayer fails)", mark,
		"ERROR characters: cannot claim a character code=internal error=join the campaign: claims-fault-join")
	tb.faults.set("JoinAsPlayer", nil)
	if got := tb.master.get(t, tb.campaign, own.GetId()); !got.GetReserved() || tb.h.memberStatus(tb.campaign, tb.stranger.id) != "" {
		t.Errorf("after the faults Ragna reserved %v, the stranger's membership %q; want reserved and none", got.GetReserved(), tb.h.memberStatus(tb.campaign, tb.stranger.id))
	}

	// A token that is not one, and no token without a sign-in, are the unusable link.
	for name, tok := range map[string]string{"not a token": "x", "no token, no sign-in": ""} {
		_, err := tb.stranger.claim(t, tok)
		tb.wantClaimsRefusal(t, "ClaimCharacter("+name+")", err, claimsWantUnusable)
	}
	if _, err := tb.stranger.claim(t, ownToken); err != nil {
		t.Errorf("ClaimCharacter() once the faults are gone error = %v", err)
	}
}

// ClaimCharacter: a living character of the person written by someone else between the
// claim's RN-03 check and its write. The answer is RN-03, as the check would have given.
func TestClaimsReviewFrozen_ClaimLosesRN03ToAnotherWrite(t *testing.T) {
	t.Parallel()
	dbtest.PoolSize(t, 2) // the side write needs its own connection
	tb := newClaimsReviewTable(t)
	kai, ragna := tb.reserved(t, "Kai"), tb.reserved(t, "Ragna")
	token := tb.master.link(t, kai, 7)
	side := dbtest.SideConnection(t, tb.h.pool)
	tb.faults.mu.Lock()
	tb.faults.onJoin = func(ctx context.Context) {
		// Ragna becomes Lia's, committed, while the claim's transaction is open.
		if _, err := side.Exec(ctx, "UPDATE characters SET reserved = false, player_user_id = $1, claimed_at = now() WHERE id = $2", tb.lia.id, ragna.GetId()); err != nil {
			t.Errorf("side write: %v", err)
		}
	}
	tb.faults.mu.Unlock()
	_, err := tb.lia.claim(t, token)
	refusal := tb.claimsRefusal(err)
	// Whether the database answers with the unique index (no name) or with a retry
	// that finds Ragna (her name) depends on its timing; both are RN-03 naming Ragna.
	if refusal != `failed_precondition "you already have a living character in this campaign" blocked=LIVING_CHARACTER_EXISTS id=<Ragna> name=Ragna campaign=<campaign>` &&
		refusal != `failed_precondition "you already have a living character in this campaign" blocked=LIVING_CHARACTER_EXISTS id=<Ragna> campaign=<campaign>` {
		t.Errorf("ClaimCharacter(lost RN-03) = %s, want RN-03 naming Ragna", refusal)
	}
	t.Logf("ClaimCharacter(lost RN-03) = %s", refusal)
	if got := tb.master.get(t, tb.campaign, kai.GetId()); !got.GetReserved() {
		t.Error("Kai was taken although RN-03 refused the claim")
	}
}

// RevokeClaimLink: every kind of character and link, with its answer and log line.
func TestClaimsReviewFrozen_RevokeClaimLink(t *testing.T) {
	t.Parallel()
	tb := newClaimsReviewTable(t)
	npc := tb.master.create(t, tb.campaign, charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, "Orc", enemySheet())
	tb.label(npc.GetId(), "<Orc>")
	kai := tb.reserved(t, "Kai")

	for _, tc := range []struct {
		name    string
		id      string
		want    string
		logline []string
	}{
		{"an NPC", npc.GetId(), `invalid_argument "character_id is an NPC: only a reserved player character has a claim link" field=character_id`, nil},
		// Finding: errBlocked has no sentence for NOT_RESERVED, so the message is empty.
		{"a player's own character", tb.caioPC.GetId(), `failed_precondition "" blocked=NOT_RESERVED id=<Ícaro>`, nil},
		{"a pending character", tb.evaPC.GetId(), `failed_precondition "" blocked=NOT_RESERVED id=<Ondina>`, nil},
		{"a reserved character with no link", kai.GetId(), "ok", []string{"DEBUG event character_id=<Kai> event=character.claim_link_revoked"}},
	} {
		mark := tb.mark()
		tb.wantClaimsRefusal(t, "RevokeClaimLink("+tc.name+")", tb.master.revokeLinkOf(tb.campaign, tc.id), tc.want)
		tb.wantClaimsLog(t, "RevokeClaimLink("+tc.name+")", mark, tc.logline...)
	}

	// A live link, then the same again: both answer ok and both log the revoke.
	tb.master.link(t, kai, 7)
	for _, name := range []string{"a live link", "a revoked link"} {
		mark := tb.mark()
		tb.wantClaimsRefusal(t, "RevokeClaimLink("+name+")", tb.master.revokeLinkOf(tb.campaign, kai.GetId()), "ok")
		tb.wantClaimsLog(t, "RevokeClaimLink("+name+")", mark, "DEBUG event character_id=<Kai> event=character.claim_link_revoked")
	}

	// A link Lia used: the refusal names her, and nothing is logged.
	if _, err := tb.lia.claim(t, tb.master.link(t, kai, 7)); err != nil {
		t.Fatalf("ClaimCharacter() error = %v", err)
	}
	mark := tb.mark()
	tb.wantClaimsRefusal(t, "RevokeClaimLink(used by Lia)", tb.master.revokeLinkOf(tb.campaign, kai.GetId()),
		`failed_precondition "a player already used this link and took the character" blocked=CLAIM_LINK_USED id=<Kai> player=Lia`)
	tb.wantClaimsLog(t, "RevokeClaimLink(used by Lia)", mark)
	// The name cannot be read: the refusal is the database's, with its line.
	tb.faults.set("DisplayNames", errors.New("claims-fault-names"))
	mark = tb.mark()
	tb.wantClaimsRefusal(t, "RevokeClaimLink(used, names fail)", tb.master.revokeLinkOf(tb.campaign, kai.GetId()), claimsWantInternal)
	tb.wantClaimsLog(t, "RevokeClaimLink(used, names fail)", mark,
		"ERROR characters: cannot read a display name code=internal error=read display names: claims-fault-names")
	tb.faults.set("DisplayNames", nil)

	// Given back to the reserve, the used link is no live link: ok, and logged.
	if _, err := tb.master.returnToReserve(tb.campaign, kai.GetId()); err != nil {
		t.Fatalf("ReturnCharacterToReserve() error = %v", err)
	}
	mark = tb.mark()
	tb.wantClaimsRefusal(t, "RevokeClaimLink(used, then given back)", tb.master.revokeLinkOf(tb.campaign, kai.GetId()), "ok")
	tb.wantClaimsLog(t, "RevokeClaimLink(used, then given back)", mark, "DEBUG event character_id=<Kai> event=character.claim_link_revoked")

	// The player who used it deleted the account: the refusal names nobody.
	ragna := tb.claimed(t, tb.h.newUser("Breve"), "Ragna")
	var owner string
	if err := tb.h.pool.QueryRow(t.Context(), "SELECT player_user_id::STRING FROM characters WHERE id = $1", ragna.GetId()).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	tb.h.deleteUser(owner)
	tb.wantClaimsRefusal(t, "RevokeClaimLink(used by a deleted account)", tb.master.revokeLinkOf(tb.campaign, ragna.GetId()),
		`failed_precondition "a player already used this link and took the character" blocked=CLAIM_LINK_USED id=<Ragna>`)
}

// ReturnCharacterToReserve: every kind of character, the combat's answer, the faults,
// and the answer, the stream and the log line of a character given back.
func TestClaimsReviewFrozen_ReturnCharacterToReserve(t *testing.T) {
	t.Parallel()
	tb := newClaimsReviewTable(t)
	npc := tb.master.create(t, tb.campaign, charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, "Orc", enemySheet())
	tb.label(npc.GetId(), "<Orc>")
	kai := tb.claimed(t, tb.lia, "Kai")
	reserved := tb.reserved(t, "Ragna")
	dead := tb.claimed(t, tb.h.newUser("Morto"), "Defunto")
	tb.master.markDead(t, tb.master.get(t, tb.campaign, dead.GetId()))
	partyBefore := tb.live.count()

	// Finding: errBlocked has no sentence for NOT_CLAIMED nor CHARACTER_IN_COMBAT, so the
	// messages are empty.
	for _, tc := range []struct {
		name, id, want string
	}{
		{"an NPC", npc.GetId(), `failed_precondition "" blocked=NOT_CLAIMED id=<Orc>`},
		{"a reserved character", reserved.GetId(), `failed_precondition "" blocked=NOT_CLAIMED id=<Ragna>`},
		{"a player's own character", tb.caioPC.GetId(), `failed_precondition "" blocked=NOT_CLAIMED id=<Ícaro>`},
		{"a pending character", tb.evaPC.GetId(), `failed_precondition "" blocked=NOT_CLAIMED id=<Ondina>`},
		{"a dead claimed character", dead.GetId(), `failed_precondition "" blocked=NOT_CLAIMED id=<Defunto>`},
	} {
		_, err := tb.master.returnToReserve(tb.campaign, tc.id)
		tb.wantClaimsRefusal(t, "ReturnCharacterToReserve("+tc.name+")", err, tc.want)
	}

	// The rules content is read before anything else, the character's checks included.
	tb.faults.set("ContentFor", errors.New("claims-fault-content"))
	mark := tb.mark()
	_, err := tb.master.returnToReserve(tb.campaign, uuid.New().String())
	tb.wantClaimsRefusal(t, "ReturnCharacterToReserve(content fails)", err, claimsWantInternal)
	tb.wantClaimsLog(t, "ReturnCharacterToReserve(content fails)", mark,
		"ERROR characters: cannot read rules content code=internal error=claims-fault-content")
	tb.faults.set("ContentFor", nil)

	// The combat holds it, or cannot say.
	tb.h.svc.SetCreatureHost(reserveCombatHost{f: tb.faults})
	tb.faults.setInCombat(true)
	_, err = tb.master.returnToReserve(tb.campaign, kai.GetId())
	tb.wantClaimsRefusal(t, "ReturnCharacterToReserve(in a combat)", err, `failed_precondition "" blocked=CHARACTER_IN_COMBAT id=<Kai>`)
	tb.faults.set("CharacterInCombat", errors.New("claims-fault-combat"))
	mark = tb.mark()
	_, err = tb.master.returnToReserve(tb.campaign, kai.GetId())
	tb.wantClaimsRefusal(t, "ReturnCharacterToReserve(the combat fails)", err, claimsWantInternal)
	tb.wantClaimsLog(t, "ReturnCharacterToReserve(the combat fails)", mark,
		"ERROR characters: cannot return a character to the reserve code=internal error=find the character in a combat: claims-fault-combat")
	tb.faults.set("CharacterInCombat", nil)
	tb.faults.setInCombat(false)
	if got := tb.live.count(); got != partyBefore {
		t.Errorf("the party was told %d times by refusals, want 0", got-partyBefore)
	}

	mark = tb.mark()
	back, err := tb.master.returnToReserve(tb.campaign, kai.GetId())
	if err != nil {
		t.Fatalf("ReturnCharacterToReserve() error = %v", err)
	}
	if !back.GetReserved() || back.GetPlayerUserId() != "" || back.GetName() != "Kai" {
		t.Errorf("ReturnCharacterToReserve() = reserved %v, owner %q, name %q; want Kai reserved with no owner", back.GetReserved(), back.GetPlayerUserId(), back.GetName())
	}
	tb.wantClaimsLog(t, "ReturnCharacterToReserve()", mark, "DEBUG event character_id=<Kai> event=character.returned_to_reserve")
	if got := tb.live.count(); got != partyBefore+1 {
		t.Errorf("the party was told %d times, want 1", got-partyBefore)
	}
	// The link Lia used cannot be used again.
	if got := summaryOf(tb.master.list(t, tb.campaign), kai.GetId()); got.GetClaimState() != charactersv1.ClaimState_CLAIM_STATE_NONE {
		t.Errorf("claim state after the return = %v, want NONE", got.GetClaimState())
	}
}

// RequestCharacterChanges: every kind of character, the replay, the reused key, the
// faults, and the answer, the stream and the log line of a request.
func TestClaimsReviewFrozen_RequestCharacterChanges(t *testing.T) {
	t.Parallel()
	tb := newClaimsReviewTable(t)
	npc := tb.master.create(t, tb.campaign, charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, "Orc", enemySheet())
	tb.label(npc.GetId(), "<Orc>")
	reserved := tb.reserved(t, "Kai")
	orphan := tb.h.newUser("Órfã")
	tb.h.joinPending(tb.master, tb.campaign, orphan)
	orphanPC := orphan.create(t, tb.campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Sem Dono", pensantusSheet())
	tb.label(orphanPC.GetId(), "<Sem Dono>")
	tb.h.deleteUser(orphan.id)

	for _, tc := range []struct {
		name, id, want string
	}{
		{"an NPC", npc.GetId(), `invalid_argument "character_id is an NPC: only player characters wait for approval" field=character_id`},
		{"an active character", tb.caioPC.GetId(), `failed_precondition "the character does not wait for approval: once approved, it stays in the campaign" blocked=NOT_PENDING id=<Ícaro>`},
		{"a reserved character", reserved.GetId(), `failed_precondition "the character does not wait for approval: once approved, it stays in the campaign" blocked=NOT_PENDING id=<Kai>`},
		{"a pending character whose player deleted the account", orphanPC.GetId(), `invalid_argument "character_id has no player to answer: approve or reject it" field=character_id`},
	} {
		_, err := tb.master.askChanges(tb.campaign, tc.id, "Falta algo.", key())
		tb.wantClaimsRefusal(t, "RequestCharacterChanges("+tc.name+")", err, tc.want)
	}

	// The content is read after the request's own checks and before the character's.
	tb.faults.set("ContentFor", errors.New("claims-fault-content"))
	mark := tb.mark()
	_, err := tb.master.askChanges(tb.campaign, uuid.New().String(), "Falta algo.", key())
	tb.wantClaimsRefusal(t, "RequestCharacterChanges(content fails)", err, claimsWantInternal)
	tb.wantClaimsLog(t, "RequestCharacterChanges(content fails)", mark, "ERROR characters: cannot read rules content code=internal error=claims-fault-content")
	tb.faults.set("ContentFor", nil)

	k := key()
	mark = tb.mark()
	asked, err := tb.master.askChanges(tb.campaign, tb.evaPC.GetId(), "  Falta o equipamento. ", k)
	if err != nil {
		t.Fatalf("RequestCharacterChanges() error = %v", err)
	}
	if r := asked.GetReview(); r.GetStatus() != charactersv1.Review_REVIEW_CHANGES_REQUESTED || r.GetReason() != "Falta o equipamento." {
		t.Errorf("RequestCharacterChanges() review = %v, want CHANGES_REQUESTED with the trimmed reason", r)
	}
	tb.wantClaimsLog(t, "RequestCharacterChanges()", mark, "DEBUG event character_id=<Ondina> event=character.changes_requested reason_length=20")
	// The retry: the same answer, no log line, nothing told.
	mark = tb.mark()
	if _, err := tb.master.askChanges(tb.campaign, tb.evaPC.GetId(), "  Falta o equipamento. ", k); err != nil {
		t.Fatalf("RequestCharacterChanges(retry) error = %v", err)
	}
	tb.wantClaimsLog(t, "RequestCharacterChanges(retry)", mark)
	// The key for another reason, or for the same request with the IDs in capitals,
	// is a reused key.
	_, err = tb.master.askChanges(tb.campaign, tb.evaPC.GetId(), "Outro motivo.", k)
	tb.wantClaimsRefusal(t, "RequestCharacterChanges(the key, another reason)", err, claimsWantKeyReused)
	// Finding: the request's hash is taken over the IDs as they came, so a retry that
	// spells the same IDs in capitals is refused as a reused key.
	_, err = tb.master.askChanges(strings.ToUpper(tb.campaign), strings.ToUpper(tb.evaPC.GetId()), "  Falta o equipamento. ", k)
	tb.wantClaimsRefusal(t, "RequestCharacterChanges(the key, the IDs in capitals)", err, claimsWantKeyReused)

	// The answer cannot be read after the commit: the request stays, logged and told.
	tb.faults.set("DisplayNames", errors.New("claims-fault-names"))
	mark = tb.mark()
	_, err = tb.master.askChanges(tb.campaign, tb.evaPC.GetId(), "Segundo pedido.", key())
	tb.wantClaimsRefusal(t, "RequestCharacterChanges(names fail after the commit)", err, claimsWantInternal)
	tb.wantClaimsLog(t, "RequestCharacterChanges(names fail after the commit)", mark,
		"DEBUG event character_id=<Ondina> event=character.changes_requested reason_length=15",
		"ERROR characters: cannot read a character code=internal error=read display names: claims-fault-names")
	tb.faults.set("DisplayNames", nil)
	if got := tb.named(strings.Join(tb.host.told(), ",")); got != "changes_requested:<Ondina>,changes_requested:<Ondina>" {
		t.Errorf("the streams were told %s, want two changes_requested", got)
	}
	if got := tb.master.get(t, tb.campaign, tb.evaPC.GetId()).GetReview().GetReason(); got != "Segundo pedido." {
		t.Errorf("reason after the failed answer = %q, want the second one", got)
	}
}

// ResubmitCharacter: every state of the review, the replay, the reused key, the faults,
// and the answer, the stream and the log line of a resubmission.
func TestClaimsReviewFrozen_ResubmitCharacter(t *testing.T) {
	t.Parallel()
	tb := newClaimsReviewTable(t)

	// The content is read after the call's own checks and before the character's.
	tb.faults.set("ContentFor", errors.New("claims-fault-content"))
	mark := tb.mark()
	_, err := tb.eva.resubmitAgain(tb.campaign, uuid.New().String(), key())
	tb.wantClaimsRefusal(t, "ResubmitCharacter(content fails)", err, claimsWantInternal)
	tb.wantClaimsLog(t, "ResubmitCharacter(content fails)", mark, "ERROR characters: cannot read rules content code=internal error=claims-fault-content")
	tb.faults.set("ContentFor", nil)

	if _, err := tb.master.askChanges(tb.campaign, tb.evaPC.GetId(), "Falta o equipamento.", key()); err != nil {
		t.Fatalf("RequestCharacterChanges() error = %v", err)
	}
	k := key()
	mark = tb.mark()
	sent, err := tb.eva.resubmitAgain(tb.campaign, tb.evaPC.GetId(), k)
	if err != nil {
		t.Fatalf("ResubmitCharacter() error = %v", err)
	}
	if sent.GetReview().GetStatus() != charactersv1.Review_REVIEW_RESUBMITTED || sent.GetCanResubmit() {
		t.Errorf("ResubmitCharacter() review %v can_resubmit %v; want RESUBMITTED and nothing more to send", sent.GetReview(), sent.GetCanResubmit())
	}
	tb.wantClaimsLog(t, "ResubmitCharacter()", mark, "DEBUG event character_id=<Ondina> event=character.resubmitted")
	mark = tb.mark()
	if _, err := tb.eva.resubmitAgain(tb.campaign, tb.evaPC.GetId(), k); err != nil {
		t.Fatalf("ResubmitCharacter(retry) error = %v", err)
	}
	tb.wantClaimsLog(t, "ResubmitCharacter(retry)", mark)
	// Finding: as RequestCharacterChanges, a retry with the IDs in capitals is a reused key.
	_, err = tb.eva.resubmitAgain(strings.ToUpper(tb.campaign), strings.ToUpper(tb.evaPC.GetId()), k)
	tb.wantClaimsRefusal(t, "ResubmitCharacter(the key, the IDs in capitals)", err, claimsWantKeyReused)
	// Resubmitted already: another key finds nothing open.
	_, err = tb.eva.resubmitAgain(tb.campaign, tb.evaPC.GetId(), key())
	// Finding: errBlocked has no sentence for NO_CHANGES_REQUESTED, so the message is empty.
	tb.wantClaimsRefusal(t, "ResubmitCharacter(resubmitted already)", err, `failed_precondition "" blocked=NO_CHANGES_REQUESTED id=<Ondina>`)
	if got := tb.named(strings.Join(tb.host.told(), ",")); got != "changes_requested:<Ondina>,resubmitted:<Ondina>" {
		t.Errorf("the streams were told %s, want changes_requested then resubmitted", got)
	}

	// The answer cannot be read after the commit: the resubmission stays, logged and told.
	if _, err := tb.master.askChanges(tb.campaign, tb.evaPC.GetId(), "Ainda falta.", key()); err != nil {
		t.Fatalf("RequestCharacterChanges() error = %v", err)
	}
	tb.faults.set("DisplayNames", errors.New("claims-fault-names"))
	mark = tb.mark()
	_, err = tb.eva.resubmitAgain(tb.campaign, tb.evaPC.GetId(), key())
	tb.wantClaimsRefusal(t, "ResubmitCharacter(names fail after the commit)", err, claimsWantInternal)
	tb.wantClaimsLog(t, "ResubmitCharacter(names fail after the commit)", mark,
		"DEBUG event character_id=<Ondina> event=character.resubmitted",
		"ERROR characters: cannot read a character code=internal error=read display names: claims-fault-names")
	tb.faults.set("DisplayNames", nil)

	// Approved: no longer pending.
	if _, err := tb.master.approveOf(tb.campaign, tb.evaPC.GetId()); err != nil {
		t.Fatalf("ApproveCharacter() error = %v", err)
	}
	_, err = tb.eva.resubmitAgain(tb.campaign, tb.evaPC.GetId(), key())
	tb.wantClaimsRefusal(t, "ResubmitCharacter(approved)", err,
		`failed_precondition "the character does not wait for approval: once approved, it stays in the campaign" blocked=NOT_PENDING id=<Ondina>`)
}

// ApproveCharacter and RejectCharacter: every kind of character, the faults of the
// campaigns module (the transaction gives everything back), who is called, and the
// answers and log lines.
func TestClaimsReviewFrozen_ApproveAndReject(t *testing.T) {
	t.Parallel()
	tb := newClaimsReviewTable(t)
	npc := tb.master.create(t, tb.campaign, charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, "Orc", enemySheet())
	tb.label(npc.GetId(), "<Orc>")

	_, err := tb.master.approveOf(tb.campaign, npc.GetId())
	tb.wantClaimsRefusal(t, "ApproveCharacter(an NPC)", err, `invalid_argument "character_id is an NPC: only player characters wait for approval" field=character_id`)
	tb.wantClaimsRefusal(t, "RejectCharacter(an NPC)", tb.master.rejectOf(tb.campaign, npc.GetId()), `invalid_argument "character_id is an NPC: only player characters wait for approval" field=character_id`)
	tb.wantClaimsRefusal(t, "RejectCharacter(an active character)", tb.master.rejectOf(tb.campaign, tb.caioPC.GetId()),
		`failed_precondition "the character does not wait for approval: once approved, it stays in the campaign" blocked=NOT_PENDING id=<Ícaro>`)

	// Approving an active character changes nothing, calls nobody, and still logs.
	tb.faults.called()
	mark := tb.mark()
	if got, err := tb.master.approveOf(tb.campaign, tb.caioPC.GetId()); err != nil || got.GetState() != charactersv1.CharacterState_CHARACTER_STATE_DRAFT {
		t.Errorf("ApproveCharacter(active) = %v, %v; want the draft as it is", got.GetState(), err)
	}
	tb.wantClaimsLog(t, "ApproveCharacter(active)", mark, "DEBUG event character_id=<Ícaro> event=character.approved")
	if got := tb.faults.called(); !slices.Equal(got, []string{"ContentFor", "DisplayNames"}) {
		t.Errorf("ApproveCharacter(active) called %v, want only the content and the names", got)
	}

	// The content is read before anything else.
	tb.faults.set("ContentFor", errors.New("claims-fault-content"))
	mark = tb.mark()
	_, err = tb.master.approveOf(tb.campaign, uuid.New().String())
	tb.wantClaimsRefusal(t, "ApproveCharacter(content fails)", err, claimsWantInternal)
	tb.wantClaimsLog(t, "ApproveCharacter(content fails)", mark, "ERROR characters: cannot read rules content code=internal error=claims-fault-content")
	tb.faults.set("ContentFor", nil)

	// The membership cannot change: the character stays pending, with its review.
	if _, err := tb.master.askChanges(tb.campaign, tb.evaPC.GetId(), "Falta algo.", key()); err != nil {
		t.Fatalf("RequestCharacterChanges() error = %v", err)
	}
	tb.faults.set("ActivatePendingMember", errors.New("claims-fault-activate"))
	mark = tb.mark()
	_, err = tb.master.approveOf(tb.campaign, tb.evaPC.GetId())
	tb.wantClaimsRefusal(t, "ApproveCharacter(the membership fails)", err, claimsWantInternal)
	tb.wantClaimsLog(t, "ApproveCharacter(the membership fails)", mark, "ERROR characters: cannot approve a character code=internal error=claims-fault-activate")
	tb.faults.set("ActivatePendingMember", nil)
	tb.faults.set("DeletePendingMember", errors.New("claims-fault-delete"))
	mark = tb.mark()
	tb.wantClaimsRefusal(t, "RejectCharacter(the membership fails)", tb.master.rejectOf(tb.campaign, tb.evaPC.GetId()), claimsWantInternal)
	tb.wantClaimsLog(t, "RejectCharacter(the membership fails)", mark, "ERROR characters: cannot reject a character code=internal error=claims-fault-delete")
	tb.faults.set("DeletePendingMember", nil)
	if got := tb.master.get(t, tb.campaign, tb.evaPC.GetId()); got.GetState() != charactersv1.CharacterState_CHARACTER_STATE_PENDING ||
		tb.h.reviewRows(tb.campaign) != 1 || tb.h.memberStatus(tb.campaign, tb.eva.id) != "pending" {
		t.Errorf("after the faults: state %v, %d reviews, membership %q; want pending, 1, pending", got.GetState(), tb.h.reviewRows(tb.campaign), tb.h.memberStatus(tb.campaign, tb.eva.id))
	}

	// The answer cannot be read after the commit: the approval stays, logged.
	tb.faults.set("DisplayNames", errors.New("claims-fault-names"))
	mark = tb.mark()
	_, err = tb.master.approveOf(tb.campaign, tb.evaPC.GetId())
	tb.wantClaimsRefusal(t, "ApproveCharacter(names fail after the commit)", err, claimsWantInternal)
	tb.wantClaimsLog(t, "ApproveCharacter(names fail after the commit)", mark,
		"DEBUG event character_id=<Ondina> event=character.approved",
		"ERROR characters: cannot read a character code=internal error=read display names: claims-fault-names")
	tb.faults.set("DisplayNames", nil)
	if tb.h.memberStatus(tb.campaign, tb.eva.id) != "active" || tb.h.reviewRows(tb.campaign) != 0 {
		t.Errorf("after the approval: membership %q, %d reviews; want active and none", tb.h.memberStatus(tb.campaign, tb.eva.id), tb.h.reviewRows(tb.campaign))
	}

	// A pending character whose player deleted the account: approved or rejected with no
	// membership to settle (the faults are set and never reached).
	orphanOf := func(name string) *charactersv1.Character {
		t.Helper()
		u := tb.h.newUser(name)
		tb.h.joinPending(tb.master, tb.campaign, u)
		c := u.create(t, tb.campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, name, pensantusSheet())
		tb.label(c.GetId(), "<"+name+">")
		tb.h.deleteUser(u.id)
		return c
	}
	first, second := orphanOf("Primeira"), orphanOf("Segunda")
	tb.faults.set("ActivatePendingMember", errors.New("claims-fault-activate"))
	tb.faults.set("DeletePendingMember", errors.New("claims-fault-delete"))
	mark = tb.mark()
	if got, err := tb.master.approveOf(tb.campaign, first.GetId()); err != nil || got.GetState() != charactersv1.CharacterState_CHARACTER_STATE_DRAFT {
		t.Errorf("ApproveCharacter(no player) = %v, %v; want a draft", got.GetState(), err)
	}
	tb.wantClaimsLog(t, "ApproveCharacter(no player)", mark, "DEBUG event character_id=<Primeira> event=character.approved")
	// Finding: RejectCharacter, alone of the batch's writes, writes no log line.
	mark = tb.mark()
	tb.wantClaimsRefusal(t, "RejectCharacter(no player)", tb.master.rejectOf(tb.campaign, second.GetId()), "ok")
	tb.wantClaimsLog(t, "RejectCharacter(no player)", mark)
	tb.wantClaimsRefusal(t, "RejectCharacter(rejected already)", tb.master.rejectOf(tb.campaign, second.GetId()), claimsWantNoCharacter)
	if got := tb.named(strings.Join(tb.host.told(), ",")); got != "changes_requested:<Ondina>" {
		t.Errorf("the streams were told %s, want only the request for changes", got)
	}
}
