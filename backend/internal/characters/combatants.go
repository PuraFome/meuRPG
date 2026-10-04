package characters

import (
	"context"
	"fmt"
	"strings"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// The characters that fight (MR-013). Package play declares what it needs
// (its CombatRoster interface) and this Service implements it with
// CombatParty and CombatCharacters, like the vitals: the numbers of a
// combatant (initiative bonus, speed, an NPC's hit points) come from the
// sheet, so they are read here, where the sheets are. Package play never
// touches the characters table. The methods take no caller: they run after
// play's own authorization check, and the master's notes never go through
// them (RN-11).

// CombatParty returns the campaign's living, active player characters,
// oldest first: the party that fights by default. Not NPCs, not the dead,
// not a character waiting for approval.
func (s *Service) CombatParty(ctx context.Context, campaignID string) ([]link.Character, error) {
	rows, err := s.queries.ListCombatParty(ctx, campaignID)
	if err != nil {
		return nil, s.dbError(ctx, "list the party", err)
	}
	out := make([]link.Character, 0, len(rows))
	for _, row := range rows {
		c, err := s.combatCharacter(row.ID, row.Kind, row.Name, row.PlayerUserID, row.Sheet)
		if err != nil {
			return nil, s.dbError(ctx, "list the party", err)
		}
		out = append(out, c)
	}
	return out, nil
}

// SessionCharacters returns those of ids that are characters of the campaign,
// whatever their status (a dead one included), with their name and player
// only. It implements play.CombatRoster.
func (s *Service) SessionCharacters(ctx context.Context, campaignID string, ids []string) ([]link.Character, error) {
	valid := make([]string, 0, len(ids))
	for _, id := range ids {
		if id, ok := parseUUID(id); ok {
			valid = append(valid, id)
		}
	}
	if len(valid) == 0 {
		return nil, nil
	}
	rows, err := s.queries.ListSessionCharacters(ctx, charactersdb.ListSessionCharactersParams{CampaignID: campaignID, Ids: valid})
	if err != nil {
		return nil, s.dbError(ctx, "list the characters of a session", err)
	}
	out := make([]link.Character, 0, len(rows))
	for _, r := range rows {
		out = append(out, link.Character{ID: r.ID, Name: r.Name, Player: r.Kind == "player", PlayerUserID: deref(r.PlayerUserID)})
	}
	return out, nil
}

// CombatCharacters returns those of ids that may fight in a combat of the
// campaign: its living characters, players' or NPCs, oldest first. IDs that
// are not UUIDs, or name any other character, are left out.
func (s *Service) CombatCharacters(ctx context.Context, campaignID string, ids []string) ([]link.Character, error) {
	valid := make([]string, 0, len(ids))
	for _, id := range ids {
		if id, ok := parseUUID(id); ok {
			valid = append(valid, id)
		}
	}
	if len(valid) == 0 {
		return nil, nil
	}
	rows, err := s.queries.ListCombatCharacters(ctx, charactersdb.ListCombatCharactersParams{CampaignID: campaignID, Ids: valid})
	if err != nil {
		return nil, s.dbError(ctx, "list the characters of a combat", err)
	}
	out := make([]link.Character, 0, len(rows))
	for _, row := range rows {
		c, err := s.combatCharacter(row.ID, row.Kind, row.Name, row.PlayerUserID, row.Sheet)
		if err != nil {
			return nil, s.dbError(ctx, "list the characters of a combat", err)
		}
		out = append(out, c)
	}
	return out, nil
}

// combatCharacter reads the numbers a combatant starts with from a stored
// sheet: derived by the rules for a full sheet (a player, an enemy, a boss),
// as written for a basic one (a minion, a story NPC).
func (s *Service) combatCharacter(id, kind, name string, playerUserID *string, doc []byte) (link.Character, error) {
	sheet, err := loadSheet(id, doc)
	if err != nil {
		return link.Character{}, err
	}
	c := link.Character{ID: id, Name: name, Player: kind == "player", PlayerUserID: deref(playerUserID)}
	switch {
	case sheet.GetFull() != nil:
		d := rules.Derive(buildOf(sheet.GetFull()), s.rules)
		c.InitiativeBonus, c.SpeedFt, c.HitPointsMax = d.Initiative, d.SpeedWalkFt, max(d.HitPointsMax, 0)
		c.SpeedFlyFt, c.Size = d.SpeedFlyFt, s.raceSize(sheet.GetFull().GetRaceKey())
		jumps := combat.JumpLimits(d)
		c.JumpLongDFt, c.JumpHighDFt = jumps.LongRunning, jumps.HighRunning
		if !c.Player {
			c.XPValue = int(sheet.GetFull().GetXpValue())
			c.PortraitImageID = sheet.GetFull().GetPortraitImageId()
		}
	case sheet.GetBasic() != nil:
		b := sheet.GetBasic()
		c.InitiativeBonus, c.SpeedFt, c.HitPointsMax = int(b.GetInitiativeBonus()), int(b.GetSpeedFt()), int(b.GetHitPointsMax())
		c.XPValue = int(b.GetXpValue())
		c.PortraitImageID = b.GetPortraitImageId()
		c.Size = sizeKey(b.GetSize())
	default:
		return link.Character{}, fmt.Errorf("%w: the sheet of character %s has no content", errCorruptDocument, id)
	}
	return c, nil
}

// raceSize is the size of a race, as a link.Character.Size: "medium" for a race
// the content does not know.
func (s *Service) raceSize(raceKey string) string {
	for _, r := range s.rules.Catalog().Races {
		if r.Key == raceKey {
			return strings.ToLower(r.Size)
		}
	}
	return "medium"
}

// sizeKey is a basic sheet's Tamanho as a link.Character.Size: "medium" when the
// sheet says none.
func sizeKey(z rulesv1.CreatureSize) string {
	if z == rulesv1.CreatureSize_CREATURE_SIZE_UNSPECIFIED {
		return "medium"
	}
	return strings.ToLower(strings.TrimPrefix(z.String(), "CREATURE_SIZE_"))
}
