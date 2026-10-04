package play

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The combat log, "Registro do combate" (D10, MR-012; the history screen
// ADR-0007 deferred). It is built on every read from the combat's
// session_events, never stored on its own, so the history and the log cannot
// disagree. Each entry is structured (kind, who, the roll, the outcome, the
// damage): the app writes the Portuguese sentence.
//
// What each viewer gets (RN-10, RN-20):
//
//	                    master                    player
//	an entry            all                       only those without a hidden combatant
//	                                              (when it happened, and now)
//	the dice            all                       their own character's only
//	hit points after    yes                       never
//	"hidden" mark       set on the ones the       never set
//	                    players do not get
//	undo                the last action's entry   never
//
// A combatant the master reveals later does not bring its old entries with
// it: each entry remembers, from the event, whether a hidden combatant was in
// it, and a player never gets that one.

// logEventLimit is how many of a combat's latest events the log reads. A
// variable only so a test can make a combat longer than it.
var logEventLimit int32 = 5000

// logEntry is an entry while it is being built: the event's numbers and who
// may see it.
type logEntry struct {
	id     string
	kind   playv1.CombatLogKind
	at     time.Time
	ev     actionEvent
	status playv1.PendingDamageStatus // the damage of an attack, once it hit
	dmg    *actionEvent               // the damage roll, once it was rolled
	// applied is the master's apply of an attack's damage on a character.
	applied *actionEvent
	// pend is, for a spell, what became of each pending damage it opened, and
	// stopped the pending damages Escudo stopped (a spell attack or an attack).
	pend    map[string]*damageLog
	stopped []string
	// masterOnly is a line only the master gets, whatever the combatants.
	masterOnly bool
	// hosts are the events the entry shows: an attack's own and the ones of its
	// damage, so the entry of the last action is the one to undo.
	hosts []string
}

// damageLog is what became of one pending damage of a spell: where it is, the
// roll that settled it (shared by the whole cast), what it did to its target,
// and the master's apply.
type damageLog struct {
	status  playv1.PendingDamageStatus
	roll    *actionEvent
	hit     damageHit
	applied *actionEvent
}

// ListCombatLog implements playv1connect.CombatServiceHandler.
func (s *Service) ListCombatLog(
	ctx context.Context,
	req *connect.Request[playv1.ListCombatLogRequest],
) (*connect.Response[playv1.ListCombatLogResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	encID, err := parseCombatID(req.Msg.GetEncounterId(), "encounter")
	if err != nil {
		return nil, err
	}
	session, err := s.openSession(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}
	enc, err := s.queries.GetEncounterInSession(ctx, playdb.GetEncounterInSessionParams{GameSessionID: session.ID, ID: encID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("encounter not found"))
	}
	if err != nil {
		return nil, s.dbError(ctx, "find the encounter", err)
	}
	events, err := s.queries.ListEncounterEvents(ctx, playdb.ListEncounterEventsParams{EncounterID: &enc.ID, Limit: logEventLimit})
	if err != nil {
		return nil, s.dbError(ctx, "list the combat's events", err)
	}
	slices.Reverse(events) // oldest first, as buildLog reads them
	// With the dismissed creatures: their lines survive a concentration ending.
	cs, err := s.queries.ListCombatantsWithDismissed(ctx, enc.ID)
	if err != nil {
		return nil, s.dbError(ctx, "list the combatants", err)
	}
	v := viewerOf(m)
	entries := buildLog(events)

	res := &playv1.ListCombatLogResponse{}
	var lastID string
	if v.master {
		recent, err := s.queries.ListRecentSessionEvents(ctx, playdb.ListRecentSessionEventsParams{GameSessionID: session.ID, Limit: recentEvents})
		if err != nil {
			return nil, s.dbError(ctx, "read the latest events", err)
		}
		if last, ok := lastAction(recent, enc.ID); ok {
			lastID = last.ID
			res.UndoableEventId = last.ID
		}
	}

	names := &keyNames{s: s, campaignID: m.CampaignID, byCharacter: map[string]link.Sheet{}}
	byID := make(map[string]playdb.Combatant, len(cs))
	for _, c := range cs {
		byID[c.ID] = c
	}
	// Latest first, in groups by round.
	for _, e := range slices.Backward(entries) {
		out, ok := e.view(ctx, v, byID, names, lastID)
		if !ok {
			continue
		}
		if n := len(res.Rounds); n == 0 || res.Rounds[n-1].Round != out.Round {
			res.Rounds = append(res.Rounds, &playv1.CombatLogRound{Round: out.Round})
		}
		last := res.Rounds[len(res.Rounds)-1]
		last.Entries = append(last.Entries, out)
	}
	return connect.NewResponse(res), nil
}

// buildLog turns a combat's events, in order, into entries. An event an undo
// took back leaves no trace; the events of one attack (its roll, its damage,
// the master's apply or discard, a reaction) merge into one entry, and those of
// a spell (its cast, the damage rolls and applies of its targets) into one.
func buildLog(events []playdb.ListEncounterEventsRow) []*logEntry {
	undone := map[string]bool{}
	for _, e := range events {
		if e.Kind != eventActionUndone {
			continue
		}
		if ev, err := readEvent(e.Payload); err == nil {
			undone[ev.Undone] = true
		}
	}
	var out []*logEntry
	byPending := map[string]*logEntry{}
	for _, e := range events {
		if undone[e.ID] || e.Kind == eventActionUndone {
			continue
		}
		ev, err := readEvent(e.Payload)
		if err != nil {
			continue // never: this package wrote it
		}
		if ev.Round == 0 {
			continue // the log starts when the combat begins: the setup is not in it
		}
		entry := &logEntry{id: e.ID, at: e.CreatedAt, ev: ev, hosts: []string{e.ID}}
		switch e.Kind {
		case eventCombatBegun:
			entry.kind = playv1.CombatLogKind_COMBAT_LOG_KIND_COMBAT_BEGUN
		case eventEncounterEnded:
			entry.kind = playv1.CombatLogKind_COMBAT_LOG_KIND_COMBAT_ENDED
		case eventCombatantMoved:
			if !ev.OnTurn || (ev.DistanceFt == 0 && ev.DistanceDFt == 0 && ev.Jump != jumpHigh) {
				continue // placing a token is not a move of the fight
			}
			entry.kind = playv1.CombatLogKind_COMBAT_LOG_KIND_MOVED
		case eventCombatantHiddenSet:
			entry.kind, entry.masterOnly = playv1.CombatLogKind_COMBAT_LOG_KIND_REVEAL_CHANGED, true
		case eventActionTaken:
			entry.kind = playv1.CombatLogKind_COMBAT_LOG_KIND_ACTION
		case eventHitPointsAdjusted:
			entry.kind, entry.masterOnly = playv1.CombatLogKind_COMBAT_LOG_KIND_HIT_POINTS_ADJUSTED, true
		case eventDeathSaveRolled:
			entry.kind = playv1.CombatLogKind_COMBAT_LOG_KIND_DEATH_SAVE
		case eventDeathConfirmed:
			entry.kind = playv1.CombatLogKind_COMBAT_LOG_KIND_DEATH_CONFIRMED
		case eventConditionsSet:
			entry.kind = playv1.CombatLogKind_COMBAT_LOG_KIND_CONDITIONS_CHANGED
		case eventTurnPartEnded:
			entry.kind = playv1.CombatLogKind_COMBAT_LOG_KIND_TURN_PART_ENDED
		case eventAttackRolled:
			entry.kind = playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK
			if ev.Pending != "" {
				entry.status = playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_AWAITING_ROLL
				byPending[ev.Pending] = entry
			}
		case eventSpellCast:
			entry.kind = playv1.CombatLogKind_COMBAT_LOG_KIND_SPELL_CAST
			entry.pend = map[string]*damageLog{}
			for _, h := range ev.Hits {
				if h.Pending != "" {
					entry.pend[h.Pending] = &damageLog{status: playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_AWAITING_ROLL}
					byPending[h.Pending] = entry
				}
			}
		case eventReactionUsed:
			entry.kind = playv1.CombatLogKind_COMBAT_LOG_KIND_REACTION
			// Escudo that stopped the attack takes its damage away; the hit waits for
			// its roll otherwise, which the roll event says.
			if host, ok := byPending[ev.Pending]; ok {
				host.hosts = append(host.hosts, e.ID)
				if ev.Stopped {
					host.setStatus(ev.Pending, playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_DISCARDED)
					host.stopped = append(host.stopped, ev.Pending)
				}
			}
		case eventReactionDeclined:
			if host, ok := byPending[ev.Pending]; ok {
				host.hosts = append(host.hosts, e.ID) // a decline is the entry's last action to undo
			}
			continue
		case eventDamageRolled, eventDamageApplied, eventDamageDiscarded:
			// The damage lands in the attack's or the spell's entry.
			host, ok := byPending[ev.Pending]
			if !ok {
				continue
			}
			host.hosts = append(host.hosts, e.ID)
			host.land(e.Kind, ev)
			continue
		default:
			continue // initiative, turns, reinforcements: not lines of the log
		}
		out = append(out, entry)
	}
	return out
}

// setStatus records where a pending damage of the entry is.
func (e *logEntry) setStatus(pending string, status playv1.PendingDamageStatus) {
	if dl, ok := e.pend[pending]; ok {
		dl.status = status
		return
	}
	e.status = status
}

// land puts a damage event on the entry of the attack or the spell it belongs
// to: the roll, the master's apply, or his discard.
func (e *logEntry) land(kind string, ev actionEvent) {
	switch kind {
	case eventDamageRolled:
		hits := ev.Settled
		if len(hits) == 0 {
			hits = []damageHit{{Pending: ev.Pending, Target: ev.Target, Amount: ev.Amount, Applied: ev.Applied, After: ev.After, ConcentrationDC: ev.ConcentrationDC}}
		}
		for _, h := range hits {
			status := playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_ROLLED
			if h.Applied {
				status = playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_APPLIED
			}
			if dl, ok := e.pend[h.Pending]; ok {
				dl.roll, dl.hit, dl.status = &ev, h, status
				continue
			}
			e.dmg, e.status = &ev, status
			e.dmg.After = h.After // the target's hit points after, for an NPC that took it at once
		}
	case eventDamageApplied:
		if dl, ok := e.pend[ev.Pending]; ok {
			dl.applied, dl.status = &ev, playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_APPLIED
			return
		}
		e.status, e.applied = playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_APPLIED, &ev
		if e.dmg != nil {
			e.dmg.After = ev.After // the character's hit points after
		}
	case eventDamageDiscarded:
		e.setStatus(ev.Pending, playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_DISCARDED)
	}
}

// view builds the entry the viewer gets, and false when they do not get it.
func (e *logEntry) view(ctx context.Context, v combatViewer, byID map[string]playdb.Combatant, names *keyNames, lastID string) (*playv1.CombatLogEntry, bool) {
	actor, target := byID[e.ev.Actor], byID[e.ev.Target]
	switch e.kind {
	case playv1.CombatLogKind_COMBAT_LOG_KIND_HIT_POINTS_ADJUSTED:
		actor, target = playdb.Combatant{}, byID[e.ev.Actor] // the affected one is the target
	case playv1.CombatLogKind_COMBAT_LOG_KIND_REACTION:
		target = playdb.Combatant{} // who attacked is not part of the line
	case playv1.CombatLogKind_COMBAT_LOG_KIND_DEATH_CONFIRMED, playv1.CombatLogKind_COMBAT_LOG_KIND_CONDITIONS_CHANGED:
		actor, target = playdb.Combatant{}, byID[e.ev.Actor]
	}
	// What a player may see: nothing with a hidden combatant in it, when it
	// happened or now (a combatant the master hides again takes its lines back).
	// A combatant that left the combat is unknown now, and an entry about it
	// would be anonymous: a player does not get it either.
	_, actorKnown := byID[e.ev.Actor]
	_, targetKnown := byID[e.ev.Target]
	known := (e.ev.Actor == "" || actorKnown) && (e.ev.Target == "" || targetKnown || e.kind == playv1.CombatLogKind_COMBAT_LOG_KIND_REACTION)
	visible := !e.masterOnly && !e.ev.Secret && !actor.Hidden && !target.Hidden && known
	for _, h := range e.ev.Hits { // every target of a spell
		hit, ok := byID[h.Target]
		visible = visible && ok && !hit.Hidden
	}
	if !v.master && !visible {
		return nil, false
	}

	// The dice are the master's and the owner's: the master's rolls are not the
	// players'.
	dice := v.master || v.owns(actor)
	out := &playv1.CombatLogEntry{
		Id: e.id, Kind: e.kind, At: timestamppb.New(e.at), Round: e.ev.Round,
		ActorId: actor.ID, ActorLabel: actor.Label, TargetId: target.ID, TargetLabel: target.Label,
		Key: e.ev.Key,
	}
	if v.master {
		out.Hidden = !visible
		out.Undoable = slices.Contains(e.hosts, lastID)
	}
	switch e.kind {
	case playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK, playv1.CombatLogKind_COMBAT_LOG_KIND_ACTION,
		playv1.CombatLogKind_COMBAT_LOG_KIND_SPELL_CAST, playv1.CombatLogKind_COMBAT_LOG_KIND_REACTION:
		out.KeyNamePt = names.of(ctx, actor, e.ev.Key)
	}
	switch e.kind {
	case playv1.CombatLogKind_COMBAT_LOG_KIND_MOVED:
		out.DistanceFt, out.DistanceDft = e.ev.DistanceFt, e.ev.DistanceDFt
		if out.DistanceDft == 0 { // an event written before the tenths of a foot
			out.DistanceDft = e.ev.DistanceFt * 10
		}
		switch e.ev.Jump {
		case jumpLong:
			out.Jump = playv1.JumpKind_JUMP_KIND_LONG
		case jumpHigh:
			out.Jump, out.JumpHeightDft = playv1.JumpKind_JUMP_KIND_HIGH, e.ev.HeightDFt
		}
		out.LandingDifficult = v.master && e.ev.LandingDifficult // the Acrobatics reminder is the master's alone
	case playv1.CombatLogKind_COMBAT_LOG_KIND_REVEAL_CHANGED:
		out.NowHidden = e.ev.NowHidden
		out.ActorId, out.ActorLabel = "", ""
		out.TargetId, out.TargetLabel = e.ev.Actor, byID[e.ev.Actor].Label
	case playv1.CombatLogKind_COMBAT_LOG_KIND_HIT_POINTS_ADJUSTED:
		out.HitPointsDelta = e.ev.Delta
		if e.ev.After != nil {
			out.HitPointsAfter = &e.ev.After.HP
		}
	case playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK:
		out.Outcome = outcomeToProto[e.ev.Outcome]
		out.AsReaction = e.ev.AsReaction
		out.Cover, out.CoverSource = coverDegreeProto(e.ev.Cover), coverSourceProto(e.ev.CoverSource)
		if v.master && e.ev.TargetAC > 0 { // "CA 17: 15 + 2 de meia cobertura": a player never gets an armor class (RN-20)
			out.TargetArmorClass, out.CoverBonus = &e.ev.TargetAC, e.ev.CoverBonus
		}
		if dice {
			out.AttackRoll = diceRoll(1, 20, []int32{e.ev.D20}, e.ev.Modifier, e.ev.Total, e.ev.Physical)
		}
		if len(e.stopped) > 0 { // Escudo stopped it: a miss, with no damage
			out.Outcome, out.StoppedByReaction = playv1.AttackOutcome_ATTACK_OUTCOME_MISS, true
		} else if e.ev.Pending != "" {
			out.Damage = e.damage(dice, v.master, v.master || v.owns(target), out)
		}
	case playv1.CombatLogKind_COMBAT_LOG_KIND_ACTION:
		if e.ev.Heal { // Retomar o fôlego: only the master and its own player see the numbers
			if v.master || v.owns(actor) {
				out.Damage = &playv1.CombatLogDamage{
					Status: playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_APPLIED, Healing: true, Amount: e.ev.Amount,
					Roll: diceRoll(e.ev.DiceCount, e.ev.DiceSides, e.ev.Faces, e.ev.Modifier, e.ev.Total, e.ev.Physical),
				}
			}
		}
	case playv1.CombatLogKind_COMBAT_LOG_KIND_SPELL_CAST:
		out.Spell = e.spellView(v, byID)
	case playv1.CombatLogKind_COMBAT_LOG_KIND_REACTION:
		out.Spell = &playv1.CombatLogSpell{Slot: slotProto(e.ev.Slot)}
	case playv1.CombatLogKind_COMBAT_LOG_KIND_DEATH_SAVE:
		after := e.ev.Death
		if after == nil {
			after = &deathState{}
		}
		out.DeathSave = &playv1.CombatLogDeathSave{
			Outcome: deathOutcomeToProto[e.ev.DeathOutcome], Successes: after.Successes, Failures: after.Failures,
			Stable: after.Successes >= 3, Dying: v.master && after.Failures >= 3,
		}
		if v.master || v.owns(actor) {
			out.DeathSave.Roll = diceRoll(1, 20, []int32{e.ev.D20}, 0, e.ev.D20, e.ev.Physical)
		}
	case playv1.CombatLogKind_COMBAT_LOG_KIND_CONDITIONS_CHANGED:
		out.Conditions = e.ev.Conditions
		out.ConcentrationEndedKey = e.ev.ConcEnded
	}
	return out, true
}

// spellView is the cast as the viewer gets it: what each target did, in the
// order they were listed. A target's save dice are the master's and the
// target's own player's; the caster's d20 the master's and the caster's.
func (e *logEntry) spellView(v combatViewer, byID map[string]playdb.Combatant) *playv1.CombatLogSpell {
	caster := byID[e.ev.Actor]
	out := &playv1.CombatLogSpell{Slot: slotProto(e.ev.Slot), Concentrating: e.ev.Concentrate, ConcentrationEndedKey: e.ev.ConcEnded}
	out.EffectKind, out.PoolRoll, out.EffectConditionKey, out.EffectThreshold = effectHeader(e.ev, v, caster)
	for _, h := range e.ev.Hits {
		target := byID[h.Target]
		t := &playv1.CombatLogSpellTarget{
			TargetId: target.ID, TargetLabel: target.Label, Darts: h.Darts, Outcome: outcomeToProto[h.Outcome],
			AttackRoll: attackRollView(h, v, caster), Save: saveView(h.Save, v, caster, target),
			Effect: effectView(h, v, target),
			Cover:  coverDegreeProto(h.Cover), CoverSource: coverSourceProto(h.CoverSource),
		}
		if v.master && h.TargetAC > 0 {
			t.TargetArmorClass, t.CoverBonus = &h.TargetAC, h.CoverBonus
		}
		if dl, ok := e.pend[h.Pending]; ok {
			if slices.Contains(e.stopped, h.Pending) { // Escudo stopped the spell attack
				t.Outcome = playv1.AttackOutcome_ATTACK_OUTCOME_MISS
			} else {
				t.Damage = dl.view(v, caster, target)
			}
		}
		out.Targets = append(out.Targets, t)
	}
	return out
}

// view is a spell target's damage or heal as the viewer gets it: the amount that
// landed, the dice to the master and the caster's player, what the master
// overruled to him alone.
func (d *damageLog) view(v combatViewer, caster, target playdb.Combatant) *playv1.CombatLogDamage {
	out := &playv1.CombatLogDamage{Status: d.status}
	if d.roll == nil {
		return out // the cast opened it and it is not rolled yet
	}
	r := d.roll
	out.Amount, out.DamageTypeKey, out.DamageTypePt = d.hit.Amount, r.DamageType, damageTypePT[r.DamageType]
	out.Half, out.Healing = d.hit.Half, r.Heal
	if r.Heal && !v.master && !v.owns(target) {
		// A heal capped at the maximum would tell how many hit points the target
		// lacked: everyone but the master and the target's player gets the roll (RN-20).
		out.Amount = r.Total
	}
	if v.master || v.owns(caster) { // the caster's dice, as for an attack's damage
		out.Roll = diceRoll(r.DiceCount, r.DiceSides, r.Faces, r.Modifier, r.Total, r.Physical)
	}
	if d.applied != nil { // the master's apply of a character's damage
		out.Amount, out.DeathFailuresAdded = d.applied.Amount, d.applied.FailuresAdded
		if d.applied.Overridden && v.master {
			rolled := d.applied.Rolled
			out.RolledAmount = &rolled
		}
		if dc := d.applied.ConcentrationDC; dc > 0 && (v.master || v.owns(target)) {
			out.ConcentrationDc = &dc
		}
		if after := d.applied.After; after != nil {
			out.TargetDown = !after.Defeated && after.HP == 0
		}
		return out
	}
	if after := d.hit.After; after != nil && d.status == playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_APPLIED {
		out.TargetDefeated = after.Defeated
		out.TargetDown = !after.Defeated && after.HP == 0
		if dc := d.hit.ConcentrationDC; dc > 0 && (v.master || v.owns(target)) {
			out.ConcentrationDc = &dc
		}
	}
	return out
}

// damage is the damage of an attack as the viewer gets it. The target's hit
// points after it are the master's alone, set on the entry; the concentration
// DC is the master's and the target's own player's (targetsOwn).
func (e *logEntry) damage(dice, master, targetsOwn bool, out *playv1.CombatLogEntry) *playv1.CombatLogDamage {
	d := &playv1.CombatLogDamage{Status: e.status}
	if e.dmg == nil {
		return d // the attack hit and its damage is not rolled yet
	}
	d.Amount, d.DamageTypeKey, d.DamageTypePt = e.dmg.Amount, e.dmg.DamageType, damageTypePT[e.dmg.DamageType]
	if dice {
		d.Roll = diceRoll(e.dmg.DiceCount, e.dmg.DiceSides, e.dmg.Faces, e.dmg.Modifier, e.dmg.Amount, e.dmg.Physical)
	}
	if ap := e.applied; ap != nil { // the master's apply of a character's damage
		d.Amount, d.DeathFailuresAdded = ap.Amount, ap.FailuresAdded
		if ap.Overridden && master {
			rolled := ap.Rolled
			d.RolledAmount = &rolled
		}
		if dc := ap.ConcentrationDC; dc > 0 && targetsOwn {
			d.ConcentrationDc = &dc
		}
	}
	if after := e.dmg.After; after != nil && e.status == playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_APPLIED {
		d.TargetDefeated = after.Defeated
		d.TargetDown = !after.Defeated && after.HP == 0
		if master {
			out.HitPointsAfter = &after.HP
		}
	}
	if e.applied == nil && e.dmg.ConcentrationDC > 0 && targetsOwn && e.status == playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_APPLIED {
		dc := e.dmg.ConcentrationDC
		d.ConcentrationDc = &dc
	}
	return d
}

// keyNames finds the Portuguese names of the attacks and actions an entry
// mentions, from the actor's sheet, read once for each character.
type keyNames struct {
	s           *Service
	campaignID  string
	byCharacter map[string]link.Sheet
}

// of returns the name of the key on the combatant's sheet, or "" when the
// combatant left the combat or the sheet no longer has it.
func (n *keyNames) of(ctx context.Context, c playdb.Combatant, key string) string {
	if c.CharacterID == "" || key == "" {
		return ""
	}
	sheet, ok := n.byCharacter[sheetKey(c)]
	if !ok {
		var err error
		if sheet, err = n.s.sheetOf(ctx, n.campaignID, c); err != nil {
			n.s.logger.WarnContext(ctx, "play: cannot read a sheet for the combat log") // no names or IDs in logs
		}
		n.byCharacter[sheetKey(c)] = sheet
	}
	for _, a := range sheet.Attacks {
		if a.Key == key {
			return a.Name
		}
	}
	for _, a := range sheet.Actions {
		if a.Key == key {
			return a.Name
		}
	}
	for _, a := range sheet.FeatureActions {
		if a.Key == key {
			return a.Name
		}
	}
	if strings.HasPrefix(key, "spell:") && !isCreature(c) {
		if sp, err := n.s.roster.CombatSpell(ctx, n.campaignID, c.CharacterID, key, 0); err == nil {
			return sp.Name
		}
	}
	return ""
}
