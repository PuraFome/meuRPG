package characters

import (
	"slices"
	"strings"
	"testing"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// subclassWithPostures is an invented table subclass whose level 3 feature has an option list.
func subclassWithPostures(featureKey string, optionKeys ...string) *rulesv1.TableSubclass {
	keyAt := func(i int) string {
		if i < len(optionKeys) {
			return optionKeys[i]
		}
		return ""
	}
	return &rulesv1.TableSubclass{
		NamePt:   "Mestre das Posturas",
		ClassKey: "class:fighter",
		Levels: []*rulesv1.TableSubclassLevel{{Level: 3, Features: []*rulesv1.TableFeature{{
			Key: featureKey, NamePt: "Posturas",
			Effects: []*rulesv1.TableEffect{{Type: "choice", Choice: "feature", Count: 1}},
			Options: []*rulesv1.TableFeature{
				{Key: keyAt(0), NamePt: "Postura da Garça"},
				{Key: keyAt(1), NamePt: "Postura do Touro"},
			},
		}}}},
	}
}

// New options get keys made by the server, an update keeps them, and an editor
// that sends them back empty gets the same ones (an option keeps the picks sheets made on it).
func TestOptionKeysAreMadeByTheServerAndKeptOnUpdate(t *testing.T) {
	t.Parallel()
	const entry = "subclass:mestre-das-posturas@mesa"
	fresh := subclassWithPostures("")
	if v := featureKeys(entry, fresh, nil); len(v) != 0 {
		t.Fatalf("violations on a new entry: %v", v)
	}
	feature := fresh.GetLevels()[0].GetFeatures()[0]
	var keys []string
	for _, o := range feature.GetOptions() {
		if !strings.HasPrefix(o.GetKey(), "feature:") || !strings.HasSuffix(o.GetKey(), "@mesa") {
			t.Errorf("option key %q is not a table feature key", o.GetKey())
		}
		keys = append(keys, o.GetKey())
	}
	if len(keys) != 2 || keys[0] == keys[1] {
		t.Fatalf("option keys = %v, want two different ones", keys)
	}

	// The editor sends the keys back: nothing changes.
	again := subclassWithPostures(feature.GetKey(), keys...)
	if v := featureKeys(entry, again, fresh); len(v) != 0 {
		t.Fatalf("violations on an update: %v", v)
	}
	for i, o := range again.GetLevels()[0].GetFeatures()[0].GetOptions() {
		if o.GetKey() != keys[i] {
			t.Errorf("option %d key = %q, want %q kept", i, o.GetKey(), keys[i])
		}
	}

	// It sends them empty: the same names take the same keys.
	blank := subclassWithPostures(feature.GetKey())
	if v := featureKeys(entry, blank, fresh); len(v) != 0 {
		t.Fatalf("violations on an update without option keys: %v", v)
	}
	var got []string
	for _, o := range blank.GetLevels()[0].GetFeatures()[0].GetOptions() {
		got = append(got, o.GetKey())
	}
	if !slices.Equal(got, keys) {
		t.Errorf("keys without being sent back = %v, want %v", got, keys)
	}

	// A key the entry never had is refused at the option's own path.
	forged := subclassWithPostures(feature.GetKey(), "feature:de-outro@mesa")
	v := featureKeys(entry, forged, fresh)
	if len(v) != 1 || v[0].GetField() != "table_subclass.levels[0].features[0].options[0].key" {
		t.Errorf("violations for a forged option key = %v, want one at options[0].key", v)
	}
}

// A content pack carries the option keys of the campaign it came from (the picks of the sheets
// point at them): the import keeps the ones that have the entry's own stem.
func TestImportKeepsTheOptionKeysOfThePack(t *testing.T) {
	t.Parallel()
	const entry = "subclass:mestre-das-posturas@mesa"
	made := subclassWithPostures("")
	featureKeys(entry, made, nil)
	feature := made.GetLevels()[0].GetFeatures()[0]
	packed := subclassWithPostures(feature.GetKey(), feature.GetOptions()[0].GetKey(), feature.GetOptions()[1].GetKey())
	if v := importFeatureKeys(entry, packed, nil); len(v) != 0 {
		t.Fatalf("import refused the pack's keys: %v", v)
	}
	for i, o := range packed.GetLevels()[0].GetFeatures()[0].GetOptions() {
		if o.GetKey() != feature.GetOptions()[i].GetKey() {
			t.Errorf("option %d key = %q, want the pack's %q", i, o.GetKey(), feature.GetOptions()[i].GetKey())
		}
	}
}
