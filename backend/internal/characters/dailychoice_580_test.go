package characters

import (
	"slices"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
)

// #580 S3 through the service: the pick of a daily choice is written on a locked sheet, by the owner and by the master,
// and the rests clear it; a fixed proficiency until the short rest is spent by it and given back by the long rest.

func dailyHarness(t *testing.T) (h *harness, master, player *user, campaign string, pc *charactersv1.Character, choice string) {
	t.Helper()
	h = newHarness(t)
	master, player = h.newUser("Samuel"), h.newUser("Dona")
	campaign = h.newCampaign(master, "Mirathel", player)
	feat := master.addEntry(t, campaign, &rulesv1.TableFeat{
		NamePt: "Repertório do Dia", DescPt: []string{"Texto."},
		Effects: []*rulesv1.TableEffect{
			{Type: "choice", Choice: "skill", Count: 1, DailyChoice: true, Duration: "long_rest"},
			{Type: "choice", Choice: "tool", Count: 1, DailyChoice: true, Duration: "short_rest"},
			{Type: "proficiency", Proficiency: "skill:perception", Duration: "short_rest"},
		},
	})
	sheet := pensantusSheet()
	sheet.GetFull().SkillProficiencyKeys = nil
	pc = player.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus", sheet)
	// A player's feats come from the level-up; the master adds one to a sheet.
	sheet.GetFull().FeatKeys = []string{feat.GetKey()}
	var err error
	if pc, err = master.update(t, pc, "Pensantus", sheet); err != nil {
		t.Fatalf("UpdateCharacter(feat) error = %v", err)
	}
	return h, master, player, campaign, pc, feat.GetKey() + "#0"
}

func (u *user) pick(t *testing.T, campaign, character, choice, key string) error {
	t.Helper()
	_, err := u.api.SetDailyChoice(t.Context(), connect.NewRequest(&charactersv1.SetDailyChoiceRequest{CampaignId: campaign, CharacterId: character, ChoiceId: choice, Key: key}))
	return err
}

func skillOfSheet(c *charactersv1.Character, key string) *rulesv1.DerivedSkill {
	for _, s := range c.GetDerived().GetSkills() {
		if s.GetKey() == key {
			return s
		}
	}
	return nil
}

func takeRest(t *testing.T, h *harness, campaign string, kind playv1.RestKind) {
	t.Helper()
	err := db.InTx(t.Context(), h.pool, func(tx pgx.Tx) error {
		_, _, err := h.svc.TakeRest(t.Context(), tx, campaign, &playv1.TakeRestRequest{Kind: kind})
		return err
	})
	if err != nil {
		t.Fatalf("TakeRest(%v) error = %v", kind, err)
	}
}

func TestADailyChoiceIsPickedOnALockedSheetAndTheRestsClearIt(t *testing.T) {
	t.Parallel()
	h, master, player, campaign, pc, choice := dailyHarness(t)
	if h.lockSheets(campaign) != 1 {
		t.Fatal("expected the sheet to lock")
	}
	if got := player.get(t, campaign, pc.GetId()).GetDerived().GetDailyChoices(); len(got) != 2 || got[0].GetPicked() != "" {
		t.Fatalf("daily choices = %v, want two waiting", got)
	}

	if err := player.pick(t, campaign, pc.GetId(), choice, "skill:stealth"); err != nil {
		t.Fatalf("the owner's pick on a locked sheet: %v", err)
	}
	if s := skillOfSheet(player.get(t, campaign, pc.GetId()), "skill:stealth"); s.GetProficiency() != rulesv1.ProficiencyLevel_PROFICIENCY_LEVEL_PROFICIENT || s.GetUntilRest() != rulesv1.RestBound_REST_BOUND_LONG {
		t.Errorf("stealth after the pick = %v, want full proficiency until the long rest", s)
	}
	if err := master.pick(t, campaign, pc.GetId(), choice, "skill:arcana"); err != nil {
		t.Fatalf("the master's pick: %v", err)
	}
	for name, bad := range map[string]struct{ choice, key string }{
		"a key the choice does not offer": {choice, "skill:nada"},
		"a choice the sheet lacks":        {"feat:nada#0", "skill:stealth"},
	} {
		if err := player.pick(t, campaign, pc.GetId(), bad.choice, bad.key); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: error = %v, want invalid_argument", name, err)
		}
	}
	if err := h.newUser("Intrusa").pick(t, campaign, pc.GetId(), choice, "skill:stealth"); err == nil {
		t.Error("a stranger picked for the sheet")
	}

	// A short rest spends the fixed proficiency but keeps the long-rest pick; the long rest ends the pick and gives it back.
	takeRest(t, h, campaign, playv1.RestKind_REST_KIND_SHORT)
	got := player.get(t, campaign, pc.GetId())
	if s := skillOfSheet(got, "skill:perception"); s.GetProficiency() != rulesv1.ProficiencyLevel_PROFICIENCY_LEVEL_NONE {
		t.Errorf("perception after the short rest = %v, want it spent", s)
	}
	if s := skillOfSheet(got, "skill:arcana"); s.GetUntilRest() != rulesv1.RestBound_REST_BOUND_LONG {
		t.Errorf("a pick until the long rest after a short rest = %v, want it kept", s)
	}
	takeRest(t, h, campaign, playv1.RestKind_REST_KIND_LONG)
	got = player.get(t, campaign, pc.GetId())
	if s := skillOfSheet(got, "skill:perception"); s.GetProficiency() != rulesv1.ProficiencyLevel_PROFICIENCY_LEVEL_PROFICIENT {
		t.Errorf("perception after the long rest = %v, want it back", s)
	}
	if s := skillOfSheet(got, "skill:arcana"); s.GetUntilRest() != rulesv1.RestBound_REST_BOUND_UNSPECIFIED || slices.ContainsFunc(got.GetDerived().GetDailyChoices(), func(c *rulesv1.DailyChoice) bool { return c.GetPicked() != "" }) {
		t.Errorf("after the long rest the pick should be gone: %v %v", s, got.GetDerived().GetDailyChoices())
	}
}

// A save never writes the picks: the client's copy of them is dropped and the stored ones kept.
func TestASaveKeepsTheStoredDailyPicks(t *testing.T) {
	t.Parallel()
	_, master, player, campaign, pc, choice := dailyHarness(t)
	if err := player.pick(t, campaign, pc.GetId(), choice, "skill:stealth"); err != nil {
		t.Fatal(err)
	}
	cur := master.get(t, campaign, pc.GetId())
	sheet := cur.GetSheet()
	sheet.GetFull().DailyPicks = []*charactersv1.DailyPick{{ChoiceId: choice, Key: "skill:arcana", UntilRest: rulesv1.RestBound_REST_BOUND_LONG}}
	sheet.GetFull().ShortRested = true
	if _, err := master.update(t, cur, "Pensantus", sheet); err != nil {
		t.Fatalf("UpdateCharacter() error = %v", err)
	}
	got := master.get(t, campaign, pc.GetId())
	if got.GetSheet().GetFull().GetShortRested() || len(got.GetSheet().GetFull().GetDailyPicks()) != 1 || got.GetSheet().GetFull().GetDailyPicks()[0].GetKey() != "skill:stealth" {
		t.Errorf("the saved sheet's picks = %v (short rested %v), want the stored stealth pick and no mark", got.GetSheet().GetFull().GetDailyPicks(), got.GetSheet().GetFull().GetShortRested())
	}
}
