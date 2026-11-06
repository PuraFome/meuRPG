package leaktest

import (
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The table's roll_minimum (#757): the number a d20 of an attack or a die of damage counted as, and the feature that did
// it, go with the face of the die. They are the roller's player's and the master's: another player reads nothing of the
// roll, so nothing of the minimum either, in the combat, in the log, in the answers or in the live stream. The roller and
// the master are the positive controls (checkCanariesAreReachable finds each canary in their answers).
//
// The world is the attack rolls': Toren, the paladin of Caio, holds the table feat that gives both minimums.

const (
	// minimumAttack is the number an attack d20 of 2 counts as; minimumDamage the number the dice of a Divine Smite count as.
	minimumAttack = 12
	minimumDamage = 7
	// Numbers that turn up all over an answer are searched in the fields that name them.
	kindMinimumSrc    = "minimum-source"
	kindMinimumD20    = "minimum-d20-counted"
	kindMinimumDamage = "minimum-damage-counted"
)

// minimumFeat gives Toren the table feat with a minimum for the attack d20 and one for the dice of damage, and returns its key.
func (rt *rollsTable) minimumFeat() string {
	ctx := rt.t.Context()
	entry := must(rt.master.table.CreateTableEntry(ctx, rq(&rulesv1.CreateTableEntryRequest{
		CampaignId: rt.campaign, Body: &rulesv1.CreateTableEntryRequest_TableFeat{TableFeat: &rulesv1.TableFeat{
			NamePt: "Piso da Mesa", DescPt: []string{"Um talento da mesa."},
			Effects: []*rulesv1.TableEffect{
				{Type: "roll_minimum", Rolls: []string{"attack"}, Value: "12"},
				{Type: "roll_minimum", Rolls: []string{"damage"}, Value: "7"},
			},
		}},
	}))).GetEntry()
	sheet := rt.toren.GetSheet()
	sheet.GetFull().FeatKeys = append(sheet.GetFull().GetFeatKeys(), entry.GetKey())
	rt.toren = must(rt.master.characters.UpdateCharacter(ctx, rq(&charactersv1.UpdateCharacterRequest{
		CampaignId: rt.campaign, CharacterId: rt.toren.GetId(), Revision: rt.toren.GetRevision(), Name: rt.toren.GetName(), Sheet: sheet,
	}))).GetCharacter()
	return entry.GetKey()
}

func TestRollMinimumReachesOnlyWhoMayReadIt(t *testing.T) {
	var feat string
	rt := newRollsTableOf(t, true, true, func(rt *rollsTable) { feat = rt.minimumFeat() })
	w, m, caio, ana := rt.world, rt.master, rt.caio, rt.ana
	rt.buildRows()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("the table: %v", r)
		}
	}()
	w.secrets.add(&canary{needle: feat, kind: kindMinimumSrc, readers: names([]*person{caio})})
	w.secrets.add(&canary{needle: "12", kind: kindMinimumD20, numeric: true, value: minimumAttack, fields: []string{"treated_as"}, readers: names([]*person{caio})})
	w.secrets.add(&canary{needle: "7", kind: kindMinimumDamage, numeric: true, value: minimumDamage, fields: []string{"counted_as"}, readers: names([]*person{caio})})
	for _, label := range []string{labelToren, labelPens, "Esqueleto", "Arbusto", "Bandido", "Lobo"} {
		w.secrets.id("minimum-combatant", rt.ids[label], ana, caio)
	}
	watchers := map[*person]*streamWatcher{m: w.watchStream(m), ana: w.watchStream(ana), caio: w.watchStream(caio)}
	rt.toTurn(labelToren)
	rt.ask(t, "the combat begins")

	// 1. Toren's attack shows a 2, which counts as 12 and hits the skeleton: the d20 counted is the roller's and the master's.
	hit := rt.mustAttack(caio, labelToren, dagger, "Esqueleto", []int32{2}, nil)
	roll := hit.GetRoll().GetD20()
	if hit.GetPendingDamage() == nil || roll.GetTreatedAs() != minimumAttack || roll.GetTreatedAsSource() != feat {
		t.Fatalf("Toren's attack = %v, want the 2 counted as %d by the feat, and a hit", hit.GetRoll(), minimumAttack)
	}
	rt.ask(t, "the attack counted the minimum")
	for who, p := range map[string]*person{"the master": m, "Caio": caio} {
		lines := rt.attackLines(p, "Esqueleto")
		if len(lines) != 1 || lines[0].GetAttackRoll().GetTreatedAs() != minimumAttack || lines[0].GetAttackRoll().GetTreatedAsSource() != feat {
			t.Errorf("%s reads %v, want the d20 counted as %d by the feat in the log (the positive control)", who, lines, minimumAttack)
		}
	}
	for _, en := range rt.attackLines(ana, "Esqueleto") {
		if en.GetAttackRoll() != nil && (en.GetAttackRoll().TreatedAs != nil || en.GetAttackRoll().GetTreatedAsSource() != "") {
			t.Errorf("Ana reads the d20 counted as %v by %q, want nothing of another player's roll", en.GetAttackRoll().GetTreatedAs(), en.GetAttackRoll().GetTreatedAsSource())
		}
	}

	// 2. The damage is a Divine Smite: the weapon's d4 shows a 1 and counts as its highest face, the smite's d8 dice and the
	// extra die (against an undead, so it counts) show 1 and count as the minimum.
	w.dice.ScriptFor(t, 4, 1)
	w.dice.ScriptFor(t, 8, 1, 1, 1)
	rt.damage(caio, hit.GetPendingDamage().GetId(), func(r *playv1.RollDamageRequest) {
		r.ExtrasChosen = true
		r.SelectedExtras = []*playv1.SelectedExtra{{Key: "divine-smite", SlotLevel: 1}}
		r.Roll = &playv1.RollDamageRequest_RollInApp{RollInApp: true}
	})
	rt.ask(t, "the damage counted the minimum")
	for who, p := range map[string]*person{"the master": m, "Caio": caio} {
		lines := rt.attackLines(p, "Esqueleto")
		var smite *playv1.DamagePartRoll
		if len(lines) == 1 {
			for _, pr := range lines[0].GetDamage().GetParts() {
				if pr.GetPartKey() == "divine-smite" {
					smite = pr
				}
			}
		}
		if smite == nil || len(smite.GetCountedDice()) != 2 || smite.GetCountedDice()[0].GetCountedAs() != minimumDamage || smite.GetCountedDice()[0].GetSourceKey() != feat {
			t.Errorf("%s reads the smite %v, want its dice counted as %d by the feat in the log (the positive control)", who, smite, minimumDamage)
		}
	}
	for _, en := range rt.attackLines(ana, "Esqueleto") {
		if len(en.GetDamage().GetParts()) != 0 {
			t.Errorf("Ana reads the parts %v, want nothing of another player's damage", partLines(en))
		}
	}

	// The end: the stream told the master and the roller of the steps, and carried none of the minimum to Ana.
	must(m.combat.EndEncounter(t.Context(), rq(&playv1.EndEncounterRequest{CampaignId: w.campaign, EncounterId: rt.encounter.GetId(), IdempotencyKey: newKey()})))
	must(m.play.EndGameSession(t.Context(), rq(&playv1.EndGameSessionRequest{CampaignId: w.campaign, GameSessionId: w.session})))
	for p, sw := range watchers {
		select {
		case <-sw.done:
		case <-time.After(30 * time.Second):
			t.Fatalf("%s's stream did not end with the session", p.name)
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
		}
	}
	t.Run("canaries are reachable", func(t *testing.T) { checkCanariesAreReachable(t, w, rt.got) })
}
