package play

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// The spend of the Inspiration (RN-43): the owner of a player character that has it takes it for advantage on one of the
// character's own d20 rolls, chosen before the d20 is rolled. useFavor is the one place that decides it; the paths that roll
// a d20 call it with the mode they worked out and roll with the mode it gives back.

// favorSource is the circumstance a spend adds to a roll's sources: advantage, the roller's own, never hidden.
var favorSource = combat.Source{Kind: combat.SourceFavor, Effect: combat.ModeAdvantage}

// favorAsk is what a path tells useFavor about its roll.
type favorAsk struct {
	// Want is the request's spend_favor.
	Want                    bool
	CampaignID, CharacterID string
	// Own says the caller is the owner of the character that rolls: the player, never the master, and a player character.
	Own bool
	// Key and Kind find the held roll the request made, when it is a retry of one.
	Key  string
	Kind heldKind
	// Sources are the circumstances of the roll before the Inspiration (after Protection and the like): the mode with it is
	// what they and the Inspiration resolve to (SRD 5.1: any advantages and disadvantages together cancel to a straight roll).
	Sources []combat.Source
	// Rolls is false when the call rolls no d20 (a save that fails by itself, an area spell), so there is nothing to spend on.
	Rolls bool
}

// favorUse is what useFavor decided.
type favorUse struct {
	// Mode is the mode to roll with.
	Mode combat.RollMode
	// Used says the Inspiration counts on this roll: it was spent now, or by the held roll a retry or an answer belongs to.
	Used bool
	// Spent says it was spent in this call, so a hold the roll makes must remember it (favor_spent).
	Spent bool
}

// favorMode is the mode a roll has once the Inspiration counts.
func favorMode(sources []combat.Source) combat.RollMode {
	return combat.Resolve(append(append([]combat.Source(nil), sources...), favorSource))
}

// notesSources are the sources a check's notes make, for the helper: a bonus says nothing of advantage, and a note that is
// not an advantage is a disadvantage (as notesMode reads them).
func notesSources(notes []rollNote) []combat.Source {
	var out []combat.Source
	for _, n := range notes {
		if n.Kind == noteEffectBonus {
			continue
		}
		effect := combat.ModeDisadvantage
		if n.Adv {
			effect = combat.ModeAdvantage
		}
		out = append(out, combat.Source{Kind: n.Kind, Effect: effect})
	}
	return out
}

// useFavor decides the spend of a roll, in the caller's transaction (so the row is deleted only if the roll is written).
// The refusals spend nothing. A retry of a held roll and the answer to one never spend again: the hold remembers whether
// the roll was made with the Inspiration.
func (s *Service) useFavor(ctx context.Context, env gateEnv, a favorAsk) (favorUse, error) {
	if !a.Want {
		return favorUse{Mode: combat.Resolve(a.Sources)}, nil
	}
	stores := newHeldStores(env.q)
	if reply, answer := heldReplyOf(ctx); answer {
		h, err := stores.byIDOf(ctx, holdRef{Campaign: a.CampaignID, ID: reply.holdID})
		if err != nil {
			return favorUse{}, fmt.Errorf("find the held roll: %w", err)
		}
		return heldFavor(a, h), nil
	}
	h, found, err := stores.byKeyOf(ctx, a.Kind, heldRoller{CampaignID: a.CampaignID}, a.Key)
	if err != nil {
		return favorUse{}, err
	}
	if found && !h.FavorSpent {
		return favorUse{}, favorBlocked(playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_AFTER_THE_ROLL, "the d20 was already rolled")
	}
	if found {
		return heldFavor(a, h), nil
	}
	return spendFavor(ctx, env.q, a)
}

// heldFavor is the Inspiration of a roll a hold keeps: it counts when the roll was made with it, and nothing is spent or refused.
func heldFavor(a favorAsk, h heldRoll) favorUse {
	if !h.FavorSpent {
		return favorUse{Mode: combat.Resolve(a.Sources)}
	}
	return favorUse{Mode: favorMode(a.Sources), Used: true}
}

// spendFavor takes the character's Inspiration for a roll that is being made now.
func spendFavor(ctx context.Context, q *playdb.Queries, a favorAsk) (favorUse, error) {
	switch {
	case !a.Own:
		return favorUse{}, favorBlocked(playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_NOT_THE_HOLDER, "only the holder of the inspiration may spend it")
	case !a.Rolls:
		return favorUse{}, favorBlocked(playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_NOTHING_TO_ROLL, "this call rolls no d20")
	case favorMode(a.Sources) == combat.Resolve(a.Sources):
		return favorUse{}, favorBlocked(playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_ADDS_NOTHING, "the inspiration would not change the roll")
	}
	_, err := q.SpendCharacterFavor(ctx, playdb.SpendCharacterFavorParams{CharacterID: a.CharacterID, CampaignID: a.CampaignID})
	if errors.Is(err, pgx.ErrNoRows) {
		return favorUse{}, favorBlocked(playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_HAS_NONE, "the character has no inspiration")
	}
	if err != nil {
		return favorUse{}, fmt.Errorf("spend the character's inspiration: %w", err)
	}
	return favorUse{Mode: favorMode(a.Sources), Used: true, Spent: true}, nil
}

// favorGiveBack puts the Inspiration a roll spent back, for the undo of that roll. It is idempotent: a character that has it
// again keeps the one row.
func favorGiveBack(ctx context.Context, q *playdb.Queries, campaignID, characterID string) error {
	_, err := q.InsertCharacterFavor(ctx, playdb.InsertCharacterFavorParams{CharacterID: characterID, CampaignID: campaignID})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("give the character's inspiration back: %w", err)
	}
	return nil
}

// withFavor is the attack roll's mode with the Inspiration counted: the source joins the list the roller reads, and the mode
// is the one useFavor decided. A roll without it is unchanged.
func (m modeResult) withFavor(fv favorUse, names func(string) string) modeResult {
	if !fv.Used {
		return m
	}
	m.Sources = append(append([]combat.Source(nil), m.Sources...), favorSource)
	m.Shown = append(append([]*playv1.AdvantageSource(nil), m.Shown...), sourceProto(favorSource, textContext{self: true, names: names}, nil))
	m.Mode = combat.Resolve(m.Sources)
	return m
}

// withFavor is the check's mode with the Inspiration counted.
func (cm checkMode) withFavor(fv favorUse) checkMode {
	if !fv.Used {
		return cm
	}
	cm.Sources = append(append([]combat.Source(nil), cm.Sources...), favorSource)
	cm.Mode = combat.Resolve(cm.Sources)
	return cm
}

// withFavor is the combatant's check with the Inspiration counted: a note of it, and the mode their notes make.
func (p preparedCheck) withFavor(fv favorUse) preparedCheck {
	if !fv.Used {
		return p
	}
	p.notes = append(append([]rollNote(nil), p.notes...), rollNote{Kind: noteFavor, Label: labelFavor, Adv: true})
	p.mode = notesMode(p.notes)
	return p
}
