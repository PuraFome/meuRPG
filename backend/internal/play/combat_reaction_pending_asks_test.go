package play

import (
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The line of a Cutting Words that cut a held roll whose replay asked its roller, for every ask and every reader
// (combat_reaction_pending_line_test.go has the first case and the words). The roll that asks may be a weapon attack or a spell
// attack, the question Lucky's, the Bardic Inspiration die's (used or kept), and the roller may be a player's character or a creature
// of the master's that holds a die. Whatever the question, a player reads the same words while it waits.

// readers are the people who read the log of a table, by the name a failure gives them, and the line each must read.
type readers map[string]*user

// wantLines checks the master reads exactly one reaction line, master, and every player of the readers reads players.
func (a *armed) wantLines(t *testing.T, e *playv1.Encounter, who readers, master, players string) {
	t.Helper()
	a.wantCutLine(t, e, "the master", a.master, master)
	for name, u := range who {
		a.wantCutLine(t, e, name, u, players)
	}
}

// bardTableReaders are the players of bardTable: the roller's (Pipo's), Toren's, and the bard player's, who has no part in the cut.
func (a *armed) bardTableReaders() readers {
	return readers{"Pipo's player": a.caio, "Toren's player": a.ana, "Orla's player": a.bia}
}

// heldWeaponCut holds Pipo's attack on Bardo for the window, and answers the window with a die of 5: the replay then asks Pipo. It
// returns the question that waits.
func heldWeaponCut(t *testing.T, a *armed, e *playv1.Encounter, roll func(*playv1.RollAttackRequest)) *playv1.HeldRollOffer {
	t.Helper()
	held := a.mustAttack(t, a.caio, e, "Pipo", battleaxe, "Bardo", roll)
	if !held.GetRoll().GetHeldForReaction() {
		t.Fatalf("the attack = %v, want it held for the bard's window", held)
	}
	w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_CUTTING_WORDS)
	if w == nil {
		t.Fatalf("the master's windows = %v, want the bard's Cutting Words", a.windows(t, a.master))
	}
	a.mustAnswer(t, a.master, e, w.GetId(), cutWith(5))
	offers := a.listHeldRolls(t, a.caio)
	if len(offers) != 1 {
		t.Fatalf("Pipo's questions after the window = %v, want one", offers)
	}
	return offers[0]
}

// A Bardic Inspiration die the roller holds is asked about the roll the bard's die left: the line waits, and the die the roller adds
// lands in the total the line then reads (10 + 5 - 5 + 4 = 14, a hit against 12), while a die kept leaves 10, a miss.
func TestACutLineWaitsForTheBardicDieOfTheRollItCut(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		use            bool
		master, player string
	}{
		{"the die used", true,
			"Bardo usou Palavras de Interrupção: d6 (5) subtraído do ataque de Pipo: 19 → 14 contra CA 12: acertou.",
			"Bardo usou Palavras de Interrupção: o ataque de Pipo ainda acertou."},
		{"the die kept", false,
			"Bardo usou Palavras de Interrupção: d6 (5) subtraído do ataque de Pipo: 15 → 10 contra CA 12: errou.",
			"Bardo usou Palavras de Interrupção: o ataque de Pipo errou."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, e := bardTable(t)
			a.giveCombatantDie(t, "Pipo", e)
			offer := heldWeaponCut(t, a, e, d20(10))
			if offer.GetDie() == nil {
				t.Fatalf("Pipo's question = %v, want the die's", offer)
			}
			a.wantLines(t, e, a.bardTableReaders(), cutPendingMaster, cutPendingPlayers)
			if tc.use {
				a.h.roller.queue(4)
			}
			if _, err := a.answerHeldRoll(t, a.caio, offer.GetHoldId(), dieAnswer(tc.use)); err != nil {
				t.Fatalf("AnswerHeldRoll() error = %v", err)
			}
			a.wantLines(t, e, a.bardTableReaders(), tc.master, tc.player)
		})
	}
}

// The master's Inspiração, spent with the attack, is no question of its own: it is chosen before the d20 and gives the roll two dice.
// The replay of a held attack that spent it asks Lucky when both dice show a 1, and the line waits for that answer as it does for any.
func TestACutLineWaitsForTheLuckyAnswerOfARollMadeWithTheFavor(t *testing.T) {
	t.Parallel()
	a, e := bardTable(t)
	f := favorFightOf(t, a, e)
	f.give(t, a.toren)
	offer := heldWeaponCut(t, a, e, func(r *playv1.RollAttackRequest) { r.D20Faces, r.SpendFavor = []int32{1, 1}, true })
	if offer.GetLucky() == nil {
		t.Fatalf("Pipo's question = %v, want Lucky's", offer)
	}
	a.wantLines(t, e, a.bardTableReaders(), cutPendingMaster, cutPendingPlayers)
	if _, err := a.answerHeldRoll(t, a.caio, offer.GetHoldId(), luckyAnswer(15)); err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	a.wantLines(t, e, a.bardTableReaders(),
		"Bardo usou Palavras de Interrupção: d6 (5) subtraído do ataque de Pipo: 20 → 15 contra CA 12: acertou.",
		"Bardo usou Palavras de Interrupção: o ataque de Pipo ainda acertou.")
	if f.favorHolds(t) || f.eventFavor(t, eventAttackRolled) != "true" {
		t.Errorf("the Inspiração is there = %v, the attack says %q, want it spent and the roll to say so", f.favorHolds(t), f.eventFavor(t, eventAttackRolled))
	}
}

// What a player reads while the roll waits is the same words whatever it waits for: Lucky, the Bardic Inspiration die, or the roll the
// Inspiração made. The pending line tells the roller nothing their own question does not.
func TestThePendingLineOfACutIsTheSameForEveryQuestion(t *testing.T) {
	t.Parallel()
	asks := []struct {
		name string
		set  func(t *testing.T, a *armed, e *playv1.Encounter) func(*playv1.RollAttackRequest)
	}{
		{"the Bardic die", func(t *testing.T, a *armed, e *playv1.Encounter) func(*playv1.RollAttackRequest) {
			a.giveCombatantDie(t, "Pipo", e)
			return d20(10)
		}},
		{"the Inspiração", func(t *testing.T, a *armed, e *playv1.Encounter) func(*playv1.RollAttackRequest) {
			favorFightOf(t, a, e).give(t, a.toren)
			return func(r *playv1.RollAttackRequest) { r.D20Faces, r.SpendFavor = []int32{1, 1}, true }
		}},
		{"Lucky", func(*testing.T, *armed, *playv1.Encounter) func(*playv1.RollAttackRequest) { return d20(1) }},
	}
	read := map[string][]string{}
	for _, ask := range asks {
		a, e := bardTable(t)
		heldWeaponCut(t, a, e, ask.set(t, a, e))
		for name, u := range a.bardTableReaders() {
			read[name] = append(read[name], strings.Join(a.cutLinesOf(t, u, e), "\n"))
		}
	}
	for name, lines := range read {
		if len(lines) != len(asks) || slices.Compact(slices.Clone(lines))[0] != cutPendingPlayers || len(slices.Compact(slices.Clone(lines))) != 1 {
			t.Errorf("%s reads %q across the Bardic die, the Inspiração and Lucky, want %q every time", name, lines, cutPendingPlayers)
		}
	}
}

// spellBardTable is a table of a gnome or halfling wizard (Pensantus, Ana's) who casts Acid Arrow at an enemy lore bard, Bardo, whose
// window holds the cast. Bardo's armor class is 12.
func spellBardTable(t *testing.T, race string) (*armed, *playv1.Encounter) {
	t.Helper()
	var bard *charactersv1.Character
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 2, scores16(), []string{battleaxe}, nil)
		a.pens = a.ana.caster(t, a.campaignID, "Pensantus", "class:wizard", race, 3,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, nil, []string{fireBolt},
			[]string{acidArrow}, []string{acidArrow})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, scores16(), []string{rapier}, nil)
		bard = loreBardEnemy(t, a)
	})
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: bard.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1}, reveal: []string{"Bardo"},
		at: map[string][2]int32{"Toren": {3, 3}, "Bardo": {4, 3}, "Pensantus": {10, 3}, "Brisa": {4, 9}},
	})
	return a, a.passTo(t, e, "Pensantus")
}

// spellCut holds Pensantus's cast at Bardo for the window and answers it with a die of 5, and returns the question that waits for her.
func spellCut(t *testing.T, a *armed, e *playv1.Encounter, face int32) *playv1.HeldRollOffer {
	t.Helper()
	held, err := a.castKey(t, a.ana, e, "Pensantus", acidArrow, slotOfLevel(2), a.at(t, "Bardo"), d20Face(face), newKey())
	if err != nil || held.GetCast() != nil || held.GetHeldRollOffer() != nil {
		t.Fatalf("CastSpell() = %v, %v; want it held in the bard's window", held, err)
	}
	w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_CUTTING_WORDS)
	if w == nil {
		t.Fatalf("the master's windows = %v, want the bard's Cutting Words", a.windows(t, a.master))
	}
	a.mustAnswer(t, a.master, e, w.GetId(), cutWith(5))
	offers := a.listHeldRolls(t, a.ana)
	if len(offers) != 1 {
		t.Fatalf("Pensantus's questions after the window = %v, want one", offers)
	}
	return offers[0]
}

// spellReaders are the players of spellBardTable: the caster's, and the other two.
func (a *armed) spellReaders() readers {
	return readers{"Pensantus's player": a.ana, "Toren's player": a.caio, "Brisa's player": a.bia}
}

const (
	spellPendingMaster  = "Bardo usou Palavras de Interrupção: d6 (5) subtraído do ataque de Pensantus (aguardando a rolagem)."
	spellPendingPlayers = "Bardo usou Palavras de Interrupção: o ataque de Pensantus diminuiu."
)

// A halfling's spell attack that the bard's die left on a 1: the line of the cut waits for Lucky's answer, then says what a spell attack
// that asked nothing says: 15 + 5 - 5 = 15 against 12.
func TestACutLineWaitsForTheLuckyAnswerOfASpellAttack(t *testing.T) {
	t.Parallel()
	a, e := spellBardTable(t, "race:halfling")
	offer := spellCut(t, a, e, 1)
	if offer.GetLucky() == nil {
		t.Fatalf("Pensantus's question = %v, want Lucky's", offer)
	}
	a.wantLines(t, e, a.spellReaders(), spellPendingMaster, spellPendingPlayers)
	if _, err := a.answerHeldRoll(t, a.ana, offer.GetHoldId(), luckyAnswer(15)); err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	a.wantLines(t, e, a.spellReaders(),
		"Bardo usou Palavras de Interrupção: d6 (5) subtraído do ataque de Pensantus: 20 → 15 contra CA 12: acertou.",
		"Bardo usou Palavras de Interrupção: o ataque de Pensantus ainda acertou.")
}

// A gnome's spell attack with a Bardic Inspiration die: used (15 + 6 - 5 + 4 = 20, a hit) and kept (16, a hit), the line waiting for
// either.
func TestACutLineWaitsForTheBardicDieOfASpellAttack(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		use            bool
		master, player string
	}{
		{"the die used", true,
			"Bardo usou Palavras de Interrupção: d6 (5) subtraído do ataque de Pensantus: 25 → 20 contra CA 12: acertou.",
			"Bardo usou Palavras de Interrupção: o ataque de Pensantus ainda acertou."},
		{"the die kept", false,
			"Bardo usou Palavras de Interrupção: d6 (5) subtraído do ataque de Pensantus: 21 → 16 contra CA 12: acertou.",
			"Bardo usou Palavras de Interrupção: o ataque de Pensantus ainda acertou."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, e := spellBardTable(t, "race:gnome")
			a.giveCombatantDie(t, "Pensantus", e)
			offer := spellCut(t, a, e, 15)
			if offer.GetDie() == nil {
				t.Fatalf("Pensantus's question = %v, want the die's", offer)
			}
			a.wantLines(t, e, a.spellReaders(), spellPendingMaster, spellPendingPlayers)
			if tc.use {
				a.h.roller.queue(4)
			}
			if _, err := a.answerHeldRoll(t, a.ana, offer.GetHoldId(), dieAnswer(tc.use)); err != nil {
				t.Fatalf("AnswerHeldRoll() error = %v", err)
			}
			a.wantLines(t, e, a.spellReaders(), tc.master, tc.player)
		})
	}
}

// goblinCut is a table where the bard is a player (Ana's Reator): the master's goblin holds a Bardic Inspiration die and attacks Toren,
// the bard cuts the attack with a die of 5, and the replay asks the master, who rolls for the goblin. It returns the question.
func goblinCut(t *testing.T) (*armed, *playv1.Encounter, *playv1.HeldRollOffer) {
	t.Helper()
	a, e := reactorFight(t, loreBard)
	a.giveCombatantDie(t, "Goblin", e)
	w := bardWindow(t, a, e, 8)
	a.mustAnswer(t, a.ana, e, w.GetId(), typedAnswer(5))
	offers := a.listHeldRolls(t, a.master)
	if len(offers) != 1 || offers[0].GetDie() == nil {
		t.Fatalf("the master's questions after the cut = %v, want the goblin's die", offers)
	}
	return a, e, offers[0]
}

// goblinReaders are the players of reactorFight: the bard's, the target's and the third.
func (a *armed) goblinReaders() readers {
	return readers{"the bard's player": a.ana, "Toren's player": a.caio, "Brisa's player": a.bia}
}

const (
	goblinPendingMaster  = "Reator usou Palavras de Interrupção: d6 (5) subtraído do ataque de Goblin (aguardando a rolagem)."
	goblinPendingPlayers = "Reator usou Palavras de Interrupção: o ataque de Goblin diminuiu."
)

// A creature of the master's that holds a die asks the master, and the bard's own player reads no more than the other players do while
// the question waits. The goblin's total is 8 + 4 - 5 = 7; the master's answer settles it.
func TestACutLineOfAPlayersBardWaitsForTheMastersAnswer(t *testing.T) {
	t.Parallel()
	a, e, offer := goblinCut(t)
	a.wantLines(t, e, a.goblinReaders(), goblinPendingMaster, goblinPendingPlayers)
	if _, err := a.answerHeldRoll(t, a.master, offer.GetHoldId(), dieAnswer(false)); err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	a.wantLines(t, e, a.goblinReaders(),
		"Reator usou Palavras de Interrupção: d6 (5) subtraído do ataque de Goblin: 12 → 7 contra CA 12: errou.",
		"Reator usou Palavras de Interrupção: o ataque de Goblin errou.")
}

// A roller the master hides before the answer is a roller the players do not hear: the line stays what it was for them, whatever the roll
// became, with no total, armor class or outcome; the master reads the completed line.
func TestACutLineOfAHiddenRollerStaysNeutralForThePlayers(t *testing.T) {
	t.Parallel()
	a, e, offer := goblinCut(t)
	setHidden(t, a, e, "Goblin", true)
	if _, err := a.answerHeldRoll(t, a.master, offer.GetHoldId(), dieAnswer(false)); err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	a.wantCutLine(t, e, "the master", a.master,
		"Reator usou Palavras de Interrupção: d6 (5) subtraído do ataque de Goblin: 12 → 7 contra CA 12: errou.")
	for name, u := range a.goblinReaders() {
		a.wantCutLine(t, e, name, u, "Reator usou Palavras de Interrupção: o ataque diminuiu.") // the goblin is hidden: no name, no total, no outcome
		for _, l := range a.logEntries(t, u, e) {
			if l.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK {
				t.Errorf("%s reads the hidden goblin's attack %v", name, l)
			}
		}
	}
}

// reactionEntry is the reaction line u reads of the log response, "" when it holds none.
func reactionEntry(res *playv1.ListCombatLogResponse) *playv1.CombatLogEntry {
	for _, r := range res.GetRounds() {
		for _, l := range r.GetEntries() {
			if l.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_REACTION_WINDOW {
				return l
			}
		}
	}
	return nil
}

// since reads the log of u after the number.
func (a *armed) since(t *testing.T, u *user, e *playv1.Encounter, n int64) *playv1.ListCombatLogResponse {
	t.Helper()
	res, err := u.combat.ListCombatLog(t.Context(), connect.NewRequest(&playv1.ListCombatLogRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), SinceSeq: n}))
	if err != nil {
		t.Fatalf("ListCombatLog(since %d) error = %v", n, err)
	}
	return res.Msg
}

// A reader who holds the pending line and asks for what changed after the answer is given the completed one, as a reader who opens the log
// then is; a reader who opens it between the two reads the pending one.
func TestACutLineIsCompletedInTheDeltaOfTheAnswer(t *testing.T) {
	t.Parallel()
	a, e := bardTable(t)
	offer := heldWeaponCut(t, a, e, d20(1))
	held := map[string]int64{}
	users := map[string]*user{"the master": a.master, "Pipo's player": a.caio, "Toren's player": a.ana, "Orla's player": a.bia}
	for name, u := range users {
		first := a.log(t, u, e)
		if got := reactionEntry(first).GetReactionTextPt(); got != cutPendingPlayers && got != cutPendingMaster {
			t.Errorf("%s opens the log in the middle and reads %q, want the pending line", name, got)
		}
		held[name] = first.GetLastSeq()
	}
	if _, err := a.answerHeldRoll(t, a.caio, offer.GetHoldId(), luckyAnswer(15)); err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	for name, u := range users {
		delta := a.since(t, u, e, held[name])
		got := reactionEntry(delta).GetReactionTextPt()
		want := "Bardo usou Palavras de Interrupção: o ataque de Pipo ainda acertou."
		if u == a.master {
			want = "Bardo usou Palavras de Interrupção: d6 (5) subtraído do ataque de Pipo: 20 → 15 contra CA 12: acertou."
		}
		if got != want {
			t.Errorf("%s reads %q in the delta after the answer (full = %v), want %q", name, got, delta.GetFull(), want)
		}
	}
}

// The undo of the roll the answer wrote puts the line back to waiting, for the master and for the players. The reaction's own event is not
// one an undo takes back (it is not in undoableKinds): it is never the action the log offers, and asking to undo it is refused.
func TestUndoingTheAnsweredRollPutsTheCutLineBackToWaiting(t *testing.T) {
	t.Parallel()
	a, e := bardTable(t)
	offer := heldWeaponCut(t, a, e, d20(1))
	if _, err := a.answerHeldRoll(t, a.caio, offer.GetHoldId(), luckyAnswer(15)); err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	a.undoLast(t, e)
	a.wantLines(t, e, a.bardTableReaders(), cutPendingMaster, cutPendingPlayers)

	var reactionID string
	if err := a.h.pool.QueryRow(t.Context(), `SELECT id FROM session_events WHERE kind = $1`, eventReactionAnswered).Scan(&reactionID); err != nil {
		t.Fatalf("read the reaction's event: %v", err)
	}
	if got := a.log(t, a.master, e).GetUndoableEventId(); got == reactionID {
		t.Errorf("the log offers the reaction's own event %s to undo", got)
	}
	if err := a.undo(t, a.master, e, reactionID); err == nil {
		t.Error("UndoLastAction() of the reaction's own event succeeded, want it refused")
	}
	a.wantLines(t, e, a.bardTableReaders(), cutPendingMaster, cutPendingPlayers)
}
