package rules

import (
	"slices"
	"strconv"
	"strings"
	"testing"
)

// #580, board 580-05: a lost prerequisite reads "X pede A, e Tor está no 5: a opção só vale de novo quando Tor cumprir.",
// saying of each missing condition what the character has. The stored message names nobody and nothing hidden; the
// sentence a reader gets (LostPrerequisite.SentencePT) is built for that reader.

func lostIssue(t *testing.T, d Derived, code string) Issue {
	t.Helper()
	i := slices.IndexFunc(d.Issues, func(i Issue) bool { return i.Code == code })
	if i < 0 {
		t.Fatalf("issues = %+v, want %q", d.Issues, code)
	}
	if d.Issues[i].Lost == nil {
		t.Fatalf("issue %+v has no Lost", d.Issues[i])
	}
	return d.Issues[i]
}

func TestALostOptionSaysTheLevelAskedAndTheLevelTheCharacterHas(t *testing.T) {
	t.Parallel()
	c := withOverlay(t, gatedPostureOverlay())
	is := lostIssue(t, Derive(postureBuild(3, postureBull), c), IssueOptionPrerequisite)
	const want = "Postura do Touro pede Guerreiro 5, e Tor está no 3: a opção só vale de novo quando Tor cumprir."
	for _, master := range []bool{false, true} {
		if got := is.Lost.SentencePT("Tor", master); got != want {
			t.Errorf("master=%v: sentence = %q, want %q", master, got, want)
		}
	}
	const stored = "Postura do Touro pede Guerreiro 5, e o personagem está no 3: a opção só vale de novo quando o personagem cumprir."
	if is.Message != stored {
		t.Errorf("message = %q, want %q", is.Message, stored)
	}
}

func TestALostFeatSaysWhatIsMissingInTheSameShape(t *testing.T) {
	t.Parallel()
	c := gatedFeats(t)
	spell := c.c.namePT("spell:magic-missile")
	fighter := choiceBuild("race:human", "", ClassLevel{Class: "class:fighter", Level: 3})
	cases := []struct {
		name string
		b    Build
		feat string
		want string
	}{
		{"a level in the class, too low", warlock(3), featKeyWarlock5, "Bruxo 5 pede Bruxo 5, e Tor está no 3: o talento só vale de novo quando Tor cumprir."},
		{"no level in the class", fighter, featKeyWarlock5, "Bruxo 5 pede Bruxo 5, e Tor não tem nível de Bruxo: o talento só vale de novo quando Tor cumprir."},
		{"a spell", warlock(3), featKeyMagicMissl, "Mísseis pede a magia " + spell + ", que Tor não conhece mais: o talento só vale de novo quando Tor cumprir."},
		{"two options, joined as a list", fighter, featKeyNeedsStyle, "Estilo pede " + optionLabel(c.c.namePT("feature:fighting-style-defense")) + ", que Tor não tem mais e " +
			optionLabel(c.c.namePT("feature:fighting-style-dueling")) + ", que Tor não tem mais: o talento só vale de novo quando Tor cumprir."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := tc.b
			b.Feats = []string{tc.feat}
			is := lostIssue(t, Derive(b, c), IssueFeatPrerequisite)
			if got := is.Lost.SentencePT("Tor", false); got != tc.want {
				t.Errorf("sentence = %q, want %q", got, tc.want)
			}
		})
	}
}

// A feat also asks for an ability score or a proficiency: the sentence says the score the character has, or that it lacks the proficiency.
func TestALostFeatSaysTheScoreAndTheProficiency(t *testing.T) {
	t.Parallel()
	c := gatedFeats(t)
	b := warlock(3)
	d := Derive(b, c)
	score := 0
	for _, s := range d.Abilities {
		if s.Ability == STR {
			score = s.Score
		}
	}
	l := LostPrerequisite{EntryPT: "Peso", Conditions: []LostCondition{
		c.c.lostCondition(FeatUnmet{Kind: FeatUnmetAbilityMinimum, Abilities: []AbilityMinimum{{Ability: STR, Minimum: 13}}}, b, d),
		c.c.lostCondition(FeatUnmet{Kind: FeatUnmetProficiency, Key: "proficiency:heavy-armor"}, b, d),
	}}
	want := "Peso pede " + c.c.namePT(string(STR)) + " 13, e Tor tem " + strconv.Itoa(score) + " e proficiência em " + c.c.namePT("proficiency:heavy-armor") +
		", que Tor não tem: o talento só vale de novo quando Tor cumprir."
	if got := l.SentencePT("Tor", false); got != want {
		t.Errorf("sentence = %q, want %q", got, want)
	}
}

// RN-23: a hidden missing condition reads "uma condição que a mesa não mostra" for a player and is named for the master;
// the stored message never names it, and a player's view of the content holds no name to give.
func TestALostOptionWithAHiddenConditionIsMaskedForAPlayerAndNamedForTheMaster(t *testing.T) {
	t.Parallel()
	o, words := hiddenAskOverlay(t)
	c := withOverlay(t, o)
	is := lostIssue(t, Derive(postureBuild(3, postureHeron), c), IssueOptionPrerequisite)
	player, master := is.Lost.SentencePT("Tor", false), is.Lost.SentencePT("Tor", true)
	if !strings.Contains(player, "pede uma condição que a mesa não mostra") || !strings.HasSuffix(player, ": a opção só vale de novo quando Tor cumprir.") {
		t.Errorf("player's sentence = %q", player)
	}
	for _, w := range words {
		if strings.Contains(player, w) || strings.Contains(is.Message, w) {
			t.Errorf("the player's sentence or the stored message names %q: %q / %q", w, player, is.Message)
		}
	}
	for _, w := range words[:4] {
		if !strings.Contains(master, w) {
			t.Errorf("the master's sentence does not name %q: %q", w, master)
		}
	}

	pv := lostIssue(t, Derive(postureBuild(3, postureHeron), c.ForPlayer()), IssueOptionPrerequisite)
	for _, w := range words {
		if s := pv.Lost.SentencePT("Tor", true); strings.Contains(s, w) {
			t.Errorf("a player's view names %q even read as the master's: %q", w, s)
		}
	}
}
