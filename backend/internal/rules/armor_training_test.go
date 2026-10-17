package rules

import "testing"

// TestArmorUntrained checks which of the worn body armor and the shield the
// character has no training for: the class proficiencies decide, a category or
// the very item.
func TestArmorUntrained(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	tests := []struct {
		name   string
		class  string
		armor  string
		shield bool
		want   ArmorUntrained
	}{
		{"nothing worn", "class:wizard", "", false, ArmorUntrained{}},
		{"wizard in light armor", "class:wizard", "equipment:leather-armor", false, ArmorUntrained{Armor: true}},
		{"wizard with a shield", "class:wizard", "", true, ArmorUntrained{Shield: true}},
		{"wizard in both", "class:wizard", "equipment:chain-mail", true, ArmorUntrained{Armor: true, Shield: true}},
		{"fighter in heavy armor and shield", "class:fighter", "equipment:plate-armor", true, ArmorUntrained{}},
		{"cleric in medium armor and shield", "class:cleric", "equipment:scale-mail", true, ArmorUntrained{}},
		{"cleric in heavy armor", "class:cleric", "equipment:chain-mail", false, ArmorUntrained{Armor: true}},
		{"barbarian in heavy armor", "class:barbarian", "equipment:chain-mail", false, ArmorUntrained{Armor: true}},
		{"rogue in light armor", "class:rogue", "equipment:leather-armor", false, ArmorUntrained{}},
		{"rogue with a shield alone", "class:rogue", "", true, ArmorUntrained{Shield: true}},
		{"monk in light armor", "class:monk", "equipment:leather-armor", false, ArmorUntrained{Armor: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b := standard(tt.class, 1)
			b.Armor, b.Shield = tt.armor, tt.shield
			got := Derive(b, c).ArmorUntrained
			if got != tt.want {
				t.Errorf("ArmorUntrained = %+v, want %+v", got, tt.want)
			}
			if wantAny := tt.want.Armor || tt.want.Shield; got.Any() != wantAny {
				t.Errorf("Any() = %v, want %v", got.Any(), wantAny)
			}
		})
	}
}

// TestArmorUntrainedMatchesTheIssues checks the sheet's warning and the rule read
// the same answer: an issue exists exactly when the function says so.
func TestArmorUntrainedMatchesTheIssues(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	b := standard("class:wizard", 1)
	b.Armor, b.Shield = "equipment:chain-mail", true
	d := Derive(b, c)
	codes := issueCodes(d)
	for _, want := range []string{IssueArmorProficiency + " full.armor_key", IssueArmorProficiency + " full.shield"} {
		found := false
		for _, code := range codes {
			found = found || code == want
		}
		if !found || !d.ArmorUntrained.Any() {
			t.Errorf("issues = %v, untrained = %+v; want %q and the rule to agree", codes, d.ArmorUntrained, want)
		}
	}
}
