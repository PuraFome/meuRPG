package leaktest

import (
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1/charactersv1connect"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1/rulesv1connect"
)

// A creature whose numbers are formulas of its owner ATTACKS (#578). Its to-hit and its damage bonus are the numbers
// the snapshot kept when it was made (Pensantus, level 3 and proficiency +2: +29 to hit and +28 to damage), and
// they are the creature's stat block: the attack's owner and the master read them; Caio, a third player, reads
// none of them, not in the combat, its log, the question of a roll that waits, nor the stream. The numbers are
// searched in the fields that carry a bonus; the positive controls show them in the answers of Ana and the master.

const (
	creatureToHit     = 29 // 27 + prof()
	creatureDamageNum = 28 // 25 + level()
	biterLabel        = "Cão Fiel"
	biterHitFace      = 14 // against the skeleton's armor class of 13
	biterDamageFace   = 3
)

func TestAFormulaCreatureThatAttacksKeepsItsNumbersFromAThirdPlayer(t *testing.T) {
	var key string
	rt := newRollsTableWith(t, func(rt *rollsTable) {
		w, ctx := rt.world, rt.t.Context()
		entry := must(w.master.table.CreateTableEntry(ctx, rq(&rulesv1.CreateTableEntryRequest{
			CampaignId: w.campaign, Body: &rulesv1.CreateTableEntryRequest_TableCreature{TableCreature: &rulesv1.TableCreature{
				NamePt: "Cão de Prata", BaseKey: "monster:giant-crab",
				Attacks: []*rulesv1.TableCreatureAttack{{
					NamePt: "Garra de Prata", Melee: true, DiceCount: 1, DiceSides: 6, DamageType: "damage-type:slashing",
					ToHit: "27 + prof()", DamageBonus: "25 + level()",
				}},
			}},
		}))).GetEntry()
		key = entry.GetKey()
		must(w.master.characters.GiveCreature(ctx, rq(&charactersv1.GiveCreatureRequest{
			CampaignId: w.campaign, CharacterId: w.pens.GetId(), MonsterKey: key, Name: biterLabel,
		})))
	})
	w, m, caio, ana := rt.world, rt.master, rt.caio, rt.ana
	ctx := t.Context()
	rt.buildRows()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("the table: %v", r)
		}
	}()
	attackKey := key + "#garra-de-prata"
	bonusFields := []string{"modifier", "bonus", "to_hit", "damage"}
	w.secrets.number("formula-creature-to-hit", creatureToHit, bonusFields...)
	w.secrets.number("formula-creature-damage-bonus", creatureDamageNum, bonusFields...)
	w.secrets.allowNeedle("29", ana)
	w.secrets.allowNeedle("28", ana)
	w.secrets.add(&canary{needle: attackKey, kind: "formula-creature-attack-key", readers: names([]*person{ana, caio})}) // the log names the attack, as it names an SRD one
	w.secrets.id("formula-creature-key", key, ana, caio)
	w.secrets.id("formula-creature-combatant", rt.ids[biterLabel], ana, caio)
	rt.rows = append(rt.rows,
		read{
			procedure: playv1connect.CombatServiceGetTurnOptionsProcedure, label: "the formula creature", allow: onlyAna, why: "the options of a creature are its owner's and the master's",
			req: func(w *world) proto.Message {
				return &playv1.GetTurnOptionsRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: rt.ids[biterLabel]}
			},
		},
		read{
			procedure: charactersv1connect.CharacterServiceListCharacterCreaturesProcedure, allow: onlyAna,
			req: func(w *world) proto.Message {
				return &charactersv1.ListCharacterCreaturesRequest{CampaignId: w.campaign, CharacterId: w.pens.GetId()}
			},
		},
		read{
			procedure: rulesv1connect.ContentServiceGetCreatureProcedure, label: "the formula creature", allow: onlyAna, why: "a table creature is not found by a player who holds none",
			req: func(w *world) proto.Message { return &rulesv1.GetCreatureRequest{CampaignId: w.campaign, Key: key} },
		},
		read{
			procedure: playv1connect.ResourceServiceListHeldRollsProcedure, allow: members,
			req: func(w *world) proto.Message { return &playv1.ListHeldRollsRequest{CampaignId: w.campaign} },
		},
	)
	// The creature stands next to the skeleton.
	must(m.combat.MoveCombatant(ctx, rq(&playv1.MoveCombatantRequest{
		CampaignId: w.campaign, EncounterId: rt.encounter.GetId(), CombatantId: rt.ids[biterLabel], IdempotencyKey: newKey(), Col: 5, Row: 5, Forced: true,
	})))
	watchers := map[*person]*streamWatcher{m: w.watchStream(m), ana: w.watchStream(ana), caio: w.watchStream(caio)}
	rt.toTurn(biterLabel)
	rt.ask(t, "the creature's turn")

	// 1. Ana rolls the creature's attack: the roll and the damage are worked out with the snapshot's numbers.
	hit := rt.mustAttack(ana, biterLabel, attackKey, "Esqueleto", []int32{biterHitFace}, nil)
	if hit.GetPendingDamage() == nil || hit.GetRoll().GetD20().GetModifier() != creatureToHit || hit.GetRoll().GetD20().GetTotal() != biterHitFace+creatureToHit {
		t.Fatalf("the creature's attack = %v, want a hit with +%d (the snapshot's)", hit.GetRoll(), creatureToHit)
	}
	rt.ask(t, "the creature hit")
	dmg := rt.damage(ana, hit.GetPendingDamage().GetId(), func(r *playv1.RollDamageRequest) {
		r.ExtrasChosen, r.Roll = true, &playv1.RollDamageRequest_TypedSum{TypedSum: biterDamageFace}
	}).GetPendingDamage()
	if dmg.GetAmount() != biterDamageFace+creatureDamageNum {
		t.Errorf("the creature's damage = %d, want %d (the dice and the snapshot's +%d)", dmg.GetAmount(), biterDamageFace+creatureDamageNum, creatureDamageNum)
	}
	rt.ask(t, "the creature's damage")
	rt.endTurn(ana)

	// 2. The creature rolls holding a die: the question carries its to-hit, and is Ana's and the master's alone.
	rt.toTurn(biterLabel)
	rt.giveDie(biterLabel)
	held := rt.mustAttack(ana, biterLabel, attackKey, "Lobo", []int32{biterHitFace}, nil)
	offer := held.GetHeldRollOffer()
	if offer == nil || offer.GetKind() != playv1.HeldRollKind_HELD_ROLL_KIND_WEAPON_ATTACK || offer.GetD20().GetModifier() != creatureToHit {
		t.Fatalf("the creature's attack with a die = %v, want the question of a weapon attack with +%d", held, creatureToHit)
	}
	w.secrets.id("formula-creature-held-roll", offer.GetHoldId(), ana)
	rt.ask(t, "the creature's attack waits")
	for who, p := range map[string]*person{"the master": m, "Ana": ana} {
		if got := rt.listHeld(p); len(got) != 1 || got[0].GetHoldId() != offer.GetHoldId() {
			t.Errorf("%s lists %v, want the creature's question (the positive control)", who, got)
		}
	}
	if got := rt.listHeld(caio); len(got) != 0 {
		t.Errorf("Caio lists %v, want nothing", got)
	}
	rt.clock.advance(11 * time.Second)
	must(ana.resources.AnswerHeldRoll(ctx, rq(&playv1.AnswerHeldRollRequest{
		CampaignId: w.campaign, HoldId: offer.GetHoldId(), IdempotencyKey: newKey(),
		Answer: &playv1.AnswerHeldRollRequest_Die{Die: &playv1.DieAnswer{Use: false}},
	})))
	rt.ask(t, "the creature's attack was settled")

	// The end: the stream told every player of every step, and carried none of the numbers.
	must(m.combat.EndEncounter(ctx, rq(&playv1.EndEncounterRequest{CampaignId: w.campaign, EncounterId: rt.encounter.GetId(), IdempotencyKey: newKey()})))
	must(m.play.EndGameSession(ctx, rq(&playv1.EndGameSessionRequest{CampaignId: w.campaign, GameSessionId: w.session})))
	for p, sw := range watchers {
		select {
		case <-sw.done:
		case <-time.After(30 * time.Second):
			t.Fatalf("%s's stream did not end with the session", p.name)
		}
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
		}
	}
	// The positive controls: the master and Ana read both numbers in their answers (the options of the creature), Caio neither.
	for _, kind := range []string{"formula-creature-to-hit", "formula-creature-damage-bonus"} {
		c := w.canaryOf(kind)
		for _, p := range []*person{m, ana} {
			if !rt.got.holds(p, c) {
				t.Errorf("%s reads %s in no answer: the control is missing", p.name, kind)
			}
		}
	}
	t.Run("canaries are reachable", func(t *testing.T) { checkCanariesAreReachable(t, w, rt.got) })
}
