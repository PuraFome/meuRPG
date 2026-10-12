package play

import (
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// A table feat's ranged_attack effect, end to end (MR-025): the table's "Atirador de
// magia" (a spell that needs an attack roll reaches twice as far, and its ranged spell
// attacks see past half and three-quarters cover) and "Atirador de elite" (ranged weapon
// attacks see past that cover and keep their aim at long range). The SRD has neither
// feat: the table writes them with the effect, and the engine reads only the effect.
// These tests need the database (MEURPG_TEST_DATABASE_URL).

// sniperColumns is the grid of the fight's map: 40 x 20 squares (200 ft x 100 ft), so
// the goblin can stand beyond Fire Bolt's 120 ft and within twice that.
const sniperColumns = 40

type sniperTable struct {
	*armed
	spellSniper, sharpshooter string // the feats' keys
}

// newSniperTable makes the two feats, and the party with no feat yet: Pensantus, a
// level 3 wizard with Fire Bolt and Scorching Ray (120 ft each); Toren and Brisa,
// fighters with a shortbow (80 ft, 320 ft long). The fight is on a 40-column map.
func newSniperTable(t *testing.T) *sniperTable {
	t.Helper()
	s := &sniperTable{}
	s.armed = newArmedWith(t, func(a *armed) {
		feat := func(name string, e *rulesv1.TableEffect) string {
			res, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(&rulesv1.CreateTableEntryRequest{
				CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableFeat{TableFeat: &rulesv1.TableFeat{
					NamePt: name, DescPt: []string{"Um talento da mesa."}, Effects: []*rulesv1.TableEffect{e},
				}},
			}))
			if err != nil {
				t.Fatalf("CreateTableEntry(%s) error = %v", name, err)
			}
			return res.Msg.GetEntry().GetKey()
		}
		s.spellSniper = feat("Atirador de magia", &rulesv1.TableEffect{Type: "ranged_attack", Applies: "spell", RangeMultiplier: 2, IgnoreCover: true})
		s.sharpshooter = feat("Atirador de elite", &rulesv1.TableEffect{Type: "ranged_attack", Applies: "weapon", IgnoreCover: true, NoLongRangeDisadvantage: true})
		archer := &rulesv1.AbilityScores{Strength: 12, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 2, archer, []string{shortbowKey}, nil)
		a.pens = a.ana.caster(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 3,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, nil, []string{fireBolt},
			[]string{scorchingRay, magicMissileSpell}, []string{scorchingRay, magicMissileSpell})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, archer, []string{shortbowKey}, nil)
	})
	mapID := s.h.newMap(s.campaignID, sniperColumns)
	if _, err := s.master.play.SetCurrentMap(t.Context(), connect.NewRequest(&playv1.SetCurrentMapRequest{CampaignId: s.campaignID, MapId: mapID})); err != nil {
		t.Fatalf("SetCurrentMap() error = %v", err)
	}
	return s
}

// fight starts the combat: Pensantus, Brisa, Toren, then the goblin (revealed). The
// goblin stands 150 ft from Pensantus and 100 ft from each fighter, with nobody on
// any of the lines.
func (s *sniperTable) fight(t *testing.T) *playv1.Encounter {
	t.Helper()
	return s.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: s.goblin.GetId()}}, npcRolls: []int{1}, reveal: []string{"Goblin"},
		players: map[string]int32{"Pensantus": 20, "Brisa": 15, "Toren": 10},
		at:      map[string][2]int32{"Pensantus": {2, 10}, "Goblin": {32, 10}, "Brisa": {12, 14}, "Toren": {12, 6}},
	})
}

// giveFeat is the master adding a feat to a sheet, which he may do at any time (MR-025).
func (s *sniperTable) giveFeat(t *testing.T, c *charactersv1.Character, feat string) {
	t.Helper()
	cur := s.master.character(t, c)
	sheet := cur.GetSheet()
	sheet.GetFull().FeatKeys = append(sheet.GetFull().GetFeatKeys(), feat)
	if _, err := s.master.characters.UpdateCharacter(t.Context(), connect.NewRequest(&charactersv1.UpdateCharacterRequest{
		CampaignId: s.campaignID, CharacterId: cur.GetId(), Revision: cur.GetRevision(), Name: cur.GetName(), Sheet: sheet,
	})); err != nil {
		t.Fatalf("UpdateCharacter(%s gets %s) error = %v", cur.GetName(), feat, err)
	}
}

// markHalf is the master's half cover mark on a combatant.
func (s *sniperTable) markHalf(t *testing.T, label string) {
	t.Helper()
	if _, err := s.master.combat.SetCombatantCover(t.Context(), connect.NewRequest(&playv1.SetCombatantCoverRequest{
		CampaignId: s.campaignID, EncounterId: s.get(t, s.master).GetId(), CombatantId: s.id(t, label), IdempotencyKey: newKey(),
		Cover: playv1.CoverDegree_COVER_DEGREE_HALF,
	})); err != nil {
		t.Fatalf("SetCombatantCover(%s) error = %v", label, err)
	}
}

// lastEntry is the master's last log entry of a kind.
func (s *sniperTable) lastEntry(t *testing.T, kind playv1.CombatLogKind) *playv1.CombatLogEntry {
	t.Helper()
	// The log lists the rounds latest first, each round's entries in order: the last entry of
	// the kind is the last one of the first round that has any.
	var last *playv1.CombatLogEntry
	for _, r := range s.log(t, s.master, s.get(t, s.master)).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == kind {
				last = en
			}
		}
		if last != nil {
			break
		}
	}
	if last == nil {
		t.Fatalf("no %v in the master's log", kind)
	}
	return last
}

// attackOf is the master's log entry of the latest attack the combatant with the label made.
func (s *sniperTable) attackOf(t *testing.T, label string) *playv1.CombatLogEntry {
	t.Helper()
	for _, r := range s.log(t, s.master, s.get(t, s.master)).GetRounds() { // the latest round first
		var last *playv1.CombatLogEntry
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK && en.GetActorLabel() == label {
				last = en
			}
		}
		if last != nil {
			return last
		}
	}
	return nil
}

// attacksInLog lists the attacks of the master's log, for a failure's message.
func (s *sniperTable) attacksInLog(t *testing.T) string {
	t.Helper()
	var out []string
	for _, r := range s.log(t, s.master, s.get(t, s.master)).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK {
				out = append(out, fmt.Sprintf("round %d %s: AC %d, cover %d", en.GetRound(), en.GetActorLabel(), en.GetTargetArmorClass(), en.GetCoverBonus()))
			}
		}
	}
	return strings.Join(out, "; ")
}

func hasSource(tg *playv1.TargetInReach, kind playv1.AdvantageSourceKind) bool {
	for _, src := range tg.GetSources() {
		if src.GetKind() == kind {
			return true
		}
	}
	return false
}

// TestMR025_ATableFeatDoublesTheSpellAttackRangeAndSeesPastCover: without the feat,
// Fire Bolt (an attack cantrip, rolled with RollAttack) and Scorching Ray (cast with
// CastSpell), 120 ft each, cannot reach the goblin 150 ft away: by the lists, by
// RollAttack and by CastSpell. With it they reach 240 ft, the half cover the master
// marked is not in their lists, and neither roll's armor class has the +2 (the events
// say no cover). A fighter with no feat still sees the cover.
func TestMR025_ATableFeatDoublesTheSpellAttackRangeAndSeesPastCover(t *testing.T) {
	t.Parallel()
	s := newSniperTable(t)
	e := s.fight(t)
	inApp := func(r *playv1.CastSpellRequest) { r.Roll = &playv1.CastSpellRequest_RollInApp{RollInApp: true} }

	opts := s.mustOptions(t, s.ana, e, "Pensantus")
	if tg := targetOf(opts, fireBolt, "Goblin"); tg == nil || tg.GetDistanceFt() != 150 || !tg.GetTooFar() {
		t.Fatalf("Fire Bolt's target with no feat = %v, want the goblin 150 ft away and too far", tg)
	}
	if tg := targetOf2(spellTargetsOf(opts, scorchingRay), "Goblin"); tg == nil || !tg.GetTooFar() {
		t.Fatalf("Scorching Ray's target with no feat = %v, want too far", tg)
	}
	_, err := s.attack(t, s.ana, e, "Pensantus", fireBolt, "Goblin", d20(15))
	if b := wantBlockedBy(t, "Fire Bolt at 150 ft with no feat", err, blockedOutOfReach); b.GetMissingFt() != 30 {
		t.Errorf("Fire Bolt misses %d ft, want 30 (150 - 120)", b.GetMissingFt())
	}
	_, err = s.cast(t, s.ana, e, "Pensantus", scorchingRay, slotOfLevel(2), s.at(t, "Goblin"), inApp)
	if b := wantBlockedBy(t, "Scorching Ray at 150 ft with no feat", err, blockedOutOfReach); b.GetMissingFt() != 30 {
		t.Errorf("Scorching Ray misses %d ft, want 30 (150 - 120)", b.GetMissingFt())
	}

	// The master gives Pensantus the feat (twice the range for the spells with an attack
	// roll, past half and three-quarters cover), and marks half cover on the goblin.
	s.giveFeat(t, s.pens, s.spellSniper)
	s.markHalf(t, "Goblin")
	opts = s.mustOptions(t, s.ana, s.get(t, s.ana), "Pensantus")
	bolt := targetOf(opts, fireBolt, "Goblin")
	if bolt == nil || bolt.GetTooFar() || bolt.GetCover() != playv1.CoverDegree_COVER_DEGREE_NONE {
		t.Errorf("Fire Bolt's target with the feat = %v, want in reach and no cover (the half cover is ignored)", bolt)
	}
	if bolt != nil && hasSource(bolt, playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_LONG_RANGE) {
		t.Errorf("Fire Bolt within its doubled range has the long range disadvantage: %v", bolt.GetSources())
	}
	if ray := targetOf2(spellTargetsOf(opts, scorchingRay), "Goblin"); ray == nil || ray.GetTooFar() || ray.GetCover() != playv1.CoverDegree_COVER_DEGREE_NONE {
		t.Errorf("Scorching Ray's target with the feat = %v, want in reach and no cover", ray)
	}
	// The mark is there: a fighter with no feat sees it.
	if tg := targetOf(s.mustOptions(t, s.caio, s.get(t, s.caio), "Toren"), shortbowKey, "Goblin"); tg == nil || tg.GetCover() != playv1.CoverDegree_COVER_DEGREE_HALF {
		t.Errorf("Toren's shortbow target = %v, want half cover", tg)
	}

	hit := s.mustAttack(t, s.ana, s.get(t, s.ana), "Pensantus", fireBolt, "Goblin", d20(15))
	if hit.GetRoll().GetCover() != playv1.CoverDegree_COVER_DEGREE_NONE {
		t.Errorf("Fire Bolt's roll = %v, want no cover", hit.GetRoll())
	}
	if en := s.lastEntry(t, playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK); en.GetTargetArmorClass() != 12 || en.GetCoverBonus() != 0 {
		t.Errorf("the master's log of Fire Bolt = armor class %d, cover bonus %d; want 12 and 0", en.GetTargetArmorClass(), en.GetCoverBonus())
	}
	s.undoLast(t, s.get(t, s.master)) // the action again, for the cast

	cast := s.mustCast(t, s.ana, s.get(t, s.ana), "Pensantus", scorchingRay, slotOfLevel(2), s.at(t, "Goblin"), inApp)
	if hits := cast.GetCast().GetTargets(); len(hits) != 1 || hits[0].GetCover() != playv1.CoverDegree_COVER_DEGREE_NONE {
		t.Errorf("the cast's target = %v, want the goblin with no cover", hits)
	}
	en := s.lastEntry(t, playv1.CombatLogKind_COMBAT_LOG_KIND_SPELL_CAST)
	if targets := en.GetSpell().GetTargets(); len(targets) != 1 || targets[0].GetTargetArmorClass() != 12 || targets[0].GetCoverBonus() != 0 {
		t.Errorf("the master's log of the cast = %v, want armor class 12 with no cover bonus", targets)
	}
}

// TestMR025_ATableFeatLetsRangedWeaponsIgnoreCoverAndLongRange: Toren (no feat) and
// Brisa (the feat) shoot a shortbow at the goblin 100 ft away, beyond the normal 80 ft,
// with the master's half cover mark on it. Toren's list says disadvantage for the long
// range and half cover, and his roll is against 12 + 2; Brisa's says neither, and her
// roll is against 12, with no cover in the event.
func TestMR025_ATableFeatLetsRangedWeaponsIgnoreCoverAndLongRange(t *testing.T) {
	t.Parallel()
	s := newSniperTable(t)
	s.giveFeat(t, s.bri, s.sharpshooter)
	e := s.fight(t)
	s.markHalf(t, "Goblin")

	toren := targetOf(s.mustOptions(t, s.caio, s.get(t, s.caio), "Toren"), shortbowKey, "Goblin")
	if toren == nil || toren.GetTooFar() || toren.GetRollMode() != playv1.RollMode_ROLL_MODE_DISADVANTAGE ||
		!hasSource(toren, playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_LONG_RANGE) || toren.GetCover() != playv1.CoverDegree_COVER_DEGREE_HALF {
		t.Errorf("Toren's target = %v, want in reach, at long range (disadvantage) and half cover", toren)
	}
	brisa := targetOf(s.mustOptions(t, s.bia, s.get(t, s.bia), "Brisa"), shortbowKey, "Goblin")
	if brisa == nil || brisa.GetTooFar() || brisa.GetRollMode() != playv1.RollMode_ROLL_MODE_NORMAL ||
		hasSource(brisa, playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_LONG_RANGE) || brisa.GetCover() != playv1.CoverDegree_COVER_DEGREE_NONE {
		t.Errorf("Brisa's target = %v, want in reach, a normal roll and no cover", brisa)
	}

	e = s.passTo(t, e, "Brisa")
	hit := s.mustAttack(t, s.bia, e, "Brisa", shortbowKey, "Goblin", d20(15))
	if hit.GetRoll().GetCover() != playv1.CoverDegree_COVER_DEGREE_NONE {
		t.Errorf("Brisa's roll = %v, want no cover", hit.GetRoll())
	}
	if en := s.lastEntry(t, playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK); en.GetTargetArmorClass() != 12 || en.GetCoverBonus() != 0 {
		t.Errorf("the master's log of Brisa's shot = armor class %d, cover bonus %d; want 12 and 0", en.GetTargetArmorClass(), en.GetCoverBonus())
	}

	e = s.passTo(t, e, "Toren")
	hit = s.mustAttack(t, s.caio, e, "Toren", shortbowKey, "Goblin", disadvantage(15))
	if hit.GetRoll().GetCover() != playv1.CoverDegree_COVER_DEGREE_HALF {
		t.Errorf("Toren's roll = %v, want half cover", hit.GetRoll())
	}
	if en := s.attackOf(t, "Toren"); en == nil || en.GetTargetArmorClass() != 14 || en.GetCoverBonus() != 2 {
		t.Errorf("the master's log of Toren's shot = %v; want armor class 14 = 12 + 2, cover bonus 2; the attacks in the log: %s", en, s.attacksInLog(t))
	}
}

// TestRangedAttackCoverIsIgnoredOnlyAtRange: which attack sees past cover, by the
// attacker's switch and the attack's shape (no database). A weapon switch is for a
// ranged weapon and a thrown weapon beyond 5 ft (never in melee, nor without a map,
// where a thrown weapon is a melee one); a spell switch is for a spell attack with a
// range, never a touch. Half and three-quarters cover become none, in the roll and in
// the pair the event keeps; total cover stays.
func TestRangedAttackCoverIsIgnoredOnlyAtRange(t *testing.T) {
	t.Parallel()
	weapons := link.Traits{RangedWeaponIgnoresCover: true}
	spells := link.Traits{SpellAttackIgnoresCover: true}
	bow := shapeOfAttack(link.Attack{Weapon: true, RangeFt: 80, LongRangeFt: 320})
	dagger := shapeOfAttack(link.Attack{Weapon: true, Melee: true, RangeFt: 20, LongRangeFt: 60})
	sword := shapeOfAttack(link.Attack{Weapon: true, Melee: true, RangeFt: 5})
	bolt := shapeOfAttack(link.Attack{Spell: true, RangeFt: 120})
	touch := shapeOfAttack(link.Attack{Spell: true})
	for _, tc := range []struct {
		name   string
		traits link.Traits
		shape  attackShape
		dist   int32
		known  bool
		want   bool
	}{
		{"a bow, for the sharpshooter", weapons, bow, 100, true, true},
		{"a bow without a map", weapons, bow, 0, false, true},
		{"a dagger thrown 30 ft", weapons, dagger, 30, true, true},
		{"a dagger in melee", weapons, dagger, 5, true, false},
		{"a dagger without a map", weapons, dagger, 0, false, false},
		{"a sword", weapons, sword, 5, true, false},
		{"Fire Bolt, for the sharpshooter", weapons, bolt, 100, true, false},
		{"a bow, for the spell sniper", spells, bow, 100, true, false},
		{"Fire Bolt, for the spell sniper", spells, bolt, 100, true, true},
		{"a touch spell, for the spell sniper", spells, touch, 5, true, false},
		{"a bow, with no switch", link.Traits{}, bow, 100, true, false},
	} {
		if got := ignoresCover(tc.traits, tc.shape, tc.shape.rangedAt(tc.dist, tc.known)); got != tc.want {
			t.Errorf("%s: ignoresCover = %v, want %v", tc.name, got, tc.want)
		}
	}
	for _, tc := range []struct {
		name string
		sp   link.Spell
		want bool
	}{
		{"a ranged spell attack", link.Spell{AttackType: "ranged", RangeFt: 120}, true},
		{"a melee spell attack", link.Spell{AttackType: "melee"}, false},
		{"a Dexterity save", link.Spell{SaveAbility: "dex", RangeFt: 150}, false},
	} {
		if got := spellIgnoresCover(spells, tc.sp); got != tc.want {
			t.Errorf("%s: spellIgnoresCover = %v, want %v", tc.name, got, tc.want)
		}
	}
	if spellIgnoresCover(weapons, link.Spell{AttackType: "ranged"}) {
		t.Error("the weapon switch makes a spell attack ignore cover")
	}

	half := coverView{degree: grid.CoverHalf, source: playv1.CoverSource_COVER_SOURCE_MAP}
	if got := half.ignoring(true); got.degree != grid.CoverNone || got.bonus() != 0 || got.key() != "" || got.sourceKey() != "" {
		t.Errorf("half cover ignored = %+v, want none", got)
	}
	if got := half.ignoring(false); got != half {
		t.Errorf("half cover not ignored = %+v, want it as it was", got)
	}
	total := coverView{degree: grid.CoverTotal, source: playv1.CoverSource_COVER_SOURCE_MARK}
	if got := total.ignoring(true); !got.total() {
		t.Errorf("total cover ignored = %+v, want it total still", got)
	}
	three := coverView{degree: grid.CoverThreeQuarters, source: playv1.CoverSource_COVER_SOURCE_MAP}
	pair := coverPair{real: three, shown: three, restricted: true, seenBy: []string{"u1"}}
	if got := pair.ignoring(true); got.real.degree != grid.CoverNone || got.shown.degree != grid.CoverNone || got.restricted || got.seenBy != nil {
		t.Errorf("three-quarters cover ignored = %+v, want none, unrestricted", got)
	}
	listed := &playv1.TargetInReach{Cover: playv1.CoverDegree_COVER_DEGREE_HALF, CoverSource: playv1.CoverSource_COVER_SOURCE_MARK}
	clearPartialCover(listed)
	if listed.GetCover() != playv1.CoverDegree_COVER_DEGREE_NONE || listed.GetCoverSource() != playv1.CoverSource_COVER_SOURCE_UNSPECIFIED {
		t.Errorf("a listed half cover cleared = %v, want none", listed)
	}
	walled := &playv1.TargetInReach{Cover: playv1.CoverDegree_COVER_DEGREE_TOTAL, CoverSource: playv1.CoverSource_COVER_SOURCE_MAP, Untargetable: true}
	clearPartialCover(walled)
	if walled.GetCover() != playv1.CoverDegree_COVER_DEGREE_TOTAL {
		t.Errorf("a listed total cover cleared = %v, want it total", walled)
	}
}
