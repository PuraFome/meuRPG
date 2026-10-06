package rules

import (
	"regexp"
	"strings"
	"testing"
)

// The Multiattack count the engine uses (Derived.AttacksPerAction) must be the one
// the SRD's own Multiattack text gives (MR-042, RN-29). The 5e-database snapshot
// has a few wrong; effects/corrections.json fixes them, and this test reads every
// creature's text to prove no other differs.

var numberWords = map[string]int{"one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6, "seven": 7, "a": 1, "an": 1}

var (
	numWord      = `(one|two|three|four|five|six|seven|a|an)`
	totalPattern = regexp.MustCompile(`\bmakes? ` + numWord + ` [a-z ]*?attacks?:`)
	firstPattern = regexp.MustCompile(`\bmakes ` + numWord + ` [a-z ]*?attacks?\b(.*)`)
	moreAttack   = regexp.MustCompile(`\b(?:and|also)(?: makes| make)? ` + numWord + ` [a-z ]*?attacks?\b`)
	alsoMake     = regexp.MustCompile(`\bcan also make ` + numWord + ` [a-z ]*?attacks?\b`)
)

// textCount reads the number of attacks a Multiattack text gives: "makes three
// attacks: ..." is three, "makes three claw attacks and one bite attack" is four,
// "makes two ... attacks. ... can also make a ... attack" is three. ok is false
// when the text does not fit these shapes.
func textCount(desc string) (n int, ok bool) {
	var sentence string
	for s := range strings.SplitSeq(strings.ReplaceAll(desc, "\n", " "), ". ") {
		if strings.Contains(s, "attack") {
			sentence = s
			break
		}
	}
	if m := totalPattern.FindStringSubmatch(sentence); m != nil {
		return numberWords[m[1]], true
	}
	m := firstPattern.FindStringSubmatch(sentence)
	if m == nil {
		return 0, false
	}
	n = numberWords[m[1]]
	for _, x := range moreAttack.FindAllStringSubmatch(m[2], -1) {
		n += numberWords[x[1]]
	}
	if x := alsoMake.FindStringSubmatch(desc); x != nil && !strings.Contains(sentence, x[0]) {
		n += numberWords[x[1]]
	}
	return n, true
}

// notReadable are the creatures whose text this test cannot read, or whose count
// is the largest of conditional or alternative routines. Each has the count the
// engine must have and why.
var notReadable = map[string]struct {
	count  int
	reason string
}{
	"monster:chuul":         {3, "two pincers, and the tentacles once more when it grapples: the count is the most it can make"},
	"monster:grick":         {2, "one tentacle attack and, if it hits, one beak attack: the most it can make"},
	"monster:hydra":         {5, "as many bites as it has heads: the engine counts its five"},
	"monster:medusa":        {3, "three melee attacks or two ranged: the most it can make"},
	"monster:violet-fungus": {1, "1d4 touches: the engine counts one, and the die stays in the count text"},
}

func TestMultiattackCountsFollowTheSRDText(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for _, e := range mustList(t, c, CreatureFilter{}) {
		cr, _ := c.CreatureByKey(e.Key)
		var text string
		for _, a := range cr.Actions {
			if len(a.Multiattack) > 0 {
				text = a.Text
			}
		}
		if text == "" {
			continue
		}
		d, _ := c.MonsterDerived(e.Key)
		if want, ok := notReadable[e.Key]; ok {
			if d.AttacksPerAction != want.count {
				t.Errorf("%s: %d attacks per action, want %d (%s)", e.Key, d.AttacksPerAction, want.count, want.reason)
			}
			continue
		}
		want, ok := textCount(text)
		if !ok {
			t.Errorf("%s: cannot read the Multiattack text %q; read it by hand and add it to notReadable", e.Key, text)
			continue
		}
		if d.AttacksPerAction != want {
			t.Errorf("%s: %d attacks per action, the SRD's text says %d: %q", e.Key, d.AttacksPerAction, want, text)
		}
	}
	for key, want := range map[string]int{"monster:veteran": 3, "monster:shambling-mound": 2, "monster:brown-bear": 2, "monster:gibbering-mouther": 1} {
		if d, _ := c.MonsterDerived(key); d.AttacksPerAction != want {
			t.Errorf("%s: %d attacks per action, want %d", key, d.AttacksPerAction, want)
		}
	}
}
