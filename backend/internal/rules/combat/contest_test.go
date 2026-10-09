package combat

import "testing"

func TestCompareContestHigherTotalWinsAndATieChangesNothing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		in, def   int
		want      ContestWinner
		wantLabel string
	}{
		{"the initiator rolls higher", 20, 10, ContestInitiator, "initiator"},
		{"the defender rolls higher", 8, 15, ContestDefender, "defender"},
		{"a tie is nobody's", 12, 12, ContestTie, "tie"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := CompareContest(tt.in, tt.def); got != tt.want {
				t.Errorf("CompareContest(%d, %d) = %v, want %v (%s)", tt.in, tt.def, got, tt.want, tt.wantLabel)
			}
		})
	}
}

func TestMeetsEscapeDCReachingTheDCSucceeds(t *testing.T) {
	t.Parallel()
	if !MeetsEscapeDC(16, 16) || !MeetsEscapeDC(23, 16) {
		t.Error("a total that reaches the escape DC escapes")
	}
	if MeetsEscapeDC(15, 16) {
		t.Error("a total below the escape DC stays grappled")
	}
}

func TestCanGrappleOrShoveAllowsNoMoreThanOneSizeLarger(t *testing.T) {
	t.Parallel()
	tests := []struct {
		attacker, target string
		want             bool
	}{
		{"medium", "medium", true},
		{"medium", "small", true},
		{"medium", "tiny", true},
		{"medium", "large", true},
		{"medium", "huge", false},
		{"small", "large", false},
		{"small", "medium", true},
		{"large", "gargantuan", false},
		{"huge", "gargantuan", true},
		{"medium", "no such size", true}, // unknown is medium
	}
	for _, tt := range tests {
		if got := CanGrappleOrShove(tt.attacker, tt.target); got != tt.want {
			t.Errorf("CanGrappleOrShove(%q, %q) = %v, want %v", tt.attacker, tt.target, got, tt.want)
		}
	}
}

func TestDragHalvesSpeedUnlessTheCreatureIsTwoSizesSmaller(t *testing.T) {
	t.Parallel()
	tests := []struct {
		grappler, grappled string
		want               bool
	}{
		{"medium", "medium", true},
		{"medium", "small", true},
		{"medium", "tiny", false},
		{"large", "small", false},
		{"large", "medium", true},
		{"huge", "medium", false},
		{"small", "large", true},
		{"tiny", "tiny", true},
	}
	for _, tt := range tests {
		if got := DragHalvesSpeed(tt.grappler, tt.grappled); got != tt.want {
			t.Errorf("DragHalvesSpeed(%q, %q) = %v, want %v", tt.grappler, tt.grappled, got, tt.want)
		}
	}
}

func TestPushStepGoesOneSquareStraightAwayFromTheShover(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                         string
		sc, sr, tc, tr, wantC, wantR int
	}{
		{"east", 3, 3, 4, 3, 5, 3},
		{"west", 3, 3, 2, 3, 1, 3},
		{"south", 3, 3, 3, 4, 3, 5},
		{"north-east diagonal", 3, 3, 4, 2, 5, 1},
		{"south-west diagonal", 3, 3, 2, 4, 1, 5},
	}
	for _, tt := range tests {
		c, r := PushStep(tt.sc, tt.sr, tt.tc, tt.tr)
		if c != tt.wantC || r != tt.wantR {
			t.Errorf("%s: PushStep = (%d,%d), want (%d,%d)", tt.name, c, r, tt.wantC, tt.wantR)
		}
	}
}

func TestResolveCheckModeCancelsAdvantageAgainstDisadvantage(t *testing.T) {
	t.Parallel()
	adv := CheckSource{Kind: "help", Advantage: true}
	dis := CheckSource{Kind: "poisoned"}
	tests := []struct {
		name    string
		sources []CheckSource
		want    CheckMode
		dice    int
	}{
		{"none", nil, CheckNormal, 1},
		{"one advantage", []CheckSource{adv}, CheckAdvantage, 2},
		{"two advantages roll one extra die only", []CheckSource{adv, adv}, CheckAdvantage, 2},
		{"one disadvantage", []CheckSource{dis}, CheckDisadvantage, 2},
		{"advantage and disadvantage cancel", []CheckSource{adv, dis}, CheckNormal, 1},
		{"many disadvantages and one advantage still cancel", []CheckSource{dis, dis, adv}, CheckNormal, 1},
	}
	for _, tt := range tests {
		got := ResolveCheckMode(tt.sources)
		if got != tt.want || got.Dice() != tt.dice {
			t.Errorf("%s: mode %v with %d dice, want %v with %d", tt.name, got, got.Dice(), tt.want, tt.dice)
		}
	}
}

func TestPickD20KeepsTheHigherForAdvantageAndTheLowerForDisadvantage(t *testing.T) {
	t.Parallel()
	if got := CheckAdvantage.PickD20([]int{5, 17}); got != 17 {
		t.Errorf("advantage of 5 and 17 = %d, want 17", got)
	}
	if got := CheckDisadvantage.PickD20([]int{17, 5}); got != 5 {
		t.Errorf("disadvantage of 17 and 5 = %d, want 5", got)
	}
	if got := CheckNormal.PickD20([]int{9}); got != 9 {
		t.Errorf("normal d20 = %d, want 9", got)
	}
	if got := CheckNormal.PickD20(nil); got != 0 {
		t.Errorf("no face = %d, want 0", got)
	}
}

func TestPassivePerceptionOfAddsFiveForAdvantageAndSubtractsFiveForDisadvantage(t *testing.T) {
	t.Parallel()
	// The SRD's example: Wisdom 15 (+2) and proficiency (+2) is 14.
	if got := PassivePerceptionOf(4, CheckNormal); got != 14 {
		t.Errorf("passive = %d, want 14", got)
	}
	if got := PassivePerceptionOf(4, CheckAdvantage); got != 19 {
		t.Errorf("with advantage = %d, want 19", got)
	}
	if got := PassivePerceptionOf(4, CheckDisadvantage); got != 9 {
		t.Errorf("with disadvantage = %d, want 9", got)
	}
}

func TestNoticesHiderNeedsTheStealthTotalToBeatThePassivePerception(t *testing.T) {
	t.Parallel()
	if NoticesHider(19, 10) {
		t.Error("19 beats a passive 10: the hider is not noticed")
	}
	if !NoticesHider(10, 10) {
		t.Error("a tie keeps the hider noticed")
	}
	if !NoticesHider(8, 10) {
		t.Error("a lower total is noticed")
	}
}

func TestGroupCheckPassesWhenAtLeastHalfPass(t *testing.T) {
	t.Parallel()
	tests := []struct {
		passed, asked int
		needed        int
		want          bool
	}{
		{2, 5, 3, false},
		{3, 5, 3, true},
		{2, 4, 2, true},
		{1, 4, 2, false},
		{1, 1, 1, true},
		{0, 1, 1, false},
		{0, 0, 0, false},
	}
	for _, tt := range tests {
		if got := GroupCheckNeeded(tt.asked); got != tt.needed {
			t.Errorf("GroupCheckNeeded(%d) = %d, want %d", tt.asked, got, tt.needed)
		}
		if got := GroupCheckPasses(tt.passed, tt.asked); got != tt.want {
			t.Errorf("GroupCheckPasses(%d, %d) = %v, want %v", tt.passed, tt.asked, got, tt.want)
		}
	}
}

func TestNoticesNoThreatComparesEveryHiderWithTheCreature(t *testing.T) {
	t.Parallel()
	brisa := HideTotals{ID: "brisa", Total: 19}
	clumsy := HideTotals{ID: "toren", Total: 8}
	if !NoticesNoThreat(9, []HideTotals{brisa}) {
		t.Error("a 19 beats a passive 9: the creature notices no threat")
	}
	if NoticesNoThreat(10, []HideTotals{brisa, clumsy}) {
		t.Error("the clumsy hider is noticed: the creature notices a threat, whatever the best total is")
	}
	if NoticesNoThreat(9, nil) {
		t.Error("nobody hides: the creature notices them")
	}
}

func TestHelpLastsThroughTheEndOfTheHelpersNextTurn(t *testing.T) {
	t.Parallel()
	// The helper is third in the order (index 2) and helped in round 2.
	tests := []struct {
		name           string
		round, current int
		want           bool
	}{
		{"later in the same round", 2, 3, true},
		{"next round before the helper's turn", 3, 0, true},
		{"next round on the helper's turn", 3, 2, true},
		{"next round after the helper's turn", 3, 3, false},
		{"two rounds later", 4, 0, false},
	}
	for _, tt := range tests {
		if got := HelpLastsThrough(2, tt.round, tt.current, 2); got != tt.want {
			t.Errorf("%s: HelpLastsThrough = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestSurprisedUntilTurnEndsLastsThroughTheFirstTurn(t *testing.T) {
	t.Parallel()
	// The creature is second in the order (index 1).
	tests := []struct {
		name           string
		running        bool
		round, current int
		want           bool
	}{
		{"before the combat begins", false, 0, 0, true},
		{"round 1, before its turn", true, 1, 0, true},
		{"round 1, on its turn", true, 1, 1, true},
		{"round 1, its turn ended", true, 1, 2, false},
		{"round 2", true, 2, 0, false},
	}
	for _, tt := range tests {
		if got := SurprisedUntilTurnEnds(tt.running, tt.round, tt.current, 1); got != tt.want {
			t.Errorf("%s: SurprisedUntilTurnEnds = %v, want %v", tt.name, got, tt.want)
		}
	}
}
