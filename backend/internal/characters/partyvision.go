package characters

import (
	"context"

	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/maps/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/vision"
)

// PartyVision returns the campaign's living player characters that have a
// player, each with what it sees with: its derived darkvision, blindsight and
// truesight (MR-036, D6). The maps module asks for it to work out what each
// player sees on a map with the fog of war on, after its own authorization
// check, so it takes no caller. It implements maps.CharacterDirectory.
//
// The senses come from the derived sheet, so the race and the features count.
// Slice 9.10 will have a druid in Wild Shape send the beast's senses here
// instead, and a familiar the player sees through add its eyes (link.PartyMember.Extra).
func (s *Service) PartyVision(ctx context.Context, campaignID string) ([]link.PartyMember, error) {
	rows, err := s.queries.ListCombatParty(ctx, campaignID)
	if err != nil {
		return nil, s.dbError(ctx, "list the party's senses", err)
	}
	out := make([]link.PartyMember, 0, len(rows))
	for _, row := range rows {
		user := deref(row.PlayerUserID)
		if user == "" {
			continue // its player deleted the account (RN-16): nobody sees through it
		}
		member := link.PartyMember{CharacterID: row.ID, UserID: user}
		sheet, err := loadSheet(row.ID, row.Sheet)
		if err != nil {
			return nil, s.dbError(ctx, "list the party's senses", err)
		}
		if full := sheet.GetFull(); full != nil {
			member.Senses = sensesOf(rules.Derive(buildOf(full), s.rules).Senses)
		}
		out = append(out, member)
	}
	return out, nil
}

// sensesOf turns the derived senses into the ranges the fog uses: the largest
// range of each. Tremorsense shows no square, so it is left out.
func sensesOf(derived []rules.Sense) vision.Senses {
	var out vision.Senses
	for _, d := range derived {
		switch d.Key {
		case "darkvision":
			out.DarkvisionFt = max(out.DarkvisionFt, d.RangeFt)
		case "blindsight":
			out.BlindsightFt = max(out.BlindsightFt, d.RangeFt)
		case "truesight":
			out.TruesightFt = max(out.TruesightFt, d.RangeFt)
		}
	}
	return out
}

// PortraitInUse says whether any NPC of the campaign has the gallery image as its
// portrait. The maps module asks it before it gives a map with the fog of war on
// an image that something else uses (MR-036). It implements maps.CharacterDirectory.
func (s *Service) PortraitInUse(ctx context.Context, campaignID, imageID string) (bool, error) {
	used, err := s.queries.PortraitInUse(ctx, charactersdb.PortraitInUseParams{CampaignID: campaignID, ImageID: imageID})
	if err != nil {
		return false, s.dbError(ctx, "read whether an image is a portrait", err)
	}
	return used, nil
}
