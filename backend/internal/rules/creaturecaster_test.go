package rules

import "testing"

// The stat block's casting as the combat reads it (SRD 5.1): the Mage's slots and numbers, the Couatl's
// innate spells at will and with uses a day, the numbers an Innate Spellcasting without a DC works out.
func TestMonsterCasterReadsBothTraits(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	mage, ok := c.MonsterCaster("monster:mage")
	if !ok || mage.Spellcasting.SaveDC != 14 || mage.Spellcasting.AttackBonus != 6 || mage.Spellcasting.Ability != INT {
		t.Fatalf("mage = %+v (%v), want Intelligence, DC 14, +6", mage.Spellcasting, ok)
	}
	if mage.Slots[0] != 4 || mage.Slots[1] != 3 || mage.Slots[2] != 3 || len(mage.Granted) != 0 || len(mage.Resources) != 0 {
		t.Errorf("mage slots = %v granted %d, want 4/3/3 and no innate spell", mage.Slots, len(mage.Granted))
	}
	prepared := map[string]bool{}
	for _, s := range mage.Spells {
		prepared[s.Spell.Key] = s.Prepared
	}
	for _, k := range []string{"spell:fire-bolt", "spell:magic-missile", "spell:counterspell"} {
		if !prepared[k] {
			t.Errorf("mage does not have %s prepared", k)
		}
	}

	couatl, ok := c.MonsterCaster("monster:couatl")
	if !ok || couatl.Spellcasting.SaveDC != 14 {
		t.Fatalf("couatl = %+v (%v)", couatl.Spellcasting, ok)
	}
	byKey := map[string]GrantedSpell{}
	for _, g := range couatl.Granted {
		byKey[g.Spell.Key] = g
	}
	if g := byKey["spell:detect-magic"]; !g.AtWill || g.Resource != "" {
		t.Errorf("detect magic = %+v, want at will", g)
	}
	bless := byKey["spell:bless"]
	if bless.AtWill || bless.Resource != "innate_3_day_bless" || bless.SaveDC != 14 {
		t.Errorf("bless = %+v, want 3/day with its own resource and DC 14", bless)
	}
	var uses int
	for _, r := range couatl.Resources {
		if r.Key == bless.Resource {
			uses = r.Max
		}
	}
	if uses != 3 {
		t.Errorf("bless resource max = %d, want 3", uses)
	}

	// The Cloud Giant lists no DC: 8 + proficiency bonus 4 + Charisma modifier +3.
	giant, ok := c.MonsterCaster("monster:cloud-giant")
	if !ok || giant.Spellcasting.SaveDC != 15 || giant.Spellcasting.AttackBonus != 7 {
		t.Errorf("cloud giant = %+v (%v), want DC 15, +7", giant.Spellcasting, ok)
	}

	// The Lamia's text runs its lists together on one line: at will, 3/day each and 1/day.
	lamia, ok := c.MonsterCaster("monster:lamia")
	if !ok {
		t.Fatal("the lamia has Innate Spellcasting")
	}
	got := map[string]string{}
	for _, g := range lamia.Granted {
		got[g.Spell.Key] = g.Resource
		if g.AtWill {
			got[g.Spell.Key] = "at-will"
		}
	}
	want := map[string]string{
		"spell:disguise-self": "at-will", "spell:major-image": "at-will", "spell:charm-person": "innate_3_day_charm-person",
		"spell:mirror-image": "innate_3_day_mirror-image", "spell:scrying": "innate_3_day_scrying", "spell:suggestion": "innate_3_day_suggestion", "spell:geas": "innate_1_day",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("lamia %s = %q, want %q (all: %v)", k, got[k], v, got)
		}
	}

	if _, ok := c.MonsterCaster("monster:goblin"); ok {
		t.Error("a goblin casts nothing")
	}
}
