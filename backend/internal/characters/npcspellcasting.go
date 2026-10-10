package characters

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The NPC made from a spellcaster (MR-042) carries the creature's Spellcasting as
// text in the sheet's description, in Portuguese: the ability, the spell save DC,
// the spell attack bonus and the spells by level with their slots, with the
// spells' official Portuguese names. It is a note for the master: the basic sheet
// has no spell engine, so the NPC casts nothing by itself and the master reads
// the DC and rolls (the spell's own save or attack is on the spell's page).

var (
	spellDCRe     = regexp.MustCompile(`spell save DC (\d+)`)
	spellAttackRe = regexp.MustCompile(`\+(\d+) to hit with spell attacks`)
	spellLevelRe  = regexp.MustCompile(`(\d+)(?:st|nd|rd|th)-level spellcaster`)
	spellAbility  = regexp.MustCompile(`(?i)casting ability is (Intelligence|Wisdom|Charisma)`)
	slotLabelRe   = regexp.MustCompile(`^(\d+)(?:st|nd|rd|th) level \((\d+) slots?\)$`)
	perDayRe      = regexp.MustCompile(`^(\d+)/day( each)?$`)
	spellKeyClean = regexp.MustCompile(`[^a-z0-9]+`)
)

var spellAbilityPT = map[string]string{"Intelligence": "Inteligência", "Wisdom": "Sabedoria", "Charisma": "Carisma"}

// maxSpellNoteLength keeps the note well inside the description's limit.
const maxSpellNoteLength = 1500

// spellcastingNote is the Portuguese note of the creature's Spellcasting or
// Innate Spellcasting traits, or "" when it has none.
func spellcastingNote(content *rules.Content, c rules.Creature) string {
	var blocks []string
	for _, t := range c.Traits {
		if !strings.Contains(t.Name, "pellcasting") {
			continue
		}
		if b := spellcastingBlock(content, t); b != "" {
			blocks = append(blocks, b)
		}
	}
	return limitNote(strings.Join(blocks, "\n"))
}

func limitNote(s string) string {
	if len(s) <= maxSpellNoteLength {
		return s
	}
	cut := strings.LastIndex(s[:maxSpellNoteLength], "\n")
	if cut <= 0 {
		return ""
	}
	return s[:cut] + "\n(Lista de magias cortada: veja a ficha da criatura.)"
}

func spellcastingBlock(content *rules.Content, t rules.CreatureAbility) string {
	lines := strings.Split(t.Text, "\n")
	var head []string
	if t.Name == "Innate Spellcasting" {
		head = append(head, "Conjuração inata")
	} else {
		head = append(head, "Conjuração")
	}
	if m := spellLevelRe.FindStringSubmatch(t.Text); m != nil {
		head[0] += " de " + m[1] + "º nível"
	}
	if m := spellAbility.FindStringSubmatch(t.Text); m != nil {
		head = append(head, "habilidade "+spellAbilityPT[m[1]])
	}
	if m := spellDCRe.FindStringSubmatch(t.Text); m != nil {
		head = append(head, "CD "+m[1])
	}
	if m := spellAttackRe.FindStringSubmatch(t.Text); m != nil {
		head = append(head, "ataque de magia +"+m[1])
	}
	var out []string
	for _, line := range lines {
		label, list, ok := strings.Cut(strings.TrimPrefix(strings.TrimSpace(line), "- "), ": ")
		if !ok {
			continue
		}
		// An intro that ends with the first label ("... components. At will: a, b").
		if i := strings.LastIndex(label, ". "); i >= 0 {
			label = label[i+2:]
		}
		pt, ok := spellLabelPT(label)
		if !ok {
			continue
		}
		names := make([]string, 0, 4)
		for _, n := range strings.Split(list, ", ") {
			names = append(names, spellNamePT(content, n))
		}
		out = append(out, pt+": "+strings.Join(names, ", ")+".")
	}
	if len(out) == 0 && len(head) == 1 {
		return ""
	}
	return strings.Join(head, ", ") + ".\n" + strings.Join(out, "\n")
}

// spellLabelPT translates the label of a spell list line.
func spellLabelPT(label string) (string, bool) {
	label = strings.TrimSpace(label)
	switch strings.ToLower(label) {
	case "cantrips (at will)":
		return "Truques (à vontade)", true
	case "at will":
		return "À vontade", true
	}
	if m := slotLabelRe.FindStringSubmatch(label); m != nil {
		word := "espaços"
		if m[2] == "1" {
			word = "espaço"
		}
		return fmt.Sprintf("%sº nível (%s %s)", m[1], m[2], word), true
	}
	if m := perDayRe.FindStringSubmatch(label); m != nil {
		each := ""
		if m[2] != "" {
			each = " cada"
		}
		return m[1] + "/dia" + each, true
	}
	return "", false
}

// spellNamePT is the spell's Portuguese name, or the SRD's text when the name
// is not one the content knows ("detect magic (self only)" keeps its note).
func spellNamePT(content *rules.Content, raw string) string {
	name := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(raw), "."))
	note := ""
	if i := strings.Index(name, " ("); i >= 0 {
		note = name[i:]
		name = name[:i]
	}
	key := "spell:" + strings.Trim(spellKeyClean.ReplaceAllString(strings.ReplaceAll(strings.ToLower(name), "'", ""), "-"), "-")
	if pt := content.NamePT(key); pt != "" && pt != name {
		return pt + note
	}
	return name + note
}
