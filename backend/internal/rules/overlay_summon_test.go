package rules

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// The summon of a table spell. The names are invented; the creatures are a table creature on the SRD's
// wolf and the SRD's giant spider.

const callOfThePack = "spell:chamado-da-matilha@mesa"

// summonSpell is a level 2 spell of the wizard's list that brings the pack leader or a wolf: one creature,
// one more for every two circles of the slot, one more for each circle above the spell's own.
func summonSpell() TableSpell {
	ts := tableTestSpells()[0]
	ts.Key, ts.NamePT, ts.Level = callOfThePack, "Chamado da Matilha", 2
	ts.Attack, ts.Damage = "", nil
	ts.Range, ts.Target = TableRange{Kind: RangeSelf}, SpellTarget{Kind: TargetSelf}
	ts.Concentration = true
	ts.Duration = TableDuration{Kind: DurationTimed, Amount: 1, Unit: DurationHour, UpTo: true}
	ts.Summon = &TableSummon{
		CreatureKeys: []string{packLeader, wolfKey}, Count: "1 + floor(slot() / 2)", CountPerSlotLevel: 1,
		TurnMode: SummonTurnOwnerRollGroup, MaxLive: 12, Replaces: SummonReplacesSame,
	}
	return ts
}

func tableSummonContent(t *testing.T, ts TableSpell) *Content {
	t.Helper()
	srd := loadForTest(t)
	return withOverlayOn(t, srd, Overlay{Revision: 1, Creatures: []TableCreature{packLeaderCreature()}, Spells: []TableSpell{ts}})
}

func TestATableSpellSummonsThroughTheSRDChecks(t *testing.T) {
	t.Parallel()
	c := tableSummonContent(t, summonSpell())
	if !slices.Contains(c.SummonSpells(), callOfThePack) {
		t.Fatalf("SummonSpells() = %v, want the table spell among them", c.SummonSpells())
	}
	for circle, want := range map[int]int{2: 2, 3: 3, 4: 5, 5: 6, 9: 12} { // 1 + floor(slot / 2) + (slot - 2)
		choices, err := c.SummonOptions(callOfThePack, circle, Build{})
		if err != nil {
			t.Fatalf("circle %d: %v", circle, err)
		}
		if len(choices.Options) != 1 || choices.Options[0].Count != min(want, MaxTableSummonCount) || !choices.Concentration {
			t.Errorf("circle %d: options = %+v, want one option of %d creatures, concentration", circle, choices.Options, want)
		}
		if got := len(choices.Options[0].Forms); got != 2 {
			t.Errorf("circle %d: %d forms, want the two creatures", circle, got)
		}
	}
	if _, err := c.SummonOptions(callOfThePack, 1, Build{}); !errors.Is(err, ErrSummonCircle) {
		t.Errorf("circle 1: err = %v, want ErrSummonCircle", err)
	}
	made, err := c.CheckSummon(callOfThePack, 2, Build{}, SummonPick{Creatures: []string{packLeader, wolfKey}})
	if err != nil || len(made) != 2 || made[0].Attack != SummonAttackFull {
		t.Errorf("CheckSummon = %+v, %v, want 2 creatures that attack", made, err)
	}
	if _, err := c.CheckSummon(callOfThePack, 2, Build{}, SummonPick{Creatures: []string{packLeader}}); !errors.Is(err, ErrSummonCount) {
		t.Errorf("one creature of two: err = %v, want ErrSummonCount", err)
	}
	if _, err := c.CheckSummon(callOfThePack, 2, Build{}, SummonPick{Creatures: []string{packLeader, "monster:giant-spider"}}); !errors.Is(err, ErrSummonCreature) {
		t.Errorf("a creature the spell does not list: err = %v, want ErrSummonCreature", err)
	}
}

func TestATableSummonKeepsItsRule(t *testing.T) {
	t.Parallel()
	c := tableSummonContent(t, summonSpell())
	rule, ok := c.SummonRule(callOfThePack)
	if !ok || rule.TurnMode != SummonTurnOwnerRollGroup || rule.MaxLive != 12 || !rule.ReplacesSame {
		t.Errorf("SummonRule = %+v, %v; want the group roll, 12 live, replacing its own", rule, ok)
	}
	if _, ok := c.SummonRule("spell:conjure-animals"); ok {
		t.Error("an SRD summoning spell has a table rule")
	}
	ts := summonSpell()
	ts.Summon.TurnMode, ts.Summon.Replaces, ts.Summon.Role = "", "", ""
	rule, _ = tableSummonContent(t, ts).SummonRule(callOfThePack)
	if rule.TurnMode != SummonTurnOwnInitiative || rule.ReplacesSame {
		t.Errorf("SummonRule of the defaults = %+v, want its own initiative and nothing replaced", rule)
	}
}

func TestTheCountOfASummonReadsTheCasterAndIsClamped(t *testing.T) {
	t.Parallel()
	ts := summonSpell()
	ts.Summon.Count, ts.Summon.CountPerSlotLevel = `mod("cha")`, 0
	c := tableSummonContent(t, ts)
	b := standard("class:wizard", 5)
	b.BaseScores[CHA] = 17 // 18 with the human's +1: +4
	got, err := c.SummonOptions(callOfThePack, 2, b)
	if err != nil || got.Options[0].Count != 4 {
		t.Errorf("a Charisma of 18: %+v, %v, want 4 creatures", got.Options, err)
	}
	b.BaseScores[CHA] = 5 // 6: -2
	if got, _ := c.SummonOptions(callOfThePack, 2, b); got.Options[0].Count != 1 {
		t.Errorf("a Charisma of 6: %+v, want the least, 1 creature", got.Options)
	}
	ts.Summon.Count, ts.Summon.MaxLive = "25", 30 // clamped to 20, under the limit
	if got, err := tableSummonContent(t, ts).SummonOptions(callOfThePack, 2, b); err != nil || got.Options[0].Count != MaxTableSummonCount {
		t.Errorf("a count of 25 with a limit of 30: %+v, %v, want %d creatures", got.Options, err, MaxTableSummonCount)
	}
	ts.Summon.Count, ts.Summon.MaxLive = "50", 0
	if got, _ := tableSummonContent(t, ts).SummonOptions(callOfThePack, 2, b); got.Options[0].Count != MaxTableSummonCount {
		t.Errorf("a count of 50: %+v, want %d creatures", got.Options, MaxTableSummonCount)
	}
}

func TestATableSummonIsRefusedAtTheFieldThatIsWrong(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		edit  func(*TableSpell)
		field string
		more  string
	}{
		{"acting after the owner", func(s *TableSpell) { s.Summon.TurnMode = SummonTurnAfterOwner }, ".summon.turn_mode", ReasonValue},
		{"an unknown turn mode", func(s *TableSpell) { s.Summon.TurnMode = "unknown" }, ".summon.turn_mode", ReasonValue},
		{"a mount", func(s *TableSpell) { s.Summon.Role = SummonRoleMount }, ".summon.role", ReasonValue},
		{"an unknown role", func(s *TableSpell) { s.Summon.Role = "unknown" }, ".summon.role", ReasonValue},
		{"an unknown replacement", func(s *TableSpell) { s.Summon.Replaces = "unknown" }, ".summon.replaces", ReasonValue},
		{"a resource key", func(s *TableSpell) { s.Summon.ResourceKey = "pontos" }, ".summon.resource_key", ReasonValue},
		{"a resource cost", func(s *TableSpell) { s.Summon.ResourceCost = 2 }, ".summon.resource_key", ReasonValue},
		{"no count", func(s *TableSpell) { s.Summon.Count = " " }, ".summon.count", ReasonValue},
		{"a count that does not compile", func(s *TableSpell) { s.Summon.Count = "1 +" }, ".summon.count", ReasonFormula},
		{"a count with an unknown function", func(s *TableSpell) { s.Summon.Count = "nothing()" }, ".summon.count", ReasonFormula},
		{"a negative number for each circle", func(s *TableSpell) { s.Summon.CountPerSlotLevel = -1 }, ".summon.count_per_slot_level", ReasonValue},
		{"too many for each circle", func(s *TableSpell) { s.Summon.CountPerSlotLevel = MaxTableSummonPerSlot + 1 }, ".summon.count_per_slot_level", ReasonValue},
		{"a limit below 0", func(s *TableSpell) { s.Summon.MaxLive = -1 }, ".summon.max_live", ReasonValue},
		{"a limit above 40", func(s *TableSpell) { s.Summon.MaxLive = MaxTableSummonLive + 1 }, ".summon.max_live", ReasonValue},
		{"a static count over the limit", func(s *TableSpell) { s.Summon.Count, s.Summon.MaxLive = "5", 4 }, ".summon.max_live", ReasonValue},
		{"a ritual", func(s *TableSpell) { s.Ritual = true }, ".ritual", ReasonValue},
		{"no creature", func(s *TableSpell) { s.Summon.CreatureKeys = nil }, ".summon.creature_keys", ReasonLimit},
		{"nine creatures", func(s *TableSpell) {
			s.Summon.CreatureKeys = []string{"monster:wolf", "monster:giant-spider", "monster:bat", "monster:cat", "monster:crab", "monster:frog", "monster:hawk", "monster:lizard", "monster:rat"}
		}, ".summon.creature_keys", ReasonLimit},
		{"a creature that does not exist", func(s *TableSpell) { s.Summon.CreatureKeys = []string{packLeader, "creature:fantasma@mesa"} }, ".summon.creature_keys[1]", ReasonReference},
		{"an SRD key that is no creature", func(s *TableSpell) { s.Summon.CreatureKeys = []string{"spell:fireball"} }, ".summon.creature_keys[0]", ReasonReference},
		{"a creature twice", func(s *TableSpell) { s.Summon.CreatureKeys = []string{wolfKey, packLeader, wolfKey} }, ".summon.creature_keys[2]", ReasonValue},
		{"a cantrip", func(s *TableSpell) {
			s.Level, s.Concentration, s.Duration = 0, false, TableDuration{Kind: DurationInstantaneous}
		}, ".summon", ReasonValue},
		{"a target that is not the caster", func(s *TableSpell) {
			s.Range, s.Target = TableRange{Kind: RangeRanged, DistanceFt: 30}, SpellTarget{Kind: TargetCreature}
		}, ".target.kind", ReasonValue},
		{"damage", func(s *TableSpell) { s.Damage = []TableSpellDamage{{Type: "damage-type:force", Dice: "1d6"}} }, ".summon", ReasonValue},
		{"healing", func(s *TableSpell) { s.Heal = &TableSpellHeal{Dice: "1d4"} }, ".summon", ReasonValue},
		{"a saving throw", func(s *TableSpell) { s.Save = &SpellSave{Ability: CON, OnSuccess: "half"} }, ".summon", ReasonValue},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ts := summonSpell()
			tc.edit(&ts)
			_, err := loadForTest(t).With(Overlay{Revision: 1, Creatures: []TableCreature{packLeaderCreature()}, Spells: []TableSpell{ts}})
			var oe *OverlayError
			if !errors.As(err, &oe) {
				t.Fatalf("With() error = %v, want a refusal", err)
			}
			for _, v := range oe.Violations() {
				if strings.HasSuffix(v.Field, tc.field) && v.Reason == tc.more {
					return
				}
			}
			t.Errorf("no %s violation at %s: %v", tc.more, tc.field, err)
		})
	}
}

func TestSlotInASummonCountOnlyInASummonCount(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	ts := summonSpell()
	ts.Summon.Count = "slot()"
	if _, err := srd.With(Overlay{Revision: 1, Creatures: []TableCreature{packLeaderCreature()}, Spells: []TableSpell{ts}}); err != nil {
		t.Errorf("slot() in a summon count: %v", err)
	}
	// The creature's formulas read the slot of the cast too: a summoned creature is made with it.
	cr := packLeaderCreature()
	cr.HitPointsFormula = "10 * slot()"
	c := withOverlayOn(t, srd, Overlay{Revision: 1, Creatures: []TableCreature{cr}, Spells: []TableSpell{summonSpell()}})
	r, ok := c.ResolveCreature(packLeader, owner(5, 3, nil, nil), 4)
	if !ok || r.HitPoints != 40 {
		t.Errorf("a creature made with a 4th circle slot: %+v, %v, want 40 hit points", r, ok)
	}
}

func TestAnArchivedCreatureIsNotOfferedByASummonButTheOthersAre(t *testing.T) {
	t.Parallel()
	cr := packLeaderCreature()
	cr.Archived = true
	c := withOverlayOn(t, loadForTest(t), Overlay{Revision: 1, Creatures: []TableCreature{cr}, Spells: []TableSpell{summonSpell()}})
	got, err := c.SummonOptions(callOfThePack, 2, Build{})
	if err != nil || len(got.Options[0].Forms) != 1 || got.Options[0].Forms[0].Key != wolfKey {
		t.Fatalf("options = %+v, %v, want only the SRD wolf", got.Options, err)
	}
	if _, err := c.CheckSummon(callOfThePack, 2, Build{}, SummonPick{Creatures: []string{packLeader, wolfKey}}); !errors.Is(err, ErrSummonCreature) {
		t.Errorf("a cast with the archived creature: err = %v, want ErrSummonCreature", err)
	}
	if made, err := c.CheckSummon(callOfThePack, 2, Build{}, SummonPick{Creatures: []string{wolfKey, wolfKey}}); err != nil || len(made) != 2 {
		t.Errorf("a cast with the wolf alone: %+v, %v, want two creatures", made, err)
	}
}
