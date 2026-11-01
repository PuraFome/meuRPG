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
// spells' official Portuguese names. It is a note for the master: the NPC's own
// spells in combat come from rules.Content.MonsterCaster (basicDerived), not from
// this text.

var (
	slotLabelRe   = regexp.MustCompile(`^(\d+)(?:st|nd|rd|th) level \((\d+) slots?\)$`)
	perDayRe      = regexp.MustCompile(`^(\d+)/day( each)?$`)
	spellKeyClean = regexp.MustCompile(`[^a-z0-9]+`)
)

var spellAbilityPT = map[rules.Ability]string{rules.INT: "Inteligência", rules.WIS: "Sabedoria", rules.CHA: "Carisma"}

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

// spellcastingBlock is the note of one Spellcasting trait: its header and its
// spell lists, or "" when the trait says nothing the note can use.
func spellcastingBlock(content *rules.Content, t rules.CreatureAbility) string {
	head := spellcastingHead(t)
	var out []string
	for _, line := range strings.Split(t.Text, "\n") {
		if l, ok := spellListLine(content, line); ok {
			out = append(out, l)
		}
	}
	if len(out) == 0 && len(head) == 1 {
		return ""
	}
	return strings.Join(head, ", ") + ".\n" + strings.Join(out, "\n")
}

// spellcastingHead is the note's header, in parts: the kind of spellcasting with
// the caster's level, then the ability, the DC and the attack bonus it finds.
func spellcastingHead(t rules.CreatureAbility) []string {
	head := []string{"Conjuração"}
	if t.Name == "Innate Spellcasting" {
		head[0] = "Conjuração inata"
	}
	n := rules.SpellcastingNumbers(t.Text)
	if n.HasLevel {
		head[0] += fmt.Sprintf(" de %dº nível", n.Level)
	}
	if n.HasAbility {
		head = append(head, "habilidade "+spellAbilityPT[n.Ability])
	}
	if n.HasDC {
		head = append(head, fmt.Sprintf("CD %d", n.DC))
	}
	if n.HasAttack {
		head = append(head, fmt.Sprintf("ataque de magia +%d", n.Attack))
	}
	return head
}

// spellListLine translates a "label: spells" line of the trait; false for a line
// that is no spell list, or whose label the note does not know.
func spellListLine(content *rules.Content, line string) (string, bool) {
	label, list, ok := strings.Cut(strings.TrimPrefix(strings.TrimSpace(line), "- "), ": ")
	if !ok {
		return "", false
	}
	// An intro that ends with the first label ("... components. At will: a, b").
	if i := strings.LastIndex(label, ". "); i >= 0 {
		label = label[i+2:]
	}
	pt, ok := spellLabelPT(label)
	if !ok {
		return "", false
	}
	names := make([]string, 0, 4)
	for _, n := range strings.Split(list, ", ") {
		names = append(names, spellNamePT(content, n))
	}
	return pt + ": " + strings.Join(names, ", ") + ".", true
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
