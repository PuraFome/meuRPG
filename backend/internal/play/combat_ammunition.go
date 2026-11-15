package play

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// Ammunition in the attack roll (SRD 5.1, "Weapons": the ammunition property). A player's character that makes a ranged
// attack with a weapon that fires ammunition (the line carries the kind) takes one piece of it, in the transaction that
// writes the attack: with none left the attack is refused before anything is written, and the piece is taken after the
// roll settled, so a roll that waits for an answer takes nothing yet. The undo of the attack gives the piece back in its own
// transaction (ammunitionBack). A thrown weapon, a melee one, the improvised melee line of a bow and the attacks of the
// creatures and of the master's NPCs carry no kind and take nothing.

// firesAmmunition says the attack takes a piece of ammunition: a player's character with a weapon line that fires some.
func (f *attackFlow) firesAmmunition() bool {
	return f.attack.Ammunition != "" && !f.attack.Melee && f.attacker.Kind == kindPlayer && backedByCharacter(f.attacker)
}

// checkAmmunition refuses the attack when the character has none of the kind left.
func (f *attackFlow) checkAmmunition(ctx context.Context) error {
	if !f.firesAmmunition() {
		return nil
	}
	left, err := f.s.vitals.AmmunitionLeft(ctx, f.c.tx, f.m.CampaignID, f.attacker.CharacterID, f.attack.Ammunition)
	if err != nil {
		return err
	}
	if left <= 0 {
		return errAmmunitionEmpty()
	}
	return nil
}

// errAmmunitionEmpty is the refusal of a shot with no ammunition left.
func errAmmunitionEmpty() error {
	return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_AMMUNITION_EMPTY, "no ammunition left for this weapon")
}

// spendAmmunition takes the piece of the shot and writes the master's and the owner's line of it, before the attack's own
// event. The keeper locks the character's row, so two shots at the last piece never both fire: the second is refused as
// if it came after.
func (f *attackFlow) spendAmmunition(ctx context.Context) error {
	if !f.firesAmmunition() {
		return nil
	}
	campaignID, charID := f.m.CampaignID, f.attacker.CharacterID
	left, err := f.s.vitals.SpendAmmunition(ctx, f.c.tx, campaignID, charID, f.attack.Ammunition, f.c.enc.ID)
	if errors.Is(err, link.ErrNoAmmunition) {
		return errAmmunitionEmpty()
	}
	if err != nil {
		return err
	}
	names, err := f.s.roster.ContentNames(ctx, f.c.tx, campaignID)
	if err != nil {
		return err
	}
	ev := itemUseEvent{
		Round: f.c.enc.Round, Actor: f.attacker.ID, Secret: true, Item: f.attack.Key, Name: f.attack.Name, Amount: 1, Left: left,
		Outcome: itemOutcomeSpent, Ammo: names(f.attack.Ammunition),
	}
	body, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("encode the event payload: %w", err)
	}
	line, err := insertRawAs(ctx, f.c, eventAmmunitionSpent, &f.m.UserID, &charID, nil, body)
	if err != nil {
		return err
	}
	f.ammoSpent, f.ammoLine = f.attack.Ammunition, line
	f.s.tellVitalsAfter(f.c.hints, campaignID, charID)
	return nil
}

// ammunitionBack gives back the piece an undone attack took, to the character that fired it (the keeper lowers the combat's
// count of what was fired by it). An attack that took none, or whose attacker is no player's character, gives nothing.
func (u *undoer) ammunitionBack(ctx context.Context, who playdb.Combatant, ev actionEvent) error {
	if ev.Ammo == "" || !backedByCharacter(who) {
		return nil
	}
	if _, err := u.s.vitals.RefundAmmunition(ctx, u.c.tx, u.c.session.CampaignID, who.CharacterID, ev.Ammo, u.c.enc.ID, 1); err != nil {
		return err
	}
	u.s.tellVitalsAfter(u.c.hints, u.c.session.CampaignID, who.CharacterID)
	return nil
}
