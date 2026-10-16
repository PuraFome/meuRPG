package play

import (
	"testing"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The scenarios of the RollDamage golden that need more than the usual fight: the goblin's hits on a player's
// character, the table's rules, the extras of a rogue, a barbarian, a paladin and a fighter with maneuvers,
// the spells' damage and heals, the reactions that hold a damage and the opportunity attack.

func setHidden(t *testing.T, a *armed, e *playv1.Encounter, label string, hidden bool) {
	t.Helper()
	if _, err := a.master.combat.SetCombatantHidden(t.Context(), connect.NewRequest(&playv1.SetCombatantHiddenRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, label), IdempotencyKey: newKey(), Hidden: hidden,
	})); err != nil {
		t.Fatalf("SetCombatantHidden(%s) error = %v", label, err)
	}
}

func endEncounter(t *testing.T, a *armed, e *playv1.Encounter) {
	t.Helper()
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
}

// rollFirst rolls the damage of the pending damage as the master, in the app, before the scenario's own roll.
func rollFirst(t *testing.T, a *armed, e *playv1.Encounter, id string) {
	t.Helper()
	if _, err := a.rawDamage(t, a.master, e, id, inAppDamage); err != nil {
		t.Fatalf("RollDamage() error = %v", err)
	}
}

// usualDamages are the scenarios of the usual fight that change it or that play the goblin's hit.
func usualDamages() []usualDamage {
	setTemp := func(t *testing.T, a *armed, _ *playv1.Encounter, _ string) {
		t.Helper()
		a.execSQL(t, `UPDATE combatants SET hp_temp = 4 WHERE id = $1`, a.id(t, "Goblin"))
	}
	return []usualDamage{
		{name: "a hit with scripted faces", before: scriptedFaces(2)},
		{name: "an unarmed strike, which has no dice", key: unarmed, edit: inAppDamage},
		{name: "an unarmed strike as the master", key: unarmed, as: asMaster},
		{name: "an unarmed strike on a critical hit", key: unarmed, crit: true},
		{name: "the goblin hits Toren and hides before the roll", goblin: true, as: asToren, prepare: func(t *testing.T, a *armed, e *playv1.Encounter, _ string) {
			t.Helper()
			setHidden(t, a, e, "Goblin", true)
		}},
		{name: "a hit that leaves the goblin standing", edit: typedSum(1)},
		{name: "a hit with temporary hit points on the goblin", prepare: setTemp, edit: typedSum(2)},
		{name: "a hit that goes through the temporary hit points", prepare: setTemp, edit: typedSum(6)},
		{name: "a hit on a goblin that concentrates", edit: typedSum(1), prepare: func(t *testing.T, a *armed, _ *playv1.Encounter, _ string) {
			t.Helper()
			a.concentrate(t, "Goblin", webSpell)
		}},
		{name: "a hit on a hidden goblin as its player", prepare: func(t *testing.T, a *armed, e *playv1.Encounter, _ string) {
			t.Helper()
			setHidden(t, a, e, "Goblin", true)
		}},
		{name: "a hit on a hidden goblin as the master", as: asMaster, before: scriptedFaces(3), prepare: func(t *testing.T, a *armed, e *playv1.Encounter, _ string) {
			t.Helper()
			setHidden(t, a, e, "Goblin", true)
		}},
		{name: "a typed sum when everybody rolls in the app", edit: typedSum(5), prepare: func(t *testing.T, a *armed, _ *playv1.Encounter, _ string) {
			t.Helper()
			a.forceDice(t, campaignsv1.DiceMode_DICE_MODE_APP)
		}},
		{name: "the app's roll when everybody rolls real dice", prepare: func(t *testing.T, a *armed, _ *playv1.Encounter, _ string) {
			t.Helper()
			a.forceDice(t, campaignsv1.DiceMode_DICE_MODE_PHYSICAL)
		}},
		{name: "the app's roll as the master when everybody rolls real dice", as: asMaster, prepare: func(t *testing.T, a *armed, _ *playv1.Encounter, _ string) {
			t.Helper()
			a.forceDice(t, campaignsv1.DiceMode_DICE_MODE_PHYSICAL)
		}},
		{name: "a roll in a combat that ended", prepare: func(t *testing.T, a *armed, e *playv1.Encounter, _ string) {
			t.Helper()
			endEncounter(t, a, e)
		}},
		{name: "a roll on a damage the master discarded", prepare: func(t *testing.T, a *armed, e *playv1.Encounter, id string) {
			t.Helper()
			if _, err := a.settle(t, a.master, e, id, false); err != nil {
				t.Fatalf("DiscardPendingDamage() error = %v", err)
			}
		}},
		{name: "a roll on a damage rolled and applied already", prepare: rollFirstAsMaster},
		{name: "a critical hit under the maximum plus a roll", crit: true, before: func(t *testing.T, a *armed, _ *playv1.Encounter) {
			t.Helper()
			a.setRules(t, criticalIs(maxRoll))
			a.h.roller.queue(3)
		}},
		{name: "a critical hit under the maximum plus a roll, typed", crit: true, edit: typedSum(5), before: func(t *testing.T, a *armed, _ *playv1.Encounter) {
			t.Helper()
			a.setRules(t, criticalIs(maxRoll))
		}},
		{name: "the goblin hits Toren, rolled in the app", goblin: true, before: scriptedFaces(3)},
		{name: "the goblin hits Toren, rolled with a typed sum", goblin: true, edit: typedSum(4)},
		{name: "the goblin crits Toren, rolled in the app", goblin: true, crit: true, before: scriptedFaces(3, 5)},
		{name: "the goblin hits Toren, rolled by Toren's player", goblin: true, as: asToren},
		{name: "the goblin hits Toren, rolled twice", goblin: true, prepare: rollFirstAsMaster},
		{name: "the goblin hits Toren, rolled and applied", goblin: true, prepare: func(t *testing.T, a *armed, e *playv1.Encounter, id string) {
			t.Helper()
			rollFirst(t, a, e, id)
			if _, err := a.settle(t, a.master, e, id, true); err != nil {
				t.Fatalf("ApplyPendingDamage() error = %v", err)
			}
		}},
		{name: "the goblin hits Toren, rolled and discarded", goblin: true, prepare: func(t *testing.T, a *armed, e *playv1.Encounter, id string) {
			t.Helper()
			rollFirst(t, a, e, id)
			if _, err := a.settle(t, a.master, e, id, false); err != nil {
				t.Fatalf("DiscardPendingDamage() error = %v", err)
			}
		}},
		{name: "the goblin hits Toren at 0 hit points", goblin: true, prepare: func(t *testing.T, a *armed, _ *playv1.Encounter, _ string) {
			t.Helper()
			a.correct(t, a.toren, hpIs(0))
		}},
		{name: "the goblin crits Toren at 0 hit points", goblin: true, crit: true, prepare: func(t *testing.T, a *armed, _ *playv1.Encounter, _ string) {
			t.Helper()
			a.correct(t, a.toren, hpIs(0))
		}},
		{name: "the goblin hits Toren who concentrates", goblin: true, edit: typedSum(3), prepare: func(t *testing.T, a *armed, _ *playv1.Encounter, _ string) {
			t.Helper()
			a.concentrate(t, "Toren", webSpell)
		}},
	}
}

// rollFirstAsMaster rolls the damage as the master, so that it is rolled before the scenario's own roll.
func rollFirstAsMaster(t *testing.T, a *armed, e *playv1.Encounter, id string) {
	t.Helper()
	rollFirst(t, a, e, id)
}

// The extras of a hit: Sneak Attack of a rogue, the rage of a barbarian, Divine Smite of a paladin and the
// maneuvers of a fighter.

// extrasDamage is a scenario that plays its own table up to a hit and rolls its damage.
type extrasDamage struct {
	name string
	// hit plays the table and the attack and answers the table, the encounter and the pending damage.
	hit  func(t *testing.T) (*armed, *playv1.Encounter, string)
	as   func(a *armed) *user
	edit func(a *armed, r *playv1.RollDamageRequest)
	// before runs after the hit, before the roll.
	before func(t *testing.T, a *armed, e *playv1.Encounter, id string)
}

func (x extrasDamage) scenario() damageScenario {
	return damageOn(x.name, func(t *testing.T) damagePlay {
		t.Helper()
		a, e, id := x.hit(t)
		if x.before != nil {
			x.before(t, a, e, id)
		}
		as := x.as
		if as == nil {
			as = func(a *armed) *user { return a.caio }
		}
		return a.rolled(t, as(a), e, id, func(r *playv1.RollDamageRequest) {
			inAppDamage(r)
			if x.edit != nil {
				x.edit(a, r)
			}
		})
	})
}

func markedExtras(keys ...string) func(a *armed, r *playv1.RollDamageRequest) {
	return func(_ *armed, r *playv1.RollDamageRequest) {
		r.ExtrasChosen = true
		for _, k := range keys {
			r.SelectedExtras = append(r.SelectedExtras, &playv1.SelectedExtra{Key: k})
		}
	}
}

func typedMarked(parts map[string]int32, keys ...string) func(a *armed, r *playv1.RollDamageRequest) {
	return func(a *armed, r *playv1.RollDamageRequest) {
		markedExtras(keys...)(a, r)
		r.Roll = nil
		for _, k := range []string{"weapon", "sneak-attack", "divine-smite", "divine-smite-extra", "rage"} {
			if n, ok := parts[k]; ok {
				r.TypedParts = append(r.TypedParts, &playv1.TypedPart{PartKey: k, Sum: n})
			}
		}
	}
}

// rogueHit is Toren, a rogue, hitting the restrained goblin with the d20 faces given (advantage when two).
func rogueHit(faces []int32, tweak func(t *testing.T, a *armed, e *playv1.Encounter)) func(t *testing.T) (*armed, *playv1.Encounter, string) {
	return func(t *testing.T) (*armed, *playv1.Encounter, string) {
		t.Helper()
		a, e := rollsTable(t)
		e = a.passTo(t, e, "Toren")
		a.setConditions(t, e, "Goblin", "condition:restrained")
		if tweak != nil {
			tweak(t, a, e)
		}
		res := a.mustAttack(t, a.caio, a.get(t, a.caio), "Toren", rapier, "Goblin", func(r *playv1.RollAttackRequest) { r.D20Faces = faces })
		if res.GetPendingDamage() == nil {
			t.Fatalf("RollAttack() = %v, want a hit", res.GetRoll())
		}
		return a, e, res.GetPendingDamage().GetId()
	}
}

// rageHit is Brisa, a raging barbarian, hitting the goblin with her battleaxe.
func rageHit(t *testing.T) (*armed, *playv1.Encounter, string) {
	t.Helper()
	a, e := rollsTable(t)
	e, err := a.action(t, a.bia, e, "Brisa", "feature:rage")
	if err != nil {
		t.Fatalf("TakeAction(Rage) error = %v", err)
	}
	res := a.mustAttack(t, a.bia, e, "Brisa", battleaxe, "Goblin", d20(15))
	if res.GetPendingDamage() == nil {
		t.Fatalf("RollAttack() = %v, want a hit", res.GetRoll())
	}
	return a, e, res.GetPendingDamage().GetId()
}

// rageTarget is the goblin hitting Brisa while she rages.
func rageTarget(face int32) func(t *testing.T) (*armed, *playv1.Encounter, string) {
	return func(t *testing.T) (*armed, *playv1.Encounter, string) {
		t.Helper()
		a, e := rollsTable(t)
		e, err := a.action(t, a.bia, e, "Brisa", "feature:rage")
		if err != nil {
			t.Fatalf("TakeAction(Rage) error = %v", err)
		}
		a.mustAttack(t, a.bia, e, "Brisa", battleaxe, "Goblin", d20(2)) // attacked a hostile: the rage holds
		e = a.passTo(t, a.mustEndTurn(t, a.bia, e), "Goblin")
		res := a.mustAttack(t, a.master, e, "Goblin", sword, "Brisa", d20(face))
		if res.GetPendingDamage() == nil {
			t.Fatalf("RollAttack() = %v, want a hit", res.GetRoll())
		}
		return a, e, res.GetPendingDamage().GetId()
	}
}

func asMasterUser(a *armed) *user { return a.master }

func physicalFor(t *testing.T, a *armed, _ *playv1.Encounter, _ string) {
	t.Helper()
	a.forceDice(t, campaignsv1.DiceMode_DICE_MODE_PHYSICAL)
}

// rogueAndBarbarianDamages are the rogue's Sneak Attack and the barbarian's rage.
func rogueAndBarbarianDamages() []damageScenario {
	advantaged, critical := []int32{5, 18}, []int32{20, 5}
	sneak := map[string]int32{"weapon": 3, "sneak-attack": 5}
	rows := []extrasDamage{
		{name: "sneak attack with typed parts", hit: rogueHit(advantaged, nil), edit: typedMarked(sneak, "sneak-attack")},
		{
			name: "sneak attack rolled in the app", hit: rogueHit(advantaged, nil), edit: markedExtras("sneak-attack"),
			before: func(t *testing.T, a *armed, _ *playv1.Encounter, _ string) { t.Helper(); a.h.roller.queue(3, 4, 2) },
		},
		{
			name: "sneak attack by default", hit: rogueHit(advantaged, nil),
			before: func(t *testing.T, a *armed, _ *playv1.Encounter, _ string) { t.Helper(); a.h.roller.queue(3, 4, 2) },
		},
		{
			name: "extras chosen and none marked", hit: rogueHit(advantaged, nil), edit: markedExtras(),
			before: func(t *testing.T, a *armed, _ *playv1.Encounter, _ string) { t.Helper(); a.h.roller.queue(6) },
		},
		{name: "sneak attack with a typed sum only", hit: rogueHit(advantaged, nil), edit: func(a *armed, r *playv1.RollDamageRequest) {
			markedExtras("sneak-attack")(a, r)
			typedSum(9)(r)
		}},
		{name: "sneak attack with no sum for the weapon", hit: rogueHit(advantaged, nil), edit: typedMarked(map[string]int32{"sneak-attack": 5}, "sneak-attack")},
		{name: "sneak attack with a sum out of range", hit: rogueHit(advantaged, nil), edit: typedMarked(map[string]int32{"weapon": 3, "sneak-attack": 99}, "sneak-attack")},
		{
			name: "sneak attack on a critical hit in the app", hit: rogueHit(critical, nil), edit: markedExtras("sneak-attack"),
			before: func(t *testing.T, a *armed, _ *playv1.Encounter, _ string) {
				t.Helper()
				a.h.roller.queue(3, 4, 2, 6, 1, 5)
			},
		},
		{name: "sneak attack on a critical hit with typed parts", hit: rogueHit(critical, nil), edit: typedMarked(map[string]int32{"weapon": 9, "sneak-attack": 15}, "sneak-attack")},
		{
			name: "sneak attack as the master", hit: rogueHit(advantaged, nil), as: asMasterUser, edit: markedExtras("sneak-attack"),
			before: func(t *testing.T, a *armed, _ *playv1.Encounter, _ string) { t.Helper(); a.h.roller.queue(2, 6, 6) },
		},
		{name: "sneak attack when everybody rolls real dice", hit: rogueHit(advantaged, nil), edit: markedExtras("sneak-attack"), before: physicalFor},
		{name: "sneak attack the hit does not allow", hit: rogueHit([]int32{18}, func(t *testing.T, a *armed, e *playv1.Encounter) {
			t.Helper()
			a.setConditions(t, e, "Toren", "condition:poisoned")
		}), edit: markedExtras("sneak-attack")},
		{
			name: "a barbarian's rage in the app", hit: rageHit, as: func(a *armed) *user { return a.bia },
			before: func(t *testing.T, a *armed, _ *playv1.Encounter, _ string) { t.Helper(); a.h.roller.queue(5) },
		},
		{
			name: "a barbarian's rage with typed parts", hit: rageHit, as: func(a *armed) *user { return a.bia },
			edit: typedMarked(map[string]int32{"weapon": 4, "rage": 2}),
		},
		{name: "a barbarian's rage with a typed sum", hit: rageHit, as: func(a *armed) *user { return a.bia }, edit: func(_ *armed, r *playv1.RollDamageRequest) { typedSum(4)(r) }},
		{
			name: "a hit on a raging barbarian", hit: rageTarget(18), as: asMasterUser,
			before: func(t *testing.T, a *armed, _ *playv1.Encounter, _ string) { t.Helper(); a.h.roller.queue(4) },
		},
		{
			name: "a critical hit on a raging barbarian", hit: rageTarget(20), as: asMasterUser,
			before: func(t *testing.T, a *armed, _ *playv1.Encounter, _ string) { t.Helper(); a.h.roller.queue(4, 5) },
		},
	}
	out := make([]damageScenario, 0, len(rows))
	for _, x := range rows {
		out = append(out, x.scenario())
	}
	return out
}

// smiteMark marks the paladin's Divine Smite with a slot level.
func smiteMark(level int32) func(a *armed, r *playv1.RollDamageRequest) {
	return func(_ *armed, r *playv1.RollDamageRequest) {
		r.ExtrasChosen = true
		r.SelectedExtras = []*playv1.SelectedExtra{{Key: "divine-smite", SlotLevel: level}}
	}
}

// smiteTyped types the sums of the weapon, the smite and its extra die, with the smite marked at a slot level.
func smiteTyped(level, weapon, smite, extra int32) func(a *armed, r *playv1.RollDamageRequest) {
	return func(a *armed, r *playv1.RollDamageRequest) {
		smiteMark(level)(a, r)
		r.Roll = nil
		r.TypedParts = []*playv1.TypedPart{{PartKey: "weapon", Sum: weapon}, {PartKey: "divine-smite", Sum: smite}, {PartKey: "divine-smite-extra", Sum: extra}}
	}
}

// smiteHit is Toren, a paladin, hitting a target (a natural 20 for a critical hit).
func smiteHit(target string, face int32) func(t *testing.T) (*armed, *playv1.Encounter, string) {
	return func(t *testing.T) (*armed, *playv1.Encounter, string) {
		t.Helper()
		a, e := smiteTable(t)
		res := a.mustAttack(t, a.caio, a.get(t, a.caio), "Toren", battleaxe, target, d20(face))
		if res.GetPendingDamage() == nil {
			t.Fatalf("RollAttack() = %v, want a hit", res.GetRoll())
		}
		return a, e, res.GetPendingDamage().GetId()
	}
}

// paladinDamages are the Divine Smites.
func paladinDamages() []damageScenario {
	queue := func(fs ...int) func(t *testing.T, a *armed, e *playv1.Encounter, id string) {
		return func(t *testing.T, a *armed, _ *playv1.Encounter, _ string) {
			t.Helper()
			a.h.roller.queue(fs...)
		}
	}
	rows := []extrasDamage{
		{name: "a smite on the undead with typed parts", hit: smiteHit("Esqueleto", 15), edit: smiteTyped(1, 4, 9, 3)},
		{name: "a smite on a human with typed parts", hit: smiteHit("Bandido", 15), edit: smiteTyped(1, 4, 9, 3)},
		{name: "a smite on the undead in the app", hit: smiteHit("Esqueleto", 15), edit: smiteMark(1), before: queue(4, 5, 6, 7)},
		{name: "a smite on a human in the app", hit: smiteHit("Bandido", 15), edit: smiteMark(1), before: queue(4, 5, 6, 7)},
		{name: "a smite on a critical hit with typed parts", hit: smiteHit("Esqueleto", 20), edit: smiteTyped(1, 9, 14, 5)},
		{name: "a smite on a critical hit in the app", hit: smiteHit("Esqueleto", 20), edit: smiteMark(1), before: queue(4, 5, 6, 7, 2, 3, 1, 8)},
		{name: "a smite with a slot the paladin does not have", hit: smiteHit("Esqueleto", 15), edit: smiteMark(2)},
		{name: "a smite with no slot named", hit: smiteHit("Esqueleto", 15), edit: smiteMark(0)},
		{name: "a smite with a sum out of range", hit: smiteHit("Esqueleto", 15), edit: smiteTyped(1, 4, 99, 3)},
		{name: "a smite with no sum for its extra die", hit: smiteHit("Esqueleto", 15), edit: func(a *armed, r *playv1.RollDamageRequest) {
			smiteTyped(1, 4, 9, 3)(a, r)
			r.TypedParts = r.TypedParts[:2]
		}},
		{name: "a smite as the master", hit: smiteHit("Esqueleto", 15), as: asMasterUser, edit: smiteMark(1), before: queue(4, 5, 6, 7)},
		{name: "a smite with the slots spent", hit: smiteHit("Esqueleto", 15), edit: smiteMark(1), before: func(t *testing.T, a *armed, _ *playv1.Encounter, _ string) {
			t.Helper()
			a.correct(t, a.toren, func(r *playv1.AdjustCharacterVitalsRequest) {
				r.SpellSlotsUsed = []*playv1.SpellSlotsUsed{{Level: 1, Used: 3}}
			})
		}},
	}
	out := make([]damageScenario, 0, len(rows))
	for _, x := range rows {
		out = append(out, x.scenario())
	}
	return out
}

// maneuverScenario plays Pensantus, a fighter with maneuvers, hitting the Capitão; edit changes the roll with
// the keys of the maneuvers.
func maneuverScenario(name, uses string, face int32, as func(a *armed) *user, queue []int, edit func(m *maneuverTable, r *playv1.RollDamageRequest), first ...func(m *maneuverTable) string) damageScenario {
	return damageOn(name, func(t *testing.T) damagePlay {
		t.Helper()
		m := newManeuverTable(t, uses)
		e := m.passTo(t, m.castersFight(t, 1), "Pensantus")
		res := m.mustAttack(t, m.ana, e, "Pensantus", longsword, "Capitão Goblin", d20(face))
		if res.GetPendingDamage() == nil {
			t.Fatalf("RollAttack() = %v, want a hit", res.GetRoll())
		}
		if len(first) > 0 { // a roll that spends the only use before the one the scenario reads
			m.h.roller.queue(5, 4)
			m.mustDamage(t, m.ana, e, res.GetPendingDamage().GetId(), marking(first[0](m)))
			res = m.mustAttack(t, m.ana, e, "Pensantus", longsword, "Capitão Goblin", d20(face)) // the second attack of Extra Attack
		}
		m.h.roller.queue(queue...)
		return m.rolled(t, as(m.armed), e, res.GetPendingDamage().GetId(), func(r *playv1.RollDamageRequest) {
			inAppDamage(r)
			edit(m, r)
		})
	})
}

func asAnaUser(a *armed) *user { return a.ana }

// maneuverDamages are the maneuvers added to a hit.
func maneuverDamages() []damageScenario {
	mark := func(pick func(m *maneuverTable) []string) func(m *maneuverTable, r *playv1.RollDamageRequest) {
		return func(m *maneuverTable, r *playv1.RollDamageRequest) {
			r.ExtrasChosen = true
			for _, k := range pick(m) {
				r.SelectedExtras = append(r.SelectedExtras, &playv1.SelectedExtra{Key: k})
			}
		}
	}
	heron := mark(func(m *maneuverTable) []string { return []string{m.heron} })
	return []damageScenario{
		maneuverScenario("a maneuver in the app", "4", 19, asAnaUser, []int{5, 4}, heron),
		maneuverScenario("a maneuver with typed parts", "4", 19, asAnaUser, nil, func(m *maneuverTable, r *playv1.RollDamageRequest) {
			heron(m, r)
			r.Roll = nil
			r.TypedParts = []*playv1.TypedPart{{PartKey: "weapon", Sum: 5}, {PartKey: m.heron, Sum: 4}}
		}),
		maneuverScenario("a maneuver as the master", "4", 19, asMasterUser, []int{5, 4}, heron),
		maneuverScenario("a maneuver on a critical hit", "4", 20, asAnaUser, []int{5, 4, 6, 2}, heron),
		maneuverScenario("two maneuvers for one attack", "4", 19, asAnaUser, nil, mark(func(m *maneuverTable) []string { return []string{m.heron, m.bull} })),
		maneuverScenario("extras chosen and no maneuver", "4", 19, asAnaUser, []int{5}, mark(func(*maneuverTable) []string { return nil })),
		maneuverScenario("a maneuver the grip is not", "4", 19, asAnaUser, nil, mark(func(m *maneuverTable) []string { return []string{m.grip} })),
		maneuverScenario("a maneuver with no use left", "1", 19, asAnaUser, nil, mark(func(m *maneuverTable) []string { return []string{m.bull} }),
			func(m *maneuverTable) string { return m.heron }),
	}
}

// skeletonDamage plays the skeleton's fight: Toren's mace, or Pensantus's Poison Spray, then the damage.
func skeletonDamage(name string, spray bool, as func(a *armed) *user, queue []int, edit func(*playv1.RollDamageRequest)) damageScenario {
	return damageOn(name, func(t *testing.T) damagePlay {
		t.Helper()
		a, e, skel := skeletonFight(t)
		var id string
		if spray {
			e = a.passTo(t, e, "Pensantus")
			a.h.roller.queue(1, 6, 6) // the skeleton fails its save; the damage dice of the cast
			res, err := a.cast(t, a.ana, e, "Pensantus", poisonSpray, nil, a.at(t, skel), noCastRoll)
			if err != nil {
				t.Fatalf("CastSpell(Poison Spray) error = %v", err)
			}
			id = res.GetCast().GetPendingDamages()[0].GetId()
		} else {
			res := a.mustAttack(t, a.caio, e, "Toren", maceKey, skel, d20(15))
			id = res.GetPendingDamage().GetId()
		}
		a.h.roller.queue(queue...)
		return a.rolled(t, as(a), e, id, edit)
	})
}

// skeletonDamages are the skeleton's vulnerability and immunity.
func skeletonDamages() []damageScenario {
	return []damageScenario{
		skeletonDamage("a mace on a skeleton, typed", false, asToren, nil, typedSum(3)),
		skeletonDamage("a mace on a skeleton, in the app", false, asToren, []int{4}, inAppDamage),
		skeletonDamage("a mace on a skeleton, as the master", false, asMasterUser, []int{4}, inAppDamage),
		skeletonDamage("poison spray on a skeleton, typed", true, asAnaUser, nil, typedSum(6)),
		skeletonDamage("poison spray on a skeleton, in the app", true, asAnaUser, []int{5, 6}, inAppDamage),
	}
}

// areaCast is Pensantus's Burning Hands over two goblins and the Capitão (the first saves, the second does not
// and the Capitão does not: Goblin 2 saves and takes half): the pending damages of the cast, in the order of
// its targets.
func areaCast(t *testing.T) (*armed, *playv1.Encounter, []string) {
	t.Helper()
	a := newCasters(t)
	e := a.castersFight(t, 2)
	a.put(t, "Goblin 1", 7, 5)
	a.put(t, "Capitão Goblin", 8, 6)
	a.h.roller.queue(5, 18, 12) // the saves of Goblin 1, Goblin 2 and the Capitão
	cast := a.mustCast(t, a.ana, e, "Pensantus", burningHands, slotOfLevel(1), a.at(t, "Goblin 1", "Goblin 2", "Capitão Goblin"), noCastRoll)
	var ids []string
	for _, p := range cast.GetCast().GetPendingDamages() {
		ids = append(ids, p.GetId())
	}
	return a, e, ids
}

// healFight is Brisa, on her turn, with a heal for Toren (who stands next to her) or the Capitão.
func healFight(t *testing.T, target string, hurt func(t *testing.T, a *armed, e *playv1.Encounter)) (*armed, *playv1.Encounter, string) {
	t.Helper()
	a := newCasters(t)
	e := a.castersFight(t, 1)
	e = a.passTo(t, e, "Brisa")
	a.put(t, "Capitão Goblin", 4, 6)
	if hurt != nil {
		hurt(t, a, e)
	}
	cast := a.mustCast(t, a.bia, e, "Brisa", cureWounds, slotOfLevel(1), a.at(t, target), noCastRoll)
	return a, e, cast.GetCast().GetPendingDamages()[0].GetId()
}

// spellDamages are the damage and the heals of spells.
func spellDamages() []damageScenario {
	area := func(name string, which int, as func(a *armed) *user, queue []int, edit func(*playv1.RollDamageRequest), also ...int) damageScenario {
		return damageOn(name, func(t *testing.T) damagePlay {
			t.Helper()
			a, e, ids := areaCast(t)
			a.h.roller.queue(queue...)
			for _, i := range also { // damages the cast settles before the one the scenario reads
				a.mustDamage(t, a.ana, e, ids[i], inAppDamage)
			}
			return a.rolled(t, as(a), e, ids[which], edit)
		})
	}
	heal := func(name, target string, hurt func(t *testing.T, a *armed, e *playv1.Encounter), queue []int, edit func(*playv1.RollDamageRequest)) damageScenario {
		return damageOn(name, func(t *testing.T) damagePlay {
			t.Helper()
			a, e, id := healFight(t, target, hurt)
			a.h.roller.queue(queue...)
			return a.rolled(t, a.bia, e, id, edit)
		})
	}
	down := func(t *testing.T, a *armed, _ *playv1.Encounter) {
		t.Helper()
		a.correct(t, a.toren, hpIs(0))
		a.execSQL(t, `UPDATE combatants SET death_failures = 2, death_successes = 1 WHERE id = $1`, a.id(t, "Toren"))
	}
	flame := func(name, target string, concentrates bool, as func(a *armed) *user, queues ...[]int) damageScenario {
		return damageOn(name, func(t *testing.T) damagePlay {
			t.Helper()
			a, e := flameStrikeFixture(t)
			if concentrates {
				a.concentrate(t, target, webSpell)
			}
			rolls := len(queues)
			fire, radiant := flameStrikeOn(t, a, e, target)
			ids := []string{fire, radiant}
			for i := range rolls - 1 {
				a.h.roller.queue(queues[i]...)
				a.mustDamage(t, as(a), e, ids[i], inAppDamage)
			}
			a.h.roller.queue(queues[rolls-1]...)
			return a.rolled(t, as(a), e, ids[rolls-1], inAppDamage)
		})
	}
	return []damageScenario{
		area("an area spell rolled by its caster", 0, asAnaUser, []int{3, 3, 3}, inAppDamage),
		area("an area spell rolled from its last target", 2, asAnaUser, []int{2, 5, 6}, inAppDamage),
		area("an area spell rolled from the target that saved", 1, asAnaUser, []int{4, 4, 5}, inAppDamage),
		area("an area spell rolled by the master", 0, asMasterUser, []int{3, 3, 3}, inAppDamage),
		area("an area spell with a typed sum", 0, asAnaUser, nil, typedSum(9)),
		area("an area spell with a typed sum out of range", 0, asAnaUser, nil, typedSum(19)),
		area("an area spell already settled", 1, asAnaUser, []int{3, 3, 3}, inAppDamage, 0),
		heal("a heal that brings Toren back from 0", "Toren", down, []int{4}, inAppDamage),
		heal("a heal for Toren with a typed sum", "Toren", down, nil, typedSum(5)),
		heal("a heal for Toren at full hit points", "Toren", nil, []int{4}, inAppDamage),
		heal("a heal for Toren who is hurt", "Toren", func(t *testing.T, a *armed, _ *playv1.Encounter) {
			t.Helper()
			a.correct(t, a.toren, hpIs(10))
		}, []int{4}, inAppDamage),
		heal("a heal for the Capitão who is hurt", "Capitão Goblin", func(t *testing.T, a *armed, e *playv1.Encounter) {
			t.Helper()
			if _, err := a.adjustHP(t, e, "Capitão Goblin", damageHP(12)); err != nil {
				t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
			}
		}, []int{4}, inAppDamage),
		heal("a heal for the Capitão at full hit points", "Capitão Goblin", nil, []int{4}, inAppDamage),
		flame("fire of Flame Strike on a concentrating NPC", "Capitão Goblin", true, asMasterUser, []int{5, 5, 5, 5}),
		flame("fire and radiant of Flame Strike on a concentrating NPC that falls", "Capitão Goblin", true, asMasterUser, []int{5, 5, 5, 5}, []int{2, 2, 2, 2}),
		flame("fire and radiant of Flame Strike on a concentrating NPC that stands", "Capitão Goblin", true, asMasterUser, []int{3, 3, 3, 3}, []int{2, 2, 2, 2}),
		flame("fire and radiant of Flame Strike on a concentrating character", "Toren", true, asMasterUser, []int{5, 5, 5, 5}, []int{2, 2, 2, 2}),
		flame("fire and radiant of Flame Strike on a character", "Toren", false, asMasterUser, []int{5, 5, 5, 5}, []int{2, 2, 2, 2}),
		flame("fire and radiant of Flame Strike rolled by the caster", "Capitão Goblin", false, func(a *armed) *user { return a.bia }, []int{3, 1, 4, 1}, []int{5, 9, 2, 6}),
	}
}

// held rolls the damage of the pending damage as u and then answers the window of kind, if answer is not nil:
// the report is read after the answer, with the events since before the roll.
func held(t *testing.T, a *armed, e *playv1.Encounter, u *user, id string, edit func(*playv1.RollDamageRequest), kind playv1.ReactionKind, reactor *user, answer func(*playv1.AnswerReactionRequest)) damagePlay {
	t.Helper()
	p := a.rolled(t, u, e, id, edit)
	if answer == nil || p.err != nil {
		return p
	}
	w := a.windowOf(t, reactor, kind)
	if w == nil {
		t.Fatalf("no %v window after the damage", kind)
	}
	a.mustAnswer(t, reactor, e, w.GetId(), answer)
	return p
}

func typedDamageReactionAnswer(n int32) func(*playv1.AnswerReactionRequest) {
	return func(r *playv1.AnswerReactionRequest) {
		r.Answer, r.Roll = playv1.ReactionChoice_REACTION_CHOICE_USE, &playv1.AnswerReactionRequest_Typed{Typed: n}
	}
}

// answerChecks passes the master's checks the table asks for, as many as are open.
func answerChecks(t *testing.T, a *armed, e *playv1.Encounter) {
	t.Helper()
	for range 4 {
		w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_MASTER_CHECK)
		if w == nil {
			return
		}
		a.mustAnswer(t, a.master, e, w.GetId(), passAnswer)
	}
}

// reactionDamages are the damage rolls that wait for a reaction and the ones that land when it is answered.
func reactionDamages() []damageScenario {
	uncanny := func(name string, answer func(*playv1.AnswerReactionRequest)) damageScenario {
		return damageOn(name, func(t *testing.T) damagePlay {
			t.Helper()
			a, e := reactorFight(t, rogueFive)
			hit := a.mustAttack(t, a.master, e, "Goblin", sword, "Reator", d20(15))
			return held(t, a, e, a.master, hit.GetPendingDamage().GetId(), typedSum(6), playv1.ReactionKind_REACTION_KIND_UNCANNY_DODGE, a.ana, answer)
		})
	}
	cutting := func(name string, answer func(*playv1.AnswerReactionRequest)) damageScenario {
		return damageOn(name, func(t *testing.T) damagePlay {
			t.Helper()
			a, e := reactorFight(t, bardAsking(charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL))
			a.mustAttack(t, a.master, e, "Goblin", sword, "Toren", d20(19))
			a.mustAnswer(t, a.ana, e, a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CUTTING_WORDS).GetId(), passAnswer)
			return held(t, a, e, a.master, a.firstPending(t), typedSum(4), playv1.ReactionKind_REACTION_KIND_CUTTING_WORDS, a.ana, answer)
		})
	}
	deflect := func(name string, answer func(*playv1.AnswerReactionRequest)) damageScenario {
		return damageOn(name, func(t *testing.T) damagePlay {
			t.Helper()
			a, e := reactorFight(t, monkThree)
			hit := a.mustAttack(t, a.master, e, "Goblin", shortBow, "Reator", disadvantage(15))
			return held(t, a, e, a.master, hit.GetPendingDamage().GetId(), typedSum(6), playv1.ReactionKind_REACTION_KIND_DEFLECT_MISSILES, a.ana, answer)
		})
	}
	always := func(name string, parts bool, answer func(*playv1.AnswerReactionRequest)) damageScenario {
		return damageOn(name, func(t *testing.T) damagePlay {
			t.Helper()
			var a *armed
			var e *playv1.Encounter
			if parts {
				a, e = rollsTable(t)
				e = a.passTo(t, e, "Toren")
				a.setConditions(t, e, "Goblin", "condition:restrained")
			} else {
				a = newCasters(t)
				e = a.threeAndAGoblin(t)
			}
			a.setRules(t, enemyReactionsAlways)
			var held *playv1.RollAttackResponse
			if parts {
				held = a.mustAttack(t, a.caio, a.get(t, a.caio), "Toren", rapier, "Goblin", func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{5, 18} })
			} else {
				held = a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", d20(15))
			}
			if !held.GetRoll().GetHeldForReaction() {
				t.Fatalf("RollAttack() = %v, want it held for the master's check", held.GetRoll())
			}
			answerChecks(t, a, e)
			edit := inAppDamage
			if parts {
				edit = func(r *playv1.RollDamageRequest) {
					inAppDamage(r)
					r.ExtrasChosen = true
					r.SelectedExtras = []*playv1.SelectedExtra{{Key: "sneak-attack"}}
				}
				a.h.roller.queue(3, 4, 2)
			} else {
				a.h.roller.queue(5)
			}
			return heldByMaster(t, a, e, a.caio, a.firstPending(t), edit, answer)
		})
	}
	return []damageScenario{
		uncanny("a damage held for Uncanny Dodge", nil),
		uncanny("a damage held for Uncanny Dodge, then halved", useAnswer(0)),
		uncanny("a damage held for Uncanny Dodge, then passed", passAnswer),
		cutting("a damage held for Cutting Words", nil),
		cutting("a damage held for Cutting Words, then cut", typedDamageReactionAnswer(3)),
		cutting("a damage held for Cutting Words, then passed", passAnswer),
		deflect("a damage held for Deflect Missiles", nil),
		deflect("a damage held for Deflect Missiles, then deflected", typedDamageReactionAnswer(8)),
		deflect("a damage held for Deflect Missiles, then passed", passAnswer),
		always("a damage held for the master's check", false, nil),
		always("a damage held for the master's check, then passed", false, passAnswer),
		always("a damage with Sneak Attack held for the master's check", true, nil),
		always("a damage with Sneak Attack held for the master's check, then passed", true, passAnswer),
		damageOn("a damage while the Mage may react", func(t *testing.T) damagePlay {
			t.Helper()
			a, e := mageFight(t)
			hit := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Mago", d20(9))
			return a.rolled(t, a.caio, e, hit.GetPendingDamage().GetId(), inAppDamage)
		}),
	}
}

// heldByMaster is held for the master's check window.
func heldByMaster(t *testing.T, a *armed, e *playv1.Encounter, u *user, id string, edit func(*playv1.RollDamageRequest), answer func(*playv1.AnswerReactionRequest)) damagePlay {
	t.Helper()
	return held(t, a, e, u, id, edit, playv1.ReactionKind_REACTION_KIND_MASTER_CHECK, a.master, answer)
}

// opportunityDamage is Toren's opportunity attack on Goblin 1, who leaves his reach, and the damage that drops
// it to 0 hit points: it goes back to where it left the reach, unless another combatant stands there.
func opportunityDamage(name string, taken bool, edit func(*playv1.RollDamageRequest)) damageScenario {
	return damageOn(name, func(t *testing.T) damagePlay {
		t.Helper()
		c := newCave(t)
		c.oppFight(t)
		c.passTo(t, c.get(t, c.master), "Goblin 1")
		c.moveOffering(t, "Goblin 1", 5, 9)
		offers := c.offersOf(t, c.caio)
		if len(offers) != 1 {
			t.Fatalf("Toren's player's offers = %v, want one", offers)
		}
		if taken {
			c.mustMove(t, c.master, "Goblin 2", offers[0].GetLeftCol(), offers[0].GetLeftRow())
		}
		res, err := c.offerAttack(t, c.caio, "Toren", offers[0].GetAttacks()[0].GetKey(), "Goblin 1", offers[0].GetId(), d20(15))
		if err != nil || res.GetPendingDamage() == nil {
			t.Fatalf("Toren's opportunity attack = (%v, %v), want a hit", res, err)
		}
		return c.rolled(t, c.caio, c.get(t, c.master), res.GetPendingDamage().GetId(), edit)
	})
}

// opportunityDamages are the opportunity attacks' damage.
func opportunityDamages() []damageScenario {
	return []damageScenario{
		opportunityDamage("an opportunity attack that sends the mover back", false, typedSum(8)),
		opportunityDamage("an opportunity attack that finds the square taken", true, typedSum(8)),
		opportunityDamage("an opportunity attack that leaves the mover standing", false, typedSum(1)),
	}
}
