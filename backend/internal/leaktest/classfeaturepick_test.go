package leaktest

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1/charactersv1connect"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The rows of a table class feature that gives a pick from a short list (M1): a sheet's sources are those of the entries it
// holds or gets, for every reader, so a feature the master archived is still given to a character of the class and answers
// as when it is open, and a feature the table switched off answers as a table that never had it.

// revisionText is the table's revision, which the archive and the switch move (the sheet's own content revision and the
// content version it is derived with): the one thing the answers of the two states may differ in.
var revisionText = regexp.MustCompile(`"contentRevision":\d+|\+fx\.\d+\+mesa\.\d+`)

const (
	pickInside, pickOutside       = "skill:performance", "skill:stealth"
	toolInside, toolOutside       = "proficiency:smiths-tools", "proficiency:glaives"
	toolOther                     = "proficiency:thieves-tools"
	absentToolOwner               = "feature:nenhum--dono@mesa"
	skillFeatureLevel             = 2
	toolFeatureLevel              = 1
	risingExperience        int32 = 300
)

// pickWorld is a campaign with a table class feature of the SRD wizard that gives a skill pick at level 2 and one of a tool at
// level 1 (or none, for the table without it), and the characters the rows call with: one set for each phase, because a call
// that is accepted moves its character.
type pickWorld struct {
	w                   *world
	skillFeat, toolFeat string // the keys of the features, empty when the table has none
	toolOwner           string // the owner the tool picks name: the feature's key, or one that does not exist
}

func newPickWorld(t *testing.T, withFeature bool) *pickWorld {
	t.Helper()
	p := newChoicesProbe(t)
	w, ctx := p.w, t.Context()
	must(w.master.campaigns.SetCampaignXpMode(ctx, rq(&campaignsv1.SetCampaignXpModeRequest{CampaignId: w.campaign, XpMode: campaignsv1.XpMode_XP_MODE_ENEMIES})))
	pw := &pickWorld{w: w, toolOwner: absentToolOwner}
	if withFeature {
		feature := func(name string, level int32, effect *rulesv1.TableEffect) string {
			return must(w.master.table.CreateTableEntry(ctx, rq(&rulesv1.CreateTableEntryRequest{
				CampaignId: w.campaign, Body: &rulesv1.CreateTableEntryRequest_TableClassFeature{TableClassFeature: &rulesv1.TableClassFeature{
					NamePt: name, DescPt: []string{"Texto de teste."}, ClassKey: "class:wizard", Level: level, Effects: []*rulesv1.TableEffect{effect},
				}},
			}))).GetEntry().GetKey()
		}
		pw.skillFeat = feature("Canto da mesa", skillFeatureLevel, &rulesv1.TableEffect{Type: "choice", Choice: "skill", Count: 1, From: []string{pickInside, "skill:deception"}})
		pw.toolFeat = feature("Oficio da mesa", toolFeatureLevel, &rulesv1.TableEffect{Type: "choice", Choice: "tool", Count: 1, From: []string{toolInside, toolOther}})
		pw.toolOwner = pw.toolFeat
	}
	return pw
}

// stored is a character of Ana's with the sheet, and the call that retires it (a player has one living character, and the
// next call needs the same sheet again).
func (pw *pickWorld) stored(s *charactersv1.CharacterSheet) (*charactersv1.Character, func()) {
	c := must(pw.w.ana.characters.CreateCharacter(pw.w.t.Context(), rq(&charactersv1.CreateCharacterRequest{
		CampaignId: pw.w.campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Perita", Sheet: s,
	}))).GetCharacter()
	return c, func() { pw.retire(c.GetId()) }
}

func (pw *pickWorld) retire(id string) {
	must(pw.w.master.characters.MarkCharacterDead(pw.w.t.Context(), rq(&charactersv1.MarkCharacterDeadRequest{CampaignId: pw.w.campaign, CharacterId: id})))
}

// createdBy is the CreateCharacter of the draft by Caio, and retires the character it made.
func (pw *pickWorld) createdBy(draft *charactersv1.CharacterSheet) reply {
	r := pw.w.caio.call(charactersv1connect.CharacterServiceCreateCharacterProcedure, &charactersv1.CreateCharacterRequest{
		CampaignId: pw.w.campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Nova", Sheet: draft,
	})
	if r.ok() {
		var res charactersv1.CreateCharacterResponse
		if err := protojson.Unmarshal(r.body, &res); err != nil {
			pw.w.t.Fatalf("CreateCharacter: %v", err)
		}
		pw.retire(res.GetCharacter().GetId())
	}
	return r
}

// archive and off close the two features the way the table can.
func (pw *pickWorld) archive(t *testing.T) {
	t.Helper()
	for _, key := range []string{pw.skillFeat, pw.toolFeat} {
		must(pw.w.master.table.ArchiveTableEntry(t.Context(), rq(&rulesv1.ArchiveTableEntryRequest{CampaignId: pw.w.campaign, Key: key})))
	}
}

func (pw *pickWorld) off(t *testing.T) {
	t.Helper()
	switches := []*rulesv1.OptionSwitch{{Key: pw.skillFeat, Off: true}, {Key: pw.toolFeat, Off: true}}
	must(pw.w.master.table.SetOptionSwitches(t.Context(), rq(&rulesv1.SetOptionSwitchesRequest{CampaignId: pw.w.campaign, Switches: switches})))
}

// plain is the answer without what only the call's own moment tells: the keys of the features, the ids and the times.
func (pw *pickWorld) plain(r reply) string {
	body := string(r.body)
	for _, key := range []string{pw.skillFeat, pw.toolOwner} {
		if key != "" {
			body = keyless([]byte(body), key)
		}
	}
	body = revisionText.ReplaceAllString(uuidText.ReplaceAllString(timeText.ReplaceAllString(body, "TIME"), "ID"), "REV")
	return body + " HTTP " + strconv.Itoa(r.status)
}

func levelUpChoices(skills, expertise []string) *charactersv1.LevelUpChoices {
	return &charactersv1.LevelUpChoices{
		ClassKey: "class:wizard", SubclassKey: "subclass:evocation", KnownSpellKeys: []string{"spell:magic-missile", "spell:shield"},
		SkillProficiencyKeys: skills, ExpertiseSkillKeys: expertise,
		HitPoints: &charactersv1.LevelUpHitPoints{Method: charactersv1.LevelUpHitPointsMethod_LEVEL_UP_HIT_POINTS_METHOD_AVERAGE},
	}
}

// answers are the replies of the players to every call that takes a pick of the feature, for a pick inside its list and one
// outside it, by the name of the call and the pick.
func (pw *pickWorld) answers() map[string]string {
	w := pw.w
	out := map[string]string{}
	put := func(name string, r reply) { out[name] = pw.plain(r) }
	skillDraft := func(skill string) *charactersv1.CharacterSheet {
		s := sheet("class:wizard", skillFeatureLevel)
		s.GetFull().SkillProficiencyKeys = []string{skill}
		return s
	}
	toolDraft := func(tool string) *charactersv1.CharacterSheet {
		return sheet("class:wizard", toolFeatureLevel, rules.ScopedChoice(rules.ToolPickKey(pw.toolOwner), tool))
	}
	for _, skill := range []string{pickInside, pickOutside} {
		held, done := pw.stored(sheet("class:wizard", 1))
		put("PreviewChoices "+skill, w.ana.call(charactersv1connect.CharacterServicePreviewChoicesProcedure, &charactersv1.PreviewChoicesRequest{
			CampaignId: w.campaign, CharacterId: held.GetId(), Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Sheet: skillDraft(skill),
		}))
		done()
		put("CreateCharacter "+skill, pw.createdBy(skillDraft(skill)))
		rs := sheet("class:wizard", 1)
		rs.GetFull().ExperiencePoints = risingExperience
		rising, done := pw.stored(rs)
		put("GetLevelUpOptions "+skill, w.ana.call(charactersv1connect.CharacterServiceGetLevelUpOptionsProcedure, &charactersv1.GetLevelUpOptionsRequest{
			CampaignId: w.campaign, CharacterId: rising.GetId(), ClassKey: "class:wizard",
		}))
		put("LevelUpCharacter "+skill, w.ana.call(charactersv1connect.CharacterServiceLevelUpCharacterProcedure, &charactersv1.LevelUpCharacterRequest{
			CampaignId: w.campaign, CharacterId: rising.GetId(), Revision: rising.GetRevision(), Choices: levelUpChoices([]string{skill}, nil),
		}))
		done()
	}
	for _, tool := range []string{toolInside, toolOutside} {
		held, done := pw.stored(sheet("class:wizard", 1))
		put("tool PreviewChoices "+tool, w.ana.call(charactersv1connect.CharacterServicePreviewChoicesProcedure, &charactersv1.PreviewChoicesRequest{
			CampaignId: w.campaign, CharacterId: held.GetId(), Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Sheet: toolDraft(tool),
		}))
		put("tool UpdateCharacter "+tool, w.ana.call(charactersv1connect.CharacterServiceUpdateCharacterProcedure, &charactersv1.UpdateCharacterRequest{
			CampaignId: w.campaign, CharacterId: held.GetId(), Revision: held.GetRevision(), Name: held.GetName(), Sheet: toolDraft(tool),
		}))
		done()
		put("tool CreateCharacter "+tool, pw.createdBy(toolDraft(tool)))
	}
	return out
}

func accepted(answer string) bool { return strings.HasSuffix(answer, "HTTP 200") }

func compareAnswers(t *testing.T, what string, got, want map[string]string) {
	t.Helper()
	for name, w := range want {
		if got[name] != w {
			i := 0
			for i < len(w) && i < len(got[name]) && w[i] == got[name][i] {
				i++
			}
			t.Errorf("%s, %s differ at %d:\n...%s\nwant:\n...%s", what, name, i, got[name][max(i-80, 0):min(i+240, len(got[name]))], w[max(i-80, 0):min(i+240, len(w))])
		}
	}
	if len(got) != len(want) {
		t.Errorf("%s: %d answers, want %d", what, len(got), len(want))
	}
}

// A class feature the master archived is still given to a character of the class, so for a player every call that takes a
// skill or a tool pick answers as when the feature is open: for a pick inside its list and for one outside it.
func TestAnArchivedClassFeatureIsAnsweredAsAnOpenOneOnEveryCallThatTakesAPick(t *testing.T) {
	pw := newPickWorld(t, true)
	open := pw.answers()
	for _, name := range []string{"CreateCharacter " + pickInside, "LevelUpCharacter " + pickInside, "tool CreateCharacter " + toolInside, "tool UpdateCharacter " + toolInside} {
		if !accepted(open[name]) {
			t.Fatalf("with the feature open, %s: %s, want it accepted (the positive control)", name, shorten([]byte(open[name])))
		}
	}
	for _, name := range []string{"CreateCharacter " + pickOutside, "LevelUpCharacter " + pickOutside, "tool CreateCharacter " + toolOutside, "tool UpdateCharacter " + toolOutside} {
		if accepted(open[name]) {
			t.Fatalf("with the feature open, %s is accepted: the pick is off the list", name)
		}
	}
	pw.archive(t)
	compareAnswers(t, "the feature archived", pw.answers(), open)
}

// A class feature the table switched off is no source: every call that takes one of its picks answers as a table that never
// had the feature, and a tool pick under it as a key that does not exist.
func TestASwitchedOffClassFeatureIsAnsweredAsATableWithoutIt(t *testing.T) {
	without := newPickWorld(t, false).answers()
	for _, name := range []string{"CreateCharacter " + pickInside, "LevelUpCharacter " + pickInside, "tool CreateCharacter " + toolInside} {
		if accepted(without[name]) {
			t.Fatalf("without the feature, %s is accepted: the row measures nothing", name)
		}
	}
	pw := newPickWorld(t, true)
	if open := pw.answers(); !accepted(open["CreateCharacter "+pickInside]) {
		t.Fatalf("with the feature open the pick of %s is answered %s (the positive control)", pickInside, shorten([]byte(open["CreateCharacter "+pickInside])))
	}
	pw.off(t)
	compareAnswers(t, "the feature switched off", pw.answers(), without)
}

// skillsOfTheSRD are the keys of every skill the SRD has.
func skillsOfTheSRD(t *testing.T) []string {
	t.Helper()
	srd, err := rules.LoadSRD()
	if err != nil {
		t.Fatalf("LoadSRD: %v", err)
	}
	var out []string
	for _, s := range srd.Catalog().Skills {
		out = append(out, s.Key)
	}
	if len(out) != 18 {
		t.Fatalf("the SRD has %d skills, want 18", len(out))
	}
	return out
}

// At a level-up that gains a class feature and asks one new pick, the skill (or the expertise) is accepted if and only if the
// allowed list the player was given holds it, for every skill of the SRD, and the list is neither empty nor every skill.
func TestEverySkillOfTheSRDIsTakenAtALevelUpIffTheAllowedListHoldsIt(t *testing.T) {
	for _, kind := range []string{"skill", "expertise"} {
		t.Run(kind, func(t *testing.T) {
			p := newChoicesProbe(t)
			w, ctx := p.w, t.Context()
			must(w.master.campaigns.SetCampaignXpMode(ctx, rq(&campaignsv1.SetCampaignXpModeRequest{CampaignId: w.campaign, XpMode: campaignsv1.XpMode_XP_MODE_ENEMIES})))
			from := []string{pickInside, "skill:deception", "skill:persuasion"}
			if kind == "expertise" {
				from = []string{"skill:arcana"}
			}
			feature := must(w.master.table.CreateTableEntry(ctx, rq(&rulesv1.CreateTableEntryRequest{
				CampaignId: w.campaign, Body: &rulesv1.CreateTableEntryRequest_TableClassFeature{TableClassFeature: &rulesv1.TableClassFeature{
					NamePt: "Estudo da mesa", DescPt: []string{"Texto de teste."}, ClassKey: "class:wizard", Level: 2,
					Effects: []*rulesv1.TableEffect{{Type: "choice", Choice: kind, Count: 1, From: from}},
				}},
			}))).GetEntry()
			// The feature is archived: the list a player is given is the one the level-up takes.
			must(w.master.table.ArchiveTableEntry(ctx, rq(&rulesv1.ArchiveTableEntryRequest{CampaignId: w.campaign, Key: feature.GetKey()})))
			fresh := func() *charactersv1.Character {
				s := sheet("class:wizard", 1)
				s.GetFull().ExperiencePoints = risingExperience
				s.GetFull().SkillProficiencyKeys = []string{"skill:arcana", "skill:history"}
				return must(w.ana.characters.CreateCharacter(ctx, rq(&charactersv1.CreateCharacterRequest{
					CampaignId: w.campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Perita", Sheet: s,
				}))).GetCharacter()
			}
			rising := fresh()
			r := w.ana.call(charactersv1connect.CharacterServiceGetLevelUpOptionsProcedure, &charactersv1.GetLevelUpOptionsRequest{CampaignId: w.campaign, CharacterId: rising.GetId(), ClassKey: "class:wizard"})
			if !r.ok() {
				t.Fatalf("GetLevelUpOptions: HTTP %d %s", r.status, shorten(r.body))
			}
			var res charactersv1.GetLevelUpOptionsResponse
			if err := protojson.Unmarshal(r.body, &res); err != nil {
				t.Fatalf("GetLevelUpOptions: %v", err)
			}
			allowed := res.GetOptions().GetSkillsAllowed()
			if kind == "expertise" {
				allowed = res.GetOptions().GetExpertiseAllowed()
			}
			all := skillsOfTheSRD(t)
			if len(allowed) == 0 || len(allowed) >= len(all) {
				t.Fatalf("the allowed list is %v: the row measures nothing", allowed)
			}
			for _, skill := range all {
				var choices *charactersv1.LevelUpChoices
				if kind == "skill" {
					choices = levelUpChoices([]string{skill}, nil)
				} else {
					choices = levelUpChoices(nil, []string{skill})
				}
				got := w.ana.call(charactersv1connect.CharacterServiceLevelUpCharacterProcedure, &charactersv1.LevelUpCharacterRequest{
					CampaignId: w.campaign, CharacterId: rising.GetId(), Revision: rising.GetRevision(), Choices: choices,
				})
				if want := slices.Contains(allowed, skill); got.ok() != want {
					t.Errorf("%s %s at the level-up: HTTP %d %s, want accepted=%v (the allowed list is %v)", kind, skill, got.status, shorten(got.body), want, allowed)
				}
				if got.ok() {
					must(w.master.characters.MarkCharacterDead(ctx, rq(&charactersv1.MarkCharacterDeadRequest{CampaignId: w.campaign, CharacterId: rising.GetId()})))
					rising = fresh() // the accepted level-up moved the character
				}
			}
		})
	}
}
