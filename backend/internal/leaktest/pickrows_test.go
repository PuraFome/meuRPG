package leaktest

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1/charactersv1connect"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// risingWizard is a wizard of Ana's at level 1 with the experience of level 2.
func (pw *pickWorld) risingWizard(skills ...string) (*charactersv1.Character, func()) {
	s := sheet("class:wizard", 1)
	s.GetFull().ExperiencePoints = risingExperience
	s.GetFull().SkillProficiencyKeys = skills
	return pw.stored(s)
}

// A level that gains no pick refuses a new skill, on the level-up itself: the table has no feature that asks one, so the skill
// the request adds is refused, and the same request without it is accepted.
func TestALevelThatGainsNoPickRefusesANewSkillOnTheLevelUp(t *testing.T) {
	pw := newPickWorld(t, false)
	w := pw.w
	rising, done := pw.risingWizard()
	defer done()
	levelUp := func(skills ...string) error {
		_, err := w.ana.characters.LevelUpCharacter(t.Context(), rq(&charactersv1.LevelUpCharacterRequest{
			CampaignId: w.campaign, CharacterId: rising.GetId(), Revision: rising.GetRevision(), Choices: levelUpChoices(skills, nil),
		}))
		return err
	}
	if err := levelUp(pickInside); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("the level-up with a new skill answers %v, want a refusal", err)
	}
	if err := levelUp(); err != nil {
		t.Errorf("the level-up with no skill answers %v, the positive control", err)
	}
}

// A tool pick of a class feature sent with a level-up is a key the level does not offer a player: the answer is the refusal of
// the pick alone, with no sheet of afterwards.
func TestAClassFeaturesToolPickAtALevelUpIsAKeyTheLevelDoesNotOfferAPlayer(t *testing.T) {
	pw := newPickWorld(t, true)
	w := pw.w
	rising, done := pw.risingWizard()
	defer done()
	choices := levelUpChoices(nil, nil)
	choices.FeatureChoiceKeys = []string{rules.ScopedChoice(rules.ToolPickKey(pw.toolFeat), toolOutside)}
	res, err := w.ana.characters.PreviewLevelUp(t.Context(), rq(&charactersv1.PreviewLevelUpRequest{CampaignId: w.campaign, CharacterId: rising.GetId(), Choices: choices}))
	if err != nil {
		t.Fatalf("PreviewLevelUp: %v", err)
	}
	r := res.Msg.GetRefusal()
	if r.GetField() != "choices.feature_choice_keys" || r.GetReason() != charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_FEATURE_CHOICE || res.Msg.GetAfter() != nil {
		t.Errorf("the refusal is %v and the sheet of afterwards %v, want the refusal of the pick alone", r, res.Msg.GetAfter() != nil)
	}
}

// The master's preview of a level-up reads a tool pick that is no tool, or of an owner that does not exist, as a key the content does not have: the check of the plan refuses it, as the save of a sheet does.
func TestTheMastersPreviewOfALevelUpRefusesAToolPickThatIsNoKnownKey(t *testing.T) {
	pw := newPickWorld(t, false)
	w := pw.w
	for name, key := range map[string]string{
		"an owner that does not exist": rules.ScopedChoice(rules.ToolPickKey(absentToolOwner), toolInside),
		"a value that is no tool":      rules.ScopedChoice(rules.ToolPickKey(absentToolOwner), "skill:arcana"),
	} {
		rising, done := pw.risingWizard()
		choices := levelUpChoices(nil, nil)
		choices.FeatureChoiceKeys = []string{key}
		res, err := w.master.characters.PreviewLevelUp(t.Context(), rq(&charactersv1.PreviewLevelUpRequest{CampaignId: w.campaign, CharacterId: rising.GetId(), Choices: choices}))
		done()
		if connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), "feature_choice_keys") || res != nil {
			t.Errorf("%s: the master's preview answers %v, want the refusal of the key that is no known key", name, err)
		}
	}
}

// answersOnExpertise are the replies of a player to every call that takes an expertise pick of a class feature (the wizard's,
// at level 2, on arcana), for the pick inside its list and one outside it.
func (pw *pickWorld) answersOnExpertise() map[string]string {
	w := pw.w
	out := map[string]string{}
	held := []string{"skill:arcana", "skill:history"}
	draft := func(expertise string) *charactersv1.CharacterSheet {
		s := sheet("class:wizard", skillFeatureLevel)
		s.GetFull().SkillProficiencyKeys, s.GetFull().ExpertiseSkillKeys = held, []string{expertise}
		return s
	}
	for _, x := range []string{"skill:arcana", "skill:history"} {
		stored, done := pw.stored(sheet("class:wizard", skillFeatureLevel))
		out["PreviewChoices "+x] = pw.plain(w.ana.call(charactersv1connect.CharacterServicePreviewChoicesProcedure, &charactersv1.PreviewChoicesRequest{
			CampaignId: w.campaign, CharacterId: stored.GetId(), Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Sheet: draft(x),
		}))
		out["UpdateCharacter "+x] = pw.plain(w.ana.call(charactersv1connect.CharacterServiceUpdateCharacterProcedure, &charactersv1.UpdateCharacterRequest{
			CampaignId: w.campaign, CharacterId: stored.GetId(), Revision: stored.GetRevision(), Name: stored.GetName(), Sheet: draft(x),
		}))
		done()
		out["CreateCharacter "+x] = pw.plain(pw.createdBy(draft(x)))
		rising, done := pw.risingWizard(held...)
		out["GetLevelUpOptions "+x] = pw.plain(w.ana.call(charactersv1connect.CharacterServiceGetLevelUpOptionsProcedure, &charactersv1.GetLevelUpOptionsRequest{
			CampaignId: w.campaign, CharacterId: rising.GetId(), ClassKey: "class:wizard",
		}))
		out["LevelUpCharacter "+x] = pw.plain(w.ana.call(charactersv1connect.CharacterServiceLevelUpCharacterProcedure, &charactersv1.LevelUpCharacterRequest{
			CampaignId: w.campaign, CharacterId: rising.GetId(), Revision: rising.GetRevision(), Choices: levelUpChoices(nil, []string{x}),
		}))
		done()
	}
	return out
}

// A class feature the master archived that gives expertise is answered as an open one, whole answers, on every call that takes
// an expertise pick.
func TestAnArchivedClassFeatureIsAnsweredAsAnOpenOneOnEveryCallThatTakesExpertise(t *testing.T) {
	pw := newPickWorld(t, false)
	feature := must(pw.w.master.table.CreateTableEntry(t.Context(), rq(&rulesv1.CreateTableEntryRequest{
		CampaignId: pw.w.campaign, Body: &rulesv1.CreateTableEntryRequest_TableClassFeature{TableClassFeature: &rulesv1.TableClassFeature{
			NamePt: "Estudo da mesa", DescPt: []string{"Texto de teste."}, ClassKey: "class:wizard", Level: skillFeatureLevel,
			Effects: []*rulesv1.TableEffect{{Type: "choice", Choice: "expertise", Count: 1, From: []string{"skill:arcana"}}},
		}},
	}))).GetEntry().GetKey()
	pw.skillFeat = feature
	open := pw.answersOnExpertise()
	for _, name := range []string{"CreateCharacter skill:arcana", "LevelUpCharacter skill:arcana", "UpdateCharacter skill:arcana"} {
		if !accepted(open[name]) {
			t.Fatalf("with the feature open, %s: %s, want it accepted (the positive control)", name, shorten([]byte(open[name])))
		}
	}
	for _, name := range []string{"CreateCharacter skill:history", "LevelUpCharacter skill:history", "UpdateCharacter skill:history"} {
		if accepted(open[name]) {
			t.Fatalf("with the feature open, %s is accepted: the pick is off the list", name)
		}
	}
	must(pw.w.master.table.ArchiveTableEntry(t.Context(), rq(&rulesv1.ArchiveTableEntryRequest{CampaignId: pw.w.campaign, Key: feature})))
	compareAnswers(t, "the feature archived", pw.answersOnExpertise(), open)
}

// A table class the master closed, archived or switched off, is answered as an open one to a player whose sheet holds the
// class: the lists of the choices preview and the save of a skill, for a pick inside the class feature's list and one outside.
func TestAClosedTableClassTheSheetHoldsIsAnsweredAsAnOpenOne(t *testing.T) {
	for _, how := range []string{"archived", "switched off"} {
		t.Run(how, func(t *testing.T) {
			p := newChoicesProbe(t)
			w := p.w
			pw := &pickWorld{w: w}
			class := p.createClass(tableClass("Cantor da mesa", &rulesv1.TableFeature{
				NamePt: "Canto da mesa", DescPt: []string{"Texto de teste."},
				Effects: []*rulesv1.TableEffect{{Type: "choice", Choice: "skill", Count: 1, From: []string{pickInside, "skill:deception"}}},
			}))
			// A closed class cannot be taken by a new sheet: both characters are made while it is open, and each round is read by
			// one of them (an accepted save moves its character).
			held := map[*person]*charactersv1.Character{}
			for _, who := range []*person{w.ana, w.caio} {
				held[who] = must(who.characters.CreateCharacter(t.Context(), rq(&charactersv1.CreateCharacterRequest{
					CampaignId: w.campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: who.name, Sheet: sheet(class, 1),
				}))).GetCharacter()
			}
			answers := func(who *person) map[string]string {
				out := map[string]string{}
				stored := held[who]
				for _, x := range []string{pickOutside, pickInside} {
					draft := sheet(class, 1)
					draft.GetFull().SkillProficiencyKeys = []string{x}
					out["PreviewChoices "+x] = pw.plain(who.call(charactersv1connect.CharacterServicePreviewChoicesProcedure, &charactersv1.PreviewChoicesRequest{
						CampaignId: w.campaign, CharacterId: stored.GetId(), Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Sheet: draft,
					}))
					out["UpdateCharacter "+x] = pw.plain(who.call(charactersv1connect.CharacterServiceUpdateCharacterProcedure, &charactersv1.UpdateCharacterRequest{
						CampaignId: w.campaign, CharacterId: stored.GetId(), Revision: stored.GetRevision(), Name: stored.GetName(), Sheet: draft,
					}))
				}
				for k, v := range out {
					out[k] = strings.ReplaceAll(v, who.name, "WHO")
				}
				return out
			}
			open := answers(w.ana)
			if !accepted(open["UpdateCharacter "+pickInside]) || accepted(open["UpdateCharacter "+pickOutside]) {
				t.Fatalf("with the class open the save of %s is %s and of %s %s: the row measures nothing", pickInside, shorten([]byte(open["UpdateCharacter "+pickInside])), pickOutside, shorten([]byte(open["UpdateCharacter "+pickOutside])))
			}
			if how == "archived" {
				must(w.master.table.ArchiveTableEntry(t.Context(), rq(&rulesv1.ArchiveTableEntryRequest{CampaignId: w.campaign, Key: class})))
			} else {
				must(w.master.table.SetOptionSwitches(t.Context(), rq(&rulesv1.SetOptionSwitchesRequest{CampaignId: w.campaign, Switches: []*rulesv1.OptionSwitch{{Key: class, Off: true}}})))
			}
			compareAnswers(t, "the class "+how, answers(w.caio), open)
		})
	}
}

// The master's save of a skill under a class feature the master archived is kept, inside the feature's list or outside it (the
// fit issue tells the second).
func TestTheMastersSaveOfASkillUnderAnArchivedClassFeatureIsKept(t *testing.T) {
	pw := newPickWorld(t, true)
	pw.archive(t)
	for _, skill := range []string{pickInside, pickOutside} {
		s := sheet("class:wizard", skillFeatureLevel)
		s.GetFull().SkillProficiencyKeys = []string{skill}
		r := pw.w.master.call(charactersv1connect.CharacterServiceCreateCharacterProcedure, &charactersv1.CreateCharacterRequest{
			CampaignId: pw.w.campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, Name: "Mestre " + skill, Sheet: s,
		})
		if !r.ok() {
			t.Errorf("the master's save of %s: HTTP %d %s, want it kept", skill, r.status, shorten(r.body))
		}
	}
}
