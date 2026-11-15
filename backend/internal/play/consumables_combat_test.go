package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// A consumable used in a running combat (ChargesService.UseConsumable, ListConsumableTargets): drinking or giving costs the
// user's action on its turn, a gift reaches only a creature within 5 feet that the giver sees, and the line of the log tells the
// table the item and the owners the numbers. These tests need the database.

var blockedNotAdjacent = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_TARGET_NOT_ADJACENT_OR_SEEN

// potionFight is Toren (Caio's, first on the turn), Pensantus (Ana's, next to Toren) and Brisa (Bia's, far) in a combat
// with a goblin far away; each of the three holds the potions.
type potionFight struct {
	*stocked
	e *playv1.Encounter
}

func newPotionFight(t *testing.T, keys ...string) *potionFight {
	t.Helper()
	s := newStocked(t, keys...)
	s.bri = s.withItems(t, s.bri, keys...)
	e := s.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: s.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Toren": {3, 3}, "Pensantus": {4, 4}, "Brisa": {9, 8}, "Goblin": {12, 8}},
	})
	return &potionFight{stocked: s, e: e}
}

// line is the consumable line a user reads in the log, nil when it reads none.
func (f *potionFight) line(t *testing.T, u *user) *playv1.CombatLogEntry {
	t.Helper()
	for _, r := range f.log(t, u, f.e).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_CONSUMABLE_USED {
				return en
			}
		}
	}
	return nil
}

func TestDrinkingInACombatCostsTheActionOnYourTurn(t *testing.T) {
	t.Parallel()
	f := newPotionFight(t, potionCommon, potionGreater)
	f.hurt(t, f.toren, 1)
	_, err := f.use(t, f.ana, f.pens, potionCommon)
	wantBlockedBy(t, "a drink off turn", err, blockedNotYourTurn)
	res := f.mustUse(t, f.caio, f.toren, potionCommon)
	if res.GetHealed() == 0 || f.holds(t, f.toren, potionCommon) {
		t.Errorf("the drink = %v, want the unit spent and a heal", res)
	}
	_, err = f.use(t, f.caio, f.toren, potionGreater)
	wantBlockedBy(t, "a second drink in the turn", err, blockedActionUsed)
	if !f.holds(t, f.toren, potionGreater) {
		t.Error("the refused drink spent the potion")
	}
}

func TestGivingAPotionNextToTheGiverHealsTheReceiverAndSpendsTheGiversAction(t *testing.T) {
	t.Parallel()
	f := newPotionFight(t, potionCommon, potionGreater)
	f.hurt(t, f.pens, 1)
	res := f.mustUse(t, f.caio, f.toren, potionCommon, giveTo(f.pens))
	if res.GetHealed() == 0 || res.GetTargetCharacterId() != f.pens.GetId() || f.holds(t, f.toren, potionCommon) || !f.holds(t, f.pens, potionCommon) {
		t.Errorf("the gift = %v, want Pensantus healed by Toren's potion", res)
	}
	if len(res.GetVitals()) != 0 {
		t.Errorf("the giver was told the receiver's vitals: %v", res.GetVitals())
	}
	_, err := f.use(t, f.caio, f.toren, potionGreater)
	wantBlockedBy(t, "a drink after the gift", err, blockedActionUsed)
}

func TestAPotionIsNotGivenOutOfReach(t *testing.T) {
	t.Parallel()
	f := newPotionFight(t, potionCommon)
	_, err := f.use(t, f.caio, f.toren, potionCommon, giveTo(f.bri))
	wantBlockedBy(t, "a gift out of reach", err, blockedNotAdjacent)
	if !f.holds(t, f.toren, potionCommon) {
		t.Error("a refused gift spent the potion")
	}
	f.mustUse(t, f.caio, f.toren, potionCommon, giveTo(f.pens)) // the refusals spent no action
}

func TestTheMasterUsesAPotionForACharacterBoundByItsTurn(t *testing.T) {
	t.Parallel()
	f := newPotionFight(t, potionCommon)
	f.hurt(t, f.pens, 1)
	_, err := f.use(t, f.master, f.pens, potionCommon)
	wantBlockedBy(t, "the master acting for a character off turn", err, blockedNotYourTurn)
	if res := f.mustUse(t, f.master, f.toren, potionCommon, giveTo(f.pens)); res.GetHealed() == 0 || len(res.GetVitals()) != 1 {
		t.Errorf("the master's gift = %v, want a heal and the receiver's vitals", res)
	}
}

func TestAReceiverAtZeroHitPointsWakesAndAnUnconsciousUserCannotAct(t *testing.T) {
	t.Parallel()
	f := newPotionFight(t, potionCommon, potionSupreme)
	f.hurt(t, f.pens, 0)
	f.mustUse(t, f.caio, f.toren, potionCommon, giveTo(f.pens))
	if got := f.vitals(t, f.pens).GetHitPointsCurrent(); got == 0 {
		t.Error("Pensantus is still at 0 hit points")
	}
	if c := byLabel(t, f.get(t, f.master), "Pensantus"); c.GetDeathSuccesses() != 0 || c.GetDeathFailures() != 0 {
		t.Errorf("the death saves = %d/%d, want them reset", c.GetDeathSuccesses(), c.GetDeathFailures())
	}
	f.hurt(t, f.toren, 0)
	_, err := f.use(t, f.caio, f.toren, potionSupreme)
	wantBlockedBy(t, "the unconscious user", err, blockedDown)
}

func TestASupremePotionIsCutAtTheMaximumInACombat(t *testing.T) {
	t.Parallel()
	f := newPotionFight(t, potionSupreme)
	full := f.maxHP(t, f.toren)
	f.hurt(t, f.toren, full-3)
	if res := f.mustUse(t, f.caio, f.toren, potionSupreme); res.GetHealed() != 3 || f.vitals(t, f.toren).GetHitPointsCurrent() != full {
		t.Errorf("the supreme potion healed %d, want 3 up to %d", res.GetHealed(), full)
	}
}

func TestAResistancePotionInACombatIsAnEffectOfTheReceiverForTheHourInRounds(t *testing.T) {
	t.Parallel()
	f := newPotionFight(t, potionFire)
	f.mustUse(t, f.caio, f.toren, potionFire, giveTo(f.pens))
	var ends int32
	if err := f.h.pool.QueryRow(t.Context(), `SELECT ends_round FROM combatant_states WHERE combatant_id = $1 AND source_key = $2`, f.id(t, "Pensantus"), potionFire).Scan(&ends); err != nil {
		t.Fatalf("read the effect: %v", err)
	}
	if want := f.e.GetRound() + 600; ends != want {
		t.Errorf("the effect ends in round %d, want %d", ends, want)
	}
}

func TestAConsumableEventClosesTheUndoChainAndItsRetryIsTheSameUse(t *testing.T) {
	t.Parallel()
	f := newPotionFight(t, potionCommon)
	f.hurt(t, f.toren, 1)
	if _, err := f.adjustHP(t, f.e, "Goblin", healHP(1)); err != nil {
		t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
	}
	if f.log(t, f.master, f.e).GetUndoableEventId() == "" {
		t.Fatal("the heal of the goblin is not undoable")
	}
	key, entry := newKey(), f.entryOf(t, f.toren, potionCommon)
	same := func(r *playv1.UseConsumableRequest) { r.IdempotencyKey, r.EntryId = key, entry }
	first := f.mustUse(t, f.caio, f.toren, potionCommon, same)
	again := f.mustUse(t, f.caio, f.toren, potionCommon, same)
	if again.GetHealed() != first.GetHealed() || again.GetEntryId() != first.GetEntryId() || f.countEvents(t, eventConsumableUsed) != 1 {
		t.Errorf("the retry answered %v after %v", again, first)
	}
	if got := f.log(t, f.master, f.e).GetUndoableEventId(); got != "" {
		t.Errorf("the undoable event = %q after the drink, want none", got)
	}
}

// The table reads who used which item on whom; the dice and the hit points are the master's and the two owners'.
func TestTheLineOfAGiftTellsTheTableTheItemAndTheOwnersTheNumbers(t *testing.T) {
	t.Parallel()
	f := newPotionFight(t, potionCommon)
	f.hurt(t, f.pens, 1)
	f.mustUse(t, f.caio, f.toren, potionCommon, giveTo(f.pens))
	for name, u := range map[string]*user{"the master": f.master, "the giver's owner": f.caio, "the receiver's owner": f.ana} {
		l := f.line(t, u).GetConsumableUse()
		if l == nil || l.GetHealed() == 0 || l.GetDiceText() == "" || len(l.GetFaces()) == 0 || l.GetRollTotal() == 0 || l.GetTargetLabel() != "Pensantus" {
			t.Errorf("%s reads %v, want the numbers and the receiver", name, l)
		}
	}
	l := f.line(t, f.bia).GetConsumableUse()
	if l == nil || l.GetItemNamePt() == "" || l.GetTargetLabel() != "Pensantus" {
		t.Fatalf("another player reads %v, want the item and the receiver", l)
	}
	if l.GetHealed() != 0 || l.GetDiceText() != "" || len(l.GetFaces()) != 0 || l.GetRollTotal() != 0 {
		t.Errorf("another player reads numbers: %v", l)
	}
}

func TestTheTargetsOfAGiftAreTheAdjacentCharacters(t *testing.T) {
	t.Parallel()
	f := newPotionFight(t, potionCommon)
	f.hurt(t, f.pens, 0)
	list := func(u *user, who string) ([]*playv1.ConsumableTarget, error) {
		res, err := u.charges.ListConsumableTargets(t.Context(), connect.NewRequest(&playv1.ListConsumableTargetsRequest{
			CampaignId: f.campaignID, CharacterId: who, EntryId: f.entryOf(t, f.toren, potionCommon),
		}))
		if err != nil {
			return nil, err
		}
		return res.Msg.GetTargets(), nil
	}
	for _, u := range []*user{f.caio, f.master} {
		got, err := list(u, f.toren.GetId())
		if err != nil || len(got) != 1 || got[0].GetCharacterId() != f.pens.GetId() || got[0].GetDistanceFt() != 5 ||
			got[0].GetState() != playv1.ConsumableTargetState_CONSUMABLE_TARGET_STATE_DOWN || !got[0].GetMayReceive() {
			t.Errorf("targets = %v, %v; want Pensantus, 5 ft, down", got, err)
		}
	}
	if _, err := list(f.bia, f.toren.GetId()); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("another player's list = %v, want permission_denied", err)
	}
}

func TestTheTargetsOutOfACombatAreTheOtherCharactersOfTheParty(t *testing.T) {
	t.Parallel()
	s := newStocked(t, potionCommon)
	res, err := s.caio.charges.ListConsumableTargets(t.Context(), connect.NewRequest(&playv1.ListConsumableTargetsRequest{
		CampaignId: s.campaignID, CharacterId: s.toren.GetId(), EntryId: s.entryOf(t, s.toren, potionCommon),
	}))
	if err != nil {
		t.Fatalf("ListConsumableTargets() error = %v", err)
	}
	if len(res.Msg.GetTargets()) != 2 {
		t.Fatalf("targets = %v; want Pensantus and Brisa", res.Msg.GetTargets())
	}
	for _, g := range res.Msg.GetTargets() {
		if g.GetCharacterId() == s.toren.GetId() || g.GetDistanceFt() != 0 || !g.GetMayReceive() {
			t.Errorf("target %v: want another character, no distance", g)
		}
	}
	_, err = s.caio.charges.ListConsumableTargets(t.Context(), connect.NewRequest(&playv1.ListConsumableTargetsRequest{
		CampaignId: s.campaignID, CharacterId: s.toren.GetId(), EntryId: newKey(),
	}))
	wantBlockedBy(t, "an entry that is gone", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ITEM_GONE)
}

// Haste's extra action pays for a potion when the action of the turn is used, once; the action stays as it was.
func TestAPotionUnderHasteIsPaidByTheExtraActionOnceWhenTheActionIsUsed(t *testing.T) {
	t.Parallel()
	f := newPotionFight(t, potionCommon, potionGreater, potionSuperior)
	f.mustAddEffect(t, f.e, "spell:haste", []string{"Toren"}, f.rounds(t, 3, "Toren"))
	f.mustUse(t, f.caio, f.toren, potionCommon) // the action
	spent := func() (action, extra bool) {
		if err := f.h.pool.QueryRow(t.Context(), `SELECT action_used, extra_action_used FROM combatants WHERE id = $1`, f.id(t, "Toren")).Scan(&action, &extra); err != nil {
			t.Fatalf("read the economy of Toren: %v", err)
		}
		return action, extra
	}
	if action, extra := spent(); !action || extra {
		t.Fatalf("after the first drink action used = %v, extra = %v; want the action spent and the extra one free", action, extra)
	}
	f.mustUse(t, f.caio, f.toren, potionGreater, giveTo(f.pens)) // the extra action
	if action, extra := spent(); !action || !extra {
		t.Fatalf("after the second potion action used = %v, extra = %v; want both spent", action, extra)
	}
	_, err := f.use(t, f.caio, f.toren, potionSuperior)
	wantBlockedBy(t, "a third potion with both spent", err, blockedActionUsed)
	if !f.holds(t, f.toren, potionSuperior) {
		t.Error("the refused potion was spent")
	}
}

// A receiver of the campaign that does not fight in the active combat is out of the giver's reach: nothing is spent, no action either.
func TestAPotionIsNotGivenToACharacterThatDoesNotFightInTheCombat(t *testing.T) {
	t.Parallel()
	s := newStocked(t, potionCommon)
	e := s.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: s.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Toren": {3, 3}, "Pensantus": {4, 4}, "Brisa": {9, 8}, "Goblin": {12, 8}},
		setup:    true,
	})
	// Brisa leaves the combat before it begins: she is still a character of the campaign, but no combatant of the fight.
	if _, err := s.master.combat.RemoveCombatant(t.Context(), connect.NewRequest(&playv1.RemoveCombatantRequest{
		CampaignId: s.campaignID, EncounterId: e.GetId(), CombatantId: s.id(t, "Brisa"), IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("RemoveCombatant(Brisa) error = %v", err)
	}
	if _, err := s.master.combat.BeginCombat(t.Context(), connect.NewRequest(&playv1.BeginCombatRequest{CampaignId: s.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("BeginCombat() error = %v", err)
	}
	f := &potionFight{stocked: s, e: e}
	_, err := f.use(t, f.caio, f.toren, potionCommon, giveTo(f.bri))
	wantBlockedBy(t, "a gift to a character outside the combat", err, blockedNotAdjacent)
	if !f.holds(t, f.toren, potionCommon) {
		t.Error("the refused gift spent the potion")
	}
	f.mustUse(t, f.caio, f.toren, potionCommon, giveTo(f.pens)) // the refusal spent no action
}
