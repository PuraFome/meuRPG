package rules

import (
	"fmt"
	"io/fs"
	"slices"
)

// ruleTextFiles are the Portuguese texts of the SRD's class and subclass features, racial
// traits, background features and feats: our own translation of the SRD 5.1 English in
// data/, one file per kind, each entry keyed like the content ("feature:<index>",
// "trait:<index>", "background-feature:<index>", "feat:<index>") and holding only `desc`,
// one paragraph per English one.
var ruleTextFiles = []struct {
	file string
	// english returns the English paragraphs of a key, and whether the key is an SRD entry of the kind.
	english func(c *content, key string) ([]string, bool)
	kind    string
}{
	{"effects/features_pt.json", func(c *content, k string) ([]string, bool) {
		f, ok := c.features[k]
		if !ok {
			return nil, false
		}
		return f.Desc, true
	}, "feature"},
	{"effects/traits_pt.json", func(c *content, k string) ([]string, bool) {
		t, ok := c.traits[k]
		if !ok {
			return nil, false
		}
		return t.Desc, true
	}, "trait"},
	{"effects/backgrounds_pt.json", func(c *content, k string) ([]string, bool) {
		for _, b := range c.backgrounds {
			if b.Feature.Key == k {
				return b.Feature.Desc, true
			}
		}
		return nil, false
	}, "background feature"},
	{"effects/feats_pt.json", func(c *content, k string) ([]string, bool) {
		f, ok := c.feats[k]
		if !ok {
			return nil, false
		}
		return f.Desc, true
	}, "feat"},
}

// ruleTextsFile is one of those files, by key.
type ruleTextsFile map[string]struct {
	Desc []string `json:"desc"`
}

// loadRuleTextsPT reads the four files into c.ruleTextsPT. Each refuses a key that is not
// an SRD entry of its kind, a JSON field the format does not know, a paragraph count
// different from the English one and an empty paragraph. An SRD entry a file lacks is not
// an error here: it falls back to the English (TestRuleTextsPTCoverEverySRDEntry asks for
// every one).
func (c *content) loadRuleTextsPT(fsys fs.FS) error {
	c.ruleTextsPT = map[string][]string{}
	for _, rf := range ruleTextFiles {
		var texts ruleTextsFile
		if err := readJSON(fsys, rf.file, &texts); err != nil {
			return err
		}
		for _, k := range sortedKeys(texts) {
			en, ok := rf.english(c, k)
			if !ok {
				return fmt.Errorf("%s: %q is not an SRD %s", rf.file, k, rf.kind)
			}
			fail := func(format string, a ...any) error {
				return fmt.Errorf("%s: %s: %s", rf.file, k, fmt.Sprintf(format, a...))
			}
			if err := checkParagraphs(texts[k].Desc, en, "desc", fail); err != nil {
				return err
			}
			c.ruleTextsPT[k] = texts[k].Desc
		}
	}
	return nil
}

// ruleTextPT is the Portuguese text of an SRD feature, trait, background feature or feat
// and whether it has one. A table entry (key "@mesa") carries the master's Portuguese in
// its own description, so it has none here.
func (c *content) ruleTextPT(key string) ([]string, bool) {
	t, ok := c.ruleTextsPT[key]
	return t, ok
}

// textPT is the Portuguese text of an entry for the screens: the translation of an SRD entry,
// the entry's own text for a table one (which is Portuguese already, so there is no English to
// offer), and missing for an SRD entry with no translation.
func (c *content) textPT(key string, desc []string) (pt []string, missing, only bool) {
	if isTableKey(key) {
		return slices.Clone(desc), false, true
	}
	if t, ok := c.ruleTextPT(key); ok {
		return slices.Clone(t), false, false
	}
	return nil, true, false
}

// withTextPT fills the Portuguese text of a feature.
func (f Feature) withTextPT(c *content) Feature {
	f.DescriptionPT, f.DescriptionPTMissing, f.DescriptionPTOnly = c.textPT(f.Key, f.Description)
	return f
}
