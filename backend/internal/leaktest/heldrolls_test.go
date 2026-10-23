package leaktest

import (
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
)

// The rolls that wait for a question (RN-10, RN-35): the hold of a player's roll, with its id, is for the roller's
// player and for the master; the hold of an NPC is the master's alone; another player reads nothing of either, not
// the id, not that a roll waits, in the combat, in the list of the questions, in the answer to a try at answering it or
// in the live stream. The master reads every one (the positive controls of the ids).
//
// The world is the attack rolls' (a fog map, two players, hidden NPCs): Toren, the paladin of Caio, rolls an attack
// holding a Bardic Inspiration die; a goblin the master hides rolls one at Ana's barbarian holding one too.

// giveDie puts a d8 on the combatant, as the bard's inspiration would.
func (rt *rollsTable) giveDie(label string) {
	round := rt.encounterOf(rt.master).GetRound()
	if _, err := rt.pool.Exec(rt.t.Context(), `UPDATE combatants SET inspiration_sides = 8, inspiration_expires_round = $2 WHERE id = $1`, rt.ids[label], round+50); err != nil {
		panic(err)
	}
}

// listHeld is the questions p may answer.
func (rt *rollsTable) listHeld(p *person) []*playv1.HeldRollOffer {
	return must(p.resources.ListHeldRolls(rt.t.Context(), rq(&playv1.ListHeldRollsRequest{CampaignId: rt.campaign}))).GetOffers()
}

func TestHeldRollsReachOnlyWhoMayReadThem(t *testing.T) {
	rt := newRollsTable(t)
	w, m, caio, ana := rt.world, rt.master, rt.caio, rt.ana
	rt.buildRows()
	rt.rows = append(rt.rows, read{
		procedure: playv1connect.ResourceServiceListHeldRollsProcedure, allow: members,
		req: func(w *world) proto.Message { return &playv1.ListHeldRollsRequest{CampaignId: w.campaign} },
	})
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("the table: %v", r)
		}
	}()
	for _, label := range []string{labelToren, labelPens, "Esqueleto", "Arbusto", "Bandido", "Lobo"} {
		w.secrets.id("held-combatant", rt.ids[label], ana, caio)
	}
	watchers := map[*person]*streamWatcher{m: w.watchStream(m), ana: w.watchStream(ana), caio: w.watchStream(caio)}
	rt.toTurn(labelToren)
	rt.ask(t, "the combat begins")

	// 1. Toren rolls with a die in hand: the question is Caio's and the master's.
	rt.giveDie(labelToren)
	held := rt.mustAttack(caio, labelToren, dagger, "Esqueleto", []int32{14}, nil)
	offer := held.GetHeldRollOffer()
	if offer == nil || offer.GetKind() != playv1.HeldRollKind_HELD_ROLL_KIND_WEAPON_ATTACK || offer.GetDie() == nil || offer.GetCombatantId() != rt.ids[labelToren] {
		t.Fatalf("Toren's attack = %v, want the die's question of his weapon attack", held)
	}
	toren := w.secrets.id("held-roll-toren", offer.GetHoldId(), caio)
	rt.ask(t, "Toren's attack waits")
	for who, p := range map[string]*person{"the master": m, "Caio": caio} {
		if got := rt.combatantOf(p, labelToren).GetHeldRollOffer(); got.GetHoldId() != toren {
			t.Errorf("%s reads %v on Toren, want his question (the positive control)", who, got)
		}
		if got := rt.listHeld(p); len(got) != 1 || got[0].GetHoldId() != toren {
			t.Errorf("%s lists %v, want Toren's question", who, got)
		}
	}
	if got := rt.combatantOf(ana, labelToren).GetHeldRollOffer(); got != nil {
		t.Errorf("Ana reads %v on Toren, want nothing", got)
	}
	if got := rt.listHeld(ana); len(got) != 0 {
		t.Errorf("Ana lists %v, want nothing", got)
	}
	// A try at answering it says nothing but that there is no such hold.
	_, err := ana.resources.AnswerHeldRoll(t.Context(), rq(&playv1.AnswerHeldRollRequest{
		CampaignId: w.campaign, HoldId: toren, IdempotencyKey: newKey(),
		Answer: &playv1.AnswerHeldRollRequest_Die{Die: &playv1.DieAnswer{Use: false}},
	}))
	if connect.CodeOf(err) != connect.CodeNotFound || strings.Contains(err.Error(), toren) {
		t.Errorf("Ana's answer = %v, want not_found without the id", err)
	}
	rt.ask(t, "Ana tried to answer")

	// 2. The master answers Toren's question for him, on his turn, once the player's own chance has passed: the attack is
	// settled, and nothing of it reaches Ana.
	rt.clock.advance(11 * time.Second)
	res := must(m.resources.AnswerHeldRoll(t.Context(), rq(&playv1.AnswerHeldRollRequest{
		CampaignId: w.campaign, HoldId: toren, IdempotencyKey: newKey(),
		Answer: &playv1.AnswerHeldRollRequest_Die{Die: &playv1.DieAnswer{Use: false}},
	})))
	if res.GetAttack().GetRoll().GetD20().GetFaces()[0] != 14 || res.GetNext() != nil {
		t.Errorf("the master's answer = %v, want Toren's attack settled on the d20 of 14", res)
	}
	rt.ask(t, "the master answered for Toren")
	if got := rt.listHeld(caio); len(got) != 0 {
		t.Errorf("Caio lists %v after the answer, want nothing", got)
	}

	// 3. A goblin the master hides rolls at Ana's barbarian with a die: the question is the master's alone.
	rt.toTurn("EscondidoB")
	rt.giveDie("EscondidoB")
	npc := rt.mustAttack(m, "EscondidoB", "basic:0", labelPens, []int32{13}, nil)
	npcOffer := npc.GetHeldRollOffer()
	if npcOffer == nil || npcOffer.GetCharacterId() != "" || npcOffer.GetCombatantId() != rt.ids["EscondidoB"] {
		t.Fatalf("the goblin's attack = %v, want its question, with no character id", npc)
	}
	hidden := w.secrets.id("held-roll-hidden-npc", npcOffer.GetHoldId())
	rt.ask(t, "a hidden goblin's attack waits")
	if got := rt.listHeld(m); len(got) != 1 || got[0].GetHoldId() != hidden {
		t.Errorf("the master lists %v, want the goblin's question (the positive control)", got)
	}
	for who, p := range map[string]*person{"Ana": ana, "Caio": caio} {
		for _, o := range rt.listHeld(p) {
			if o.GetHoldId() == hidden {
				t.Errorf("%s lists the hidden goblin's question %v", who, o)
			}
		}
		if got := rt.combatantOf(p, "EscondidoB"); got != nil {
			t.Errorf("%s reads the hidden goblin %v in the combat", who, got)
		}
	}

	// The end: the stream told the master and the rollers of every step, and carried no question.
	must(m.combat.EndEncounter(t.Context(), rq(&playv1.EndEncounterRequest{CampaignId: w.campaign, EncounterId: rt.encounter.GetId(), IdempotencyKey: newKey()})))
	must(m.play.EndGameSession(t.Context(), rq(&playv1.EndGameSessionRequest{CampaignId: w.campaign, GameSessionId: w.session})))
	for p, sw := range watchers {
		select {
		case <-sw.done:
		case <-time.After(30 * time.Second):
			t.Fatalf("%s's stream did not end with the session", p.name)
		}
		var hints int
		for _, ev := range sw.events {
			body, err := protojson.Marshal(ev)
			if err != nil {
				t.Fatalf("marshal an event: %v", err)
			}
			if eventCase(ev) == "held_rolls_changed" {
				hints++
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
		switch p {
		case ana:
			if hints != 0 {
				t.Errorf("Ana's stream heard %d hints of the held rolls, want none: none was hers", hints)
			}
		case caio, m:
			if hints == 0 {
				t.Errorf("%s's stream heard no hint of the held rolls, want one (the positive control)", p.name)
			}
		}
	}
	t.Run("canaries are reachable", func(t *testing.T) { checkCanariesAreReachable(t, w, rt.got) })
}
