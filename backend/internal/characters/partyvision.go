package characters

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/maps/link"
	playlink "github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/vision"
)

// basicPassivePerception is the passive Perception of a sheet with no skills: the plain 10.
const basicPassivePerception = 10

// PartyVision returns the campaign's player characters that have a player (the
// living ones, and the dead ones as Dead), each with what it sees with: its derived darkvision, blindsight and
// truesight (MR-036, D6). The maps module asks for it to work out what each
// player sees on a map with the fog of war on, after its own authorization
// check, so it takes no caller. It implements maps.CharacterDirectory.
//
// The senses come from the derived sheet, so the race and the features count. A
// druid in Wild Shape sends the beast's senses instead (a wolf has no darkvision),
// and a player looking through their familiar's eyes sends them as Eyes (MR-037, D6).
func (s *Service) PartyVision(ctx context.Context, tx pgx.Tx, campaignID string) ([]link.PartyMember, error) {
	rows, err := s.queriesIn(tx).ListPartyVision(ctx, campaignID)
	if err != nil {
		return nil, s.dbError(ctx, "list the party's senses", err)
	}
	content, err := s.contentFor(ctx, tx, campaignID)
	if err != nil {
		return nil, s.dbError(ctx, "read rules content", err)
	}
	out := make([]link.PartyMember, 0, len(rows))
	for _, row := range rows {
		user := deref(row.PlayerUserID)
		if user == "" {
			continue // its player deleted the account (RN-16): nobody sees through it
		}
		// A basic sheet has no skills: its passive Perception is the plain 10.
		member := link.PartyMember{CharacterID: row.ID, UserID: user, Dead: row.IsDead, PassivePerception: basicPassivePerception}
		if member.Dead {
			out = append(out, member) // it sees and notices nothing: its player sees the party's view
			continue
		}
		sheet, err := loadSheet(row.ID, row.Sheet)
		if err != nil {
			return nil, s.dbError(ctx, "list the party's senses", err)
		}
		if full := sheet.GetFull(); full != nil {
			// The form's senses and passive Perception, when the druid is a beast (MR-037).
			derived := derive(content, full, row.WildShapeBeast)
			member.Senses = sensesOf(derived.Senses)
			member.PassivePerception = derived.PassivePerception
		}
		if row.FamiliarID != nil && row.FamiliarMonsterKey != nil {
			if d, ok := content.MonsterDerived(*row.FamiliarMonsterKey); ok {
				member.Eyes = &link.FamiliarEyes{CreatureID: *row.FamiliarID, Senses: sensesOf(d.Senses)}
			}
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

// senseRanges is sensesOf for a combat's sheet (play/link): what an NPC or a
// creature sees with on a map with the fog of war.
func senseRanges(derived []rules.Sense) playlink.Senses {
	v := sensesOf(derived)
	return playlink.Senses{DarkvisionFt: v.DarkvisionFt, BlindsightFt: v.BlindsightFt, TruesightFt: v.TruesightFt}
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
