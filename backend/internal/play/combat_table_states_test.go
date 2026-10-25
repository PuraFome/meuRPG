package play

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The temporary states a table feature defines, in a combat, end to end: "a stance for 1 minute, bonus
// action, 1 use per short rest, +2 AC and +10 ft speed" and "a form that lasts until a long rest" (MR-025).
// Every name is invented. These tests need the database (MEURPG_TEST_DATABASE_URL).

const (
	stanceUses = "posturas"
	stanceName = "postura"
	formName   = "forma"
)

// stanceEffects is the stance: a resource, the state that spends it, and the effects the state gates.
func stanceEffects(duration string, ends ...string) []*rulesv1.TableEffect {
	return []*rulesv1.TableEffect{
		{Type: "resource", Resource: stanceUses, Max: "1", Recharge: "short_rest"},
		{Type: "state", StateName: stanceName, DurationRounds: duration, EndsOn: ends, Resource: stanceUses, Economy: "bonus_action", TextPt: "Postura firme."},
		{Type: "modifier", Target: "ac", Mode: "add", Value: "2", When: `active("postura")`},
		{Type: "modifier", Target: "speed.walk", Mode: "add", Value: "10", When: `active("postura")`},
		{Type: "resistance", Except: "damage-type:psychic", While: stanceName},
	}
}

// stanceRig is a fight with Toren, a fighter 5 whose subclass has the stance, next to the goblin.
type stanceRig struct {
	a       *armed
	e       *playv1.Encounter
	feature string // the key of the feature that defines the stance
}

// stateKey is the key of a state the feature defines.
func (r *stanceRig) stateKey(name string) string { return r.feature + "#" + name }

func newStanceRig(t *testing.T, effects []*rulesv1.TableEffect) *stanceRig {
	t.Helper()
	return newStanceRigOf(t, effects, abilities(16, 13, 14, 10, 8))
}

// newStanceRigOf is newStanceRig for a Toren with the scores.
func newStanceRigOf(t *testing.T, effects []*rulesv1.TableEffect, scores *rulesv1.AbilityScores) *stanceRig {
	t.Helper()
	r := &stanceRig{}
	r.a = newArmedWith(t, func(a *armed) {
		res, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(&rulesv1.CreateTableEntryRequest{
			CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableSubclass{TableSubclass: &rulesv1.TableSubclass{
				NamePt: "Guardião de Teste", ClassKey: "class:fighter", DescPt: []string{"Uma postura firme."},
				Levels: []*rulesv1.TableSubclassLevel{{Level: 3, Features: []*rulesv1.TableFeature{{NamePt: "Postura Firme", DescPt: []string{"Fique firme."}, Effects: effects}}}},
			}},
		}))
		if err != nil {
			t.Fatalf("CreateTableEntry() error = %v", err)
		}
		entry := res.Msg.GetEntry()
		r.feature = entry.GetTableSubclass().GetLevels()[0].GetFeatures()[0].GetKey()
		a.toren = a.caio.heroWith(t, a.campaignID, "Toren", classLevel("class:fighter", 5, entry.GetKey()),
			scores, []string{longsword}, nil, []string{"feature:fighter-fighting-style-defense"})
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, nil, []string{fireBolt})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{rapier}, nil)
	})
	r.e = r.a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: r.a.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Pensantus": {10, 3}, "Brisa": {8, 8}},
	})
	return r
}

// toggle takes the state's card as Toren's player: ending it when end is set.
func (r *stanceRig) toggle(t *testing.T, name string, end bool) (*playv1.TakeActionResponse, error) {
	t.Helper()
	return r.a.takeActionRaw(t, r.a.caio, r.a.get(t, r.a.master), "Toren", r.stateKey(name), func(q *playv1.TakeActionRequest) { q.EndState = end })
}

func (r *stanceRig) mustToggle(t *testing.T, name string, end bool) {
	t.Helper()
	if _, err := r.toggle(t, name, end); err != nil {
		t.Fatalf("TakeAction(%s, end %v) error = %v", name, end, err)
	}
}

// states are the keys of the states that are on for Toren, as the master reads them.
func (r *stanceRig) states(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, st := range r.a.vitals(t, r.a.toren).GetActiveStates() {
		out = append(out, st.GetKey())
	}
	return out
}

// derived is Toren's derived sheet as his player reads it.
func (r *stanceRig) derived(t *testing.T) *rulesv1.DerivedSheet {
	t.Helper()
	res, err := r.a.caio.characters.GetCharacter(t.Context(), connect.NewRequest(&charactersv1.GetCharacterRequest{CampaignId: r.a.campaignID, CharacterId: r.a.toren.GetId()}))
	if err != nil {
		t.Fatalf("GetCharacter() error = %v", err)
	}
	return res.Msg.GetCharacter().GetDerived()
}

// undoLast has the master take back the last action.
func (r *stanceRig) undoLast(t *testing.T) {
	t.Helper()
	e := r.a.get(t, r.a.master)
	if err := r.a.undo(t, r.a.master, e, r.a.log(t, r.a.master, e).GetUndoableEventId()); err != nil {
		t.Fatalf("UndoLastAction() error = %v", err)
	}
}

// goblinHits has the goblin attack Toren with the natural d20 face and returns whether it hit.
func (r *stanceRig) goblinHits(t *testing.T, face int32) bool {
	t.Helper()
	e := r.a.passTo(t, r.a.get(t, r.a.master), "Goblin")
	res := r.a.mustAttack(t, r.a.master, e, "Goblin", sword, "Toren", d20(face))
	if p := res.GetPendingDamage(); p.GetId() != "" {
		if _, err := r.a.settle(t, r.a.master, e, p.GetId(), false); err != nil {
			t.Fatalf("discarding the damage error = %v", err)
		}
	}
	return res.GetRoll().GetOutcome() == playv1.AttackOutcome_ATTACK_OUTCOME_HIT
}

func TestAStateStartsSpendsItsCostAndChangesTheSheet(t *testing.T) {
	t.Parallel()
	r := newStanceRig(t, stanceEffects("10", "dismiss", "incapacitated"))
	key := r.stateKey(stanceName)

	// Before: the card says Ativar, costs a bonus action and a use, and the numbers are the plain ones.
	opts := r.a.mustOptions(t, r.a.caio, r.e, "Toren")
	i := slices.IndexFunc(opts.GetOptions().GetFeatureActions(), func(o *rulesv1.ActionOption) bool { return o.GetAction().GetKey() == key })
	if i < 0 {
		t.Fatalf("no card for the state among %v", opts.GetOptions().GetFeatureActions())
	}
	card := opts.GetOptions().GetFeatureActions()[i]
	if a := card.GetAction(); a.GetStateKey() != key || a.GetStateActive() || a.GetEconomy() != rulesv1.ActionEconomy_ACTION_ECONOMY_BONUS_ACTION || a.GetResourceKey() != stanceUses || !card.GetEnabled() {
		t.Fatalf("card = %v, want an enabled bonus action that starts the state and spends %s", card, stanceUses)
	}
	plain := r.derived(t)
	if len(plain.GetStates()) != 1 || plain.GetStates()[0].GetActive() || plain.GetStates()[0].GetDurationRounds() != 10 {
		t.Fatalf("states = %v, want the stance, off, for 10 rounds", plain.GetStates())
	}

	r.mustToggle(t, stanceName, false)

	v := r.a.vitals(t, r.a.toren)
	if got := usedOf(v, stanceUses); got != 1 {
		t.Errorf("uses spent = %d, want 1", got)
	}
	if len(v.GetActiveStates()) != 1 || v.GetActiveStates()[0].GetKey() != key || v.GetActiveStates()[0].GetRoundsLeft() != 10 || v.GetActiveStates()[0].GetDurationRounds() != 10 {
		t.Errorf("active states = %v, want the stance with 10 rounds left", v.GetActiveStates())
	}
	if toren := byLabel(t, r.a.get(t, r.a.master), "Toren"); !toren.GetBonusActionUsed() {
		t.Error("the bonus action is not spent")
	}
	on := r.derived(t)
	if on.GetArmorClass() != plain.GetArmorClass()+2 || on.GetSpeedWalkFt() != plain.GetSpeedWalkFt()+10 {
		t.Errorf("AC %d and speed %d with the state, %d and %d without: want +2 and +10", on.GetArmorClass(), on.GetSpeedWalkFt(), plain.GetArmorClass(), plain.GetSpeedWalkFt())
	}
	// The combat's copy of the speed follows, and the armor class the goblin must beat is the sheet's.
	if got := byLabel(t, r.a.get(t, r.a.master), "Toren").GetSpeedFt(); got != on.GetSpeedWalkFt() {
		t.Errorf("Toren's combat speed = %d, want %d", got, on.GetSpeedWalkFt())
	}
	// The card now ends it.
	opts = r.a.mustOptions(t, r.a.caio, r.e, "Toren")
	for _, o := range opts.GetOptions().GetFeatureActions() {
		if o.GetAction().GetKey() == key && (!o.GetAction().GetStateActive() || o.GetAction().GetEconomy() != rulesv1.ActionEconomy_ACTION_ECONOMY_FREE || o.GetAction().GetResourceKey() != "") {
			t.Errorf("card while on = %v, want a free one that ends the state", o)
		}
	}
}

func TestTheSheetsArmorClassIsWhatTheGoblinMustBeat(t *testing.T) {
	t.Parallel()
	r := newStanceRig(t, stanceEffects("10", "dismiss"))
	plain := r.derived(t).GetArmorClass()
	r.mustToggle(t, stanceName, false)
	// The goblin attacks at +4: a total equal to the plain armor class hits it and misses the +2.
	face := plain - 4
	if r.goblinHits(t, face) {
		t.Errorf("the goblin hit with %d against the armor class of the stance (%d)", plain, plain+2)
	}
	r.a.passTo(t, r.a.get(t, r.a.master), "Toren")
	r.mustToggle(t, stanceName, true)
	if !r.goblinHits(t, face) {
		t.Errorf("the goblin missed with %d against the plain armor class (%d) after the stance ended", plain, plain)
	}
}

func TestAStateThatIsOnCannotStartAgainAndOneThatIsOffCannotEnd(t *testing.T) {
	t.Parallel()
	r := newStanceRig(t, stanceEffects("10", "dismiss"))

	_, err := r.toggle(t, stanceName, true)
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_STATE_NOT_ON)
	if v := r.a.vitals(t, r.a.toren); usedOf(v, stanceUses) != 0 || len(v.GetActiveStates()) != 0 {
		t.Errorf("a refused end changed the vitals: %v", v)
	}

	r.mustToggle(t, stanceName, false)
	_, err = r.toggle(t, stanceName, false)
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_STATE_ALREADY_ON)
	if got := usedOf(r.a.vitals(t, r.a.toren), stanceUses); got != 1 {
		t.Errorf("uses spent = %d after a refused second start, want 1", got)
	}
}

func TestEndingAStateIsFreeAndKeepsTheUse(t *testing.T) {
	t.Parallel()
	r := newStanceRig(t, stanceEffects("10", "dismiss"))
	r.mustToggle(t, stanceName, false)
	r.mustToggle(t, stanceName, true) // the bonus action is spent already: ending is free
	if got := r.states(t); len(got) != 0 {
		t.Errorf("states = %v, want none", got)
	}
	if got := usedOf(r.a.vitals(t, r.a.toren), stanceUses); got != 1 {
		t.Errorf("uses spent = %d, want 1: the use is not given back", got)
	}
	if got := byLabel(t, r.a.get(t, r.a.master), "Toren").GetSpeedFt(); got != r.derived(t).GetSpeedWalkFt() {
		t.Errorf("Toren's combat speed = %d, want the plain %d", got, r.derived(t).GetSpeedWalkFt())
	}
	// With no use left the stance cannot start again.
	opts := r.a.mustOptions(t, r.a.caio, r.e, "Toren")
	for _, o := range opts.GetOptions().GetFeatureActions() {
		if o.GetAction().GetKey() == r.stateKey(stanceName) && o.GetEnabled() {
			t.Errorf("the card is enabled with no use left: %v", o)
		}
	}
}

func TestAStateWithRoundsCountsThemDownOnItsOwnersTurns(t *testing.T) {
	t.Parallel()
	r := newStanceRig(t, stanceEffects("2", "dismiss"))
	plain := r.derived(t).GetArmorClass()
	r.mustToggle(t, stanceName, false)
	rounds := func() int32 {
		v := r.a.vitals(t, r.a.toren)
		if len(v.GetActiveStates()) == 0 {
			return -1
		}
		return v.GetActiveStates()[0].GetRoundsLeft()
	}
	if got := rounds(); got != 2 {
		t.Fatalf("rounds left = %d, want 2", got)
	}
	r.a.passTo(t, r.a.get(t, r.a.master), "Goblin")
	if got := rounds(); got != 2 {
		t.Errorf("rounds left = %d in the goblin's turn, want 2: only the owner's turns count", got)
	}
	r.a.passTo(t, r.a.get(t, r.a.master), "Toren") // the next round
	if got := rounds(); got != 1 {
		t.Errorf("rounds left = %d at the start of the second round, want 1", got)
	}
	r.a.passTo(t, r.a.get(t, r.a.master), "Goblin")
	r.a.passTo(t, r.a.get(t, r.a.master), "Toren")
	if got := rounds(); got != -1 {
		t.Errorf("the state is still on with %d rounds left: it should end when none is left", got)
	}
	if v := r.derived(t); v.GetArmorClass() != plain || len(v.GetStates()) != 1 || v.GetStates()[0].GetActive() {
		t.Errorf("the sheet after the end = AC %d, states %v; want the plain %d and the state off", v.GetArmorClass(), v.GetStates(), plain)
	}
	if !stateLine(t, r.a, r.a.caio, r.e, stateEndDuration) || !stateLine(t, r.a, r.a.ana, r.e, stateEndDuration) {
		t.Error("the log has no line for the end of the stance")
	}
}

// stateLine says whether the player reads a log line for a table state that ended for the reason.
func stateLine(t *testing.T, a *armed, u *user, e *playv1.Encounter, reason string) bool {
	t.Helper()
	for _, rd := range a.log(t, u, e).GetRounds() {
		for _, en := range rd.GetEntries() {
			if s := en.GetState(); s != nil && s.GetKind() == playv1.CombatantStateKind_COMBATANT_STATE_KIND_TABLE && !s.GetStarted() && s.GetReason() == reason && s.GetNamePt() == "Postura Firme" {
				return true
			}
		}
	}
	return false
}

func TestAStateWithNoDurationIsNotCountedAndTheCombatEndKeepsIt(t *testing.T) {
	t.Parallel()
	effects := []*rulesv1.TableEffect{
		{Type: "state", StateName: formName, EndsOn: []string{"long_rest", "dismiss"}, Economy: "free"},
		{Type: "modifier", Target: "initiative", Mode: "add", Value: "2", When: `active("forma")`},
	}
	r := newStanceRig(t, effects)
	r.mustToggle(t, formName, false)
	r.a.passTo(t, r.a.get(t, r.a.master), "Goblin")
	r.a.passTo(t, r.a.get(t, r.a.master), "Toren")
	v := r.a.vitals(t, r.a.toren)
	if len(v.GetActiveStates()) != 1 || v.GetActiveStates()[0].GetRoundsLeft() != 0 || v.GetActiveStates()[0].GetDurationRounds() != 0 {
		t.Fatalf("active states = %v, want the form, on, with no rounds", v.GetActiveStates())
	}
	if _, err := r.a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: r.a.campaignID, EncounterId: r.e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	if got := r.states(t); !slices.Equal(got, []string{r.stateKey(formName)}) {
		t.Errorf("states after the combat = %v, want the form to stay", got)
	}
}

func TestTheCombatEndEndsTheStatesThatCountRounds(t *testing.T) {
	t.Parallel()
	r := newStanceRig(t, stanceEffects("10", "dismiss"))
	r.mustToggle(t, stanceName, false)
	if _, err := r.a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: r.a.campaignID, EncounterId: r.e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	if got := r.states(t); len(got) != 0 {
		t.Errorf("states after the combat = %v, want none: the rounds are counted in a combat only", got)
	}
}

func TestAStateEndsWhenTheCharacterIsAtZeroHitPointsOrIncapacitated(t *testing.T) {
	t.Parallel()
	t.Run("hit points", func(t *testing.T) {
		t.Parallel()
		r := newStanceRig(t, stanceEffects("10", "incapacitated"))
		r.mustToggle(t, stanceName, false)
		r.a.hurt(t, r.a.toren, 0)
		if got := r.states(t); len(got) != 0 {
			t.Errorf("states at 0 hit points = %v, want none", got)
		}
	})
	t.Run("a condition", func(t *testing.T) {
		t.Parallel()
		r := newStanceRig(t, stanceEffects("10", "incapacitated"))
		r.mustToggle(t, stanceName, false)
		r.a.setConditions(t, r.e, "Toren", "condition:stunned")
		if got := r.states(t); len(got) != 0 {
			t.Errorf("states while stunned = %v, want none", got)
		}
	})
	t.Run("a state that does not say so stays", func(t *testing.T) {
		t.Parallel()
		r := newStanceRig(t, stanceEffects("10", "dismiss"))
		r.mustToggle(t, stanceName, false)
		r.a.hurt(t, r.a.toren, 0)
		r.a.setConditions(t, r.e, "Toren", "condition:stunned")
		if got := r.states(t); len(got) != 1 {
			t.Errorf("states = %v, want the stance to stay: it does not end on incapacity", got)
		}
	})
}

func TestARestEndsTheStatesThatSayIt(t *testing.T) {
	t.Parallel()
	effects := []*rulesv1.TableEffect{
		{Type: "state", StateName: "curta", EndsOn: []string{"short_rest"}, Economy: "free"},
		{Type: "state", StateName: formName, EndsOn: []string{"long_rest"}, Economy: "free"},
	}
	r := newStanceRig(t, effects)
	r.mustToggle(t, "curta", false)
	r.mustToggle(t, formName, false)
	if _, err := r.a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: r.a.campaignID, EncounterId: r.e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	rest := func(kind playv1.RestKind) {
		t.Helper()
		if _, err := r.a.master.resource.TakeRest(t.Context(), connect.NewRequest(&playv1.TakeRestRequest{CampaignId: r.a.campaignID, Kind: kind, IdempotencyKey: newKey()})); err != nil {
			t.Fatalf("TakeRest(%v) error = %v", kind, err)
		}
	}
	rest(playv1.RestKind_REST_KIND_SHORT)
	if got := r.states(t); !slices.Equal(got, []string{r.stateKey(formName)}) {
		t.Errorf("states after a short rest = %v, want only the form", got)
	}
	rest(playv1.RestKind_REST_KIND_LONG)
	if got := r.states(t); len(got) != 0 {
		t.Errorf("states after a long rest = %v, want none", got)
	}
}

func TestALongRestEndsAStateThatSaysShortRestToo(t *testing.T) {
	t.Parallel()
	r := newStanceRig(t, []*rulesv1.TableEffect{{Type: "state", StateName: "curta", EndsOn: []string{"short_rest"}, Economy: "free"}})
	r.mustToggle(t, "curta", false)
	if _, err := r.a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: r.a.campaignID, EncounterId: r.e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	if _, err := r.a.master.resource.TakeRest(t.Context(), connect.NewRequest(&playv1.TakeRestRequest{CampaignId: r.a.campaignID, Kind: playv1.RestKind_REST_KIND_LONG, IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("TakeRest(long) error = %v", err)
	}
	if got := r.states(t); len(got) != 0 {
		t.Errorf("states after a long rest = %v, want none", got)
	}
}

func TestTheMastersCorrectionEndsAStateByHand(t *testing.T) {
	t.Parallel()
	r := newStanceRig(t, stanceEffects("10", "dismiss"))
	r.mustToggle(t, stanceName, false)
	key := r.stateKey(stanceName)
	adjust := func(keys ...string) error {
		_, err := r.a.master.play.AdjustCharacterVitals(t.Context(), connect.NewRequest(&playv1.AdjustCharacterVitalsRequest{
			CampaignId: r.a.campaignID, CharacterId: r.a.toren.GetId(), IdempotencyKey: newKey(), EndStates: keys,
		}))
		return err
	}
	if err := adjust("feature:nao-esta@mesa#nada"); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("ending a state that is not on = %v, want invalid_argument", err)
	}
	if got := r.states(t); len(got) != 1 {
		t.Fatalf("states = %v after a refused correction, want the stance", got)
	}
	if err := adjust(key); err != nil {
		t.Fatalf("AdjustCharacterVitals(end_states) error = %v", err)
	}
	if got := r.states(t); len(got) != 0 {
		t.Errorf("states = %v, want none", got)
	}
	// A player cannot: the correction is the master's.
	if _, err := r.a.caio.play.AdjustCharacterVitals(t.Context(), connect.NewRequest(&playv1.AdjustCharacterVitalsRequest{
		CampaignId: r.a.campaignID, CharacterId: r.a.toren.GetId(), IdempotencyKey: newKey(), EndStates: []string{key},
	})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("a player's end_states = %v, want permission_denied", err)
	}
}

func TestTheUndoOfAStartPutsTheStateAndTheUseBack(t *testing.T) {
	t.Parallel()
	r := newStanceRig(t, stanceEffects("10", "dismiss"))
	plain := r.derived(t)
	r.mustToggle(t, stanceName, false)
	r.undoLast(t)
	v := r.a.vitals(t, r.a.toren)
	if len(v.GetActiveStates()) != 0 || usedOf(v, stanceUses) != 0 {
		t.Errorf("vitals after the undo = states %v, uses %d; want none and 0", v.GetActiveStates(), usedOf(v, stanceUses))
	}
	if toren := byLabel(t, r.a.get(t, r.a.master), "Toren"); toren.GetBonusActionUsed() || toren.GetSpeedFt() != plain.GetSpeedWalkFt() {
		t.Errorf("Toren after the undo: bonus action used %v, speed %d; want it free and the plain %d", toren.GetBonusActionUsed(), toren.GetSpeedFt(), plain.GetSpeedWalkFt())
	}
}

func TestTheUndoOfAnEndPutsTheStateBack(t *testing.T) {
	t.Parallel()
	r := newStanceRig(t, stanceEffects("10", "dismiss"))
	r.mustToggle(t, stanceName, false)
	r.a.passTo(t, r.a.get(t, r.a.master), "Goblin")
	r.a.passTo(t, r.a.get(t, r.a.master), "Toren")
	r.mustToggle(t, stanceName, true)
	r.undoLast(t)
	v := r.a.vitals(t, r.a.toren)
	if len(v.GetActiveStates()) != 1 || v.GetActiveStates()[0].GetRoundsLeft() != 9 {
		t.Errorf("states after the undo = %v, want the stance with the 9 rounds it had", v.GetActiveStates())
	}
}

func TestAnotherPlayerNeverReadsTheStatesOfACharacter(t *testing.T) {
	t.Parallel()
	r := newStanceRig(t, stanceEffects("10", "dismiss"))
	r.mustToggle(t, stanceName, false)
	for name, u := range map[string]*user{"another player": r.a.ana, "a third": r.a.bia} {
		for _, v := range u.liveSession(t, r.a.campaignID).GetVitals() {
			if v.GetCharacterId() == r.a.toren.GetId() {
				t.Errorf("%s reads Toren's vitals: %v", name, v)
			}
		}
		if got := len(byLabel(t, r.a.get(t, u), "Toren").GetEffects()); got != 0 {
			t.Errorf("%s reads %d effects on Toren's combatant", name, got)
		}
	}
	if v := r.a.caio.liveSession(t, r.a.campaignID).GetVitals(); len(v) != 1 || len(v[0].GetActiveStates()) != 1 {
		t.Errorf("Toren's player reads %v, want his own vitals with the stance", v)
	}
}

func TestTheStatesPersistAcrossRequests(t *testing.T) {
	t.Parallel()
	r := newStanceRig(t, stanceEffects("10", "dismiss"))
	plain := r.derived(t).GetArmorClass()
	r.mustToggle(t, stanceName, false)
	for range 2 {
		if got := r.states(t); len(got) != 1 {
			t.Fatalf("states = %v, want the stance on in every read", got)
		}
		if d := r.derived(t); !d.GetStates()[0].GetActive() || d.GetArmorClass() != plain+2 {
			t.Fatalf("the sheet reads AC %d and states %v, want %d and the stance on", d.GetArmorClass(), d.GetStates(), plain+2)
		}
	}
}
