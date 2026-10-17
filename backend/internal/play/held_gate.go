package play

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// The gate: the one place a d20 that may wait for a question is rolled, kept, asked about and settled.
// The path that rolls (an attack, a check, a save) tells it what the roll is (rollSpec) and goes on
// with what comes back (gateResult): nothing, when the roll waits for nobody (the path rolls as it
// always did); the question to put to the player, when it waits (nothing is written but the hold); or
// the settled d20, when the gate decided it (a lucky roller's d20 that showed no 1, or the answer to a
// question). The question is answered by running the path's own request again with the answer in the
// context (heldReply): the gate then finds the roll it kept and settles it.

// gateEnv is where the gate runs: the path's transaction and its queries, never the pool.
type gateEnv struct {
	tx  pgx.Tx
	q   *playdb.Queries
	now time.Time
	// c is the combat's change, nil out of one; the gate touches c.enc when a question starts to wait in
	// a store whose questions the combat's readers see.
	c *combatTx
}

// combatGate is the environment of a path that runs in a combat's change.
func combatGate(c *combatTx) gateEnv { return gateEnv{tx: c.tx, q: c.q, now: c.now, c: c} }

// outsideGate is the environment of a path that rolls in a transaction of its own, out of a combat's
// change. The hold is stamped with the clock's time.
func (s *Service) outsideGate(tx pgx.Tx, q *playdb.Queries) gateEnv {
	return gateEnv{tx: tx, q: q, now: s.now()}
}

// inspirationGate is the environment of a path in a combat's change whose rolls wait in inspiration_holds:
// the change's transaction, and the question does not move the combat's revision.
func (s *Service) inspirationGate(c *combatTx) gateEnv {
	env := combatGate(c)
	env.now = s.now()
	return env
}

// saveSpec is the saving throw a player rolls for their own combatant (an effect's, a concentration's):
// the master's roll, an NPC's and a creature's never wait for anybody.
func saveSpec(
	m authz.Membership, kind heldKind, reactor playdb.Combatant, key string, raw proto.Message,
	in rollInput, bonus int, mode combat.RollMode, lucky bool,
) rollSpec {
	return rollSpec{
		Kind: kind, Key: key, Request: raw, Roller: heldRoller{CampaignID: m.CampaignID, CharacterID: reactor.CharacterID, UserID: m.UserID},
		In: in, Modifier: bonus, Mode: mode, Lucky: lucky,
		MayAsk: m.Role != authz.RoleMaster && reactor.Kind == kindPlayer && backedByCharacter(reactor) && reactor.UserID != nil && *reactor.UserID == m.UserID,
	}
}

// attackRoller is the attacker of a weapon attack as the gate sees it: its combatant, in the round, and its
// character when a character backs it (a creature or an NPC's copy shares one without being it).
func attackRoller(m authz.Membership, attacker playdb.Combatant, round int32) heldRoller {
	r := heldRoller{CampaignID: m.CampaignID, UserID: m.UserID, Combatant: &attacker, Round: round}
	if backedByCharacter(attacker) {
		r.CharacterID = attacker.CharacterID
	}
	return r
}

// attackRolled is the d20 of a weapon attack as the gate gave it, with the mode it was rolled with.
type attackRolled struct {
	// Held is the question to put to the player: nothing else is settled.
	Held   *heldOffer
	Choice modeChoice
	D20    d20Roll
	// ToHit is the bonus the roll was made with: the held roll's, when an answer settles it.
	ToHit int
	// Bonus is the Bardic Inspiration die the player rolled and added, as it was held; Lucky the die
	// Lucky rolled again.
	Bonus *bonusUse
	Lucky *luckyReroll
}

// rollHeldAttack is the d20 of a weapon attack that may wait for a question: the mode is worked out
// without writing while the roll only waits (an answer settles it with the mode the roll was made with),
// the gate keeps the d20 or settles it, and the reason of the mode is written once the roll goes on, a
// lucky attacker's d20 with no 1 included. An answer settles the attack on the bonus it was made with, Cutting Words
// already off it.
func (s *Service) rollHeldAttack(
	ctx context.Context, c *combatTx, v combatViewer, req *playv1.RollAttackRequest, spec rollSpec,
	attacker, target playdb.Combatant, attackKey string, truth combat.RollMode, in rollInput, toHit int,
) (attackRolled, error) {
	env := combatGate(c)
	answer, answering := heldReplyOf(ctx)
	if answering {
		var err error
		if toHit, err = s.heldModifier(ctx, env, spec); err != nil {
			return attackRolled{}, err
		}
	}
	mayWait, err := s.heldMayWait(ctx, env, spec)
	if err != nil {
		return attackRolled{}, err
	}
	choice, err := s.chooseMode(ctx, c, v, attacker, target, attackKey, truth, modeAsk{
		want: req.GetRollMode(), reason: req.GetModeReason(), requestID: req.GetRollModeRequestId(), dry: mayWait, trusted: answering,
	})
	if err != nil {
		return attackRolled{}, err
	}
	spec.In, spec.Modifier, spec.Mode = in, toHit, choice.Mode
	if mayWait {
		spec.Request = pinnedAttack(req, choice)
	}
	rolled, err := s.heldGate(ctx, env, spec)
	if err != nil || rolled.Held != nil {
		return attackRolled{Held: rolled.Held}, err
	}
	if choice, err = s.keepReason(ctx, c, choice); err != nil {
		return attackRolled{}, err
	}
	out := attackRolled{Choice: choice, D20: rolled.Roll, ToHit: toHit, Lucky: rolled.Lucky}
	if rolled.Bonus != nil {
		out.Bonus = answer.die
	}
	return out, nil
}

// pinnedAttack is the request a held attack keeps: as it was sent, with the mode the roll was made with and its
// reason, so that the answer settles this roll and not another.
func pinnedAttack(raw *playv1.RollAttackRequest, choice modeChoice) *playv1.RollAttackRequest {
	pinned, _ := proto.Clone(raw).(*playv1.RollAttackRequest)
	pinned.RollMode, pinned.ModeReason = modeToProto[choice.Mode], choice.Reason
	return pinned
}

// rollSpec is what a path tells the gate about a roll.
type rollSpec struct {
	Kind heldKind
	Key  string
	// Window says a reaction window held the request first (c.replay != nil): the question gets a key of
	// its own, and the roll is not refused for the questions that are open.
	Window bool
	// Request is the request the answer runs again, pinned by the caller (the mode and its reason) before
	// the gate stores it.
	Request proto.Message
	Roller  heldRoller
	In      rollInput
	// Modifier, Mode and Reliable are what the d20 is counted with: the bonus, advantage or disadvantage,
	// and Reliable Talent.
	Modifier int
	Mode     combat.RollMode
	Reliable bool
	// Lucky says the roller is a halfling: a d20 that shows a 1 asks even without a die.
	Lucky bool
	// MayAsk is the path's policy: false for the rolls that never wait for anybody (the master's rolls
	// out of a combat, an NPC's saves). E4-10 changes it.
	MayAsk bool
}

// gateResult is what the gate gives back.
type gateResult struct {
	// Held says the question is to be put to the player: the path writes nothing and answers with it.
	Held *heldOffer
	// Settled says the gate decided the d20: Roll is it, with Reliable Talent applied and the die added
	// when the player used it (Bonus), and Lucky the die rolled again, which the path's event keeps.
	Settled bool
	Roll    d20Roll
	Bonus   *playv1.BonusDie
	Lucky   *luckyReroll
}

// heldReply is the answer to a question, carried to the path that is run again.
type heldReply struct {
	holdID, key string
	// question is the question the answer is for.
	question questionKind
	// reroll is the die Lucky rolled again, nil when the player kept the 1.
	reroll *luckyReroll
	// die is the Bardic Inspiration die the player rolled and added, nil when they kept it.
	die *bonusUse
}

type heldReplyKey struct{}

func withHeldReply(ctx context.Context, a heldReply) context.Context {
	return context.WithValue(ctx, heldReplyKey{}, a)
}

// heldReplyOf is the answer the context carries: the path is being run again for it.
func heldReplyOf(ctx context.Context) (heldReply, bool) {
	a, ok := ctx.Value(heldReplyKey{}).(heldReply)
	return a, ok
}

// answering is what a question needs to apply an answer: the environment, the path's spec and the
// roller's state.
type answering struct {
	gateEnv
	spec rollSpec
	st   rollerState
}

// heldBlocked refuses the roll while a question of the same roller is open, in either table, before a
// reaction window can hold the new one. The request that made the question (a retry), the answer itself and
// a roll a window already held pass; so does a roll that cannot wait.
func (s *Service) heldBlocked(ctx context.Context, env gateEnv, spec rollSpec) error {
	if _, answer := heldReplyOf(ctx); answer || spec.Window {
		return nil
	}
	st, err := s.rollerStateOf(ctx, env, spec.Roller)
	if err != nil || (!st.Holds && !spec.Lucky) {
		return err
	}
	open, err := newHeldStores(env.q).openOf(ctx, spec.Roller.in(st), rollHolds, inspirationHolds)
	if err != nil {
		return err
	}
	if i := slices.IndexFunc(open, func(h heldRoll) bool { return !h.fromSameRequest(spec.Key) && !h.stale(st) }); i >= 0 {
		return heldRefusal(open[i])
	}
	return nil
}

// heldMayWait says the roll may wait for a question in this call, which is chooseMode's dry run: the
// roll of a roller that holds a die, or of a lucky one (held only when a die shows a 1), or the Lucky
// answer of a roller that holds a die (held again for the die's question). Its mode is then worked out
// without writing, and keepReason writes the reason of a change of mode once the roll goes on after all.
func (s *Service) heldMayWait(ctx context.Context, env gateEnv, spec rollSpec) (bool, error) {
	st, err := s.rollerStateOf(ctx, env, spec.Roller)
	if err != nil {
		return false, err
	}
	if a, answer := heldReplyOf(ctx); answer {
		return a.question == askLucky && st.Holds, nil
	}
	return st.Holds || spec.Lucky, nil
}

// heldModifier is the bonus a held roll was made with, which the answer settles on: the attack's, with a
// Cutting Words die already off it.
func (s *Service) heldModifier(ctx context.Context, env gateEnv, spec rollSpec) (int, error) {
	a, _ := heldReplyOf(ctx)
	h, err := newHeldStores(env.q).byIDOf(ctx, spec.Kind, holdRef{Campaign: spec.Roller.CampaignID, Encounter: encounterOf(spec.Roller), ID: a.holdID})
	if err != nil {
		return 0, fmt.Errorf("find the held roll: %w", err)
	}
	return h.Modifier, nil
}

// encounterOf is the combat of a roller that has a combatant.
func encounterOf(r heldRoller) string {
	if r.Combatant == nil {
		return ""
	}
	return r.Combatant.EncounterID
}

// heldGate is the d20 of a roll that may wait: the answer to a question it kept, else the question, else
// the settled d20 of a roll with nothing to ask, else nothing (the path rolls).
func (s *Service) heldGate(ctx context.Context, env gateEnv, spec rollSpec) (gateResult, error) {
	if !spec.MayAsk {
		return s.gateRoll(spec)
	}
	st, err := s.rollerStateOf(ctx, env, spec.Roller)
	if err != nil {
		return gateResult{}, err
	}
	ae := answering{gateEnv: env, spec: spec, st: st}
	if a, answer := heldReplyOf(ctx); answer {
		return s.gateAnswer(ctx, ae, a)
	}
	if !st.Holds && !spec.Lucky {
		return s.gateRoll(spec)
	}
	return s.gateAsk(ctx, ae)
}

// gateD20 rolls the d20 of a roll, counted as the spec says: Reliable Talent applied, the faces as they
// came up.
func (s *Service) gateD20(spec rollSpec) (d20Roll, error) {
	roll, err := s.d20With(spec.In, spec.Modifier, spec.Mode)
	if err != nil {
		return d20Roll{}, err
	}
	return reliableD20(roll, spec.Modifier, spec.Reliable), nil
}

// gateRoll is the d20 of a roll that waits for nobody: rolled here and settled, for the paths that take
// the gate's roll. The group check (rollCheck) and the trap search (the second die of a Perception search)
// roll their own, and get nothing back.
func (s *Service) gateRoll(spec rollSpec) (gateResult, error) {
	if heldRegistry[spec.Kind].ownRoll {
		return gateResult{}, nil
	}
	roll, err := s.gateD20(spec)
	return gateResult{Settled: true, Roll: roll}, err
}

// gateAsk rolls the d20 of a roll that may wait and keeps it when a question is to be asked. A retry of the
// request that made a question finds it, and rolls nothing again.
func (s *Service) gateAsk(ctx context.Context, ae answering) (gateResult, error) {
	spec := ae.spec
	stores := newHeldStores(ae.q)
	h, found, err := stores.byKeyOf(ctx, spec.Kind, spec.Roller.in(ae.st), spec.Key)
	if err != nil {
		return gateResult{}, err
	}
	if found {
		return gateResult{Held: new(askedBy(h, ae.st))}, nil
	}
	if err := s.gateOpen(ctx, ae, stores); err != nil {
		return gateResult{}, err
	}
	// The hold keeps the faces as they came up; the total counts the d20 by Reliable Talent (the offer
	// shows it), and a Lucky answer counts the new d20 again.
	roll, err := s.gateD20(spec)
	if err != nil {
		return gateResult{}, err
	}
	// A lucky roller without a die is asked only about a 1: any other roll goes on at once.
	luckyAt := luckyIndexOf(roll.Faces, spec.Lucky)
	if !ae.st.Holds && luckyAt == nil {
		return gateResult{Settled: true, Roll: roll}, nil
	}
	return s.gateInsert(ctx, ae, stores, roll, luckyAt)
}

// gateOpen is one question at a time: a roll of the same roller that waits for its answer refuses this one,
// and one whose question went stale is forgotten when it is in the table this roll writes. The tables read
// are the kind's refusedBy.
func (s *Service) gateOpen(ctx context.Context, ae answering, stores heldStores) error {
	def := heldRegistry[ae.spec.Kind]
	open, err := stores.openOf(ctx, ae.spec.Roller.in(ae.st), def.refusedBy...)
	if err != nil {
		return err
	}
	if def.ownKindOnly {
		open = slices.DeleteFunc(open, func(h heldRoll) bool { return h.Kind != ae.spec.Kind })
	}
	for _, h := range open {
		switch {
		case !h.stale(ae.st):
			return heldRefusal(h)
		case h.table == def.write || def.ownKindOnly:
			if err := stores.of(h.table).forget(ctx, h); err != nil {
				return fmt.Errorf("forget the old held roll: %w", err)
			}
		}
	}
	return nil
}

// gateInsert keeps the d20 with the request, so the answer settles this roll and not another, and gives
// the question.
func (s *Service) gateInsert(ctx context.Context, ae answering, stores heldStores, roll d20Roll, luckyAt *int32) (gateResult, error) {
	def := heldRegistry[ae.spec.Kind]
	body, err := def.pinned(ae.spec.Request, roll.Faces)
	if err != nil {
		return gateResult{}, fmt.Errorf("keep the request: %w", err)
	}
	store := stores.of(def.write)
	h, err := store.insert(ctx, ae.gateEnv, ae.newRoll(body, roll, luckyAt))
	if err != nil {
		return gateResult{}, err
	}
	// The question may be part of what the combat's readers see: its revision moves.
	if err := store.asked(ctx, ae.gateEnv, h); err != nil {
		return gateResult{}, err
	}
	return gateResult{Held: new(askedBy(h, ae.st))}, nil
}

// newRoll is the roll to keep: the d20 as it came up and where it was rolled.
func (ae answering) newRoll(body []byte, roll d20Roll, luckyAt *int32) heldRoll {
	spec, r := ae.spec, ae.spec.Roller.in(ae.st)
	h := heldRoll{
		CampaignID: r.CampaignID, CharacterID: r.CharacterID, UserID: r.UserID, Key: spec.Key, Kind: spec.Kind, Window: spec.Window,
		Physical: roll.Physical, Request: body, Faces: roll.Faces, Counted: roll.Index, Modifier: spec.Modifier, Total: roll.Total,
		LuckyIndex: luckyAt,
	}
	if r.Combatant != nil {
		h.CombatantID, h.EncounterID, h.Round = r.Combatant.ID, r.Combatant.EncounterID, &r.Round
		if h.CharacterID == "" {
			// A creature or the copy of an NPC is not backed by its character, but carries one, and the row needs it.
			h.CharacterID = r.Combatant.CharacterID
		}
	}
	return h
}

// gateAnswer settles the roll the answer is for: the question being answered is applied, and then the next
// question's offer, or the settled d20.
func (s *Service) gateAnswer(ctx context.Context, ae answering, a heldReply) (gateResult, error) {
	def := heldRegistry[ae.spec.Kind]
	if a.die != nil && !ae.st.Holds && checksTheAnswer(ae.spec.Kind, def.write) {
		return gateResult{}, errDieGone()
	}
	stores := newHeldStores(ae.q)
	h, err := stores.byIDOf(ctx, ae.spec.Kind, holdRef{Campaign: ae.spec.Roller.CampaignID, Encounter: encounterOf(ae.spec.Roller), ID: a.holdID})
	if err != nil {
		return gateResult{}, fmt.Errorf("find the held roll: %w", err)
	}
	if err := h.answerable(ae.st, a.key); err != nil {
		return gateResult{}, err
	}
	if a.question != askLucky {
		return s.gateSettle(ctx, ae, h, a)
	}
	// The Lucky answer writes the new d20 in the roll first; a roller that holds a die is asked about it
	// next, on the new roll, and nothing else is settled.
	if h, err = (luckyQuestion{}).apply(ctx, ae, h, a); err != nil {
		return gateResult{}, err
	}
	if !(dieQuestion{}).pending(h, ae.st) {
		return s.gateSettle(ctx, ae, h, a)
	}
	if err := stores.of(h.table).asked(ctx, ae.gateEnv, h); err != nil {
		return gateResult{}, err
	}
	return gateResult{Held: new(askedBy(h, ae.st))}, nil
}

// answerable refuses an answer to a roll another answer settled, and, in a table that checks its answers,
// one whose question went stale.
func (h heldRoll) answerable(st rollerState, key string) error {
	if h.AnswerKey != nil && *h.AnswerKey != key {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("the roll was answered already"))
	}
	if h.AnswerKey == nil && h.checksTheAnswer() && h.stale(st) {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("the roll waited too long: its round is over"))
	}
	return nil
}

// gateSettle is the d20 of a roll whose questions are answered, with the die added when the player used
// it: the roll is closed under the answer's key.
func (s *Service) gateSettle(ctx context.Context, ae answering, h heldRoll, a heldReply) (gateResult, error) {
	modifier := ae.spec.Modifier
	roll := d20Roll{Faces: h.Faces, Index: int(clamp32(h.Counted, 0, len(h.Faces)-1)), Physical: h.Physical}
	roll.Total = roll.Face() + modifier
	roll = reliableD20(roll, modifier, ae.spec.Reliable)
	closed, err := (dieQuestion{}).apply(ctx, ae, h, a)
	if err != nil {
		return gateResult{}, err
	}
	out := gateResult{Settled: true, Roll: roll, Lucky: luckyOf(closed)}
	if a.die != nil {
		out.Roll.Total += int(a.die.Face)
		out.Bonus = &playv1.BonusDie{SourceKey: bardicInspirationFt, Sides: ae.st.Die.Sides, Face: a.die.Face, Used: true}
	}
	return out, nil
}
