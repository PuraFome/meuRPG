package play

import (
	"context"
	"fmt"
	"slices"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// The halfling's Lucky (SRD 5.1, Halfling): a halfling who rolls a 1 on the d20 of an attack roll,
// an ability check or a saving throw may roll that die again, and has to keep the new roll. With
// advantage or disadvantage a feature like this rerolls only one of the two d20 (SRD 5.1,
// "Advantage and Disadvantage"), and the SRD's own example rerolls a 1 that advantage would drop:
// the question comes when any of the dice shows a 1, and only that die is rolled again (the first
// one when both show a 1).
//
// The table asks the player, "Tirou 1: rolar de novo?", on the roll of a weapon attack or an
// attack cantrip. The roll waits in the same row a Bardic Inspiration die waits in
// (combat_inspiration.go): RollAttack keeps the d20 of a lucky attacker that shows a 1 with the
// request (lucky_index), resolves and spends nothing, and answers with the question. AnswerLucky
// rolls the die again (in the app or typed, RN-18), or keeps the 1, and asks RollAttack again for
// the same request with the new d20: the attack is resolved, or, when the attacker also holds a
// Bardic Inspiration die, the roll stays held for the die's question, now on the new roll. The 1
// is on the table first, so Lucky comes first; the die is added to whatever the d20 is after it.
// The new roll is never asked about again, even when it is another 1.
//
// The saving throws the app rolls for a player's character inside someone else's call (a spell's
// save, a trap's, the Dexterity save against Hellish Rebuke) have nobody to ask, so the app rolls
// the 1 again by itself (autoLucky): a reroll of a 1 never lowers the result, so a player always
// takes it. The new roll stands, even another 1; with advantage or disadvantage the first die that
// shows a 1 is the one rolled again, and the die that counts is picked again from the new pair.
// Only a player's character is lucky there (luckySaver): an NPC never is, whatever its sheet says.
// The reroll is kept where the save keeps its d20 (saveRoll.Lucky for a spell's and a trap's save,
// actionEvent.Lucky for Hellish Rebuke), so the result and the log say "Sortudo: 1 → 14" to whoever
// reads the save's dice, and the undo of the call has nothing more to put back.

// luckyReroll is the die Lucky rolled again: its index in the faces in the order rolled, the 1 it
// showed and the new face. An attack's event keeps it, so the log still says "Sortudo: 1 → 14"
// after a reload.
//
// A d20 a reroll rule of the table's features rolled again (held_reroll.go) is carried the same way, since the two never
// meet in one roll: Source is then the rule's feature (the Reroll's source key), and Resource and Cost the uses it
// spent, which the undo of the attack or the cast gives back. All three are empty for Lucky's.
type luckyReroll struct {
	Index    int32  `json:"index"`
	From     int32  `json:"from"`
	To       int32  `json:"to"`
	Source   string `json:"source,omitempty"`
	Resource string `json:"resource,omitempty"`
	Cost     int32  `json:"cost,omitempty"`
}

// proto is the reroll as a DiceRoll lists it: none for a nil reroll (no question, or the 1 kept).
func (r *luckyReroll) proto() []*playv1.Reroll {
	if r == nil {
		return nil
	}
	source := link.LuckyTrait
	if r.Source != "" {
		source = r.Source
	}
	return []*playv1.Reroll{{Index: r.Index, From: r.From, To: r.To, SourceKey: source}}
}

// luckyNote is the sentence a log line written as text adds for the d20 Lucky rolled again
// (" Sortudo: 1 → 14."), "" when none was: the structured lines carry it in DiceRoll.rerolled.
func luckyNote(r *luckyReroll) string {
	switch {
	case r == nil:
		return ""
	case r.Source != "": // a reroll rule's: the line does not name the feature, the structured one does
		return fmt.Sprintf(" Rolou de novo: %d → %d.", r.From, r.To)
	}
	return fmt.Sprintf(" Sortudo: %d → %d.", r.From, r.To)
}

// luckyIndexOf is the die of a lucky attacker's roll that shows a 1, the one Lucky would roll
// again: nil when none does, or when the attacker is not lucky.
func luckyIndexOf(faces []int, lucky bool) *int32 {
	if !lucky {
		return nil
	}
	i := slices.Index(faces, 1)
	if i < 0 {
		return nil
	}
	return new(clamp32(i, 0, 1))
}

// luckySaver says the app rolls a 1 again on a saving throw it rolls for this combatant: a
// player's character whose sheet has Lucky. An NPC is never lucky here, even with a halfling's
// sheet.
func luckySaver(c playdb.Combatant, traits link.Traits) bool {
	return c.Kind == kindPlayer && traits.Lucky
}

// autoLucky rolls again, with d20, the first of the faces that shows a 1 when the roller is lucky,
// and gives the new faces and what Lucky did; the faces as they were and nil when nothing was
// rolled again. The new face stands, even another 1.
func autoLucky(faces []int, lucky bool, d20 func() (int, error)) ([]int, *luckyReroll, error) {
	at := luckyIndexOf(faces, lucky)
	if at == nil {
		return faces, nil, nil
	}
	face, err := d20()
	if err != nil {
		return nil, nil, err
	}
	out := slices.Clone(faces)
	out[*at] = face
	return out, &luckyReroll{Index: *at, From: 1, To: clamp32(face, 1, d20Sides)}, nil
}

// appD20 rolls one d20 in the app: the die autoLucky rolls again on a save the app rolled.
func (s *Service) appD20() (int, error) {
	face, _, err := s.d20(rollInput{inApp: true}, 0)
	return face, err
}

// luckySave is a saving throw's d20 the app rolled (roll, with the mode and the modifier it was
// rolled with) after autoLucky: the die that counts and the total are picked again from the new
// faces, as d20With picks them. The roll as it was and nil when nothing was rolled again.
func (s *Service) luckySave(roll d20Roll, modifier int, mode combat.RollMode, lucky bool) (d20Roll, *luckyReroll, error) {
	faces, reroll, err := autoLucky(roll.Faces, lucky, s.appD20)
	if err != nil || reroll == nil {
		return roll, nil, err
	}
	idx := mode.Pick(faces)
	return d20Roll{Faces: faces, Index: idx, Total: faces[idx] + modifier, Physical: roll.Physical}, reroll, nil
}

// luckyOfferView is the held roll of a combatant that waits for its Lucky answer, for its player
// and the master, so the question is still there when the screen is read again.
func luckyOfferView(d *encounterData, c playdb.Combatant, v combatViewer) *playv1.LuckyOffer {
	if !v.master && !v.owns(c) {
		return nil
	}
	for _, h := range d.holds {
		if h.CombatantID != c.ID || derefInt32(h.Round) != d.enc.Round {
			continue
		}
		if luckyAsks(h) {
			return heldOffer{Hold: h, Asks: askLucky}.luckyOffer()
		}
	}
	return nil
}

// luckyRollOf reads how the new d20 of AnswerLucky comes: none when the player keeps the 1; else
// rolled in the app or typed from a real die, 1 to 20.
func luckyRollOf(msg *playv1.AnswerLuckyRequest) (rollInput, error) {
	if !msg.GetReroll() {
		return rollInput{}, nil
	}
	var p rollPick
	switch roll := msg.GetRoll().(type) {
	case *playv1.AnswerLuckyRequest_RollInApp:
		p.app = &roll.RollInApp
	case *playv1.AnswerLuckyRequest_TypedFace:
		p.face = &roll.TypedFace
	}
	return rollOfAnswer(p, askLucky)
}

// AnswerLucky implements playv1connect.ResourceServiceHandler: the player of a held roll whose d20
// shows a 1 rolls that die again, or keeps the 1. The attack is then resolved, or, when the
// attacker holds a Bardic Inspiration die, held again for the die's question on the new roll.
func (s *Service) AnswerLucky(
	ctx context.Context,
	req *connect.Request[playv1.AnswerLuckyRequest],
) (*connect.Response[playv1.AnswerLuckyResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	encID, err := parseCombatID(req.Msg.GetEncounterId(), "encounter")
	if err != nil {
		return nil, err
	}
	holdID, err := parseCombatID(req.Msg.GetHoldId(), "held roll")
	if err != nil {
		return nil, err
	}
	in, err := luckyRollOf(req.Msg)
	if err != nil {
		return nil, err
	}
	out, err := s.answerHeld(ctx, m, answerRequest{
		ref: holdRef{Class: attackHold, Campaign: m.CampaignID, Encounter: encID, ID: holdID}, key: key, question: askLucky, yes: req.Msg.GetReroll(), in: in,
	})
	if err != nil {
		return nil, err
	}
	attack, _ := out.(*playv1.RollAttackResponse)
	return connect.NewResponse(&playv1.AnswerLuckyResponse{Attack: attack}), nil
}

// rollLuckyDie rolls again the die of a hold that shows the 1, at index (or takes the face the
// player typed from a real die): the new roll stands, whatever it is. AnswerLucky and
// AnswerOutsideLucky (inspiration_lucky.go) both roll it here.
func (s *Service) rollLuckyDie(ctx context.Context, index int32, in rollInput) (*luckyReroll, error) {
	face := in.typed
	if in.inApp {
		rolled, err := dice.Roll(s.roller, dice.Expr{Count: 1, Sides: d20Sides})
		if err != nil {
			return nil, s.dbError(ctx, "roll the d20", err)
		}
		face = rolled.Faces[0]
	}
	return &luckyReroll{Index: index, From: 1, To: clamp32(face, 1, d20Sides)}, nil
}
