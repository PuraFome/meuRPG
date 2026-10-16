package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The characterization of SetExhaustion and LowerExhaustion (see lasting_core_frozen_test.go).

// coreSetExhaustion calls SetExhaustion as u, with the request edited by edit.
func (a *armed) coreSetExhaustion(t *testing.T, u *user, e *playv1.Encounter, edit func(*playv1.SetExhaustionRequest)) (*playv1.SetExhaustionResponse, error) {
	t.Helper()
	req := &playv1.SetExhaustionRequest{CampaignId: a.campaignID, IdempotencyKey: newKey(), EncounterId: e.GetId(), Level: 1}
	edit(req)
	res, err := u.lasting.SetExhaustion(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// coreLowerExhaustion calls LowerExhaustion as u, with the request edited by edit.
func (a *armed) coreLowerExhaustion(t *testing.T, u *user, e *playv1.Encounter, edit func(*playv1.LowerExhaustionRequest)) (*playv1.LowerExhaustionResponse, error) {
	t.Helper()
	req := &playv1.LowerExhaustionRequest{
		CampaignId: a.campaignID, IdempotencyKey: newKey(), EncounterId: e.GetId(), By: 1, Reason: playv1.ExhaustionLowerReason_EXHAUSTION_LOWER_REASON_MASTER,
	}
	edit(req)
	res, err := u.lasting.LowerExhaustion(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func setOn(id string) func(*playv1.SetExhaustionRequest) {
	return func(r *playv1.SetExhaustionRequest) {
		r.Subject = &playv1.SetExhaustionRequest_CombatantId{CombatantId: id}
	}
}

func lowerOn(id string) func(*playv1.LowerExhaustionRequest) {
	return func(r *playv1.LowerExhaustionRequest) {
		r.Subject = &playv1.LowerExhaustionRequest_CombatantId{CombatantId: id}
	}
}

// Every refusal of the two calls, in the order the checks run: who calls, the key, the subject,
// the numbers, then what the combat holds. Nothing is written.
func TestLastingCoreExhaustionRefusals(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	goblin := a.id(t, "Goblin")
	var l coreLog
	set := func(name string, u *user, edit func(*playv1.SetExhaustionRequest)) {
		t.Helper()
		_, err := a.coreSetExhaustion(t, u, e, edit)
		l.say("set %s -> %s", name, coreRefusal(err))
	}
	lower := func(name string, u *user, edit func(*playv1.LowerExhaustionRequest)) {
		t.Helper()
		_, err := a.coreLowerExhaustion(t, u, e, edit)
		l.say("lower %s -> %s", name, coreRefusal(err))
	}
	set("by a player", a.caio, setOn(goblin))
	set("a bad key", a.master, func(r *playv1.SetExhaustionRequest) { setOn(goblin)(r); r.IdempotencyKey = "not-a-key" })
	set("no subject", a.master, func(*playv1.SetExhaustionRequest) {})
	set("no subject and a bad level", a.master, func(r *playv1.SetExhaustionRequest) { r.Level = 9 })
	set("a bad character id", a.master, func(r *playv1.SetExhaustionRequest) {
		r.Subject = &playv1.SetExhaustionRequest_CharacterId{CharacterId: "nope"}
	})
	set("a bad encounter id", a.master, func(r *playv1.SetExhaustionRequest) { setOn(goblin)(r); r.EncounterId = "nope" })
	set("an empty encounter id", a.master, func(r *playv1.SetExhaustionRequest) { setOn(goblin)(r); r.EncounterId = "" })
	set("a bad combatant id", a.master, setOn("nope"))
	set("a bad combatant id and a bad level", a.master, func(r *playv1.SetExhaustionRequest) { setOn("nope")(r); r.Level = -1 })
	set("a level under 0", a.master, func(r *playv1.SetExhaustionRequest) { setOn(goblin)(r); r.Level = -1 })
	set("a level over 6", a.master, func(r *playv1.SetExhaustionRequest) { setOn(goblin)(r); r.Level = 7 })
	set("a combatant that is not in the combat", a.master, setOn(a.toren.GetId()))
	set("a character that is not in the campaign", a.master, func(r *playv1.SetExhaustionRequest) {
		r.Subject = &playv1.SetExhaustionRequest_CharacterId{CharacterId: a.goblin.GetId()}
	})
	set("a stale expected level", a.master, func(r *playv1.SetExhaustionRequest) { setOn(goblin)(r); r.ExpectedLevel = 2 })
	set("level 6 without confirmation", a.master, func(r *playv1.SetExhaustionRequest) { setOn(goblin)(r); r.Level = 6 })
	set("a stale level before level 6 asks to confirm", a.master, func(r *playv1.SetExhaustionRequest) {
		setOn(goblin)(r)
		r.Level, r.ExpectedLevel = 6, 1
	})
	lower("by a player", a.caio, lowerOn(goblin))
	lower("a bad key", a.master, func(r *playv1.LowerExhaustionRequest) { lowerOn(goblin)(r); r.IdempotencyKey = "not-a-key" })
	lower("no subject", a.master, func(*playv1.LowerExhaustionRequest) {})
	lower("by 0", a.master, func(r *playv1.LowerExhaustionRequest) { lowerOn(goblin)(r); r.By = 0 })
	lower("by 7", a.master, func(r *playv1.LowerExhaustionRequest) { lowerOn(goblin)(r); r.By = 7 })
	lower("by -1", a.master, func(r *playv1.LowerExhaustionRequest) { lowerOn(goblin)(r); r.By = -1 })
	lower("no reason", a.master, func(r *playv1.LowerExhaustionRequest) {
		lowerOn(goblin)(r)
		r.Reason = playv1.ExhaustionLowerReason_EXHAUSTION_LOWER_REASON_UNSPECIFIED
	})
	lower("by 0 and no reason", a.master, func(r *playv1.LowerExhaustionRequest) {
		lowerOn(goblin)(r)
		r.By, r.Reason = 0, playv1.ExhaustionLowerReason_EXHAUSTION_LOWER_REASON_UNSPECIFIED
	})
	lower("a stale expected level", a.master, func(r *playv1.LowerExhaustionRequest) { lowerOn(goblin)(r); r.ExpectedLevel = 3 })
	l.say("state:\n%s", a.coreCombatants(t, e.GetId()))
	l.say("events:\n%s", a.coreEvents(t, eventExhaustion))
	wantLastingGolden(t, "exhaustion_refusals", a.coreNames(t, l.String()))
}

// An NPC takes the levels one by one: the speed, the maximum halved at 4 with the old one kept,
// speed 0 at 5, defeated at 6 (which takes it out of the turn it holds); taking the levels off
// gives the maximum back and defeats nobody twice.
func TestLastingCoreExhaustionLadderOfAnNPC(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	a.passTo(t, e, "Goblin")
	goblin := a.id(t, "Goblin")
	var l coreLog
	var before int32
	step := func(name string, res interface {
		GetLevel() int32
		GetHitPointsMax() int32
	}, err error,
	) {
		t.Helper()
		if err != nil {
			l.say("%s -> %s", name, coreRefusal(err))
			return
		}
		l.say("%s -> level %d max %d, current turn %q", name, res.GetLevel(), res.GetHitPointsMax(), a.get(t, a.master).GetCurrentCombatantId())
		before = res.GetLevel()
		l.say("%s", a.coreCombatants(t, e.GetId(), "Goblin"))
	}
	for _, level := range []int32{1, 2, 3, 4, 5} {
		res, err := a.coreSetExhaustion(t, a.master, e, func(r *playv1.SetExhaustionRequest) {
			setOn(goblin)(r)
			r.Level, r.ExpectedLevel = level, before
		})
		step("set "+string(rune('0'+level)), res, err)
	}
	res, err := a.coreSetExhaustion(t, a.master, e, func(r *playv1.SetExhaustionRequest) {
		setOn(goblin)(r)
		r.Level, r.ExpectedLevel, r.ConfirmDeath = 6, before, true
	})
	step("set 6", res, err)
	// Level 6 again: the same level, so no confirmation is asked.
	res, err = a.coreSetExhaustion(t, a.master, e, func(r *playv1.SetExhaustionRequest) {
		setOn(goblin)(r)
		r.Level, r.ExpectedLevel = 6, before
	})
	step("set 6 again", res, err)
	for _, by := range []int32{1, 3, 6} {
		res, err := a.coreLowerExhaustion(t, a.master, e, func(r *playv1.LowerExhaustionRequest) {
			lowerOn(goblin)(r)
			r.By, r.ExpectedLevel = by, before
		})
		step("lower by "+string(rune('0'+by)), res, err)
	}
	l.say("events:\n%s", a.coreEvents(t, eventExhaustion, eventTurnEnded))
	wantLastingGolden(t, "exhaustion_npc_ladder", a.coreNames(t, l.String()))
}

// An NPC that is not on turn, and one the players do not see: the maximum and the speed are the
// same, the turn does not move, and the event is secret.
func TestLastingCoreExhaustionOfAnNPCOffTurnAndOfAHiddenOne(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}, {CharacterId: a.capitao.GetId()}},
		npcRolls: []int{3, 2},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Pensantus": {10, 3}, "Brisa": {4, 4}},
	})
	var l coreLog
	for _, c := range []struct {
		label string
		level int32
	}{{"Goblin", 6}, {"Capitão Goblin", 4}, {"Capitão Goblin", 2}} {
		id := a.id(t, c.label)
		before := byLabel(t, a.get(t, a.master), c.label).GetExhaustionLevel()
		res, err := a.coreSetExhaustion(t, a.master, e, func(r *playv1.SetExhaustionRequest) {
			setOn(id)(r)
			r.Level, r.ExpectedLevel, r.ConfirmDeath = c.level, before, true
		})
		if err != nil {
			t.Fatalf("SetExhaustion(%s) error = %v", c.label, err)
		}
		l.say("%s %d -> level %d max %d, current turn %q", c.label, c.level, res.GetLevel(), res.GetHitPointsMax(), a.get(t, a.master).GetCurrentCombatantId())
	}
	l.say("%s", a.coreCombatants(t, e.GetId(), "Goblin", "Capitão Goblin"))
	l.say("events:\n%s", a.coreEvents(t, eventExhaustion, eventTurnEnded))
	wantLastingGolden(t, "exhaustion_npc_hidden", a.coreNames(t, l.String()))
}

// A character is named by its combatant or by its character id, inside a combat or out of it:
// the level is its vitals', the maximum halves at 4, level 6 takes it to the death confirmation
// in a combat and kills it at once outside one.
func TestLastingCoreExhaustionOfCharacters(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	var l coreLog
	// Out of a combat, by the character id.
	byChar := func(c interface{ GetId() string }) func(*playv1.SetExhaustionRequest) {
		return func(r *playv1.SetExhaustionRequest) {
			r.Subject = &playv1.SetExhaustionRequest_CharacterId{CharacterId: c.GetId()}
		}
	}
	idle := &playv1.Encounter{}
	res, err := a.coreSetExhaustion(t, a.master, idle, func(r *playv1.SetExhaustionRequest) { byChar(a.bri)(r); r.Level = 4 })
	if err != nil {
		t.Fatalf("SetExhaustion(Brisa) error = %v", err)
	}
	l.say("out of a combat: Brisa 4 -> level %d max %d; %s", res.GetLevel(), res.GetHitPointsMax(), a.coreVitals(t, "Brisa", a.bri))
	low, err := a.coreLowerExhaustion(t, a.master, idle, func(r *playv1.LowerExhaustionRequest) {
		r.Subject = &playv1.LowerExhaustionRequest_CharacterId{CharacterId: a.bri.GetId()}
		r.By, r.ExpectedLevel = 2, 4
	})
	if err != nil {
		t.Fatalf("LowerExhaustion(Brisa) error = %v", err)
	}
	l.say("out of a combat: Brisa lower by 2 -> level %d max %d; %s", low.GetLevel(), low.GetHitPointsMax(), a.coreVitals(t, "Brisa", a.bri))
	// An ended combat is no combat for it.
	e := a.closeFight(t)
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	res, err = a.coreSetExhaustion(t, a.master, idle, func(r *playv1.SetExhaustionRequest) { byChar(a.toren)(r); r.Level = 2 })
	if err != nil {
		t.Fatalf("SetExhaustion(Toren) error = %v", err)
	}
	l.say("after the combat ended: Toren 2 -> level %d max %d; %s", res.GetLevel(), res.GetHitPointsMax(), a.coreVitals(t, "Toren", a.toren))
	res, err = a.coreSetExhaustion(t, a.master, idle, func(r *playv1.SetExhaustionRequest) { byChar(a.pens)(r); r.Level = 6 })
	l.say("out of a combat: Pensantus 6 without confirmation -> %s", coreRefusal(err))
	res, err = a.coreSetExhaustion(t, a.master, idle, func(r *playv1.SetExhaustionRequest) { byChar(a.pens)(r); r.Level, r.ConfirmDeath = 6, true })
	if err != nil {
		t.Fatalf("SetExhaustion(Pensantus) error = %v", err)
	}
	l.say("out of a combat: Pensantus 6 -> level %d max %d; state %s", res.GetLevel(), res.GetHitPointsMax(), a.master.character(t, a.pens).GetState())
	// The same level again asks for nothing and changes nothing.
	res, err = a.coreSetExhaustion(t, a.master, idle, func(r *playv1.SetExhaustionRequest) { byChar(a.pens)(r); r.Level, r.ExpectedLevel = 6, 6 })
	l.say("out of a combat: Pensantus 6 again -> %v level %d", coreRefusal(err), res.GetLevel())
	l.say("%s", a.coreCombatants(t, e.GetId()))
	l.say("events:\n%s", a.coreEvents(t, eventExhaustion))
	wantLastingGolden(t, "exhaustion_characters_out", a.coreNames(t, l.String()))
}

// In a combat, a character is found among the combatants even when it is named by the character
// id; the level is copied to its combatant and level 6 sets the third failed death save.
func TestLastingCoreExhaustionOfCharactersInACombat(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	var l coreLog
	res, err := a.coreSetExhaustion(t, a.master, e, func(r *playv1.SetExhaustionRequest) {
		r.Subject = &playv1.SetExhaustionRequest_CharacterId{CharacterId: a.toren.GetId()}
		r.Level = 3
	})
	if err != nil {
		t.Fatalf("SetExhaustion(Toren) error = %v", err)
	}
	l.say("Toren by character id 3 -> level %d max %d; %s", res.GetLevel(), res.GetHitPointsMax(), a.coreVitals(t, "Toren", a.toren))
	low, err := a.coreLowerExhaustion(t, a.master, e, func(r *playv1.LowerExhaustionRequest) {
		r.Subject = &playv1.LowerExhaustionRequest_CharacterId{CharacterId: a.toren.GetId()}
		r.By, r.ExpectedLevel = 5, 3
	})
	if err != nil {
		t.Fatalf("LowerExhaustion(Toren) error = %v", err)
	}
	l.say("Toren lower by 5 -> level %d max %d; %s", low.GetLevel(), low.GetHitPointsMax(), a.coreVitals(t, "Toren", a.toren))
	for _, level := range []int32{4, 5} {
		res, err = a.coreSetExhaustion(t, a.master, e, func(r *playv1.SetExhaustionRequest) {
			setOn(a.id(t, "Brisa"))(r)
			r.Level, r.ExpectedLevel = level, level-1
			if level == 4 {
				r.ExpectedLevel = 0
			}
		})
		if err != nil {
			t.Fatalf("SetExhaustion(Brisa) error = %v", err)
		}
		l.say("Brisa by combatant %d -> level %d max %d; %s", level, res.GetLevel(), res.GetHitPointsMax(), a.coreVitals(t, "Brisa", a.bri))
	}
	res, err = a.coreSetExhaustion(t, a.master, e, func(r *playv1.SetExhaustionRequest) {
		setOn(a.id(t, "Brisa"))(r)
		r.Level, r.ExpectedLevel, r.ConfirmDeath = 6, 5, true
	})
	if err != nil {
		t.Fatalf("SetExhaustion(Brisa 6) error = %v", err)
	}
	l.say("Brisa by combatant 6 -> level %d max %d; %s", res.GetLevel(), res.GetHitPointsMax(), a.coreVitals(t, "Brisa", a.bri))
	res, err = a.coreSetExhaustion(t, a.master, e, func(r *playv1.SetExhaustionRequest) {
		setOn(a.id(t, "Brisa"))(r)
		r.Level, r.ExpectedLevel = 6, 6
	})
	l.say("Brisa 6 again -> %s level %d", coreRefusal(err), res.GetLevel())
	l.say("%s", a.coreCombatants(t, e.GetId()))
	l.say("events:\n%s", a.coreEvents(t, eventExhaustion))
	wantLastingGolden(t, "exhaustion_characters_in", a.coreNames(t, l.String()))
}
