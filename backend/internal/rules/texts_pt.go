package rules

import (
	"fmt"
	"io/fs"
	"strings"
)

// SpellTextPT is a spell's text in Portuguese: our own translation of the SRD 5.1
// English (effects/spells_pt.json). Description and HigherLevel have exactly as many
// paragraphs as the English ones; Material is set only for a spell with a material
// component.
type SpellTextPT struct {
	Description []string
	HigherLevel []string
	Material    string
}

// spellTextsFile is effects/spells_pt.json, by spell key.
type spellTextsFile map[string]struct {
	Desc        []string `json:"desc"`
	HigherLevel []string `json:"higher_level"`
	Material    string   `json:"material"`
}

// itemTextsFile is effects/magic_items_pt.json, by item key.
type itemTextsFile map[string]struct {
	Desc []string `json:"desc"`
}

// loadTextsPT reads effects/spells_pt.json and effects/magic_items_pt.json. Both refuse
// a key that is not an SRD spell (or item), a JSON field the format does not know, a
// paragraph count different from the English one, an empty paragraph and a material
// text without a material component (or the reverse). An SRD entry that has no
// Portuguese text is not an error here: it falls back to the English (TestTextsPTCoverEverySpellAndItem
// is what asks for every one).
func (c *content) loadTextsPT(fsys fs.FS) error {
	c.spellTextsPT = map[string]*SpellTextPT{}
	c.itemTextsPT = map[string][]string{}

	const spellsName = "effects/spells_pt.json"
	var spells spellTextsFile
	if err := readJSON(fsys, spellsName, &spells); err != nil {
		return err
	}
	for _, k := range sortedKeys(spells) {
		t := spells[k]
		fail := func(format string, a ...any) error {
			return fmt.Errorf("%s: %s: %s", spellsName, k, fmt.Sprintf(format, a...))
		}
		en, ok := c.spells[k]
		if !ok {
			return fmt.Errorf("%s: %q is not an SRD spell", spellsName, k)
		}
		if err := checkParagraphs(t.Desc, en.Desc, "desc", fail); err != nil {
			return err
		}
		if err := checkParagraphs(t.HigherLevel, en.HigherLevel, "higher_level", fail); err != nil {
			return err
		}
		switch {
		case en.Material == "" && t.Material != "":
			return fail("material, but the spell has no material component")
		case en.Material != "" && strings.TrimSpace(t.Material) == "":
			return fail("the spell has a material component, and material is empty or missing")
		}
		c.spellTextsPT[k] = &SpellTextPT{Description: t.Desc, HigherLevel: t.HigherLevel, Material: t.Material}
	}

	const itemsName = "effects/magic_items_pt.json"
	var items itemTextsFile
	if err := readJSON(fsys, itemsName, &items); err != nil {
		return err
	}
	for _, k := range sortedKeys(items) {
		fail := func(format string, a ...any) error {
			return fmt.Errorf("%s: %s: %s", itemsName, k, fmt.Sprintf(format, a...))
		}
		en, ok := c.magicItems[k]
		if !ok {
			return fmt.Errorf("%s: %q is not an SRD magic item", itemsName, k)
		}
		if err := checkParagraphs(items[k].Desc, en.Desc, "desc", fail); err != nil {
			return err
		}
		c.itemTextsPT[k] = items[k].Desc
	}
	return nil
}

// checkParagraphs refuses a paragraph list of another length than the English one,
// or an empty paragraph where the English one has text. The SRD itself has a few
// empty paragraphs (Scrying's is one): the translation keeps them empty, so the two
// lists stay aligned paragraph by paragraph.
func checkParagraphs(pt, en []string, field string, fail func(string, ...any) error) error {
	if len(pt) != len(en) {
		return fail("%s has %d paragraphs, the English has %d", field, len(pt), len(en))
	}
	for i, p := range pt {
		if strings.TrimSpace(p) == "" && strings.TrimSpace(en[i]) != "" {
			return fail("%s[%d] is empty", field, i)
		}
	}
	return nil
}
