package play

import (
	"fmt"
	"slices"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// magicItemWith reads the sheet entry of the key on the character's sheet as the master reads it.
func magicItemWith(t *testing.T, a *armed, c *charactersv1.Character, key string) *charactersv1.SheetMagicItem {
	t.Helper()
	cur, err := a.master.characters.GetCharacter(t.Context(), connect.NewRequest(&charactersv1.GetCharacterRequest{CampaignId: a.campaignID, CharacterId: c.GetId()}))
	if err != nil {
		t.Fatalf("GetCharacter() error = %v", err)
	}
	for _, it := range cur.Msg.GetCharacter().GetSheet().GetFull().GetMagicItems() {
		if it.GetKey() == key {
			return it
		}
	}
	t.Fatalf("%s is not on the sheet", key)
	return nil
}

// Drinking each healing potion rolls within the dice the sheet says for it: the faces the sheet's field asks for are accepted, one
// more is refused, and the total is the field's bonus plus the faces.
func TestEachHealingPotionRollsTheDiceTheSheetSays(t *testing.T) {
	t.Parallel()
	for _, key := range []string{potionCommon, potionGreater, potionSuperior, potionSupreme} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			s := newStocked(t, key)
			field := magicItemWith(t, s.armed, s.toren, key).GetConsumable()
			count, sides, bonus := field.GetHealDiceCount(), field.GetHealDiceSides(), field.GetHealBonus()
			if count == 0 || sides == 0 {
				t.Fatalf("the sheet says no healing for %s: %v", key, field)
			}
			faces := make([]int32, count)
			for i := range faces {
				faces[i] = sides
			}
			if _, err := s.use(t, s.caio, s.toren, key, typedDice(append(slices.Clone(faces), 1)...)); connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Fatalf("one die more than the sheet says: %v, want invalid_argument", err)
			}
			res := s.mustUse(t, s.caio, s.toren, key, typedDice(faces...))
			if want := count*sides + bonus; res.GetRollTotal() != want {
				t.Errorf("max roll = %d, want %d", res.GetRollTotal(), want)
			}
			if want := fmt.Sprintf("%dd%d+%d", count, sides, bonus); res.GetDiceText() != want {
				t.Errorf("dice text = %q, want %q", res.GetDiceText(), want)
			}
		})
	}
}

// The effect a resistance potion leaves reads with its own title, the length of the hour and the chip of what it resists.
func TestAResistancePotionsEffectIsTitledAndLongAndCarriesItsChip(t *testing.T) {
	t.Parallel()
	s := newStocked(t, potionFire)
	s.mustUse(t, s.ana, s.pens, potionFire)
	for name, u := range map[string]*user{"master": s.master, "owner": s.ana} {
		res, err := u.lasting.ListCharacterEffects(t.Context(), connect.NewRequest(&playv1.ListCharacterEffectsRequest{CampaignId: s.campaignID}))
		if err != nil {
			t.Fatalf("%s: ListCharacterEffects() error = %v", name, err)
		}
		var got *playv1.CharacterEffect
		for _, e := range res.Msg.GetEffects() {
			if e.GetCharacterId() == s.pens.GetId() && e.GetSourceKey() == potionFire {
				got = e
			}
		}
		if got == nil {
			t.Fatalf("%s reads no effect", name)
		}
		if got.GetTitlePt() != "Resistência a fogo" || got.GetTotalDurationTextPt() != "por 1 hora" || !slices.Contains(got.GetTagsPt(), "Resistência: fogo") {
			t.Errorf("%s reads title %q, total %q, tags %v", name, got.GetTitlePt(), got.GetTotalDurationTextPt(), got.GetTagsPt())
		}
	}
}

// Every effect has a title, whatever its source: the source's name when the content gives the effect none. The total length is told
// only for the length the catalog gave: a master's own seconds, shorter or longer, never read as the catalog's.
func TestACharacterEffectIsTitledByItsSourceAndSaysItsFixedLength(t *testing.T) {
	t.Parallel()
	c, err := rules.LoadSRD()
	if err != nil {
		t.Fatal(err)
	}
	names := func(k string) string {
		return map[string]string{"spell:bless": "Bênção", "effect:dodging": "Esquiva", potionFire: "Poção de resistência a fogo"}[k]
	}
	secs := func(n int32) *int32 { return &n }
	caster := "c1"
	for name, tc := range map[string]struct {
		row          playdb.CharacterEffect
		title, total string
	}{
		"a spell cast by a character":              {playdb.CharacterEffect{SourceKey: "spell:bless", SourceKind: "spell", SourceCharacterID: &caster, SecondsLeft: secs(40)}, "Bênção", "por 1 minuto"},
		"a spell the master gave a shorter length": {playdb.CharacterEffect{SourceKey: "spell:bless", SourceKind: "spell", SecondsLeft: secs(30)}, "Bênção", ""},
		"a spell the master gave a longer length":  {playdb.CharacterEffect{SourceKey: "spell:bless", SourceKind: "spell", SecondsLeft: secs(7200)}, "Bênção", ""},
		"a concentration":                          {playdb.CharacterEffect{SourceKey: "spell:bless", SourceKind: "spell", SourceCharacterID: &caster, Concentration: true}, "Bênção", ""},
		"until dismissed":                          {playdb.CharacterEffect{SourceKey: "effect:dodging", SourceKind: "master"}, "Esquiva", ""},
		"a potion drunk":                           {playdb.CharacterEffect{SourceKey: potionFire, SourceKind: "item", SecondsLeft: secs(600)}, "Resistência a fogo", "por 1 hora"},
	} {
		e := characterEffectProto(tc.row, "Toren", names, c, true, true)
		if e.GetTitlePt() != tc.title || e.GetTotalDurationTextPt() != tc.total {
			t.Errorf("%s: title %q total %q, want %q %q", name, e.GetTitlePt(), e.GetTotalDurationTextPt(), tc.title, tc.total)
		}
	}
}
