package characters

import (
	"context"

	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// SceneOptions implements play.CombatRoster: what a character adds to each
// check of an RP scene (MR-015), from its derived sheet: the rules engine's
// SceneOptions. It takes no caller: it runs after play's own authorization
// check, and the DC never passes through it. `not_found` for a character that
// is not one of the campaign's living ones.
func (s *Service) SceneOptions(ctx context.Context, campaignID, characterID string, keys []string) ([]link.SceneOption, error) {
	_, d, err := s.fighter(ctx, nil, campaignID, characterID)
	if err != nil {
		return nil, err
	}
	out := make([]link.SceneOption, 0, len(keys))
	for _, k := range keys {
		// One key at a time: a sheet without the check (a basic sheet has no
		// skills) leaves just that one unknown, not the whole scene.
		opts, err := rules.SceneOptions(d, []rules.SceneAction{{Key: k}})
		if err != nil {
			out = append(out, link.SceneOption{})
			continue
		}
		o := opts[0]
		out = append(out, link.SceneOption{Known: true, CheckName: o.NamePT, Bonus: o.Bonus, Passive: o.Passive, HasPassive: o.HasPassive})
	}
	return out, nil
}

// SceneCheckName implements play.CombatRoster: the Portuguese name of a scene
// check by its key, "" for any other key.
func (s *Service) SceneCheckName(key string) string {
	name, _ := s.rules.SceneCheckName(key)
	return name
}
