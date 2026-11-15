package rules

import "testing"

func intelligenceOf(d Derived, a Ability) (AbilityScore, SavingThrow) {
	var score AbilityScore
	var save SavingThrow
	for _, s := range d.Abilities {
		if s.Ability == a {
			score = s
		}
	}
	for _, s := range d.SavingThrows {
		if s.Ability == a {
			save = s
		}
	}
	return score, save
}

// TestMonsterDerivedMinInt: a steed has Intelligence 6 at least (SRD 5.1, Find Steed): a score below it is raised, with its
// modifier and its saving throw; a score at or above it stays, and the rest of the stat block is the creature's own.
func TestMonsterDerivedMinInt(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for _, key := range []string{"monster:warhorse", "monster:pony", "monster:camel", "monster:elk", "monster:mastiff"} {
		plain, ok := c.MonsterDerived(key)
		if !ok {
			t.Fatalf("%s is no creature", key)
		}
		got, ok := c.MonsterDerivedMinInt(key, 6)
		if !ok {
			t.Fatalf("MonsterDerivedMinInt(%s) found no creature", key)
		}
		score, save := intelligenceOf(got, INT)
		if score.Score != 6 || score.Modifier != -2 || save.Bonus != -2 {
			t.Errorf("%s: Intelligence %+v, save %+v, want 6, modifier -2, save -2", key, score, save)
		}
		if got.HitPointsMax != plain.HitPointsMax || got.ArmorClass != plain.ArmorClass || got.SpeedWalkFt != plain.SpeedWalkFt {
			t.Errorf("%s: the rest of the stat block changed", key)
		}
		if was, _ := intelligenceOf(plain, INT); was.Score >= 6 {
			t.Errorf("%s: Intelligence %d already: the floor proves nothing", key, was.Score)
		}
	}
	plain, _ := c.MonsterDerived("monster:goblin")
	was, _ := intelligenceOf(plain, INT)
	got, ok := c.MonsterDerivedMinInt("monster:goblin", 6)
	if !ok || was.Score < 6 {
		t.Fatalf("a goblin has Intelligence %d: the case needs one of 6 or more", was.Score)
	}
	if now, _ := intelligenceOf(got, INT); now.Score != was.Score {
		t.Errorf("a goblin's Intelligence changed from %d to %d", was.Score, now.Score)
	}
	if _, ok := c.MonsterDerivedMinInt("monster:dodo", 6); ok {
		t.Error("a creature that does not exist was found")
	}
}
