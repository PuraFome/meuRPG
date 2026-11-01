package characters

import (
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// A table creature whose numbers are formulas of its owner (issue #578, slice 2a): an SRD wolf with 5 hit
// points per level, an armor class of 10 + proficiency + Dexterity, and a claw that hits with proficiency +
// Strength. These tests need the database (MEURPG_TEST_DATABASE_URL).

const (
	silverWolfKey = "creature:lobo-de-prata@mesa"
	clawKey       = silverWolfKey + "#garra-de-prata"
)

func silverWolf() *rulesv1.TableCreature {
	return &rulesv1.TableCreature{
		NamePt: "Lobo de Prata", BaseKey: "monster:wolf",
		HitPointsFormula: "5 * level()", ArmorClassFormula: `10 + prof() + mod("dex")`,
		Attacks: []*rulesv1.TableCreatureAttack{{
			NamePt: "Garra de Prata", Melee: true, DiceCount: 1, DiceSides: 6, DamageType: "damage-type:slashing",
			ToHit: `prof() + mod("str")`, DamageBonus: `mod("str")`,
		}},
	}
}

// creatureNumbers is what a combat reads of a creature: armor class, and the to-hit and damage bonus of the claw.
type creatureNumbers struct{ hp, ac, toHit, damageBonus int }

// numbersOf reads the creature through the list (hit points, armor class) and the roster's sheet (the claw).
func (tb *creatureTable) numbersOf(t *testing.T, u *user, owner *charactersv1.Character, id string) creatureNumbers {
	t.Helper()
	res, err := u.api.ListCharacterCreatures(t.Context(), connect.NewRequest(&charactersv1.ListCharacterCreaturesRequest{CampaignId: tb.campaign, CharacterId: owner.GetId()}))
	if err != nil {
		t.Fatalf("ListCharacterCreatures() error = %v", err)
	}
	var out creatureNumbers
	found := false
	for _, c := range res.Msg.GetCreatures() {
		if c.GetId() == id {
			out.hp, out.ac, found = int(c.GetHitPointsMax()), int(c.GetArmorClass()), true
		}
	}
	if !found {
		t.Fatalf("creature %s is not listed", id)
	}
	var sheet link.Sheet
	err = db.InTx(t.Context(), tb.h.pool, func(tx pgx.Tx) error {
		var ok bool
		var err error
		sheet, ok, err = tb.h.svc.CreatureSheet(t.Context(), tx, tb.campaign, id, silverWolfKey, "full")
		if err == nil && !ok {
			t.Fatal("CreatureSheet() does not know the creature")
		}
		return err
	})
	if err != nil {
		t.Fatalf("CreatureSheet() error = %v", err)
	}
	if sheet.ArmorClass != out.ac {
		t.Errorf("the sheet's armor class %d differs from the list's %d", sheet.ArmorClass, out.ac)
	}
	i := slices.IndexFunc(sheet.Attacks, func(a link.Attack) bool { return a.Key == clawKey })
	if i < 0 {
		t.Fatalf("the sheet's attacks %v lack the claw", sheet.Attacks)
	}
	out.toHit, out.damageBonus = sheet.Attacks[i].ToHit, sheet.Attacks[i].DiceBonus
	return out
}

func (tb *creatureTable) statBlockOf(t *testing.T, id string) []byte {
	t.Helper()
	var raw []byte
	if err := tb.h.pool.QueryRow(t.Context(), "SELECT stat_block FROM character_creatures WHERE id = $1", id).Scan(&raw); err != nil {
		t.Fatalf("read the stat block: %v", err)
	}
	return raw
}

func TestMR578_AFormulaCreatureHasTheNumbersOfItsOwner(t *testing.T) {
	t.Parallel()
	tb := newCreatureTable(t)
	fighter := tb.other.create(t, tb.campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Toren", levelUpFighterSheet(5, 6500, ""))
	tb.master.addEntry(t, tb.campaign, silverWolf())
	tb.master.addEntry(t, tb.campaign, tableWolf())

	give := func(c *charactersv1.Character, key string) *charactersv1.CharacterCreature {
		t.Helper()
		got, err := tb.give(tb.master, &charactersv1.GiveCreatureRequest{CharacterId: c.GetId(), MonsterKey: key})
		if err != nil {
			t.Fatalf("GiveCreature(%s) error = %v", key, err)
		}
		return got
	}
	wizardWolf, fighterWolf := give(tb.pc, silverWolfKey), give(fighter, silverWolfKey)

	// Pensantus: level 3, proficiency +2, Dexterity 16 (+3), Strength 12 (+1).
	want := creatureNumbers{hp: 15, ac: 15, toHit: 3, damageBonus: 1}
	if got := tb.numbersOf(t, tb.owner, tb.pc, wizardWolf.GetId()); got != want {
		t.Errorf("the wizard's wolf = %+v, want %+v", got, want)
	}
	// Toren: level 5, proficiency +3, Dexterity 13 (+1), Strength 16 (+3).
	wantFighter := creatureNumbers{hp: 25, ac: 14, toHit: 6, damageBonus: 3}
	if got := tb.numbersOf(t, tb.other, fighter, fighterWolf.GetId()); got != wantFighter {
		t.Errorf("the fighter's wolf = %+v, want %+v", got, wantFighter)
	}
	if wizardWolf.GetHitPointsMax() != 15 || wizardWolf.GetHitPointsCurrent() != 15 || fighterWolf.GetHitPointsMax() != 25 {
		t.Errorf("the gifts' hit points = %d/%d and %d, want 15/15 and 25", wizardWolf.GetHitPointsCurrent(), wizardWolf.GetHitPointsMax(), fighterWolf.GetHitPointsMax())
	}
	// The master reads the same numbers.
	if got := tb.numbersOf(t, tb.master, tb.pc, wizardWolf.GetId()); got != want {
		t.Errorf("the master reads %+v, want %+v", got, want)
	}

	// The snapshot is kept: the owner levels up and the master edits the formulas, and the creature stays.
	up := pensantusSheet()
	up.GetFull().Classes[0].Level = 5
	up.GetFull().ExperiencePoints = 6500
	if _, err := tb.master.update(t, tb.master.get(t, tb.campaign, tb.pc.GetId()), "Pensantus", up); err != nil {
		t.Fatalf("raising the owner's level: %v", err)
	}
	entry := tb.master.entries(t, tb.campaign)[silverWolfKey]
	next := silverWolf()
	next.HitPointsFormula, next.ArmorClassFormula = "100", "30"
	next.Attacks[0].ToHit, next.Attacks[0].DamageBonus = "9", "9"
	if _, err := tb.master.table.UpdateTableEntry(t.Context(), connect.NewRequest(updateReq(tb.campaign, silverWolfKey, entry.GetRevision(), next))); err != nil {
		t.Fatalf("UpdateTableEntry() error = %v", err)
	}
	if got := tb.numbersOf(t, tb.owner, tb.pc, wizardWolf.GetId()); got != want {
		t.Errorf("after the owner's level-up and the edit: %+v, want the snapshot %+v", got, want)
	}
	// A creature given now has the new formulas and the new level.
	later := give(tb.pc, silverWolfKey)
	if got := tb.numbersOf(t, tb.owner, tb.pc, later.GetId()); got != (creatureNumbers{hp: 100, ac: 30, toHit: 9, damageBonus: 9}) {
		t.Errorf("a creature given after the edit = %+v, want the new formulas", got)
	}

	// A table creature with no formula keeps no snapshot; an SRD creature neither.
	static := give(tb.pc, tableWolfKey)
	if raw := tb.statBlockOf(t, static.GetId()); raw != nil {
		t.Errorf("a creature with no formula has a snapshot: %s", raw)
	}
	if raw := tb.statBlockOf(t, tb.wolf(t, "Comum").GetId()); raw != nil {
		t.Errorf("an SRD creature has a snapshot: %s", raw)
	}
	if raw := tb.statBlockOf(t, wizardWolf.GetId()); !strings.Contains(string(raw), `"HitPoints": 15`) {
		t.Errorf("the snapshot = %s, want the resolved numbers", raw)
	}
}

func TestMR578_TheFormulaFieldsOfACreatureHaveTheirPaths(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Samuel"), h.newUser("Dona")
	campaign := h.newCampaign(master, "Mirathel", player)
	bad := silverWolf()
	bad.Attacks = append(bad.Attacks, bad.Attacks[0], &rulesv1.TableCreatureAttack{
		NamePt: "Mordida", Melee: true, DiceCount: 1, DiceSides: 4, DamageType: "damage-type:piercing", ToHit: "prof() +",
	})
	bad.Attacks[1] = &rulesv1.TableCreatureAttack{NamePt: "Pancada", Melee: true, DiceCount: 1, DiceSides: 4, DamageType: "damage-type:bludgeoning"}
	bad.SaveDcFormula = "wisdom()"
	_, err := master.table.CreateTableEntry(t.Context(), connect.NewRequest(createReq(campaign, bad)))
	fields := map[string]string{}
	for _, v := range violationsOfErr(t, err) {
		fields[v.GetField()] = v.GetReason()
	}
	if fields["table_creature.attacks[2].to_hit"] != rules.ReasonFormula || fields["table_creature.save_dc_formula"] != rules.ReasonFormula || len(fields) != 2 {
		t.Errorf("violations = %v, want a formula at table_creature.attacks[2].to_hit and at table_creature.save_dc_formula", fields)
	}
}

func TestMR578_AFormulaCreatureTravelsInAPackWithEveryField(t *testing.T) {
	t.Parallel()
	pt := newPackTable(t)
	full := silverWolf()
	full.DamageBonusFormula, full.AttackBonusFormula, full.SaveDcFormula = "1", "prof()", "8 + prof()"
	full.HitPoints, full.ArmorClass, full.DamageBonus = 30, 14, 2
	full.Attacks = append(full.Attacks, &rulesv1.TableCreatureAttack{
		NamePt: "Espinho", Melee: false, RangeFt: 30, LongRangeFt: 90, DiceCount: 2, DiceSides: 8, DamageType: "damage-type:piercing",
		ToHit: "slot() + prof()", DamageBonus: "slot()",
	})
	made := pt.master.addEntry(t, pt.from, full)
	pack := pt.export(t, pt.from)
	file, err := storeJSON.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	loaded := &rulesv1.TableContentPack{}
	if err := loadJSON.Unmarshal(file, loaded); err != nil {
		t.Fatal(err)
	}
	pt.apply(t, pt.to, loaded, "formula-1")
	in := pt.entries(t, pt.to)[silverWolfKey]
	if in == nil || !proto.Equal(bodyMessage(in), bodyMessage(made)) {
		t.Fatalf("the imported creature = %v, want the exported one", in)
	}
	got := bodyMessage(in).(*rulesv1.TableCreature)
	if got.GetSaveDcFormula() != "8 + prof()" || len(got.GetAttacks()) != 2 || got.GetAttacks()[1].GetLongRangeFt() != 90 || got.GetAttacks()[1].GetToHit() != "slot() + prof()" {
		t.Errorf("the imported creature lost a field: %v", got)
	}
}
