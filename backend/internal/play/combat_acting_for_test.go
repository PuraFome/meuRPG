package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// logLinesOf are the log lines of the kind the caller reads.
func logLinesOf(t *testing.T, a *armed, u *user, e *playv1.Encounter, kind playv1.CombatLogKind) []*playv1.CombatLogEntry {
	t.Helper()
	var out []*playv1.CombatLogEntry
	for _, r := range a.log(t, u, e).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == kind {
				out = append(out, en)
			}
		}
	}
	return out
}

// TestWhatTheMasterDoesForACharacterIsMarkedByMaster: an attack the master rolls for Toren is
// marked in the log for the master, Toren's player and the others; the same attack by its own
// player, and by the master as the Goblin, is not.
func TestWhatTheMasterDoesForACharacterIsMarkedByMaster(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.threeAndAGoblin(t)
	a.h.roller.queue(15)
	hit := a.mustAttack(t, a.master, e, "Toren", battleaxe, "Goblin", inAppRoll) // the master acts for Toren
	a.h.roller.queue(3)
	a.mustDamage(t, a.master, e, hit.GetPendingDamage().GetId(), inAppDamage)
	a.mustEndTurn(t, a.master, e)
	a.mustAttack(t, a.ana, e, "Pensantus", fireBolt, "Goblin", d20(19)) // his own player
	for who, u := range map[string]*user{"the master": a.master, "Toren's player": a.caio, "Brisa's player": a.bia} {
		lines := logLinesOf(t, a, u, e, playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK)
		if len(lines) != 2 {
			t.Fatalf("%s reads %d attack lines, want 2", who, len(lines))
		}
		for _, l := range lines {
			if want := l.GetActorLabel() == "Toren"; l.GetByMaster() != want {
				t.Errorf("%s reads the attack of %s with by_master %v, want %v", who, l.GetActorLabel(), l.GetByMaster(), want)
			}
		}
	}
}

// TestTheMastersDeathSaveForACharacterIsMarkedByMaster: the save the master rolls for a downed
// character, with a typed d20, is marked; the player's own is not.
func TestTheMastersDeathSaveForACharacterIsMarkedByMaster(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.castersFight(t, 1)
	a.correct(t, a.toren, hpIs(0))
	a.mustEndTurn(t, a.ana, e) // Toren's turn
	if _, err := a.deathSave(t, a.master, e, "Toren", deathFace(12)); err != nil {
		t.Fatalf("the master's death save: %v", err)
	}
	for who, u := range map[string]*user{"the master": a.master, "Toren's player": a.caio} {
		lines := logLinesOf(t, a, u, e, playv1.CombatLogKind_COMBAT_LOG_KIND_DEATH_SAVE)
		if len(lines) != 1 || !lines[0].GetByMaster() {
			t.Errorf("%s reads the death save %v, want one marked by_master", who, lines)
		}
	}
}

// TestTheMasterActingForACharacterSeesOnlyWhatTheCharacterSees: a creature the master hid from
// the players is not a target for Toren's attack when the master rolls it for him, is listed
// in Toren's turn options as not seen, and is still the master's own target as himself.
func TestTheMasterActingForACharacterSeesOnlyWhatTheCharacterSees(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.threeAndAGoblin(t)
	if _, err := a.master.combat.SetCombatantHidden(t.Context(), connect.NewRequest(&playv1.SetCombatantHiddenRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Goblin"), IdempotencyKey: newKey(), Hidden: true,
	})); err != nil {
		t.Fatalf("SetCombatantHidden() error = %v", err)
	}
	opts := a.mustOptions(t, a.master, e, "Toren")
	seen := false
	for _, at := range opts.GetAttackTargets() {
		for _, tg := range at.GetTargets() {
			if tg.GetLabel() == "Goblin" {
				seen = true
				if !tg.GetNotSeen() || !tg.GetUntargetable() {
					t.Errorf("the hidden Goblin in the master's options for Toren = %v, want not_seen and untargetable", tg)
				}
			} else if tg.GetNotSeen() {
				t.Errorf("%s is marked not seen", tg.GetLabel())
			}
		}
	}
	if !seen {
		t.Error("the master's options for Toren do not list the hidden Goblin (dimmed)")
	}
	for _, at := range a.mustOptions(t, a.master, e, "Goblin").GetAttackTargets() {
		for _, tg := range at.GetTargets() {
			if tg.GetNotSeen() {
				t.Errorf("the Goblin's own targets carry not_seen: %v", tg)
			}
		}
	}
	before := len(a.eventTrace(t, e, 0))
	_, err := a.attack(t, a.master, e, "Toren", battleaxe, "Goblin", inAppRoll)
	wantCode(t, "the master attacking a hidden creature for Toren", err, connect.CodeNotFound)
	if n := len(a.eventTrace(t, e, 0)); n != before {
		t.Errorf("events after the refusal = %d, want %d", n, before)
	}
	// The Goblin's own turn keeps the master's full view.
	if got := a.mustOptions(t, a.master, e, "Goblin"); len(got.GetAttackTargets()) == 0 {
		t.Error("the Goblin's options for the master list no attack targets")
	}
}

// TestTheMasterFillsTheMissingInitiativesBeforeTheCombatBegins: the combat waits for every
// initiative and names who is missing; the master rolls for the gaps (a typed d20 for one, the
// app's dice for another), and then the combat begins.
func TestTheMasterFillsTheMissingInitiativesBeforeTheCombatBegins(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Toren": 18},
		setup:    true,
	})
	begin := func() error {
		_, err := a.master.combat.BeginCombat(t.Context(), connect.NewRequest(&playv1.BeginCombatRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()}))
		return err
	}
	err := begin()
	b := wantBlockedBy(t, "BeginCombat with two initiatives missing", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_INITIATIVE_MISSING)
	if len(b.GetCombatantIds()) != 2 {
		t.Fatalf("missing = %v, want Pensantus and Brisa", b.GetCombatantIds())
	}
	if _, err := a.submitInitiative(t, a.master, e, "Pensantus", 12); err != nil {
		t.Fatalf("the master's typed initiative for Pensantus: %v", err)
	}
	b = wantBlockedBy(t, "BeginCombat with one initiative missing", begin(), playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_INITIATIVE_MISSING)
	if len(b.GetCombatantIds()) != 1 || b.GetCombatantIds()[0] != a.id(t, "Brisa") {
		t.Fatalf("missing = %v, want Brisa", b.GetCombatantIds())
	}
	a.h.roller.queue(9)
	if _, err := a.master.combat.SubmitInitiative(t.Context(), connect.NewRequest(&playv1.SubmitInitiativeRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Brisa"), IdempotencyKey: newKey(),
		Roll: &playv1.SubmitInitiativeRequest_RollInApp{RollInApp: true},
	})); err != nil {
		t.Fatalf("the master's app roll of initiative for Brisa: %v", err)
	}
	if err := begin(); err != nil {
		t.Fatalf("BeginCombat with every initiative: %v", err)
	}
}

// TestTheMastersCastForACharacterSpendsItsSlotAndIsMarkedByMaster: the master casts a leveled
// spell as Pensantus, on Pensantus's turn: the character's slot is spent and the line reads as
// the master's act.
func TestTheMastersCastForACharacterSpendsItsSlotAndIsMarkedByMaster(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.castersFight(t, 1)
	a.passTo(t, e, "Pensantus")
	a.mustCast(t, a.master, e, "Pensantus", magicMissileSpell, slotOfLevel(1), []*playv1.SpellTarget{darts(a, t, "Capitão Goblin", 3)}, noCastRoll)
	for who, u := range map[string]*user{"the master": a.master, "Pensantus's player": a.ana} {
		lines := logLinesOf(t, a, u, e, playv1.CombatLogKind_COMBAT_LOG_KIND_SPELL_CAST)
		if len(lines) == 0 {
			lines = logLinesOf(t, a, u, e, playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK)
		}
		if len(lines) != 1 || !lines[0].GetByMaster() || lines[0].GetActorLabel() != "Pensantus" {
			t.Errorf("%s reads %v, want one line by Pensantus marked by_master", who, lines)
		}
	}
}
