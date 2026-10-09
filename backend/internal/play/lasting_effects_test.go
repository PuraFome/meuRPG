package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Effects that last (RN-22): the clock, the order of the turn, the card and its visibility.
// The fixture is the close fight: Toren (18), Capitão Goblin (3), Pensantus (10) and Brisa
// (1), the goblin out of the way.

// addEffect has the master put the effect of the catalog on the labels.
func (a *armed) addEffect(t *testing.T, e *playv1.Encounter, key string, targets []string, edit func(*playv1.AddLastingEffectRequest)) (*playv1.AddLastingEffectResponse, error) {
	t.Helper()
	req := &playv1.AddLastingEffectRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey(), CatalogKey: key}
	for _, l := range targets {
		req.TargetIds = append(req.TargetIds, a.id(t, l))
	}
	if edit != nil {
		edit(req)
	}
	res, err := a.master.lasting.AddLastingEffect(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (a *armed) mustAddEffect(t *testing.T, e *playv1.Encounter, key string, targets []string, edit func(*playv1.AddLastingEffectRequest)) *playv1.AddLastingEffectResponse {
	t.Helper()
	res, err := a.addEffect(t, e, key, targets, edit)
	if err != nil {
		t.Fatalf("AddLastingEffect(%s) error = %v", key, err)
	}
	return res
}

// rounds is a duration of n rounds anchored to the label (the first target when empty).
func (a *armed) rounds(t *testing.T, n int32, anchor string) func(*playv1.AddLastingEffectRequest) {
	t.Helper()
	return func(r *playv1.AddLastingEffectRequest) {
		r.Duration = &playv1.EffectDurationChoice{Kind: playv1.EffectDurationKind_EFFECT_DURATION_KIND_ROUNDS, Rounds: n}
		if anchor != "" {
			r.Duration.AnchorCombatantId = a.id(t, anchor)
		}
	}
}

// listEffects is the master's list of the effects in play.
func (a *armed) listEffects(t *testing.T, e *playv1.Encounter) *playv1.ListLastingEffectsResponse {
	t.Helper()
	res, err := a.master.lasting.ListLastingEffects(t.Context(), connect.NewRequest(&playv1.ListLastingEffectsRequest{CampaignId: a.campaignID, EncounterId: e.GetId()}))
	if err != nil {
		t.Fatalf("ListLastingEffects() error = %v", err)
	}
	return res.Msg
}

func cardOf(t *testing.T, c *playv1.Combatant, key string) *playv1.LastingEffect {
	t.Helper()
	for _, f := range c.GetEffects() {
		if f.GetSourceKey() == key {
			return f
		}
	}
	return nil
}

// An effect with a duration of rounds ends at the start of its anchor's turn that many rounds
// later, and the card counts the rounds left (SRD 5.1, Duration: 1 minute is 10 rounds; the
// effect ends when the caster's turn comes round again).
func TestAnEffectWithARoundsDurationCountsDownAndEndsAtTheStartOfTheNamedTurn(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	res := a.mustAddEffect(t, e, "spell:bless", []string{"Toren"}, a.rounds(t, 2, "Toren"))
	if len(res.GetEffects()) != 1 {
		t.Fatalf("the effects = %v, want one", res.GetEffects())
	}
	f := cardOf(t, byLabel(t, res.GetEncounter(), "Toren"), "spell:bless")
	if f == nil || f.GetRoundsLeft() != 2 || !strings.Contains(f.GetClockTextPt(), "Restam 2 rodadas") {
		t.Fatalf("the card = %v, want 2 rounds left", f)
	}
	// Round 2, Toren's turn: one round is left.
	e = a.turnOf(t, "Toren")
	if f := cardOf(t, byLabel(t, a.get(t, a.master), "Toren"), "spell:bless"); f == nil || f.GetRoundsLeft() != 1 {
		t.Fatalf("round 2: the card = %v, want 1 round left", f)
	}
	// Round 3, the start of Toren's turn: it ended before he could use it.
	e = a.turnOf(t, "Toren")
	if f := cardOf(t, byLabel(t, e, "Toren"), "spell:bless"); f != nil {
		t.Errorf("round 3: the effect is still there: %v", f)
	}
}

// The same request twice is one effect.
func TestAddingAnEffectTwiceWithTheSameKeyMakesOne(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	req := &playv1.AddLastingEffectRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey(), CatalogKey: "spell:bless", TargetIds: []string{a.id(t, "Toren")},
		Duration: &playv1.EffectDurationChoice{Kind: playv1.EffectDurationKind_EFFECT_DURATION_KIND_ROUNDS, Rounds: 5},
	}
	for range 2 {
		if _, err := a.master.lasting.AddLastingEffect(t.Context(), connect.NewRequest(req)); err != nil {
			t.Fatalf("AddLastingEffect() error = %v", err)
		}
	}
	if got := len(byLabel(t, a.get(t, a.master), "Toren").GetEffects()); got != 1 {
		t.Errorf("Toren has %d effects, want 1", got)
	}
}

// The same spell recast does not stack (SRD 5.1, Combining Magical Effects).
func TestTheSameSpellOnATargetReplacesTheFirst(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	a.mustAddEffect(t, e, "spell:bless", []string{"Toren"}, a.rounds(t, 5, "Toren"))
	a.mustAddEffect(t, e, "spell:bless", []string{"Toren"}, a.rounds(t, 9, "Toren"))
	got := byLabel(t, a.get(t, a.master), "Toren").GetEffects()
	if len(got) != 1 || got[0].GetRoundsLeft() != 9 {
		t.Errorf("Toren's effects = %v, want one with 9 rounds", got)
	}
}

// The master changes the duration and ends the effect.
func TestTheMasterChangesTheDurationAndEndsAnEffect(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	res := a.mustAddEffect(t, e, "spell:bless", []string{"Toren"}, a.rounds(t, 3, "Toren"))
	id := res.GetEffects()[0].GetId()
	changed, err := a.master.lasting.ChangeLastingEffectDuration(t.Context(), connect.NewRequest(&playv1.ChangeLastingEffectDurationRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey(), EffectId: id,
		Duration: &playv1.EffectDurationChoice{Kind: playv1.EffectDurationKind_EFFECT_DURATION_KIND_ROUNDS, Rounds: 8, AnchorCombatantId: a.id(t, "Toren")},
	}))
	if err != nil {
		t.Fatalf("ChangeLastingEffectDuration() error = %v", err)
	}
	if f := cardOf(t, byLabel(t, changed.Msg.GetEncounter(), "Toren"), "spell:bless"); f == nil || f.GetRoundsLeft() != 8 {
		t.Errorf("after the change the card = %v, want 8 rounds", f)
	}
	ended, err := a.master.lasting.EndLastingEffect(t.Context(), connect.NewRequest(&playv1.EndLastingEffectRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey(), EffectId: id, Scope: playv1.EffectEndScope_EFFECT_END_SCOPE_THIS,
	}))
	if err != nil {
		t.Fatalf("EndLastingEffect() error = %v", err)
	}
	if f := cardOf(t, byLabel(t, ended.Msg.GetEncounter(), "Toren"), "spell:bless"); f != nil {
		t.Errorf("after the end the effect is still there: %v", f)
	}
	// An effect that is gone is not found.
	if _, err := a.master.lasting.EndLastingEffect(t.Context(), connect.NewRequest(&playv1.EndLastingEffectRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey(), EffectId: id, Scope: playv1.EffectEndScope_EFFECT_END_SCOPE_THIS,
	})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("ending an effect that is gone: error = %v, want not_found", err)
	}
}

// A player may not touch the master's effects.
func TestAPlayerCannotAddOrEndAnEffect(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	_, err := a.caio.lasting.AddLastingEffect(t.Context(), connect.NewRequest(&playv1.AddLastingEffectRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey(), CatalogKey: "spell:bless", TargetIds: []string{a.id(t, "Toren")},
	}))
	wantCode(t, "a player adds an effect", err, connect.CodePermissionDenied)
	_, err = a.caio.lasting.ListLastingEffects(t.Context(), connect.NewRequest(&playv1.ListLastingEffectsRequest{CampaignId: a.campaignID, EncounterId: e.GetId()}))
	wantCode(t, "a player lists the effects", err, connect.CodePermissionDenied)
}

// rollEffectSave calls RollEffectSave as u.
func (a *armed) rollEffectSave(t *testing.T, u *user, e *playv1.Encounter, windowID string, set func(*playv1.RollEffectSaveRequest)) (*playv1.RollEffectSaveResponse, error) {
	t.Helper()
	req := &playv1.RollEffectSaveRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), WindowId: windowID, IdempotencyKey: newKey()}
	set(req)
	res, err := u.lasting.RollEffectSave(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func effectSaveFace(face int32) func(*playv1.RollEffectSaveRequest) {
	return func(r *playv1.RollEffectSaveRequest) { r.Roll = &playv1.RollEffectSaveRequest_D20Face{D20Face: face} }
}

// holdTheGoblin has Pensantus cast Hold Person on the Goblin and the Goblin fail the save.
func (a *armed) holdTheGoblin(t *testing.T) *playv1.Encounter {
	t.Helper()
	e := a.closeFight(t)
	e = a.passTo(t, e, "Pensantus")
	a.h.roller.queue(1)
	res, err := a.cast(t, a.ana, e, "Pensantus", holdPerson, slotOfLevel(2), a.at(t, "Goblin"), noCastRoll)
	if err != nil {
		t.Fatalf("CastSpell(Hold Person) error = %v", err)
	}
	_ = res
	return a.get(t, a.master)
}

// Hold Person (SRD 5.1): a humanoid that fails the Wisdom save is paralyzed for a minute, and
// repeats the save at the end of each of its turns, ending the spell on itself on a success.
// The save waits on the turn: the turn does not pass until it is answered.
func TestHoldPersonParalyzesAndTheEndOfTurnSaveFreesTheTarget(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.holdTheGoblin(t)
	goblin := byLabel(t, e, "Goblin")
	f := cardOf(t, goblin, holdPerson)
	if f == nil {
		t.Fatalf("the Goblin has no Hold Person effect: %v", goblin)
	}
	if !strings.Contains(strings.Join(goblin.GetConditions(), ","), "condition:paralyzed") {
		t.Errorf("the Goblin's conditions = %v, want paralyzed", goblin.GetConditions())
	}
	if f.GetEndSave().GetAbility() != "wis" {
		t.Errorf("the end save = %v, want Wisdom", f.GetEndSave())
	}
	// The Goblin's turn: nothing to do, and the turn ends with the save.
	e = a.passTo(t, e, "Goblin")
	opts := a.mustOptions(t, a.master, e, "Goblin")
	if len(opts.GetOptions().GetAttacks()) > 0 && opts.GetOptions().GetAttacks()[0].GetEnabled() {
		t.Error("a paralyzed Goblin can attack")
	}
	if _, err := a.endTurnRaw(t, a.master, e, true); err != nil {
		t.Fatalf("EndTurn() error = %v", err)
	}
	w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE)
	if w == nil {
		t.Fatalf("the master's windows = %v, want the Goblin's save", a.windows(t, a.master))
	}
	if got := a.get(t, a.caio).GetReactionWait().GetTitlePt(); got != "Esperando o mestre" {
		t.Errorf("a player reads %q, want the master awaited", got)
	}
	if cur := a.get(t, a.master).GetCurrentCombatantId(); cur != "" && cur != a.id(t, "Goblin") {
		t.Errorf("the turn passed to %q before the save was answered", cur)
	}
	// A pass frees it and the turn goes on.
	res, err := a.rollEffectSave(t, a.master, e, w.GetId(), effectSaveFace(20))
	if err != nil {
		t.Fatalf("RollEffectSave() error = %v", err)
	}
	if !res.GetResult().GetSaved() {
		t.Errorf("the result = %v, want saved", res.GetResult())
	}
	after := a.get(t, a.master)
	if cardOf(t, byLabel(t, after, "Goblin"), holdPerson) != nil || strings.Contains(strings.Join(byLabel(t, after, "Goblin").GetConditions(), ","), "paralyzed") {
		t.Errorf("the Goblin is still held: %v", byLabel(t, after, "Goblin"))
	}
	if cur := after.GetCurrentCombatantId(); cur == "" || cur == a.id(t, "Goblin") {
		t.Errorf("the turn did not pass after the save: current = %q", cur)
	}
}

// A round is 6 seconds in combat and outside it (SRD 5.1, "The Order of Combat"): a partial
// round does not count as a full one.
func TestSecondsBecomeWholeRounds(t *testing.T) {
	t.Parallel()
	for seconds, want := range map[int32]int32{60: 10, 59: 9, 6: 1, 5: 0, 600: 100, 3600: 600} {
		if got := roundsOfSeconds(seconds); got != want {
			t.Errorf("roundsOfSeconds(%d) = %d, want %d", seconds, got, want)
		}
	}
}

// Bless cast a minute before the fight (no game time passed) enters it with 10 rounds; when
// the combat ends the rounds left go back to game time and the cast keeps running.
func TestAnOutsideCastEntersACombatInRoundsAndLeavesItInGameTime(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	cast := a.mustCastOut(t, a.bia, a.bri, blessKey, slotOfLevel(1), false, []*charactersv1.Character{a.toren}).GetCast()
	if cast.GetDurationSeconds() != 60 {
		t.Fatalf("Bless lasts %d seconds, want 60", cast.GetDurationSeconds())
	}
	e := a.closeFight(t)
	f := cardOf(t, byLabel(t, e, "Toren"), blessKey)
	if f == nil || f.GetRoundsLeft() != 10 {
		t.Fatalf("Toren's Bless = %v, want 10 rounds left", f)
	}
	// One round later the effect has 9 left; the combat ends and the cast has 54 seconds.
	e = a.turnOf(t, "Toren")
	if f := cardOf(t, byLabel(t, e, "Toren"), blessKey); f == nil || f.GetRoundsLeft() != 9 {
		t.Fatalf("round 2: Toren's Bless = %v, want 9 rounds left", f)
	}
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	c := a.castByID(t, a.master, cast.GetId())
	if c == nil || c.GetStatus() != playv1.OutsideCastStatus_OUTSIDE_CAST_STATUS_ACTIVE || c.GetDurationSeconds() != 54 {
		t.Errorf("after the combat the cast = %v, want active with 54 seconds", c)
	}
}

// advanceGameTime has the master move game time on by seconds.
func (a *armed) advanceGameTime(t *testing.T, seconds int32) *playv1.AdvanceGameTimeResponse {
	t.Helper()
	res, err := a.master.lasting.AdvanceGameTime(t.Context(), connect.NewRequest(&playv1.AdvanceGameTimeRequest{CampaignId: a.campaignID, IdempotencyKey: newKey(), Seconds: seconds}))
	if err != nil {
		t.Fatalf("AdvanceGameTime() error = %v", err)
	}
	return res.Msg
}

// The effect of a spell cast before a fight is the same record in it: Bless cast outside a
// combat a round before the fight (6 seconds of game time) enters with 9 rounds, and a saving
// throw inside the combat adds its d4 (SRD 5.1, Bless: 1 minute, 1d4 on attack rolls and
// saving throws).
func TestABuffCastBeforeTheFightEntersItWithTheTimeItHasLeft(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	a.mustCastOut(t, a.bia, a.bri, blessKey, slotOfLevel(1), false, []*charactersv1.Character{a.toren})
	a.advanceGameTime(t, 6)
	e := a.closeFight(t)
	f := cardOf(t, byLabel(t, e, "Toren"), blessKey)
	if f == nil || f.GetRoundsLeft() != 9 {
		t.Fatalf("Toren's Bless = %v, want 9 rounds left (54 seconds, a partial round does not count)", f)
	}
	// A saving throw in the combat takes the die.
	a.mustAddEffect(t, e, holdPerson, []string{"Toren"}, a.rounds(t, 10, "Toren"))
	a.passTo(t, e, "Toren")
	if _, err := a.endTurnRaw(t, a.master, e, true); err != nil {
		t.Fatalf("EndTurn() error = %v", err)
	}
	w := a.windowOf(t, a.caio, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE)
	if w == nil {
		t.Fatalf("Toren's windows = %v, want the save of Hold Person", a.windows(t, a.caio))
	}
	if got := w.GetEffectSave().GetExtraDice(); len(got) != 1 || got[0].GetFaces() != 4 || got[0].GetSign() != 1 {
		t.Errorf("the prompt's extra dice = %v, want Bless's d4", got)
	}
	a.h.roller.queue(10, 3)
	res, err := a.rollEffectSave(t, a.caio, e, w.GetId(), func(r *playv1.RollEffectSaveRequest) {
		r.Roll = &playv1.RollEffectSaveRequest_RollInApp{RollInApp: true}
	})
	if err != nil {
		t.Fatalf("RollEffectSave() error = %v", err)
	}
	if got := res.GetResult().GetExtraDice(); len(got) != 1 || got[0].GetFace() != 3 {
		t.Errorf("the result's extra dice = %v, want a d4 that rolled 3", got)
	}
	// The record is the same one all along: it goes back to game time with the rounds left.
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	var left int32
	if err := a.h.pool.QueryRow(t.Context(), `SELECT seconds_left FROM character_effects WHERE source_key = $1`, blessKey).Scan(&left); err != nil {
		t.Fatalf("the effect did not go back to the character: %v", err)
	}
	if left%6 != 0 || left > 54 || left < 6 {
		t.Errorf("seconds left after the combat = %d, want whole rounds of at most 54", left)
	}
}

// Outside a combat the effects count in the saving throws a scene asks: Bless adds its d4, and
// the sources of the roll say so.
func TestAnOutsideSavingThrowTakesTheDiceOfItsEffects(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	a.mustCastOut(t, a.bia, a.bri, blessKey, slotOfLevel(1), false, []*charactersv1.Character{a.toren})
	point, actions := a.h.newScene(a.mapID, "Ponte", true, 3, sceneSpec{key: "save:wis"})
	a.openScene(t, point)
	a.h.roller.queue(10, 4)
	res, err := a.caio.play.RollSceneCheck(t.Context(), connect.NewRequest(&playv1.RollSceneCheckRequest{
		CampaignId: a.campaignID, ActionId: actions[0], IdempotencyKey: newKey(), Roll: &playv1.RollSceneCheckRequest_RollInApp{RollInApp: true},
	}))
	if err != nil {
		t.Fatalf("RollSceneCheck() error = %v", err)
	}
	var die *playv1.AdvantageSource
	for _, src := range res.Msg.GetRoll().GetSources() {
		if src.GetKind() == playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_EFFECT_DIE {
			die = src
		}
	}
	if die == nil || !strings.Contains(die.GetTextPt(), "Bênção") || !strings.Contains(die.GetTextPt(), "1d4") {
		t.Errorf("the sources = %v, want Bênção +1d4", res.Msg.GetRoll().GetSources())
	}
}

// RN-10: an effect the master hides from the players is "Outra fonte" in the sources of a roll.
func TestAHiddenEffectIsAnotherSourceInAScenesRoll(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	a.execSQL(t, `INSERT INTO character_effects (campaign_id, character_id, group_id, source_key, source_kind, condition_keys, duration_kind, player_visible)
		VALUES ($1, $2, gen_random_uuid(), 'effect:LEAKCANARY-source-1', 'master', ARRAY['condition:poisoned'], 'until_dismissed', false)`, a.campaignID, a.toren.GetId())
	point, actions := a.h.newScene(a.mapID, "Ponte", true, 3, sceneSpec{key: "skill:perception"})
	a.openScene(t, point)
	a.h.roller.queue(10, 12)
	res, err := a.caio.play.RollSceneCheck(t.Context(), connect.NewRequest(&playv1.RollSceneCheckRequest{
		CampaignId: a.campaignID, ActionId: actions[0], IdempotencyKey: newKey(), Roll: &playv1.RollSceneCheckRequest_RollInApp{RollInApp: true},
	}))
	if err != nil {
		t.Fatalf("RollSceneCheck() error = %v", err)
	}
	roll := res.Msg.GetRoll()
	if roll.GetMode() == 0 && len(roll.GetSources()) == 0 {
		t.Fatalf("the hidden Poisoned changed nothing: %v", roll)
	}
	for _, src := range roll.GetSources() {
		if strings.Contains(src.GetTextPt(), "Envenenado") || src.GetKind() != playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_OTHER_SOURCE {
			t.Errorf("source %v: a hidden effect must read Outra fonte", src)
		}
	}
	if !strings.Contains(strings.Join(sourceTexts(roll.GetSources()), ","), "Outra fonte") {
		t.Errorf("the sources = %v, want Outra fonte", roll.GetSources())
	}
}

func sourceTexts(s []*playv1.AdvantageSource) []string {
	var out []string
	for _, x := range s {
		out = append(out, x.GetTextPt())
	}
	return out
}
