package combat

import (
	"slices"
	"testing"
)

func kinds(sources []Source) []string {
	out := make([]string, len(sources))
	for i, s := range sources {
		out[i] = s.Kind
	}
	slices.Sort(out)
	return out
}

func TestResolveFollowsTheSRDCancellation(t *testing.T) {
	adv := Source{Kind: SourceProneTarget, Effect: ModeAdvantage}
	dis := Source{Kind: SourcePoisonedAttacker, Effect: ModeDisadvantage}
	tests := []struct {
		name    string
		sources []Source
		want    RollMode
	}{
		{"none", nil, ModeNormal},
		{"one advantage", []Source{adv}, ModeAdvantage},
		{"three advantages are still one extra die", []Source{adv, adv, adv}, ModeAdvantage},
		{"one disadvantage", []Source{dis}, ModeDisadvantage},
		{"several disadvantages", []Source{dis, dis}, ModeDisadvantage},
		{"one of each cancels", []Source{adv, dis}, ModeNormal},
		{"many disadvantages and one advantage cancel", []Source{dis, dis, dis, adv}, ModeNormal},
		{"many advantages and one disadvantage cancel", []Source{adv, adv, adv, dis}, ModeNormal},
		{"a normal source changes nothing", []Source{{Kind: "x", Effect: ModeNormal}}, ModeNormal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Resolve(tt.sources); got != tt.want {
				t.Fatalf("Resolve = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRollModeKeepsTheHigherOrLowerDie(t *testing.T) {
	tests := []struct {
		mode  RollMode
		faces []int
		want  int
		dice  int
	}{
		{ModeNormal, []int{9}, 0, 1},
		{ModeAdvantage, []int{5, 17}, 1, 2},
		{ModeAdvantage, []int{17, 5}, 0, 2},
		{ModeAdvantage, []int{12, 12}, 0, 2},
		{ModeDisadvantage, []int{5, 17}, 0, 2},
		{ModeDisadvantage, []int{17, 5}, 1, 2},
		{ModeDisadvantage, []int{8, 8}, 0, 2},
	}
	for _, tt := range tests {
		if got := tt.mode.Pick(tt.faces); got != tt.want {
			t.Errorf("%v.Pick(%v) = %d, want %d", tt.mode, tt.faces, got, tt.want)
		}
		if got := tt.mode.Dice(); got != tt.dice {
			t.Errorf("%v.Dice() = %d, want %d", tt.mode, got, tt.dice)
		}
	}
	if !ModeAdvantage.Better(ModeNormal) || !ModeNormal.Better(ModeDisadvantage) || ModeDisadvantage.Better(ModeNormal) || ModeNormal.Better(ModeNormal) {
		t.Fatal("Better does not order disadvantage < normal < advantage")
	}
}

func TestAttackModeFromConditions(t *testing.T) {
	melee := AttackScene{DistanceKnown: true, DistanceFt: 5, ReachFt: 5}
	ranged := AttackScene{Ranged: true, DistanceKnown: true, DistanceFt: 30, ReachFt: 80, LongRangeFt: 320}
	with := func(base AttackScene, attacker, target []string) AttackScene {
		base.Attacker.Conditions, base.Target.Conditions = attacker, target
		return base
	}
	tests := []struct {
		name  string
		scene AttackScene
		want  RollMode
		kinds []string
		crit  bool
	}{
		{"a clean melee attack", melee, ModeNormal, []string{}, false},
		{"prone target within 5 ft: advantage", with(melee, nil, []string{"condition:prone"}), ModeAdvantage, []string{SourceProneTarget}, false},
		{"prone target at a distance: disadvantage", with(ranged, nil, []string{"condition:prone"}), ModeDisadvantage, []string{SourceProneTarget}, false},
		{"prone attacker", with(melee, []string{"condition:prone"}, nil), ModeDisadvantage, []string{SourceProneAttacker}, false},
		{"restrained attacker", with(melee, []string{"condition:restrained"}, nil), ModeDisadvantage, []string{SourceRestrainedAttacker}, false},
		{"restrained target", with(melee, nil, []string{"condition:restrained"}), ModeAdvantage, []string{SourceRestrainedTarget}, false},
		{"blinded attacker", with(melee, []string{"condition:blinded"}, nil), ModeDisadvantage, []string{SourceBlindedAttacker}, false},
		{"blinded target", with(melee, nil, []string{"condition:blinded"}), ModeAdvantage, []string{SourceBlindedTarget}, false},
		{"poisoned attacker", with(melee, []string{"condition:poisoned"}, nil), ModeDisadvantage, []string{SourcePoisonedAttacker}, false},
		{"frightened attacker", with(melee, []string{"condition:frightened"}, nil), ModeDisadvantage, []string{SourceFrightenedAttacker}, false},
		{"invisible attacker", with(melee, []string{"condition:invisible"}, nil), ModeAdvantage, []string{SourceInvisibleAttacker}, false},
		{"invisible target", with(melee, nil, []string{"condition:invisible"}), ModeDisadvantage, []string{SourceInvisibleTarget}, false},
		{"stunned target", with(melee, nil, []string{"condition:stunned"}), ModeAdvantage, []string{SourceStunnedTarget}, false},
		{"petrified target", with(melee, nil, []string{"condition:petrified"}), ModeAdvantage, []string{SourcePetrifiedTarget}, false},
		{"paralyzed target within 5 ft is a critical hit", with(melee, nil, []string{"condition:paralyzed"}), ModeAdvantage, []string{SourceParalyzedTarget}, true},
		{"unconscious target within 5 ft is a critical hit", with(melee, nil, []string{"condition:unconscious"}), ModeAdvantage, []string{SourceUnconsciousTarget}, true},
		{"paralyzed target from afar is no critical hit", with(ranged, nil, []string{"condition:paralyzed"}), ModeAdvantage, []string{SourceParalyzedTarget}, false},
		{"poisoned attacker against a stunned target cancel", with(melee, []string{"condition:poisoned"}, []string{"condition:stunned"}), ModeNormal, []string{SourcePoisonedAttacker, SourceStunnedTarget}, false},
		{"two disadvantages and one advantage cancel", with(melee, []string{"condition:poisoned", "condition:prone"}, []string{"condition:blinded"}), ModeNormal, []string{SourceBlindedTarget, SourcePoisonedAttacker, SourceProneAttacker}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := AttackMode(tt.scene)
			if got := Resolve(r.Sources); got != tt.want {
				t.Fatalf("mode = %v, want %v (sources %v)", got, tt.want, kinds(r.Sources))
			}
			want := slices.Clone(tt.kinds)
			slices.Sort(want)
			if got := kinds(r.Sources); !slices.Equal(got, want) {
				t.Fatalf("sources = %v, want %v", got, want)
			}
			if r.CriticalOnHit != tt.crit {
				t.Fatalf("CriticalOnHit = %v, want %v", r.CriticalOnHit, tt.crit)
			}
		})
	}
}

func TestAttackModeWithoutAMapAssumesTheUsualMeleeReach(t *testing.T) {
	melee := AttackScene{ReachFt: 5}
	melee.Target.Conditions = []string{"condition:prone", "condition:paralyzed"}
	r := AttackMode(melee)
	if !r.CriticalOnHit || Resolve(r.Sources) != ModeAdvantage {
		t.Fatalf("a melee attack with the usual reach is within 5 ft without a map: %v crit %v", kinds(r.Sources), r.CriticalOnHit)
	}
	longReach := AttackScene{ReachFt: 10}
	longReach.Target.Conditions = []string{"condition:prone"}
	if got := AttackMode(longReach).Sources; len(got) != 0 {
		t.Fatalf("a 10 ft reach without a map leaves the prone rule to the master, got %v", kinds(got))
	}
	ranged := AttackScene{Ranged: true, ReachFt: 80, LongRangeFt: 320}
	ranged.Target.Conditions = []string{"condition:prone", "condition:unconscious"}
	r = AttackMode(ranged)
	if r.CriticalOnHit || slices.Contains(kinds(r.Sources), SourceProneTarget) {
		t.Fatalf("a ranged attack without a map knows no distance: %v crit %v", kinds(r.Sources), r.CriticalOnHit)
	}
}

func TestAttackModeFromTheBarbarianAndTheDodgeAction(t *testing.T) {
	melee := AttackScene{DistanceKnown: true, DistanceFt: 5, ReachFt: 5, StrengthMelee: true, OwnTurn: true}
	t.Run("reckless attack on the barbarian's own strength melee attack", func(t *testing.T) {
		s := melee
		s.Attacker.Reckless = true
		if got := Resolve(AttackMode(s).Sources); got != ModeAdvantage {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("reckless attack gives nothing to a dexterity or ranged attack", func(t *testing.T) {
		s := melee
		s.Attacker.Reckless, s.StrengthMelee = true, false
		if got := AttackMode(s).Sources; len(got) != 0 {
			t.Fatalf("got %v", kinds(got))
		}
	})
	t.Run("reckless attack on another creature's turn (an opportunity attack) gives nothing", func(t *testing.T) {
		s := melee
		s.Attacker.Reckless, s.OwnTurn = true, false
		if got := AttackMode(s).Sources; len(got) != 0 {
			t.Fatalf("got %v", kinds(got))
		}
	})
	t.Run("attacks against a reckless barbarian have advantage, melee or ranged", func(t *testing.T) {
		s := AttackScene{Ranged: true, DistanceKnown: true, DistanceFt: 20, ReachFt: 80}
		s.Target.Reckless = true
		if got := Resolve(AttackMode(s).Sources); got != ModeAdvantage {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("a dodging target imposes disadvantage on an attacker it sees", func(t *testing.T) {
		s := melee
		s.Target.Dodging = true
		if got := Resolve(AttackMode(s).Sources); got != ModeDisadvantage {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("a dodging target gives nothing against an attacker it cannot see", func(t *testing.T) {
		for name, s := range map[string]AttackScene{
			"hidden":          {AttackerUnseen: true},
			"invisible":       {Attacker: Creature{Conditions: []string{"condition:invisible"}}},
			"blinded dodger":  {Target: Creature{Conditions: []string{"condition:blinded"}}},
			"attacker unseen": {AttackerUnseen: true, Ranged: true},
		} {
			s.Target.Dodging = true
			s.ReachFt = 5
			if slices.Contains(kinds(AttackMode(s).Sources), SourceDodgingTarget) {
				t.Errorf("%s: dodge still counted", name)
			}
		}
	})
	t.Run("the dodge is lost when incapacitated or the speed is 0", func(t *testing.T) {
		for _, c := range []string{"condition:incapacitated", "condition:stunned", "condition:grappled", "condition:restrained", "condition:paralyzed", "condition:unconscious", "condition:petrified"} {
			s := melee
			s.Target = Creature{Dodging: true, Conditions: []string{c}}
			if slices.Contains(kinds(AttackMode(s).Sources), SourceDodgingTarget) {
				t.Errorf("%s: dodge still counted", c)
			}
		}
	})
	t.Run("a dodging reckless barbarian: advantage and disadvantage cancel", func(t *testing.T) {
		s := melee
		s.Target = Creature{Dodging: true, Reckless: true}
		if got := Resolve(AttackMode(s).Sources); got != ModeNormal {
			t.Fatalf("got %v", got)
		}
	})
}

func TestAttackModePackTacticsAndUnseenCreatures(t *testing.T) {
	s := AttackScene{DistanceKnown: true, DistanceFt: 5, ReachFt: 5}
	s.Attacker.PackTactics, s.AllyNearTarget = true, true
	if got := kinds(AttackMode(s).Sources); !slices.Equal(got, []string{SourcePackTactics}) {
		t.Fatalf("pack tactics: %v", got)
	}
	s.AllyNearTarget = false
	if got := AttackMode(s).Sources; len(got) != 0 {
		t.Fatalf("no ally near the target: %v", kinds(got))
	}
	s = AttackScene{AllyNearTarget: true, ReachFt: 5}
	if got := AttackMode(s).Sources; len(got) != 0 {
		t.Fatalf("an ally helps only a creature with Pack Tactics: %v", kinds(got))
	}
	unseen := AttackScene{ReachFt: 5, AttackerUnseen: true}
	if got := kinds(AttackMode(unseen).Sources); !slices.Equal(got, []string{SourceUnseenAttacker}) {
		t.Fatalf("unseen attacker: %v", got)
	}
	unseen = AttackScene{ReachFt: 5, TargetUnseen: true}
	if got := kinds(AttackMode(unseen).Sources); !slices.Equal(got, []string{SourceUnseenTarget}) {
		t.Fatalf("unseen target: %v", got)
	}
	both := AttackScene{ReachFt: 5, TargetUnseen: true, AttackerUnseen: true}
	if got := Resolve(AttackMode(both).Sources); got != ModeNormal {
		t.Fatalf("each side unseen cancels: %v", got)
	}
	invisible := AttackScene{ReachFt: 5, AttackerUnseen: true}
	invisible.Attacker.Conditions = []string{"condition:invisible"}
	if got := kinds(AttackMode(invisible).Sources); !slices.Equal(got, []string{SourceInvisibleAttacker}) {
		t.Fatalf("an invisible attacker is listed once, by its condition: %v", got)
	}
}

func TestAttackModeForRangedAttacks(t *testing.T) {
	arrow := AttackScene{Ranged: true, DistanceKnown: true, ReachFt: 150, LongRangeFt: 600}
	tests := []struct {
		name     string
		distance int
		near     bool
		want     []string
	}{
		{"inside the normal range", 100, false, []string{}},
		{"exactly at the normal range", 150, false, []string{}},
		{"between normal and long range", 151, false, []string{SourceLongRange}},
		{"at the long range", 600, false, []string{SourceLongRange}},
		{"a hostile within 5 ft", 100, true, []string{SourceHostileNearby}},
		{"long range and a hostile within 5 ft", 300, true, []string{SourceHostileNearby, SourceLongRange}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := arrow
			s.DistanceFt, s.HostileNearAttacker = tt.distance, tt.near
			want := slices.Clone(tt.want)
			slices.Sort(want)
			if got := kinds(AttackMode(s).Sources); !slices.Equal(got, want) {
				t.Fatalf("sources = %v, want %v", got, want)
			}
		})
	}
	melee := AttackScene{DistanceKnown: true, DistanceFt: 5, ReachFt: 5, HostileNearAttacker: true}
	if got := AttackMode(melee).Sources; len(got) != 0 {
		t.Fatalf("a melee attack has no range disadvantage: %v", kinds(got))
	}
	noLong := AttackScene{Ranged: true, DistanceKnown: true, DistanceFt: 30, ReachFt: 20}
	if got := kinds(AttackMode(noLong).Sources); !slices.Equal(got, []string{SourceLongRange}) {
		t.Fatalf("a spell beyond its range with no long range: %v", got)
	}
}

func TestSaveModeSources(t *testing.T) {
	danger := Creature{DangerSense: true}
	tests := []struct {
		name  string
		scene SaveScene
		want  []string
	}{
		{"danger sense against an effect it sees", SaveScene{Creature: danger, Ability: "dex", EffectVisible: true}, []string{SourceDangerSense}},
		{"danger sense against a trap it does not see", SaveScene{Creature: danger, Ability: "dex"}, []string{}},
		{"danger sense is for dexterity only", SaveScene{Creature: danger, Ability: "con", EffectVisible: true}, []string{}},
		{"danger sense is lost when blinded", SaveScene{Creature: Creature{DangerSense: true, Conditions: []string{"condition:blinded"}}, Ability: "dex", EffectVisible: true}, []string{}},
		{"danger sense is lost when deafened", SaveScene{Creature: Creature{DangerSense: true, Conditions: []string{"condition:deafened"}}, Ability: "dex", EffectVisible: true}, []string{}},
		{"danger sense is lost when incapacitated", SaveScene{Creature: Creature{DangerSense: true, Conditions: []string{"condition:incapacitated"}}, Ability: "dex", EffectVisible: true}, []string{}},
		{"dodge gives advantage on dexterity saves", SaveScene{Creature: Creature{Dodging: true}, Ability: "dex"}, []string{SourceDodgingSave}},
		{"dodge gives nothing on a wisdom save", SaveScene{Creature: Creature{Dodging: true}, Ability: "wis"}, []string{}},
		{"dodge is lost at speed 0", SaveScene{Creature: Creature{Dodging: true, Conditions: []string{"condition:grappled"}}, Ability: "dex"}, []string{}},
		{"restrained imposes disadvantage on dexterity saves", SaveScene{Creature: Creature{Conditions: []string{"condition:restrained"}}, Ability: "dex"}, []string{SourceRestrainedSave}},
		{"rage gives advantage on strength saves", SaveScene{Creature: Creature{Raging: true}, Ability: "str"}, []string{SourceRageStrength}},
		{"rage gives advantage on strength checks", SaveScene{Creature: Creature{Raging: true}, Ability: "str", Check: true}, []string{SourceRageStrength}},
		{"rage gives nothing on a dexterity save", SaveScene{Creature: Creature{Raging: true}, Ability: "dex"}, []string{}},
		{"poisoned imposes disadvantage on ability checks", SaveScene{Creature: Creature{Conditions: []string{"condition:poisoned"}}, Ability: "wis", Check: true}, []string{SourcePoisonedCheck}},
		{"poisoned does not touch saving throws", SaveScene{Creature: Creature{Conditions: []string{"condition:poisoned"}}, Ability: "wis"}, []string{}},
		{"frightened imposes disadvantage on ability checks", SaveScene{Creature: Creature{Conditions: []string{"condition:frightened"}}, Ability: "cha", Check: true}, []string{SourceFrightenedCheck}},
		{"a restrained dodger keeps only the restraint (speed 0 ends the dodge)", SaveScene{Creature: Creature{Dodging: true, Conditions: []string{"condition:restrained"}}, Ability: "dex"}, []string{SourceRestrainedSave}},
		{"danger sense and a restraint are both listed", SaveScene{Creature: Creature{DangerSense: true, Conditions: []string{"condition:restrained"}}, Ability: "dex", EffectVisible: true}, []string{SourceDangerSense, SourceRestrainedSave}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := kinds(SaveMode(tt.scene))
			want := slices.Clone(tt.want)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Fatalf("sources = %v, want %v", got, want)
			}
		})
	}
}

func TestAutoFailsSaveForTheCreaturesThatCannotMove(t *testing.T) {
	for _, c := range []string{"condition:stunned", "condition:paralyzed", "condition:unconscious", "condition:petrified"} {
		cr := Creature{Conditions: []string{c}}
		if !AutoFailsSave(cr, "str") || !AutoFailsSave(cr, "dex") {
			t.Errorf("%s must fail Strength and Dexterity saves", c)
		}
		if AutoFailsSave(cr, "con") || AutoFailsSave(cr, "wis") {
			t.Errorf("%s fails only Strength and Dexterity saves", c)
		}
	}
	if AutoFailsSave(Creature{Conditions: []string{"condition:prone"}}, "dex") || AutoFailsSave(Creature{}, "str") {
		t.Fatal("a creature that is not held fails nothing")
	}
}

func TestSaveModeCancelsLikeAnyOtherRoll(t *testing.T) {
	c := Creature{DangerSense: true, Conditions: []string{"condition:restrained"}}
	if got := Resolve(SaveMode(SaveScene{Creature: c, Ability: "dex", EffectVisible: true})); got != ModeNormal {
		t.Fatalf("danger sense against a restraint is a normal roll, got %v", got)
	}
}
