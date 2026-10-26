package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// wantActedByMaster checks the log lines of the kinds whose actor is the label: the master, the
// character's player and a bystander all read at least one, every one marked by_master, and the
// bystander gets none of the master's fields (RN-10: the undo marker, the hidden mark, the armor
// class, the hit points after).
func wantActedByMaster(t *testing.T, a *armed, e *playv1.Encounter, label string, owner, bystander *user, kinds ...playv1.CombatLogKind) {
	t.Helper()
	for who, u := range map[string]*user{"the master": a.master, "the character's player": owner, "a bystander": bystander} {
		var lines []*playv1.CombatLogEntry
		for _, k := range kinds {
			for _, l := range logLinesOf(t, a, u, e, k) {
				if l.GetActorLabel() == label {
					lines = append(lines, l)
				}
			}
		}
		if len(lines) == 0 {
			t.Errorf("%s reads no line of %v by %s", who, kinds, label)
		}
		for _, l := range lines {
			if !l.GetByMaster() {
				t.Errorf("%s reads %v by %s without by_master", who, l.GetKind(), label)
			}
			if u == bystander && (l.GetUndoable() || l.GetHidden() || l.TargetArmorClass != nil || l.HitPointsAfter != nil) {
				t.Errorf("the bystander reads master-only fields of %v: %v", l.GetKind(), l)
			}
		}
	}
}

// TestTheMasterActsForAPlayersCharacterOnEveryPath: each act the screen of "Agir por" makes (a
// spell attack with a held die, a feature, a move, Wild Shape, Bardic Inspiration, a summon, the
// familiar's eyes) works for the master on the character's turn, marks the log line by_master,
// and stays refused to a player who is not the character's.
func TestTheMasterActsForAPlayersCharacterOnEveryPath(t *testing.T) {
	t.Parallel()
	paths := map[string]func(t *testing.T){
		"a spell attack with the player's die held": func(t *testing.T) {
			t.Helper()
			a := newCasters(t)
			e := a.start(t, plan{
				npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
				npcRolls: []int{3},
				players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1},
				reveal:   []string{"Goblin"},
				at:       map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Pensantus": {10, 3}, "Brisa": {4, 9}},
			})
			a.passTo(t, e, "Brisa")
			e = a.get(t, a.master)
			a.giveCombatantDie(t, "Brisa", e)
			typed := func(r *playv1.CastSpellRequest) { r.Roll = &playv1.CastSpellRequest_D20Face{D20Face: 4} }
			_, err := a.cast(t, a.ana, e, "Brisa", guidingBolt, slotOfLevel(1), a.at(t, "Goblin"), typed)
			wantCode(t, "another player casting for Brisa", err, connect.CodePermissionDenied)
			res, err := a.cast(t, a.master, e, "Brisa", guidingBolt, slotOfLevel(1), a.at(t, "Goblin"), typed)
			if err != nil || res.GetHeldRollOffer() == nil {
				t.Fatalf("CastSpell(master) = %v, %v; want the die's question", res, err)
			}
			if _, err := a.answerOutside(t, a.bia, res.GetHeldRollOffer().GetHoldId(), true, false, 8); err != nil { // the player answers first
				t.Fatalf("the player's answer: %v", err)
			}
			wantActedByMaster(t, a, e, "Brisa", a.bia, a.ana, playv1.CombatLogKind_COMBAT_LOG_KIND_SPELL_CAST, playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK)
		},
		"Second Wind": func(t *testing.T) {
			t.Helper()
			a := newCasters(t)
			e := a.castersFight(t, 1)
			a.correct(t, a.toren, hpIs(20))
			a.mustEndTurn(t, a.ana, e) // Toren's turn
			_, err := a.feature(t, a.ana, e, "Toren", secondWindKey, takeInApp)
			wantCode(t, "another player using Toren's feature", err, connect.CodePermissionDenied)
			a.h.roller.queue(6)
			if _, err := a.feature(t, a.master, e, "Toren", secondWindKey, takeInApp); err != nil {
				t.Fatalf("TakeAction(second wind) by the master: %v", err)
			}
			wantActedByMaster(t, a, e, "Toren", a.caio, a.ana, playv1.CombatLogKind_COMBAT_LOG_KIND_ACTION)
		},
		"a move": func(t *testing.T) {
			t.Helper()
			a := newArmed(t)
			e := a.threeAndAGoblin(t)
			_, err := a.frozenMove(t, a.ana, "Toren", 2, 3)
			wantCode(t, "another player moving Toren", err, connect.CodePermissionDenied)
			if _, err := a.frozenMove(t, a.master, "Toren", 2, 3); err != nil {
				t.Fatalf("MoveCombatant by the master: %v", err)
			}
			wantActedByMaster(t, a, e, "Toren", a.caio, a.ana, playv1.CombatLogKind_COMBAT_LOG_KIND_MOVED)
		},
		"Wild Shape in and out": func(t *testing.T) {
			t.Helper()
			s := newShapers(t)
			e := s.fight(t)
			_, err := s.assume(t, s.caio, s.bri, wolfKey)
			wantCode(t, "another player assuming Sálvia's form", err, connect.CodePermissionDenied)
			s.mustAssume(t, s.master, s.bri, wolfKey)
			s.mustLeave(t, s.master, s.bri)
			wantActedByMaster(t, s.armed, e, "Sálvia", s.bia, s.ana, playv1.CombatLogKind_COMBAT_LOG_KIND_WILD_SHAPE)
		},
		"Bardic Inspiration": func(t *testing.T) {
			t.Helper()
			a := newResourceTable(t)
			e := a.resourceFight(t)
			a.passTo(t, e, "Orla")
			e = a.get(t, a.master)
			_, err := a.giveInspiration(t, a.ana, e, "Orla", "Tavo")
			wantCode(t, "another player giving Orla's inspiration", err, connect.CodePermissionDenied)
			if _, err := a.giveInspiration(t, a.master, e, "Orla", "Tavo"); err != nil {
				t.Fatalf("GiveBardicInspiration by the master: %v", err)
			}
			wantActedByMaster(t, a, e, "Orla", a.bia, a.ana, playv1.CombatLogKind_COMBAT_LOG_KIND_RESOURCE)
			for _, l := range logLinesOf(t, a, a.ana, e, playv1.CombatLogKind_COMBAT_LOG_KIND_RESOURCE) {
				if l.GetTargetLabel() != "" || l.GetResource().GetDieSides() != 0 {
					t.Errorf("a bystander reads the target %q and the die %d of the master's giving", l.GetTargetLabel(), l.GetResource().GetDieSides())
				}
			}
		},
		"a summon cast": func(t *testing.T) {
			t.Helper()
			a := newSummoners(t)
			e := a.summonersFight(t)
			a.toSalvia(t, e)
			_, err := a.summonWolves(t, a.caio, e, 12)
			wantCode(t, "another player summoning for Sálvia", err, connect.CodePermissionDenied)
			if _, err := a.summonWolves(t, a.master, e, 12); err != nil {
				t.Fatalf("the master's Conjurar Animais for Sálvia: %v", err)
			}
			wantActedByMaster(t, a, e, "Sálvia", a.bia, a.caio, playv1.CombatLogKind_COMBAT_LOG_KIND_SPELL_CAST)
		},
		"the familiar's eyes": func(t *testing.T) {
			t.Helper()
			// The familiar's sight leaves no log line: it is the character's own state, read in its vitals.
			s := newShapers(t)
			a := s.armed
			owl := s.nanquim(t)
			x, y := centerBP(0, 0)
			a.h.placeToken(a.mapID, s.pens.GetId(), x, y)
			a.placeCreatureToken(t, owl, 12, 9)
			_, _, err := a.sightCall(t, s.caio, s.pens, true)
			wantCode(t, "another player starting Pensantus's familiar sight", err, connect.CodePermissionDenied)
			v, _ := a.mustSight(t, s.master, s.pens, true)
			if v.GetFamiliarSight().GetCreatureId() != owl {
				t.Errorf("the master's familiar sight = %v, want the owl", v.GetFamiliarSight())
			}
			a.mustSight(t, s.master, s.pens, false)
		},
	}
	for name, run := range paths {
		t.Run(name, func(t *testing.T) {
			t.Helper()
			t.Parallel()
			run(t)
		})
	}
}
