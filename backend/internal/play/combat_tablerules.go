package play

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/tablerules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// The table's rules inside the combat (MR-025, RN-24, slice 10.4b): the critical
// hit and who sees the death saves. They are read, never cached: a change inside
// a transaction reads them in that transaction (combatTx.rules, PR #121), and a
// read outside one reads them when it builds the viewer (combatViewer.hideDeath),
// so a change of the rules applies from the next roll on. Without Config.Defaults
// the table has the SRD's rules.

// tableRules reads the campaign's table rules. tx is the caller's open
// transaction, nil for a read outside one.
func (s *Service) tableRules(ctx context.Context, tx pgx.Tx, campaignID string) (tablerules.Rules, error) {
	if s.defaults == nil {
		return tablerules.Rules{}, nil
	}
	r, err := s.defaults.StoredTableRules(ctx, tx, campaignID)
	if err != nil {
		return tablerules.Rules{}, fmt.Errorf("read the table's rules: %w", err)
	}
	return r, nil
}

// criticalRuleOf is the rules engine's critical rule for the table's.
func criticalRuleOf(r tablerules.Rules) combat.CriticalRule {
	if r.CriticalMaxPlusRoll {
		return combat.CriticalMaxPlusRoll
	}
	return combat.CriticalDoubledDice
}

// criticalRuleProto is the table's critical rule as the API says it, for the
// screen's hint ("role os dados duas vezes" or "o máximo mais uma rolagem").
func criticalRuleProto(r tablerules.Rules) playv1.CriticalDamageRule {
	if r.CriticalMaxPlusRoll {
		return playv1.CriticalDamageRule_CRITICAL_DAMAGE_RULE_MAX_PLUS_ROLL
	}
	return playv1.CriticalDamageRule_CRITICAL_DAMAGE_RULE_DOUBLED_DICE
}

// pendingCriticalRule is the rule a pending damage was made under: none for a hit
// that was not a critical one, otherwise the one the table had when it was opened
// (critical_max_rule: a critical hit with a flat damage keeps no maximum, and still
// reports it).
func pendingCriticalRule(critical, maxRule bool) playv1.CriticalDamageRule {
	switch {
	case !critical:
		return playv1.CriticalDamageRule_CRITICAL_DAMAGE_RULE_UNSPECIFIED
	case maxRule:
		return playv1.CriticalDamageRule_CRITICAL_DAMAGE_RULE_MAX_PLUS_ROLL
	}
	return playv1.CriticalDamageRule_CRITICAL_DAMAGE_RULE_DOUBLED_DICE
}

// openTx makes the combatTx of a change that is not a write() (the session's
// end, the scenes, the traps, the creatures' leaving), with the table's rules
// read in its transaction: every combatTx is built here or in writeOnce, so no
// path can forget them (a critical hit and the death saves follow them).
func (s *Service) openTx(ctx context.Context, c combatTx) (*combatTx, error) {
	if c.session.CampaignID == "" {
		return &c, nil // a change of the master's after the session ended (a trap damage): no combat to follow the rules
	}
	rules, err := s.tableRules(ctx, c.tx, c.session.CampaignID)
	if err != nil {
		return nil, err
	}
	c.rules = rules
	return &c, nil
}
