package play

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/reaction"
)

// A readied spell (SRD 5.1, "Ready"). Only a spell whose casting time is 1 action can be readied. The caster casts it on
// its turn: Ready checks it as CastSpell does (castFlow), spends its slot at once and holds its energy with
// concentration until the trigger, whatever the spell's duration. Nothing of the spell happens yet: no target, no roll,
// no effect, no zone. The release is the cast itself, as a reaction: CastSpell off the caster's turn while its READIED
// window is open (or AnswerReaction "Usar", which builds that request), with the slot already spent and the
// concentration already held. The spell is lost, and its slot stays spent, when the concentration ends before the
// release, when the window is let pass, and whenever the row stops waiting otherwise (the next turn, a defeat, a
// cancel).

// concentrationReadied is the concentration of a combatant holding a readied spell. The held spell is in the row, which
// the master and the readier's player read: every other viewer reads no concentration at all (RN-10).
const concentrationReadied = "readied"

// checkSpell checks the spell a SPELL holds as CastSpell checks a cast on the caster's turn, and reads it at the slot it
// is cast with: the caster may cast now, the spell is one it has, its casting time is 1 action, it may be cast now (a
// slot free, no bonus action spell this turn), the slot fits it, and a verbal spell is not cast in a Silence.
func (f *readyFlow) checkSpell(ctx context.Context) error {
	if f.call.kind != readiedSpell {
		return nil
	}
	t := f.take
	req := &playv1.CastSpellRequest{CampaignId: t.m.CampaignID, EncounterId: t.c.enc.ID, CasterId: t.who.ID, SpellKey: f.call.spellKey}
	cf := newCastFlow(t.s, t.c, t.m, req, castCall{encounterID: t.c.enc.ID, casterID: t.who.ID, spellKey: f.call.spellKey}, &castOut{})
	for _, phase := range []func(context.Context) error{cf.findCaster, cf.pickSpell, cf.checkReadyTime, cf.checkEnabled, f.pickReadySlot(cf), cf.pickSlot, cf.checkSilence} {
		if err := phase(ctx); err != nil {
			return err
		}
	}
	f.spell, f.name = cf, spellOptionName(t.opts, f.call.spellKey)
	return nil
}

// checkReadyTime refuses a spell whose casting time is not 1 action: a bonus action, a reaction, a minute or more.
func (f *castFlow) checkReadyTime(context.Context) error {
	if f.cast.economy != rulesv1.ActionEconomy_ACTION_ECONOMY_ACTION {
		return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_READY_SPELL_TIME, "only a spell with a casting time of 1 action can be readied")
	}
	return nil
}

// pickReadySlot sets the slot the request's slot_level names on the cast: a free slot of that level, a Warlock's pact
// slot when it has no other; none for a cantrip. pickSlot then checks it as CastSpell's slot.
func (f *readyFlow) pickReadySlot(cf *castFlow) func(context.Context) error {
	return func(context.Context) error {
		level := f.call.slotLevel
		if cf.cast.level == 0 && level == 0 {
			return nil
		}
		slot := &playv1.SpellSlot{Level: level}
		pact := slices.IndexFunc(cf.cast.slots, func(s *rulesv1.SlotChoice) bool { return s.GetLevel() == level && s.GetResource() == "" && s.GetPact() })
		plain := slices.ContainsFunc(cf.cast.slots, func(s *rulesv1.SlotChoice) bool { return s.GetLevel() == level && s.GetResource() == "" && !s.GetPact() })
		slot.Pact = !plain && pact >= 0
		cf.req.Slot = slot
		return nil
	}
}

// spellOptionName is the Portuguese name of one of the caster's spells.
func spellOptionName(opts *rulesv1.TurnOptions, key string) string {
	for _, sp := range opts.GetSpells() {
		if sp.GetSpell().GetKey() == key {
			return sp.GetSpell().GetNamePt()
		}
	}
	return ""
}

// holdSpell is what a SPELL spends at Ready, besides the action: the slot (as CastSpell spends it), the spell cast this
// turn, and the concentration, which ends the one the caster had with its creatures, its areas and its effects. The
// event keeps what the undo puts back.
func (f *readyFlow) holdSpell(ctx context.Context) error {
	cf := f.spell
	if cf == nil {
		return nil
	}
	cf.out.made = actionEvent{CastID: uuid.New().String(), Slot: cf.slot}
	cf.sp.Concentration = true // a readied spell holds concentration whatever its duration
	for _, phase := range []func(context.Context) error{cf.spendCastSlot, cf.noteSpellCast, func(ctx context.Context) error { return cf.concentrateOn(ctx, concentrationReadied) }, cf.markHunters} {
		if err := phase(ctx); err != nil {
			return err
		}
	}
	held, made := cf.out.made, &f.take.made
	made.Slot, made.SlotCounted, made.CastID = held.Slot, held.SlotCounted, held.CastID
	made.Concentrate, made.ConcBefore, made.ConcEnded, made.Dismissed, made.MarksBefore = true, held.ConcBefore, held.ConcEnded, held.Dismissed, held.MarksBefore
	made.SpellCastBefore, made.BonusSpellBefore = cf.caster.SpellCast, cf.caster.BonusSpellCast
	f.vitals = cf.out.vitals
	return nil
}

// mayCastNow checks the caster may cast now: on its turn, or, for the spell it readied, as the reaction its open
// READIED window offers. Metamagic shapes a cast, and a readied spell was cast at Ready: its release takes none.
func (f *castFlow) mayCastNow(ctx context.Context) error {
	if err := f.findReadied(ctx); err != nil {
		return err
	}
	if f.readied == nil {
		return f.s.mustActNow(ctx, f.c, f.caster)
	}
	if len(f.req.GetMetamagic()) > 0 {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("a readied spell was cast at Ready: its release takes no Metamagic"))
	}
	return f.s.mustReactNowOrOnTurn(ctx, f.c, f.caster)
}

// findReadied finds the readied spell the cast releases: the caster's open READIED window holds this spell.
func (f *castFlow) findReadied(ctx context.Context) error {
	open, err := openWindowsOf(ctx, f.c)
	if err != nil {
		return err
	}
	for _, w := range open {
		if reaction.Kind(w.Kind) != reaction.Readied || conv.OrZero(w.ReactorID) != f.caster.ID {
			continue
		}
		row, err := f.c.q.GetReadiedAction(ctx, playdb.GetReadiedActionParams{ID: windowTriggerOf(w).Readied, EncounterID: f.c.enc.ID})
		if err != nil {
			return fmt.Errorf("read the readied action: %w", err)
		}
		if row.Kind != readiedSpell || conv.OrZero(row.ActionKey) != f.call.spellKey {
			continue
		}
		f.readied, err = readiedUseOf(ctx, f.c, w.ID, f.caster, readiedSpell)
		return err
	}
	return nil
}

// chosenSlot is the slot of the cast: the request's, or the one a readied spell spent at Ready.
func (f *castFlow) chosenSlot() (*slotRef, error) {
	if f.readied == nil {
		return slotOf(f.req.GetSlot(), f.cast)
	}
	if f.readied.row.SlotLevel == 0 {
		return nil, nil
	}
	return &slotRef{Level: f.readied.row.SlotLevel}, nil
}

// castID is the id of the cast: a readied spell's is the one it got at Ready.
func (f *castFlow) castID() string {
	if f.readied != nil && f.readied.row.SpellCastID != nil {
		return *f.readied.row.SpellCastID
	}
	return uuid.New().String()
}

// releaseHold ends the concentration a readied spell that needs none held until its release.
func (f *castFlow) releaseHold(ctx context.Context) error {
	if f.readied == nil || f.sp.Concentration {
		return nil
	}
	if err := f.c.q.SetCombatantConcentration(ctx, playdb.SetCombatantConcentrationParams{ID: f.caster.ID}); err != nil {
		return fmt.Errorf("release the readied spell: %w", err)
	}
	return nil
}

// spendReadied writes the use of a readied spell: its window answered, its row fired, the reaction's record and the
// readied_fired line.
func (f *castFlow) spendReadied(ctx context.Context) error {
	if f.readied == nil {
		return nil
	}
	return f.readied.spend(ctx, f.c, f.caster, &f.out.made)
}

// heldRelease says a held cast (reaction_holds, kind "cast") is the release of the readied spell of the READIED window:
// its caster is the window's reactor and its spell the one the window holds.
func heldRelease(ctx context.Context, q *playdb.Queries, encID, windowID string, request []byte) (bool, error) {
	var req playv1.CastSpellRequest
	if err := proto.Unmarshal(request, &req); err != nil {
		return false, fmt.Errorf("read a held cast: %w", err)
	}
	w, err := q.GetReactionWindow(ctx, playdb.GetReactionWindowParams{EncounterID: encID, ID: windowID})
	if err != nil {
		return false, fmt.Errorf("find the readied action's window: %w", err)
	}
	t := windowTriggerOf(w)
	return req.GetCasterId() == conv.OrZero(w.ReactorID) && req.GetSpellKey() == t.Key, nil
}

// useReadiedSpell is "Usar" on a readied spell: CastSpell by the readier, under the answer's key, with the targets the
// answer names in creature_ids (or the creature named at Ready) and the answer's die. A spell placed on the map (an area,
// a wall) is released with CastSpell itself, which carries its point. The use is spent once the spell is cast: a cast
// held by a Counterspell is used when it happens.
func (s *Service) useReadiedSpell(ctx context.Context, m authz.Membership, in answerInput, msg *playv1.AnswerReactionRequest, row playdb.ReadiedAction) (*connect.Response[playv1.AnswerReactionResponse], error) {
	req := &playv1.CastSpellRequest{
		CampaignId: msg.GetCampaignId(), EncounterId: in.encID, CasterId: row.CombatantID, SpellKey: conv.OrZero(row.ActionKey),
		IdempotencyKey: in.key, Roll: &playv1.CastSpellRequest_RollInApp{RollInApp: true},
	}
	targets := msg.GetCreatureIds()
	if len(targets) == 0 && row.TargetID != nil {
		targets = []string{*row.TargetID}
	}
	for _, id := range targets {
		req.Targets = append(req.Targets, &playv1.SpellTarget{CombatantId: id})
	}
	if typed, ok := msg.GetRoll().(*playv1.AnswerReactionRequest_Typed); ok {
		req.Roll = &playv1.CastSpellRequest_D20Face{D20Face: typed.Typed}
	}
	res, err := s.castSpell(ctx, m, connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.AnswerReactionResponse{
		Encounter: res.Msg.GetEncounter(),
		Result: &playv1.ReactionResult{
			Kind: playv1.ReactionKind_REACTION_KIND_READIED, ByMaster: m.Role == authz.RoleMaster,
			Used: res.Msg.GetCast() != nil, // a cast held by a Counterspell is used once it happens
		},
		HeldRollOffer: res.Msg.GetHeldRollOffer(),
	}), nil
}

// heldSpellView is the concentration of a combatant holding a readied spell as the viewer reads it: the held spell for
// the master and the readier's player (readied is their copy of the row), nothing for anyone else (RN-10), who reads no
// spell, no level and not even that a spell is held.
func heldSpellView(out *playv1.Combatant, readied *playv1.ReadiedAction, c playdb.Combatant) {
	if conv.OrZero(c.ConcentrationSpell) != concentrationReadied {
		return
	}
	out.ConcentrationSpell, out.ConcentrationSpellNamePt = "", ""
	if readied.GetKind() == playv1.ReadiedKind_READIED_KIND_SPELL {
		out.ConcentrationSpell, out.ConcentrationSpellNamePt = readied.GetActionKey(), readied.GetActionNamePt()
	}
}

// concentrationEndedFor is the concentration a line says ended, as the viewer reads it: the one that held a readied
// spell is the master's and the readier's player's (RN-10).
func concentrationEndedFor(key string, v combatViewer, who playdb.Combatant) string {
	if key == concentrationReadied && !v.master && !v.owns(who) {
		return ""
	}
	return key
}
