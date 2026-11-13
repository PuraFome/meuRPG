package rules

import "testing"

func TestFeatureTargetsRefusal(t *testing.T) {
	t.Parallel()
	ally := FeatureTargets{Min: 1, Max: 2, Who: FeatureWhoAlly, RangeFt: 30}
	enemy := FeatureTargets{Max: 1, Who: FeatureWhoEnemy, RangeFt: 10}
	cases := []struct {
		name  string
		t     FeatureTargets
		c     FeatureCandidate
		heals bool
		want  string
	}{
		{"an ally in reach", ally, FeatureCandidate{SameSide: true, OnMap: true, Distance: 30}, false, ""},
		{"an ally out of reach", ally, FeatureCandidate{SameSide: true, OnMap: true, Distance: 35}, false, "Fora de alcance: até 9 m"},
		{"an ally off the map", ally, FeatureCandidate{SameSide: true}, false, "Fora do mapa"},
		{"the master is not held to the reach", ally, FeatureCandidate{SameSide: true, IgnoreRange: true}, false, ""},
		{"an enemy for an ally feature", ally, FeatureCandidate{OnMap: true}, false, "Lado errado: só pega aliados"},
		{"an ally for an enemy feature", enemy, FeatureCandidate{SameSide: true, OnMap: true}, false, "Lado errado: só pega inimigos"},
		{"the user among allies", ally, FeatureCandidate{Self: true, SameSide: true}, false, "É você"},
		{"the user with no other ally", ally, FeatureCandidate{Self: true, SameSide: true, OnlyAlly: true}, false, ""},
		{"somebody else for a self feature", FeatureTargets{Max: 1, Who: FeatureWhoSelf}, FeatureCandidate{SameSide: true}, false, "Só você"},
		{"a creature that is down", ally, FeatureCandidate{SameSide: true, Down: true, OnMap: true}, false, "Caído"},
		{"a creature that is down, healed", ally, FeatureCandidate{SameSide: true, Down: true, OnMap: true}, true, ""},
		{"any side", FeatureTargets{Max: 1, Who: FeatureWhoAny}, FeatureCandidate{}, false, ""},
	}
	for _, c := range cases {
		if got := c.t.Refusal(c.c, c.heals); got != c.want {
			t.Errorf("%s: refusal = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestFeatureTargetsAllows(t *testing.T) {
	t.Parallel()
	cases := []struct {
		t    FeatureTargets
		n    int
		want bool
	}{
		{FeatureTargets{}, 0, true},
		{FeatureTargets{}, 1, false},
		{FeatureTargets{Max: 1, Who: FeatureWhoSelf}, 0, true},
		{FeatureTargets{Max: 1, Who: FeatureWhoSelf}, 1, true},
		{FeatureTargets{Max: 1, Who: FeatureWhoSelf}, 2, false},
		{FeatureTargets{Min: 0, Max: 2, Who: FeatureWhoAlly}, 0, false},
		{FeatureTargets{Min: 2, Max: 3, Who: FeatureWhoAlly}, 1, false},
		{FeatureTargets{Min: 2, Max: 3, Who: FeatureWhoAlly}, 3, true},
		{FeatureTargets{Min: 2, Max: 3, Who: FeatureWhoAlly}, 4, false},
	}
	for _, c := range cases {
		if got := c.t.Allows(c.n); got != c.want {
			t.Errorf("%+v.Allows(%d) = %v, want %v", c.t, c.n, got, c.want)
		}
	}
}
