package leaktest

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The material resistance of an NPC (RN-10, RN-20): a wight resists the piercing of a weapon that is not magical
// and not silvered. Caio's paladin hits it twice, with a plain dagger and with a shortbow his sheet marks as silvered
// (full.weapon_marks). The step that halves the plain hit, its label ("... de ataque não mágico nem prateado")
// and the amounts before and after are the master's alone; the attacker's player and a third player read the damage as it was rolled,
// in the roll, in the combat, in the log and in the stream, with none of the words that say what the monster resists
// or what the weapon is made of. The master's own lines are the positive controls. The sheet with the weapon marks is
// read by the owner and the master only (GetCharacter), the list gives a summary without it: the marks need no row of their own.

const wightKey = "monster:wight"

// materialNeedles are the words a player must never read: the qualifier of the resistance, and the materials.
var materialNeedles = []string{"prateado", "adamantino", "não mágico", "nao magico", "nonmagical", "silvered", "adamantine"}

func TestAMaterialResistanceIsTheMastersAlone(t *testing.T) {
	var silverBow string
	rt := newRollsTableWith(t, func(rt *rollsTable) {
		w, ctx := rt.world, rt.t.Context()
		toren := w.toren
		sheet := toren.GetSheet().GetFull()
		sheet.WeaponMarks = []*charactersv1.WeaponMark{{WeaponKey: shortbow, Silvered: true}}
		must(w.caio.characters.UpdateCharacter(ctx, rq(&charactersv1.UpdateCharacterRequest{
			CampaignId: w.campaign, CharacterId: toren.GetId(), Revision: toren.GetRevision(), Name: toren.GetName(),
			Sheet: &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: sheet}},
		})))
		silverBow = shortbow
	})
	w, m, caio, ana := rt.world, rt.master, rt.caio, rt.ana
	rt.buildRows()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("the table: %v", r)
		}
	}()
	ctx := t.Context()
	// The words the master's lines carry are canaries too (the master's reads are their positive controls); the others are
	// only scanned for, with all of them, in the payloads below.
	for _, n := range []string{"prateado", "não mágico"} {
		w.secrets.add(&canary{needle: n, kind: "material-resistance"})
	}

	// The wight joins the combat in plain sight, next to the paladin, and the skeleton is out of the way of the bow.
	rt.encounter = rt.encounterOf(m)
	name := w.secrets.public("vis-Carnical")
	hidden := false
	res := must(m.combat.AddMonsters(ctx, rq(&playv1.AddMonstersRequest{
		CampaignId: w.campaign, EncounterId: rt.encounter.GetId(), IdempotencyKey: newKey(), CreatureKey: wightKey, Count: 1, Name: name, Hidden: &hidden,
	})))
	rt.ids["Carnical"] = res.GetCombatantIds()[0]
	for label, sq := range map[string][2]int32{"Carnical": {4, 4}, "Esqueleto": {8, 13}} {
		must(m.combat.MoveCombatant(ctx, rq(&playv1.MoveCombatantRequest{CampaignId: w.campaign, EncounterId: rt.encounter.GetId(), CombatantId: rt.ids[label], IdempotencyKey: newKey(), Col: sq[0], Row: sq[1], Forced: true})))
	}
	w.secrets.add(&canary{needle: wightKey, kind: "monster-key"})
	watchers := map[*person]*streamWatcher{m: w.watchStream(m), ana: w.watchStream(ana), caio: w.watchStream(caio)}

	rt.toTurn(labelToren)
	rt.ask(t, "the wight is in the combat")

	// 1. The plain dagger: the wight halves the 9 (4 and the modifier 5) to 4.
	plainHit := rt.mustAttack(caio, labelToren, dagger, "Carnical", []int32{18}, nil)
	if plainHit.GetPendingDamage() == nil {
		t.Fatalf("the dagger on the wight = %v, want a hit", plainHit.GetRoll())
	}
	plain := rt.damage(caio, plainHit.GetPendingDamage().GetId(), func(r *playv1.RollDamageRequest) {
		r.ExtrasChosen, r.Roll = true, &playv1.RollDamageRequest_TypedSum{TypedSum: 4}
	}).GetPendingDamage()
	rt.ask(t, "the plain dagger hit the wight")
	step := rt.checkMaterialStep(t, plain, dagger, true)
	if step.GetBefore() != 9 || step.GetAfter() != 4 {
		t.Errorf("the master's step takes the damage from %d to %d, want 9 to 4", step.GetBefore(), step.GetAfter())
	}
	// The master's log line names the reason; the player's line for the same damage has no step and no reason.
	masterLine := rt.hitLine(m, "Carnical", dagger)
	label := masterLine.GetDamage().GetSteps()[0].GetLabelPt()
	if !strings.Contains(label, "prateado") {
		t.Fatalf("the master's log line gives the step %q, want the silvered qualifier in its label (the positive control)", label)
	}
	for _, p := range []*person{ana, caio} {
		line := rt.hitLine(p, "Carnical", dagger)
		if line == nil || line.GetDamage().GetAmount() != 9 || len(line.GetDamage().GetSteps()) != 0 {
			t.Errorf("%s reads the log line %v, want the 9 that was rolled and no step", p.name, line)
		}
		rt.expectNoWords(t, p, "the log", rt.logOf(p))
	}
	rt.endTurn(caio)

	// 2. The silvered bow: no step on the master's line either, and the amounts are as rolled.
	rt.toTurn(labelToren)
	shotHit, err := rt.attack(caio, labelToren, silverBow, "Carnical", []int32{18, 17}, nil)
	if err != nil {
		t.Fatalf("the shot on the wight: %v", err)
	}
	if shotHit.GetPendingDamage() == nil {
		t.Fatalf("the shot on the wight = %v, want a hit", shotHit.GetRoll())
	}
	shot := rt.damage(caio, shotHit.GetPendingDamage().GetId(), func(r *playv1.RollDamageRequest) {
		r.ExtrasChosen, r.Roll = true, &playv1.RollDamageRequest_TypedSum{TypedSum: 4}
	}).GetPendingDamage()
	rt.ask(t, "the silvered shot hit the wight")
	if line := rt.hitLine(m, "Carnical", silverBow); line == nil || len(line.GetDamage().GetSteps()) != 0 || line.GetDamage().GetAmount() != 6 {
		t.Errorf("the master reads %v on the silvered shot, want the 6 as rolled and no step (the wight does not resist it)", line)
	}
	rt.checkMaterialStep(t, shot, silverBow, false)
	rt.endTurn(caio)

	// The end: the stream told every player of both hits, and carried none of the words.
	must(m.combat.EndEncounter(ctx, rq(&playv1.EndEncounterRequest{CampaignId: w.campaign, EncounterId: rt.encounter.GetId(), IdempotencyKey: newKey()})))
	must(m.play.EndGameSession(ctx, rq(&playv1.EndGameSessionRequest{CampaignId: w.campaign, GameSessionId: w.session})))
	for p, sw := range watchers {
		<-sw.done
		if len(sw.events) < 2 {
			t.Errorf("%s's stream heard %d events of all this, so it proves nothing", p.name, len(sw.events))
		}
		for _, ev := range sw.events {
			body, err := protojson.Marshal(ev)
			if err != nil {
				t.Fatalf("marshal an event: %v", err)
			}
			r := reply{status: 200, body: body, msg: ev}
			rt.got.keep(p, r)
			if p == m {
				continue
			}
			for _, f := range w.inspect(p, r, nil) {
				t.Errorf("%s: event %s: %s\n\tevent: %s", p.name, eventCase(ev), f, shorten(body))
			}
			rt.expectNoWordsIn(t, p, "an event of the stream", string(body))
		}
	}
	t.Run("canaries are reachable", func(t *testing.T) { checkCanariesAreReachable(t, w, rt.got) })
}

// checkMaterialStep proves what the master and the players read of the damage on the wight. With resisted, the master reads
// one resistance step with the qualifier; without, no step. The players read the damage as rolled either way.
func (rt *rollsTable) checkMaterialStep(t *testing.T, rolled *playv1.PendingDamage, weapon string, resisted bool) *playv1.DamageStep {
	t.Helper()
	line := rt.hitLine(rt.master, "Carnical", weapon)
	if line == nil {
		t.Fatalf("the master reads no attack line of %s on the wight", weapon)
	}
	var step *playv1.DamageStep
	if resisted {
		if len(line.GetDamage().GetSteps()) != 1 {
			t.Fatalf("the master reads the steps %v, want the resistance of the wight (the positive control)", line.GetDamage().GetSteps())
		}
		step = line.GetDamage().GetSteps()[0]
		if step.GetKind() != playv1.DamageStepKind_DAMAGE_STEP_KIND_RESISTANCE || step.GetAfter() >= step.GetBefore() || !strings.Contains(step.GetLabelPt(), "não mágico nem prateado") {
			t.Fatalf("the master's step = %v, want the wight's resistance with the silvered qualifier in its label", step)
		}
	}
	if len(rolled.GetSteps()) != 0 || rolled.AmountAfterSteps != nil {
		t.Errorf("the roll answers the player with steps %v and %v after steps, want the damage as rolled", rolled.GetSteps(), rolled.AmountAfterSteps)
	}
	for _, p := range []*person{rt.ana, rt.caio} {
		walk(rt.encounterOf(p).ProtoReflect(), "Encounter", func(path string, _ protoreflect.Message, fd protoreflect.FieldDescriptor, _ protoreflect.Value) {
			if fd.Name() == "steps" || fd.Name() == "amount_after_steps" {
				t.Errorf("%s reads %s in the combat", p.name, path)
			}
		})
		rt.expectNoWords(t, p, "the combat", rt.encounterOf(p))
		rt.expectNoWords(t, p, "the roll", rolled)
	}
	return step
}

// hitLine is the attack line with damage of the weapon on the target, as p reads the log.
func (rt *rollsTable) hitLine(p *person, target, weapon string) *playv1.CombatLogEntry {
	for _, en := range rt.attackLines(p, target) {
		if en.GetDamage() != nil && en.GetKey() == weapon {
			return en
		}
	}
	return nil
}

func (rt *rollsTable) expectNoWords(t *testing.T, p *person, what string, msg proto.Message) {
	t.Helper()
	body, err := protojson.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal %s: %v", what, err)
	}
	rt.expectNoWordsIn(t, p, what, string(body))
}

func (rt *rollsTable) expectNoWordsIn(t *testing.T, p *person, what, body string) {
	t.Helper()
	lower := strings.ToLower(body)
	for _, n := range materialNeedles {
		if strings.Contains(lower, strings.ToLower(n)) {
			t.Errorf("%s reads %q in %s", p.name, n, what)
		}
	}
}
