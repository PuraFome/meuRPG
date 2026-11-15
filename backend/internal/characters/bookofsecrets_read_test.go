package characters

import (
	"testing"

	"connectrpc.com/connect"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// TestTheOwnerKeepsReadingARitualOfTheirBookAfterTheMasterSwitchesItOff: the ritual a warlock holds in the Book of
// Ancient Secrets is a key of the sheet (RN-23), so its owner reads it after the master switched it off, and another
// player does not.
func TestTheOwnerKeepsReadingARitualOfTheirBookAfterTheMasterSwitchesItOff(t *testing.T) {
	t.Parallel()
	tb := newChoiceTable(t)
	row := spellSwitchRows()[0]
	if _, err := tb.create(tb.ana, row.sheet(rules.ScopedChoice(row.choiceKey, row.held), rules.ScopedChoice(row.choiceKey, row.allowed))); err != nil {
		t.Fatalf("a warlock with %s in the book: %v", row.held, err)
	}
	tb.master.setOff(t, tb.campaign, true, row.held)
	read := func(u *user) error {
		_, err := u.content.GetSpellDetails(t.Context(), connect.NewRequest(&rulesv1.GetSpellDetailsRequest{CampaignId: tb.campaign, SpellKey: row.held}))
		return err
	}
	if err := read(tb.ana); err != nil {
		t.Errorf("the owner of the sheet that holds %s in the book: error = %v, want it read", row.held, err)
	}
	if err := read(tb.caio); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("another player: error = %v, want not_found", err)
	}
	if err := read(tb.master); err != nil {
		t.Errorf("the master: error = %v, want it read", err)
	}
}
