package characters

import (
	"context"
	"encoding/base64"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
)

// The guided level-up's calls, frozen before levelup.go is split into phases: every
// refusal with its code, message and detail, in the order the checks run, who may
// call, what each viewer reads, and what a level-up writes and says afterwards.

// levelUpErrWant is what a refusal of the guided level-up says: the Connect code,
// the message, and its one detail (nil: none).
type levelUpErrWant struct {
	code   connect.Code
	msg    string
	detail proto.Message
}

// wantLevelUpErr fails the test unless err is exactly want.
func wantLevelUpErr(t *testing.T, call string, err error, want levelUpErrWant) {
	t.Helper()
	ce, ok := errors.AsType[*connect.Error](err)
	if !ok {
		t.Errorf("%s error = %v, want %v %q", call, err, want.code, want.msg)
		return
	}
	if ce.Code() != want.code || ce.Message() != want.msg {
		t.Errorf("%s error = %v %q, want %v %q", call, ce.Code(), ce.Message(), want.code, want.msg)
	}
	var details []proto.Message
	for _, d := range ce.Details() {
		v, derr := d.Value()
		if derr != nil {
			t.Fatalf("%s: decode a detail: %v", call, derr)
		}
		details = append(details, v)
	}
	switch {
	case want.detail == nil && len(details) != 0:
		t.Errorf("%s details = %v, want none", call, details)
	case want.detail != nil && (len(details) != 1 || !proto.Equal(details[0], want.detail)):
		t.Errorf("%s details = %v, want [%v]", call, details, want.detail)
	}
}

// The refusals the calls share.
func levelUpNotFound() levelUpErrWant {
	return levelUpErrWant{code: connect.CodeNotFound, msg: "character not found"}
}

func levelUpCampaignNotFound() levelUpErrWant {
	return levelUpErrWant{code: connect.CodeNotFound, msg: "campaign not found"}
}

func levelUpInvalid(field, msg string) levelUpErrWant {
	return levelUpErrWant{code: connect.CodeInvalidArgument, msg: field + " " + msg, detail: &charactersv1.InvalidField{Field: field}}
}

func levelUpBlocked(reason charactersv1.CharacterBlockedReason, id, msg string) levelUpErrWant {
	return levelUpErrWant{code: connect.CodeFailedPrecondition, msg: msg, detail: &charactersv1.CharacterBlocked{Reason: reason, CharacterId: id}}
}

func levelUpRefused(r *charactersv1.LevelUpRefusal) levelUpErrWant {
	return levelUpErrWant{code: connect.CodeFailedPrecondition, msg: "the level up does not allow this choice: " + r.GetField(), detail: r}
}

func levelUpOnlyThePlayer() levelUpErrWant {
	return levelUpErrWant{code: connect.CodePermissionDenied, msg: "only the owning player levels up a character with the guided level-up"}
}

const (
	levelUpDeadMsg    = "the character is dead, so only the master edits its sheet"
	levelUpPendingMsg = "the character waits for your approval: approve or reject it instead"
	levelUpAverageMsg = "the table gives everybody the average hit points: there is no roll"
	// Finding: errBlocked has no message for CANNOT_LEVEL_UP, DICE_FORCED_PHYSICAL and
	// DICE_FORCED_IN_APP, so these refusals carry an empty message (the app reads the
	// detail's reason only). Frozen as it is.
	levelUpNoMsg = ""
)

// levelUpLogLines is a slog handler that keeps the events a test logs.
type levelUpLogLines struct {
	mu    sync.Mutex
	lines []string
}

func (*levelUpLogLines) Enabled(context.Context, slog.Level) bool { return true }

func (l *levelUpLogLines) Handle(_ context.Context, r slog.Record) error {
	line := r.Level.String() + " " + r.Message
	r.Attrs(func(a slog.Attr) bool {
		line += " " + a.Key + "=" + a.Value.String()
		return true
	})
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, line)
	return nil
}

func (l *levelUpLogLines) WithAttrs([]slog.Attr) slog.Handler { return l }
func (l *levelUpLogLines) WithGroup(string) slog.Handler      { return l }

// events is the lines that are the level-up's domain events (logging.Event).
func (l *levelUpLogLines) events() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, line := range l.lines {
		if strings.HasPrefix(line, "DEBUG event event=levelup.") {
			out = append(out, line)
		}
	}
	return out
}

// levelUpFaults makes the progression rule or the dice setting fail, as a database
// error behind them would.
type levelUpFaults struct {
	dice      *testDiceRules
	diceFails atomic.Bool
	ruleFails atomic.Bool
}

func (f *levelUpFaults) LevelUpDice(ctx context.Context, tx pgx.Tx, campaignID, userID string) (charactersv1.LevelUpDiceRule, error) {
	if f.diceFails.Load() {
		return 0, errors.New("the dice setting is gone")
	}
	return f.dice.LevelUpDice(ctx, tx, campaignID, userID)
}

func (f *levelUpFaults) LevelUpReason(ctx context.Context, tx pgx.Tx, campaignID, characterID string, level, xp, next int32) (charactersv1.LevelUpReason, error) {
	if f.ruleFails.Load() {
		return 0, errors.New("the progression rule is gone")
	}
	return xpLevelUps{}.LevelUpReason(ctx, tx, campaignID, characterID, level, xp, next)
}

// newLevelUpFrozenTable is newLevelUpTable whose service logs into the returned lines,
// and whose progression rule and dice setting can be made to fail.
func newLevelUpFrozenTable(t *testing.T, xp int32, faces ...int) (*levelUpTable, *levelUpLogLines, *levelUpFaults) {
	t.Helper()
	logs := &levelUpLogLines{}
	tb := &levelUpTable{dice: &testDiceRules{}, live: &liveSpy{}, hitDieFaces: &dice.Fixed{Faces: faces}}
	faults := &levelUpFaults{dice: tb.dice}
	tb.dice.rule.Store(int32(charactersv1.LevelUpDiceRule_LEVEL_UP_DICE_RULE_PLAYER_CHOOSES))
	tb.h = newHarnessWith(t, func(c *Config) { c.Dice, c.Roller, c.Logger = faults, tb.hitDieFaces, slog.New(logs) })
	tb.h.svc.SetLevelUps(faults)
	tb.h.svc.SetLive(tb.live)
	tb.master, tb.owner, tb.other = tb.h.newUser("Samuel"), tb.h.newUser("Dona"), tb.h.newUser("Outra")
	tb.campaign = tb.h.newCampaign(tb.master, "Mirathel", tb.owner, tb.other)
	sheet := pensantusSheet()
	sheet.GetFull().ExperiencePoints = xp
	tb.pensantusSheet = sheet
	tb.pc = tb.owner.create(t, tb.campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus", sheet)
	tb.h.lockSheets(tb.campaign)
	tb.pc = tb.owner.get(t, tb.campaign, tb.pc.GetId())
	return tb, logs, faults
}

// errorLines is the lines logged at ERROR, without the error's text.
func (l *levelUpLogLines) errorLines() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, line := range l.lines {
		if len(line) > len("ERROR ") && line[:len("ERROR ")] == "ERROR " {
			if i := strings.Index(line, " error="); i >= 0 {
				rest := line[i+len(" error="):]
				if j := strings.Index(rest, " code="); j >= 0 {
					line = line[:i] + rest[j:]
				}
			}
			out = append(out, line)
		}
	}
	return out
}

// rollWith calls RollLevelUpHitPoints with the given request fields.
func (tb *levelUpTable) rollWith(u *user, req *charactersv1.RollLevelUpHitPointsRequest) (*charactersv1.RollLevelUpHitPointsResponse, error) {
	if req.CampaignId == "" {
		req.CampaignId = tb.campaign
	}
	res, err := u.api.RollLevelUpHitPoints(tb.h.t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// levelUpWith calls LevelUpCharacter with the given request.
func (tb *levelUpTable) levelUpWith(u *user, req *charactersv1.LevelUpCharacterRequest) (*charactersv1.Character, error) {
	if req.CampaignId == "" {
		req.CampaignId = tb.campaign
	}
	res, err := u.api.LevelUpCharacter(tb.h.t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetCharacter(), nil
}

// optionsWith calls GetLevelUpOptions with the given request.
func (tb *levelUpTable) optionsWith(u *user, req *charactersv1.GetLevelUpOptionsRequest) (*charactersv1.LevelUpOptions, error) {
	if req.CampaignId == "" {
		req.CampaignId = tb.campaign
	}
	res, err := u.api.GetLevelUpOptions(tb.h.t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetOptions(), nil
}

// setLevelUpStatus writes a character's status straight in the database, for a state
// the calls cannot reach by themselves.
func (tb *levelUpTable) setLevelUpStatus(id, status string) {
	tb.h.t.Helper()
	if _, err := tb.h.pool.Exec(tb.h.t.Context(), `UPDATE characters SET status = $2 WHERE id = $1`, id, status); err != nil {
		tb.h.t.Fatalf("set the status: %v", err)
	}
}

// keptLevelUpRolls is the kept rolls of a character, as "class to_level die value".
func (tb *levelUpTable) keptLevelUpRolls(id string) []string {
	tb.h.t.Helper()
	rows, err := tb.h.pool.Query(tb.h.t.Context(), `SELECT class_key, to_level, die, value FROM character_level_up_rolls WHERE character_id = $1 ORDER BY to_level`, id)
	if err != nil {
		tb.h.t.Fatalf("read the kept rolls: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var class string
		var to, die, value int32
		if err := rows.Scan(&class, &to, &die, &value); err != nil {
			tb.h.t.Fatalf("scan a kept roll: %v", err)
		}
		out = append(out, class+" "+itoa32(to)+" d"+itoa32(die)+" "+itoa32(value))
	}
	return out
}

func itoa32(n int32) string { return deref32(&n) }

// TestGetLevelUpOptionsFrozenRefusals: every refusal of GetLevelUpOptions, as the
// caller gets it, in the order the checks run.
func TestGetLevelUpOptionsFrozenRefusals(t *testing.T) {
	t.Parallel()
	tb := newLevelUpTable(t, 2700)
	id := tb.pc.GetId()
	stranger := tb.h.newUser("Estranha")
	npc := tb.master.create(t, tb.campaign, charactersv1.CharacterKind_CHARACTER_KIND_BOSS, "Strahd", enemySheet())
	cannot := charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_CANNOT_LEVEL_UP
	tests := []struct {
		name string
		u    *user
		req  *charactersv1.GetLevelUpOptionsRequest
		want levelUpErrWant
	}{
		{
			"nobody", tb.h.anonymous(), &charactersv1.GetLevelUpOptionsRequest{CharacterId: id},
			levelUpErrWant{code: connect.CodeUnauthenticated, msg: "sign in to continue"},
		},
		{"a stranger", stranger, &charactersv1.GetLevelUpOptionsRequest{CharacterId: id}, levelUpCampaignNotFound()},
		{"a campaign that is not a UUID", tb.owner, &charactersv1.GetLevelUpOptionsRequest{CampaignId: "nope", CharacterId: id}, levelUpCampaignNotFound()},
		{"a character that is not a UUID", tb.owner, &charactersv1.GetLevelUpOptionsRequest{CharacterId: "nope"}, levelUpNotFound()},
		{"a character that does not exist", tb.owner, &charactersv1.GetLevelUpOptionsRequest{CharacterId: uuid.New().String()}, levelUpNotFound()},
		{"another player's character", tb.other, &charactersv1.GetLevelUpOptionsRequest{CharacterId: id}, levelUpNotFound()},
		// Visibility comes before the class: a class that is not one is still "not found".
		{"another player's character and a bad class", tb.other, &charactersv1.GetLevelUpOptionsRequest{CharacterId: id, ClassKey: "class:nope"}, levelUpNotFound()},
		{"an NPC", tb.master, &charactersv1.GetLevelUpOptionsRequest{CharacterId: npc.GetId()}, levelUpBlocked(cannot, npc.GetId(), levelUpNoMsg)},
		{"an NPC with a bad class", tb.master, &charactersv1.GetLevelUpOptionsRequest{CharacterId: npc.GetId(), ClassKey: "class:nope"}, levelUpBlocked(cannot, npc.GetId(), levelUpNoMsg)},
		{"a class that is not one", tb.owner, &charactersv1.GetLevelUpOptionsRequest{CharacterId: id, ClassKey: "class:nope"}, levelUpInvalid("class_key", "is not a class")},
		{"a subclass key as the class", tb.owner, &charactersv1.GetLevelUpOptionsRequest{CharacterId: id, ClassKey: "subclass:evocation"}, levelUpInvalid("class_key", "is not a class")},
		// Strength 12: a new barbarian level needs 13.
		{
			"a new class without its prerequisite", tb.owner, &charactersv1.GetLevelUpOptionsRequest{CharacterId: id, ClassKey: "class:barbarian"},
			levelUpRefused(&charactersv1.LevelUpRefusal{
				Field: "class_key", Reason: charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_MULTICLASS_PREREQUISITE,
				ClassKey: "class:barbarian", Ability: rulesv1.Ability_ABILITY_STRENGTH, Minimum: 13, Have: 12,
			}),
		},
	}
	for _, tt := range tests {
		_, err := tb.optionsWith(tt.u, tt.req)
		wantLevelUpErr(t, tt.name, err, tt.want)
	}

	// Not enough XP, for the owner and the master alike.
	low := newLevelUpTable(t, 900)
	for _, u := range []*user{low.owner, low.master} {
		_, err := low.options(u, low.pc)
		wantLevelUpErr(t, "900 XP", err, levelUpBlocked(cannot, low.pc.GetId(), levelUpNoMsg))
	}
	// Dead: the reason comes before the XP and before the class.
	dead := low.master.markDead(t, low.pc)
	for _, u := range []*user{low.owner, low.master} {
		_, err := low.optionsWith(u, &charactersv1.GetLevelUpOptionsRequest{CampaignId: low.campaign, CharacterId: dead.GetId(), ClassKey: "class:nope"})
		wantLevelUpErr(t, "dead", err, levelUpBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_CHARACTER_DEAD, dead.GetId(), levelUpDeadMsg))
	}
	// Pending (written straight in the database: an active member's character is never
	// pending by itself), for the master; the owner no longer sees it.
	tb.setLevelUpStatus(id, statusPending)
	_, err := tb.optionsWith(tb.master, &charactersv1.GetLevelUpOptionsRequest{CharacterId: id, ClassKey: "class:nope"})
	wantLevelUpErr(t, "pending", err, levelUpBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_AWAITING_APPROVAL, id, levelUpPendingMsg))
	_, err = tb.options(tb.owner, tb.pc)
	wantLevelUpErr(t, "pending, the owner", err, levelUpBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_AWAITING_APPROVAL, id, levelUpPendingMsg))
	tb.setLevelUpStatus(id, statusActive)

	// A sheet that fails today's rules is the master's, before the class is looked at.
	if _, err := tb.h.pool.Exec(t.Context(), `UPDATE characters SET sheet = jsonb_set(sheet, '{full,race_key}', '"race:gone"') WHERE id = $1`, id); err != nil {
		t.Fatalf("break the sheet: %v", err)
	}
	for _, u := range []*user{tb.owner, tb.master} {
		_, err = tb.optionsWith(u, &charactersv1.GetLevelUpOptionsRequest{CharacterId: id, ClassKey: "class:nope"})
		wantLevelUpErr(t, "a broken sheet", err, levelUpRefused(&charactersv1.LevelUpRefusal{
			Field: "full.race_key", Reason: charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_SHEET_NEEDS_MASTER,
		}))
	}
}

// TestGuidedLevelUpWithoutTheProgressionRule: a service without the progression
// module's rule (SetLevelUps never called) answers CANNOT_LEVEL_UP to every call.
func TestGuidedLevelUpWithoutTheProgressionRule(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, owner := h.newUser("Samuel"), h.newUser("Dona")
	campaign := h.newCampaign(master, "Mirathel", owner)
	sheet := pensantusSheet()
	sheet.GetFull().ExperiencePoints = 2700
	pc := owner.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus", sheet)
	want := levelUpBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_CANNOT_LEVEL_UP, pc.GetId(), levelUpNoMsg)
	_, err := owner.api.GetLevelUpOptions(t.Context(), connect.NewRequest(&charactersv1.GetLevelUpOptionsRequest{CampaignId: campaign, CharacterId: pc.GetId()}))
	wantLevelUpErr(t, "GetLevelUpOptions", err, want)
	_, err = owner.api.PreviewLevelUp(t.Context(), connect.NewRequest(&charactersv1.PreviewLevelUpRequest{CampaignId: campaign, CharacterId: pc.GetId(), Choices: pensantusLevelUp()}))
	wantLevelUpErr(t, "PreviewLevelUp", err, want)
	_, err = owner.api.RollLevelUpHitPoints(t.Context(), connect.NewRequest(&charactersv1.RollLevelUpHitPointsRequest{CampaignId: campaign, CharacterId: pc.GetId(), IdempotencyKey: uuid.New().String()}))
	wantLevelUpErr(t, "RollLevelUpHitPoints", err, want)
	_, err = owner.api.LevelUpCharacter(t.Context(), connect.NewRequest(&charactersv1.LevelUpCharacterRequest{CampaignId: campaign, CharacterId: pc.GetId(), Revision: pc.GetRevision(), Choices: pensantusLevelUp()}))
	wantLevelUpErr(t, "LevelUpCharacter", err, want)
}

// TestGetLevelUpOptionsFrozenGolden: what the owner and the master read for
// Pensantus's level 4, with and without a kept roll, and for a new class.
func TestGetLevelUpOptionsFrozenGolden(t *testing.T) {
	t.Parallel()
	tb := newLevelUpTable(t, 2700, 5)
	labels := map[string]string{tb.pc.GetId(): "<pensantus>", tb.campaign: "<campaign>"}
	for _, v := range []struct {
		name string
		u    *user
	}{{"owner", tb.owner}, {"master", tb.master}} {
		o, err := tb.options(v.u, tb.pc)
		if err != nil {
			t.Fatalf("GetLevelUpOptions(%s) error = %v", v.name, err)
		}
		levelUpGolden(t, "options_pensantus_"+v.name, o, labels)
		f, err := tb.optionsFor(v.u, tb.pc, "class:fighter")
		if err != nil {
			t.Fatalf("GetLevelUpOptions(%s, fighter) error = %v", v.name, err)
		}
		levelUpGolden(t, "options_pensantus_fighter_"+v.name, f, labels)
	}
	if _, err := tb.roll(tb.owner, tb.pc); err != nil {
		t.Fatalf("RollLevelUpHitPoints() error = %v", err)
	}
	o, err := tb.options(tb.owner, tb.pc)
	if err != nil {
		t.Fatalf("GetLevelUpOptions(after the roll) error = %v", err)
	}
	if o.GetKeptHitPointRoll() != 5 || o.GetKeptHitPointRollClassKey() != "class:wizard" {
		t.Errorf("kept roll = %d %q, want 5 for the wizard", o.GetKeptHitPointRoll(), o.GetKeptHitPointRollClassKey())
	}
	// The kept roll is for the level, whatever class the options are asked for.
	f, err := tb.optionsFor(tb.owner, tb.pc, "class:fighter")
	if err != nil || f.GetKeptHitPointRoll() != 5 || f.GetKeptHitPointRollClassKey() != "class:wizard" {
		t.Errorf("fighter options after the wizard's roll = %d %q, %v; want the wizard's 5", f.GetKeptHitPointRoll(), f.GetKeptHitPointRollClassKey(), err)
	}
	// The dice setting and the table's rule are what the caller's table says.
	tb.dice.rule.Store(int32(charactersv1.LevelUpDiceRule_LEVEL_UP_DICE_RULE_FORCED_IN_APP))
	setRules(t, tb.master, tb.campaign, hitPointsRule(campaignsv1.HitPointsRule_HIT_POINTS_RULE_AVERAGE))
	o, err = tb.options(tb.owner, tb.pc)
	if err != nil || o.GetDiceRule() != charactersv1.LevelUpDiceRule_LEVEL_UP_DICE_RULE_FORCED_IN_APP ||
		o.GetHitPointsRule() != charactersv1.LevelUpHitPointsRule_LEVEL_UP_HIT_POINTS_RULE_AVERAGE_ONLY {
		t.Errorf("options = %v %v, %v; want FORCED_IN_APP and AVERAGE_ONLY", o.GetDiceRule(), o.GetHitPointsRule(), err)
	}
}

// TestRollLevelUpHitPointsFrozen: every refusal of the roll in order, the roll kept
// once per level, and what is written.
func TestRollLevelUpHitPointsFrozen(t *testing.T) {
	t.Parallel()
	tb := newLevelUpTable(t, 2700, 4, 6)
	id := tb.pc.GetId()
	key := uuid.New().String()
	cannot := charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_CANNOT_LEVEL_UP
	npc := tb.master.create(t, tb.campaign, charactersv1.CharacterKind_CHARACTER_KIND_BOSS, "Strahd", enemySheet())
	tests := []struct {
		name string
		u    *user
		req  *charactersv1.RollLevelUpHitPointsRequest
		want levelUpErrWant
	}{
		{
			"nobody", tb.h.anonymous(), &charactersv1.RollLevelUpHitPointsRequest{CharacterId: id, IdempotencyKey: key},
			levelUpErrWant{code: connect.CodeUnauthenticated, msg: "sign in to continue"},
		},
		{"a stranger", tb.h.newUser("Estranha"), &charactersv1.RollLevelUpHitPointsRequest{CharacterId: id, IdempotencyKey: key}, levelUpCampaignNotFound()},
		// The master is refused before anything about the character is read.
		{"the master", tb.master, &charactersv1.RollLevelUpHitPointsRequest{CharacterId: "nope", IdempotencyKey: "x"}, levelUpOnlyThePlayer()},
		{"a bad character before a bad key", tb.owner, &charactersv1.RollLevelUpHitPointsRequest{CharacterId: "nope", IdempotencyKey: "x"}, levelUpNotFound()},
		{"a bad key", tb.owner, &charactersv1.RollLevelUpHitPointsRequest{CharacterId: id, IdempotencyKey: "x"}, levelUpInvalid("idempotency_key", "must be a UUID")},
		{"no key", tb.owner, &charactersv1.RollLevelUpHitPointsRequest{CharacterId: id}, levelUpInvalid("idempotency_key", "must be a UUID")},
		{"a character that does not exist", tb.owner, &charactersv1.RollLevelUpHitPointsRequest{CharacterId: uuid.New().String(), IdempotencyKey: key}, levelUpNotFound()},
		{"another player's character", tb.other, &charactersv1.RollLevelUpHitPointsRequest{CharacterId: id, IdempotencyKey: key}, levelUpNotFound()},
		{"an NPC, to a player", tb.owner, &charactersv1.RollLevelUpHitPointsRequest{CharacterId: npc.GetId(), IdempotencyKey: key}, levelUpNotFound()},
		{"a class that is not one", tb.owner, &charactersv1.RollLevelUpHitPointsRequest{CharacterId: id, IdempotencyKey: key, ClassKey: "class:nope"}, levelUpInvalid("class_key", "is not a class")},
		{
			"a new class without its prerequisite", tb.owner, &charactersv1.RollLevelUpHitPointsRequest{CharacterId: id, IdempotencyKey: key, ClassKey: "class:barbarian"},
			levelUpRefused(&charactersv1.LevelUpRefusal{
				Field: "class_key", Reason: charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_MULTICLASS_PREREQUISITE,
				ClassKey: "class:barbarian", Ability: rulesv1.Ability_ABILITY_STRENGTH, Minimum: 13, Have: 12,
			}),
		},
	}
	for _, tt := range tests {
		_, err := tb.rollWith(tt.u, tt.req)
		wantLevelUpErr(t, tt.name, err, tt.want)
	}
	// The dice setting comes before the table's rule, and both before the class's
	// prerequisites are judged by the offer.
	tb.dice.rule.Store(int32(charactersv1.LevelUpDiceRule_LEVEL_UP_DICE_RULE_FORCED_PHYSICAL))
	setRules(t, tb.master, tb.campaign, hitPointsRule(campaignsv1.HitPointsRule_HIT_POINTS_RULE_AVERAGE))
	_, err := tb.rollWith(tb.owner, &charactersv1.RollLevelUpHitPointsRequest{CharacterId: id, IdempotencyKey: key})
	wantLevelUpErr(t, "physical dice and the average", err, levelUpBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_DICE_FORCED_PHYSICAL, id, levelUpNoMsg))
	tb.dice.rule.Store(int32(charactersv1.LevelUpDiceRule_LEVEL_UP_DICE_RULE_FORCED_IN_APP))
	_, err = tb.rollWith(tb.owner, &charactersv1.RollLevelUpHitPointsRequest{CharacterId: id, IdempotencyKey: key})
	wantLevelUpErr(t, "the average", err, levelUpBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_HIT_POINTS_AVERAGE_ONLY, id, levelUpAverageMsg))
	setRules(t, tb.master, tb.campaign, hitPointsRule(campaignsv1.HitPointsRule_HIT_POINTS_RULE_ROLL))
	if got := tb.keptLevelUpRolls(id); len(got) != 0 {
		t.Fatalf("kept rolls after the refusals = %v, want none", got)
	}

	// The roll: a d6 for the wizard's level 4, kept; the same answer afterwards, with
	// any key (Finding: the idempotency key is only checked as a UUID, never stored nor
	// compared: the kept roll is what makes a second call return the same).
	first, err := tb.rollWith(tb.owner, &charactersv1.RollLevelUpHitPointsRequest{CharacterId: id, IdempotencyKey: key})
	if err != nil || !proto.Equal(first, &charactersv1.RollLevelUpHitPointsResponse{Die: 6, Value: 4}) {
		t.Fatalf("first roll = %v, %v; want a d6 showing 4", first, err)
	}
	again, err := tb.rollWith(tb.owner, &charactersv1.RollLevelUpHitPointsRequest{CharacterId: id, IdempotencyKey: key, ClassKey: "class:wizard"})
	if err != nil || !proto.Equal(again, &charactersv1.RollLevelUpHitPointsResponse{Die: 6, Value: 4, AlreadyRolled: true}) {
		t.Errorf("the same key again = %v, %v; want the kept 4", again, err)
	}
	if got := tb.keptLevelUpRolls(id); !slices.Equal(got, []string{"class:wizard 4 d6 4"}) {
		t.Errorf("kept rolls = %v", got)
	}
	// Another class for the same level: the roll is the wizard's.
	_, err = tb.rollWith(tb.owner, &charactersv1.RollLevelUpHitPointsRequest{CharacterId: id, IdempotencyKey: uuid.New().String(), ClassKey: "class:fighter"})
	wantLevelUpErr(t, "the fighter after the wizard", err, levelUpRefused(&charactersv1.LevelUpRefusal{
		Field: "class_key", Reason: charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_HIT_POINT_ROLL_OTHER_CLASS,
	}))

	// Dead: before the dice setting.
	tb.dice.rule.Store(int32(charactersv1.LevelUpDiceRule_LEVEL_UP_DICE_RULE_FORCED_PHYSICAL))
	dead := tb.master.markDead(t, tb.pc)
	_, err = tb.rollWith(tb.owner, &charactersv1.RollLevelUpHitPointsRequest{CharacterId: dead.GetId(), IdempotencyKey: key, ClassKey: "class:nope"})
	wantLevelUpErr(t, "dead", err, levelUpBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_CHARACTER_DEAD, id, levelUpDeadMsg))
	tb.setLevelUpStatus(id, statusPending)
	_, err = tb.rollWith(tb.owner, &charactersv1.RollLevelUpHitPointsRequest{CharacterId: id, IdempotencyKey: key})
	wantLevelUpErr(t, "pending", err, levelUpBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_AWAITING_APPROVAL, id, levelUpPendingMsg))

	// Not enough XP: cannot, before the dice setting.
	low := newLevelUpTable(t, 900)
	low.dice.rule.Store(int32(charactersv1.LevelUpDiceRule_LEVEL_UP_DICE_RULE_FORCED_PHYSICAL))
	_, err = low.roll(low.owner, low.pc)
	wantLevelUpErr(t, "900 XP", err, levelUpBlocked(cannot, low.pc.GetId(), levelUpNoMsg))
}

// TestLevelUpCharacterFrozenRefusals: every refusal of LevelUpCharacter, in the order
// the checks run, and that none of them writes, logs or publishes.
func TestLevelUpCharacterFrozenRefusals(t *testing.T) {
	t.Parallel()
	tb, logs, _ := newLevelUpFrozenTable(t, 2700)
	id, rev := tb.pc.GetId(), tb.pc.GetRevision()
	good := pensantusLevelUp
	with := func(mod func(c *charactersv1.LevelUpChoices)) *charactersv1.LevelUpChoices {
		c := good()
		mod(c)
		return c
	}
	app := charactersv1.LevelUpHitPointsMethod_LEVEL_UP_HIT_POINTS_METHOD_ROLLED_IN_APP
	phys := charactersv1.LevelUpHitPointsMethod_LEVEL_UP_HIT_POINTS_METHOD_ROLLED_PHYSICAL
	tests := []struct {
		name string
		u    *user
		req  *charactersv1.LevelUpCharacterRequest
		want levelUpErrWant
	}{
		{
			"nobody", tb.h.anonymous(), &charactersv1.LevelUpCharacterRequest{CharacterId: id, Revision: rev, Choices: good()},
			levelUpErrWant{code: connect.CodeUnauthenticated, msg: "sign in to continue"},
		},
		{"a stranger", tb.h.newUser("Estranha"), &charactersv1.LevelUpCharacterRequest{CharacterId: id, Revision: rev, Choices: good()}, levelUpCampaignNotFound()},
		{"the master, with nothing right", tb.master, &charactersv1.LevelUpCharacterRequest{CharacterId: "nope"}, levelUpOnlyThePlayer()},
		{"a bad character before a bad revision", tb.owner, &charactersv1.LevelUpCharacterRequest{CharacterId: "nope"}, levelUpNotFound()},
		{"no revision, before the choices", tb.owner, &charactersv1.LevelUpCharacterRequest{CharacterId: id}, levelUpInvalid("revision", "must be at least 1")},
		{"a negative revision", tb.owner, &charactersv1.LevelUpCharacterRequest{CharacterId: id, Revision: -1, Choices: good()}, levelUpInvalid("revision", "must be at least 1")},
		{"no choices", tb.owner, &charactersv1.LevelUpCharacterRequest{CharacterId: id, Revision: rev}, levelUpInvalid("choices", "is required")},
		{"a list repeats, before the method", tb.owner, &charactersv1.LevelUpCharacterRequest{CharacterId: id, Revision: rev, Choices: with(func(c *charactersv1.LevelUpChoices) {
			c.CantripKeys, c.HitPoints = []string{"spell:light", "spell:light"}, nil
		})}, levelUpInvalid("choices.cantrip_keys", "repeats an entry")},
		{
			"no method", tb.owner, &charactersv1.LevelUpCharacterRequest{CharacterId: id, Revision: rev, Choices: with(func(c *charactersv1.LevelUpChoices) { c.HitPoints = nil })},
			levelUpInvalid("choices.hit_points.method", "is required"),
		},
		{"the unspecified method", tb.owner, &charactersv1.LevelUpCharacterRequest{CharacterId: id, Revision: rev, Choices: with(func(c *charactersv1.LevelUpChoices) {
			c.HitPoints = &charactersv1.LevelUpHitPoints{Value: 3}
		})}, levelUpInvalid("choices.hit_points.method", "is required")},
		{"an unknown method", tb.owner, &charactersv1.LevelUpCharacterRequest{CharacterId: id, Revision: rev, Choices: with(func(c *charactersv1.LevelUpChoices) {
			c.HitPoints = &charactersv1.LevelUpHitPoints{Method: 99}
		})}, levelUpInvalid("choices.hit_points.method", "is not a known method")},
		{"a character that does not exist", tb.owner, &charactersv1.LevelUpCharacterRequest{CharacterId: uuid.New().String(), Revision: rev, Choices: good()}, levelUpNotFound()},
		{"another player's character", tb.other, &charactersv1.LevelUpCharacterRequest{CharacterId: id, Revision: rev, Choices: good()}, levelUpNotFound()},
		// The revision comes before every check of the level.
		{
			"a stale revision with a bad class", tb.owner, &charactersv1.LevelUpCharacterRequest{CharacterId: id, Revision: rev + 1, Choices: with(func(c *charactersv1.LevelUpChoices) { c.ClassKey = "class:nope" })},
			levelUpErrWant{code: connect.CodeAborted, msg: "the character changed since you opened it; reload it and try again"},
		},
		{
			"a class that is not one", tb.owner, &charactersv1.LevelUpCharacterRequest{CharacterId: id, Revision: rev, Choices: with(func(c *charactersv1.LevelUpChoices) { c.ClassKey = "class:nope" })},
			levelUpInvalid("class_key", "is not a class"),
		},
		{
			"a new class without its prerequisite", tb.owner, &charactersv1.LevelUpCharacterRequest{CharacterId: id, Revision: rev, Choices: with(func(c *charactersv1.LevelUpChoices) { c.ClassKey = "class:barbarian" })},
			levelUpRefused(&charactersv1.LevelUpRefusal{
				Field: "class_key", Reason: charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_MULTICLASS_PREREQUISITE,
				ClassKey: "class:barbarian", Ability: rulesv1.Ability_ABILITY_STRENGTH, Minimum: 13, Have: 12,
			}),
		},
		{"a feat at a table without feats", tb.owner, &charactersv1.LevelUpCharacterRequest{CharacterId: id, Revision: rev, Choices: with(func(c *charactersv1.LevelUpChoices) {
			c.FeatKey, c.AbilityIncrease = "feat:grappler", nil
		})}, levelUpRefused(&charactersv1.LevelUpRefusal{Field: "choices.feat_key", Reason: charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_FEATS_NOT_ALLOWED})},
		// Finding: levelUpPlan.refusal says "the first rule the choices break", but a refusal
		// of the hit points replaces the feats one set before it: the last one wins.
		{"a feat and a die out of range", tb.owner, &charactersv1.LevelUpCharacterRequest{CharacterId: id, Revision: rev, Choices: with(func(c *charactersv1.LevelUpChoices) {
			c.FeatKey, c.AbilityIncrease = "feat:grappler", nil
			c.HitPoints = &charactersv1.LevelUpHitPoints{Method: phys, Value: 9}
		})}, levelUpRefused(&charactersv1.LevelUpRefusal{Field: "choices.hit_points.value", Reason: charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_HIT_POINTS})},
		{"a feat and no roll in the app", tb.owner, &charactersv1.LevelUpCharacterRequest{CharacterId: id, Revision: rev, Choices: with(func(c *charactersv1.LevelUpChoices) {
			c.FeatKey, c.AbilityIncrease = "feat:grappler", nil
			c.HitPoints = &charactersv1.LevelUpHitPoints{Method: app}
		})}, levelUpRefused(&charactersv1.LevelUpRefusal{Field: "choices.hit_points", Reason: charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_HIT_POINT_ROLL_MISSING})},
		{"a die out of range", tb.owner, &charactersv1.LevelUpCharacterRequest{CharacterId: id, Revision: rev, Choices: with(func(c *charactersv1.LevelUpChoices) {
			c.HitPoints = &charactersv1.LevelUpHitPoints{Method: phys, Value: 7}
		})}, levelUpRefused(&charactersv1.LevelUpRefusal{Field: "choices.hit_points.value", Reason: charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_HIT_POINTS})},
		{"a die of 0", tb.owner, &charactersv1.LevelUpCharacterRequest{CharacterId: id, Revision: rev, Choices: with(func(c *charactersv1.LevelUpChoices) {
			c.HitPoints = &charactersv1.LevelUpHitPoints{Method: phys}
		})}, levelUpRefused(&charactersv1.LevelUpRefusal{Field: "choices.hit_points.value", Reason: charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_HIT_POINTS})},
		{"no roll in the app", tb.owner, &charactersv1.LevelUpCharacterRequest{CharacterId: id, Revision: rev, Choices: with(func(c *charactersv1.LevelUpChoices) {
			c.HitPoints = &charactersv1.LevelUpHitPoints{Method: app}
		})}, levelUpRefused(&charactersv1.LevelUpRefusal{Field: "choices.hit_points", Reason: charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_HIT_POINT_ROLL_MISSING})},
		// A refusal of the hit points stands before the rules' check of the rest.
		{"a die out of range and no cantrip", tb.owner, &charactersv1.LevelUpCharacterRequest{CharacterId: id, Revision: rev, Choices: with(func(c *charactersv1.LevelUpChoices) {
			c.HitPoints = &charactersv1.LevelUpHitPoints{Method: phys, Value: 7}
			c.CantripKeys = nil
		})}, levelUpRefused(&charactersv1.LevelUpRefusal{Field: "choices.hit_points.value", Reason: charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_HIT_POINTS})},
		{
			"no cantrip", tb.owner, &charactersv1.LevelUpCharacterRequest{CharacterId: id, Revision: rev, Choices: with(func(c *charactersv1.LevelUpChoices) { c.CantripKeys = nil })},
			levelUpRefused(&charactersv1.LevelUpRefusal{Field: "full.cantrip_keys", Reason: charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_CANTRIPS}),
		},
		{"a 3rd-circle spell", tb.owner, &charactersv1.LevelUpCharacterRequest{CharacterId: id, Revision: rev, Choices: with(func(c *charactersv1.LevelUpChoices) {
			c.KnownSpellKeys, c.PreparedSpellKeys = []string{"spell:misty-step", "spell:fireball"}, nil
		})}, levelUpRefused(&charactersv1.LevelUpRefusal{Field: "full.known_spell_keys[11]", Reason: charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_SHEET_ISSUE, IssueCode: "spell_level"})},
		// An unknown key is the sheet's check, before the rules of the level.
		{"an unknown cantrip", tb.owner, &charactersv1.LevelUpCharacterRequest{CharacterId: id, Revision: rev, Choices: with(func(c *charactersv1.LevelUpChoices) {
			c.CantripKeys = []string{"spell:not-a-spell"}
			c.HitPoints = &charactersv1.LevelUpHitPoints{Method: phys, Value: 7}
		})}, levelUpErrWant{code: connect.CodeInvalidArgument}},
	}
	for _, tt := range tests {
		_, err := tb.levelUpWith(tt.u, tt.req)
		if tt.want.code == connect.CodeInvalidArgument && tt.want.msg == "" {
			// The message is the rules' own (frozen in package rules): the code and the field are this call's.
			wantCode(t, tt.name, err, connect.CodeInvalidArgument)
			continue
		}
		wantLevelUpErr(t, tt.name, err, tt.want)
	}
	// The dice setting: a blocked error, not a refusal, and it comes before the kept roll.
	tb.dice.rule.Store(int32(charactersv1.LevelUpDiceRule_LEVEL_UP_DICE_RULE_FORCED_PHYSICAL))
	_, err := tb.levelUp(tb.owner, tb.pc, with(func(c *charactersv1.LevelUpChoices) { c.HitPoints = &charactersv1.LevelUpHitPoints{Method: app} }))
	wantLevelUpErr(t, "in the app, physical dice", err, levelUpBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_DICE_FORCED_PHYSICAL, id, levelUpNoMsg))
	tb.dice.rule.Store(int32(charactersv1.LevelUpDiceRule_LEVEL_UP_DICE_RULE_FORCED_IN_APP))
	_, err = tb.levelUp(tb.owner, tb.pc, with(func(c *charactersv1.LevelUpChoices) {
		c.HitPoints = &charactersv1.LevelUpHitPoints{Method: phys, Value: 3}
	}))
	wantLevelUpErr(t, "typed, in-app dice", err, levelUpBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_DICE_FORCED_IN_APP, id, levelUpNoMsg))
	// The feats refusal does not stop the dice setting's error.
	_, err = tb.levelUp(tb.owner, tb.pc, with(func(c *charactersv1.LevelUpChoices) {
		c.FeatKey, c.AbilityIncrease = "feat:grappler", nil
		c.HitPoints = &charactersv1.LevelUpHitPoints{Method: phys, Value: 3}
	}))
	wantLevelUpErr(t, "a feat, typed, in-app dice", err, levelUpBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_DICE_FORCED_IN_APP, id, levelUpNoMsg))
	tb.dice.rule.Store(int32(charactersv1.LevelUpDiceRule_LEVEL_UP_DICE_RULE_PLAYER_CHOOSES))
	// The table's rule refuses the method it does not allow; the average stands in, so
	// the dice setting is never read.
	setRules(t, tb.master, tb.campaign, hitPointsRule(campaignsv1.HitPointsRule_HIT_POINTS_RULE_AVERAGE))
	tb.dice.rule.Store(int32(charactersv1.LevelUpDiceRule_LEVEL_UP_DICE_RULE_FORCED_IN_APP))
	_, err = tb.levelUp(tb.owner, tb.pc, with(func(c *charactersv1.LevelUpChoices) {
		c.HitPoints = &charactersv1.LevelUpHitPoints{Method: phys, Value: 9}
	}))
	wantLevelUpErr(t, "typed at an average table", err, levelUpRefused(&charactersv1.LevelUpRefusal{
		Field: "choices.hit_points.method", Reason: charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_HIT_POINTS_RULE,
	}))
	tb.dice.rule.Store(int32(charactersv1.LevelUpDiceRule_LEVEL_UP_DICE_RULE_PLAYER_CHOOSES))
	setRules(t, tb.master, tb.campaign, hitPointsRule(campaignsv1.HitPointsRule_HIT_POINTS_RULE_ROLL))
	_, err = tb.levelUp(tb.owner, tb.pc, good())
	wantLevelUpErr(t, "the average at a rolling table", err, levelUpRefused(&charactersv1.LevelUpRefusal{
		Field: "choices.hit_points.method", Reason: charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_HIT_POINTS_RULE,
	}))
	// The table's rule replaces the feats refusal too (the finding above).
	_, err = tb.levelUp(tb.owner, tb.pc, with(func(c *charactersv1.LevelUpChoices) { c.FeatKey, c.AbilityIncrease = "feat:grappler", nil }))
	wantLevelUpErr(t, "a feat and the average at a rolling table", err, levelUpRefused(&charactersv1.LevelUpRefusal{
		Field: "choices.hit_points.method", Reason: charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_HIT_POINTS_RULE,
	}))
	setRules(t, tb.master, tb.campaign, hitPointsRule(campaignsv1.HitPointsRule_HIT_POINTS_RULE_PLAYER_CHOOSES))

	// Dead and pending come before the revision.
	tb.setLevelUpStatus(id, statusPending)
	_, err = tb.levelUpWith(tb.owner, &charactersv1.LevelUpCharacterRequest{CharacterId: id, Revision: rev + 5, Choices: good()})
	wantLevelUpErr(t, "pending, a stale revision", err, levelUpBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_AWAITING_APPROVAL, id, levelUpPendingMsg))
	tb.setLevelUpStatus(id, statusActive)

	if got := tb.owner.get(t, tb.campaign, id); !proto.Equal(got, tb.pc) {
		t.Errorf("a refused level-up changed the character:\n%v\nwant\n%v", got, tb.pc)
	}
	if got, err := tb.levelUps(tb.master, ""); err != nil || len(got) != 0 {
		t.Errorf("level-ups recorded by the refusals = %v, %v", got, err)
	}
	if tb.live.count() != 0 || len(tb.live.vitals) != 0 || len(logs.events()) != 0 {
		t.Errorf("a refusal published %d hints, %v vitals, logged %v", tb.live.count(), tb.live.vitals, logs.events())
	}

	dead := tb.master.markDead(t, tb.pc)
	_, err = tb.levelUpWith(tb.owner, &charactersv1.LevelUpCharacterRequest{CharacterId: id, Revision: dead.GetRevision() + 5, Choices: good()})
	wantLevelUpErr(t, "dead, a stale revision", err, levelUpBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_CHARACTER_DEAD, id, levelUpDeadMsg))

	// Not enough XP: the revision first, then cannot.
	low := newLevelUpTable(t, 900)
	_, err = low.levelUpWith(low.owner, &charactersv1.LevelUpCharacterRequest{CampaignId: low.campaign, CharacterId: low.pc.GetId(), Revision: low.pc.GetRevision() + 1, Choices: good()})
	wantLevelUpErr(t, "900 XP, stale", err, levelUpErrWant{code: connect.CodeAborted, msg: "the character changed since you opened it; reload it and try again"})
	_, err = low.levelUp(low.owner, low.pc, good())
	wantLevelUpErr(t, "900 XP", err, levelUpBlocked(charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_CANNOT_LEVEL_UP, low.pc.GetId(), levelUpNoMsg))
}

// TestLevelUpCharacterFrozenWrites: what a level-up writes (the sheet, the record, the
// used roll gone), what it answers, and what it logs and publishes, once.
func TestLevelUpCharacterFrozenWrites(t *testing.T) {
	t.Parallel()
	tb, logs, _ := newLevelUpFrozenTable(t, 2700, 3)
	id := tb.pc.GetId()
	if _, err := tb.roll(tb.owner, tb.pc); err != nil {
		t.Fatalf("RollLevelUpHitPoints() error = %v", err)
	}
	ch := pensantusLevelUp()
	ch.HitPoints = &charactersv1.LevelUpHitPoints{Method: charactersv1.LevelUpHitPointsMethod_LEVEL_UP_HIT_POINTS_METHOD_ROLLED_IN_APP, Value: 999}
	ch.FeatureChoiceText = map[string]string{"feature:a": "  "}
	tb.h.clock.Advance(time.Second)
	up, err := tb.levelUp(tb.owner, tb.pc, ch)
	if err != nil {
		t.Fatalf("LevelUpCharacter() error = %v", err)
	}
	// The answer is the character as the owner reads it now.
	if got := tb.owner.get(t, tb.campaign, id); !proto.Equal(got, up) {
		t.Errorf("the answer differs from a read:\n%v\nwant\n%v", up, got)
	}
	// 3 on the d6 and Con +2... the stored rolls are the sheet's: [4 4 3].
	if hp := up.GetSheet().GetFull().GetHitPoints(); hp.GetMethod() != charactersv1.HitPointsMethod_HIT_POINTS_METHOD_ROLLED || !slices.Equal(hp.GetRolls(), []int32{4, 4, 3}) {
		t.Errorf("hit points = %v, want rolled [4 4 3]", hp)
	}
	if got := tb.keptLevelUpRolls(id); len(got) != 0 {
		t.Errorf("kept rolls after the level-up = %v, want none", got)
	}
	got, err := tb.levelUps(tb.master, id)
	if err != nil || len(got) != 1 {
		t.Fatalf("ListLevelUps() = %v, %v", got, err)
	}
	l := got[0]
	// The record: the choices as sent, with the class and the value the level took.
	want := pensantusLevelUp()
	want.HitPoints = &charactersv1.LevelUpHitPoints{Method: charactersv1.LevelUpHitPointsMethod_LEVEL_UP_HIT_POINTS_METHOD_ROLLED_IN_APP, Value: 3}
	// The blank text was dropped from the request before it was recorded.
	if !proto.Equal(l.GetChoices(), want) {
		t.Errorf("recorded choices = %v, want %v", l.GetChoices(), want)
	}
	if l.GetFromLevel() != 3 || l.GetToLevel() != 4 || l.GetClassKey() != "class:wizard" {
		t.Errorf("record = %v", l)
	}
	// The clock moves a microsecond each read: the sheet's time is read first, then
	// the vitals' (carryHitPoints), then the record's.
	if sheetAt, at := up.GetUpdatedAt().AsTime(), l.GetCreatedAt().AsTime(); !at.Equal(sheetAt.Add(2 * time.Microsecond)) {
		t.Errorf("record at %v, sheet at %v; want the record two ticks after the sheet", at, sheetAt)
	}
	if ev := logs.events(); !slices.Equal(ev, []string{"DEBUG event event=levelup.done character_id=" + id}) {
		t.Errorf("events = %q", ev)
	}
	if !slices.Equal(tb.live.calls, []string{tb.campaign}) || !slices.Equal(tb.live.vitals, []string{id}) {
		t.Errorf("live = %v, %v", tb.live.calls, tb.live.vitals)
	}
}

// TestListLevelUpsFrozen: who may list, every refusal in order, the names and the
// pages, and a stored record that cannot be read.
func TestListLevelUpsFrozen(t *testing.T) {
	t.Parallel()
	tb := newLevelUpTable(t, 2700)
	list := func(u *user, req *charactersv1.ListLevelUpsRequest) (*charactersv1.ListLevelUpsResponse, error) {
		if req.CampaignId == "" {
			req.CampaignId = tb.campaign
		}
		res, err := u.api.ListLevelUps(t.Context(), connect.NewRequest(req))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	tests := []struct {
		name string
		u    *user
		req  *charactersv1.ListLevelUpsRequest
		want levelUpErrWant
	}{
		{"nobody", tb.h.anonymous(), &charactersv1.ListLevelUpsRequest{}, levelUpErrWant{code: connect.CodeUnauthenticated, msg: "sign in to continue"}},
		{"a stranger", tb.h.newUser("Estranha"), &charactersv1.ListLevelUpsRequest{}, levelUpCampaignNotFound()},
		{"a player, with a bad page", tb.owner, &charactersv1.ListLevelUpsRequest{PageSize: -1}, levelUpErrWant{code: connect.CodePermissionDenied, msg: "only the campaign's master can do this"}},
		{"a negative page, before the token", tb.master, &charactersv1.ListLevelUpsRequest{PageSize: -1, PageToken: "nope"}, levelUpInvalid("page_size", "must be 1 to 50")},
		{"a page of 51", tb.master, &charactersv1.ListLevelUpsRequest{PageSize: maxLevelUpsListed + 1}, levelUpInvalid("page_size", "must be 1 to 50")},
		{"a bad token, before the character", tb.master, &charactersv1.ListLevelUpsRequest{PageToken: "nope", CharacterId: "nope"}, levelUpInvalid("page_token", "is not a token of this list")},
		{"a token that is not base64", tb.master, &charactersv1.ListLevelUpsRequest{PageToken: "!!"}, levelUpInvalid("page_token", "is not a token of this list")},
		{"a token without a colon", tb.master, &charactersv1.ListLevelUpsRequest{PageToken: "YWJj"}, levelUpInvalid("page_token", "is not a token of this list")},
		{"a token with a bad time", tb.master, &charactersv1.ListLevelUpsRequest{PageToken: levelUpsTokenRaw("x:" + uuid.New().String())}, levelUpInvalid("page_token", "is not a token of this list")},
		{"a token with a bad id", tb.master, &charactersv1.ListLevelUpsRequest{PageToken: levelUpsTokenRaw("12:nope")}, levelUpInvalid("page_token", "is not a token of this list")},
		{"a bad character", tb.master, &charactersv1.ListLevelUpsRequest{CharacterId: "nope"}, levelUpNotFound()},
	}
	for _, tt := range tests {
		_, err := list(tt.u, tt.req)
		wantLevelUpErr(t, tt.name, err, tt.want)
	}
	// A page of 50 is allowed; an empty list has no token.
	if res, err := list(tb.master, &charactersv1.ListLevelUpsRequest{PageSize: maxLevelUpsListed}); err != nil || len(res.GetLevelUps()) != 0 || res.GetNextPageToken() != "" {
		t.Errorf("an empty page of 50 = %v, %v", res, err)
	}

	// Two level-ups: one of Pensantus, one of an NPC the master leveled by hand (no player).
	npc := tb.master.create(t, tb.campaign, charactersv1.CharacterKind_CHARACTER_KIND_BOSS, "Strahd", enemySheet())
	insert := func(characterID, class, choices string) string {
		t.Helper()
		tb.h.clock.Advance(1000)
		var lid string
		if err := tb.h.pool.QueryRow(t.Context(), `
			INSERT INTO character_level_ups (campaign_id, character_id, class_key, from_level, to_level, hp_method, hp_value, choices, created_at)
			VALUES ($1, $2, $3, 3, 4, 'average', 4, $4, $5) RETURNING id`,
			tb.campaign, characterID, class, choices, tb.h.clock.Now()).Scan(&lid); err != nil {
			t.Fatalf("insert a level-up: %v", err)
		}
		return lid
	}
	pensLevel := insert(tb.pc.GetId(), "class:wizard", `{"cantripKeys":["spell:mage-hand"],"featKey":"feat:not-there","instrumentKey":"proficiency:lute","subclassKey":""}`)
	npcLevel := insert(npc.GetId(), "class:gone", `{}`)
	res, err := list(tb.master, &charactersv1.ListLevelUpsRequest{})
	if err != nil {
		t.Fatalf("ListLevelUps() error = %v", err)
	}
	// The times come from the clock, which starts now: newest first, then left out of the golden.
	if len(res.GetLevelUps()) != 2 || !res.GetLevelUps()[0].GetCreatedAt().AsTime().After(res.GetLevelUps()[1].GetCreatedAt().AsTime()) {
		t.Fatalf("list = %v, want two, newest first", res)
	}
	for _, l := range res.GetLevelUps() {
		l.CreatedAt = nil
	}
	levelUpGolden(t, "list_master", res, map[string]string{
		tb.pc.GetId(): "<pensantus>", npc.GetId(): "<strahd>", tb.campaign: "<campaign>", pensLevel: "<pensantus-level>", npcLevel: "<strahd-level>",
	})
	// The page size is exact: one page of one, then the last one with no token.
	first, err := list(tb.master, &charactersv1.ListLevelUpsRequest{PageSize: 1})
	if err != nil || len(first.GetLevelUps()) != 1 || first.GetNextPageToken() == "" {
		t.Fatalf("first page = %v, %v", first, err)
	}
	second, err := list(tb.master, &charactersv1.ListLevelUpsRequest{PageSize: 1, PageToken: first.GetNextPageToken()})
	if err != nil || len(second.GetLevelUps()) != 1 || second.GetNextPageToken() != "" || second.GetLevelUps()[0].GetCharacterId() != tb.pc.GetId() {
		t.Errorf("second page = %v, %v", second, err)
	}
	// A record that is not JSON: internal, with the fixed message.
	insert(tb.pc.GetId(), "class:wizard", `{"cantripKeys": 3}`)
	_, err = list(tb.master, &charactersv1.ListLevelUpsRequest{})
	wantLevelUpErr(t, "a broken record", err, levelUpErrWant{code: connect.CodeInternal, msg: "this character cannot be read right now"})
}

// levelUpsTokenRaw encodes a token as levelUpsToken does, from its raw text.
func levelUpsTokenRaw(raw string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}
