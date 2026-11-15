package leaktest

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The rider's falls (RN-21): the save a push or a prone asks of a rider and the dismount a mount that is knocked prone or killed asks.
// Each runs the real trigger (a shove the master resolves for Ana's Pensantus, the master's damage to the mount) with the mount hidden
// and with the rider hidden, and with both on the table as the control. What the call, the encounter, the wait, the log and the stream
// say never reaches a player who does not see both creatures, and the master and the rider's owner read what is theirs.

// pairWorld is a table with a rider on a Large mount at (12, 8) and Ana's Pensantus next to it at (13, 8). hide says who the master
// hides: "mount", "rider" or "" (both shown).
func pairWorld(t *testing.T, hide string) (w *world, rider, mount *playv1.Combatant) {
	t.Helper()
	w = newWorld(t)
	ctx := t.Context()
	e := w.encounter
	switch hide {
	case "rider": // an NPC rides the visible mount
		mount, rider = w.combatant(w.seenNPC), w.combatant(w.hiddenNPC)
	case "mount": // Caio's character rides the hidden mount
		mount, rider = w.combatant(w.hiddenNPC), w.combatant(w.toren)
	default:
		mount, rider = w.combatant(w.seenNPC), w.combatant(w.toren)
		must(w.master.combat.SetCombatantHidden(ctx, rq(&playv1.SetCombatantHiddenRequest{CampaignId: w.campaign, EncounterId: e.GetId(), CombatantId: w.combatant(w.hiddenNPC).GetId(), IdempotencyKey: newKey(), Hidden: true})))
	}
	if _, err := w.pool.Exec(ctx, `UPDATE combatants SET size = 'large', side = 'party' WHERE id = $1`, mount.GetId()); err != nil {
		t.Fatalf("make the mount Large and of the party: %v", err)
	}
	if hide == "rider" && !rider.GetHidden() {
		t.Fatalf("the fixture's hidden NPC is not hidden")
	}
	pens := w.combatant(w.pens)
	w.placeForced(w.master, e.GetId(), placement{mount.GetId(), 12, 8}, placement{rider.GetId(), 12, 8}, placement{pens.GetId(), 13, 8})
	if _, err := w.pool.Exec(ctx, `UPDATE combatants SET mounted_on_id = $1 WHERE id = $2`, mount.GetId(), rider.GetId()); err != nil {
		t.Fatalf("put the rider on the mount: %v", err)
	}
	must(w.master.combat.SetCombatantMount(ctx, rq(&playv1.SetCombatantMountRequest{CampaignId: w.campaign, EncounterId: e.GetId(), CombatantId: mount.GetId(), IdempotencyKey: newKey(), Kind: "controlled"})))
	must(w.master.play.AdjustCharacterVitals(ctx, rq(&playv1.AdjustCharacterVitalsRequest{
		CampaignId: w.campaign, CharacterId: w.pens.GetId(), IdempotencyKey: newKey(), ExpectedRevision: w.master.revisionOf(w, w.pens.GetId()), HitPointsCurrent: proto.Int32(5),
	}))) // the fixture left her down
	return w, rider, mount
}

// copies is what each person reads of the encounter and the log right now.
func (w *world) copies(e string) map[string]string {
	out := map[string]string{}
	for _, p := range []*person{w.master, w.ana, w.caio} {
		out[p.name] = jsonOf(must(p.combat.ListCombatLog(w.t.Context(), rq(&playv1.ListCombatLogRequest{CampaignId: w.campaign, EncounterId: e})))) +
			jsonOf(must(p.combat.GetEncounter(w.t.Context(), rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))))
	}
	return out
}

func (w *world) encounterOf(p *person) *playv1.Encounter {
	return must(p.combat.GetEncounter(w.t.Context(), rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter()
}

// shovePushes has the master shove the mount for Pensantus: she wins the contest and the shove pushes it.
func (w *world) shovePushes(mount *playv1.Combatant) {
	w.t.Helper()
	ctx := w.t.Context()
	e := w.encounter.GetId()
	w.toTurnOf(w.pens.GetId())
	w.dice.ScriptFor(w.t, 20, 19, 2)
	started := must(w.master.contests.StartContest(ctx, rq(&playv1.StartContestRequest{
		CampaignId: w.campaign, EncounterId: e, IdempotencyKey: newKey(), Roll: appRoll(),
		InitiatorId: w.combatant(w.pens).GetId(), TargetId: mount.GetId(), Purpose: playv1.ContestPurpose_CONTEST_PURPOSE_SHOVE,
	}))).GetContest()
	view := must(w.master.contests.RespondContest(ctx, rq(&playv1.RespondContestRequest{
		CampaignId: w.campaign, EncounterId: e, IdempotencyKey: newKey(), ContestId: started.GetId(), Skill: playv1.ContestSkill_CONTEST_SKILL_ATHLETICS, Roll: appRoll(),
	}))).GetContest()
	must(w.master.contests.ResolveShove(ctx, rq(&playv1.ResolveShoveRequest{
		CampaignId: w.campaign, EncounterId: e, IdempotencyKey: newKey(), ContestId: view.GetId(), Outcome: playv1.ShoveOutcome_SHOVE_OUTCOME_PUSH,
	})))
}

// noTrace fails when a player reads the creature the master hides, in the copy or the stream, and holds the master's copy to it.
func noTrace(t *testing.T, w *world, when string, hidden *playv1.Combatant, copies map[string]string, heard map[string][]string, players ...*person) {
	t.Helper()
	if !strings.Contains(copies[w.master.name], hidden.GetLabel()) {
		t.Fatalf("the positive control failed: the master's copy %s has no %s", when, hidden.GetLabel())
	}
	for _, p := range players {
		body := copies[p.name] + strings.Join(heard[p.name], "\n")
		for _, needle := range []string{hidden.GetId(), hidden.GetLabel()} {
			if strings.Contains(body, needle) {
				t.Errorf("%s reads %q %s", p.name, needle, when)
			}
		}
	}
}

func TestARidersSaveNeverNamesAHiddenCreature(t *testing.T) {
	for _, tt := range []struct {
		hide   string
		owner  bool // Caio's character is the rider: he reads the save
		hidden func(rider, mount *playv1.Combatant) *playv1.Combatant
		other  bool // Ana's copy of the log reads the fall (both creatures shown)
	}{
		{"mount", true, func(_, m *playv1.Combatant) *playv1.Combatant { return m }, false},
		{"rider", false, func(r, _ *playv1.Combatant) *playv1.Combatant { return r }, false},
		{"", true, nil, true},
	} {
		t.Run("hidden "+tt.hide, func(t *testing.T) {
			w, rider, mount := pairWorld(t, tt.hide)
			reads, falls := map[string]string{}, map[string]string{}
			var hidden *playv1.Combatant
			if tt.hidden != nil {
				hidden = tt.hidden(rider, mount)
			}
			var asked map[string]*playv1.MountSavePrompt
			got := w.hiddenOnlyEvents(func() {
				w.shovePushes(mount)
				reads = w.copies(w.encounter.GetId())
				asked = map[string]*playv1.MountSavePrompt{}
				for _, p := range []*person{w.master, w.ana, w.caio} {
					asked[p.name] = w.encounterOf(p).GetMountSave()
				}
				roller := w.master
				if tt.owner {
					roller = w.caio
				}
				must(roller.combat.RollMountSave(w.t.Context(), rq(&playv1.RollMountSaveRequest{
					CampaignId: w.campaign, EncounterId: w.encounter.GetId(), RiderId: rider.GetId(), IdempotencyKey: newKey(), Roll: &playv1.RollMountSaveRequest_D20Face{D20Face: 1},
				})))
				falls = w.copies(w.encounter.GetId())
			})
			if len(got[w.master.name]) == 0 {
				t.Errorf("the positive control failed: the master's stream carried nothing of the push and the fall")
			}
			// The controls: the master reads the save and the fall, and the owner reads his own rider's.
			if asked[w.master.name].GetRiderId() != rider.GetId() {
				t.Fatalf("the positive control failed: the master reads the save %v, want the rider's", asked[w.master.name])
			}
			if tt.owner && asked[w.caio.name].GetRiderId() != rider.GetId() {
				t.Errorf("the positive control failed: Caio reads the save %v, want his rider's", asked[w.caio.name])
			}
			if !strings.Contains(falls[w.master.name], "COMBAT_LOG_KIND_RIDER_THROWN") {
				t.Fatalf("the positive control failed: the master's log has no line of the fall")
			}
			if tt.hide == "" {
				for _, p := range []*person{w.ana, w.caio} {
					if !strings.Contains(falls[p.name], "COMBAT_LOG_KIND_RIDER_THROWN") {
						t.Errorf("the positive control failed: %s, who sees both creatures, reads no line of the fall", p.name)
					}
				}
				return
			}
			players := []*person{w.ana, w.caio}
			if tt.hide == "mount" { // Caio's rider is shown: he reads his own, and nothing of the mount
				if asked[w.caio.name].GetMountId() != "" {
					t.Errorf("Caio reads the hidden mount in %v", asked[w.caio.name])
				}
			} else if asked[w.caio.name] != nil {
				t.Errorf("Caio reads the save %v of a rider he does not see", asked[w.caio.name])
			}
			if asked[w.ana.name] != nil {
				t.Errorf("Ana reads the save %v", asked[w.ana.name])
			}
			noTrace(t, w, "while the save is asked", hidden, reads, got, players...)
			noTrace(t, w, "after the fall", hidden, falls, nil, players...)
			for _, p := range players {
				for _, kind := range []string{"COMBAT_LOG_KIND_MOUNT_SAVE", "COMBAT_LOG_KIND_RIDER_THROWN"} {
					if strings.Contains(falls[p.name], kind) {
						t.Errorf("%s reads a %s line of a pair they do not see", p.name, kind)
					}
				}
			}
		})
	}
}

func TestAMountsDismountNeverNamesAHiddenCreature(t *testing.T) {
	for _, tt := range []struct {
		hide   string
		owner  bool
		hidden func(rider, mount *playv1.Combatant) *playv1.Combatant
	}{
		{"mount", true, func(_, m *playv1.Combatant) *playv1.Combatant { return m }},
		{"rider", false, func(r, _ *playv1.Combatant) *playv1.Combatant { return r }},
		{"", true, nil},
	} {
		t.Run("hidden "+tt.hide, func(t *testing.T) {
			w, rider, mount := pairWorld(t, tt.hide)
			var asked, landed map[string]string
			var windows map[string]*playv1.ReactionWindow
			got := w.hiddenOnlyEvents(func() {
				must(w.master.combat.AdjustCombatantHitPoints(w.t.Context(), rq(&playv1.AdjustCombatantHitPointsRequest{
					CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: mount.GetId(), IdempotencyKey: newKey(),
					Change: &playv1.AdjustCombatantHitPointsRequest_Damage{Damage: 9999},
				})))
				asked = w.copies(w.encounter.GetId())
				windows = map[string]*playv1.ReactionWindow{}
				for _, p := range []*person{w.master, w.ana, w.caio} {
					for _, rw := range w.encounterOf(p).GetReactionWindows() {
						if rw.GetKind() == playv1.ReactionKind_REACTION_KIND_DISMOUNT {
							windows[p.name] = rw
						}
					}
				}
				answerer := w.master
				if windows[w.caio.name] != nil {
					answerer = w.caio
				}
				must(answerer.combat.AnswerReaction(w.t.Context(), rq(&playv1.AnswerReactionRequest{
					CampaignId: w.campaign, EncounterId: w.encounter.GetId(), WindowId: windows[w.master.name].GetId(), IdempotencyKey: newKey(), Answer: playv1.ReactionChoice_REACTION_CHOICE_PASS,
				})))
				landed = w.copies(w.encounter.GetId())
			})
			if len(got[w.master.name]) == 0 {
				t.Errorf("the positive control failed: the master's stream carried nothing of the dismount")
			}
			if windows[w.master.name] == nil {
				t.Fatalf("the positive control failed: the master reads no dismount window")
			}
			if tt.owner && windows[w.caio.name] == nil {
				t.Errorf("the positive control failed: Caio reads no dismount window of his own rider")
			}
			if !strings.Contains(landed[w.master.name], "COMBAT_LOG_KIND_RIDER_THROWN") {
				t.Fatalf("the positive control failed: the master's log has no line of the fall")
			}
			if tt.hide == "" {
				for _, p := range []*person{w.ana, w.caio} {
					if !strings.Contains(landed[p.name], "COMBAT_LOG_KIND_RIDER_THROWN") {
						t.Errorf("the positive control failed: %s reads no line of the fall", p.name)
					}
				}
				return
			}
			hidden := tt.hidden(rider, mount)
			if windows[w.ana.name] != nil {
				t.Errorf("Ana reads the dismount window %v", windows[w.ana.name])
			}
			if tt.hide == "rider" && windows[w.caio.name] != nil {
				t.Errorf("Caio reads the dismount window %v of a rider he does not see", windows[w.caio.name])
			}
			for name, rw := range windows {
				if name != w.master.name && (strings.Contains(jsonOf(rw), hidden.GetId()) || strings.Contains(jsonOf(rw), hidden.GetLabel())) {
					t.Errorf("%s reads %s in the window", name, hidden.GetLabel())
				}
			}
			noTrace(t, w, "while the dismount is asked", hidden, asked, got, w.ana, w.caio)
			noTrace(t, w, "after the fall", hidden, landed, nil, w.ana, w.caio)
			for _, p := range []*person{w.ana, w.caio} {
				if strings.Contains(landed[p.name], "COMBAT_LOG_KIND_RIDER_THROWN") {
					t.Errorf("%s reads a line of the fall of a pair they do not see", p.name)
				}
			}
		})
	}
}
