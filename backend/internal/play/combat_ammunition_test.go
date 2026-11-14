package play

import (
	"bytes"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// Ammunition in combat (SRD 5.1, "Weapons": the ammunition property): a ranged attack with a bow, a crossbow, a sling or a
// blowgun takes a piece from the character's stock in the attack's own transaction. A thrown weapon, a melee weapon and the
// improvised melee line of a bow take none. These tests need the database (MEURPG_TEST_DATABASE_URL).

// stockArrows gives a character twenty arrows, as the master does.
func (a *armed) stockArrows(t *testing.T, c *charactersv1.Character) {
	t.Helper()
	_, err := a.master.charges.SetAmmunition(t.Context(), connect.NewRequest(&playv1.SetAmmunitionRequest{CampaignId: a.campaignID, CharacterId: c.GetId(), AmmoKey: arrowAmmo, Quantity: 20}))
	if err != nil {
		t.Fatalf("SetAmmunition(%s) error = %v", c.GetName(), err)
	}
}

// archery is the usual party with Toren a level 5 fighter (Extra Attack: two attacks) who carries a shortbow, a handaxe
// and a battleaxe.
type archery struct {
	*armed
}

func newArchery(t *testing.T) *archery {
	t.Helper()
	return &archery{newArmedWith(t, func(a *armed) {
		scores := func(str, dex, con, intl int32) *rulesv1.AbilityScores {
			return &rulesv1.AbilityScores{Strength: str, Dexterity: dex, Constitution: con, Intelligence: intl, Wisdom: 10, Charisma: 8}
		}
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5, scores(14, 16, 14, 10), []string{shortbowKey, handaxeKey, battleaxe}, nil)
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1, scores(10, 14, 12, 16), nil, []string{fireBolt})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, scores(10, 16, 14, 10), []string{rapier}, nil)
	})}
}

// stock sets what Toren holds of the arrows, as the master.
func (s *archery) stock(t *testing.T, pieces int32) {
	t.Helper()
	_, err := s.master.charges.SetAmmunition(t.Context(), connect.NewRequest(&playv1.SetAmmunitionRequest{CampaignId: s.campaignID, CharacterId: s.toren.GetId(), AmmoKey: arrowAmmo, Quantity: pieces}))
	if err != nil {
		t.Fatalf("SetAmmunition(%d) error = %v", pieces, err)
	}
}

// held is the arrows on Toren's sheet as the owner reads it (0 when the sheet lists none).
func (s *archery) held(t *testing.T) int32 {
	t.Helper()
	res, err := s.caio.characters.GetCharacter(t.Context(), connect.NewRequest(&charactersv1.GetCharacterRequest{CampaignId: s.campaignID, CharacterId: s.toren.GetId()}))
	if err != nil {
		t.Fatalf("GetCharacter() error = %v", err)
	}
	for _, am := range res.Msg.GetCharacter().GetSheet().GetFull().GetAmmunition() {
		if am.GetKey() == arrowAmmo {
			return am.GetQuantity()
		}
	}
	return 0
}

// fight starts the combat with Toren first and the goblin (revealed) at the column; Toren stands at column 3 of row 3.
func (s *archery) fight(t *testing.T, goblinCol int32) *playv1.Encounter {
	t.Helper()
	return s.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: s.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Toren": {3, 3}, "Goblin": {goblinCol, 3}, "Pensantus": {10, 8}, "Brisa": {8, 8}},
	})
}

// shoot is Toren's attack on the goblin with the key, a miss (the d20 shows a 1) so no damage waits.
func (s *archery) shoot(t *testing.T, e *playv1.Encounter, key string) (*playv1.RollAttackResponse, error) {
	t.Helper()
	return s.attack(t, s.caio, e, "Toren", key, "Goblin", d20(1))
}

// ammoLines are the AMMUNITION_SPENT lines of the log as u reads it, the latest first.
func (s *archery) ammoLines(t *testing.T, u *user, e *playv1.Encounter) []*playv1.CombatLogEntry {
	t.Helper()
	var out []*playv1.CombatLogEntry
	for _, r := range s.log(t, u, e).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_AMMUNITION_SPENT {
				out = append(out, en)
			}
		}
	}
	return out
}

func TestARangedAttackWithAmmunitionSpendsOnePieceAndLogsItForTheOwnerAndTheMasterAlone(t *testing.T) {
	t.Parallel()
	s := newArchery(t)
	s.stock(t, 20)
	e := s.fight(t, 8)
	if _, err := s.shoot(t, e, shortbowKey); err != nil {
		t.Fatalf("RollAttack(shortbow) error = %v", err)
	}
	if got := s.held(t); got != 19 {
		t.Errorf("arrows after one shot = %d, want 19", got)
	}
	if n := s.countEvents(t, eventAmmunitionSpent); n != 1 {
		t.Errorf("the session has %d ammunition_spent events, want 1", n)
	}
	var name string
	for _, u := range []*user{s.master, s.caio} {
		lines := s.ammoLines(t, u, e)
		if len(lines) != 1 {
			t.Fatalf("a reader with the line sees %d ammunition lines, want 1", len(lines))
		}
		use := lines[0].GetItemUse()
		if use.GetAmount() != 1 || use.GetLeft() != 19 || use.GetAmmunitionNamePt() == "" || use.GetItemNamePt() == "" || lines[0].GetActorLabel() != "Toren" {
			t.Errorf("the ammunition line = %v, want 1 spent, 19 left, with the names, by Toren", lines[0])
		}
		name = use.GetAmmunitionNamePt()
	}
	for who, u := range map[string]*user{"Ana": s.ana, "Bia": s.bia} {
		if lines := s.ammoLines(t, u, e); len(lines) != 0 {
			t.Errorf("%s reads %d ammunition lines, want none", who, len(lines))
		}
		raw, err := proto.Marshal(s.log(t, u, e))
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		if bytes.Contains(raw, []byte(name)) {
			t.Errorf("%s's whole log carries the ammunition's name %q", who, name)
		}
	}
}

func TestAnAttackWithNoAmmunitionLeftIsRefusedAndWritesNothing(t *testing.T) {
	t.Parallel()
	for name, pieces := range map[string]int32{"none ever held": -1, "none left": 0} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := newArchery(t)
			if pieces >= 0 {
				s.stock(t, pieces)
			}
			e := s.fight(t, 8)
			_, err := s.shoot(t, e, shortbowKey)
			wantBlockedBy(t, "a shot with no ammunition", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_AMMUNITION_EMPTY)
			if n := s.countEvents(t, eventAttackRolled); n != 0 {
				t.Errorf("the session has %d attack events, want none", n)
			}
			if n := s.countEvents(t, eventAmmunitionSpent); n != 0 {
				t.Errorf("the session has %d ammunition events, want none", n)
			}
			if used := s.mustOptions(t, s.caio, e, "Toren").GetOptions().GetEconomy().GetAction().GetUsed(); used {
				t.Error("the refused shot spent the action")
			}
			if got := s.held(t); got != 0 {
				t.Errorf("arrows = %d, want 0", got)
			}
		})
	}
}

func TestExtraAttackSpendsOnePiecePerAttackRoll(t *testing.T) {
	t.Parallel()
	s := newArchery(t)
	s.stock(t, 5)
	e := s.fight(t, 8)
	for i := range 2 {
		if _, err := s.shoot(t, e, shortbowKey); err != nil {
			t.Fatalf("shot %d error = %v", i+1, err)
		}
	}
	if got := s.held(t); got != 3 {
		t.Errorf("arrows after the two attacks of the action = %d, want 3", got)
	}
	if n := len(s.ammoLines(t, s.master, e)); n != 2 {
		t.Errorf("the master reads %d ammunition lines, want 2", n)
	}
}

func TestAThrownWeaponAndAMeleeWeaponSpendNoAmmunition(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		goblinCol int32
		keys      []string
	}{
		"thrown handaxe at 15 ft":                   {goblinCol: 6, keys: []string{handaxeKey}},
		"battleaxe and the improvised line at 5 ft": {goblinCol: 4, keys: []string{battleaxe, shortbowKey + ":melee"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := newArchery(t)
			s.stock(t, 7)
			e := s.fight(t, tc.goblinCol)
			for _, key := range tc.keys {
				if _, err := s.shoot(t, e, key); err != nil {
					t.Fatalf("RollAttack(%s) error = %v", key, err)
				}
			}
			if got := s.held(t); got != 7 {
				t.Errorf("arrows = %d, want 7", got)
			}
			if n := s.countEvents(t, eventAmmunitionSpent); n != 0 {
				t.Errorf("the session has %d ammunition events, want none", n)
			}
		})
	}
}

func TestTheTurnOptionsCarryTheAmmunitionAndTheImprovisedMeleeLine(t *testing.T) {
	t.Parallel()
	s := newArchery(t)
	s.stock(t, 12)
	e := s.fight(t, 8)
	byKey := map[string]*rulesv1.AttackOption{}
	for _, o := range s.mustOptions(t, s.caio, e, "Toren").GetOptions().GetAttacks() {
		byKey[o.GetAttack().GetKey()] = o
	}
	bow := byKey[shortbowKey].GetAttack()
	if bow == nil || bow.AmmunitionLeft == nil || bow.GetAmmunitionLeft() != 12 || bow.GetImprovised() {
		t.Errorf("the shortbow line = %v, want 12 arrows left and not improvised", bow)
	}
	club := byKey[shortbowKey+":melee"].GetAttack()
	if club == nil {
		t.Fatalf("the attacks %v have no improvised line for the shortbow", keysOf(byKey))
	}
	// 1d4 plus Strength (+2), no proficiency: +2 to hit.
	if !club.GetImprovised() || !club.GetMelee() || club.GetDamage() != "1d4+2" || club.GetAttackBonus() != 2 || club.AmmunitionLeft != nil || club.GetRangeFt() > 5 {
		t.Errorf("the improvised line = %v, want a melee 1d4+2 at +2 with no ammunition", club)
	}
	if club.GetNamePt() == "" || club.GetNamePt() == bow.GetNamePt() {
		t.Errorf("the improvised line's name = %q, want the weapon's with \", corpo a corpo\"", club.GetNamePt())
	}
	for _, key := range []string{handaxeKey, battleaxe} {
		if a := byKey[key].GetAttack(); a == nil || a.AmmunitionLeft != nil || a.GetImprovised() {
			t.Errorf("the %s line = %v, want no ammunition and not improvised", key, a)
		}
		if byKey[key+":melee"] != nil {
			t.Errorf("%s has an improvised line", key)
		}
	}
	// The master reading the turn sees the count too.
	master := s.mustOptions(t, s.master, e, "Toren").GetOptions().GetAttacks()
	for _, o := range master {
		if o.GetAttack().GetKey() == shortbowKey && o.GetAttack().GetAmmunitionLeft() != 12 {
			t.Errorf("the master reads %d arrows, want 12", o.GetAttack().GetAmmunitionLeft())
		}
	}
	// The count follows the stock.
	if _, err := s.shoot(t, e, shortbowKey); err != nil {
		t.Fatalf("RollAttack() error = %v", err)
	}
	for _, o := range s.mustOptions(t, s.caio, e, "Toren").GetOptions().GetAttacks() {
		if o.GetAttack().GetKey() == shortbowKey && o.GetAttack().GetAmmunitionLeft() != 11 {
			t.Errorf("arrows left after a shot = %d, want 11", o.GetAttack().GetAmmunitionLeft())
		}
	}
}

func keysOf(m map[string]*rulesv1.AttackOption) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

// Two attacks of the same action at the last piece (a second request that arrives while the first is in its transaction):
// one wins and the other is refused as if it came after.
func TestTwoAttacksAtTheLastPieceGiveOneShot(t *testing.T) {
	t.Parallel()
	s := newArchery(t)
	s.stock(t, 1)
	e := s.fight(t, 8)
	attacker, target := s.id(t, "Toren"), s.id(t, "Goblin")
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		req := &playv1.RollAttackRequest{
			CampaignId: s.campaignID, EncounterId: e.GetId(), AttackerId: attacker, AttackKey: shortbowKey, TargetId: target, IdempotencyKey: newKey(),
			Roll: &playv1.RollAttackRequest_D20Face{D20Face: 1},
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = s.caio.combat.RollAttack(t.Context(), connect.NewRequest(req))
		}()
	}
	wg.Wait()
	won, refused := 0, 0
	for _, err := range errs {
		if err == nil {
			won++
		} else if b := encounterBlocked(t, err); b.GetReason() == playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_AMMUNITION_EMPTY {
			refused++
		}
	}
	if won != 1 || refused != 1 {
		t.Fatalf("the two shots at the last piece = %v, %v; want one success and one AMMUNITION_EMPTY", errs[0], errs[1])
	}
	if n := s.countEvents(t, eventAmmunitionSpent); n != 1 {
		t.Errorf("the session has %d ammunition_spent events, want 1", n)
	}
	if got := s.held(t); got != 0 {
		t.Errorf("arrows = %d, want 0", got)
	}
}

// The ammunition line never closes the undo chain: taking the second shot back leaves the first one to take back.
func TestUndoingAShotLeavesTheShotBeforeItToUndo(t *testing.T) {
	t.Parallel()
	s := newArchery(t)
	s.stock(t, 10)
	e := s.fight(t, 8)
	for i := range 2 {
		if _, err := s.shoot(t, e, shortbowKey); err != nil {
			t.Fatalf("shot %d error = %v", i+1, err)
		}
	}
	s.undoLast(t, e)
	if id := s.log(t, s.master, s.get(t, s.master)).GetUndoableEventId(); id == "" {
		t.Error("after the undo of the second shot the first one is not offered to undo")
	}
}
