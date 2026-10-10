package play

import (
	"slices"
	"testing"
)

// The conditions only their creature's player and the master read (they have no outward sign)
// are the ones the content file says are owner-class.
func TestTheOwnerOnlyConditionsAreTheOnesTheContentSays(t *testing.T) {
	t.Parallel()
	c, err := testRules()
	if err != nil {
		t.Fatalf("rules.LoadSRD() error = %v", err)
	}
	var owner []string
	for _, k := range c.ConditionKeys() {
		if info, ok := c.ConditionInfo(k); ok && info.Visibility == "owner" {
			owner = append(owner, k)
		}
	}
	slices.Sort(owner)
	want := slices.Clone(ownerOnlyConditions)
	slices.Sort(want)
	if !slices.Equal(owner, want) {
		t.Errorf("owner-class conditions in the content = %v, in the service = %v", owner, want)
	}
}
