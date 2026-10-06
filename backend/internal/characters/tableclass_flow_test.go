package characters

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
)

// classFlow is a campaign whose master wrote the guardian, its subclass and a
// third caster, with Davi's character Ícaro (a Guardião do Vale of level 1 and
// the XP for level 5) on a locked sheet: the fixture of the creation, level-up and
// "A classe mudou" tests of slice 10.3.
type classFlow struct {
	h        *harness
	master   *user
	davi     *user
	campaign string
	defaults *rulesv1.GetClassTableDefaultsResponse
	menu     *rulesv1.GetEffectMenuResponse
	guardian *rulesv1.TableEntry
	path     *rulesv1.TableEntry
	ink      *rulesv1.TableEntry
	dice     *testDiceRules
}

func newClassFlow(t *testing.T) *classFlow {
	t.Helper()
	f := &classFlow{dice: &testDiceRules{}}
	f.dice.rule.Store(int32(charactersv1.LevelUpDiceRule_LEVEL_UP_DICE_RULE_PLAYER_CHOOSES))
	f.h = newHarnessWith(t, func(c *Config) { c.Dice, c.Roller = f.dice, &dice.Fixed{Faces: []int{5}} })
	f.h.svc.SetLevelUps(xpLevelUps{})
	f.master, f.davi = f.h.newUser("Samuel"), f.h.newUser("Davi")
	f.campaign = f.h.newCampaign(f.master, "Mirathel", f.davi)
	f.defaults, f.menu = f.master.classDefaults(t, f.campaign), f.master.effectMenu(t, f.campaign)
	f.guardian = f.master.addEntry(t, f.campaign, guardianClass(t, f.defaults, f.menu))
	f.path = f.master.addEntry(t, f.campaign, valleyPath(f.guardian.GetKey()))
	f.ink = f.master.addEntry(t, f.campaign, inkBlade(t, f.defaults))
	return f
}

// icaroSheet is Ícaro at level 1: the guardian, "Outro" as the background (SRD 5.1
// "Customizing a Background"), the two skills of the class, the XP of level 5.
func (f *classFlow) icaroSheet() *charactersv1.CharacterSheet {
	return &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores: &rulesv1.AbilityScores{Strength: 14, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 15, Charisma: 8}, RaceKey: "race:human",
		Background: &charactersv1.FullSheet_CustomBackground{CustomBackground: &charactersv1.CustomBackground{
			Name: "Batedor de torre", SkillKeys: []string{"skill:perception", "skill:survival"}, ProficiencyKeys: []string{"proficiency:thieves-tools", "language:elvish"},
			FeatureName: "Olho no horizonte", FeatureText: "Você sempre acha o ponto mais alto de um lugar.", Equipment: "Uma luneta, um rolo de corda e 10 PO.",
		}},
		Classes:              []*charactersv1.ClassLevel{{ClassKey: f.guardian.GetKey(), Level: 1}},
		SkillProficiencyKeys: []string{"skill:athletics", "skill:nature"},
		HitPoints:            hitPointsAverage(), ExperiencePoints: 6500,
	}}}
}

// spells lists the spells of a class's list at the given circles, as the editor's
// spell picker does.
func (f *classFlow) spells(t *testing.T, class string, levels ...int32) []string {
	t.Helper()
	res, err := f.master.content.ListSpells(t.Context(), connect.NewRequest(&rulesv1.ListSpellsRequest{CampaignId: f.campaign, ClassKey: class, Levels: levels, PageSize: 400}))
	if err != nil {
		t.Fatalf("ListSpells(%s) error = %v", class, err)
	}
	var out []string
	for _, s := range res.Msg.GetSpells() {
		out = append(out, s.GetKey())
	}
	return out
}

// firstNew are the first n keys of from that are not in have.
func firstNew(from, have []string, n int) []string {
	var out []string
	for _, k := range from {
		if len(out) < n && !slices.Contains(have, k) {
			out = append(out, k)
		}
	}
	return out
}

// levelUp makes the choices the options of the next level ask for, the first valid
// ones (with subKey when the subclass is due), and goes up: the player's
// guided level-up, through the API.
func (f *classFlow) levelUp(t *testing.T, u *user, c *charactersv1.Character, classKey, subKey string) (*charactersv1.Character, *charactersv1.LevelUpOptions) {
	t.Helper()
	res, err := u.api.GetLevelUpOptions(t.Context(), connect.NewRequest(&charactersv1.GetLevelUpOptionsRequest{CampaignId: f.campaign, CharacterId: c.GetId(), ClassKey: classKey}))
	if err != nil {
		t.Fatalf("GetLevelUpOptions() error = %v", err)
	}
	o := res.Msg.GetOptions()
	full := c.GetSheet().GetFull()
	ch := &charactersv1.LevelUpChoices{ClassKey: o.GetClassKey(), HitPoints: &charactersv1.LevelUpHitPoints{Method: charactersv1.LevelUpHitPointsMethod_LEVEL_UP_HIT_POINTS_METHOD_AVERAGE}}
	if o.GetAbilityScoreImprovement() {
		ch.AbilityIncrease = &rulesv1.AbilityScores{Strength: 2}
	}
	choices, cantrips, spells, spellList, maxLevel, kind := o.GetFeatureChoices(), o.GetCantrips(), o.GetSpells(), o.GetSpellListClassKey(), o.GetMaxSpellLevel(), o.GetSpellsKind()
	if o.GetSubclassDue() {
		var picked *charactersv1.LevelUpSubclass
		for _, s := range o.GetSubclasses() {
			if s.GetKey() == subKey {
				picked = s
			}
		}
		if picked == nil {
			t.Fatalf("the subclass %s is not offered: %v", subKey, o.GetSubclasses())
		}
		ch.SubclassKey = picked.GetKey()
		choices = append(choices, picked.GetFeatureChoices()...)
		cantrips += picked.GetCantrips()
		if picked.GetSpells() > 0 {
			spells += picked.GetSpells()
			spellList, maxLevel, kind = picked.GetSpellListClassKey(), picked.GetMaxSpellLevel(), picked.GetSpellsKind()
		}
	}
	for _, fc := range choices {
		var keys []string
		for _, op := range fc.GetOptions() {
			keys = append(keys, op.GetKey())
		}
		ch.FeatureChoiceKeys = append(ch.FeatureChoiceKeys, keys[:fc.GetChoose()]...)
	}
	if spellList != "" {
		ch.CantripKeys = firstNew(f.spells(t, spellList, 0), full.GetCantripKeys(), int(cantrips))
		if kind != charactersv1.LevelUpSpellsKind_LEVEL_UP_SPELLS_KIND_UNSPECIFIED {
			var pool []string
			for lvl := int32(1); lvl <= maxLevel; lvl++ {
				pool = append(pool, f.spells(t, spellList, lvl)...)
			}
			ch.KnownSpellKeys = firstNew(pool, full.GetKnownSpellKeys(), int(spells))
		}
	}
	if o.GetPrepares() {
		var pool []string
		for lvl := int32(1); lvl <= max(o.GetMaxSpellLevel(), 1); lvl++ {
			pool = append(pool, f.spells(t, o.GetSpellListClassKey(), lvl)...)
		}
		ch.PreparedSpellKeys = firstNew(pool, full.GetPreparedSpellKeys(), int(o.GetPreparedMaxAfter())-len(full.GetPreparedSpellKeys()))
	}
	if _, err := u.api.PreviewLevelUp(t.Context(), connect.NewRequest(&charactersv1.PreviewLevelUpRequest{CampaignId: f.campaign, CharacterId: c.GetId(), Choices: ch})); err != nil {
		t.Fatalf("PreviewLevelUp(%v) error = %v", ch, err)
	}
	up, err := u.api.LevelUpCharacter(t.Context(), connect.NewRequest(&charactersv1.LevelUpCharacterRequest{CampaignId: f.campaign, CharacterId: c.GetId(), Revision: c.GetRevision(), Choices: ch}))
	if err != nil {
		t.Fatalf("LevelUpCharacter(to %d) error = %v\nchoices: %v", o.GetToLevel(), err, ch)
	}
	return up.Msg.GetCharacter(), o
}

func slotCounts(d *rulesv1.DerivedSheet) []int32 {
	var out []int32
	for _, s := range d.GetSpellSlots() {
		out = append(out, s.GetCount())
	}
	return out
}

func hasFeature(d *rulesv1.DerivedSheet, name string) bool {
	return slices.ContainsFunc(d.GetFeatures(), func(f *rulesv1.Feature) bool { return f.GetNamePt() == name })
}

func hasSpell(d *rulesv1.DerivedSheet, key string, prepared bool) bool {
	return slices.ContainsFunc(d.GetSpells(), func(s *rulesv1.CharacterSpell) bool {
		return s.GetSpell().GetKey() == key && (!prepared || s.GetPrepared())
	})
}

// TestTableClassCreationAndLevelUp (E10-02 states 5 and 7, MR-025, MR-040): Davi
// creates Ícaro, a Guardião do Vale of the table with the background "Outro", and
// takes him from level 1 to 5 with the guided level-up: the hit points by the
// average, a fighting style chosen from the SRD's set at level 2 with the half
// caster's slots and prepared spells, the subclass at level 3 with its
// always-prepared spells, the Ability Score Improvement at level 4 and the slots of
// the table at level 5. A Fighter with the table's third-caster subclass goes up
// the same way, with the spells and slots of its own table.
func TestTableClassCreationAndLevelUp(t *testing.T) {
	t.Parallel()
	f := newClassFlow(t)

	c := f.davi.create(t, f.campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Ícaro", f.icaroSheet())
	d := c.GetDerived()
	if len(d.GetIssues()) != 0 || d.GetTotalLevel() != 1 || d.GetBackgroundNamePt() != "Batedor de torre" || d.GetHitPointsMax() != 12 || !hasFeature(d, "Olhos do Vale") {
		t.Fatalf("Ícaro: issues %v, level %d, background %q, %d HP, features %v; want a clean level-1 Guardião with 12 PV (d10 + CON +2) and the class's sense",
			d.GetIssues(), d.GetTotalLevel(), d.GetBackgroundNamePt(), d.GetHitPointsMax(), d.GetFeatures())
	}
	if len(d.GetSpellcasting()) != 0 || len(d.GetSpellSlots()) != 0 && d.GetSpellSlots()[0].GetCount() != 0 {
		t.Errorf("a half caster casts from level 2: spellcasting %v, slots %v at level 1", d.GetSpellcasting(), d.GetSpellSlots())
	}
	if f.h.lockSheets(f.campaign) != 1 {
		t.Fatal("expected one sheet to lock")
	}
	c = f.davi.get(t, f.campaign, c.GetId())

	hp := d.GetHitPointsMax()
	for to := 2; to <= 5; to++ {
		var o *charactersv1.LevelUpOptions
		c, o = f.levelUp(t, f.davi, c, f.guardian.GetKey(), f.path.GetKey())
		d = c.GetDerived()
		if len(d.GetIssues()) != 0 || d.GetTotalLevel() != int32(to) {
			t.Fatalf("level %d: issues %v, total level %d", to, d.GetIssues(), d.GetTotalLevel())
		}
		hp += o.GetHitPointAverage() + 2
		if d.GetHitPointsMax() != hp {
			t.Errorf("level %d: %d PV, want %d (the average %d + CON +2 on the level before)", to, d.GetHitPointsMax(), hp, o.GetHitPointAverage())
		}
		switch to {
		case 2:
			if got := slotCounts(d); len(got) == 0 || got[0] != 2 || !hasFeature(d, "Estilo de luta") || !hasFeature(d, "Conjuração") {
				t.Errorf("level 2: slots %v, features %v; want 2 first-circle slots from the table, the fighting style and the spellcasting", got, d.GetFeatures())
			}
			if len(c.GetSheet().GetFull().GetFeatureChoiceKeys()) != 1 || len(d.GetSpellcasting()) != 1 || d.GetSpellcasting()[0].GetPreparedMax() < 1 || len(c.GetSheet().GetFull().GetPreparedSpellKeys()) != int(d.GetSpellcasting()[0].GetPreparedMax()) {
				t.Errorf("level 2: choices %v, casting %v, prepared %v; want the style and the prepared spells up to the maximum", c.GetSheet().GetFull().GetFeatureChoiceKeys(), d.GetSpellcasting(), c.GetSheet().GetFull().GetPreparedSpellKeys())
			}
			if o.GetSpellListClassKey() != f.guardian.GetKey() || len(f.spells(t, o.GetSpellListClassKey(), 1)) < 10 {
				t.Errorf("level 2: the spell list = %q (%d spells of the 1st circle), want the class's own, which is the druid's", o.GetSpellListClassKey(), len(f.spells(t, o.GetSpellListClassKey(), 1)))
			}
		case 3:
			if !o.GetSubclassDue() || c.GetSheet().GetFull().GetClasses()[0].GetSubclassKey() != f.path.GetKey() || !hasFeature(d, "Passo do Vale") {
				t.Errorf("level 3: subclass due %v, sheet %v, features %v; want the subclass chosen with its feature", o.GetSubclassDue(), c.GetSheet().GetFull().GetClasses(), d.GetFeatures())
			}
			if !hasSpell(d, "spell:cure-wounds", true) {
				t.Errorf("level 3: the always-prepared Cure Wounds is not prepared: %v", d.GetSpells())
			}
		case 4:
			if !o.GetAbilityScoreImprovement() || d.GetAbilities()[0].GetScore() != 17 {
				t.Errorf("level 4: ASI due %v, STR %d; want the Ability Score Improvement and Strength 17 (14, +1 human, +2)", o.GetAbilityScoreImprovement(), d.GetAbilities()[0].GetScore())
			}
		case 5:
			if got := slotCounts(d); len(got) < 2 || got[0] != 4 || got[1] != 2 || !hasFeature(d, "Ataque duplo") || !hasSpell(d, "spell:lesser-restoration", true) {
				t.Errorf("level 5: slots %v, features %v; want 4 and 2 slots (the paladin's table), the extra attack and the 5th-level always-prepared spell", got, d.GetFeatures())
			}
		}
	}
	if d.GetProficiencyBonus() != 3 {
		t.Errorf("proficiency bonus at level 5 = %d, want 3", d.GetProficiencyBonus())
	}
	if lu, err := f.master.api.ListLevelUps(t.Context(), connect.NewRequest(&charactersv1.ListLevelUpsRequest{CampaignId: f.campaign, CharacterId: c.GetId()})); err != nil || len(lu.Msg.GetLevelUps()) != 4 {
		t.Errorf("level-ups = %v, %v; want 4 in the log", lu, err)
	}
}

// thirdCasterSheet is a Fighter 3 with the table's Lâmina de Tinta (E10-02: a third
// caster's subclass): two skills, the fighting style, two cantrips and three spells
// of the wizard's list.
func thirdCasterSheet(sub string) *charactersv1.CharacterSheet {
	return &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores: &rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 14, Wisdom: 10, Charisma: 8}, RaceKey: "race:human",
		Background:           &charactersv1.FullSheet_BackgroundKey{BackgroundKey: "background:acolyte"},
		Classes:              []*charactersv1.ClassLevel{ownedSubclass(&charactersv1.ClassLevel{ClassKey: "class:fighter", Level: 3}, sub)},
		SkillProficiencyKeys: []string{"skill:athletics", "skill:perception"},
		FeatureChoiceKeys:    []string{"feature:fighter-fighting-style-defense"},
		CantripKeys:          []string{"spell:fire-bolt", "spell:mage-hand"},
		KnownSpellKeys:       []string{"spell:magic-missile", "spell:shield", "spell:sleep"},
		WeaponKeys:           []string{"equipment:longsword"},
		HitPoints:            hitPointsAverage(), ExperiencePoints: 6500,
	}}}
}

// TestTableClassThirdCasterCreationAndLevelUp (MR-025, E10-02 state 7): a Fighter
// with the table's third caster knows two cantrips and three spells, casts with
// Intelligence from the wizard's list, has the slots of the third caster's table,
// and the level-up to 4 and 5 adds the Ability Score Improvement, the 4th spell
// known and the 3rd slot.
func TestTableClassThirdCasterCreationAndLevelUp(t *testing.T) {
	t.Parallel()
	f := newClassFlow(t)
	c := f.davi.create(t, f.campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Cavaleiro", thirdCasterSheet(f.ink.GetKey()))
	d := c.GetDerived()
	if len(d.GetIssues()) != 0 || len(d.GetSpellcasting()) != 1 {
		t.Fatalf("issues %v, casters %v; want a clean sheet with one caster", d.GetIssues(), d.GetSpellcasting())
	}
	sc := d.GetSpellcasting()[0]
	if sc.GetClassKey() != "class:fighter" || sc.GetAbility() != rulesv1.Ability_ABILITY_INTELLIGENCE || sc.GetSaveDc() != 8+2+2 || sc.GetAttackBonus() != 4 || sc.GetCantripsKnown() != 2 || sc.GetSpellsKnown() != 3 {
		t.Errorf("casting = %v, want Intelligence, DC 12, +4, 2 cantrips and 3 spells", sc)
	}
	if got := slotCounts(d); len(got) == 0 || got[0] != 2 || !hasFeature(d, "Lâmina entintada") || !hasFeature(d, "Conjuração") {
		t.Errorf("level 3: slots %v, features %v; want 2 slots and the subclass's features", got, d.GetFeatures())
	}
	f.h.lockSheets(f.campaign)
	c = f.davi.get(t, f.campaign, c.GetId())

	for to := 4; to <= 5; to++ {
		var o *charactersv1.LevelUpOptions
		c, o = f.levelUp(t, f.davi, c, "class:fighter", f.ink.GetKey())
		d = c.GetDerived()
		if len(d.GetIssues()) != 0 || d.GetTotalLevel() != int32(to) {
			t.Fatalf("level %d: issues %v, total level %d", to, d.GetIssues(), d.GetTotalLevel())
		}
		switch to {
		case 4:
			if !o.GetAbilityScoreImprovement() || o.GetSpellListClassKey() != "class:wizard" {
				t.Errorf("level 4: ASI %v, list %q; want the Ability Score Improvement and the wizard's list for a third caster", o.GetAbilityScoreImprovement(), o.GetSpellListClassKey())
			}
			if got := slotCounts(d); got[0] != 3 {
				t.Errorf("level 4: slots %v, want 3 first-circle slots", got)
			}
		case 5:
			if sc := d.GetSpellcasting()[0]; sc.GetSpellsKnown() != 4 || len(c.GetSheet().GetFull().GetKnownSpellKeys()) != 4 || o.GetSpells() != 1 {
				t.Errorf("level 5: knows %d (sheet %d), the offer asked for %d new; want 4 known, one of them new", sc.GetSpellsKnown(), len(c.GetSheet().GetFull().GetKnownSpellKeys()), o.GetSpells())
			}
			if got := slotCounts(d); len(got) < 1 || got[0] != 3 || d.GetProficiencyBonus() != 3 {
				t.Errorf("level 5: slots %v, bonus %d; want 3 slots and +3", got, d.GetProficiencyBonus())
			}
		}
	}
}

// TestTableClassSubclassPickedAtLevelUpGivesItsSpells: a Fighter of level 2 who
// reaches level 3 chooses the third caster's subclass in the guided level-up, and
// the options say how many cantrips and spells it adds, from which list and up to
// which circle, so the web draws the step without knowing the table (slice 10.3).
func TestTableClassSubclassPickedAtLevelUpGivesItsSpells(t *testing.T) {
	t.Parallel()
	f := newClassFlow(t)
	sheet := thirdCasterSheet("")
	full := sheet.GetFull()
	full.Classes = []*charactersv1.ClassLevel{{ClassKey: "class:fighter", Level: 2}}
	full.CantripKeys, full.KnownSpellKeys = nil, nil
	c := f.davi.create(t, f.campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Cavaleiro", sheet)
	if d := c.GetDerived(); len(d.GetIssues()) != 0 {
		t.Fatalf("the level-2 Fighter has issues: %v", d.GetIssues())
	}
	f.h.lockSheets(f.campaign)
	c = f.davi.get(t, f.campaign, c.GetId())

	res, err := f.davi.api.GetLevelUpOptions(t.Context(), connect.NewRequest(&charactersv1.GetLevelUpOptionsRequest{CampaignId: f.campaign, CharacterId: c.GetId(), ClassKey: "class:fighter"}))
	if err != nil {
		t.Fatalf("GetLevelUpOptions() error = %v", err)
	}
	o := res.Msg.GetOptions()
	var blade *charactersv1.LevelUpSubclass
	for _, s := range o.GetSubclasses() {
		if s.GetKey() == f.ink.GetKey() {
			blade = s
		}
	}
	if !o.GetSubclassDue() || blade == nil {
		t.Fatalf("subclass due %v, subclasses %v; want the table's third caster among them", o.GetSubclassDue(), o.GetSubclasses())
	}
	if blade.GetCantrips() != 2 || blade.GetSpells() != 3 || blade.GetSpellsKind() != charactersv1.LevelUpSpellsKind_LEVEL_UP_SPELLS_KIND_KNOWN || blade.GetSpellListClassKey() != "class:wizard" || blade.GetMaxSpellLevel() != 1 {
		t.Errorf("the subclass's offer = %v, want 2 cantrips, 3 known spells of the wizard's list, up to the 1st circle", blade)
	}
	if champion := slices.IndexFunc(o.GetSubclasses(), func(s *charactersv1.LevelUpSubclass) bool { return s.GetKey() == "subclass:champion" }); champion < 0 || o.GetSubclasses()[champion].GetSpells() != 0 {
		t.Errorf("the SRD's Champion = %v, want it offered with no spells", o.GetSubclasses())
	}
	c, _ = f.levelUp(t, f.davi, c, "class:fighter", f.ink.GetKey())
	if d := c.GetDerived(); len(d.GetIssues()) != 0 || len(d.GetSpellcasting()) != 1 || d.GetSpellcasting()[0].GetSpellsKnown() != 3 || len(c.GetSheet().GetFull().GetKnownSpellKeys()) != 3 || len(c.GetSheet().GetFull().GetCantripKeys()) != 2 {
		t.Errorf("after level 3: issues %v, casting %v, sheet %v; want a clean third caster with 2 cantrips and 3 spells", d.GetIssues(), d.GetSpellcasting(), c.GetSheet().GetFull())
	}
}

// edit updates an entry's body as the master, from what the editor read.
func (f *classFlow) edit(t *testing.T, key string, change func(body proto.Message)) *rulesv1.UpdateTableEntryResponse {
	t.Helper()
	e := f.master.entries(t, f.campaign)[key]
	body := proto.Clone(bodyMessage(e))
	change(body)
	res, err := f.master.table.UpdateTableEntry(t.Context(), connect.NewRequest(updateReq(f.campaign, key, e.GetRevision(), body)))
	if err != nil {
		t.Fatalf("UpdateTableEntry(%s) error = %v", key, err)
	}
	return res.Msg
}

// notice is what the sheet's owner and the master read as "A classe mudou": the
// sentences of the changed entry, or none.
func (f *classFlow) notice(t *testing.T, c *charactersv1.Character, key string) []string {
	t.Helper()
	var out []string
	for _, u := range []*user{f.davi, f.master} {
		var got []string
		for _, ch := range u.get(t, f.campaign, c.GetId()).GetDerived().GetChangedContent() {
			if ch.GetKey() == key {
				got = ch.GetMessages()
			}
		}
		if out != nil && !slices.Equal(out, got) {
			t.Errorf("the owner and the master read different notices: %v and %v", out, got)
		}
		out = got
	}
	return out
}

// TestTableClassChangeIsTold (E10-02 state 8, question 80): the master changes a
// class the sheet uses and the sheet, locked as it is, recalculates at once and
// says, in one sentence, what no longer matches, for each common change: the skill
// count, the spells (cantrips, known and prepared), the level of the subclass, an
// option the class stopped offering. The notice goes away by itself when the numbers
// agree again.
func TestTableClassChangeIsTold(t *testing.T) {
	t.Parallel()
	f := newClassFlow(t)
	icaro := f.davi.create(t, f.campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Ícaro", f.icaroSheet())
	f.h.lockSheets(f.campaign)
	icaro = f.davi.get(t, f.campaign, icaro.GetId())
	for to := 2; to <= 3; to++ {
		icaro, _ = f.levelUp(t, f.davi, icaro, f.guardian.GetKey(), f.path.GetKey())
	}
	icaro = f.davi.get(t, f.campaign, icaro.GetId())
	key := f.guardian.GetKey()
	if len(f.notice(t, icaro, key)) != 0 || len(icaro.GetDerived().GetIssues()) != 0 {
		t.Fatalf("Ícaro starts with a notice or issues: %v", icaro.GetDerived().GetIssues())
	}
	prepared := len(icaro.GetSheet().GetFull().GetPreparedSpellKeys())
	if prepared < 2 {
		t.Fatalf("Ícaro prepared %d spells, want a few", prepared)
	}

	// The skills: the class gives 1 now, Ícaro chose 2.
	res := f.edit(t, key, func(b proto.Message) { b.(*rulesv1.TableClass).SkillChoose = 1 })
	want := "Guardião do Vale agora dá 1 perícia no nível 1; esta ficha tem 2."
	if got := f.notice(t, icaro, key); !slices.Equal(got, []string{want}) {
		t.Errorf("fewer skills: notice %q, want %q", got, want)
	}
	if a := res.GetAffectedCharacters(); len(a) != 1 || a[0].GetCharacterId() != icaro.GetId() || a[0].GetPlayerDisplayName() != "Davi" || a[0].GetIssues() != 1 {
		t.Errorf("affected = %v, want Ícaro of Davi with 1 issue", a)
	}
	if !f.davi.get(t, f.campaign, icaro.GetId()).GetSheetLockedAt().IsValid() {
		t.Error("the change unlocked the sheet")
	}
	// It goes away by itself when the numbers agree again.
	f.edit(t, key, func(b proto.Message) { b.(*rulesv1.TableClass).SkillChoose = 2 })
	if got := f.notice(t, icaro, key); len(got) != 0 {
		t.Errorf("skills back to 2: notice %q, want none", got)
	}
	f.edit(t, key, func(b proto.Message) { b.(*rulesv1.TableClass).SkillChoose = 3 })
	want = "Guardião do Vale agora dá 3 perícias no nível 1; esta ficha tem 2."
	if got := f.notice(t, icaro, key); !slices.Equal(got, []string{want}) {
		t.Errorf("more skills: notice %q, want %q", got, want)
	}
	f.edit(t, key, func(b proto.Message) { b.(*rulesv1.TableClass).SkillChoose = 2 })

	// The prepared spells: the class prepares 1 now.
	f.edit(t, key, func(b proto.Message) { b.(*rulesv1.TableClass).Casting.PreparedMax = "1" })
	// The Cure Wounds that the subclass now always prepares does not count against the maximum.
	counted := prepared
	if slices.Contains(icaro.GetSheet().GetFull().GetPreparedSpellKeys(), "spell:cure-wounds") {
		counted--
	}
	want = "Guardião do Vale agora prepara 1 magia; esta ficha tem " + strconv.Itoa(counted) + "."
	if got := f.notice(t, icaro, key); !slices.Equal(got, []string{want}) {
		t.Errorf("fewer prepared spells: notice %q, want %q", got, want)
	}
	f.edit(t, key, func(b proto.Message) { b.(*rulesv1.TableClass).Casting.PreparedMax = "" })
	if got := f.notice(t, icaro, key); len(got) != 0 {
		t.Errorf("prepared back to the default: notice %q, want none", got)
	}

	// The subclass level: the subclass first (its features move to level 5), then the class.
	f.edit(t, f.path.GetKey(), func(b proto.Message) {
		s := b.(*rulesv1.TableSubclass)
		s.Levels = s.Levels[1:] // the level-3 feature goes; the level-7 one stays
	})
	f.edit(t, key, func(b proto.Message) { b.(*rulesv1.TableClass).SubclassLevel = 5 })
	want = "Guardião do Vale agora escolhe a subclasse no nível 5; esta ficha tem nível 3 nela."
	if got := f.notice(t, icaro, key); !slices.Contains(got, want) {
		t.Errorf("subclass level moved: notice %q, want it to have %q", got, want)
	}
	f.edit(t, key, func(b proto.Message) { b.(*rulesv1.TableClass).SubclassLevel = 3 })
	if got := f.notice(t, icaro, key); len(got) != 0 {
		t.Errorf("subclass level back to 3: notice %q, want none", got)
	}

	// An option the class stopped offering: the feature of level 2 now offers other
	// fighting styles than the one Ícaro chose.
	styles := fightingStyles(t, f.menu)
	f.edit(t, key, func(b proto.Message) { b.(*rulesv1.TableClass).Levels[1].Features[0].Effects[0].From = styles[1:4] })
	got := f.notice(t, icaro, key)
	if len(got) != 1 || !strings.HasPrefix(got[0], "Guardião do Vale agora não oferece a escolha ") || !strings.HasSuffix(got[0], "; ela não vale mais nesta ficha.") {
		t.Errorf("a feature choice that no longer holds: notice %q, want %q", got, "Guardião do Vale agora não oferece a escolha ...; ela não vale mais nesta ficha.")
	}
}

// TestTableClassThirdCasterChangeIsTold: a third caster's subclass that gives fewer
// cantrips and spells tells the sheet's owner, naming the subclass (E10-02 state 8).
func TestTableClassThirdCasterChangeIsTold(t *testing.T) {
	t.Parallel()
	f := newClassFlow(t)
	c := f.davi.create(t, f.campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Cavaleiro", thirdCasterSheet(f.ink.GetKey()))
	f.h.lockSheets(f.campaign)
	key := f.ink.GetKey()
	f.edit(t, key, func(b proto.Message) { b.(*rulesv1.TableSubclass).Levels[0].CantripsKnown = 1 })
	if got, want := f.notice(t, c, key), []string{"Lâmina de Tinta agora conhece 1 truque; esta ficha tem 2."}; !slices.Equal(got, want) {
		t.Errorf("fewer cantrips: notice %q, want %q", got, want)
	}
	f.edit(t, key, func(b proto.Message) {
		s := b.(*rulesv1.TableSubclass)
		s.Levels[0].CantripsKnown = 2
		s.Levels[0].SpellsKnown = 2
	})
	if got, want := f.notice(t, c, key), []string{"Lâmina de Tinta agora conhece 2 magias; esta ficha tem 3."}; !slices.Equal(got, want) {
		t.Errorf("fewer known spells: notice %q, want %q", got, want)
	}
}

// TestTableContentStrayFieldNeverMakesTheCampaignUnreadable (review of slice 10.3):
// a field the effect's type does not read is refused when the entry is written, but
// an entry already stored with one is read as if the field were not there: the
// campaign's sheets and catalog keep working.
func TestTableContentStrayFieldNeverMakesTheCampaignUnreadable(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, owner := h.newUser("Samuel"), h.newUser("Dona")
	campaign := h.newCampaign(master, "Mirathel", owner)
	d, menu := master.classDefaults(t, campaign), master.effectMenu(t, campaign)
	class := master.addEntry(t, campaign, guardianClass(t, d, menu)) // level 1's first effect is a sense
	pc := owner.createPensantus(t, campaign)

	// Stored with a stray recharge, as an older version could have left it.
	if _, err := h.pool.Exec(t.Context(), `UPDATE campaign_content SET data = jsonb_set(data, '{levels,0,features,0,effects,0,recharge}', '"long_rest"') WHERE campaign_id = $1 AND content_key = $2`, campaign, class.GetKey()); err != nil {
		t.Fatalf("store the stray field: %v", err)
	}
	if _, err := h.pool.Exec(t.Context(), `UPDATE campaign_content_state SET revision = revision + 1 WHERE campaign_id = $1`, campaign); err != nil {
		t.Fatalf("bump the revision: %v", err)
	}
	entry := master.entries(t, campaign)[class.GetKey()]
	if entry == nil || bodyMessage(entry).(*rulesv1.TableClass).GetLevels()[0].GetFeatures()[0].GetEffects()[0].GetRecharge() != "long_rest" {
		t.Fatalf("the entry does not carry the stray field: %v", entry)
	}
	if got := owner.get(t, campaign, pc.GetId()); got.GetDerived().GetTotalLevel() != 3 {
		t.Errorf("GetCharacter after the stray field: level %d, want the sheet to read", got.GetDerived().GetTotalLevel())
	}
	list, err := owner.content.ListContent(t.Context(), connect.NewRequest(&rulesv1.ListContentRequest{CampaignId: campaign}))
	if err != nil || !slices.ContainsFunc(list.Msg.GetContent().GetClasses(), func(c *rulesv1.CharacterClass) bool { return c.GetKey() == class.GetKey() }) {
		t.Errorf("ListContent after the stray field: %v, %v; want the class in the catalog", list, err)
	}
	// Writing the same entry again is refused, on the field.
	_, err = master.table.UpdateTableEntry(t.Context(), connect.NewRequest(updateReq(campaign, class.GetKey(), entry.GetRevision(), bodyMessage(entry))))
	v := violationsOfErr(t, err)
	if len(v) != 1 || v[0].GetField() != "table_class.levels[0].features[0].effects[0].recharge" || v[0].GetReason() != "bad_value" {
		t.Errorf("violations = %v, want one at the stray field (bad_value)", v)
	}
}
