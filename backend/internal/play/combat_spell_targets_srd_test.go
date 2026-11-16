package play

import (
	"fmt"
	"slices"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// Whom an SRD spell takes for a player the number of creatures, how close
// they must be to the first, and the spells that name no creature at all. These tests
// need the database (MEURPG_TEST_DATABASE_URL).

const (
	acidSplash     = "spell:acid-splash"
	waterBreathing = "spell:water-breathing"
	massCureWounds = "spell:mass-cure-wounds"
	chainLightning = "spell:chain-lightning"
	disintegrate   = "spell:disintegrate"
	wallOfIce      = "spell:wall-of-ice"
	slowSpell      = "spell:slow"
	seemingSpell   = "spell:seeming"
)

// newSrdTargets is a party of three level 17 casters (Mago the wizard, Clerigo the
// cleric, Druida the druid) against nGoblins goblins and the Capitão. The goblins stand
// in two columns of five squares, 20 ft from the first to the last; the Capitão stands
// 50 ft or more away. first plays first.
func newSrdTargets(t *testing.T, first string, nGoblins int32) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newSrdParty(t)
	e := a.srdFight(t, first, nGoblins)
	return a, e
}

// newSrdParty is the party of newSrdTargets with no combat yet.
func newSrdParty(t *testing.T) *armed {
	t.Helper()
	wizardSpells := []string{
		"spell:fire-shield", "spell:dimension-door", waterBreathing, chainLightning, disintegrate, wallOfIce, slowSpell, seemingSpell, "spell:animate-objects", "spell:passwall",
		"spell:wall-of-force", "spell:wall-of-stone", "spell:stone-shape", "spell:secret-chest",
	}
	return newArmedWith(t, func(a *armed) {
		a.pens = a.ana.caster(t, a.campaignID, "Mago", "class:wizard", "race:gnome", 17,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 20, Wisdom: 10, Charisma: 8}, nil,
			[]string{acidSplash, "spell:mage-hand", "spell:mending", "spell:prestidigitation"},
			wizardSpells, wizardSpells)
		a.bri = a.bia.caster(t, a.campaignID, "Clerigo", "class:cleric", "race:human", 17,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 20, Charisma: 8}, []string{maceKey},
			[]string{"spell:thaumaturgy"}, nil, []string{massCureWounds})
		a.toren = a.caio.caster(t, a.campaignID, "Druida", "class:druid", "race:human", 17,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 14, Intelligence: 10, Wisdom: 20, Charisma: 8}, nil,
			[]string{"spell:shillelagh"}, nil, []string{"spell:giant-insect"})
	})
}

// srdFight starts the combat of newSrdTargets.
func (a *armed) srdFight(t *testing.T, first string, nGoblins int32) *playv1.Encounter {
	t.Helper()
	at := map[string][2]int32{"Mago": {1, 4}, "Clerigo": {1, 5}, "Druida": {1, 6}, "Capitão Goblin": {14, 8}}
	reveal := []string{"Capitão Goblin"}
	rolls := []int{1}
	for i := range nGoblins {
		label := fmt.Sprintf("Goblin %d", i+1)
		reveal, at[label] = append(reveal, label), [2]int32{4 + i/5, 1 + i%5}
		rolls = append(rolls, 1)
	}
	players := map[string]int32{"Mago": 10, "Clerigo": 10, "Druida": 10}
	players[first] = 20
	return a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId(), Count: nGoblins}, {CharacterId: a.capitao.GetId()}}, npcRolls: rolls,
		reveal: reveal, at: at, players: players,
	})
}

// goblins lists n goblins by label, from the first.
func goblins(n int) []string {
	out := make([]string, n)
	for i := range n {
		out[i] = fmt.Sprintf("Goblin %d", i+1)
	}
	return out
}

// wantTooMany: a player's cast with more targets than the spell takes is refused.
func (a *armed) wantTooMany(t *testing.T, e *playv1.Encounter, caster, spell string, slot *playv1.SpellSlot, labels []string) {
	t.Helper()
	_, err := a.cast(t, a.userOf(caster), e, caster, spell, slot, a.at(t, labels...), noCastRoll)
	wantCode(t, fmt.Sprintf("%s with %d targets", spell, len(labels)), err, connect.CodeInvalidArgument)
}

// userOf is the player of a caster of this file.
func (a *armed) userOf(caster string) *user {
	switch caster {
	case "Clerigo":
		return a.bia
	case "Druida":
		return a.caio
	}
	return a.ana
}

// takes casts a spell for the player with the labels as targets and checks the cast holds exactly them.
func (a *armed) takes(t *testing.T, e *playv1.Encounter, caster, spell string, slot *playv1.SpellSlot, labels []string) *playv1.SpellCast {
	t.Helper()
	res := a.mustCast(t, a.userOf(caster), e, caster, spell, slot, a.at(t, labels...), noCastRoll).GetCast()
	if len(res.GetTargets()) != len(labels) {
		t.Fatalf("%s hit %d targets, want %d", spell, len(res.GetTargets()), len(labels))
	}
	return res
}

// TestSRDSpellTargetCounts: how many creatures a player may name for each spell the SRD limits.
func TestSRDSpellTargetCounts(t *testing.T) {
	t.Parallel()
	t.Run("Acid Splash takes one", func(t *testing.T) {
		t.Parallel()
		a, e := newSrdTargets(t, "Mago", 4)
		a.takes(t, e, "Mago", acidSplash, nil, goblins(1))
	})
	t.Run("Acid Splash takes two within 5 ft of each other", func(t *testing.T) {
		t.Parallel()
		a, e := newSrdTargets(t, "Mago", 4)
		a.wantTooMany(t, e, "Mago", acidSplash, nil, goblins(3))
		a.takes(t, e, "Mago", acidSplash, nil, goblins(2))
	})
	t.Run("Acid Splash refuses two 10 ft apart", func(t *testing.T) {
		t.Parallel()
		a, e := newSrdTargets(t, "Mago", 4)
		_, err := a.cast(t, a.ana, e, "Mago", acidSplash, nil, a.at(t, "Goblin 1", "Goblin 3"), noCastRoll)
		b := wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_TARGET_OUT_OF_REACH)
		if b.GetMissingFt() != 5 {
			t.Errorf("missing %d ft, want 5", b.GetMissingFt())
		}
	})
	t.Run("Water Breathing takes ten", func(t *testing.T) {
		t.Parallel()
		a, e := newSrdTargets(t, "Mago", 10)
		a.takes(t, e, "Mago", waterBreathing, slotOfLevel(3), goblins(10))
	})
	t.Run("Slow takes six and refuses a seventh", func(t *testing.T) {
		t.Parallel()
		a, e := newSrdTargets(t, "Mago", 8)
		a.wantTooMany(t, e, "Mago", slowSpell, slotOfLevel(3), goblins(7))
		a.takes(t, e, "Mago", slowSpell, slotOfLevel(3), goblins(6))
	})
	t.Run("Seeming takes ten", func(t *testing.T) {
		t.Parallel()
		a, e := newSrdTargets(t, "Mago", 10)
		a.takes(t, e, "Mago", seemingSpell, slotOfLevel(5), goblins(10))
	})
	t.Run("Disintegrate takes one", func(t *testing.T) {
		t.Parallel()
		a, e := newSrdTargets(t, "Mago", 3)
		a.wantTooMany(t, e, "Mago", disintegrate, slotOfLevel(6), goblins(2))
		a.takes(t, e, "Mago", disintegrate, slotOfLevel(6), goblins(1))
	})
}

// TestSRDChainLightning: four targets at the 6th circle and one more for each circle above, all
// within 30 ft of the first.
func TestSRDChainLightning(t *testing.T) {
	t.Parallel()
	for slot := int32(6); slot <= 9; slot++ {
		t.Run(fmt.Sprintf("slot %d", slot), func(t *testing.T) {
			t.Parallel()
			a, e := newSrdTargets(t, "Mago", 10)
			n := int(slot) - 2
			a.wantTooMany(t, e, "Mago", chainLightning, slotOfLevel(slot), goblins(n+1))
			a.takes(t, e, "Mago", chainLightning, slotOfLevel(slot), goblins(n))
		})
	}
	t.Run("a target 30 ft from the first is refused", func(t *testing.T) {
		t.Parallel()
		a, e := newSrdTargets(t, "Mago", 4)
		_, err := a.cast(t, a.ana, e, "Mago", chainLightning, slotOfLevel(6), a.at(t, "Goblin 1", "Capitão Goblin"), noCastRoll)
		b := wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_TARGET_OUT_OF_REACH)
		if b.GetMissingFt() < 1 {
			t.Errorf("missing %d ft, want the distance beyond 30 ft", b.GetMissingFt())
		}
	})
	t.Run("a target 30 ft from the first is taken", func(t *testing.T) {
		t.Parallel()
		a, e := newSrdTargets(t, "Mago", 4)
		a.put(t, "Capitão Goblin", 10, 1) // 30 ft from Goblin 1, on the same row
		a.takes(t, e, "Mago", chainLightning, slotOfLevel(6), []string{"Goblin 1", "Capitão Goblin"})
	})
}

// TestSRDMassCureWoundsHealsOnlyThePicked: up to six, the ones the caster names; a foe not named is not healed.
func TestSRDMassCureWoundsHealsOnlyThePicked(t *testing.T) {
	t.Parallel()
	a, e := newSrdTargets(t, "Clerigo", 8)
	a.wantTooMany(t, e, "Clerigo", massCureWounds, slotOfLevel(5), goblins(7))
	cast := a.takes(t, e, "Clerigo", massCureWounds, slotOfLevel(5), goblins(6))
	healed := map[string]bool{}
	for _, p := range cast.GetPendingDamages() {
		if !p.GetHealing() {
			t.Errorf("pending %v is not a heal", p)
		}
		healed[p.GetTargetId()] = true
	}
	for _, label := range goblins(8) {
		picked := slices.Contains(goblins(6), label)
		if got := healed[a.id(t, label)]; got != picked {
			t.Errorf("%s healed = %v, want %v", label, got, picked)
		}
	}
	if healed[a.id(t, "Capitão Goblin")] || healed[a.id(t, "Mago")] {
		t.Errorf("a creature nobody named is healed: %v", healed)
	}
}

// TestSRDWallOfIceHarmsOnlyTheListed: only the creatures the caster lists as cut by the wall take damage,
// not every creature near them.
func TestSRDWallOfIceHarmsOnlyTheListed(t *testing.T) {
	t.Parallel()
	a, e := newSrdTargets(t, "Mago", 4)
	// Goblin 2 stands between the two the wall cuts: no disc reaches both and spares it.
	listed := []string{"Goblin 1", "Goblin 3"}
	cast := a.takes(t, e, "Mago", wallOfIce, slotOfLevel(6), listed)
	damaged := map[string]int{}
	for _, p := range cast.GetPendingDamages() {
		damaged[p.GetTargetId()]++
	}
	for label, want := range map[string]int{"Goblin 1": 1, "Goblin 2": 0, "Goblin 3": 1, "Goblin 4": 0, "Capitão Goblin": 0} {
		if n := damaged[a.id(t, label)]; n != want {
			t.Errorf("%s has %d pending damages, want %d", label, n, want)
		}
	}
	if len(damaged) != 2 {
		t.Errorf("damaged = %v, want only the two the caster listed", damaged)
	}
}

// TestSRDSpellsWithNoCreatureNameNoOne: a spell that reaches no creature lists none in the turn's
// options and is cast with no target; naming one is refused.
func TestSRDSpellsWithNoCreatureNameNoOne(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		caster, spell string
		slot          *playv1.SpellSlot
	}{
		{"Mago", "spell:animate-objects", slotOfLevel(5)},
		{"Mago", "spell:passwall", slotOfLevel(5)},
		{"Mago", "spell:wall-of-force", slotOfLevel(5)},
		{"Mago", "spell:wall-of-stone", slotOfLevel(5)},
		{"Mago", "spell:fire-shield", slotOfLevel(4)},
		{"Mago", "spell:dimension-door", slotOfLevel(4)},
		{"Mago", "spell:stone-shape", slotOfLevel(4)},
		{"Mago", "spell:secret-chest", slotOfLevel(4)},
		{"Druida", "spell:giant-insect", slotOfLevel(4)},
		{"Mago", "spell:mage-hand", nil},
		{"Mago", "spell:prestidigitation", nil},
		{"Druida", "spell:shillelagh", nil},
		{"Clerigo", "spell:thaumaturgy", nil},
	} {
		t.Run(tc.spell, func(t *testing.T) {
			t.Parallel()
			a, e := newSrdTargets(t, tc.caster, 2)
			u := a.userOf(tc.caster)
			st := spellTargetsOf(a.mustOptions(t, u, e, tc.caster), tc.spell)
			if st == nil || st.GetMaxTargets() != 0 || len(st.GetTargets()) != 1 {
				t.Fatalf("%s targets = %v, want an entry that lists only the caster and takes no creature", tc.spell, st)
			}
			if st.GetPlacement() != playv1.AreaPlacement_AREA_PLACEMENT_UNSPECIFIED {
				t.Errorf("%s is placed on the map: %v", tc.spell, st.GetPlacement())
			}
			_, err := a.cast(t, u, e, tc.caster, tc.spell, tc.slot, a.at(t, "Goblin 1"), noCastRoll)
			wantCode(t, "a creature named for "+tc.spell, err, connect.CodeInvalidArgument)
			a.mustCast(t, u, e, tc.caster, tc.spell, tc.slot, nil, noCastRoll)
		})
	}
}

// TestSRDMendingNamesNoOneOutsideCombat: Mending takes a minute, so it is cast outside combat; it
// reaches the caster alone, and no creature is named.
func TestSRDMendingNamesNoOneOutsideCombat(t *testing.T) {
	t.Parallel()
	a := newSrdParty(t)
	res, err := a.ana.casting.GetCastOptions(t.Context(), connect.NewRequest(&playv1.GetCastOptionsRequest{CampaignId: a.campaignID, CharacterId: a.pens.GetId()}))
	if err != nil {
		t.Fatalf("GetCastOptions() error = %v", err)
	}
	var got *playv1.CastingSpell
	for _, o := range res.Msg.GetSpells() {
		if o.GetSpell().GetKey() == "spell:mending" {
			got = o
		}
	}
	if got == nil || !got.GetCasterOnly() || got.GetMaxTargets() != 0 {
		t.Fatalf("Mending's options = %v, want caster only with no targets", got)
	}
	a.mustCastOut(t, a.ana, a.pens, "spell:mending", nil, false, nil)
}

// srdTheatre starts the party of newSrdParty against nGoblins goblins in a combat without a map.
func (a *armed) srdTheatre(t *testing.T, first string, nGoblins int32) *playv1.Encounter {
	t.Helper()
	reveal := []string{"Capitão Goblin"}
	rolls := []int{1}
	for i := range nGoblins {
		reveal = append(reveal, fmt.Sprintf("Goblin %d", i+1))
		rolls = append(rolls, 1)
	}
	players := map[string]int32{"Mago": 10, "Clerigo": 10, "Druida": 10}
	players[first] = 20
	return a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId(), Count: nGoblins}, {CharacterId: a.capitao.GetId()}}, npcRolls: rolls,
		reveal: reveal, players: players, theatre: true,
	})
}

// TestSRDMasterIsNotHeldToTheTargets: the master casts for a creature with more targets than the spell takes
// and with targets farther apart than it allows.
func TestSRDMasterIsNotHeldToTheTargets(t *testing.T) {
	t.Parallel()
	t.Run("Slow with seven", func(t *testing.T) {
		t.Parallel()
		a, e := newSrdTargets(t, "Mago", 8)
		got := a.mustCast(t, a.master, e, "Mago", slowSpell, slotOfLevel(3), a.at(t, goblins(7)...), noCastRoll).GetCast()
		if len(got.GetTargets()) != 7 {
			t.Errorf("Slow hit %d targets, want 7", len(got.GetTargets()))
		}
	})
	t.Run("Acid Splash two targets 10 ft apart", func(t *testing.T) {
		t.Parallel()
		a, e := newSrdTargets(t, "Mago", 4)
		got := a.mustCast(t, a.master, e, "Mago", acidSplash, nil, a.at(t, "Goblin 1", "Goblin 3"), noCastRoll).GetCast()
		if len(got.GetTargets()) != 2 {
			t.Errorf("Acid Splash hit %d targets, want 2", len(got.GetTargets()))
		}
	})
}

// TestSRDNoGridNoCloseness: in a combat without a map a player is not held to how close the targets are.
func TestSRDNoGridNoCloseness(t *testing.T) {
	t.Parallel()
	a := newSrdParty(t)
	e := a.srdTheatre(t, "Mago", 4)
	a.takes(t, e, "Mago", acidSplash, nil, []string{"Goblin 1", "Capitão Goblin"})
}

// TestSRDMassCureWoundsRefusesTargetsFarApart: two picks more than 60 ft apart are refused for a player.
func TestSRDMassCureWoundsRefusesTargetsFarApart(t *testing.T) {
	t.Parallel()
	a, e := newSrdTargets(t, "Clerigo", 2)
	a.put(t, "Goblin 1", 2, 1)
	a.put(t, "Capitão Goblin", 15, 1) // 65 ft from Goblin 1
	_, err := a.cast(t, a.bia, e, "Clerigo", massCureWounds, slotOfLevel(5), a.at(t, "Goblin 1", "Capitão Goblin"), noCastRoll)
	b := wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_TARGET_OUT_OF_REACH)
	if b.GetMissingFt() != 5 {
		t.Errorf("missing %d ft, want 5", b.GetMissingFt())
	}
}
