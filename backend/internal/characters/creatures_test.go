package characters

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// TestCreatureToProto: the stat block of the Etapa 9 designs (Sálvia's wolf)
// reaches the message with its Portuguese labels and the SRD's English text.
func TestCreatureToProto(t *testing.T) {
	t.Parallel()
	c, ok := loadRules(t).CreatureByKey("monster:wolf")
	if !ok {
		t.Fatal("no wolf")
	}
	w := creatureToProto(c)
	if s := w.GetSummary(); s.GetKey() != "monster:wolf" || s.GetNamePt() != "Lobo" || s.GetName() != "Wolf" || s.GetSizePt() != "Médio" ||
		s.GetTypePt() != "fera" || s.GetChallengeRating() != "1/4" || s.GetXp() != 50 || s.GetCanFly() || s.GetCanSwim() {
		t.Errorf("summary = %v", s)
	}
	if w.GetArmorClass() != 13 || w.GetArmorClassLabelPt() != "Armadura natural" || w.GetHitPoints() != 11 || w.GetHitDice() != "2d8" ||
		w.GetHitPointsRoll() != "2d8+2" || w.GetSpeedWalkFt() != 40 || w.GetSpeedFlyFt() != 0 || w.GetPassivePerception() != 13 || w.GetProficiencyBonus() != 2 {
		t.Errorf("numbers = %v", w)
	}
	if len(w.GetAbilities()) != 6 || w.GetAbilities()[0].GetAbility() != rulesv1.Ability_ABILITY_STRENGTH || w.GetAbilities()[0].GetScore() != 12 ||
		w.GetAbilities()[1].GetModifier() != 2 || w.GetAbilities()[0].GetNamePt() != "Força" {
		t.Errorf("abilities = %v", w.GetAbilities())
	}
	if len(w.GetSkills()) != 2 || w.GetSkills()[0].GetKey() != "skill:stealth" && w.GetSkills()[0].GetKey() != "skill:perception" {
		t.Errorf("skills = %v", w.GetSkills())
	}
	if len(w.GetTraits()) != 2 || w.GetTraits()[1].GetName() != "Pack Tactics" || len(w.GetActions()) != 1 {
		t.Fatalf("traits = %v, actions = %v", w.GetTraits(), w.GetActions())
	}
	bite := w.GetActions()[0]
	if !bite.GetHasAttack() || bite.GetAttackBonus() != 4 || len(bite.GetDamage()) != 1 || bite.GetDamage()[0].GetDice() != "2d4+2" ||
		bite.GetDamage()[0].GetDamageTypePt() != "perfurante" || bite.GetSave().GetDc() != 11 ||
		bite.GetSave().GetAbility() != rulesv1.Ability_ABILITY_STRENGTH || bite.GetSave().GetOnSuccess() != rulesv1.SpellSaveSuccess_SPELL_SAVE_SUCCESS_NONE {
		t.Errorf("bite = %v", bite)
	}

	// A Multiattack, a resistance with a note and the senses.
	drag, _ := loadRules(t).CreatureByKey("monster:adult-black-dragon")
	d := creatureToProto(drag)
	var multi *rulesv1.CreatureAction
	for _, a := range d.GetActions() {
		if a.GetName() == "Multiattack" {
			multi = a
		}
	}
	if multi == nil || len(multi.GetMultiattack()) != 1 || len(multi.GetMultiattack()[0].GetAttacks()) != 3 || multi.GetMultiattack()[0].GetAttacks()[2].GetCount() != 2 {
		t.Errorf("multiattack = %v", multi)
	}
	if d.GetSenses()[0].GetSense() != "darkvision" && d.GetSenses()[0].GetSense() != "blindsight" {
		t.Errorf("a creature's sense says which one: %v", d.GetSenses())
	}
	if len(d.GetImmunities()) != 1 || d.GetImmunities()[0].GetTypes()[0].GetNamePt() == "" || len(d.GetSenses()) != 2 {
		t.Errorf("immunities = %v, senses = %v", d.GetImmunities(), d.GetSenses())
	}
	sk, _ := loadRules(t).CreatureByKey("monster:skeleton")
	if got := creatureToProto(sk); got.GetArmorClassNote() != "armor scraps" || len(got.GetVulnerabilities()) != 1 {
		t.Errorf("skeleton = %v", got)
	}
}

// TestDerivedSheetSpeeds: a creature's other speeds and its saving-throw actions
// reach the DerivedSheet.
func TestDerivedSheetSpeeds(t *testing.T) {
	t.Parallel()
	gob, _ := loadRules(t).MonsterDerived("monster:goblin")
	if s := derivedToProto(gob).GetSenses(); len(s) != 1 || s[0].GetSense() != "darkvision" || s[0].GetKey() != "monster:goblin" {
		t.Errorf("goblin senses = %v", s)
	}
	raven, _ := loadRules(t).MonsterDerived("monster:raven")
	sheet := derivedToProto(raven)
	if sheet.GetSpeedWalkFt() != 10 || sheet.GetSpeedFlyFt() != 50 || sheet.GetHover() {
		t.Errorf("raven speeds = walk %d, fly %d, hover %v", sheet.GetSpeedWalkFt(), sheet.GetSpeedFlyFt(), sheet.GetHover())
	}
	wolf, _ := loadRules(t).MonsterDerived("monster:wolf")
	sheet = derivedToProto(wolf)
	if len(sheet.GetSaveActions()) != 1 || sheet.GetSaveActions()[0].GetDc() != 11 || sheet.GetAttacks()[0].GetNotes() == "" {
		t.Errorf("wolf save actions = %v, attack notes = %q", sheet.GetSaveActions(), sheet.GetAttacks()[0].GetNotes())
	}
}

func TestCreaturesToken(t *testing.T) {
	t.Parallel()
	for _, n := range []int{0, 50, 333} {
		if got, ok := parseCreaturesToken(creaturesToken(n, "abc"), "abc"); !ok || got != n {
			t.Errorf("token of %d = %d, %v", n, got, ok)
		}
	}
	if _, ok := parseCreaturesToken(creaturesToken(5, "abc"), "other"); ok {
		t.Error("a token is only good for its own filters")
	}
	for _, bad := range []string{"x", "!!", creaturesToken(-1, "abc"), "Yw"} {
		if _, ok := parseCreaturesToken(bad, "abc"); ok {
			t.Errorf("%q is not a token", bad)
		}
	}
}

// TestListAndGetCreatures: any active member reads the SRD creatures, with
// filters and pages; bad input is invalid_argument, an unknown key not_found.
func TestListAndGetCreatures(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, "Mirathel", player)
	list := func(u *user, req *rulesv1.ListCreaturesRequest) (*rulesv1.ListCreaturesResponse, error) {
		req.CampaignId = campaign
		res, err := u.content.ListCreatures(t.Context(), connect.NewRequest(req))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}

	all, err := list(player, &rulesv1.ListCreaturesRequest{})
	if err != nil || all.GetTotal() != 334 || len(all.GetCreatures()) != 50 || all.GetNextPageToken() == "" {
		t.Fatalf("first page = %d rows of %d, token %q, %v; want 50 of 334", len(all.GetCreatures()), all.GetTotal(), all.GetNextPageToken(), err)
	}
	// Every page, once each.
	seen, token := map[string]bool{}, ""
	for pages := 0; pages < 10; pages++ {
		page, err := list(master, &rulesv1.ListCreaturesRequest{PageSize: 100, PageToken: token})
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range page.GetCreatures() {
			if seen[c.GetKey()] {
				t.Errorf("%s came twice", c.GetKey())
			}
			seen[c.GetKey()] = true
		}
		if token = page.GetNextPageToken(); token == "" {
			break
		}
	}
	if len(seen) != 334 {
		t.Errorf("the pages hold %d creatures, want 334", len(seen))
	}

	wolves, err := list(player, &rulesv1.ListCreaturesRequest{Query: "lobo", Type: "beast", MaxCr: "1", NoFly: true})
	if err != nil || wolves.GetTotal() < 2 {
		t.Fatalf("wolves = %v, %v", wolves, err)
	}
	for _, c := range wolves.GetCreatures() {
		if c.GetCanFly() || c.GetType() != "beast" {
			t.Errorf("%s does not pass the filter", c.GetKey())
		}
	}
	if empty, err := list(player, &rulesv1.ListCreaturesRequest{Type: "no-such-type"}); err != nil || empty.GetTotal() != 0 || len(empty.GetCreatures()) != 0 {
		t.Errorf("an unknown type gives an empty list: %v, %v", empty, err)
	}
	for name, req := range map[string]*rulesv1.ListCreaturesRequest{
		"a bad max_cr":             {MaxCr: "1/3"},
		"a bad min_cr":             {MinCr: "1/3"},
		"a negative size":          {PageSize: -1},
		"a huge page":              {PageSize: 401},
		"a bad token":              {PageToken: "nope"},
		"a token past end":         {PageToken: creaturesToken(9999, creaturesFilterID(&rulesv1.ListCreaturesRequest{}))},
		"a long query":             {Query: strings.Repeat("a", 101)},
		"a token of other filters": {Query: "lobo", PageToken: creaturesToken(1, creaturesFilterID(&rulesv1.ListCreaturesRequest{}))},
		"a token of a kind":        {PageToken: creaturesToken(-5, creaturesFilterID(&rulesv1.ListCreaturesRequest{}))},
	} {
		_, err := list(player, req)
		wantCode(t, name, err, connect.CodeInvalidArgument)
	}

	get := func(u *user, key string) (*rulesv1.Creature, error) {
		res, err := u.content.GetCreature(t.Context(), connect.NewRequest(&rulesv1.GetCreatureRequest{CampaignId: campaign, Key: key}))
		if err != nil {
			return nil, err
		}
		return res.Msg.GetCreature(), nil
	}
	if w, err := get(player, "monster:dire-wolf"); err != nil || w.GetSummary().GetNamePt() != "Lobo atroz" || w.GetHitPoints() != 37 {
		t.Errorf("GetCreature(dire wolf) = %v, %v", w, err)
	}
	for _, key := range []string{"monster:nope", "", "spell:fireball"} {
		_, err := get(player, key)
		wantCode(t, "key "+key, err, connect.CodeNotFound)
	}
	_, err = h.anonymous().content.GetCreature(t.Context(), connect.NewRequest(&rulesv1.GetCreatureRequest{CampaignId: campaign, Key: "monster:wolf"}))
	wantCode(t, "no session", err, connect.CodeUnauthenticated)
	_, err = h.newUser("De fora").content.ListCreatures(t.Context(), connect.NewRequest(&rulesv1.ListCreaturesRequest{CampaignId: campaign}))
	wantCode(t, "a non-member", err, connect.CodeNotFound)
}
