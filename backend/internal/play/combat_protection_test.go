package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The Protection fighting style (SRD 5.1, Fighter): the reaction of a shield-bearer that
// sees a creature attack a neighbor of its own, which gives the attack roll disadvantage.

const protectionStyleKey = "feature:fighter-fighting-style-protection"

// protectorOpts is Reator, the fighter of Ana's that may protect Toren.
type protectorOpts struct {
	style   bool
	shield  bool
	weapons []string
	at      [2]int32
	// hideGoblin leaves the Goblin hidden from the players.
	hideGoblin bool
}

// protectorAdjacent is where Reator stands next to Toren (3,3).
var protectorAdjacent = [2]int32{4, 4}

func (o protectorOpts) sheet() *charactersv1.CharacterSheet {
	var choices []string
	if o.style {
		choices = []string{protectionStyleKey}
	}
	return &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores: abilities(16, 13, 14, 10, 8), RaceKey: "race:human", Classes: []*charactersv1.ClassLevel{classLevel("class:fighter", 5, "")},
		ArmorKey: chainMailKey, Shield: o.shield, WeaponKeys: o.weapons, FeatureChoiceKeys: choices,
	}}}
}

// protectionFight is Toren (3,3), the Goblin next to him (4,3) and Reator at o.at, on the
// Goblin's turn.
func protectionFight(t *testing.T, o protectorOpts) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5, abilities(16, 13, 14, 10, 8), []string{battleaxe}, nil)
		res, err := a.ana.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
			CampaignId: a.campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Reator", Sheet: o.sheet(),
		}))
		if err != nil {
			t.Fatalf("CreateCharacter(Reator) error = %v", err)
		}
		a.pens = res.Msg.GetCharacter()
		a.bri = a.bia.caster(t, a.campaignID, "Brisa", "class:cleric", "race:human", 3, abilities(10, 16, 14, 16, 8), []string{maceKey}, []string{sacredFlame}, nil, []string{cureWounds})
	})
	var reveal []string
	if !o.hideGoblin {
		reveal = []string{"Goblin"}
	}
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Toren": 18, "Reator": 10, "Brisa": 1},
		reveal:   reveal,
		at:       map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Reator": o.at, "Brisa": {8, 8}},
	})
	return a, a.passTo(t, e, "Goblin")
}

// protector is the usual Reator: the style, a shield and a one-handed weapon, next to Toren.
func protector() protectorOpts {
	return protectorOpts{style: true, shield: true, weapons: []string{longswordKey}, at: protectorAdjacent}
}

// protectionWindow is the window Ana reads, or nil.
func (a *armed) protectionWindow(t *testing.T) *playv1.ReactionWindow {
	t.Helper()
	return a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_PROTECTION)
}

// lastAttackEntry is the newest attack line of the master's log.
func (a *armed) lastAttackEntry(t *testing.T, e *playv1.Encounter) *playv1.CombatLogEntry {
	t.Helper()
	var last *playv1.CombatLogEntry
	for _, en := range logEntries(a.log(t, a.master, e)) {
		if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK {
			last = en
		}
	}
	if last == nil {
		t.Fatal("the log has no attack line")
	}
	return last
}

// TestProtectionImposesDisadvantageOnTheAttackRoll: the Goblin's attack on Toren waits for
// Reator, who stands next to Toren with a shield; using the reaction gives the roll disadvantage
// (the typed die is joined by one the app rolls, and the lower counts), spends the reaction and
// leaves no window. Reator's player reads who attacks and who is protected, never the master's
// numbers.
func TestProtectionImposesDisadvantageOnTheAttackRoll(t *testing.T) {
	t.Parallel()
	a, e := protectionFight(t, protector())
	held := a.mustAttack(t, a.master, e, "Goblin", sword, "Toren", d20(15)) // 15 + 4 = 19 against Toren's AC
	if !held.GetRoll().GetHeldForReaction() || held.GetPendingDamage() != nil {
		t.Fatalf("the attack = %v, want it held for Reator", held)
	}
	w := a.protectionWindow(t)
	if w == nil || w.GetProtection().GetAttackerLabel() != "Goblin" || w.GetProtection().GetAllyLabel() != "Toren" {
		t.Fatalf("Reator's window = %v, want Protection of Toren against the Goblin", w)
	}
	if w.GetTrigger() != nil {
		t.Errorf("the player reads the master's trigger numbers: %v", w.GetTrigger())
	}
	if a.windowOf(t, a.caio, playv1.ReactionKind_REACTION_KIND_PROTECTION) != nil {
		t.Error("Toren's player reads Reator's window")
	}
	if a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_PROTECTION) == nil {
		t.Error("the master does not read the window")
	}
	a.h.roller.queue(4) // the second d20
	res := a.mustAnswer(t, a.ana, e, w.GetId(), useAnswer(0))
	if r := res.GetResult(); r.GetKind() != playv1.ReactionKind_REACTION_KIND_PROTECTION || !r.GetUsed() || r.GetProtection() == nil {
		t.Errorf("the result = %v, want Protection used", r)
	}
	if !byLabel(t, res.GetEncounter(), "Reator").GetReactionUsed() {
		t.Error("Reator's reaction is not used")
	}
	if n := len(res.GetEncounter().GetReactionWindows()); n != 0 {
		t.Errorf("windows left: %d", n)
	}
	en := a.lastAttackEntry(t, e)
	if f := en.GetAttackRoll().GetFaces(); len(f) != 2 || f[0] != 15 || f[1] != 4 || en.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_MISS {
		t.Errorf("the attack = faces %v, outcome %v, want 15 and 4, a miss (the lower die counts)", f, en.GetOutcome())
	}
}

// TestProtectionPassedLeavesTheRollAlone: "Deixar passar" rolls the attack as it was typed.
func TestProtectionPassedLeavesTheRollAlone(t *testing.T) {
	t.Parallel()
	a, e := protectionFight(t, protector())
	a.mustAttack(t, a.master, e, "Goblin", sword, "Toren", d20(15))
	w := a.protectionWindow(t)
	if w == nil {
		t.Fatal("no window")
	}
	res := a.mustAnswer(t, a.ana, e, w.GetId(), passAnswer)
	if byLabel(t, res.GetEncounter(), "Reator").GetReactionUsed() {
		t.Error("passing spent the reaction")
	}
	if en := a.lastAttackEntry(t, e); len(en.GetAttackRoll().GetFaces()) != 1 || en.GetAttackRoll().GetFaces()[0] != 15 {
		t.Errorf("the attack = %v, want the typed 15 alone", en.GetAttackRoll().GetFaces())
	}
}

// TestProtectionIsOfferedOnlyWhenTheRuleHolds: each case is one thing missing from the style's
// rule (the style, the shield in hand, a free reaction, sight of the attacker, five feet to the
// target, a target that is not the reactor, a conscious reactor) and opens no window.
func TestProtectionIsOfferedOnlyWhenTheRuleHolds(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(o *protectorOpts)
		target string // the label the Goblin attacks, Toren when empty
		setup  func(t *testing.T, a *armed, e *playv1.Encounter)
	}{
		{name: "without the style", mutate: func(o *protectorOpts) { o.style = false }},
		{name: "without a shield", mutate: func(o *protectorOpts) { o.shield = false }},
		{name: "with a two-handed weapon", mutate: func(o *protectorOpts) { o.weapons = []string{greatswordKey} }},
		{name: "beyond five feet of the target", mutate: func(o *protectorOpts) { o.at = [2]int32{6, 6} }},
		{name: "the target is the reactor", target: "Reator"},
		{name: "an attacker hidden from the players", mutate: func(o *protectorOpts) { o.hideGoblin = true }},
		{name: "a blinded reactor", setup: func(t *testing.T, a *armed, e *playv1.Encounter) {
			a.setConditions(t, e, "Reator", "condition:blinded")
		}},
		{name: "an incapacitated reactor", setup: func(t *testing.T, a *armed, e *playv1.Encounter) {
			a.setConditions(t, e, "Reator", "condition:incapacitated")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			o := protector()
			if tt.mutate != nil {
				tt.mutate(&o)
			}
			a, e := protectionFight(t, o)
			if tt.setup != nil {
				tt.setup(t, a, e)
			}
			target := tt.target
			if target == "" {
				target = "Toren"
			}
			held := a.mustAttack(t, a.master, e, "Goblin", sword, target, d20(15))
			if a.protectionWindow(t) != nil || a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_PROTECTION) != nil || held.GetRoll().GetHeldForReaction() {
				t.Errorf("the attack = %v: Protection was offered", held.GetRoll())
			}
		})
	}
}

// TestProtectionIsNotOfferedForAnAttackOnAnEnemy: Toren's attack on the Goblin, which is
// next to Reator, is not an attack on a neighbor Reator protects.
func TestProtectionIsNotOfferedForAnAttackOnAnEnemy(t *testing.T) {
	t.Parallel()
	a, e := protectionFight(t, protector())
	e = a.passTo(t, e, "Toren")
	held := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", d20(15))
	if held.GetRoll().GetHeldForReaction() || a.protectionWindow(t) != nil {
		t.Errorf("Toren's attack = %v: Protection was offered against an enemy of Reator", held.GetRoll())
	}
}

// TestProtectionSpendsTheReactionOncePerRound: a second attack of the same turn finds Reator's
// reaction spent.
func TestProtectionSpendsTheReactionOncePerRound(t *testing.T) {
	t.Parallel()
	a, e := protectionFight(t, protector())
	a.mustAttack(t, a.master, e, "Goblin", sword, "Toren", d20(15))
	a.h.roller.queue(4)
	a.mustAnswer(t, a.ana, e, a.protectionWindow(t).GetId(), useAnswer(0))
	again := a.mustAttack(t, a.master, e, "Goblin", sword, "Toren", d20(15))
	if again.GetRoll().GetHeldForReaction() || a.protectionWindow(t) != nil {
		t.Errorf("the second attack = %v: Protection was offered with the reaction spent", again.GetRoll())
	}
}

// TestUndoTakesProtectionBackWithTheAttack: the attack the reaction released is undone with
// the reaction it spent, so the master can make the attack again and Reator is asked again.
func TestUndoTakesProtectionBackWithTheAttack(t *testing.T) {
	t.Parallel()
	a, e := protectionFight(t, protector())
	a.mustAttack(t, a.master, e, "Goblin", sword, "Toren", d20(15))
	a.h.roller.queue(4)
	a.mustAnswer(t, a.ana, e, a.protectionWindow(t).GetId(), useAnswer(0))
	if err := a.undo(t, a.master, e, a.log(t, a.master, e).GetUndoableEventId()); err != nil {
		t.Fatalf("UndoLastAction() error = %v", err)
	}
	if byLabel(t, a.get(t, a.master), "Reator").GetReactionUsed() {
		t.Error("Reator's reaction is still spent after the undo")
	}
	again := a.mustAttack(t, a.master, e, "Goblin", sword, "Toren", d20(15))
	if !again.GetRoll().GetHeldForReaction() || a.protectionWindow(t) == nil {
		t.Errorf("the attack made again = %v, want it held for Reator again", again.GetRoll())
	}
}
