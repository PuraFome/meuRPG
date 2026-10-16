package play

import (
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The characterization of the handlers of the combat core (combat.go, combat_standup.go,
// combat_death.go, combat_conditions.go, combat_monsters.go, combat_surprise.go and
// combat_reveal.go): the answers a request gets before any transaction opens, in the order
// the checks run, with the exact code and message of each. The tests that already exist
// follow the rules; these freeze the branches a refactor of the handlers could move.

// wantCoreRefusal checks the code and the exact message of an error.
func wantCoreRefusal(t *testing.T, call string, err error, code connect.Code, msg string) {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("%s: error = %v, want a connect error %v %q", call, err, code, msg)
	}
	if ce.Code() != code || ce.Message() != msg {
		t.Fatalf("%s: error = %v %q, want %v %q", call, ce.Code(), ce.Message(), code, msg)
	}
}

// coreIDs is what a request names: each field can be spoiled by a case.
type coreIDs struct {
	key, enc, comb string
}

// coreCall sends one request of a handler as u with the ids given.
type coreCall struct {
	name   string
	master bool // only the master may call it
	send   func(f *fight, u *user, ids coreIDs) error
}

func coreCalls() []coreCall {
	return []coreCall{
		{"SubmitInitiative", false, func(f *fight, u *user, ids coreIDs) error {
			_, err := u.combat.SubmitInitiative(f.h.t.Context(), connect.NewRequest(&playv1.SubmitInitiativeRequest{
				CampaignId: f.campaignID, EncounterId: ids.enc, CombatantId: ids.comb, IdempotencyKey: ids.key,
				Roll: &playv1.SubmitInitiativeRequest_RollInApp{RollInApp: true},
			}))
			return err
		}},
		{"SetInitiativeOrder", true, func(f *fight, u *user, ids coreIDs) error {
			_, err := u.combat.SetInitiativeOrder(f.h.t.Context(), connect.NewRequest(&playv1.SetInitiativeOrderRequest{
				CampaignId: f.campaignID, EncounterId: ids.enc, IdempotencyKey: ids.key, CombatantIds: []string{newKey(), newKey()},
			}))
			return err
		}},
		{"BeginCombat", true, func(f *fight, u *user, ids coreIDs) error {
			_, err := u.combat.BeginCombat(f.h.t.Context(), connect.NewRequest(&playv1.BeginCombatRequest{
				CampaignId: f.campaignID, EncounterId: ids.enc, IdempotencyKey: ids.key,
			}))
			return err
		}},
		{"SetCombatantHidden", true, func(f *fight, u *user, ids coreIDs) error {
			_, err := u.combat.SetCombatantHidden(f.h.t.Context(), connect.NewRequest(&playv1.SetCombatantHiddenRequest{
				CampaignId: f.campaignID, EncounterId: ids.enc, CombatantId: ids.comb, IdempotencyKey: ids.key, Hidden: true,
			}))
			return err
		}},
		{"AddCombatants", true, func(f *fight, u *user, ids coreIDs) error {
			_, err := u.combat.AddCombatants(f.h.t.Context(), connect.NewRequest(&playv1.AddCombatantsRequest{
				CampaignId: f.campaignID, EncounterId: ids.enc, IdempotencyKey: ids.key,
				Participants: []*playv1.Participant{{CharacterId: f.goblin.GetId()}},
			}))
			return err
		}},
		{"RemoveCombatant", true, func(f *fight, u *user, ids coreIDs) error {
			_, err := u.combat.RemoveCombatant(f.h.t.Context(), connect.NewRequest(&playv1.RemoveCombatantRequest{
				CampaignId: f.campaignID, EncounterId: ids.enc, CombatantId: ids.comb, IdempotencyKey: ids.key,
			}))
			return err
		}},
		{"EndEncounter", true, func(f *fight, u *user, ids coreIDs) error {
			_, err := u.combat.EndEncounter(f.h.t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{
				CampaignId: f.campaignID, EncounterId: ids.enc, IdempotencyKey: ids.key,
			}))
			return err
		}},
		{"SetCombatantConditions", false, func(f *fight, u *user, ids coreIDs) error {
			_, err := u.combat.SetCombatantConditions(f.h.t.Context(), connect.NewRequest(&playv1.SetCombatantConditionsRequest{
				CampaignId: f.campaignID, EncounterId: ids.enc, CombatantId: ids.comb, IdempotencyKey: ids.key, EndConcentration: true,
			}))
			return err
		}},
		{"StandUp", false, func(f *fight, u *user, ids coreIDs) error {
			_, err := u.combat.StandUp(f.h.t.Context(), connect.NewRequest(&playv1.StandUpRequest{
				CampaignId: f.campaignID, EncounterId: ids.enc, CombatantId: ids.comb, IdempotencyKey: ids.key,
			}))
			return err
		}},
		{"RollDeathSave", false, func(f *fight, u *user, ids coreIDs) error {
			_, err := u.combat.RollDeathSave(f.h.t.Context(), connect.NewRequest(&playv1.RollDeathSaveRequest{
				CampaignId: f.campaignID, EncounterId: ids.enc, CombatantId: ids.comb, IdempotencyKey: ids.key,
				Roll: &playv1.RollDeathSaveRequest_RollInApp{RollInApp: true},
			}))
			return err
		}},
		{"ConfirmDeath", true, func(f *fight, u *user, ids coreIDs) error {
			_, err := u.combat.ConfirmDeath(f.h.t.Context(), connect.NewRequest(&playv1.ConfirmDeathRequest{
				CampaignId: f.campaignID, EncounterId: ids.enc, CombatantId: ids.comb, IdempotencyKey: ids.key,
			}))
			return err
		}},
		{"AddMonsters", true, func(f *fight, u *user, ids coreIDs) error {
			_, err := u.combat.AddMonsters(f.h.t.Context(), connect.NewRequest(&playv1.AddMonstersRequest{
				CampaignId: f.campaignID, EncounterId: ids.enc, IdempotencyKey: ids.key, CreatureKey: bandit,
			}))
			return err
		}},
		{"SetSurprised", true, func(f *fight, u *user, ids coreIDs) error {
			_, err := u.contests.SetSurprised(f.h.t.Context(), connect.NewRequest(&playv1.SetSurprisedRequest{
				CampaignId: f.campaignID, EncounterId: ids.enc, CombatantId: ids.comb, IdempotencyKey: ids.key, Surprised: true,
			}))
			return err
		}},
		{"ResolveHiddenReveal", true, func(f *fight, u *user, ids coreIDs) error {
			_, err := u.combat.ResolveHiddenReveal(f.h.t.Context(), connect.NewRequest(&playv1.ResolveHiddenRevealRequest{
				CampaignId: f.campaignID, EncounterId: ids.enc, IdempotencyKey: ids.key, PendingRevealId: newKey(), Reveal: true,
			}))
			return err
		}},
	}
}

// TestCoreHandlersReadTheirRequestInOrder freezes, for every handler of the combat core, what the
// request is refused with before a transaction opens: who may call, then the key, then the
// encounter, then the combatant. Each spoiled field is the only one wrong, and a call with
// two wrong fields gets the answer of the first.
func TestCoreHandlersReadTheirRequestInOrder(t *testing.T) {
	t.Parallel()
	f := newFight(t)
	stranger := f.h.newUser("Intruso")
	const (
		notUUIDKey  = "idempotency_key must be a UUID"
		noEncounter = "encounter not found"
		noCombatant = "combatant not found"
	)
	for _, call := range coreCalls() {
		good := coreIDs{key: newKey(), enc: newKey(), comb: newKey()}
		wantCoreRefusal(t, call.name+" by a stranger", call.send(f, stranger, good), connect.CodeNotFound, "campaign not found")
		if call.master {
			// A player gets the same answer for any id, whatever it names (RN-10).
			msg := "only the campaign's master can do this"
			if call.name == "ResolveHiddenReveal" {
				msg = "only the master decides whether hidden creatures are revealed"
			}
			wantCoreRefusal(t, call.name+" by a player", call.send(f, f.caio, good), connect.CodePermissionDenied, msg)
		}
		badKey, badEnc, badComb := good, good, good
		badKey.key, badEnc.enc, badComb.comb = "x", "x", "x"
		wantCoreRefusal(t, call.name+" with a bad key", call.send(f, f.master, badKey), connect.CodeInvalidArgument, notUUIDKey)
		wantCoreRefusal(t, call.name+" with a bad encounter", call.send(f, f.master, badEnc), connect.CodeNotFound, noEncounter)
		both := coreIDs{key: "x", enc: "x", comb: "x"}
		wantCoreRefusal(t, call.name+" with every id bad", call.send(f, f.master, both), connect.CodeInvalidArgument, notUUIDKey)
		switch call.name {
		case "SubmitInitiative", "SetCombatantHidden", "RemoveCombatant", "SetCombatantConditions", "StandUp", "RollDeathSave", "ConfirmDeath", "SetSurprised":
			wantCoreRefusal(t, call.name+" with a bad combatant", call.send(f, f.master, badComb), connect.CodeNotFound, noCombatant)
		}
	}
}

// TestCoreHandlersAnswerAnUnknownEncounter freezes what a well-formed request for a combat that
// does not exist gets (there is no combat in the session).
func TestCoreHandlersAnswerAnUnknownEncounter(t *testing.T) {
	t.Parallel()
	f := newFight(t)
	for _, call := range coreCalls() {
		ids := coreIDs{key: newKey(), enc: newKey(), comb: newKey()}
		err := call.send(f, f.master, ids)
		wantCoreRefusal(t, call.name+" on a missing combat", err, connect.CodeNotFound, "encounter not found")
	}
}

// TestSubmitInitiativeRefusesABadRollBeforeTheTransaction freezes the roll a request must carry.
func TestSubmitInitiativeRefusesABadRollBeforeTheTransaction(t *testing.T) {
	t.Parallel()
	f := newFight(t)
	send := func(roll func(*playv1.SubmitInitiativeRequest)) error {
		r := &playv1.SubmitInitiativeRequest{CampaignId: f.campaignID, EncounterId: newKey(), CombatantId: newKey(), IdempotencyKey: newKey()}
		roll(r)
		_, err := f.caio.combat.SubmitInitiative(t.Context(), connect.NewRequest(r))
		return err
	}
	wantCoreRefusal(t, "no roll", send(func(*playv1.SubmitInitiativeRequest) {}), connect.CodeInvalidArgument, "set roll_in_app or d20_face")
	wantCoreRefusal(t, "roll_in_app false", send(func(r *playv1.SubmitInitiativeRequest) {
		r.Roll = &playv1.SubmitInitiativeRequest_RollInApp{RollInApp: false}
	}), connect.CodeInvalidArgument, "roll_in_app must be true")
}

// TestRollDeathSaveRefusesABadRollBeforeTheTransaction freezes the roll a death save must carry.
func TestRollDeathSaveRefusesABadRollBeforeTheTransaction(t *testing.T) {
	t.Parallel()
	f := newFight(t)
	send := func(roll func(*playv1.RollDeathSaveRequest)) error {
		r := &playv1.RollDeathSaveRequest{CampaignId: f.campaignID, EncounterId: newKey(), CombatantId: newKey(), IdempotencyKey: newKey()}
		roll(r)
		_, err := f.caio.combat.RollDeathSave(t.Context(), connect.NewRequest(r))
		return err
	}
	wantCoreRefusal(t, "no roll", send(func(*playv1.RollDeathSaveRequest) {}), connect.CodeInvalidArgument, "set roll_in_app or d20_face")
	wantCoreRefusal(t, "roll_in_app false", send(func(r *playv1.RollDeathSaveRequest) {
		r.Roll = &playv1.RollDeathSaveRequest_RollInApp{RollInApp: false}
	}), connect.CodeInvalidArgument, "roll_in_app must be true")
	for _, face := range []int32{0, 21, -3} {
		wantCoreRefusal(t, "a face out of range", send(func(r *playv1.RollDeathSaveRequest) {
			r.Roll = &playv1.RollDeathSaveRequest_D20Face{D20Face: face}
		}), connect.CodeInvalidArgument, "d20_face must be 1 to 20")
	}
}

// TestSetInitiativeOrderRefusesABadListBeforeTheTransaction freezes the list of tied combatants.
func TestSetInitiativeOrderRefusesABadListBeforeTheTransaction(t *testing.T) {
	t.Parallel()
	f := newFight(t)
	a, b := newKey(), newKey()
	send := func(ids ...string) error {
		_, err := f.master.combat.SetInitiativeOrder(t.Context(), connect.NewRequest(&playv1.SetInitiativeOrderRequest{
			CampaignId: f.campaignID, EncounterId: newKey(), IdempotencyKey: newKey(), CombatantIds: ids,
		}))
		return err
	}
	wantCoreRefusal(t, "an id that is not a UUID", send(a, "x"), connect.CodeNotFound, "combatant not found")
	wantCoreRefusal(t, "a repeated combatant", send(a, b, a), connect.CodeInvalidArgument, "combatant_ids repeats a combatant")
	wantCoreRefusal(t, "no combatants", send(), connect.CodeInvalidArgument, "combatant_ids must name at least two combatants")
	wantCoreRefusal(t, "one combatant", send(a), connect.CodeInvalidArgument, "combatant_ids must name at least two combatants")
	wantCoreRefusal(t, "a bad id after a repeat", send(a, a, "x"), connect.CodeInvalidArgument, "combatant_ids repeats a combatant")
}

// TestStartEncounterRefusesABadRequestBeforeTheTransaction freezes every refusal that comes from
// the request alone, in the order the checks run.
func TestStartEncounterRefusesABadRequestBeforeTheTransaction(t *testing.T) {
	t.Parallel()
	f := newFight(t)
	goblin := f.goblin.GetId()
	party := f.tor.GetId()
	send := func(u *user, edit func(r *playv1.StartEncounterRequest)) error {
		r := &playv1.StartEncounterRequest{
			CampaignId: f.campaignID, IdempotencyKey: newKey(), Name: "Emboscada",
			Participants: []*playv1.Participant{{CharacterId: goblin}},
		}
		edit(r)
		_, err := u.combat.StartEncounter(t.Context(), connect.NewRequest(r))
		return err
	}
	const invalid, notFound = connect.CodeInvalidArgument, connect.CodeNotFound
	tooMany := make([]*playv1.Participant, maxCombatants+1)
	for i := range tooMany {
		tooMany[i] = &playv1.Participant{CharacterId: newKey()}
	}
	tests := []struct {
		name string
		as   *user
		edit func(r *playv1.StartEncounterRequest)
		code connect.Code
		msg  string
	}{
		{"a player", f.caio, func(*playv1.StartEncounterRequest) {}, connect.CodePermissionDenied, "only the campaign's master can do this"},
		{"a key that is not a UUID", f.master, func(r *playv1.StartEncounterRequest) { r.IdempotencyKey = "x" }, invalid, "idempotency_key must be a UUID"},
		{"a name that is empty", f.master, func(r *playv1.StartEncounterRequest) { r.Name = "  " }, invalid, "name must not be empty"},
		{"a point that is not a UUID", f.master, func(r *playv1.StartEncounterRequest) { r.MapPointId = "x" }, notFound, "point not found"},
		{"an unknown mode", f.master, func(r *playv1.StartEncounterRequest) { r.Mode = playv1.EncounterMode(99) }, invalid, "mode must be a mode of combat"},
		{"no participants and no monsters", f.master, func(r *playv1.StartEncounterRequest) { r.Participants = nil }, invalid, "participants must list who fights"},
		{"too many participants", f.master, func(r *playv1.StartEncounterRequest) { r.Participants = tooMany }, invalid, "participants must have at most 40 entries"},
		{"a participant that is not a UUID", f.master, func(r *playv1.StartEncounterRequest) {
			r.Participants = []*playv1.Participant{{CharacterId: "x"}}
		}, notFound, "character not found"},
		{"a repeated character", f.master, func(r *playv1.StartEncounterRequest) {
			r.Participants = []*playv1.Participant{{CharacterId: goblin}, {CharacterId: goblin}}
		}, invalid, "participants[1] repeats a character"},
		{"a character of nobody's campaign", f.master, func(r *playv1.StartEncounterRequest) {
			r.Participants = []*playv1.Participant{{CharacterId: newKey()}}
		}, notFound, "character not found"},
		{"a player's character twice", f.master, func(r *playv1.StartEncounterRequest) {
			r.Participants = []*playv1.Participant{{CharacterId: party, Count: 2}}
		}, invalid, "participants[0]: a player's character joins once, and is never hidden"},
		{"a player's character hidden", f.master, func(r *playv1.StartEncounterRequest) {
			hidden := true
			r.Participants = []*playv1.Participant{{CharacterId: party, Hidden: &hidden}}
		}, invalid, "participants[0]: a player's character joins once, and is never hidden"},
		{"too many copies of an NPC", f.master, func(r *playv1.StartEncounterRequest) {
			r.Participants = []*playv1.Participant{{CharacterId: goblin, Count: maxNPCCopies + 1}}
		}, invalid, "participants[0].count must be 1 to 10"},
		{"a negative count", f.master, func(r *playv1.StartEncounterRequest) {
			r.Participants = []*playv1.Participant{{CharacterId: goblin, Count: -1}}
		}, invalid, "participants[0].count must be 1 to 10"},
		{"more combatants than a combat holds", f.master, func(r *playv1.StartEncounterRequest) {
			r.Participants = []*playv1.Participant{{CharacterId: goblin, Count: maxNPCCopies}, {CharacterId: party}}
			for i := range 4 {
				extra := f.master.createCharacter(t, f.campaignID, charactersv1.CharacterKind_CHARACTER_KIND_MINION, "Extra"+strings.Repeat("x", i+1))
				r.Participants = append(r.Participants, &playv1.Participant{CharacterId: extra.GetId(), Count: maxNPCCopies})
			}
		}, invalid, "a combat has at most 40 combatants"},
		{"an unknown hit points mode", f.master, func(r *playv1.StartEncounterRequest) {
			r.MonsterHitPoints = playv1.MonsterHitPoints(99)
		}, invalid, "monster_hit_points is not a known mode"},
		{"a monster creature that is not in the SRD", f.master, func(r *playv1.StartEncounterRequest) {
			r.Monsters = []*playv1.MonsterGroup{{CreatureKey: "nope"}}
		}, invalid, "monsters[0].creature_key is not an SRD creature"},
		{"a monster count out of range", f.master, func(r *playv1.StartEncounterRequest) {
			r.Monsters = []*playv1.MonsterGroup{{CreatureKey: bandit, Count: maxEntryCount + 1}}
		}, invalid, "monsters[0].count must be 1 to 40"},
		{"a creature twice", f.master, func(r *playv1.StartEncounterRequest) {
			r.Monsters = []*playv1.MonsterGroup{{CreatureKey: bandit}, {CreatureKey: bandit}}
		}, invalid, "monsters[1] repeats a creature"},
	}
	for _, tc := range tests {
		wantCoreRefusal(t, tc.name, send(tc.as, tc.edit), tc.code, tc.msg)
	}
	if e := f.get(t, f.master); e != nil {
		t.Errorf("a refused StartEncounter left a combat: %v", e)
	}
}

// TestAddMonstersRefusesABadRequestBeforeTheTransaction freezes the checks of the request, in order.
func TestAddMonstersRefusesABadRequestBeforeTheTransaction(t *testing.T) {
	t.Parallel()
	f := newFight(t)
	send := func(edit func(r *playv1.AddMonstersRequest)) error {
		r := &playv1.AddMonstersRequest{CampaignId: f.campaignID, EncounterId: newKey(), IdempotencyKey: newKey(), CreatureKey: bandit}
		edit(r)
		_, err := f.master.combat.AddMonsters(t.Context(), connect.NewRequest(r))
		return err
	}
	const invalid = connect.CodeInvalidArgument
	wantCoreRefusal(t, "a count over the limit", send(func(r *playv1.AddMonstersRequest) { r.Count = maxNPCCopies + 1 }), invalid, "count must be 1 to 10")
	wantCoreRefusal(t, "a negative count", send(func(r *playv1.AddMonstersRequest) { r.Count = -1 }), invalid, "count must be 1 to 10")
	wantCoreRefusal(t, "an unknown hit points mode", send(func(r *playv1.AddMonstersRequest) { r.HitPoints = playv1.MonsterHitPoints(99) }), invalid, "hit_points is not a known mode")
	wantCoreRefusal(t, "a creature that is not in the SRD", send(func(r *playv1.AddMonstersRequest) { r.CreatureKey = "monster:nope" }), invalid, "creature_key is not an SRD creature")
	wantCoreRefusal(t, "a name that is too long", send(func(r *playv1.AddMonstersRequest) { r.Name = strings.Repeat("x", maxMonsterName+1) }), invalid, "name: must be at most 30 characters, got 31")
	// The count is read before the mode, the mode before the creature, the creature before the name.
	wantCoreRefusal(t, "every field bad", send(func(r *playv1.AddMonstersRequest) {
		r.Count, r.HitPoints, r.CreatureKey, r.Name = 99, playv1.MonsterHitPoints(99), "x", strings.Repeat("x", 99)
	}), invalid, "count must be 1 to 10")
}

// TestSetCombatantConditionsRefusesABadRequestBeforeTheTransaction freezes what the list of
// conditions must be.
func TestSetCombatantConditionsRefusesABadRequestBeforeTheTransaction(t *testing.T) {
	t.Parallel()
	f := newFight(t)
	send := func(edit func(r *playv1.SetCombatantConditionsRequest)) error {
		r := &playv1.SetCombatantConditionsRequest{CampaignId: f.campaignID, EncounterId: newKey(), CombatantId: newKey(), IdempotencyKey: newKey()}
		edit(r)
		_, err := f.master.combat.SetCombatantConditions(t.Context(), connect.NewRequest(r))
		return err
	}
	keys := func(k ...string) func(*playv1.SetCombatantConditionsRequest) {
		return func(r *playv1.SetCombatantConditionsRequest) { r.Conditions = &playv1.ConditionList{Keys: k} }
	}
	const invalid = connect.CodeInvalidArgument
	wantCoreRefusal(t, "nothing to do", send(func(*playv1.SetCombatantConditionsRequest) {}), invalid, "set conditions or end_concentration")
	wantCoreRefusal(t, "an unknown condition", send(keys(condProne, "nope")), invalid, "conditions.keys[1] is not one of the SRD's conditions")
	wantCoreRefusal(t, "a repeated condition", send(keys(condProne, condProne)), invalid, "conditions.keys[1] repeats a condition")
	wantCoreRefusal(t, "too many conditions", send(keys(make([]string, maxConditions+1)...)), invalid, "conditions must have at most 20 keys")
}

// TestAddCombatantsRefusesBadParticipantsBeforeTheTransaction freezes that an add, unlike a start,
// needs someone to add.
func TestAddCombatantsRefusesBadParticipantsBeforeTheTransaction(t *testing.T) {
	t.Parallel()
	f := newFight(t)
	_, err := f.master.combat.AddCombatants(t.Context(), connect.NewRequest(&playv1.AddCombatantsRequest{
		CampaignId: f.campaignID, EncounterId: newKey(), IdempotencyKey: newKey(),
	}))
	wantCoreRefusal(t, "no participants", err, connect.CodeInvalidArgument, "participants must list who fights")
	_, err = f.master.combat.AddCombatants(t.Context(), connect.NewRequest(&playv1.AddCombatantsRequest{
		CampaignId: f.campaignID, EncounterId: newKey(), IdempotencyKey: newKey(),
		Participants: []*playv1.Participant{{CharacterId: f.goblin.GetId()}},
	}))
	wantCoreRefusal(t, "a combat that does not exist", err, connect.CodeNotFound, "encounter not found")
}
