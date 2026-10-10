package rules

import (
	"strings"
	"testing"
	"testing/fstest"
)

// TestRuleTextsPTCoverEverySRDEntry asks for the Portuguese text of every SRD class and
// subclass feature, racial trait, background feature and feat. It fails while one is
// missing: the app falls back to the English for it, and this test keeps that fallback
// from becoming the rule.
func TestRuleTextsPTCoverEverySRDEntry(t *testing.T) {
	t.Parallel()
	c := loadForTest(t).c
	var keys []string
	keys = append(keys, sortedKeys(c.features)...)
	keys = append(keys, sortedKeys(c.traits)...)
	keys = append(keys, sortedKeys(c.feats)...)
	for _, b := range c.backgrounds {
		keys = append(keys, b.Feature.Key)
	}
	var missing []string
	for _, k := range keys {
		if isTableKey(k) {
			continue
		}
		if _, ok := c.ruleTextsPT[k]; !ok {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		show := missing
		if len(show) > 10 {
			show = show[:10]
		}
		t.Errorf("%d features, traits, background features and feats have no Portuguese text, such as %s", len(missing), strings.Join(show, ", "))
	}
}

func TestRuleTextsPTAreExposed(t *testing.T) {
	t.Parallel()
	c := loadForTest(t).c
	for _, k := range []string{"feature:rage", "trait:darkvision", "background-feature:shelter-of-the-faithful", "feat:grappler"} {
		en, ok := ruleTextFiles[0].english(c, k)
		for _, rf := range ruleTextFiles {
			if !ok {
				en, ok = rf.english(c, k)
			}
		}
		pt, missing, only := c.textPT(k, en)
		if !ok || missing || only || len(pt) != len(en) || len(pt) == 0 {
			t.Errorf("%s: ok=%v missing=%v only=%v, %d paragraphs for %d", k, ok, missing, only, len(pt), len(en))
		}
	}
	if pt, missing, only := c.textPT("feature:x@mesa", []string{"Texto da mesa"}); missing || !only || len(pt) != 1 {
		t.Errorf("table feature: missing=%v only=%v pt=%v", missing, only, pt)
	}
	if _, missing, _ := c.textPT("feature:not-translated", []string{"x"}); !missing {
		t.Errorf("an SRD entry without a text is not flagged missing")
	}
}

func TestRuleTextsPTRefusals(t *testing.T) {
	t.Parallel()
	c := loadForTest(t).c
	rage := c.features["feature:rage"].Desc
	n := len(rage)
	paras := func(k int) string { return `["` + strings.Repeat(`a","`, k-1) + `a"]` }
	cases := []struct{ name, file, doc, want string }{
		{"fine", "features", `{"feature:rage":{"desc":` + paras(n) + `}}`, ""},
		{"unknown feature", "features", `{"feature:nope":{"desc":["a"]}}`, "not an SRD feature"},
		{"a trait in the features file", "features", `{"trait:darkvision":{"desc":["a"]}}`, "not an SRD feature"},
		{"unknown trait", "traits", `{"trait:nope":{"desc":["a"]}}`, "not an SRD trait"},
		{"unknown background feature", "backgrounds", `{"background-feature:nope":{"desc":["a"]}}`, "not an SRD background feature"},
		{"unknown feat", "feats", `{"feat:nope":{"desc":["a"]}}`, "not an SRD feat"},
		{"fewer paragraphs", "features", `{"feature:rage":{"desc":["a"]}}`, "paragraphs, the English has"},
		{"empty paragraph", "features", `{"feature:rage":{"desc":` + strings.Replace(paras(n), `"a"`, `" "`, 1) + `}}`, "desc[0] is empty"},
		{"unknown field", "features", `{"feature:rage":{"desc":` + paras(n) + `,"name":"x"}}`, "unknown field"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fresh := &content{features: c.features, traits: c.traits, backgrounds: c.backgrounds, feats: c.feats}
			fs := fstest.MapFS{}
			for _, n := range []string{"features", "traits", "backgrounds", "feats"} {
				fs["effects/"+n+"_pt.json"] = &fstest.MapFile{Data: []byte(`{}`)}
			}
			fs["effects/"+tc.file+"_pt.json"] = &fstest.MapFile{Data: []byte(tc.doc)}
			err := fresh.loadRuleTextsPT(fs)
			switch {
			case tc.want == "" && err != nil:
				t.Errorf("unexpected error: %v", err)
			case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
				t.Errorf("error %v, want one with %q", err, tc.want)
			}
		})
	}
}
