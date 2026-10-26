package characters

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// A spell cast through a granted_spell effect: the attack bonus and the save DC are the grant's
// own, from its ability and the proficiency bonus, whatever the classes cast with (MR-025).
// The feat is the master's and its names are invented; these tests need the database
// (MEURPG_TEST_DATABASE_URL).

func TestMR025_AGrantedSpellIsCastWithTheGrantsAbility(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	p := h.newUser("Jogador")
	campaign := h.newCampaign(master, "Mirathel", p)
	feat := master.addEntry(t, campaign, &rulesv1.TableFeat{
		NamePt: "Dom de teste", DescPt: []string{"Um dom de teste."},
		Effects: []*rulesv1.TableEffect{
			{Type: "granted_spell", Spell: "spell:guiding-bolt", Ability: "wis"},
			{Type: "granted_spell", Spell: "spell:sacred-flame", Ability: "wis"},
		},
	})
	// A level 5 fighter casts nothing of his own: proficiency +3, Wisdom 16 (+3).
	sheet := casterSheet("class:fighter", "subclass:champion", 5, 16, 12, 14, 10, 16, 8, nil, nil, "feature:fighter-fighting-style-defense")
	ch := p.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Torin", sheet)
	// A player's feats come from the level-up; the master adds one to a sheet.
	sheet.GetFull().FeatKeys = []string{feat.GetKey()}
	var err error
	if ch, err = master.update(t, ch, "Torin", sheet); err != nil {
		t.Fatalf("UpdateCharacter(feat) error = %v", err)
	}

	tx, err := h.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.WithoutCancel(t.Context())) })

	bolt, err := h.svc.GrantedCombatSpell(t.Context(), tx, campaign, ch.GetId(), "spell:guiding-bolt", feat.GetKey(), 1, "")
	if err != nil {
		t.Fatalf("GrantedCombatSpell(guiding bolt) error = %v", err)
	}
	if bolt.ToHit != 6 {
		t.Errorf("attack bonus = %+d, want +6 (proficiency 3 and Wisdom 3)", bolt.ToHit)
	}
	flame, err := h.svc.GrantedCombatSpell(t.Context(), tx, campaign, ch.GetId(), "spell:sacred-flame", feat.GetKey(), 0, "")
	if err != nil {
		t.Fatalf("GrantedCombatSpell(sacred flame) error = %v", err)
	}
	if flame.SaveDC != 14 || flame.SaveAbility != "dex" {
		t.Errorf("save = %s DC %d, want a Dexterity save against 14 (8 + 3 + 3)", flame.SaveAbility, flame.SaveDC)
	}
	// A cantrip grows with the character's level, not a class's: level 5 deals 2d8.
	if got := damagesOf(flame); len(got) != 1 || got[0] != "2d8+0 damage-type:radiant" {
		t.Errorf("sacred flame damage = %v, want 2d8 radiant at level 5", got)
	}

	// The same spell with no grant is the classes' own: a fighter has no caster numbers.
	own, err := h.svc.CombatSpell(t.Context(), tx, campaign, ch.GetId(), "spell:sacred-flame", 0, "")
	if err != nil {
		t.Fatalf("CombatSpell() error = %v", err)
	}
	if own.SaveDC != 0 {
		t.Errorf("a fighter's own sacred flame has DC %d, want 0 (he casts nothing)", own.SaveDC)
	}

	// A grant the character does not have is not found.
	if _, err := h.svc.GrantedCombatSpell(t.Context(), tx, campaign, ch.GetId(), "spell:guiding-bolt", "feat:outro@mesa", 1, ""); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("a grant the sheet does not have: code = %v, want not_found", connect.CodeOf(err))
	}
}
