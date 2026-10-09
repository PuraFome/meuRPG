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
	max := a.vitalsOfCharacter(t, a.pens).GetHitPointsMax()
	a.correct(t, a.pens, func(r *playv1.AdjustCharacterVitalsRequest) { r.HitPointsCurrent = proto.Int32(max - 2) })
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
	if v := a.vitalsOfCharacter(t, a.pens); v.GetHitPointsCurrent() != max {
		t.Errorf("Nael has %d of %d hit points, want the maximum", v.GetHitPointsCurrent(), max)
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
	var seen = map[string]*playv1.CombatLogResource{}
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
