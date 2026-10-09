package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The class resource flows of a turn: Lay on Hands, Flexible Casting, Metamagic and
// Bardic Inspiration (SRD 5.1, Paladin 1, Sorcerer 2 and 3, Bard 1). These tests need
// the database (MEURPG_TEST_DATABASE_URL). The fixture is a paladin, a sorcerer and a
// bard of level 5 (one player each) and a goblin, on a map.

const (
	fireballSpell = "spell:fireball"
	chargePerson  = "spell:charm-person"
	rayOfFrost    = "spell:ray-of-frost"
)

// newResourceTable is the party: Tavo the paladin 5 (Strength 16, Charisma 14, a pool of
// 25), Nael the sorcerer 5 (Charisma 16, 5 sorcery points, Twinned Spell and Careful
// Spell) and Orla the bard 5 (Charisma 16, a d8, 3 uses), with a goblin, all revealed.
func newResourceTable(t *testing.T) *armed {
	t.Helper()
	return newResourceTableWith(t, []string{rules.MetamagicTwinned, rules.MetamagicCareful})
}

func newResourceTableWith(t *testing.T, metamagic []string) *armed {
	t.Helper()
	return newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Tavo", "class:paladin", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 10, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 14}, []string{battleaxe}, nil)
		sorcerer := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 8, Dexterity: 14, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 16}, RaceKey: "race:human",
			Classes:           []*charactersv1.ClassLevel{{ClassKey: "class:sorcerer", Level: 5}},
			CantripKeys:       []string{fireBolt, rayOfFrost},
			KnownSpellKeys:    []string{magicMissileSpell, burningHands, holdPerson, fireballSpell, chargePerson, sleepSpell},
			PreparedSpellKeys: []string{magicMissileSpell, burningHands, holdPerson, fireballSpell, chargePerson, sleepSpell},
			FeatureChoiceKeys: metamagic,
		}}}
		res, err := a.ana.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
			CampaignId: a.campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Nael", Sheet: sorcerer,
		}))
		if err != nil {
			t.Fatalf("CreateCharacter(Nael) error = %v", err)
		}
		a.pens = res.Msg.GetCharacter()
		a.bri = a.bia.hero(t, a.campaignID, "Orla", "class:bard", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 8, Dexterity: 14, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 16}, nil, nil)
	})
}

// resourceFight starts the combat of the fixture: Tavo, Nael and Orla in a row (initiative 18,
// 14 and 10), the goblin at the end, everyone revealed.
func (a *armed) resourceFight(t *testing.T) *playv1.Encounter {
	t.Helper()
	return a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Tavo": 18, "Nael": 14, "Orla": 10},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Tavo": {3, 3}, "Nael": {4, 3}, "Orla": {5, 3}, "Goblin": {4, 4}},
	})
}

// layOnHands calls UseLayOnHands as u, by labels.
func (a *armed) layOnHands(t *testing.T, u *user, e *playv1.Encounter, actor, target string, edit func(*playv1.UseLayOnHandsRequest)) (*playv1.UseLayOnHandsResponse, error) {
	t.Helper()
	req := &playv1.UseLayOnHandsRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), ActorId: a.id(t, actor), TargetId: a.id(t, target), IdempotencyKey: newKey(),
	}
	edit(req)
	res, err := u.resource.UseLayOnHands(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func healAmount(n int32) func(*playv1.UseLayOnHandsRequest) {
	return func(r *playv1.UseLayOnHandsRequest) { r.Effect = &playv1.UseLayOnHandsRequest_Amount{Amount: n} }
}

func cureOf(c playv1.LayOnHandsCure) func(*playv1.UseLayOnHandsRequest) {
	return func(r *playv1.UseLayOnHandsRequest) { r.Effect = &playv1.UseLayOnHandsRequest_Cure{Cure: c} }
}

// vitalsOfCharacter is the character's vitals as the master reads them.
func (a *armed) vitalsOfCharacter(t *testing.T, c *charactersv1.Character) *playv1.CharacterVitals {
	t.Helper()
	for _, v := range a.master.liveSession(t, a.campaignID).GetVitals() {
		if v.GetCharacterId() == c.GetId() {
			return v
		}
	}
	t.Fatalf("no vitals of %s", c.GetName())
	return nil
}

func poolOf(v *playv1.CharacterVitals, key string) (left, total int32) {
	for _, r := range v.GetResources() {
		if r.GetKey() == key {
			return r.GetTotal() - r.GetUsed(), r.GetTotal()
		}
	}
	return -1, -1
}

// SRD 5.1, Paladin 1: the touch restores up to the amount drawn from the pool of 5 hit
// points for each paladin level; the pool, the action and the vitals follow.
func TestLayOnHandsHealsTheTargetAndSpendsFromThePool(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	a.correct(t, a.pens, func(r *playv1.AdjustCharacterVitalsRequest) { r.HitPointsCurrent = proto.Int32(10) })
	e := a.resourceFight(t)
	if left, total := poolOf(a.vitalsOfCharacter(t, a.toren), rules.LayOnHandsKey); left != 25 || total != 25 {
		t.Fatalf("Tavo's pool = %d of %d, want 25 of 25 (5 for each paladin level)", left, total)
	}
	// The master's own pool reading above is the master's; Tavo's player touches Nael.
	res, err := a.layOnHands(t, a.caio, e, "Tavo", "Nael", healAmount(8))
	if err != nil {
		t.Fatalf("UseLayOnHands() error = %v", err)
	}
	if res.GetSpent() != 8 || res.GetPoolLeft() != 17 || res.GetNothingHappened() {
		t.Errorf("spent %d, pool left %d, nothing happened %v; want 8, 17, false", res.GetSpent(), res.GetPoolLeft(), res.GetNothingHappened())
	}
	// Nael's player is told what her character regained; Tavo's player (a creature that is not his) is not.
	if res.Healed != nil {
		t.Errorf("Tavo's player was told the target regained %d: only the master and the target's player are", res.GetHealed())
	}
	if v := a.vitalsOfCharacter(t, a.pens); v.GetHitPointsCurrent() != 18 {
		t.Errorf("Nael has %d hit points, want 10 + 8 = 18", v.GetHitPointsCurrent())
	}
	if left, _ := poolOf(a.vitalsOfCharacter(t, a.toren), rules.LayOnHandsKey); left != 17 {
		t.Errorf("Tavo's pool = %d, want 17", left)
	}
	// The touch is an action: the turn's action is spent.
	if c := byLabel(t, a.get(t, a.caio), "Tavo"); !c.GetActionUsed() {
		t.Error("Tavo's action is not spent by Lay on Hands")
	}
	_, err = a.layOnHands(t, a.caio, e, "Tavo", "Nael", healAmount(1))
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ACTION_USED)
}

// The healing stops at the maximum, and the whole amount drawn is spent all the same
// (the board: "Gasta 8").
func TestLayOnHandsSpendsWhatWasDrawnEvenIfTheTargetWasNearlyWhole(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	full := a.vitalsOfCharacter(t, a.pens).GetHitPointsMax()
	a.correct(t, a.pens, func(r *playv1.AdjustCharacterVitalsRequest) { r.HitPointsCurrent = proto.Int32(full - 2) })
	e := a.resourceFight(t)
	res, err := a.layOnHands(t, a.master, e, "Tavo", "Nael", healAmount(8))
	if err != nil {
		t.Fatalf("UseLayOnHands() error = %v", err)
	}
	if res.GetSpent() != 8 || res.GetPoolLeft() != 17 {
		t.Errorf("spent %d, pool left %d, want 8 and 17", res.GetSpent(), res.GetPoolLeft())
	}
	if res.Healed == nil || res.GetHealed() != 2 {
		t.Errorf("healed = %v, want the master told 2 (the target's maximum)", res.Healed)
	}
	if v := a.vitalsOfCharacter(t, a.pens); v.GetHitPointsCurrent() != full {
		t.Errorf("Nael has %d of %d hit points, want the maximum", v.GetHitPointsCurrent(), full)
	}
}

// Nael's player gets the amount her character regained; nobody else does.
func TestLayOnHandsTellsTheHealedAmountToTheTargetsPlayerAndTheMaster(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	a.correct(t, a.pens, func(r *playv1.AdjustCharacterVitalsRequest) { r.HitPointsCurrent = proto.Int32(10) })
	e := a.resourceFight(t)
	if _, err := a.layOnHands(t, a.caio, e, "Tavo", "Nael", healAmount(6)); err != nil {
		t.Fatalf("UseLayOnHands() error = %v", err)
	}
	seen := map[string]*playv1.CombatLogResource{}
	for name, u := range map[string]*user{"master": a.master, "Ana": a.ana, "Caio": a.caio, "Bia": a.bia} {
		for _, l := range logEntries(a.log(t, u, e)) {
			if l.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_RESOURCE {
				seen[name] = l.GetResource()
			}
		}
		if seen[name] == nil {
			t.Fatalf("%s has no resource line in the log", name)
		}
	}
	if seen["master"].Healed == nil || seen["master"].GetHealed() != 6 || seen["Ana"].Healed == nil || seen["Ana"].GetHealed() != 6 {
		t.Errorf("the master and the target's player read %v and %v, want 6 healed", seen["master"].Healed, seen["Ana"].Healed)
	}
	if seen["Caio"].Healed != nil || seen["Bia"].Healed != nil {
		t.Error("a player who is not the target's got the number of hit points regained")
	}
	if seen["Bia"].GetSpent() != 6 || seen["Bia"].GetKey() != layOnHandsAction {
		t.Errorf("the others read %v, want what was spent from the pool", seen["Bia"])
	}
}

func TestLayOnHandsRefusals(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	e := a.resourceFight(t)
	for name, tc := range map[string]struct {
		edit func(*playv1.UseLayOnHandsRequest)
		code connect.Code
	}{
		"no effect":          {func(*playv1.UseLayOnHandsRequest) {}, connect.CodeInvalidArgument},
		"amount zero":        {healAmount(0), connect.CodeInvalidArgument},
		"a negative amount":  {healAmount(-3), connect.CodeInvalidArgument},
		"more than the pool": {healAmount(26), connect.CodeInvalidArgument},
		"a cure of nothing":  {cureOf(playv1.LayOnHandsCure_LAY_ON_HANDS_CURE_UNSPECIFIED), connect.CodeInvalidArgument},
	} {
		if _, err := a.layOnHands(t, a.caio, e, "Tavo", "Nael", tc.edit); connect.CodeOf(err) != tc.code {
			t.Errorf("%s: err = %v, want %v", name, err, tc.code)
		}
	}
	// A refusal spends nothing.
	if left, _ := poolOf(a.vitalsOfCharacter(t, a.toren), rules.LayOnHandsKey); left != 25 {
		t.Errorf("the pool is %d after the refusals, want 25", left)
	}
	if c := byLabel(t, a.get(t, a.caio), "Tavo"); c.GetActionUsed() {
		t.Error("a refusal spent the action")
	}
	// Not the touch's own combatant, not a player's turn, not a paladin.
	if _, err := a.layOnHands(t, a.ana, e, "Tavo", "Nael", healAmount(5)); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("another player's UseLayOnHands = %v, want permission_denied", err)
	}
	if _, err := a.layOnHands(t, a.bia, e, "Orla", "Nael", healAmount(5)); err == nil {
		t.Error("a bard's UseLayOnHands worked: she has no Lay on Hands")
	} else {
		wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_NOT_AVAILABLE)
	}
	if _, err := a.layOnHands(t, a.ana, e, "Nael", "Tavo", healAmount(5)); err == nil {
		t.Error("a sorcerer's UseLayOnHands worked")
	}
}

// A player touches what is within 5 ft; farther is out of reach; the master is held to nothing.
func TestLayOnHandsReachesTheTouchOnly(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Tavo": 18, "Nael": 14, "Orla": 10}, reveal: []string{"Goblin"},
		at: map[string][2]int32{"Tavo": {3, 3}, "Nael": {4, 3}, "Orla": {9, 3}, "Goblin": {4, 4}},
	})
	_, err := a.layOnHands(t, a.caio, e, "Tavo", "Orla", healAmount(5))
	b := wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_TARGET_OUT_OF_REACH)
	if b.GetMissingFt() != 25 {
		t.Errorf("missing %d ft, want 25 (30 ft away, a reach of 5)", b.GetMissingFt())
	}
	// The paladin may touch itself, and the turn options list who it can touch.
	opts, err := a.caio.combat.GetTurnOptions(t.Context(), connect.NewRequest(&playv1.GetTurnOptionsRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Tavo")}))
	if err != nil {
		t.Fatalf("GetTurnOptions() error = %v", err)
	}
	var touch *playv1.ResourceTargets
	for _, r := range opts.Msg.GetResourceTargets() {
		if r.GetActionKey() == layOnHandsAction {
			touch = r
		}
	}
	if touch == nil || len(touch.GetTargets()) < 4 || touch.GetTargets()[0].GetTarget().GetLabel() != "Tavo" {
		t.Fatalf("the touch targets = %v, want Tavo himself first, then the others", touch)
	}
	for _, rt := range touch.GetTargets() {
		switch rt.GetTarget().GetLabel() {
		case "Nael", "Goblin", "Tavo":
			if rt.GetTarget().GetTooFar() {
				t.Errorf("%s is too far for the touch", rt.GetTarget().GetLabel())
			}
		case "Orla":
			if !rt.GetTarget().GetTooFar() {
				t.Error("Orla, 30 ft away, is not too far for the touch")
			}
		}
	}
	if _, err := a.layOnHands(t, a.master, e, "Tavo", "Orla", healAmount(5)); err != nil {
		t.Errorf("the master's touch from afar: %v", err)
	}
}

// SRD 5.1, Paladin 1: 5 points of the pool cure a disease or neutralize a poison.
func TestLayOnHandsCuresAPoisonAndADisease(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	e := a.resourceFight(t)
	a.setConditions(t, e, "Nael", "condition:poisoned", "condition:frightened")
	res, err := a.layOnHands(t, a.caio, e, "Tavo", "Nael", cureOf(playv1.LayOnHandsCure_LAY_ON_HANDS_CURE_POISON))
	if err != nil {
		t.Fatalf("UseLayOnHands(poison) error = %v", err)
	}
	if res.GetSpent() != 5 || res.GetPoolLeft() != 20 || res.GetNothingHappened() {
		t.Errorf("poison cure: spent %d, left %d, nothing %v; want 5, 20, false", res.GetSpent(), res.GetPoolLeft(), res.GetNothingHappened())
	}
	nael := byLabel(t, a.get(t, a.master), "Nael")
	if len(nael.GetConditions()) != 1 || nael.GetConditions()[0] != "condition:frightened" {
		t.Errorf("Nael's conditions = %v, want only frightened", nael.GetConditions())
	}
	// The disease has no tracking: the points are spent and the master gives the effect.
	a.h.roller.queue()
	e2 := a.turnOf(t, "Tavo")
	res, err = a.layOnHands(t, a.caio, e2, "Tavo", "Nael", cureOf(playv1.LayOnHandsCure_LAY_ON_HANDS_CURE_DISEASE))
	if err != nil {
		t.Fatalf("UseLayOnHands(disease) error = %v", err)
	}
	if res.GetSpent() != 5 || res.GetPoolLeft() != 15 || res.GetNothingHappened() {
		t.Errorf("disease cure: spent %d, left %d, nothing %v; want 5, 15, false", res.GetSpent(), res.GetPoolLeft(), res.GetNothingHappened())
	}
	// A poison that is not there: the touch does nothing and the points are spent.
	e3 := a.turnOf(t, "Tavo")
	res, err = a.layOnHands(t, a.caio, e3, "Tavo", "Nael", cureOf(playv1.LayOnHandsCure_LAY_ON_HANDS_CURE_POISON))
	if err != nil {
		t.Fatalf("UseLayOnHands(no poison) error = %v", err)
	}
	if res.GetSpent() != 5 || !res.GetNothingHappened() {
		t.Errorf("a cure of a poison that is not there: spent %d, nothing %v; want 5 and true", res.GetSpent(), res.GetNothingHappened())
	}
}

// SRD 5.1, Paladin 1: "This feature has no effect on undead and constructs". The action and the
// points are spent, the player is told only that nothing happened, and the line says why to
// the master alone (RN-10).
func TestLayOnHandsOnAnUndeadOrAConstructDoesNothingAndNeverSaysWhy(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Tavo": 18, "Nael": 14, "Orla": 10}, reveal: []string{"Goblin"}, setup: true,
		at: map[string][2]int32{"Tavo": {3, 3}, "Nael": {4, 3}, "Orla": {5, 3}, "Goblin": {4, 4}},
	})
	a.h.roller.queue(10, 11)
	monsters := a.mustAddMonsters(t, e, func(r *playv1.AddMonstersRequest) { r.CreatureKey = "monster:zombie"; r.Count = 1 })
	armor := a.mustAddMonsters(t, monsters.GetEncounter(), func(r *playv1.AddMonstersRequest) { r.CreatureKey = "monster:animated-armor"; r.Count = 1 })
	for _, label := range []string{"Zumbi", "Armadura animada"} {
		if _, err := a.master.combat.SetCombatantHidden(t.Context(), connect.NewRequest(&playv1.SetCombatantHiddenRequest{
			CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: byLabel(t, armor.GetEncounter(), label).GetId(), IdempotencyKey: newKey(), Hidden: false,
		})); err != nil {
			t.Fatalf("reveal %s: %v", label, err)
		}
	}
	for label, sq := range map[string][2]int32{"Zumbi": {3, 4}, "Armadura animada": {2, 3}} {
		if _, err := a.master.combat.MoveCombatant(t.Context(), connect.NewRequest(&playv1.MoveCombatantRequest{
			CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, label), IdempotencyKey: newKey(), Col: sq[0], Row: sq[1],
		})); err != nil {
			t.Fatalf("place %s: %v", label, err)
		}
	}
	if _, err := a.master.combat.BeginCombat(t.Context(), connect.NewRequest(&playv1.BeginCombatRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("BeginCombat() error = %v", err)
	}
	e = a.get(t, a.master)
	if e.GetCurrentCombatantId() != a.id(t, "Tavo") {
		t.Skipf("the turn is not Tavo's: %v", labels(e))
	}
	res, err := a.layOnHands(t, a.caio, e, "Tavo", "Zumbi", healAmount(7))
	if err != nil {
		t.Fatalf("UseLayOnHands(zombie) error = %v", err)
	}
	if res.GetSpent() != 7 || res.GetPoolLeft() != 18 || !res.GetNothingHappened() || res.Healed != nil {
		t.Errorf("on a zombie: spent %d, left %d, nothing %v, healed %v; want 7, 18, true and no number", res.GetSpent(), res.GetPoolLeft(), res.GetNothingHappened(), res.Healed)
	}
	e = a.turnOf(t, "Tavo")
	res, err = a.layOnHands(t, a.caio, e, "Tavo", "Armadura animada", healAmount(2))
	if err != nil || !res.GetNothingHappened() {
		t.Errorf("on a construct: %v, %v; want nothing to happen", res, err)
	}
	// What the lines say: the master reads why; the players never read a type.
	for name, u := range map[string]*user{"master": a.master, "Caio": a.caio, "Ana": a.ana} {
		var lines int
		for _, l := range logEntries(a.log(t, u, e)) {
			if l.GetKind() != playv1.CombatLogKind_COMBAT_LOG_KIND_RESOURCE {
				continue
			}
			lines++
			why := l.GetResource().NothingReason
			switch {
			case name == "master" && (why == nil || (*why != "undead" && *why != "construct")):
				t.Errorf("the master's line has the reason %v, want undead or construct", why)
			case name != "master" && why != nil:
				t.Errorf("%s's line carries the reason %q: a player never learns a creature's type", name, *why)
			}
			if !l.GetResource().GetNothingHappened() {
				t.Errorf("%s's line does not say nothing happened", name)
			}
		}
		if lines != 2 {
			t.Errorf("%s has %d resource lines, want 2", name, lines)
		}
	}
	if raw, err := proto.Marshal(res); err == nil && (strings.Contains(string(raw), "undead") || strings.Contains(string(raw), "construct")) {
		t.Error("the answer to the player carries the creature's type")
	}
}

// The master's undo gives back the points, the hit points and the cured poison.
func TestUndoTakesBackLayOnHands(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	a.correct(t, a.pens, func(r *playv1.AdjustCharacterVitalsRequest) { r.HitPointsCurrent = proto.Int32(10) })
	e := a.resourceFight(t)
	a.setConditions(t, e, "Nael", "condition:poisoned")
	if _, err := a.layOnHands(t, a.caio, e, "Tavo", "Nael", healAmount(6)); err != nil {
		t.Fatalf("UseLayOnHands() error = %v", err)
	}
	a.undoLast(t, e)
	if v := a.vitalsOfCharacter(t, a.pens); v.GetHitPointsCurrent() != 10 {
		t.Errorf("after the undo Nael has %d hit points, want 10", v.GetHitPointsCurrent())
	}
	if left, _ := poolOf(a.vitalsOfCharacter(t, a.toren), rules.LayOnHandsKey); left != 25 {
		t.Errorf("after the undo the pool is %d, want 25", left)
	}
	if c := byLabel(t, a.get(t, a.caio), "Tavo"); c.GetActionUsed() {
		t.Error("the undo did not give the action back")
	}
	// The cure of the poison comes back too.
	if _, err := a.layOnHands(t, a.caio, e, "Tavo", "Nael", cureOf(playv1.LayOnHandsCure_LAY_ON_HANDS_CURE_POISON)); err != nil {
		t.Fatalf("UseLayOnHands(poison) error = %v", err)
	}
	a.undoLast(t, e)
	if nael := byLabel(t, a.get(t, a.master), "Nael"); len(nael.GetConditions()) != 1 || nael.GetConditions()[0] != "condition:poisoned" {
		t.Errorf("after the undo Nael's conditions = %v, want poisoned again", nael.GetConditions())
	}
}

// A key repeated gives the first answer and spends nothing more; the same key for another
// touch is refused.
func TestLayOnHandsIsIdempotent(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	a.correct(t, a.pens, func(r *playv1.AdjustCharacterVitalsRequest) { r.HitPointsCurrent = proto.Int32(10) })
	e := a.resourceFight(t)
	key := newKey()
	withKey := func(amount int32) func(*playv1.UseLayOnHandsRequest) {
		return func(r *playv1.UseLayOnHandsRequest) {
			r.Effect, r.IdempotencyKey = &playv1.UseLayOnHandsRequest_Amount{Amount: amount}, key
		}
	}
	first, err := a.layOnHands(t, a.caio, e, "Tavo", "Nael", withKey(6))
	if err != nil {
		t.Fatalf("UseLayOnHands() error = %v", err)
	}
	again, err := a.layOnHands(t, a.caio, e, "Tavo", "Nael", withKey(6))
	if err != nil {
		t.Fatalf("UseLayOnHands() retry error = %v", err)
	}
	if again.GetSpent() != first.GetSpent() || again.GetPoolLeft() != 19 {
		t.Errorf("retry: spent %d, left %d; first: spent %d, left %d", again.GetSpent(), again.GetPoolLeft(), first.GetSpent(), first.GetPoolLeft())
	}
	if left, _ := poolOf(a.vitalsOfCharacter(t, a.toren), rules.LayOnHandsKey); left != 19 {
		t.Errorf("the pool is %d, want 19: the retry must not spend twice", left)
	}
	if _, err := a.layOnHands(t, a.caio, e, "Tavo", "Nael", withKey(9)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("the key reused for another amount = %v, want invalid_argument", err)
	}
}

// An NPC's resources are not counted: the master adjusts a paladin NPC by hand.
func TestLayOnHandsIsForPlayersCharacters(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	e := a.resourceFight(t)
	_, err := a.layOnHands(t, a.master, e, "Goblin", "Tavo", healAmount(5))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("an NPC's UseLayOnHands = %v, want invalid_argument", err)
	}
}

// TakeAction does not take the actions that have a flow of their own.
func TestTakeActionRefusesTheResourceFlows(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	e := a.resourceFight(t)
	_, err := a.caio.combat.TakeAction(t.Context(), connect.NewRequest(&playv1.TakeActionRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Tavo"), ActionKey: layOnHandsAction, IdempotencyKey: newKey(),
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("TakeAction(lay on hands) = %v, want invalid_argument", err)
	}
}

// logEntries is every entry of the combat log, round after round.
func logEntries(res *playv1.ListCombatLogResponse) []*playv1.CombatLogEntry {
	var out []*playv1.CombatLogEntry
	for _, r := range res.GetRounds() {
		out = append(out, r.GetEntries()...)
	}
	return out
}

// flexible calls CreateSpellSlot or ConvertSpellSlot as u, by label.
func (a *armed) createSlot(t *testing.T, u *user, e *playv1.Encounter, actor string, level int32) (*playv1.CreateSpellSlotResponse, error) {
	t.Helper()
	res, err := u.resource.CreateSpellSlot(t.Context(), connect.NewRequest(&playv1.CreateSpellSlotRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), ActorId: a.id(t, actor), SlotLevel: level, IdempotencyKey: newKey(),
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (a *armed) convertSlot(t *testing.T, u *user, e *playv1.Encounter, actor string, level int32) (*playv1.ConvertSpellSlotResponse, error) {
	t.Helper()
	res, err := u.resource.ConvertSpellSlot(t.Context(), connect.NewRequest(&playv1.ConvertSpellSlotRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), ActorId: a.id(t, actor), SlotLevel: level, IdempotencyKey: newKey(),
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func slotsOf(v *playv1.CharacterVitals, level int32) (total, used, created int32) {
	for _, s := range v.GetSpellSlots() {
		if s.GetLevel() == level {
			return s.GetTotal(), s.GetUsed(), s.GetCreated()
		}
	}
	return 0, 0, 0
}

// SRD 5.1, Sorcerer 2, Flexible Casting: 3 sorcery points make a slot of the 2nd level, as a
// bonus action. The slot is one more, and the points are spent.
func TestFlexibleCastingCreatesASlotForTheTableCost(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Nael": 18, "Tavo": 14, "Orla": 10}, reveal: []string{"Goblin"},
		at: map[string][2]int32{"Tavo": {3, 3}, "Nael": {4, 3}, "Orla": {5, 3}, "Goblin": {4, 4}},
	})
	nael := a.vitalsOfCharacter(t, a.pens)
	if left, total := poolOf(nael, rules.SorceryPointsKey); left != 5 || total != 5 {
		t.Fatalf("Nael's sorcery points = %d of %d, want 5 of 5", left, total)
	}
	before, _, _ := slotsOf(nael, 2)
	res, err := a.createSlot(t, a.ana, e, "Nael", 2)
	if err != nil {
		t.Fatalf("CreateSpellSlot() error = %v", err)
	}
	if res.GetCost() != 3 {
		t.Errorf("a slot of the 2nd level cost %d, want 3", res.GetCost())
	}
	if total, used, created := slotsOf(res.GetVitals(), 2); total != before+1 || used != 0 || created != 1 {
		t.Errorf("2nd-level slots = %d total, %d used, %d created; want %d, 0, 1", total, used, created, before+1)
	}
	if left, _ := poolOf(res.GetVitals(), rules.SorceryPointsKey); left != 2 {
		t.Errorf("sorcery points left = %d, want 2", left)
	}
	if c := byLabel(t, a.get(t, a.ana), "Nael"); !c.GetBonusActionUsed() || c.GetActionUsed() {
		t.Errorf("the bonus action used = %v and the action used = %v; want the bonus action only", c.GetBonusActionUsed(), c.GetActionUsed())
	}
	// A bonus action a turn.
	if _, err := a.createSlot(t, a.ana, e, "Nael", 1); err == nil {
		t.Error("a second creation in the same turn worked")
	} else {
		wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_BONUS_ACTION_USED)
	}
	// The slot is castable: Nael can use the 2nd-level slots he has (the sheet's and the one made).
	if total, _, _ := slotsOf(a.vitalsOfCharacter(t, a.pens), 2); total != before+1 {
		t.Errorf("the master reads %d 2nd-level slots, want %d", total, before+1)
	}
}

// Every level of the SRD table has its cost, and the refusals are typed.
func TestFlexibleCastingTableAndRefusals(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Nael": 18, "Tavo": 14, "Orla": 10}, reveal: []string{"Goblin"},
		at: map[string][2]int32{"Tavo": {3, 3}, "Nael": {4, 3}, "Orla": {5, 3}, "Goblin": {4, 4}},
	})
	// 5 points: the 4th (6) and the 5th (7) are not affordable.
	_, err := a.createSlot(t, a.ana, e, "Nael", 4)
	b := wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_NOT_ENOUGH_POINTS)
	if b.GetNeeded() != 6 || b.GetAvailable() != 5 {
		t.Errorf("needed %d, available %d; want 6 and 5", b.GetNeeded(), b.GetAvailable())
	}
	_, err = a.createSlot(t, a.ana, e, "Nael", 6)
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_SLOT_LEVEL_TOO_HIGH)
	for _, level := range []int32{0, 10, -1} {
		if _, err := a.createSlot(t, a.ana, e, "Nael", level); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("CreateSpellSlot(%d) = %v, want invalid_argument", level, err)
		}
	}
	// A refusal spends nothing, not even the bonus action.
	if c := byLabel(t, a.get(t, a.ana), "Nael"); c.GetBonusActionUsed() {
		t.Error("a refusal spent the bonus action")
	}
	if left, _ := poolOf(a.vitalsOfCharacter(t, a.pens), rules.SorceryPointsKey); left != 5 {
		t.Errorf("the points are %d after the refusals, want 5", left)
	}
	// Someone who is not a sorcerer has no Flexible Casting.
	if _, err := a.createSlot(t, a.caio, e, "Tavo", 1); err == nil {
		t.Error("a paladin made a slot")
	}
	// Not the combatant's own.
	if _, err := a.createSlot(t, a.caio, e, "Nael", 1); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("another player's CreateSpellSlot = %v, want permission_denied", err)
	}
	// Each level of the table is affordable in turn: 2, 3 and 5 points make the 1st, 2nd and 3rd.
	for level, cost := range map[int32]int32{1: 2, 2: 3, 3: 5} {
		a2 := newResourceTable(t)
		e2 := a2.start(t, plan{
			npcs: []*playv1.Participant{{CharacterId: a2.goblin.GetId()}}, npcRolls: []int{3},
			players: map[string]int32{"Nael": 18, "Tavo": 14, "Orla": 10}, reveal: []string{"Goblin"},
			at: map[string][2]int32{"Tavo": {3, 3}, "Nael": {4, 3}, "Orla": {5, 3}, "Goblin": {4, 4}},
		})
		res, err := a2.createSlot(t, a2.ana, e2, "Nael", level)
		if err != nil || res.GetCost() != cost {
			t.Errorf("CreateSpellSlot(%d) = %v, %v; want a cost of %d", level, res.GetCost(), err, cost)
		}
	}
}

// SRD 5.1, Flexible Casting: a free slot gives its level in sorcery points, up to the maximum
// (the sorcerer's level): beyond it the conversion is refused (the owner's decision).
func TestFlexibleCastingConvertsASlotIntoPointsUpToTheMaximum(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	a.correct(t, a.pens, func(r *playv1.AdjustCharacterVitalsRequest) {
		r.ResourcesUsed = []*playv1.ResourceUsed{{Key: rules.SorceryPointsKey, Used: 3}}
	})
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Nael": 18, "Tavo": 14, "Orla": 10}, reveal: []string{"Goblin"},
		at: map[string][2]int32{"Tavo": {3, 3}, "Nael": {4, 3}, "Orla": {5, 3}, "Goblin": {4, 4}},
	})
	// 2 of 5 points: a 2nd-level slot gives 2: 4 of 5.
	res, err := a.convertSlot(t, a.ana, e, "Nael", 2)
	if err != nil {
		t.Fatalf("ConvertSpellSlot() error = %v", err)
	}
	if res.GetGain() != 2 {
		t.Errorf("gain = %d, want 2", res.GetGain())
	}
	if left, _ := poolOf(res.GetVitals(), rules.SorceryPointsKey); left != 4 {
		t.Errorf("points = %d, want 4", left)
	}
	if _, used, _ := slotsOf(res.GetVitals(), 2); used != 1 {
		t.Errorf("2nd-level slots used = %d, want 1", used)
	}
	// 4 of 5: another 2nd-level slot would pass the maximum.
	e = a.turnOf(t, "Nael")
	_, err = a.convertSlot(t, a.ana, e, "Nael", 2)
	b := wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_SORCERY_POINTS_OVER)
	if b.GetNeeded() != 5 || b.GetAvailable() != 4 {
		t.Errorf("maximum %d, points %d; want 5 and 4", b.GetNeeded(), b.GetAvailable())
	}
	// A 1st-level slot reaches the maximum exactly.
	if res, err := a.convertSlot(t, a.ana, e, "Nael", 1); err != nil || res.GetGain() != 1 {
		t.Fatalf("ConvertSpellSlot(1) = %v, %v; want 1 point", res, err)
	}
	// 5 of 5: refused, whatever the slot.
	e = a.turnOf(t, "Nael")
	_, err = a.convertSlot(t, a.ana, e, "Nael", 1)
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_SORCERY_POINTS_FULL)
	// A level with no free slot: the 4th.
	_, err = a.convertSlot(t, a.ana, e, "Nael", 4)
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_NO_FREE_SLOT)
}

// A slot made and a slot converted vanish or come back with the undo; a made slot vanishes on a long rest.
func TestFlexibleCastingUndoAndTheLongRest(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Nael": 18, "Tavo": 14, "Orla": 10}, reveal: []string{"Goblin"},
		at: map[string][2]int32{"Tavo": {3, 3}, "Nael": {4, 3}, "Orla": {5, 3}, "Goblin": {4, 4}},
	})
	base, _, _ := slotsOf(a.vitalsOfCharacter(t, a.pens), 2)
	if _, err := a.createSlot(t, a.ana, e, "Nael", 2); err != nil {
		t.Fatalf("CreateSpellSlot() error = %v", err)
	}
	a.undoLast(t, e)
	v := a.vitalsOfCharacter(t, a.pens)
	if total, _, created := slotsOf(v, 2); total != base || created != 0 {
		t.Errorf("after the undo: %d slots of the 2nd level (%d made), want %d and none", total, created, base)
	}
	if left, _ := poolOf(v, rules.SorceryPointsKey); left != 5 {
		t.Errorf("after the undo the points are %d, want 5", left)
	}
	if c := byLabel(t, a.get(t, a.ana), "Nael"); c.GetBonusActionUsed() {
		t.Error("the undo did not give the bonus action back")
	}
	// A conversion, undone.
	if _, err := a.convertSlot(t, a.ana, e, "Nael", 1); err == nil {
		t.Error("a conversion at 5 of 5 worked")
	}
	a.correct(t, a.pens, func(r *playv1.AdjustCharacterVitalsRequest) {
		r.ResourcesUsed = []*playv1.ResourceUsed{{Key: rules.SorceryPointsKey, Used: 4}}
	})
	if _, err := a.convertSlot(t, a.ana, e, "Nael", 1); err != nil {
		t.Fatalf("ConvertSpellSlot() error = %v", err)
	}
	a.undoLast(t, e)
	v = a.vitalsOfCharacter(t, a.pens)
	if left, _ := poolOf(v, rules.SorceryPointsKey); left != 1 {
		t.Errorf("after the undo of a conversion the points are %d, want 1", left)
	}
	if _, used, _ := slotsOf(v, 1); used != 0 {
		t.Errorf("after the undo of a conversion %d slots of the 1st level are used, want 0", used)
	}
	// The slot made vanishes on a long rest.
	a.correct(t, a.pens, func(r *playv1.AdjustCharacterVitalsRequest) {
		r.ResourcesUsed = []*playv1.ResourceUsed{{Key: rules.SorceryPointsKey, Used: 0}}
	})
	if _, err := a.createSlot(t, a.ana, e, "Nael", 1); err != nil {
		t.Fatalf("CreateSpellSlot() error = %v", err)
	}
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	made, _, created := slotsOf(a.vitalsOfCharacter(t, a.pens), 1)
	if created != 1 {
		t.Fatalf("the slot made is not there before the rest: %d made", created)
	}
	if _, err := a.master.resource.TakeRest(t.Context(), connect.NewRequest(&playv1.TakeRestRequest{CampaignId: a.campaignID, Kind: playv1.RestKind_REST_KIND_LONG, IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("TakeRest(long) error = %v", err)
	}
	after, used, created := slotsOf(a.vitalsOfCharacter(t, a.pens), 1)
	if after != made-1 || created != 0 || used != 0 {
		t.Errorf("after a long rest: %d slots of the 1st level (%d made, %d used); want %d, none made and none used", after, created, used, made-1)
	}
}

// Bardic Inspiration (SRD 5.1, Bard 1): a creature other than the bard, within 60 feet, that
// hears; one die at a time; one use. The die is the holder's and the master's to read.
func TestBardicInspirationGivesADieToACreatureThatHearsWithinSixtyFeet(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Orla": 18, "Tavo": 14, "Nael": 10}, reveal: []string{"Goblin"},
		at: map[string][2]int32{"Orla": {3, 3}, "Tavo": {4, 3}, "Nael": {17, 3}, "Goblin": {4, 4}},
	})
	left, total := poolOf(a.vitalsOfCharacter(t, a.bri), rules.BardicInspirationKey)
	if left != 3 || total != 3 {
		t.Fatalf("Orla's uses = %d of %d, want 3 of 3 (Charisma +3)", left, total)
	}
	// The choice list: Tavo and the goblin can be chosen; Nael is 70 ft away; the bard is not listed.
	opts, err := a.bia.combat.GetTurnOptions(t.Context(), connect.NewRequest(&playv1.GetTurnOptionsRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Orla")}))
	if err != nil {
		t.Fatalf("GetTurnOptions() error = %v", err)
	}
	var voice *playv1.ResourceTargets
	for _, r := range opts.Msg.GetResourceTargets() {
		if r.GetActionKey() == bardicInspirationFt {
			voice = r
		}
	}
	if voice == nil {
		t.Fatalf("no Bardic Inspiration targets in %v", opts.Msg.GetResourceTargets())
	}
	byName := map[string]*playv1.ResourceTarget{}
	for _, r := range voice.GetTargets() {
		byName[r.GetTarget().GetLabel()] = r
	}
	if _, ok := byName["Orla"]; ok {
		t.Error("the bard is in its own list")
	}
	if byName["Tavo"] == nil || byName["Tavo"].GetDisabledReasonPt() != "" {
		t.Errorf("Tavo = %v, want a creature that can be chosen", byName["Tavo"])
	}
	if byName["Nael"] == nil || byName["Nael"].GetDisabledReasonPt() == "" {
		t.Errorf("Nael at 70 ft = %v, want it disabled with a reason", byName["Nael"])
	}
	res, err := a.giveInspiration(t, a.bia, e, "Orla", "Tavo")
	if err != nil {
		t.Fatalf("GiveBardicInspiration() error = %v", err)
	}
	if left, _ := poolOf(res.GetVitals(), rules.BardicInspirationKey); left != 2 {
		t.Errorf("uses left = %d, want 2", left)
	}
	if c := byLabel(t, a.get(t, a.bia), "Orla"); !c.GetBonusActionUsed() {
		t.Error("the bonus action is not spent")
	}
	// The die: a d8 for a level 5 bard, 100 rounds, read by the holder and the master only.
	for name, u := range map[string]*user{"master": a.master, "Caio": a.caio} {
		d := byLabel(t, a.get(t, u), "Tavo").GetInspirationDie()
		if d == nil || d.GetSides() != 8 || d.GetFromLabel() != "Orla" || d.GetExpiresAtRound() != e.GetRound()+100 {
			t.Errorf("%s reads the die %v, want a d8 from Orla that ends in round %d", name, d, e.GetRound()+100)
		}
	}
	for name, u := range map[string]*user{"Ana": a.ana, "Bia": a.bia} {
		if d := byLabel(t, a.get(t, u), "Tavo").GetInspirationDie(); d != nil {
			t.Errorf("%s reads Tavo's die %v: only the holder and the master do", name, d)
		}
	}
	// One die at a time: the same creature again is refused, and the reason shows in the list.
	e = a.turnOf(t, "Orla")
	_, err = a.giveInspiration(t, a.bia, e, "Orla", "Tavo")
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_TARGET_REFUSED)
	// Too far.
	_, err = a.giveInspiration(t, a.bia, e, "Orla", "Nael")
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_TARGET_REFUSED)
	// The bard itself.
	_, err = a.giveInspiration(t, a.bia, e, "Orla", "Orla")
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_TARGET_REFUSED)
	// One that cannot hear.
	a.setConditions(t, e, "Goblin", "condition:deafened")
	_, err = a.giveInspiration(t, a.bia, e, "Orla", "Goblin")
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_TARGET_REFUSED)
	// The line: the bard spent a use; who holds the die is the bard's, the holder's and the master's.
	for name, u := range map[string]*user{"master": a.master, "Caio": a.caio, "Bia": a.bia, "Ana": a.ana} {
		var line *playv1.CombatLogEntry
		for _, l := range logEntries(a.log(t, u, e)) {
			if l.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_RESOURCE {
				line = l
			}
		}
		if line == nil {
			t.Fatalf("%s has no line for the die", name)
		}
		knows := name == "master" || name == "Caio" || name == "Bia"
		if (line.GetTargetLabel() != "") != knows || (line.GetResource().GetDieSides() != 0) != knows {
			t.Errorf("%s reads the target %q and the die %d: the die is the bard's, the holder's and the master's alone", name, line.GetTargetLabel(), line.GetResource().GetDieSides())
		}
		if line.GetResource().GetSpent() != 1 {
			t.Errorf("%s reads %d uses spent, want 1", name, line.GetResource().GetSpent())
		}
	}
}

func (a *armed) giveInspiration(t *testing.T, u *user, e *playv1.Encounter, actor, target string) (*playv1.GiveBardicInspirationResponse, error) {
	t.Helper()
	res, err := u.resource.GiveBardicInspiration(t.Context(), connect.NewRequest(&playv1.GiveBardicInspirationRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), ActorId: a.id(t, actor), TargetId: a.id(t, target), IdempotencyKey: newKey(),
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// The die runs out after 100 rounds, the giving is undone with the use and the die, and a key repeated gives nothing twice.
func TestBardicInspirationExpiresIsUndoneAndIdempotent(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Orla": 18, "Tavo": 14, "Nael": 10}, reveal: []string{"Goblin"},
		at: map[string][2]int32{"Orla": {3, 3}, "Tavo": {4, 3}, "Nael": {5, 3}, "Goblin": {4, 4}},
	})
	key := newKey()
	req := &playv1.GiveBardicInspirationRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), ActorId: a.id(t, "Orla"), TargetId: a.id(t, "Tavo"), IdempotencyKey: key}
	if _, err := a.bia.resource.GiveBardicInspiration(t.Context(), connect.NewRequest(req)); err != nil {
		t.Fatalf("GiveBardicInspiration() error = %v", err)
	}
	if _, err := a.bia.resource.GiveBardicInspiration(t.Context(), connect.NewRequest(req)); err != nil {
		t.Fatalf("GiveBardicInspiration() retry error = %v", err)
	}
	if left, _ := poolOf(a.vitalsOfCharacter(t, a.bri), rules.BardicInspirationKey); left != 2 {
		t.Errorf("uses left = %d, want 2: the retry must not spend twice", left)
	}
	req.TargetId = a.id(t, "Nael")
	if _, err := a.bia.resource.GiveBardicInspiration(t.Context(), connect.NewRequest(req)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("the key reused for another target = %v, want invalid_argument", err)
	}
	a.undoLast(t, e)
	if left, _ := poolOf(a.vitalsOfCharacter(t, a.bri), rules.BardicInspirationKey); left != 3 {
		t.Errorf("after the undo the uses are %d, want 3", left)
	}
	if d := byLabel(t, a.get(t, a.master), "Tavo").GetInspirationDie(); d != nil {
		t.Errorf("after the undo Tavo still holds %v", d)
	}
	// A die that has run out is no die: push the round past it.
	if _, err := a.giveInspiration(t, a.bia, e, "Orla", "Tavo"); err != nil {
		t.Fatalf("GiveBardicInspiration() error = %v", err)
	}
	a.execSQL(t, `UPDATE combatants SET inspiration_expires_round = 1 WHERE id = $1`, a.id(t, "Tavo"))
	a.execSQL(t, `UPDATE encounters SET round = 2 WHERE id = $1`, e.GetId())
	if d := byLabel(t, a.get(t, a.master), "Tavo").GetInspirationDie(); d != nil {
		t.Errorf("a die that ran out in round 1 is still read in round 2: %v", d)
	}
}

// castWith calls CastSpell as u with Metamagic.
func (a *armed) castWith(t *testing.T, u *user, e *playv1.Encounter, caster, spell string, slot *playv1.SpellSlot, targets []*playv1.SpellTarget, meta ...*playv1.MetamagicChoice) (*playv1.CastSpellResponse, error) {
	t.Helper()
	req := &playv1.CastSpellRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CasterId: a.id(t, caster), SpellKey: spell, Slot: slot, Targets: targets,
		IdempotencyKey: newKey(), Metamagic: meta, Roll: &playv1.CastSpellRequest_RollInApp{RollInApp: true},
	}
	a.placeArea(t, req)
	res, err := u.combat.CastSpell(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// placeArea puts the area of a spell on the first target's square (a sphere) or toward it from
// the caster (a cone), as a placed area spell asks on a map; the targets then follow from the area.
func (a *armed) placeArea(t *testing.T, req *playv1.CastSpellRequest) {
	t.Helper()
	if len(req.GetTargets()) == 0 || (req.GetSpellKey() != fireballSpell && req.GetSpellKey() != burningHands) {
		return
	}
	var first, caster *playv1.Combatant
	for _, c := range a.get(t, a.master).GetCombatants() {
		switch c.GetId() {
		case req.GetTargets()[0].GetCombatantId():
			first = c
		case req.GetCasterId():
			caster = c
		}
	}
	if first == nil || caster == nil {
		return
	}
	if req.GetSpellKey() == fireballSpell {
		req.Area = &playv1.CastSpellRequest_Origin{Origin: &playv1.SpellOrigin{Col: first.GetCol(), Row: first.GetRow()}}
		return
	}
	req.Area = &playv1.CastSpellRequest_Direction{Direction: &playv1.SpellDirection{Dx: sign(first.GetCol() - caster.GetCol()), Dy: sign(first.GetRow() - caster.GetRow())}}
}

func sign(n int32) int32 {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	}
	return 0
}

func choice(key string) *playv1.MetamagicChoice { return &playv1.MetamagicChoice{Key: key} }

// sorcererFight starts the combat with Nael first, the goblin and the Capitão revealed.
func (a *armed) sorcererFight(t *testing.T, at map[string][2]int32) *playv1.Encounter {
	t.Helper()
	if at == nil {
		at = map[string][2]int32{"Nael": {3, 3}, "Tavo": {5, 3}, "Orla": {6, 3}, "Goblin": {4, 4}, "Capitão Goblin": {4, 6}}
	}
	return a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}, {CharacterId: a.capitao.GetId()}},
		npcRolls: []int{3, 2}, players: map[string]int32{"Nael": 20, "Tavo": 14, "Orla": 10},
		reveal: []string{"Goblin", "Capitão Goblin"}, at: at,
	})
}

func spellOptionOf(t *testing.T, o *playv1.GetTurnOptionsResponse, key string) *rulesv1.SpellOption {
	t.Helper()
	for _, s := range o.GetOptions().GetSpells() {
		if s.GetSpell().GetKey() == key {
			return s
		}
	}
	t.Fatalf("no spell %s among the options", key)
	return nil
}

func metaOption(s *rulesv1.SpellOption, key string) *rulesv1.MetamagicOption {
	for _, o := range s.GetMetamagicOptions() {
		if o.GetKey() == key {
			return o
		}
	}
	return nil
}

// SRD 5.1, Sorcerer 3: the cast sheet lists only the options the sorcerer knows, with the cost,
// and the ones the spell cannot take, with the reason.
func TestTurnOptionsListTheMetamagicTheSpellTakes(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	e := a.sorcererFight(t, nil)
	res, err := a.ana.combat.GetTurnOptions(t.Context(), connect.NewRequest(&playv1.GetTurnOptionsRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Nael")}))
	if err != nil {
		t.Fatalf("GetTurnOptions() error = %v", err)
	}
	fireball := spellOptionOf(t, res.Msg, fireballSpell)
	if len(fireball.GetMetamagicOptions()) != 2 {
		t.Fatalf("Fireball's options = %v, want the two Nael knows", fireball.GetMetamagicOptions())
	}
	if c := metaOption(fireball, rules.MetamagicCareful); c == nil || !c.GetAllowed() || c.GetCost() != 1 || c.GetNamePt() != "Magia Cuidadosa" {
		t.Errorf("Careful Spell on Fireball = %v, want allowed for 1 point", c)
	}
	if tw := metaOption(fireball, rules.MetamagicTwinned); tw == nil || tw.GetAllowed() || tw.GetCost() != 3 || tw.GetDisabledReasonPt() == "" {
		t.Errorf("Twinned Spell on Fireball = %v, want disabled (an area) and costing the spell's level, 3", tw)
	}
	hold := spellOptionOf(t, res.Msg, holdPerson)
	if tw := metaOption(hold, rules.MetamagicTwinned); tw == nil || !tw.GetAllowed() || tw.GetCost() != 2 {
		t.Errorf("Twinned Spell on Hold Person = %v, want allowed for 2 points", tw)
	}
	if c := metaOption(hold, rules.MetamagicCareful); c == nil || !c.GetAllowed() {
		t.Errorf("Careful Spell on Hold Person = %v, want allowed (it asks a saving throw)", c)
	}
	// Someone with no Metamagic gets none.
	tavo, err := a.caio.combat.GetTurnOptions(t.Context(), connect.NewRequest(&playv1.GetTurnOptionsRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Tavo")}))
	if err != nil {
		t.Fatalf("GetTurnOptions(Tavo) error = %v", err)
	}
	for _, s := range tavo.Msg.GetOptions().GetSpells() {
		if len(s.GetMetamagicOptions()) != 0 {
			t.Errorf("Tavo's spell %s has Metamagic options %v", s.GetSpell().GetKey(), s.GetMetamagicOptions())
		}
	}
}

// SRD 5.1, Careful Spell: a chosen creature automatically succeeds on its saving throw; the points are spent.
func TestCarefulSpellMakesTheChosenCreatureSaveAndTheOthersRoll(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	e := a.sorcererFight(t, nil)
	a.h.roller.queue(1, 1) // both would fail against the DC 14
	res, err := a.castWith(t, a.ana, e, "Nael", fireballSpell, slotOfLevel(3), a.at(t, "Goblin", "Capitão Goblin"),
		&playv1.MetamagicChoice{Key: rules.MetamagicCareful, CarefulIds: []string{a.id(t, "Goblin")}})
	if err != nil {
		t.Fatalf("CastSpell(careful) error = %v", err)
	}
	saved := map[string]bool{}
	for _, tr := range res.GetCast().GetTargets() {
		saved[tr.GetCombatantId()] = tr.GetSave().GetOutcome() == playv1.SaveOutcome_SAVE_OUTCOME_SAVED
	}
	if !saved[a.id(t, "Goblin")] || saved[a.id(t, "Capitão Goblin")] {
		t.Errorf("saved = %v, want the goblin to pass on its own and the Capitão to fail its d20 of 1", saved)
	}
	if got := res.GetCast().GetMetamagicKeys(); len(got) != 1 || got[0] != rules.MetamagicCareful || res.GetCast().GetSorceryPointsSpent() != 1 {
		t.Errorf("metamagic = %v, %d points; want Careful Spell and 1", got, res.GetCast().GetSorceryPointsSpent())
	}
	if left, _ := poolOf(a.vitalsOfCharacter(t, a.pens), rules.SorceryPointsKey); left != 4 {
		t.Errorf("sorcery points left = %d, want 4", left)
	}
	// The line says what was used, to everyone who gets the line.
	for name, u := range map[string]*user{"master": a.master, "Caio": a.caio} {
		var spell *playv1.CombatLogSpell
		for _, l := range logEntries(a.log(t, u, e)) {
			if l.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_SPELL_CAST {
				spell = l.GetSpell()
			}
		}
		if spell == nil || len(spell.GetMetamagicKeys()) != 1 || spell.GetSorceryPointsSpent() != 1 {
			t.Errorf("%s's line = %v, want the Metamagic and its points", name, spell)
		}
	}
}

// SRD 5.1, Twinned Spell: a spell that targets one creature can reach a second one for as many points as
// its level; the slot is spent once.
func TestTwinnedSpellReachesASecondTargetForTheSpellLevel(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	e := a.sorcererFight(t, nil)
	res, err := a.castWith(t, a.ana, e, "Nael", holdPerson, slotOfLevel(2), a.at(t, "Goblin"),
		&playv1.MetamagicChoice{Key: rules.MetamagicTwinned, TargetIds: []string{a.id(t, "Capitão Goblin")}})
	if err != nil {
		t.Fatalf("CastSpell(twinned) error = %v", err)
	}
	if len(res.GetCast().GetTargets()) != 2 || res.GetCast().GetSorceryPointsSpent() != 2 {
		t.Errorf("targets = %d, points = %d; want 2 targets and 2 points (the spell's level)", len(res.GetCast().GetTargets()), res.GetCast().GetSorceryPointsSpent())
	}
	v := a.vitalsOfCharacter(t, a.pens)
	if left, _ := poolOf(v, rules.SorceryPointsKey); left != 3 {
		t.Errorf("sorcery points left = %d, want 3", left)
	}
	if total, used, _ := slotsOf(v, 2); used != 1 || total < 1 {
		t.Errorf("2nd-level slots %d total, %d used; want exactly one slot spent", total, used)
	}
	// What the cast refuses: a second target that is the first, an area, none, two.
	e = a.turnOf(t, "Nael")
	for name, tc := range map[string]struct {
		spell   string
		slot    int32
		targets []*playv1.SpellTarget
		meta    *playv1.MetamagicChoice
	}{
		"the same target twice": {holdPerson, 2, a.at(t, "Goblin"), &playv1.MetamagicChoice{Key: rules.MetamagicTwinned, TargetIds: []string{a.id(t, "Goblin")}}},
		"no second target":      {holdPerson, 2, a.at(t, "Goblin"), &playv1.MetamagicChoice{Key: rules.MetamagicTwinned}},
		"an area spell":         {burningHands, 1, a.at(t, "Goblin"), &playv1.MetamagicChoice{Key: rules.MetamagicTwinned, TargetIds: []string{a.id(t, "Capitão Goblin")}}},
		"magic missile":         {magicMissileSpell, 1, a.at(t, "Goblin"), &playv1.MetamagicChoice{Key: rules.MetamagicTwinned, TargetIds: []string{a.id(t, "Capitão Goblin")}}},
	} {
		if _, err := a.castWith(t, a.ana, e, "Nael", tc.spell, slotOfLevel(tc.slot), tc.targets, tc.meta); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: err = %v, want invalid_argument", name, err)
		}
	}
	if left, _ := poolOf(a.vitalsOfCharacter(t, a.pens), rules.SorceryPointsKey); left != 3 {
		t.Errorf("the refusals spent points: %d left, want 3", left)
	}
}

func TestMetamagicRefusals(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	e := a.sorcererFight(t, map[string][2]int32{"Nael": {4, 9}, "Tavo": {9, 3}, "Orla": {10, 3}, "Goblin": {4, 4}, "Capitão Goblin": {9, 9}})
	goblin, captain := a.id(t, "Goblin"), a.id(t, "Capitão Goblin")
	for name, meta := range map[string][]*playv1.MetamagicChoice{
		"an option the sorcerer does not know":         {choice(rules.MetamagicSubtle)},
		"an option that is not one":                    {choice("feature:metamagic-nothing")},
		"two options that do not join":                 {{Key: rules.MetamagicCareful, CarefulIds: []string{goblin}}, {Key: rules.MetamagicTwinned, TargetIds: []string{captain}}},
		"the same option twice":                        {{Key: rules.MetamagicCareful, CarefulIds: []string{goblin}}, {Key: rules.MetamagicCareful, CarefulIds: []string{goblin}}},
		"Careful Spell with nobody":                    {choice(rules.MetamagicCareful)},
		"Careful Spell for a creature not hit":         {{Key: rules.MetamagicCareful, CarefulIds: []string{a.id(t, "Tavo")}}},
		"Careful Spell for more than Charisma":         {{Key: rules.MetamagicCareful, CarefulIds: []string{goblin, captain, a.id(t, "Tavo"), a.id(t, "Orla")}}},
		"Careful Spell for a creature protected twice": {{Key: rules.MetamagicCareful, CarefulIds: []string{goblin, goblin}}},
	} {
		if _, err := a.castWith(t, a.ana, e, "Nael", fireballSpell, slotOfLevel(3), a.at(t, "Goblin", "Capitão Goblin"), meta...); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: err = %v, want invalid_argument", name, err)
		}
	}
	// Not enough points: the master took them all.
	a.correct(t, a.pens, func(r *playv1.AdjustCharacterVitalsRequest) {
		r.ResourcesUsed = []*playv1.ResourceUsed{{Key: rules.SorceryPointsKey, Used: 5}}
	})
	_, err := a.castWith(t, a.ana, e, "Nael", fireballSpell, slotOfLevel(3), a.at(t, "Goblin"), &playv1.MetamagicChoice{Key: rules.MetamagicCareful, CarefulIds: []string{goblin}})
	b := wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_NOT_ENOUGH_POINTS)
	if b.GetNeeded() != 1 || b.GetAvailable() != 0 {
		t.Errorf("needed %d, available %d; want 1 and 0", b.GetNeeded(), b.GetAvailable())
	}
	// A refusal cast nothing: the slot is free and the action is not spent.
	v := a.vitalsOfCharacter(t, a.pens)
	if _, used, _ := slotsOf(v, 3); used != 0 {
		t.Errorf("a refused cast spent a slot of the 3rd level")
	}
	if c := byLabel(t, a.get(t, a.ana), "Nael"); c.GetActionUsed() {
		t.Error("a refused cast spent the action")
	}
	// Not a sorcerer, and an NPC.
	if _, err := a.castWith(t, a.caio, e, "Tavo", fireballSpell, slotOfLevel(3), a.at(t, "Goblin"), choice(rules.MetamagicCareful)); err == nil {
		t.Error("a paladin cast with Metamagic")
	}
}

// The master's undo of a cast gives back the points with the slot.
func TestUndoOfACastWithMetamagicGivesBackThePoints(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	e := a.sorcererFight(t, nil)
	if _, err := a.castWith(t, a.ana, e, "Nael", fireballSpell, slotOfLevel(3), a.at(t, "Goblin"), &playv1.MetamagicChoice{Key: rules.MetamagicCareful, CarefulIds: []string{a.id(t, "Goblin")}}); err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	a.undoLast(t, e)
	v := a.vitalsOfCharacter(t, a.pens)
	if left, _ := poolOf(v, rules.SorceryPointsKey); left != 5 {
		t.Errorf("after the undo the points are %d, want 5", left)
	}
	if _, used, _ := slotsOf(v, 3); used != 0 {
		t.Errorf("after the undo %d slots of the 3rd level are used, want 0", used)
	}
}

// SRD 5.1, Distant Spell and Quickened Spell, Heightened Spell and Subtle Spell, on a sorcerer that knows them.
func TestDistantQuickenedHeightenedAndSubtleSpell(t *testing.T) {
	t.Parallel()
	t.Run("Distant Spell doubles the range, and the cost is 1", func(t *testing.T) {
		t.Parallel()
		a := newResourceTableWith(t, []string{rules.MetamagicDistant, rules.MetamagicSubtle})
		// Hold Person reaches 60 ft: the goblin 80 ft away (16 squares) is out of reach, until the range doubles.
		e := a.sorcererFight(t, map[string][2]int32{"Nael": {1, 1}, "Tavo": {5, 3}, "Orla": {6, 3}, "Goblin": {17, 1}, "Capitão Goblin": {18, 1}})
		_, err := a.castWith(t, a.ana, e, "Nael", holdPerson, slotOfLevel(2), a.at(t, "Goblin"))
		wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_TARGET_OUT_OF_REACH)
		res, err := a.castWith(t, a.ana, e, "Nael", holdPerson, slotOfLevel(2), a.at(t, "Goblin"), choice(rules.MetamagicDistant))
		if err != nil {
			t.Fatalf("CastSpell(distant) error = %v", err)
		}
		if res.GetCast().GetSorceryPointsSpent() != 1 {
			t.Errorf("Distant Spell cost %d, want 1", res.GetCast().GetSorceryPointsSpent())
		}
	})
	t.Run("Quickened Spell makes the casting a bonus action, and the bonus action spell limit follows", func(t *testing.T) {
		t.Parallel()
		a := newResourceTableWith(t, []string{rules.MetamagicQuickened, rules.MetamagicSubtle})
		e := a.sorcererFight(t, nil)
		// The action goes to Dodge: a spell of 1 action cannot be cast now...
		if _, err := a.caioActs(t, a.ana, e, "Nael", "standard:dodge"); err != nil {
			t.Fatalf("TakeAction(dodge) error = %v", err)
		}
		_, err := a.castWith(t, a.ana, e, "Nael", burningHands, slotOfLevel(1), a.at(t, "Goblin"))
		wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ACTION_USED)
		// ...but Quickened Spell turns it into a bonus action.
		res, err := a.castWith(t, a.ana, e, "Nael", burningHands, slotOfLevel(1), a.at(t, "Goblin"), choice(rules.MetamagicQuickened))
		if err != nil {
			t.Fatalf("CastSpell(quickened) error = %v", err)
		}
		if res.GetCast().GetSorceryPointsSpent() != 2 {
			t.Errorf("Quickened Spell cost %d, want 2", res.GetCast().GetSorceryPointsSpent())
		}
		if c := byLabel(t, a.get(t, a.ana), "Nael"); !c.GetBonusActionUsed() {
			t.Error("the bonus action is not spent by the quickened spell")
		}
		// Another quickened spell in the turn: the bonus action is spent.
		if _, err := a.castWith(t, a.ana, e, "Nael", holdPerson, slotOfLevel(2), a.at(t, "Goblin"), choice(rules.MetamagicQuickened)); err == nil {
			t.Error("a second quickened spell worked")
		}
	})
	t.Run("Heightened Spell: disadvantage on the first saving throw", func(t *testing.T) {
		t.Parallel()
		a := newResourceTableWith(t, []string{rules.MetamagicHeightened, rules.MetamagicSubtle})
		e := a.sorcererFight(t, map[string][2]int32{"Nael": {4, 9}, "Tavo": {9, 3}, "Orla": {10, 3}, "Goblin": {4, 4}, "Capitão Goblin": {9, 9}})
		// The goblin's d20 would be 18 and pass the DC 14; with disadvantage the lower of two (2) counts.
		a.h.roller.queue(18, 2)
		res, err := a.castWith(t, a.ana, e, "Nael", fireballSpell, slotOfLevel(3), a.at(t, "Goblin"),
			&playv1.MetamagicChoice{Key: rules.MetamagicHeightened, HeightenedId: a.id(t, "Goblin")})
		if err != nil {
			t.Fatalf("CastSpell(heightened) error = %v", err)
		}
		tr := res.GetCast().GetTargets()[0]
		if tr.GetSave().GetOutcome() != playv1.SaveOutcome_SAVE_OUTCOME_FAILED {
			t.Errorf("the goblin's save = %v, want a failure: the lower d20 (2), not the 18, counts", tr.GetSave())
		}
		if res.GetCast().GetSorceryPointsSpent() != 3 {
			t.Errorf("Heightened Spell cost %d, want 3", res.GetCast().GetSorceryPointsSpent())
		}
	})
	t.Run("Subtle Spell costs a point and changes nothing the engine plays", func(t *testing.T) {
		t.Parallel()
		a := newResourceTableWith(t, []string{rules.MetamagicSubtle, rules.MetamagicEmpowered})
		e := a.sorcererFight(t, nil)
		res, err := a.castWith(t, a.ana, e, "Nael", fireballSpell, slotOfLevel(3), a.at(t, "Goblin"), choice(rules.MetamagicSubtle))
		if err != nil || res.GetCast().GetSorceryPointsSpent() != 1 {
			t.Errorf("CastSpell(subtle) = %v, %v; want 1 point", res.GetCast().GetSorceryPointsSpent(), err)
		}
		// Empowered Spell joins another option: 1 + 1.
		e = a.turnOf(t, "Nael")
		res, err = a.castWith(t, a.ana, e, "Nael", fireballSpell, slotOfLevel(3), a.at(t, "Goblin"), choice(rules.MetamagicSubtle), choice(rules.MetamagicEmpowered))
		if err != nil || res.GetCast().GetSorceryPointsSpent() != 2 {
			t.Errorf("CastSpell(subtle and empowered) = %v, %v; want 2 points", res.GetCast().GetSorceryPointsSpent(), err)
		}
	})
}

// caioActs takes a standard action as u.
func (a *armed) caioActs(t *testing.T, u *user, e *playv1.Encounter, actor, key string) (*playv1.TakeActionResponse, error) {
	t.Helper()
	res, err := u.combat.TakeAction(t.Context(), connect.NewRequest(&playv1.TakeActionRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, actor), ActionKey: key, IdempotencyKey: newKey(),
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// inspired starts the combat with Orla first and gives Tavo a die, then passes to Tavo's turn: the
// goblin (AC 12) is next to him, and his battleaxe is +6.
func (a *armed) inspired(t *testing.T) *playv1.Encounter {
	t.Helper()
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Orla": 20, "Tavo": 14, "Nael": 10}, reveal: []string{"Goblin"},
		at: map[string][2]int32{"Orla": {3, 3}, "Tavo": {4, 3}, "Nael": {5, 3}, "Goblin": {4, 4}},
	})
	if _, err := a.giveInspiration(t, a.bia, e, "Orla", "Tavo"); err != nil {
		t.Fatalf("GiveBardicInspiration() error = %v", err)
	}
	return a.turnOf(t, "Tavo")
}

func (a *armed) answer(t *testing.T, u *user, e *playv1.Encounter, holdID string, edit func(*playv1.AnswerBardicInspirationRequest)) (*playv1.AnswerBardicInspirationResponse, error) {
	t.Helper()
	req := &playv1.AnswerBardicInspirationRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), HoldId: holdID, IdempotencyKey: newKey()}
	edit(req)
	res, err := u.resource.AnswerBardicInspiration(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func useDieInApp(r *playv1.AnswerBardicInspirationRequest) {
	r.Use, r.Roll = true, &playv1.AnswerBardicInspirationRequest_RollInApp{RollInApp: true}
}

func keepTheDie(r *playv1.AnswerBardicInspirationRequest) { r.Use = false }

// SRD 5.1, Bard 1: the creature can wait until after it rolls the d20 and must decide before the GM says
// whether the roll succeeds. The attack of a character that holds a die is held until the answer: a d20
// of 5 misses the goblin's AC 12 alone (5 + 6 = 11), and hits with a d8 of 7 on top (18).
func TestABardicInspirationDieIsUsedAfterTheRollAndBeforeTheResult(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	e := a.inspired(t)
	a.h.roller.queue(5)
	held, err := a.attack(t, a.caio, e, "Tavo", battleaxe, "Goblin", inAppRoll)
	if err != nil {
		t.Fatalf("RollAttack() error = %v", err)
	}
	offer := held.GetInspirationOffer()
	if offer == nil || held.GetRoll().GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_UNSPECIFIED || held.GetPendingDamage() != nil {
		t.Fatalf("the held roll = offer %v, outcome %v, pending %v; want a question and no result", offer, held.GetRoll().GetOutcome(), held.GetPendingDamage())
	}
	if offer.GetD20().GetFaces()[0] != 5 || offer.GetD20().GetTotal() != 11 || offer.GetDie().GetSides() != 8 || offer.GetDie().GetFromLabel() != "Orla" {
		t.Errorf("offer = %v, want the d20 of 5 (total 11) and Orla's d8", offer)
	}
	// Nothing is resolved: the action is free, there is no damage, the goblin is whole, no line.
	if c := byLabel(t, a.get(t, a.caio), "Tavo"); c.GetActionUsed() {
		t.Error("the action is spent before the answer")
	}
	if cur, _, _ := a.hp(t, "Goblin"); cur != 7 {
		t.Errorf("the goblin has %d hit points before the answer, want 7", cur)
	}
	for _, l := range logEntries(a.log(t, a.master, e)) {
		if l.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK {
			t.Error("the log has the attack before the answer")
		}
	}
	// The question is on the combat for the holder and the master, and for nobody else.
	for name, u := range map[string]*user{"Caio": a.caio, "master": a.master} {
		if o := byLabel(t, a.get(t, u), "Tavo").GetInspirationOffer(); o.GetHoldId() != offer.GetHoldId() {
			t.Errorf("%s reads the question %v, want %s", name, o, offer.GetHoldId())
		}
	}
	for name, u := range map[string]*user{"Ana": a.ana, "Bia": a.bia} {
		if o := byLabel(t, a.get(t, u), "Tavo").GetInspirationOffer(); o != nil {
			t.Errorf("%s reads Tavo's question %v", name, o)
		}
	}
	// The answer: use the die; a d8 of 7 makes 18, which hits AC 12.
	a.h.roller.queue(7)
	res, err := a.answer(t, a.caio, e, offer.GetHoldId(), useDieInApp)
	if err != nil {
		t.Fatalf("AnswerBardicInspiration() error = %v", err)
	}
	roll := res.GetAttack().GetRoll()
	if roll.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT || roll.GetD20().GetTotal() != 18 || roll.GetD20().GetModifier() != 13 {
		t.Errorf("the attack = outcome %v, total %d, modifier %d; want a hit with 18 (5 + 6 + 7)", roll.GetOutcome(), roll.GetD20().GetTotal(), roll.GetD20().GetModifier())
	}
	if len(roll.GetBonusDice()) != 1 || roll.GetBonusDice()[0].GetSides() != 8 || roll.GetBonusDice()[0].GetFace() != 7 || !roll.GetBonusDice()[0].GetUsed() {
		t.Errorf("bonus dice = %v, want the d8 of 7 used", roll.GetBonusDice())
	}
	if res.GetAttack().GetPendingDamage() == nil {
		t.Error("the hit opened no damage")
	}
	if c := byLabel(t, a.get(t, a.caio), "Tavo"); !c.GetActionUsed() || c.GetInspirationDie() != nil || c.GetInspirationOffer() != nil {
		t.Errorf("after the answer: action used %v, die %v, question %v; want the action spent and the die and the question gone", c.GetActionUsed(), c.GetInspirationDie(), c.GetInspirationOffer())
	}
	// The line: the master and the attacker's player read the die; the others only the hit.
	for name, u := range map[string]*user{"master": a.master, "Caio": a.caio, "Ana": a.ana} {
		var line *playv1.CombatLogEntry
		for _, l := range logEntries(a.log(t, u, e)) {
			if l.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK {
				line = l
			}
		}
		if line == nil {
			t.Fatalf("%s has no attack line", name)
		}
		if knows := name != "Ana"; (len(line.GetBonusDice()) == 1) != knows {
			t.Errorf("%s reads bonus dice %v: the die and its face are the attacker's and the master's", name, line.GetBonusDice())
		}
	}
}

// Keeping the die: the roll goes on without it, and the die is still held.
func TestKeepingTheBardicInspirationDieResolvesTheRollWithoutIt(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	e := a.inspired(t)
	a.h.roller.queue(5)
	held, err := a.attack(t, a.caio, e, "Tavo", battleaxe, "Goblin", inAppRoll)
	if err != nil {
		t.Fatalf("RollAttack() error = %v", err)
	}
	res, err := a.answer(t, a.caio, e, held.GetInspirationOffer().GetHoldId(), keepTheDie)
	if err != nil {
		t.Fatalf("AnswerBardicInspiration() error = %v", err)
	}
	if roll := res.GetAttack().GetRoll(); roll.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_MISS || roll.GetD20().GetTotal() != 11 || len(roll.GetBonusDice()) != 0 {
		t.Errorf("the attack = %v, want a miss with 11 and no die", roll)
	}
	if c := byLabel(t, a.get(t, a.caio), "Tavo"); c.GetInspirationDie() == nil || !c.GetActionUsed() {
		t.Errorf("after keeping the die: die %v, action used %v; want the die still held and the action spent", c.GetInspirationDie(), c.GetActionUsed())
	}
}

// A real die: the d20 and the die are typed.
func TestTheBardicInspirationDieAndTheD20AreTypedWithRealDice(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	e := a.inspired(t)
	held, err := a.attack(t, a.caio, e, "Tavo", battleaxe, "Goblin", d20(5))
	if err != nil {
		t.Fatalf("RollAttack(typed) error = %v", err)
	}
	offer := held.GetInspirationOffer()
	if offer == nil || !offer.GetD20().GetPhysical() || offer.GetD20().GetFaces()[0] != 5 {
		t.Fatalf("the offer = %v, want the typed d20 of 5", offer)
	}
	for _, face := range []int32{0, 9, -2} {
		_, err := a.answer(t, a.caio, e, offer.GetHoldId(), func(r *playv1.AnswerBardicInspirationRequest) {
			r.Use, r.Roll = true, &playv1.AnswerBardicInspirationRequest_TypedFace{TypedFace: face}
		})
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("a typed face of %d = %v, want invalid_argument", face, err)
		}
	}
	if _, err := a.answer(t, a.caio, e, offer.GetHoldId(), func(r *playv1.AnswerBardicInspirationRequest) { r.Use = true }); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("using the die with no roll = %v, want invalid_argument", err)
	}
	res, err := a.answer(t, a.caio, e, offer.GetHoldId(), func(r *playv1.AnswerBardicInspirationRequest) {
		r.Use, r.Roll = true, &playv1.AnswerBardicInspirationRequest_TypedFace{TypedFace: 4}
	})
	if err != nil {
		t.Fatalf("AnswerBardicInspiration(typed) error = %v", err)
	}
	if roll := res.GetAttack().GetRoll(); roll.GetD20().GetTotal() != 15 || roll.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT {
		t.Errorf("the attack = total %d, outcome %v; want a hit with 15 (5 + 6 + 4)", roll.GetD20().GetTotal(), roll.GetOutcome())
	}
}

// One question at a time; the answer is the roll's player's or the master's; an answer repeated gives the
// same attack and rolls nothing; another key is refused; an unknown roll is not found.
func TestTheHeldRollIsAnsweredOnceByItsOwnerOrTheMaster(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	e := a.inspired(t)
	a.h.roller.queue(5)
	key := newKey()
	req := &playv1.RollAttackRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), AttackerId: a.id(t, "Tavo"), AttackKey: battleaxe, TargetId: a.id(t, "Goblin"), IdempotencyKey: key,
		Roll: &playv1.RollAttackRequest_RollInApp{RollInApp: true},
	}
	first, err := a.caio.combat.RollAttack(t.Context(), connect.NewRequest(req))
	if err != nil {
		t.Fatalf("RollAttack() error = %v", err)
	}
	// The same request again: the same question, no new d20.
	a.h.roller.queue(19)
	again, err := a.caio.combat.RollAttack(t.Context(), connect.NewRequest(req))
	if err != nil {
		t.Fatalf("RollAttack() retry error = %v", err)
	}
	if again.Msg.GetInspirationOffer().GetHoldId() != first.Msg.GetInspirationOffer().GetHoldId() || again.Msg.GetRoll().GetD20().GetFaces()[0] != 5 {
		t.Errorf("the retry = %v, want the same question and the d20 of 5", again.Msg.GetInspirationOffer())
	}
	holdID := first.Msg.GetInspirationOffer().GetHoldId()
	// Another attack while the question waits.
	_, err = a.attack(t, a.caio, e, "Tavo", battleaxe, "Goblin", inAppRoll)
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_INSPIRATION_PENDING)
	// Not another player's roll to answer; a stranger's hold is not found.
	if _, err := a.answer(t, a.ana, e, holdID, keepTheDie); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("another player's answer = %v, want permission_denied", err)
	}
	if _, err := a.answer(t, a.caio, e, newKey(), keepTheDie); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("an unknown roll = %v, want not_found", err)
	}
	// The master answers for the player.
	answerKey := newKey()
	withKey := func(r *playv1.AnswerBardicInspirationRequest) { r.Use, r.IdempotencyKey = false, answerKey }
	res, err := a.answer(t, a.master, e, holdID, withKey)
	if err != nil {
		t.Fatalf("the master's AnswerBardicInspiration() error = %v", err)
	}
	// The same answer again gives the same attack; another key finds it answered.
	a.h.roller.queue(18)
	retry, err := a.answer(t, a.master, e, holdID, withKey)
	if err != nil {
		t.Fatalf("AnswerBardicInspiration() retry error = %v", err)
	}
	if retry.GetAttack().GetRoll().GetD20().GetTotal() != res.GetAttack().GetRoll().GetD20().GetTotal() {
		t.Errorf("the retry = %v, want the first attack", retry.GetAttack().GetRoll())
	}
	if _, err := a.answer(t, a.master, e, holdID, keepTheDie); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("a second answer with another key = %v, want failed_precondition", err)
	}
	var n int
	if err := a.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM session_events WHERE kind = 'attack_rolled' AND encounter_id = $1`, e.GetId()).Scan(&n); err != nil || n != 1 {
		t.Errorf("attack events = %d (%v), want 1", n, err)
	}
}

// The master's undo of the attack gives the die back, and the player may roll again.
func TestUndoOfAnAttackThatUsedTheDieHoldsItAgain(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	e := a.inspired(t)
	a.h.roller.queue(5)
	held, err := a.attack(t, a.caio, e, "Tavo", battleaxe, "Goblin", inAppRoll)
	if err != nil {
		t.Fatalf("RollAttack() error = %v", err)
	}
	a.h.roller.queue(7)
	if _, err := a.answer(t, a.caio, e, held.GetInspirationOffer().GetHoldId(), useDieInApp); err != nil {
		t.Fatalf("AnswerBardicInspiration() error = %v", err)
	}
	a.undoLast(t, e)
	c := byLabel(t, a.get(t, a.caio), "Tavo")
	if d := c.GetInspirationDie(); d == nil || d.GetSides() != 8 || d.GetFromLabel() != "Orla" {
		t.Errorf("after the undo the die is %v, want the d8 of Orla again", d)
	}
	if c.GetActionUsed() {
		t.Error("the undo did not give the action back")
	}
	// The attack can be rolled again, with the question again.
	a.h.roller.queue(12)
	again, err := a.attack(t, a.caio, e, "Tavo", battleaxe, "Goblin", inAppRoll)
	if err != nil || again.GetInspirationOffer() == nil {
		t.Errorf("RollAttack() after the undo = %v, %v; want a new question", again.GetInspirationOffer(), err)
	}
}

// Without a die nothing changes: the attack is resolved at once.
func TestAnAttackWithoutADieIsResolvedAtOnce(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Tavo": 20, "Orla": 14, "Nael": 10}, reveal: []string{"Goblin"},
		at: map[string][2]int32{"Orla": {3, 3}, "Tavo": {4, 3}, "Nael": {5, 3}, "Goblin": {4, 4}},
	})
	a.h.roller.queue(5)
	res, err := a.attack(t, a.caio, e, "Tavo", battleaxe, "Goblin", inAppRoll)
	if err != nil {
		t.Fatalf("RollAttack() error = %v", err)
	}
	if res.GetInspirationOffer() != nil || res.GetRoll().GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_MISS {
		t.Errorf("the attack = offer %v, outcome %v; want it resolved at once as a miss", res.GetInspirationOffer(), res.GetRoll().GetOutcome())
	}
}

// RN-10: a creature the master did not reveal is not found by the resource flows, in the very words of a
// creature that does not exist: nothing says there is one.
func TestAHiddenCreatureIsNotFoundByTheResourceFlows(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	// The goblin stays hidden (not in the reveal list), next to everyone.
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Tavo": 20, "Nael": 14, "Orla": 10},
		at:      map[string][2]int32{"Tavo": {3, 3}, "Nael": {4, 3}, "Orla": {5, 3}, "Goblin": {4, 4}},
	})
	hidden := byLabel(t, a.get(t, a.master), "Goblin").GetId()
	stranger := newKey() // a combatant that never existed
	touch := func(target string) error {
		_, err := a.caio.resource.UseLayOnHands(t.Context(), connect.NewRequest(&playv1.UseLayOnHandsRequest{
			CampaignId: a.campaignID, EncounterId: e.GetId(), ActorId: a.id(t, "Tavo"), TargetId: target, IdempotencyKey: newKey(),
			Effect: &playv1.UseLayOnHandsRequest_Amount{Amount: 5},
		}))
		return err
	}
	errHidden, errNone := touch(hidden), touch(stranger)
	if connect.CodeOf(errHidden) != connect.CodeNotFound || errHidden == nil || errNone == nil || errHidden.Error() != errNone.Error() {
		t.Errorf("Lay on Hands on a hidden creature = %v, on a stranger = %v; want the same not_found", errHidden, errNone)
	}
	if left, _ := poolOf(a.vitalsOfCharacter(t, a.toren), rules.LayOnHandsKey); left != 25 {
		t.Errorf("the pool is %d after the refusals, want 25", left)
	}
	// The touch list and the voice list never carry it.
	opts, err := a.caio.combat.GetTurnOptions(t.Context(), connect.NewRequest(&playv1.GetTurnOptionsRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Tavo")}))
	if err != nil {
		t.Fatalf("GetTurnOptions() error = %v", err)
	}
	for _, rt := range opts.Msg.GetResourceTargets() {
		for _, tgt := range rt.GetTargets() {
			if tgt.GetTarget().GetCombatantId() == hidden {
				t.Errorf("the targets of %s list a hidden creature", rt.GetActionKey())
			}
		}
	}
	// Twinned Spell's second target, and Bardic Inspiration.
	e = a.turnOf(t, "Nael")
	_, errHidden = a.castWith(t, a.ana, e, "Nael", holdPerson, slotOfLevel(2), a.at(t, "Tavo"),
		&playv1.MetamagicChoice{Key: rules.MetamagicTwinned, TargetIds: []string{hidden}})
	_, errNone = a.castWith(t, a.ana, e, "Nael", holdPerson, slotOfLevel(2), a.at(t, "Tavo"),
		&playv1.MetamagicChoice{Key: rules.MetamagicTwinned, TargetIds: []string{stranger}})
	if connect.CodeOf(errHidden) != connect.CodeNotFound || errHidden == nil || errNone == nil || errHidden.Error() != errNone.Error() {
		t.Errorf("Twinned Spell on a hidden creature = %v, on a stranger = %v; want the same not_found", errHidden, errNone)
	}
	e = a.turnOf(t, "Orla")
	give := func(target string) error {
		_, err := a.bia.resource.GiveBardicInspiration(t.Context(), connect.NewRequest(&playv1.GiveBardicInspirationRequest{
			CampaignId: a.campaignID, EncounterId: e.GetId(), ActorId: a.id(t, "Orla"), TargetId: target, IdempotencyKey: newKey(),
		}))
		return err
	}
	errHidden, errNone = give(hidden), give(stranger)
	if connect.CodeOf(errHidden) != connect.CodeNotFound || errHidden == nil || errNone == nil || errHidden.Error() != errNone.Error() {
		t.Errorf("Bardic Inspiration on a hidden creature = %v, on a stranger = %v; want the same not_found", errHidden, errNone)
	}
}

// The line of a touch that heals a creature the master hid again is the master's alone.
func TestALayOnHandsLineWithAHiddenCombatantNeverReachesAPlayer(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	e := a.resourceFight(t)
	if _, err := a.layOnHands(t, a.master, e, "Tavo", "Goblin", healAmount(3)); err != nil {
		t.Fatalf("UseLayOnHands() error = %v", err)
	}
	if _, err := a.master.combat.SetCombatantHidden(t.Context(), connect.NewRequest(&playv1.SetCombatantHiddenRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Goblin"), IdempotencyKey: newKey(), Hidden: true,
	})); err != nil {
		t.Fatalf("SetCombatantHidden() error = %v", err)
	}
	for name, u := range map[string]*user{"Caio": a.caio, "Ana": a.ana, "Bia": a.bia} {
		for _, l := range logEntries(a.log(t, u, e)) {
			if l.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_RESOURCE {
				t.Errorf("%s reads a line of a touch on a creature the master hid", name)
			}
		}
	}
	found := false
	for _, l := range logEntries(a.log(t, a.master, e)) {
		found = found || l.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_RESOURCE
	}
	if !found {
		t.Error("the master does not read the line either")
	}
}
