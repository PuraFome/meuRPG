package characters

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode/utf8"

	"connectrpc.com/connect"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The picker of the sheet's magic items: the SRD items besides the +N gear, found by name. They are public rules, so
// this file only checks who may ask and the shape of the page.

const maxMagicItemQuery = 100

// ListMagicItems implements rulesv1connect.ContentServiceHandler.
func (s *Service) ListMagicItems(
	ctx context.Context,
	req *connect.Request[rulesv1.ListMagicItemsRequest],
) (*connect.Response[rulesv1.ListMagicItemsResponse], error) {
	// Active members only, as for ListSpells: a pending member has the catalog to build a sheet (RN-15).
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	if utf8.RuneCountInString(msg.GetQuery()) > maxMagicItemQuery {
		return nil, invalidArgument(fieldErr("query", "must have at most %d characters", maxMagicItemQuery))
	}
	size, err := spellsPageSize(msg.GetPageSize())
	if err != nil {
		return nil, err
	}
	filters := magicItemsFilterID(msg)
	offset := 0
	if token := msg.GetPageToken(); token != "" {
		var ok bool
		if offset, ok = parseSpellsToken(token, filters); !ok {
			return nil, invalidArgument(fieldErr("page_token", "is not a token of this list"))
		}
	}
	content, err := s.contentFor(ctx, nil, m.CampaignID)
	if err != nil {
		return nil, s.dbError(ctx, "read rules content", err)
	}
	list := content.ListMagicItems(msg.GetQuery())
	if offset > len(list) {
		return nil, invalidArgument(fieldErr("page_token", "is not a token of this list"))
	}
	end := min(offset+size, len(list))
	res := &rulesv1.ListMagicItemsResponse{Total: i32(len(list)), ContentVersion: content.Version()}
	for _, o := range list[offset:end] {
		res.Items = append(res.Items, magicOptionToProto(o))
	}
	if end < len(list) {
		res.NextPageToken = spellsToken(end, filters)
	}
	return connect.NewResponse(res), nil
}

func magicOptionToProto(o rules.MagicItemOption) *rulesv1.MagicItemOption {
	out := &rulesv1.MagicItemOption{
		Key: o.Key, NamePt: o.NamePT, Category: o.Category, Rarity: o.Rarity,
		RequiresAttunement: o.Attunement, RestrictionPt: o.RestrictionPT, Standalone: o.Standalone,
	}
	for _, v := range o.Options {
		out.Variants = append(out.Variants, &rulesv1.MagicItemVariant{
			Key: v.Key, NamePt: v.NamePT, Rarity: v.Rarity, RequiresAttunement: v.Attunement, RestrictionPt: v.RestrictionPT,
		})
	}
	return out
}

// magicItemsFilterID is a short fingerprint of the search, so a page token only works for the list it came from.
func magicItemsFilterID(r *rulesv1.ListMagicItemsRequest) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{"magic-items", r.GetQuery()}, "\x00")))
	return hex.EncodeToString(sum[:6])
}
