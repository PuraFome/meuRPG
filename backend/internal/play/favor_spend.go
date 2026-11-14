package play

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
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
	// Check only validates: a roll a reaction window holds is refused as the spend would refuse it, and nothing is taken (the
	// replay is the roll that spends). The Inspiration must be there at that time, and the replay answers for it later.
	Check bool
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
// The refusals spend nothing. The hold is read first, whatever the request says: a retry of a held roll and the answer to one
// count the Inspiration the hold remembers (even when the retry no longer asks for it) and never spend again, and an answer
// run never spends or refuses. The hold is judged before the caller on purpose: the roll it belongs to is already made, so the
// first call's checks (the holder, the roll, the row) stand and the one thing a request that adds the spend now can be told is
// that the d20 is rolled; the row is gone after the first call, so HAS_NONE would be a lie.
func (s *Service) useFavor(ctx context.Context, env gateEnv, a favorAsk) (favorUse, error) {
	h, found, answer, err := favorHoldOf(ctx, env, a)
	if err != nil {
		return favorUse{}, err
	}
	if found {
		return heldFavor(a, h, answer)
	}
	if !a.Want {
		return favorUse{Mode: combat.Resolve(a.Sources)}, nil
	}
	return spendFavor(ctx, env.q, a)
}

// favorHoldOf finds the held roll the request belongs to: the one an answer is being run for, or the one a retry made under its key.
func favorHoldOf(ctx context.Context, env gateEnv, a favorAsk) (h heldRoll, found, answer bool, err error) {
	stores := newHeldStores(env.q)
	if reply, ok := heldReplyOf(ctx); ok {
		h, err = stores.byIDOf(ctx, holdRef{Campaign: a.CampaignID, ID: reply.holdID})
		if err != nil {
			return heldRoll{}, false, false, fmt.Errorf("find the held roll: %w", err)
		}
		return h, true, true, nil
	}
	h, found, err = stores.byKeyOf(ctx, a.Kind, heldRoller{CampaignID: a.CampaignID}, a.Key)
	return h, found, false, err
}

// heldFavor is the Inspiration of a roll a hold keeps: it counts when the roll was made with it, and nothing is spent. A request
// that asks for it on a hold made without it comes after the d20 (an answer run never refuses: it only reads the hold).
func heldFavor(a favorAsk, h heldRoll, answer bool) (favorUse, error) {
	switch {
	case h.FavorSpent:
		return favorUse{Mode: favorMode(a.Sources), Used: true}, nil
	case a.Want && !answer:
		return favorUse{}, favorBlocked(playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_AFTER_THE_ROLL, "the d20 was already rolled")
	}
	return favorUse{Mode: combat.Resolve(a.Sources)}, nil
}

// spendFavor takes the character's Inspiration for a roll that is being made now.
func spendFavor(ctx context.Context, q *playdb.Queries, a favorAsk) (favorUse, error) {
	if err := favorRefusal(a); err != nil {
		return favorUse{}, err
	}
	if favorMode(a.Sources) == combat.Resolve(a.Sources) {
		return favorUse{}, favorBlocked(playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_ADDS_NOTHING, "the inspiration would not change the roll")
	}
	if a.Check {
		return checkFavor(ctx, q, a)
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

// favorRefusal is the refusal that needs no look at the roll's mode or at the row: the roller is not the owner, or the call
// rolls no d20 (a path that finds the second out before its mode is known asks it alone).
func favorRefusal(a favorAsk) error {
	switch {
	case !a.Own:
		return favorBlocked(playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_NOT_THE_HOLDER, "only the holder of the inspiration may spend it")
	case !a.Rolls:
		return favorBlocked(playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_NOTHING_TO_ROLL, "this call rolls no d20")
	}
	return nil
}

// checkFavor is the validation of a spend that is made later: the character has the Inspiration now, and the roll takes it.
func checkFavor(ctx context.Context, q *playdb.Queries, a favorAsk) (favorUse, error) {
	has, err := q.HasCharacterFavor(ctx, playdb.HasCharacterFavorParams{CharacterID: a.CharacterID, CampaignID: a.CampaignID})
	if err != nil {
		return favorUse{}, fmt.Errorf("read the character's inspiration: %w", err)
	}
	if !has {
		return favorUse{}, favorBlocked(playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_HAS_NONE, "the character has no inspiration")
	}
	return favorUse{Mode: favorMode(a.Sources)}, nil
}

// favorAgainst refuses a request that spends the Inspiration and asks for a mode worse than the one it gives (the roll
// would waste it): the mode the caller asked is not a mode the Inspiration allows. A request that answers a master's
// decision (requestID) is the master's to settle.
func favorAgainst(fv favorUse, want playv1.RollMode, requestID string) error {
	asked, ok := modeFromProto(want)
	if !fv.Spent || !ok || requestID != "" || !fv.Mode.Better(asked) {
		return nil
	}
	return connect.NewError(connect.CodeInvalidArgument, errors.New("spend_favor gives a better mode than the roll_mode asked: leave the Inspiration for another roll"))
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

// favorBack gives the Inspiration the undone roll spent back to its character. It is idempotent: a character that holds another
// one by now (a grant after the spend) keeps the one row, so the master's undo never stacks two.
func (u *undoer) favorBack(ctx context.Context, who playdb.Combatant, ev actionEvent) error {
	if !ev.Favor || who.CharacterID == "" {
		return nil
	}
	return favorGiveBack(ctx, u.c.q, u.c.session.CampaignID, who.CharacterID)
}

// withFavor is the attack roll's mode with the Inspiration counted: the source joins the list the roller reads, and the mode
// is what the sources resolve to with it. fv.Mode is not read: it is the same resolution of the same sources (the caller gave
// useFavor the sources this result holds), so a second copy of it could only disagree. A roll without it is unchanged.
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

// ownsCharacter says the viewer is the owner of the combatant's character: the player of a player's character that a character
// backs, never the master (who may roll for it, but never spends what is the player's).
func ownsCharacter(v combatViewer, c playdb.Combatant) bool {
	return v.owns(c) && c.Kind == kindPlayer && backedByCharacter(c)
}

// favorLine is the line of a roll made with the Inspiration ("Toren usou a Inspiração."). It has the audience of the roll's dice: the
// master and the roller's own player, never another player.
func favorLine(ev actionEvent, v combatViewer, actor playdb.Combatant) string {
	if !ev.Favor || (!v.master && !v.owns(actor)) {
		return ""
	}
	return actor.Label + " usou a Inspiração."
}

// ownsSave says the member who rolls a saving throw is the owner of the player's character that makes it: the player, never the
// master, whose roll for an absent player is not the player's to spend an Inspiration on.
func ownsSave(m authz.Membership, reactor playdb.Combatant) bool {
	return m.Role != authz.RoleMaster && reactor.Kind == kindPlayer && backedByCharacter(reactor) && reactor.UserID != nil && *reactor.UserID == m.UserID
}

// favorOnSave decides the Inspiration of a saving throw of a combatant: the mode to roll with, and whether it counts. rolls is false
// for a save that fails by itself.
func (s *Service) favorOnSave(ctx context.Context, env gateEnv, m authz.Membership, reactor playdb.Combatant, kind heldKind, key string, want bool, sources []combat.Source, rolls bool) (favorUse, error) {
	return s.useFavor(ctx, env, favorAsk{
		Want: want, CampaignID: m.CampaignID, CharacterID: reactor.CharacterID, Own: ownsSave(m, reactor), Key: key, Kind: kind, Sources: sources, Rolls: rolls,
	})
}
