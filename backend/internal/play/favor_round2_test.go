package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// newFavorFightOf is newFavorFight with the three heroes of one race.
func newFavorFightOf(t *testing.T, race string) *favorFight {
	t.Helper()
	a := newCastersOf(t, 3, race)
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Pensantus": {10, 3}, "Brisa": {4, 9}},
	})
	return favorFightOf(t, a, e)
}

// B1: a halfling's spell attack goes through the gate (the d20 may show a 1), and a d20 that shows no 1 is settled at once: the cast
// spends the Inspiration once, goes through, and the roll says it was made with it.
func TestFavorSpellAttackOfALuckyCasterSpendsOnce(t *testing.T) {
	t.Parallel()
	f := newFavorFightOf(t, "race:halfling")
	f.e = f.passTo(t, f.e, "Brisa")
	f.give(t, f.bri)
	f.h.roller.queue(4, 15)
	res, err := f.castKey(t, f.bia, f.e, "Brisa", guidingBolt, slotOfLevel(1), f.at(t, "Goblin"), func(r *playv1.CastSpellRequest) {
		r.Roll, r.SpendFavor = &playv1.CastSpellRequest_RollInApp{RollInApp: true}, true
	}, newKey())
	if err != nil {
		t.Fatalf("the cast error = %v, want it to go through", err)
	}
	got := castRolledOf(res)
	if len(got.faces) != 2 || got.mode != playv1.RollMode_ROLL_MODE_ADVANTAGE || !got.hasFavorSource() {
		t.Errorf("the roll = %+v, want two dice, an advantage and the source of the Inspiration", got)
	}
	if f.favorHolds(t) || f.eventFavor(t, eventSpellCast) != "true" {
		t.Errorf("the Inspiration is there = %v, the event says %q, want it spent once and the event to say so", f.favorHolds(t), f.eventFavor(t, eventSpellCast))
	}
}

// --- RN-10: ADDS_NOTHING is judged on what the roller may read ---

// hiddenEffect puts an effect on a combatant of the fight that only the master reads (player_visible off) with the modifiers given
// (a JSON list), or one everybody reads (visible), for the tests of what the Inspiração adds.
func (f *favorFight) hiddenEffect(t *testing.T, label string, visible bool, mods string) {
	t.Helper()
	f.execSQL(t, `INSERT INTO combatant_states (encounter_id, combatant_id, kind, started_round, created_at, group_id, source_key, source_kind, duration_kind, modifiers, player_visible)
		SELECT encounter_id, id, 'effect', 1, now(), gen_random_uuid(), 'spell:bless', 'master', 'until_dismissed', $3::jsonb, $4 FROM combatants WHERE encounter_id = $1 AND label = $2`,
		f.e.GetId(), label, mods, visible)
}

const (
	attackAdv   = `{"kind":"attack_advantage"}`
	attackDis   = `{"kind":"attack_disadvantage"}`
	saveAdvWis  = `{"kind":"save_advantage","abilities":["wis"]}`
	saveDisWis  = `{"kind":"save_disadvantage","abilities":["wis"]}`
	saveAdvCon  = `{"kind":"save_advantage","abilities":["con"]}`
	saveDisCon  = `{"kind":"save_disadvantage","abilities":["con"]}`
	checkAdvAll = `{"kind":"check_advantage","abilities":["str","dex","con","int","wis","cha"]}`
	checkDisAll = `{"kind":"check_disadvantage","abilities":["str","dex","con","int","wis","cha"]}`
)

// spendOutcome is what a roll made with the spend answered, as the player reads it, for the comparison with the roll that had no hidden source.
type spendOutcome struct {
	err        string
	faces      int
	mode       string
	favorShown bool
	spent      bool
	eventFavor string
	stored     bool
}

// hiddenCase is a hidden circumstance the player must not learn of by a refusal: an advantage the Inspiração adds nothing to, or an advantage
// and a disadvantage that cancel (with the Inspiração the roll has two advantages against one disadvantage, which cancel again: a straight
// roll, one d20, and the Inspiração is spent all the same, as at a table where the player does not know).
type hiddenCase struct {
	mods  string
	faces int
}

func hiddenCases(adv, dis string) map[string]hiddenCase {
	return map[string]hiddenCase{
		"a hidden advantage":                  {mods: "[" + adv + "]", faces: 2},
		"a hidden advantage and disadvantage": {mods: "[" + adv + "," + dis + "]", faces: 1},
	}
}

// answerOf is what the spend answered, without the dice and the mode the hidden source changes: the refusal or the spend, and what was written.
func (o spendOutcome) answerOf() spendOutcome {
	o.faces, o.mode = 0, ""
	return o
}

// The roll that has a hidden advantage, or a hidden pair that cancels, answers the spend as the roll with no hidden source does: the
// spend goes, and the player is not told by a refusal that something they cannot read is there.
func TestFavorAddsNothingIsJudgedOnWhatTheRollerReadsOnAnAttack(t *testing.T) {
	t.Parallel()
	for _, p := range favorPaths {
		for name, hc := range hiddenCases(attackAdv, attackDis) {
			t.Run(p.name+", "+name, func(t *testing.T) {
				t.Parallel()
				run := func(hidden bool) spendOutcome {
					f := newFavorFight(t)
					p.ready(t, f)
					u, who := p.roller(f)
					f.give(t, who)
					if hidden {
						f.hiddenEffect(t, p.actor, false, hc.mods)
					}
					f.h.roller.queue(4, 17)
					got, err := p.send(t, f, u, newKey(), true, nil)
					out := spendOutcome{faces: len(got.faces), mode: got.mode.String(), favorShown: got.hasFavorSource(), spent: !f.favorHolds(t)}
					if err != nil {
						out.err = err.Error()
					} else {
						out.eventFavor = f.eventFavor(t, p.kind)
					}
					return out
				}
				base, hidden := run(false), run(true)
				if base.err != "" || !base.spent || base.faces != 2 {
					t.Fatalf("the roll with no hidden source = %+v, want the spend to go and the pair of dice", base)
				}
				if hidden.answerOf() != base.answerOf() || hidden.faces != hc.faces {
					t.Errorf("the roll with %s = %+v, want the answer of the roll with none (%+v) and %d d20", name, hidden, base, hc.faces)
				}
			})
		}
		t.Run(p.name+", a visible advantage", func(t *testing.T) {
			t.Parallel()
			f := newFavorFight(t)
			p.ready(t, f)
			u, who := p.roller(f)
			f.give(t, who)
			f.hiddenEffect(t, p.actor, true, "["+attackAdv+"]")
			_, err := p.send(t, f, u, newKey(), true, nil)
			wantFavorBlocked(t, "the spend with an advantage the roller reads", err, playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_ADDS_NOTHING)
			if !f.favorHolds(t) {
				t.Error("the refusal took the Inspiração")
			}
		})
	}
}

// The same on the saves of a combat (a concentration save, the save of an effect): what the master hides from the players adds nothing to
// the judgement of the spend.
func TestFavorAddsNothingIsJudgedOnWhatTheRollerReadsOnASave(t *testing.T) {
	t.Parallel()
	for _, p := range favorSavePaths {
		adv, dis := saveAdvWis, saveDisWis
		if p.actor == "Pensantus" { // the concentration save is a Constitution save
			adv, dis = saveAdvCon, saveDisCon
		}
		for name, hc := range hiddenCases(adv, dis) {
			t.Run(p.name+", "+name, func(t *testing.T) {
				t.Parallel()
				run := func(hidden bool) spendOutcome {
					f, w, u, who := p.setup(t)
					f.give(t, who)
					if hidden {
						f.hiddenEffect(t, p.actor, false, hc.mods)
					}
					f.h.roller.queue(18, 17) // both pass: a failed save of the fighter asks about Indomitable first
					got, err := p.send(t, f, w, u, newKey(), true)
					out := spendOutcome{faces: len(got.faces), spent: !f.favorHolds(t)}
					if err != nil {
						out.err = err.Error()
					} else {
						out.eventFavor = f.eventFavor(t, p.kind)
					}
					return out
				}
				base, hidden := run(false), run(true)
				if base.err != "" || !base.spent || base.faces != 2 {
					t.Fatalf("the save with no hidden source = %+v, want the spend to go and the pair of dice", base)
				}
				if hidden.answerOf() != base.answerOf() || hidden.faces != hc.faces {
					t.Errorf("the save with %s = %+v, want the answer of the save with none (%+v) and %d d20", name, hidden, base, hc.faces)
				}
			})
		}
		t.Run(p.name+", a visible advantage", func(t *testing.T) {
			t.Parallel()
			f, w, u, who := p.setup(t)
			f.give(t, who)
			adv := saveAdvWis
			if p.actor == "Pensantus" {
				adv = saveAdvCon
			}
			f.hiddenEffect(t, p.actor, true, "["+adv+"]")
			_, err := p.send(t, f, w, u, newKey(), true)
			wantFavorBlocked(t, "the spend with an advantage the roller reads", err, playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_ADDS_NOTHING)
			if !f.favorHolds(t) {
				t.Error("the refusal took the Inspiração")
			}
		})
	}
}

// The same on the checks a character rolls for itself, in a scene and in a group check, out of a combat: an effect the master hides.
func TestFavorAddsNothingIsJudgedOnWhatTheRollerReadsOnACheck(t *testing.T) {
	t.Parallel()
	for _, mk := range []func(*testing.T) *checkEnv{newSceneCheckEnv, newGroupCheckEnv} {
		envName := mk(t).name
		for name, hc := range hiddenCases(checkAdvAll, checkDisAll) {
			t.Run(envName+", "+name, func(t *testing.T) {
				t.Parallel()
				run := func(hidden bool) spendOutcome {
					e := mk(t)
					e.give(t, e.who)
					if hidden {
						e.hideOutside(t, e.who.GetId(), hc.mods)
					}
					e.h.roller.queue(4, 17)
					got, err := e.send(t, newKey(), true)
					out := spendOutcome{faces: len(got.faces), mode: got.mode, spent: !e.favorHolds(t)}
					if err != nil {
						out.err = err.Error()
					} else {
						out.stored = e.stored(t)
					}
					return out
				}
				base, hidden := run(false), run(true)
				if base.err != "" || !base.spent || base.faces != 2 || !base.stored {
					t.Fatalf("the check with no hidden source = %+v, want the spend to go and the pair of dice", base)
				}
				if hidden.answerOf() != base.answerOf() || hidden.faces != hc.faces {
					t.Errorf("the check with %s = %+v, want the answer of the check with none (%+v) and %d d20", name, hidden, base, hc.faces)
				}
			})
		}
		t.Run(envName+", a visible advantage", func(t *testing.T) {
			t.Parallel()
			e := mk(t)
			e.give(t, e.who)
			e.advantage(t)
			_, err := e.send(t, newKey(), true)
			wantFavorBlocked(t, "the spend with an advantage the roller reads", err, playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_ADDS_NOTHING)
			if !e.favorHolds(t) {
				t.Error("the refusal took the Inspiração")
			}
		})
	}
}

// hideOutside puts an effect only the master reads on a character out of a combat, with the modifiers (a JSON list).
func (f *favorParty) hideOutside(t *testing.T, characterID, mods string) {
	t.Helper()
	f.execSQL(t, `INSERT INTO character_effects (campaign_id, character_id, group_id, source_key, source_kind, duration_kind, modifiers, player_visible)
		VALUES ($1, $2, gen_random_uuid(), 'spell:enhance-ability', 'spell', 'until_dismissed', $3::jsonb, false)`, f.campaignID, characterID, mods)
}

// --- B5: the Inspiração is spent when the roll is made; a Protection that comes later takes the advantage away and the Inspiração stays spent ---

// The spend is valid when made. A Protection (the style of a shield-bearer next to the target) that is answered afterwards gives the replayed
// attack its disadvantage, which cancels the advantage as it cancels any advantage: the roll is straight, and the Inspiração is spent all the same.
func TestFavorSpentBeforeAProtectionIsStillSpentWhenItCancelsTheAdvantage(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	guard, err := a.master.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: a.campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_BOSS, Name: "Guardião", Sheet: protector().sheet(),
	}))
	if err != nil {
		t.Fatalf("CreateCharacter(Guardião) error = %v", err)
	}
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}, {CharacterId: guard.Msg.GetCharacter().GetId()}},
		npcRolls: []int{3, 2},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1},
		reveal:   []string{"Goblin", "Guardião"},
		at:       map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Guardião": {5, 4}, "Pensantus": {10, 3}, "Brisa": {4, 9}},
	})
	f := favorFightOf(t, a, e)
	f.give(t, a.toren)
	held, err := favorPaths[0].send(t, f, a.caio, newKey(), true, nil)
	if err != nil || !held.heldByWindow {
		t.Fatalf("the spend on an attack the Guardião may protect against = %+v, %v, want it held for the window", held, err)
	}
	if !f.favorHolds(t) {
		t.Fatal("the Inspiração was spent while the attack waits for the Protection")
	}
	w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_PROTECTION)
	if w == nil {
		t.Fatalf("the master has no Protection window: %v", a.windows(t, a.master))
	}
	a.h.roller.queue(15)
	a.mustAnswer(t, a.master, f.e, w.GetId(), useAnswer(0))
	if f.favorHolds(t) || f.eventFavor(t, eventAttackRolled) != "true" {
		t.Errorf("after the Protection the Inspiração is there = %v and the roll says %q, want it spent: the spend was valid when made", f.favorHolds(t), f.eventFavor(t, eventAttackRolled))
	}
	entry := a.lastAttackEntry(t, f.e)
	if got := len(entry.GetAttackRoll().GetFaces()); got != 1 {
		t.Errorf("the attack rolled %d d20, want the one of a straight roll: the Protection's disadvantage cancels the Inspiração's advantage", got)
	}
	if line := f.lineOf(t, f.master, playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK); line.GetFavorTextPt() != "Toren usou a Inspiração." {
		t.Errorf("the line says %q, want that Toren used the Inspiração", line.GetFavorTextPt())
	}
}
