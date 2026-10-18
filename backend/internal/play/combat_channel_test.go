package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// Channel Divinity (SRD 5.1, Cleric). Preserve Life restores up to five times the cleric's level among the creatures
// within 30 ft, each to no more than half of its maximum; Turn Undead makes each undead within 30 ft that can see or
// hear the cleric save against the cleric's DC, and turns the ones that fail.

const (
	preserveLife = "feature:channel-divinity-preserve-life"
	turnUndead   = "feature:channel-divinity-turn-undead"
)

// channelTable is a Life cleric of level 5 (Brisa: Wisdom 16, proficiency +3, so DC 14 and 25 points to give), a
// fighter (Toren) and a wizard (Pensantus), and the zombies asked for, at the squares given by label. Brisa has the
// first turn.
func channelTable(t *testing.T, zombies map[string][2]int32) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 5,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, nil, nil)
		sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 14, Intelligence: 10, Wisdom: 16, Charisma: 8}, RaceKey: "race:human",
			Classes:    []*charactersv1.ClassLevel{classLevel("class:cleric", 5, "subclass:life")},
			WeaponKeys: []string{maceKey}, CantripKeys: []string{sacredFlame}, PreparedSpellKeys: []string{cureWounds, guidingBolt},
		}}}
		res, err := a.bia.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
			CampaignId: a.campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Brisa", Sheet: sheet,
		}))
		if err != nil {
			t.Fatalf("CreateCharacter(Brisa) error = %v", err)
		}
		a.bri = res.Msg.GetCharacter()
	})
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Brisa": 18, "Toren": 14, "Pensantus": 10}, reveal: []string{"Goblin"}, setup: true,
		at: map[string][2]int32{"Brisa": {3, 3}, "Toren": {4, 3}, "Pensantus": {5, 3}, "Goblin": {19, 9}},
	})
	for name := range zombies {
		a.h.roller.queue(10)
		e = a.mustAddMonsters(t, e, func(r *playv1.AddMonstersRequest) {
			r.CreatureKey, r.Count, r.Name, r.Hidden = "monster:zombie", 1, name, new(false)
		}).GetEncounter()
	}
	for name, sq := range zombies {
		if _, err := a.master.combat.MoveCombatant(t.Context(), connect.NewRequest(&playv1.MoveCombatantRequest{
			CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, name), IdempotencyKey: newKey(), Col: sq[0], Row: sq[1],
		})); err != nil {
			t.Fatalf("place %s: %v", name, err)
		}
	}
	if _, err := a.master.combat.BeginCombat(t.Context(), connect.NewRequest(&playv1.BeginCombatRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("BeginCombat() error = %v", err)
	}
	e = a.passTo(t, a.get(t, a.master), "Brisa")
	return a, e
}

func preserve(t *testing.T, a *armed, who map[string]int32) func(*playv1.TakeActionRequest) {
	t.Helper()
	return func(r *playv1.TakeActionRequest) {
		for label, hp := range who {
			r.PreserveLife = append(r.PreserveLife, &playv1.PreserveLifeHeal{CombatantId: a.id(t, label), HitPoints: hp})
		}
	}
}

func hitPointsOf(t *testing.T, a *armed, label string) int32 {
	t.Helper()
	return byLabel(t, a.get(t, a.master), label).GetHitPointsCurrent()
}

// TestPreserveLifeRestoresNoFurtherThanHalfTheMaximum: Toren is at 5 of 49 and Pensantus at 3 of 31; the cleric
// asks for 20 and 5. Toren's limit is half his maximum (24), so he regains 19; Pensantus regains his 5.
func TestPreserveLifeRestoresNoFurtherThanHalfTheMaximum(t *testing.T) {
	t.Parallel()
	a, e := channelTable(t, nil)
	a.hurt(t, a.toren, 5)
	a.hurt(t, a.pens, 3)
	toren, pens := byLabel(t, a.get(t, a.master), "Toren"), byLabel(t, a.get(t, a.master), "Pensantus")
	res, err := a.takeActionRaw(t, a.bia, e, "Brisa", preserveLife, preserve(t, a, map[string]int32{"Toren": 20, "Pensantus": 5}))
	if err != nil {
		t.Fatalf("TakeAction(Preserve Life) error = %v", err)
	}
	want := map[string][2]int32{"Toren": {20, toren.GetHitPointsMax()/2 - 5}, "Pensantus": {5, 5}}
	got := res.GetChannelDivinity()
	if len(got.GetHeals()) != 2 {
		t.Fatalf("heals = %v, want two", got.GetHeals())
	}
	for _, h := range got.GetHeals() {
		w := want[h.GetLabel()]
		if h.GetAsked() != w[0] || h.GetHealed() != w[1] {
			t.Errorf("%s: asked %d healed %d, want %d and %d", h.GetLabel(), h.GetAsked(), h.GetHealed(), w[0], w[1])
		}
	}
	if hp := hitPointsOf(t, a, "Toren"); hp != toren.GetHitPointsMax()/2 {
		t.Errorf("Toren is at %d, want half of %d", hp, toren.GetHitPointsMax())
	}
	if hp := hitPointsOf(t, a, "Pensantus"); hp != 8 || pens.GetHitPointsMax() < 16 {
		t.Errorf("Pensantus is at %d, want 8", hp)
	}
	cleric := byLabel(t, a.get(t, a.master), "Brisa")
	if !cleric.GetActionUsed() {
		t.Error("the action was not spent")
	}
}

// TestPreserveLifeIsRefusedBeyondItsLimits: more than five times the level in all, or an undead.
func TestPreserveLifeIsRefusedBeyondItsLimits(t *testing.T) {
	t.Parallel()
	a, e := channelTable(t, map[string][2]int32{"Morto": {4, 4}, "Morto Longe": {18, 8}})
	a.hurt(t, a.toren, 5)
	_, err := a.takeActionRaw(t, a.bia, e, "Brisa", preserveLife, preserve(t, a, map[string]int32{"Toren": 26}))
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_CHANNEL_DIVINITY_TOO_MUCH)
	_, err = a.takeActionRaw(t, a.bia, e, "Brisa", preserveLife, preserve(t, a, map[string]int32{"Morto": 5}))
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_CHANNEL_DIVINITY_TARGET)
	if hp := hitPointsOf(t, a, "Toren"); hp != 5 {
		t.Errorf("Toren is at %d after the refusals, want 5", hp)
	}
	if byLabel(t, a.get(t, a.master), "Brisa").GetActionUsed() {
		t.Error("a refused Preserve Life spent the action")
	}
}

// TestPreserveLifeWithNoOneNamedHealsTheHurtOnesOfTheClericsSide: the 25 points go to the creatures hurt the most, each
// as far as half its maximum, and never to the other side.
func TestPreserveLifeWithNoOneNamedHealsTheHurtOnesOfTheClericsSide(t *testing.T) {
	t.Parallel()
	a, e := channelTable(t, nil)
	a.hurt(t, a.toren, 5)
	a.hurt(t, a.pens, 3)
	res, err := a.takeActionRaw(t, a.bia, e, "Brisa", preserveLife, nil)
	if err != nil {
		t.Fatalf("TakeAction(Preserve Life) error = %v", err)
	}
	var given int32
	for _, h := range res.GetChannelDivinity().GetHeals() {
		given += h.GetHealed()
		c := byLabel(t, a.get(t, a.master), h.GetLabel())
		if h.GetHitPointsAfter() > c.GetHitPointsMax()/2 || c.GetKind() != playv1.CombatantKind_COMBATANT_KIND_PLAYER {
			t.Errorf("%s ended at %d of %d (kind %v), want a player at most at half", h.GetLabel(), h.GetHitPointsAfter(), c.GetHitPointsMax(), c.GetKind())
		}
	}
	if given != 25 {
		t.Errorf("%d hit points given, want all 25", given)
	}
}

// TestPreserveLifeIsLoggedForTheMasterAndUndone: the master and the cleric's player read what it gave in the log,
// another player reads nothing of the numbers, and the undo gives the hit points, the action and the use back.
func TestPreserveLifeIsLoggedForTheMasterAndUndone(t *testing.T) {
	t.Parallel()
	a, e := channelTable(t, nil)
	a.hurt(t, a.toren, 5)
	a.undoes(t, "Preserve Life", func() {
		if _, err := a.takeActionRaw(t, a.bia, e, "Brisa", preserveLife, preserve(t, a, map[string]int32{"Toren": 10})); err != nil {
			t.Fatalf("TakeAction(Preserve Life) error = %v", err)
		}
		e2 := a.get(t, a.master)
		seen := func(u *user) bool {
			for _, l := range logEntries(a.log(t, u, e2)) {
				if len(l.GetChannelDivinity().GetHeals()) == 1 {
					return true
				}
			}
			return false
		}
		if !seen(a.master) || !seen(a.bia) || seen(a.caio) {
			t.Errorf("the log shows the heals: master %v, cleric's player %v, another player %v; want true, true, false", seen(a.master), seen(a.bia), seen(a.caio))
		}
	})
}

// TestTurnUndeadTurnsTheUndeadThatFail: three zombies, one passes the save, one fails, one is too far; only the one
// that failed is turned (a lasting effect on it), the far one does not even save, and the master reads each save.
func TestTurnUndeadTurnsTheUndeadThatFail(t *testing.T) {
	t.Parallel()
	a, e := channelTable(t, map[string][2]int32{"Morto A": {4, 4}, "Morto B": {2, 3}, "Morto Longe": {18, 8}})
	a.h.roller.queue(3, 19) // the two near ones, in the order of the combat
	res, err := a.takeActionRaw(t, a.bia, e, "Brisa", turnUndead, nil)
	if err != nil {
		t.Fatalf("TakeAction(Turn Undead) error = %v", err)
	}
	got := res.GetChannelDivinity()
	if got.GetSaveDc() != 14 || len(got.GetTurns()) != 2 {
		t.Fatalf("channel = %v, want DC 14 and two saves", got)
	}
	turned := 0
	for _, tn := range got.GetTurns() {
		if tn.GetCombatantId() == a.id(t, "Morto Longe") {
			t.Errorf("the far zombie saved: %v", tn)
		}
		if tn.GetTurned() != (tn.GetTotal() < 14) {
			t.Errorf("%s: total %d, turned %v, want turned exactly when below the DC 14", tn.GetLabel(), tn.GetTotal(), tn.GetTurned())
		}
		if tn.GetTurned() {
			turned++
			if !hasLabeledEffect(byLabel(t, a.get(t, a.master), tn.GetLabel()), "effect:turned") {
				t.Errorf("%s is turned and carries no effect of it", tn.GetLabel())
			}
		}
	}
	if turned != 1 {
		t.Errorf("%d turned, want 1 (one low roll, one high)", turned)
	}
	if !byLabel(t, a.get(t, a.master), "Brisa").GetActionUsed() {
		t.Error("the action was not spent")
	}
}

// TestTurnUndeadIsUndone: the undo takes the turned effect away, and the action and the use come back.
func TestTurnUndeadIsUndone(t *testing.T) {
	t.Parallel()
	a, e := channelTable(t, map[string][2]int32{"Morto": {4, 4}})
	a.undoes(t, "Turn Undead", func() {
		a.h.roller.queue(3)
		res, err := a.takeActionRaw(t, a.bia, e, "Brisa", turnUndead, nil)
		if err != nil || len(res.GetChannelDivinity().GetTurns()) != 1 || !res.GetChannelDivinity().GetTurns()[0].GetTurned() {
			t.Fatalf("TakeAction(Turn Undead) = %v, %v; want the zombie turned", res, err)
		}
	})
}

// hasLabeledEffect says the combatant carries a lasting effect with the source key.
func hasLabeledEffect(c *playv1.Combatant, key string) bool {
	for _, ef := range c.GetEffects() {
		if ef.GetSourceKey() == key {
			return true
		}
	}
	return false
}
