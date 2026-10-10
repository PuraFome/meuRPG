package rules

import (
	"regexp"
	"strings"
	"testing"
)

func TestLanguagesPT(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"Common, Goblin":                     "Comum, Goblin",
		"Common, Druidic, Elvish, Sylvan":    "Comum, Dialeto Druídico, Élfico, Silvestre",
		"Abyssal, telepathy 120 ft.":         "Abissal, telepatia 36 m",
		"understands Common but can't speak": "entende Comum, mas não fala",
		"understands Abyssal, Celestial, Infernal, and Primordial but can't speak, telepathy 120 ft.": "entende Abissal, Celestial, Infernal e Primordial, mas não fala, telepatia 36 m",
		"any one language (usually Common)":                         "um idioma qualquer (geralmente Comum)",
		"Thieves' cant plus any two languages":                      "Gírias de Ladrão mais dois idiomas quaisquer",
		"all, telepathy 120 ft.":                                    "todos, telepatia 36 m",
		"Giant Eagle, understands Common and Auran but can't speak": "Águia Gigante, entende Comum e Auran, mas não fala",
		"understands infernal but can't speak":                      "entende Infernal, mas não fala",
		"Auran, understands Common but doesn't speak it":            "Auran, entende Comum, mas não fala",
		"Whatever": "Whatever",
	} {
		if got := LanguagesPT(in); got != want {
			t.Errorf("LanguagesPT(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestNoCreatureLanguageIsLeftInEnglish: across the 334 creatures, none of the
// SRD's language words or phrases survives.
func TestNoCreatureLanguageIsLeftInEnglish(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	left := regexp.MustCompile(`(?i)\b(common|goblin|elvish|dwarvish|draconic|infernal|abyssal|celestial|giant|gnomish|halfling|orc|primordial|sylvan|undercommon|druidic|deep speech|thieves|cant|telepathy|understands|speak|languages?|plus|any|ft|known|knew|creator|life|usually|only|creatures?|other|one|two|four|six|all|and|but|its|the|it)\b`)
	entries, err := c.ListCreatures(CreatureFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 334 {
		t.Fatalf("%d creatures, want 334", len(entries))
	}
	for _, e := range entries {
		cr, ok := c.CreatureByKey(e.Key)
		if !ok {
			t.Fatal(e.Key)
		}
		// "Goblin", "Orc", "Giant" as a name is Portuguese too: the words that are
		// the same in both languages are allowed.
		text := strings.NewReplacer("Goblin", "", "Orc", "", "Halfling", "", "Celestial", "", "Infernal", "", "Primordial", "").Replace(cr.Languages)
		if m := left.FindString(text); m != "" {
			t.Errorf("%s: %q still has %q", e.Key, cr.Languages, m)
		}
	}
}
