package play

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The release of a readied action at a table with physical dice takes the dice a cast or an attack takes (#938): the
// answer's fields are copied one to one into the request the release builds.

func useFaces(roll func(*playv1.AnswerReactionRequest), faces ...int32) func(*playv1.AnswerReactionRequest) {
	return func(r *playv1.AnswerReactionRequest) {
		useSpellAnswer(r)
		if roll != nil {
			roll(r)
		}
		r.D20Faces = faces
	}
}

func withMode(mode playv1.RollMode, reason string, faces ...int32) func(*playv1.AnswerReactionRequest) {
	return func(r *playv1.AnswerReactionRequest) {
		useSpellAnswer(r)
		r.D20Faces, r.RollMode, r.ModeReason = faces, mode, reason
	}
}

func withPool(sum int32) func(*playv1.AnswerReactionRequest) {
	return func(r *playv1.AnswerReactionRequest) {
		useSpellAnswer(r)
		r.Roll = &playv1.AnswerReactionRequest_PoolSum{PoolSum: sum}
	}
}

func (a *armed) readiedAttackLine(t *testing.T, e *playv1.Encounter) *playv1.CombatLogEntry {
	t.Helper()
	for _, r := range a.log(t, a.master, e).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK {
				return en
			}
		}
	}
	return nil
}

// wantFaces checks the d20 faces the attack line of the log shows.
func (a *armed) wantFaces(t *testing.T, e *playv1.Encounter, want []int32) {
	t.Helper()
	line := a.readiedAttackLine(t, e)
	if line == nil {
		t.Fatalf("no attack line in the log")
	}
	if got := line.GetAttackRoll().GetFaces(); !slices.Equal(got, want) {
		t.Errorf("the attack's d20 faces = %v, want %v", got, want)
	}
}

func TestAReadiedWeaponAttackTakesTypedDiceAtAPhysicalTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		by   func(a *armed) *user
		set  func(*playv1.AnswerReactionRequest)
		want []int32
	}{
		{"the player, the old typed field", func(a *armed) *user { return a.caio }, useTyped(15), []int32{15}},
		{"the player, a disadvantage pair", func(a *armed) *user { return a.caio }, withMode(playv1.RollMode_ROLL_MODE_DISADVANTAGE, "no escuro", 15, 4), []int32{15, 4}},
		{"the master, a single face in d20_faces", func(a *armed) *user { return a.master }, useFaces(nil, 15), []int32{15}},
		{"the master, an advantage pair", func(a *armed) *user { return a.master }, withMode(playv1.RollMode_ROLL_MODE_ADVANTAGE, "flanco", 15, 4), []int32{15, 4}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := newArmed(t)
			a.forceDice(t, campaignsv1.DiceMode_DICE_MODE_PHYSICAL)
			e := a.readyThenPass(t, nil)
			w := a.mustFire(t, e, "Toren")
			res := a.mustAnswer(t, tc.by(a), e, w.GetId(), tc.set)
			if !res.GetResult().GetUsed() {
				t.Fatalf("the use's result = %v", res.GetResult())
			}
			if a.readiedOpen(t, "Toren") || a.readiedStatus(t, "Toren") != readiedFired {
				t.Errorf("window open %v, row %q, want fired", a.readiedOpen(t, "Toren"), a.readiedStatus(t, "Toren"))
			}
			a.wantFaces(t, e, tc.want)
		})
	}
}

func TestAReadiedWeaponAttackRefusesTheAppsDiceAtAPhysicalTable(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	a.forceDice(t, campaignsv1.DiceMode_DICE_MODE_PHYSICAL)
	e := a.readyThenPass(t, nil)
	w := a.mustFire(t, e, "Toren")
	_, err := a.answerReaction(t, a.caio, e, w.GetId(), useSpellAnswer)
	wantCode(t, "app dice at a physical table", err, connect.CodeFailedPrecondition)
}

func TestAReadiedWeaponAttackInAppIsUnchanged(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	a.forceDice(t, campaignsv1.DiceMode_DICE_MODE_APP)
	e := a.readyThenPass(t, nil)
	w := a.mustFire(t, e, "Toren")
	a.mustAnswer(t, a.caio, e, w.GetId(), useSpellAnswer)
	if a.readiedStatus(t, "Toren") != readiedFired {
		t.Errorf("row %q, want fired", a.readiedStatus(t, "Toren"))
	}
}

func TestAReadiedSpellAttackTakesTypedFaces(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	a.forceDice(t, campaignsv1.DiceMode_DICE_MODE_PHYSICAL)
	e := a.readyForRelease(t, fireBolt, 0, "Goblin")
	w := a.mustFire(t, e, "Pensantus")
	res := a.mustAnswer(t, a.ana, e, w.GetId(), withMode(playv1.RollMode_ROLL_MODE_DISADVANTAGE, "no escuro", 12, 3))
	if !res.GetResult().GetUsed() {
		t.Fatalf("the use's result = %v", res.GetResult())
	}
	a.wantFaces(t, e, []int32{12, 3})
}

func TestAReadiedPoolSpellTakesTheTypedSum(t *testing.T) {
	t.Parallel()
	for name, by := range map[string]func(a *armed) *user{
		"the player": func(a *armed) *user { return a.ana },
		"the master": func(a *armed) *user { return a.master },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a := newCasters(t)
			a.forceDice(t, campaignsv1.DiceMode_DICE_MODE_PHYSICAL)
			e := a.start(t, plan{
				npcs: []*playv1.Participant{{CharacterId: a.capitao.GetId()}}, npcRolls: []int{1}, reveal: []string{"Capitão Goblin"}, theatre: true,
				players: map[string]int32{"Pensantus": 20, "Toren": 15, "Brisa": 10},
			})
			a.mustReady(t, a.ana, a.readySpell(t, e, "Pensantus", sleepSpell, 1, "Capitão Goblin"))
			e = a.mustEndTurn(t, a.master, e)
			w := a.mustFire(t, e, "Pensantus")
			a.mustAnswer(t, by(a), e, w.GetId(), withPool(8))
			if a.castLine(t, e, sleepSpell) == nil {
				t.Errorf("no Sleep line in the log")
			}
		})
	}
}

func TestAReadiedReleaseRefusesDiceItDoesNotUseAndStaysOpen(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		set  func(*playv1.AnswerReactionRequest)
	}{
		{"pool_sum on a weapon attack", withPool(8)},
		{"d20_faces with roll_in_app", func(r *playv1.AnswerReactionRequest) {
			useSpellAnswer(r)
			r.Roll, r.D20Faces = &playv1.AnswerReactionRequest_RollInApp{RollInApp: true}, []int32{9}
		}},
		{"d20_faces with typed", func(r *playv1.AnswerReactionRequest) {
			useTyped(9)(r)
			r.D20Faces = []int32{9}
		}},
		{"three faces", useFaces(nil, 1, 2, 3)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := newArmed(t)
			a.forceDice(t, campaignsv1.DiceMode_DICE_MODE_PHYSICAL)
			e := a.readyThenPass(t, nil)
			w := a.mustFire(t, e, "Toren")
			_, err := a.answerReaction(t, a.caio, e, w.GetId(), tc.set)
			wantCode(t, tc.name, err, connect.CodeInvalidArgument)
			if !a.readiedOpen(t, "Toren") || a.readiedStatus(t, "Toren") != readiedWaiting {
				t.Errorf("the refused answer left the window open %v and row %q", a.readiedOpen(t, "Toren"), a.readiedStatus(t, "Toren"))
			}
		})
	}
}

func TestAReadiedSpellRefusesDiceItDoesNotUse(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		spell string
		set   func(*playv1.AnswerReactionRequest)
	}{
		{"d20_faces on a pool spell", sleepSpell, useFaces(nil, 9)},
		{"roll_mode on a pool spell", sleepSpell, withMode(playv1.RollMode_ROLL_MODE_DISADVANTAGE, "no escuro", 9)},
		{"pool_sum on a spell attack", fireBolt, withPool(8)},
		{"pool_sum on a save spell", burningHands, withPool(8)},
		{"d20_faces on a save spell", burningHands, useFaces(nil, 9)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := newCasters(t)
			a.forceDice(t, campaignsv1.DiceMode_DICE_MODE_PHYSICAL)
			e := a.start(t, plan{
				npcs: []*playv1.Participant{{CharacterId: a.capitao.GetId()}}, npcRolls: []int{1}, reveal: []string{"Capitão Goblin"}, theatre: true,
				players: map[string]int32{"Pensantus": 20, "Toren": 15, "Brisa": 10},
			})
			slot := int32(1)
			if tc.spell == fireBolt {
				slot = 0
			}
			a.mustReady(t, a.ana, a.readySpell(t, e, "Pensantus", tc.spell, slot, "Capitão Goblin"))
			e = a.mustEndTurn(t, a.master, e)
			w := a.mustFire(t, e, "Pensantus")
			_, err := a.answerReaction(t, a.ana, e, w.GetId(), tc.set)
			wantCode(t, tc.name, err, connect.CodeInvalidArgument)
			if !a.readiedOpen(t, "Pensantus") || a.readiedStatus(t, "Pensantus") != readiedWaiting {
				t.Errorf("the refused answer left the window open %v and row %q", a.readiedOpen(t, "Pensantus"), a.readiedStatus(t, "Pensantus"))
			}
		})
	}
}

func TestAReadiedTypedReleaseReplaysWithTheSameKey(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	a.forceDice(t, campaignsv1.DiceMode_DICE_MODE_PHYSICAL)
	e := a.readyThenPass(t, nil)
	w := a.mustFire(t, e, "Toren")
	set := withMode(playv1.RollMode_ROLL_MODE_DISADVANTAGE, "no escuro", 15, 4)
	key := newKey()
	again := func(r *playv1.AnswerReactionRequest) { set(r); r.IdempotencyKey = key }
	first := a.mustAnswer(t, a.caio, e, w.GetId(), again)
	second := a.mustAnswer(t, a.caio, e, w.GetId(), again)
	if first.GetResult().GetUsed() != second.GetResult().GetUsed() || !second.GetResult().GetUsed() {
		t.Errorf("replay result = %v, first %v", second.GetResult(), first.GetResult())
	}
	if n := len(a.firedLines(t, a.master, e)); n != 1 {
		t.Errorf("fired lines = %d, want 1", n)
	}
}

// What the cast itself does with a physical d20 for the rays of Scorching Ray: it refuses (combat_cast.go
// checkSpellAttack): d20_face and d20_faces fit a single ray.
func TestCastScorchingRayWithPhysicalDiceIsRefusedForSeveralRays(t *testing.T) {
	t.Parallel()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
		a.pens = a.ana.caster(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 3,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, nil, []string{fireBolt},
			[]string{scorchingRay}, []string{scorchingRay})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{rapier}, nil)
	})
	a.forceDice(t, campaignsv1.DiceMode_DICE_MODE_PHYSICAL)
	e := a.castersFight(t, 1)
	for name, roll := range map[string]func(*playv1.CastSpellRequest){
		"d20_face":  func(r *playv1.CastSpellRequest) { r.Roll = &playv1.CastSpellRequest_D20Face{D20Face: 12} },
		"d20_faces": func(r *playv1.CastSpellRequest) { r.D20Faces = []int32{12} },
	} {
		_, err := a.cast(t, a.ana, e, "Pensantus", scorchingRay, slotOfLevel(2), []*playv1.SpellTarget{darts(a, t, "Goblin", 3)}, roll)
		wantCode(t, "Scorching Ray with "+name, err, connect.CodeInvalidArgument)
		t.Logf("%s: %v", name, err)
	}
}
