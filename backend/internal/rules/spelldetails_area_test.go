package rules

import "testing"

// The words of the SRD decide how an area is laid out: a spell that "spreads around
// corners" reaches the part of its shape connected to the origin (Fireball), and a
// line is as wide as its text says (Gust of Wind 10 ft; Lightning Bolt and the
// others 5 ft).
func TestAreaLayoutComesFromTheSpellText(t *testing.T) {
	t.Parallel()
	c, err := LoadSRD()
	if err != nil {
		t.Fatalf("LoadSRD() error = %v", err)
	}
	corners := map[string]bool{
		"spell:fireball": true, "spell:cloudkill": true, "spell:stinking-cloud": true,
		"spell:burning-hands": false, "spell:lightning-bolt": false, "spell:thunderwave": false,
		"spell:message": false, // "around corners", but no area
	}
	for key, want := range corners {
		d, ok := c.SpellDetails(key)
		if !ok {
			t.Fatalf("SpellDetails(%s) not found", key)
		}
		if got := d.SpreadsAroundCorners(); got != want {
			t.Errorf("%s SpreadsAroundCorners() = %v, want %v", key, got, want)
		}
	}
	widths := map[string]int{
		"spell:lightning-bolt": 5, "spell:gust-of-wind": 10, "spell:sunbeam": 5,
		"spell:burning-hands": 0, "spell:fireball": 0, // not lines
	}
	for key, want := range widths {
		d, _ := c.SpellDetails(key)
		if got := d.AreaWidthFt(); got != want {
			t.Errorf("%s AreaWidthFt() = %d, want %d", key, got, want)
		}
	}
	// The shapes the placement is built on.
	shapes := map[string]struct {
		shape string
		size  int
	}{
		"spell:fireball": {ShapeSphere, 20}, "spell:burning-hands": {ShapeCone, 15}, "spell:lightning-bolt": {ShapeLine, 100}, "spell:thunderwave": {ShapeCube, 15},
	}
	for key, want := range shapes {
		d, _ := c.SpellDetails(key)
		if d.Target.Kind != TargetArea || d.Target.Shape != want.shape || d.Target.SizeFt != want.size {
			t.Errorf("%s target = %+v, want an area %s of %d ft", key, d.Target, want.shape, want.size)
		}
	}
}
