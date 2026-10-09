package characters

import (
	"slices"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// Multiclassing at the guided level-up (SRD 5.1, "Multiclassing"): Pensantus,
// Wizard 3 with the XP for level 4 (Strength 12, Dexterity 16, Constitution 16,
// Intelligence 18, Wisdom 13, Charisma 12), takes a class he does not have.

// pensantusFighter is the choices of his level 4 as a Fighter 1: the Defense style
// and the average hit points.
func pensantusFighter() *charactersv1.LevelUpChoices {
	return &charactersv1.LevelUpChoices{
		ClassKey:          "class:fighter",
		FeatureChoiceKeys: []string{"feature:fighter-fighting-style-defense"},
		HitPoints:         &charactersv1.LevelUpHitPoints{Method: charactersv1.LevelUpHitPointsMethod_LEVEL_UP_HIT_POINTS_METHOD_AVERAGE},
	}
}

func (tb *levelUpTable) optionsFor(u *user, c *charactersv1.Character, class string) (*charactersv1.LevelUpOptions, error) {
	res, err := u.api.GetLevelUpOptions(tb.h.t.Context(), connect.NewRequest(&charactersv1.GetLevelUpOptionsRequest{CampaignId: tb.campaign, CharacterId: c.GetId(), ClassKey: class}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetOptions(), nil
}

// withScores is the character's sheet with these base scores, as the master's
// editor writes it.
func (tb *levelUpTable) withScores(t *testing.T, c *charactersv1.Character, scores *rulesv1.AbilityScores) *charactersv1.Character {
	t.Helper()
	current := tb.master.get(t, tb.campaign, c.GetId())
	sheet := proto.CloneOf(c.GetSheet())
	sheet.GetFull().BaseScores = scores
	out, err := tb.master.update(t, current, c.GetName(), sheet)
	if err != nil {
		t.Fatalf("UpdateCharacter(scores) error = %v", err)
	}
	return out
}

// TestMR040_ThePlayerTakesANewClass: the class step lists every class with its
// prerequisite; the options of the Fighter are its level 1; the preview and the
// level-up give the sheet Wizard 3 / Fighter 1 with the Fighter's average, the
// multiclass table's proficiencies and no new spell slots.
func TestMR040_ThePlayerTakesANewClass(t *testing.T) {
	t.Parallel()
	tb := newLevelUpTable(t, 2700)
	pc := tb.pc

	// The default options carry the class step: the class he has, then the other
	// eleven, each open or closed by his scores.
	base, err := tb.options(tb.owner, pc)
	if err != nil {
		t.Fatalf("GetLevelUpOptions() error = %v", err)
	}
	if base.GetIsNewClass() || base.GetClassKey() != "class:wizard" {
		t.Errorf("the default options are for %s (new %v), want the Wizard he has", base.GetClassKey(), base.GetIsNewClass())
	}
	choices := base.GetClassChoices()
	if len(choices) != 12 || choices[0].GetClassKey() != "class:wizard" || choices[0].GetIsNew() || choices[0].GetFromLevel() != 3 || choices[0].GetToLevel() != 4 || !choices[0].GetAvailable() {
		t.Fatalf("class choices = %v, want the Wizard 3 to 4 first and 11 new classes", choices)
	}
	open := map[string]bool{"class:cleric": true, "class:druid": true, "class:fighter": true, "class:monk": true, "class:ranger": true, "class:rogue": true}
	for _, ch := range choices[1:] {
		if !ch.GetIsNew() || ch.GetFromLevel() != 0 || ch.GetToLevel() != 1 || len(ch.GetPrerequisites()) == 0 {
			t.Errorf("%s = %v, want a new class from 0 to 1 with its prerequisite", ch.GetClassKey(), ch)
		}
		if ch.GetAvailable() != open[ch.GetClassKey()] {
			t.Errorf("%s available = %v, want %v", ch.GetClassKey(), ch.GetAvailable(), open[ch.GetClassKey()])
		}
		if !ch.GetAvailable() && ch.GetUnavailable() != charactersv1.LevelUpClassUnavailable_LEVEL_UP_CLASS_UNAVAILABLE_PREREQUISITE {
			t.Errorf("%s unavailable = %v", ch.GetClassKey(), ch.GetUnavailable())
		}
	}
	for _, ch := range choices {
		if ch.GetClassKey() == "class:fighter" && (!ch.GetPrerequisiteAnyOf() || len(ch.GetPrerequisites()) != 2) {
			t.Errorf("the Fighter asks for Strength or Dexterity: %v", ch)
		}
		if ch.GetClassKey() == "class:paladin" {
			// Strength 12 of 13 and Charisma 12 of 13: both missing, both said.
			if ch.GetPrerequisiteAnyOf() || ch.GetPrerequisiteMet() || len(ch.GetPrerequisites()) != 2 || ch.GetPrerequisites()[0].GetHave() != 12 || ch.GetPrerequisites()[0].GetMet() {
				t.Errorf("the Paladin = %v, want Strength 13 and Charisma 13, neither met", ch)
			}
		}
	}

	// The options of the new class: its level 1.
	o, err := tb.optionsFor(tb.owner, pc, "class:fighter")
	if err != nil {
		t.Fatalf("GetLevelUpOptions(Fighter) error = %v", err)
	}
	if !o.GetIsNewClass() || o.GetFromLevel() != 0 || o.GetToLevel() != 1 || o.GetTotalFromLevel() != 3 || o.GetTotalToLevel() != 4 ||
		o.GetHitDie() != 10 || o.GetHitPointAverage() != 6 || o.GetAbilityScoreImprovement() || o.GetSubclassDue() || o.GetCantrips() != 0 || o.GetSpells() != 0 {
		t.Errorf("options = %v, want the Fighter's level 1: d10, average 6, nothing else to choose", o)
	}
	if len(o.GetFeatureChoices()) != 1 || o.GetFeatureChoices()[0].GetChoose() != 1 || len(o.GetFeatureChoices()[0].GetOptions()) < 6 {
		t.Errorf("feature choices = %v, want the Fighting Style", o.GetFeatureChoices())
	}
	var gains []string
	for _, g := range o.GetProficiencyGains() {
		gains = append(gains, g.GetKey())
		if g.GetAlreadyHave() {
			t.Errorf("%s already have: the Wizard has no armor or martial weapons", g.GetKey())
		}
	}
	if want := []string{"proficiency:light-armor", "proficiency:medium-armor", "proficiency:shields", "proficiency:simple-weapons", "proficiency:martial-weapons"}; !slices.Equal(gains, want) {
		t.Errorf("proficiency gains = %v, want %v", gains, want)
	}
	if len(o.GetProficiencyChoices()) != 0 {
		t.Errorf("the Fighter asks for no pick as a later class: %v", o.GetProficiencyChoices())
	}
	if o.GetSpellSlotsAfter()[0] != 4 || o.GetSpellSlotsAfter()[1] != 2 || o.GetProficiencyBonusAfter() != 2 {
		t.Errorf("slots %v and proficiency bonus %d after: the Wizard 3's slots and +2 (total level 4)", o.GetSpellSlotsAfter(), o.GetProficiencyBonusAfter())
	}

	// The preview: the numbers after, the multiclass summary.
	p, err := tb.preview(tb.owner, pc, pensantusFighter())
	if err != nil {
		t.Fatalf("PreviewLevelUp() error = %v", err)
	}
	if p.GetRefusal() != nil || p.GetAfter().GetTotalLevel() != 4 || p.GetAfter().GetHitPointsMax() != 32 || len(p.GetAfter().GetClasses()) != 2 {
		t.Fatalf("preview = %v, want Wizard 3 / Fighter 1, 32 hit points (23 and the d10 average 6 + 3), no refusal", p)
	}
	sum := p.GetMulticlassSummary()
	if sum.GetNewClassKey() != "class:fighter" || sum.GetCasterLevelBefore() != 3 || sum.GetCasterLevelAfter() != 3 || sum.GetSlotsByTable() || len(sum.GetExceptions()) != 0 ||
		sum.GetSlotsAfter()[0] != 4 || sum.GetSlotsAfter()[1] != 2 {
		t.Errorf("summary = %v, want the Fighter new, caster level 3 either way, the Wizard's own slots", sum)
	}
	if hd := sum.GetHitDiceAfter(); len(hd) != 2 || hd[0].GetFaces() != 10 || hd[0].GetCount() != 1 || hd[1].GetFaces() != 6 || hd[1].GetCount() != 3 {
		t.Errorf("hit dice after = %v, want 1d10 and 3d6 apart", hd)
	}
	var gained []string
	for _, g := range sum.GetProficienciesGained() {
		gained = append(gained, g.GetKey())
	}
	if want := []string{"proficiency:light-armor", "proficiency:medium-armor", "proficiency:shields", "proficiency:martial-weapons", "proficiency:simple-weapons"}; !sameSet(gained, want) {
		t.Errorf("proficiencies gained = %v, want %v", gained, want)
	}
	if got := tb.master.get(t, tb.campaign, pc.GetId()); got.GetRevision() != pc.GetRevision() {
		t.Error("a preview changed the sheet")
	}
	// Without the Fighting Style the preview says what is missing.
	missing := pensantusFighter()
	missing.FeatureChoiceKeys = nil
	if p, err := tb.preview(tb.owner, pc, missing); err != nil || p.GetRefusal().GetReason() != charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_FEATURE_CHOICE {
		t.Errorf("preview without the style = %v, %v; want FEATURE_CHOICE", p.GetRefusal(), err)
	}

	// The level-up.
	up, err := tb.levelUp(tb.owner, pc, pensantusFighter())
	if err != nil {
		t.Fatalf("LevelUpCharacter() error = %v", err)
	}
	full := up.GetSheet().GetFull()
	d := up.GetDerived()
	if len(full.GetClasses()) != 2 || full.GetClasses()[0].GetClassKey() != "class:wizard" || full.GetClasses()[0].GetLevel() != 3 ||
		full.GetClasses()[1].GetClassKey() != "class:fighter" || full.GetClasses()[1].GetLevel() != 1 {
		t.Errorf("classes = %v, want Wizard 3, then Fighter 1", full.GetClasses())
	}
	if d.GetTotalLevel() != 4 || d.GetHitPointsMax() != 32 || len(d.GetIssues()) != 0 || !slices.Contains(full.GetFeatureChoiceKeys(), "feature:fighter-fighting-style-defense") {
		t.Errorf("sheet after: level %d, %d hit points, issues %v, options %v", d.GetTotalLevel(), d.GetHitPointsMax(), d.GetIssues(), full.GetFeatureChoiceKeys())
	}
	if up.GetRevision() != pc.GetRevision()+1 || up.GetCanLevelUp() {
		t.Errorf("revision %d, can_level_up %v; want %d and no further level", up.GetRevision(), up.GetCanLevelUp(), pc.GetRevision()+1)
	}
	// Nothing but the class changed: the equipment, the skills and the scores stay.
	was := pc.GetSheet().GetFull()
	if !slices.Equal(full.GetSkillProficiencyKeys(), was.GetSkillProficiencyKeys()) || !slices.Equal(full.GetWeaponKeys(), was.GetWeaponKeys()) ||
		full.GetArmorKey() != was.GetArmorKey() || !proto.Equal(full.GetBaseScores(), was.GetBaseScores()) || !slices.Equal(full.GetToolProficiencies(), was.GetToolProficiencies()) {
		t.Error("the level changed the equipment, the skills or the scores")
	}
	// The record for the master: the Fighter, from total level 3 to 4.
	records, err := tb.levelUps(tb.master, pc.GetId())
	if err != nil || len(records) != 1 || records[0].GetClassKey() != "class:fighter" || records[0].GetFromLevel() != 3 || records[0].GetToLevel() != 4 ||
		records[0].GetNamesPt()["class:fighter"] != "Guerreiro" {
		t.Errorf("level-ups = %v, %v; want the Fighter, 3 to 4", records, err)
	}
	if tb.live.count() != 1 {
		t.Errorf("live hints = %d, want 1", tb.live.count())
	}
	// A retry with the old revision is stale, not a second class.
	_, err = tb.levelUp(tb.owner, pc, pensantusFighter())
	wantCode(t, "LevelUpCharacter(again)", err, connect.CodeAborted)
	if got := tb.owner.get(t, tb.campaign, pc.GetId()); len(got.GetSheet().GetFull().GetClasses()) != 2 {
		t.Error("a retry changed the classes")
	}
}

// sameSet says two lists hold the same keys.
func sameSet(a, b []string) bool {
	a, b = slices.Sorted(slices.Values(a)), slices.Sorted(slices.Values(b))
	return slices.Equal(a, b)
}

// TestMR040_ANewClassNeedsItsPrerequisites: the main ability of every class the
// character has and of the new one, at 13 in the final score; the refusal says
// the class, the ability and the numbers (SRD 5.1, "Multiclassing", "Prerequisites").
func TestMR040_ANewClassNeedsItsPrerequisites(t *testing.T) {
	t.Parallel()
	tb := newLevelUpTable(t, 2700)
	pc := tb.pc
	pre := charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_MULTICLASS_PREREQUISITE
	cur := charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_MULTICLASS_PREREQUISITE_CURRENT

	// The Paladin asks for Strength 13 and Charisma 13: he has 12 and 12. The first
	// missing score is told.
	_, err := tb.optionsFor(tb.owner, pc, "class:paladin")
	r := refusal(t, "GetLevelUpOptions(Paladin)", err)
	if r.GetReason() != pre || r.GetClassKey() != "class:paladin" || r.GetAbility() != rulesv1.Ability_ABILITY_STRENGTH || r.GetMinimum() != 13 || r.GetHave() != 12 || r.GetField() != "class_key" {
		t.Errorf("refusal = %v, want the Paladin's Strength 13, have 12, at class_key", r)
	}
	// Preview, roll and level-up are refused alike, and nothing is written.
	paladin := pensantusFighter()
	paladin.ClassKey, paladin.FeatureChoiceKeys = "class:paladin", nil
	_, err = tb.preview(tb.owner, pc, paladin)
	if r := refusal(t, "PreviewLevelUp(Paladin)", err); r.GetReason() != pre {
		t.Errorf("preview refusal = %v", r)
	}
	_, err = tb.levelUp(tb.owner, pc, paladin)
	if r := refusal(t, "LevelUpCharacter(Paladin)", err); r.GetReason() != pre || r.GetClassKey() != "class:paladin" {
		t.Errorf("level-up refusal = %v", r)
	}
	_, err = tb.owner.api.RollLevelUpHitPoints(t.Context(), connect.NewRequest(&charactersv1.RollLevelUpHitPointsRequest{
		CampaignId: tb.campaign, CharacterId: pc.GetId(), ClassKey: "class:paladin", IdempotencyKey: "4b1a2f7e-3c1d-4a62-9d06-7f6f1b3e0a11",
	}))
	if r := refusal(t, "RollLevelUpHitPoints(Paladin)", err); r.GetReason() != pre {
		t.Errorf("roll refusal = %v", r)
	}
	if got := tb.owner.get(t, tb.campaign, pc.GetId()); !proto.Equal(got, pc) {
		t.Error("a refused level-up changed the character")
	}

	// The Fighter asks for Strength or Dexterity: Dexterity 16 is enough; with both
	// low the refusal names the closer one.
	if _, err := tb.optionsFor(tb.owner, pc, "class:fighter"); err != nil {
		t.Errorf("Dexterity 16 for the Fighter: %v", err)
	}
	weak := tb.withScores(t, pc, &rulesv1.AbilityScores{Strength: 12, Dexterity: 10, Constitution: 15, Intelligence: 16, Wisdom: 13, Charisma: 12})
	_, err = tb.optionsFor(tb.owner, weak, "class:fighter")
	r = refusal(t, "GetLevelUpOptions(Fighter, Str 12, Dex 10)", err)
	if r.GetReason() != pre || r.GetAbility() != rulesv1.Ability_ABILITY_STRENGTH || r.GetHave() != 12 || r.GetMinimum() != 13 {
		t.Errorf("refusal = %v, want Strength 13, have 12 (the closer of the two)", r)
	}
	// The class step says the same, and the Wizard's own next level stays open.
	o, err := tb.options(tb.owner, weak)
	if err != nil || o.GetClassKey() != "class:wizard" {
		t.Fatalf("options of the Wizard = %v, %v", o, err)
	}
	for _, ch := range o.GetClassChoices() {
		if ch.GetClassKey() == "class:fighter" && (ch.GetAvailable() || ch.GetPrerequisiteMet()) {
			t.Errorf("the Fighter is open with Strength 12 and Dexterity 10: %v", ch)
		}
	}

	// The class he has asks for Intelligence 13: with 12 he enters no class, and the
	// refusal names the Wizard.
	dull := tb.withScores(t, pc, &rulesv1.AbilityScores{Strength: 12, Dexterity: 16, Constitution: 15, Intelligence: 10, Wisdom: 13, Charisma: 12})
	_, err = tb.optionsFor(tb.owner, dull, "class:fighter")
	r = refusal(t, "GetLevelUpOptions(Fighter, Int 12)", err)
	if r.GetReason() != cur || r.GetClassKey() != "class:wizard" || r.GetAbility() != rulesv1.Ability_ABILITY_INTELLIGENCE || r.GetMinimum() != 13 || r.GetHave() != 12 {
		t.Errorf("refusal = %v, want the Wizard's Intelligence 13, have 12", r)
	}
	o, err = tb.options(tb.owner, dull)
	if err != nil {
		t.Fatalf("options with Intelligence 12: %v", err)
	}
	for _, ch := range o.GetClassChoices() {
		switch {
		case ch.GetClassKey() == "class:wizard" && (!ch.GetAvailable() || ch.GetPrerequisiteMet()):
			t.Errorf("the Wizard = %v, want its next level open and its prerequisite unmet", ch)
		case ch.GetIsNew() && (ch.GetAvailable() || ch.GetUnavailable() != charactersv1.LevelUpClassUnavailable_LEVEL_UP_CLASS_UNAVAILABLE_PREREQUISITE_CURRENT):
			t.Errorf("%s = %v, want it closed by the class he has", ch.GetClassKey(), ch)
		}
	}
	// A class that is not in the content is a malformed request.
	_, err = tb.optionsFor(tb.owner, pc, "class:artificer")
	wantCode(t, "GetLevelUpOptions(unknown class)", err, connect.CodeInvalidArgument)
	// Nobody else reads it (RN-10): the sheet is the owner's and the master's.
	_, err = tb.optionsFor(tb.other, pc, "class:fighter")
	wantCode(t, "GetLevelUpOptions(other player)", err, connect.CodeNotFound)
}

// TestMR040_TheMasterMayGoPastThePrerequisite: the master's editor adds a class to
// a sheet that does not meet the prerequisite, and the sheet shows the issue; the
// player's own edit of a draft that adds the class is refused, and a sheet that
// already missed it can still be saved.
func TestMR040_TheMasterMayGoPastThePrerequisite(t *testing.T) {
	t.Parallel()
	tb := newLevelUpTable(t, 2700)
	pc := tb.pc
	add := func(c *charactersv1.Character, class string) *charactersv1.CharacterSheet {
		sheet := proto.CloneOf(c.GetSheet())
		sheet.GetFull().Classes = append(sheet.GetFull().Classes, &charactersv1.ClassLevel{ClassKey: class, Level: 1})
		return sheet
	}
	// Barbarian asks for Strength 13 and he has 12.
	sheet := add(pc, "class:barbarian")
	sheet.GetFull().HitPoints = &charactersv1.HitPoints{Method: charactersv1.HitPointsMethod_HIT_POINTS_METHOD_AVERAGE}
	got, err := tb.master.update(t, pc, pc.GetName(), sheet)
	if err != nil {
		t.Fatalf("the master's UpdateCharacter() error = %v", err)
	}
	var codes []string
	for _, is := range got.GetDerived().GetIssues() {
		codes = append(codes, is.GetCode())
	}
	if !slices.Contains(codes, "multiclass_prerequisite") || len(got.GetSheet().GetFull().GetClasses()) != 2 {
		t.Errorf("the master's edit: issues %v, classes %v; want the multiclass_prerequisite issue and two classes", codes, got.GetSheet().GetFull().GetClasses())
	}

	// A player's pending character is refused the same edit; the master's version
	// above stays. Another character, still a draft: Dona's.
	h := tb.h
	player := h.newUser("Rafa")
	campaign := h.newCampaign(tb.master, "Outra mesa", player)
	draft := player.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus", pensantusSheet())
	twoClasses := proto.CloneOf(draft.GetSheet())
	twoClasses.GetFull().Classes = append(twoClasses.GetFull().Classes, &charactersv1.ClassLevel{ClassKey: "class:barbarian", Level: 1})
	twoClasses.GetFull().HitPoints = &charactersv1.HitPoints{Method: charactersv1.HitPointsMethod_HIT_POINTS_METHOD_AVERAGE}
	_, err = player.update(t, draft, draft.GetName(), twoClasses)
	r := refusal(t, "UpdateCharacter(a draft that adds the Barbarian)", err)
	if r.GetReason() != charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_MULTICLASS_PREREQUISITE || r.GetClassKey() != "class:barbarian" ||
		r.GetField() != "sheet.full.classes[1].class_key" || r.GetMinimum() != 13 || r.GetHave() != 12 {
		t.Errorf("refusal = %v, want the Barbarian's Strength 13, have 12, at classes[1]", r)
	}
	// With Strength 13 it saves.
	strong := proto.CloneOf(twoClasses)
	strong.GetFull().BaseScores.Strength = 13
	if _, err := player.update(t, draft, draft.GetName(), strong); err != nil {
		t.Errorf("the Barbarian with Strength 13: %v", err)
	}
	// The master's sheet is not touched by a player's edit later: the player can
	// still save what already missed the prerequisite.
	missed, err := tb.master.update(t, player.get(t, campaign, draft.GetId()), draft.GetName(), twoClasses)
	if err != nil {
		t.Fatalf("the master's edit of the draft: %v", err)
	}
	renamed := proto.CloneOf(missed.GetSheet())
	renamed.GetFull().Languages = []string{"Dracônico", "Élfico"}
	if _, err := player.update(t, missed, missed.GetName(), renamed); err != nil {
		t.Errorf("a player's edit of a sheet that already missed the prerequisite: %v", err)
	}
}

// TestMR040_TheHitPointsOfANewClassUseItsDie: a roll in the app is the new class's
// die, kept once for the level; a typed result must fit that die; the average is
// never the maximum.
func TestMR040_TheHitPointsOfANewClassUseItsDie(t *testing.T) {
	t.Parallel()
	tb := newLevelUpTable(t, 2700, 8) // the server's roll of the Fighter's d10
	pc := tb.pc
	res, err := tb.owner.api.RollLevelUpHitPoints(t.Context(), connect.NewRequest(&charactersv1.RollLevelUpHitPointsRequest{
		CampaignId: tb.campaign, CharacterId: pc.GetId(), ClassKey: "class:fighter", IdempotencyKey: "1f9a2f7e-3c1d-4a62-9d06-7f6f1b3e0a11",
	}))
	if err != nil || res.Msg.GetDie() != 10 || res.Msg.GetValue() != 8 {
		t.Fatalf("RollLevelUpHitPoints(Fighter) = %v, %v; want a d10 and 8", res, err)
	}
	// The kept roll belongs to the Fighter: the Wizard's level cannot take it.
	wiz := pensantusLevelUp()
	wiz.HitPoints = &charactersv1.LevelUpHitPoints{Method: charactersv1.LevelUpHitPointsMethod_LEVEL_UP_HIT_POINTS_METHOD_ROLLED_IN_APP}
	_, err = tb.levelUp(tb.owner, pc, wiz)
	if r := refusal(t, "LevelUpCharacter(Wizard, the Fighter's roll)", err); r.GetReason() != charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_HIT_POINT_ROLL_OTHER_CLASS {
		t.Errorf("refusal = %v", r)
	}
	fighter := pensantusFighter()
	fighter.HitPoints = &charactersv1.LevelUpHitPoints{Method: charactersv1.LevelUpHitPointsMethod_LEVEL_UP_HIT_POINTS_METHOD_ROLLED_IN_APP}
	up, err := tb.levelUp(tb.owner, pc, fighter)
	if err != nil {
		t.Fatalf("LevelUpCharacter(Fighter, rolled) error = %v", err)
	}
	// 23 hit points, then the roll of 8 and the Constitution modifier +3 (16).
	if up.GetDerived().GetHitPointsMax() != 23+8+3 {
		t.Errorf("hit points = %d, want %d", up.GetDerived().GetHitPointsMax(), 23+8+3)
	}
	if rolls := up.GetSheet().GetFull().GetHitPoints().GetRolls(); len(rolls) != 3 || rolls[2] != 8 {
		t.Errorf("rolls = %v, want the two averages of the Wizard and the Fighter's 8 last", rolls)
	}

	// A typed result must fit the die of the class that gains the level.
	tb2 := newLevelUpTable(t, 2700)
	typed := pensantusFighter()
	typed.HitPoints = &charactersv1.LevelUpHitPoints{Method: charactersv1.LevelUpHitPointsMethod_LEVEL_UP_HIT_POINTS_METHOD_ROLLED_PHYSICAL, Value: 11}
	_, err = tb2.levelUp(tb2.owner, tb2.pc, typed)
	if r := refusal(t, "LevelUpCharacter(typed 11 on a d10)", err); r.GetReason() != charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_HIT_POINTS {
		t.Errorf("refusal = %v", r)
	}
	typed.HitPoints.Value = 10
	if up, err := tb2.levelUp(tb2.owner, tb2.pc, typed); err != nil || up.GetDerived().GetHitPointsMax() != 23+10+3 {
		t.Errorf("a typed 10 on the d10: %v, %v", up.GetDerived().GetHitPointsMax(), err)
	}
}

// TestAMulticlassLevelBringsTheNewClassesChoices: a Wizard that takes Ranger 1 picks the
// ranger's level 1 choices (a favored enemy and its language, a terrain) at that level:
// the offer lists them in new_choices, the level is refused without their picks
// (FEATURE_CHOICE) and goes through with them, leaving no choice open (RN-32).
func TestAMulticlassLevelBringsTheNewClassesChoices(t *testing.T) {
	t.Parallel()
	tb := newLevelUpTable(t, 2700)
	o, err := tb.optionsFor(tb.owner, tb.pc, "class:ranger")
	if err != nil {
		t.Fatalf("GetLevelUpOptions(Ranger): %v", err)
	}
	if !o.GetIsNewClass() || len(o.GetLateChoices()) != 0 {
		t.Fatalf("options = new %v, late %v; want a new class and nothing left behind", o.GetIsNewClass(), o.GetLateChoices())
	}
	var asked []*charactersv1.Choice
	for _, g := range o.GetNewChoices() {
		asked = append(asked, g.GetChoices()...)
	}
	if len(asked) < 2 {
		t.Fatalf("new choices = %v, want the favored enemy and the terrain of level 1", o.GetNewChoices())
	}
	ch := &charactersv1.LevelUpChoices{ClassKey: "class:ranger", HitPoints: &charactersv1.LevelUpHitPoints{Method: charactersv1.LevelUpHitPointsMethod_LEVEL_UP_HIT_POINTS_METHOD_AVERAGE}}
	for _, fc := range o.GetFeatureChoices() {
		for _, opt := range fc.GetOptions()[:fc.GetChoose()] {
			ch.FeatureChoiceKeys = append(ch.FeatureChoiceKeys, opt.GetKey())
		}
	}
	for _, sk := range o.GetProficiencyChoices() {
		for _, opt := range sk.GetFrom() {
			if !opt.GetAlreadyHave() && len(ch.SkillProficiencyKeys) < int(sk.GetCount()) {
				ch.SkillProficiencyKeys = append(ch.SkillProficiencyKeys, opt.GetKey())
			}
		}
	}
	_, err = tb.levelUp(tb.owner, tb.pc, ch)
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("the level without the ranger's choices: %v, want a refusal", err)
	}
	for _, c := range asked {
		for _, opt := range c.GetOptions() {
			if opt.GetReasonPt() == "" && !opt.GetNeedsText() && len(ch.FeatureChoiceKeys) < 12 && !slices.Contains(ch.FeatureChoiceKeys, opt.GetStoredKey()) {
				ch.FeatureChoiceKeys = append(ch.FeatureChoiceKeys, opt.GetStoredKey())
				break
			}
		}
	}
	done, err := tb.levelUp(tb.owner, tb.pc, ch)
	if err != nil {
		t.Fatalf("the level with the ranger's choices: %v", err)
	}
	if len(done.GetDerived().GetClasses()) != 2 || done.GetDerived().GetTotalLevel() != 4 {
		t.Errorf("after the level: %d classes, level %d; want Wizard 3 and Ranger 1", len(done.GetDerived().GetClasses()), done.GetDerived().GetTotalLevel())
	}
}
