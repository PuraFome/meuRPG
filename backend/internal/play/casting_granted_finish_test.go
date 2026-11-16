package play

import (
	"fmt"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// A long casting through a granted_spell effect finishes at the cast level of the grant it started through (MR-025),
// also for a spell the character has prepared. These tests need the database (MEURPG_TEST_DATABASE_URL).

// newClericGrants is newCasters where Brisa (a cleric with Prayer of Healing prepared) has one feat for each of the
// cast levels: each feat grants Prayer of Healing at that level, at will, through Wisdom. It returns the feat keys in order.
func newClericGrants(t *testing.T, castLevels ...int32) (a *armed, feats []string) {
	t.Helper()
	a = newCastingParty(t)
	for _, lvl := range castLevels {
		res, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(&rulesv1.CreateTableEntryRequest{
			CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableFeat{TableFeat: &rulesv1.TableFeat{
				NamePt: fmt.Sprintf("Prece de teste %d", lvl), DescPt: []string{"Um dom de teste."},
				Effects: []*rulesv1.TableEffect{{Type: "granted_spell", Spell: prayerKey, Ability: "wis", CastLevel: lvl}},
			}},
		}))
		if err != nil {
			t.Fatalf("CreateTableEntry(feat) error = %v", err)
		}
		feats = append(feats, res.Msg.GetEntry().GetKey())
	}
	sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores: &rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 16, Charisma: 8},
		RaceKey:    "race:human", Classes: []*charactersv1.ClassLevel{classLevel("class:cleric", 3, "subclass:life")},
		CantripKeys: []string{sacredFlame}, PreparedSpellKeys: []string{cureWounds, healingWord, prayerKey, detectMagicKey, blessKey, aidSpell},
		FeatKeys: feats,
	}}}
	upd, err := a.master.characters.UpdateCharacter(t.Context(), connect.NewRequest(&charactersv1.UpdateCharacterRequest{
		CampaignId: a.campaignID, CharacterId: a.bri.GetId(), Revision: a.bri.GetRevision(), Name: "Brisa", Sheet: sheet,
	}))
	if err != nil {
		t.Fatalf("UpdateCharacter(Brisa gets the feats) error = %v", err)
	}
	a.bri = upd.Msg.GetCharacter()
	return a, feats
}

// prayerAt starts a long casting of Prayer of Healing through the grant, finishes it as the master and says how many
// dice it healed with, and that it spent no slot.
func (a *armed) prayerAt(t *testing.T, feat string) int32 {
	t.Helper()
	start := a.mustCastOut(t, a.bia, a.bri, prayerKey, nil, false, []*charactersv1.Character{a.toren}, grantedBy(feat)).GetCast()
	if start.GetStatus() != playv1.OutsideCastStatus_OUTSIDE_CAST_STATUS_CASTING {
		t.Fatalf("cast = %v, want a casting that goes on", start)
	}
	a.hurt(t, a.toren, 20)
	done, err := a.finish(t, a.master, start.GetId())
	if err != nil {
		t.Fatalf("ConfirmCastTimePassed() error = %v", err)
	}
	for lvl := int32(1); lvl <= 2; lvl++ {
		if usedSlots(a.vitals(t, a.bri), lvl) != 0 {
			t.Errorf("a slot of level %d was spent by a cast through a grant", lvl)
		}
	}
	return done.GetCast().GetDiceCount()
}

// A cleric who has Prayer of Healing prepared and a grant of it at the 4th level: the casting started through the
// grant finishes at the 4th level, not at the spell's own.
func TestMR025_ALongCastThroughAGrantOfAPreparedSpellFinishesAtTheGrantsLevel(t *testing.T) {
	t.Parallel()
	a, feats := newClericGrants(t, 4)
	if got := a.prayerAt(t, feats[0]); got != 4 {
		t.Errorf("dice of the finished casting = %d, want 4 (2d8 and one more for each level above the 2nd, at the 4th)", got)
	}
}

// Two grants of one spell from two table entries, at different cast levels: each long casting finishes at the level of
// the grant it started through.
func TestMR025_TwoGrantsOfOneSpellFinishEachAtItsOwnLevel(t *testing.T) {
	t.Parallel()
	a, feats := newClericGrants(t, 3, 5)
	if got := a.prayerAt(t, feats[1]); got != 5 {
		t.Errorf("dice of the casting through the second grant = %d, want 5", got)
	}
	if got := a.prayerAt(t, feats[0]); got != 3 {
		t.Errorf("dice of the casting through the first grant = %d, want 3", got)
	}
}
