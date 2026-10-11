package rules

import "testing"

// Song of Rest (SRD 5.1, Bard 2): the die is a d6, a d8 from level 9, a d10 from 13
// and a d12 from 17; nobody else has it.
func TestSongOfRestDieFollowsTheBardLevel(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for _, tc := range []struct {
		class string
		level int
		want  int
	}{
		{"class:bard", 1, 0},
		{"class:bard", 2, 6},
		{"class:bard", 8, 6},
		{"class:bard", 9, 8},
		{"class:bard", 12, 8},
		{"class:bard", 13, 10},
		{"class:bard", 16, 10},
		{"class:bard", 17, 12},
		{"class:bard", 20, 12},
		{"class:cleric", 20, 0},
	} {
		if got := Derive(standard(tc.class, tc.level), c).SongOfRestSides; got != tc.want {
			t.Errorf("%s %d: SongOfRestSides = %d, want %d", tc.class, tc.level, got, tc.want)
		}
	}
}

// A multiclass bard keeps the song of its bard level.
func TestSongOfRestOfAMulticlassBard(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	b := standard("class:bard", 9)
	b.Classes = append(b.Classes, ClassLevel{Class: "class:fighter", Level: 3})
	if got := Derive(b, c).SongOfRestSides; got != 8 {
		t.Errorf("bard 9 / fighter 3: SongOfRestSides = %d, want 8", got)
	}
}

// The party hears the largest die among the singers; nobody singing is no song.
func TestSongOfRestFor(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		singers []int
		want    int
	}{
		{"nobody", nil, 0},
		{"the ones who cannot sing", []int{0, 0}, 0},
		{"one bard", []int{6}, 6},
		{"two bards: the larger die", []int{6, 10}, 10},
		{"a die the feature never has", []int{7, 20}, 0},
	} {
		if got := SongOfRestFor(tc.singers); got != tc.want {
			t.Errorf("%s: SongOfRestFor(%v) = %d, want %d", tc.name, tc.singers, got, tc.want)
		}
	}
}

// Loading refuses a song_of_rest effect with a wrong die or with other fields, and
// a die on any other type of effect.
func TestSongOfRestEffectIsClosed(t *testing.T) {
	t.Parallel()
	c := loadForTest(t).c
	for name, e := range map[string]*Effect{
		"a die that does not exist": {Type: "song_of_rest", Sides: 7},
		"no die":                    {Type: "song_of_rest"},
		"another field":             {Type: "song_of_rest", Sides: 6, Count: 2},
		"sides on a note":           {Type: "note", Sides: 6},
	} {
		if err := c.compileEffect("feature:x", e); err == nil {
			t.Errorf("%s: the loader accepted it", name)
		}
	}
	for _, sides := range []int{6, 8, 10, 12} {
		if err := c.compileEffect("feature:x", &Effect{Type: "song_of_rest", Sides: sides}); err != nil {
			t.Errorf("a d%d: %v", sides, err)
		}
	}
}
