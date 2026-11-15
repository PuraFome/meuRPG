package leaktest

import (
	"slices"
	"testing"

	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
)

// The pass and the refund of the Inspiration in a combat (RN-43, RN-10): who reads each line, who reads GetFavor, and who hears each hint.
// One table of rollsTable (Toren is Caio's, Pensantus is Ana's, both combatants) and a third player, Dani, who joined and is not in the
// combat: Dani reads nothing of the Inspiration, in no read and in no hint. Each step writes, then reads again as everyone: the sentences
// are canaries with the people who may read them (the master reads the ones with no reader named), and the positive controls are the
// readers' own answers. The events of each step are the ones the streams got after the step's own marker.

// stepSpec says what a step proves: want holds who must read each canary now (nobody named: the master alone), gone the canaries nobody
// reads any more, allow the kinds of event each player may hear after the step's marker (the master hears whatever) and need the ones
// they must.
type stepSpec struct {
	want  map[*canary][]*person
	gone  []*canary
	allow map[*person][]string
	need  map[*person][]string
}

// eventCasesAfter are the kinds of the events a stream got after the first n, apart from the marker and the heartbeat.
func eventCasesAfter(sw *streamWatcher, n int) []string {
	var out []string
	for _, ev := range sw.since(n) {
		if ev.GetCurrentMapChanged() != nil || ev.GetHeartbeat() != nil {
			continue
		}
		out = append(out, eventCase(ev))
	}
	return out
}

// passRun is the table, the watchers and the canaries of one run.
type passRun struct {
	t        *testing.T
	rt       *rollsTable
	w        *world
	third    *person
	people   []*person
	watchers map[*person]*streamWatcher
	all      []*canary
	pass     *playv1.FavorPass
	prompt   string
}

func (r *passRun) add(kind, text string, readers ...*person) *canary {
	c := r.w.secrets.add(&canary{needle: text, kind: kind, readers: names(readers)})
	r.all = append(r.all, c)
	return c
}

func (r *passRun) settle() {
	r.w.settle(r.watchers[r.w.master], r.watchers[r.w.caio], r.watchers[r.w.ana], r.watchers[r.third])
}

var (
	favorReads = []func(w *world) read{
		func(*world) read {
			return read{
				procedure: playv1connect.FavorServiceGetFavorProcedure, allow: members,
				req: func(w *world) proto.Message { return &playv1.GetFavorRequest{CampaignId: w.campaign} },
			}
		},
		func(*world) read {
			return read{
				procedure: playv1connect.CombatServiceListCombatLogProcedure, allow: members,
				req: func(w *world) proto.Message {
					return &playv1.ListCombatLogRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId()}
				},
			}
		},
	}
	tableHints = []string{"encounter_changed", "combat_log_changed", "turn_changed", "combatant_moved"}
	rollHints  = append(slices.Clone(tableHints), "vitals_changed") // the roller's own vitals change with the spend
	vitalsOnly = []string{"vitals_changed"}
	vitalsLog  = []string{"vitals_changed", "combat_log_changed"}
)

// readAs reads GetFavor and the combat's log as everyone, Dani included, and keeps the answers.
func (r *passRun) readAs() *answers {
	ans := newAnswers()
	for _, mk := range favorReads {
		rd := mk(r.w)
		r.w.runRead(r.t, rd, ans)
		reply := r.third.call(rd.procedure, rd.req(r.w))
		ans.keep(r.third, reply)
		if !reply.ok() {
			r.t.Errorf("Dani was refused %s: HTTP %d %s", rd.procedure, reply.status, reply.body)
		}
		for _, f := range r.w.inspect(r.third, reply, nil) {
			r.t.Errorf("Dani's %s: %s\n\tanswer: %s", rd.procedure, f, shorten(reply.body))
		}
	}
	return ans
}

// thirdReadsNothing is the structural control: Dani's GetFavor is empty and the log holds no line of the Inspiration.
func (r *passRun) thirdReadsNothing(step string) {
	ctx := r.t.Context()
	res := must(r.third.favor.GetFavor(ctx, rq(&playv1.GetFavorRequest{CampaignId: r.w.campaign})))
	if len(res.GetHolders()) != 0 || len(res.GetPasses()) != 0 || len(res.GetRefundPrompts()) != 0 {
		r.t.Errorf("%s: Dani reads GetFavor = %v, want nothing", step, res)
	}
	log := must(r.third.combat.ListCombatLog(ctx, rq(&playv1.ListCombatLogRequest{CampaignId: r.w.campaign, EncounterId: r.w.encounter.GetId()})))
	for _, rd := range log.GetRounds() {
		for _, e := range rd.GetEntries() {
			if e.GetFavor() != nil {
				r.t.Errorf("%s: Dani reads the line %q", step, e.GetFavor().GetTextPt())
			}
		}
	}
}

func (r *passRun) checkReads(name string, spec stepSpec, ans *answers) {
	r.checkWanted(name, spec, ans)
	for _, c := range spec.gone {
		for _, p := range r.people {
			if ans.holds(p, c) {
				r.t.Errorf("%s: %s still reads %s after its undo", name, p.name, c)
			}
		}
	}
	for _, c := range r.all {
		if ans.holds(r.third, c) {
			r.t.Errorf("%s: Dani reads %s", name, c)
		}
	}
}

func (r *passRun) checkWanted(name string, spec stepSpec, ans *answers) {
	for c, readers := range spec.want {
		for _, p := range readers {
			if !ans.holds(p, c) {
				r.t.Errorf("%s: %s reads %s in no answer: the control that the line is there is missing", name, p.name, c)
			}
		}
		if len(readers) == 0 && !ans.holds(r.w.master, c) {
			r.t.Errorf("%s: the master reads %s in no answer", name, c)
		}
	}
}

func (r *passRun) checkHints(name string, spec stepSpec, marks map[*person]int) {
	for _, p := range r.people {
		got := eventCasesAfter(r.watchers[p], marks[p])
		if p != r.w.master {
			for _, k := range got {
				if !slices.Contains(spec.allow[p], k) {
					r.t.Errorf("%s: %s heard %s, want only %v (heard %v)", name, p.name, k, spec.allow[p], got)
				}
			}
		}
		for _, k := range spec.need[p] {
			if !slices.Contains(got, k) {
				r.t.Errorf("%s: %s heard no %s (heard %v): the control that the hint is sent is missing", name, p.name, k, got)
			}
		}
	}
}

// step does one write, settles the streams, reads as everyone and checks the reads and the hints.
func (r *passRun) step(name string, do func(), spec stepSpec) {
	r.t.Helper()
	marks := map[*person]int{}
	for p, sw := range r.watchers {
		marks[p] = sw.count()
	}
	do()
	r.settle()
	r.checkReads(name, spec, r.readAs())
	r.thirdReadsNothing(name)
	r.checkHints(name, spec, marks)
}

func (r *passRun) grant(character string) func() {
	return func() {
		w := r.w
		must(w.master.favor.GrantFavor(r.t.Context(), rq(&playv1.GrantFavorRequest{CampaignId: w.campaign, CharacterId: character, IdempotencyKey: newKey()})))
	}
}

func (r *passRun) ask() {
	w := r.w
	r.pass = must(w.caio.favor.PassFavor(r.t.Context(), rq(&playv1.PassFavorRequest{
		CampaignId: w.campaign, FromCharacterId: w.toren.GetId(), ToCharacterId: w.pens.GetId(), IdempotencyKey: newKey(),
	}))).GetPass()
	r.all = append(r.all, w.secrets.add(&canary{needle: r.pass.GetId(), kind: "favor-pass-id", readers: names([]*person{w.caio})}))
}

func (r *passRun) answerPass(accept bool) func() {
	return func() {
		w := r.w
		must(w.master.favor.AnswerFavorPass(r.t.Context(), rq(&playv1.AnswerFavorPassRequest{
			CampaignId: w.campaign, PassId: r.pass.GetId(), Accept: accept, IdempotencyKey: newKey(),
		})))
	}
}

func (r *passRun) cancel() {
	w := r.w
	must(w.caio.favor.CancelFavorPass(r.t.Context(), rq(&playv1.CancelFavorPassRequest{CampaignId: w.campaign, PassId: r.pass.GetId(), IdempotencyKey: newKey()})))
}

func (r *passRun) undo() {
	w := r.w
	ctx := r.t.Context()
	log := must(w.master.combat.ListCombatLog(ctx, rq(&playv1.ListCombatLogRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId()})))
	must(w.master.combat.UndoLastAction(ctx, rq(&playv1.UndoLastActionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), ExpectedEventId: log.GetUndoableEventId(), IdempotencyKey: newKey(),
	})))
}

// hide puts an advantage on the combatant that its player does not read: a spend on it adds nothing over every source.
func (r *passRun) hide(label string) {
	_, err := r.w.pool.Exec(r.t.Context(), `INSERT INTO combatant_states (encounter_id, combatant_id, kind, started_round, created_at, group_id, source_key, source_kind, duration_kind, modifiers, player_visible)
		SELECT encounter_id, id, 'effect', 1, now(), gen_random_uuid(), 'spell:bless', 'master', 'until_dismissed', $3::jsonb, false FROM combatants WHERE encounter_id = $1 AND label = $2`,
		r.w.encounter.GetId(), label, `[{"kind":"attack_advantage"}]`)
	if err != nil {
		panic(err)
	}
}

// spendAndPrompt is Toren's attack that spends the Inspiration for nothing; the master's prompt is then open.
func (r *passRun) spendAndPrompt() {
	r.rt.mustAttack(r.w.caio, labelToren, dagger, "Esqueleto", []int32{4, 17}, func(req *playv1.RollAttackRequest) { req.SpendFavor = true })
	prompts := must(r.w.master.favor.GetFavor(r.t.Context(), rq(&playv1.GetFavorRequest{CampaignId: r.w.campaign}))).GetRefundPrompts()
	if len(prompts) != 1 {
		panic("the master reads no single refund prompt")
	}
	r.prompt = prompts[0].GetId()
	r.all = append(r.all, r.w.secrets.add(&canary{needle: r.prompt, kind: "favor-refund-prompt-id"}))
}

func (r *passRun) answerRefund(giveBack bool) func() {
	return func() {
		w := r.w
		must(w.master.favor.AnswerFavorRefund(r.t.Context(), rq(&playv1.AnswerFavorRefundRequest{
			CampaignId: w.campaign, PromptId: r.prompt, GiveBack: giveBack, IdempotencyKey: newKey(),
		})))
	}
}

func newPassRun(t *testing.T) *passRun {
	t.Helper()
	rt := newRollsTable(t)
	w := rt.world
	third := w.newPerson("Dani")
	w.join(false, third)
	r := &passRun{t: t, rt: rt, w: w, third: third, people: []*person{w.master, w.caio, w.ana, third}}
	r.watchers = map[*person]*streamWatcher{}
	for _, p := range r.people {
		r.watchers[p] = w.watchStream(p)
	}
	r.settle()
	return r
}

func TestFavorPassAndRefundReachOnlyWhoTheyAreFor(t *testing.T) {
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("the test: %v", rec)
		}
	}()
	r := newPassRun(t)
	w, caio, ana := r.w, r.w.caio, r.w.ana
	toren, pens := labelToren, labelPens
	grantT := r.add("favor-line-grant", "O mestre deu a Inspiração a "+toren+".")
	refusedM := r.add("favor-line-refused-master", "O mestre recusou o pedido de "+toren+": a Inspiração continua com "+toren+".")
	refusedG := r.add("favor-line-refused-giver", "O mestre recusou: a Inspiração continua com você.", caio)
	canceledM := r.add("favor-line-canceled-master", toren+" desistiu de passar a Inspiração para "+pens+".")
	canceledG := r.add("favor-line-canceled-giver", "Você desistiu de passar a Inspiração.", caio)
	passedM := r.add("favor-line-passed-master", toren+" passou a Inspiração para "+pens+".")
	passedG := r.add("favor-line-passed-giver", "O mestre aceitou: a sua Inspiração foi para "+pens+".", caio)
	passedR := r.add("favor-line-passed-receiver", toren+" passou a Inspiração para você.", ana)
	undoneR := r.add("favor-line-undone-receiver", "O mestre desfez a passagem: a Inspiração não está mais com você.", ana)
	refundedM := r.add("favor-line-refunded-master", "O mestre devolveu a Inspiração de "+toren+".")
	refundedR := r.add("favor-line-refunded-roller", "O mestre devolveu a sua Inspiração.", caio)
	both := func(cs ...*canary) map[*canary][]*person {
		out := map[*canary][]*person{}
		for _, c := range cs {
			out[c] = nil
		}
		return out
	}
	toCaio := func(allow, need []string) (map[*person][]string, map[*person][]string) {
		return map[*person][]string{caio: allow}, map[*person][]string{caio: need}
	}

	// The grant is the master's line alone: the holder's player hears the vitals hint, not the log's.
	r.step("the grant", r.grant(w.toren.GetId()), stepSpec{want: both(grantT), allow: map[*person][]string{caio: vitalsOnly}, need: map[*person][]string{caio: vitalsOnly, w.master: vitalsLog}})
	// The ask is no line: the giver hears the vitals hint, and Ana (the receiver) and Dani nothing.
	r.step("the ask", r.ask, stepSpec{allow: map[*person][]string{caio: vitalsOnly}, need: map[*person][]string{caio: vitalsOnly}})
	// A refusal moved nothing: the giver and the master read it and hear it; the receiver hears and reads nothing.
	allow, need := toCaio(vitalsLog, vitalsLog)
	want := both(refusedM)
	want[refusedG] = []*person{caio}
	r.step("the refusal", r.answerPass(false), stepSpec{want: want, allow: allow, need: need})
	allow, need = toCaio(vitalsLog, []string{"combat_log_changed"})
	r.step("the undo of the refusal", r.undo, stepSpec{gone: []*canary{refusedM, refusedG}, allow: allow, need: need})
	r.step("the second ask", r.ask, stepSpec{allow: map[*person][]string{caio: vitalsOnly}})
	want = both(canceledM)
	want[canceledG] = []*person{caio}
	allow, need = toCaio(vitalsLog, vitalsLog)
	r.step("the cancel", r.cancel, stepSpec{want: want, allow: allow, need: need})
	allow, _ = toCaio(vitalsLog, nil)
	r.step("the undo of the cancel", r.undo, stepSpec{gone: []*canary{canceledM, canceledG}, allow: allow})
	r.step("the third ask", r.ask, stepSpec{allow: map[*person][]string{caio: vitalsOnly}})
	// Only the accepted pass reaches the receiver.
	want = both(passedM)
	want[passedG], want[passedR] = []*person{caio}, []*person{ana}
	r.step("the accept", r.answerPass(true), stepSpec{
		want:  want,
		allow: map[*person][]string{caio: vitalsLog, ana: vitalsLog}, need: map[*person][]string{caio: vitalsLog, ana: vitalsLog},
	})
	r.step("the undo of the pass", r.undo, stepSpec{
		want: map[*canary][]*person{undoneR: {ana}}, gone: []*canary{passedM, passedG, passedR},
		allow: map[*person][]string{caio: vitalsLog, ana: vitalsLog}, need: map[*person][]string{ana: vitalsOnly},
	})
	r.step("the undo of the grant", r.undo, stepSpec{gone: []*canary{grantT}, allow: map[*person][]string{caio: vitalsOnly}, need: map[*person][]string{caio: vitalsOnly}})

	// The refund: a roll that spent the Inspiration for nothing (a hidden advantage the roller does not read) asks the master alone.
	r.hide(toren)
	r.step("the grant for the refund", r.grant(w.toren.GetId()), stepSpec{want: both(grantT), allow: map[*person][]string{caio: vitalsOnly}})
	rolled := stepSpec{allow: map[*person][]string{caio: rollHints, ana: tableHints, r.third: tableHints}}
	r.step("the roll that opens the prompt", r.spendAndPrompt, rolled)
	allow, need = toCaio(vitalsLog, vitalsLog)
	r.step("the refund", r.answerRefund(true), stepSpec{want: map[*canary][]*person{refundedM: nil, refundedR: {caio}}, allow: allow, need: need})

	// The returned Inspiration is Toren's again: the next round he spends it the same way and the master keeps it spent.
	r.rt.toTurn(pens)
	r.rt.toTurn(toren)
	r.settle()
	r.step("the roll that opens the second prompt", r.spendAndPrompt, rolled)
	r.step("the keep", r.answerRefund(false), stepSpec{need: map[*person][]string{w.master: vitalsOnly}})
}
