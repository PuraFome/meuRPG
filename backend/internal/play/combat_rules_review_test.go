package play

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The rules fixes of the contests (SRD 5.1, "Grappling", "Shoving a Creature", "Barbarian: Rage",
// "Monk: Flurry of Blows", "Hide", "Help", "Group Checks", "Armor"). They need the database
// (MEURPG_TEST_DATABASE_URL).

// newMonkArena is the arena with a monk 3 as Toren (Strength 10, Dexterity 16).
func newMonkArena(t *testing.T) (*armed, *cx) {
	t.Helper()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.heroWith(t, a.campaignID, "Toren", classLevel("class:monk", 3, ""), abilities(10, 16, 14, 14, 8), nil, nil, nil)
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1, abilities(10, 14, 12, 16, 8), nil, []string{fireBolt})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, abilities(10, 16, 14, 10, 8), []string{rapier}, nil)
	})
	c := a.arena(t, arenaPlan{})
	return a, c
}

// newMailArena is the arena with a fighter in chain mail (armor that gives disadvantage on
// Stealth) as Toren.
func newMailArena(t *testing.T) (*armed, *cx) {
	t.Helper()
	a := newArmedWith(t, func(a *armed) {
		sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: abilities(15, 14, 14, 10, 8), RaceKey: "race:human", Classes: []*charactersv1.ClassLevel{{ClassKey: "class:fighter", Level: 2}},
			WeaponKeys: []string{rapier}, ArmorKey: "equipment:chain-mail",
		}}}
		res, err := a.caio.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
			CampaignId: a.campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Toren", Sheet: sheet,
		}))
		if err != nil {
			t.Fatalf("CreateCharacter(Toren) error = %v", err)
		}
		a.toren = res.Msg.GetCharacter()
		a.pens = a.ana.caster(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 3,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, nil, []string{fireBolt},
			[]string{magicMissileSpell}, []string{magicMissileSpell})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, scores16(), []string{rapier}, nil)
	})
	c := a.arena(t, arenaPlan{})
	return a, c
}

// TestAGrappleIsTheAttackActionSoFlurryOfBlowsFollowsButMartialArtsDoesNot: the grapple replaces
// an attack of the Attack action (SRD 5.1, "Grappling"), so the monk may spend 1 ki on Flurry of
// Blows ("immediately after you take the Attack action"); the Martial Arts bonus strike needs the
// Attack action taken with an unarmed strike or a monk weapon, and a grapple is neither.
func TestAGrappleIsTheAttackActionSoFlurryOfBlowsFollowsButMartialArtsDoesNot(t *testing.T) {
	t.Parallel()
	a, c := newMonkArena(t)
	res, err := c.grapple(t, a.caio, "Toren", c.hob, 12)
	if err != nil {
		t.Fatalf("StartContest() error = %v", err)
	}
	c.mustRespond(t, a.master, res.GetContest().GetId(), playv1.ContestSkill_CONTEST_SKILL_ATHLETICS, 3)
	e := c.refresh(t)
	opts := a.mustOptions(t, a.caio, e, "Toren")
	if o := actionOption(opts, flurry); !o.GetEnabled() {
		t.Errorf("Flurry of Blows after the grapple = enabled %v, reason %v, want enabled", o.GetEnabled(), o.GetReason().GetCode())
	}
	if ma := attackOption(opts, unarmed); ma.GetEnabled() {
		t.Errorf("the unarmed strike after a grapple alone = %v, want no Martial Arts strike", ma)
	}
	if _, err := a.action(t, a.caio, e, "Toren", flurry); err != nil {
		t.Fatalf("Flurry of Blows after the grapple error = %v", err)
	}
}

// TestAGrappleOfAHostileCreatureKeepsTheRageGoing (SRD 5.1, Rage: it ends early if the turn ends
// without having attacked a hostile creature or taken damage).
func TestAGrappleOfAHostileCreatureKeepsTheRageGoing(t *testing.T) {
	t.Parallel()
	a, e := rollsTable(t)
	e, err := a.action(t, a.bia, e, "Brisa", "feature:rage")
	if err != nil {
		t.Fatalf("TakeAction(Rage) error = %v", err)
	}
	if byLabel(t, e, "Brisa").GetAttackedHostileSinceLastTurn() {
		t.Fatal("Brisa attacked a hostile creature before doing anything")
	}
	c := &cx{armed: a, e: e}
	a.h.roller.queue(10, 10) // Rage gives advantage on the Strength check
	if _, err := c.startContest(t, a.bia, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Brisa"), c.id(t, "Goblin"), playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE
	}); err != nil {
		t.Fatalf("StartContest() error = %v", err)
	}
	if !byLabel(t, c.refresh(t), "Brisa").GetAttackedHostileSinceLastTurn() {
		t.Error("after grappling the Goblin Brisa has not attacked a hostile creature, want it noted so the rage holds")
	}
}

// TestHideRollsCountExhaustionAndTheRageOnlyOnStrength: exhaustion 1 is disadvantage on ability
// checks (SRD 5.1, Conditions), and advantage and disadvantage cancel.
func TestHideRollsCountExhaustionAndAdvantageCancels(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	if _, err := a.setExhaustion(t, c.e, "Toren", 1, 0, false); err != nil {
		t.Fatalf("SetExhaustion() error = %v", err)
	}
	c.h.roller.queue(18, 4)
	tried := c.mustHide(t, a.caio, "Toren", "", 0)
	roll := tried.GetRoll()
	if roll.GetMode() != playv1.RollModeKind_ROLL_MODE_KIND_DISADVANTAGE || len(roll.GetFaces()) != 2 {
		t.Errorf("the Stealth check of an exhausted rogue = %v, want disadvantage over two d20", roll)
	}
}

// TestHideInArmorThatGivesStealthDisadvantageRollsTwoDice (SRD 5.1, "Armor": chain mail).
func TestHideInArmorThatGivesStealthDisadvantageRollsTwoDice(t *testing.T) {
	t.Parallel()
	a, c := newMailArena(t)
	c.h.roller.queue(18, 4)
	tried := c.mustHide(t, a.caio, "Toren", "", 0)
	roll := tried.GetRoll()
	if roll.GetMode() != playv1.RollModeKind_ROLL_MODE_KIND_DISADVANTAGE || len(roll.GetFaces()) != 2 {
		t.Errorf("the Stealth check in chain mail = %v, want disadvantage over two d20", roll)
	}
}

// TestAGroupCheckOfStealthInChainMailHasDisadvantageAndUsesAHelp: the group check rolls carry the
// same sources as any ability check (armor on Stealth, a Help, exhaustion), a physical roll must
// type the pair of dice, and the Help is used up.
func TestAGroupCheckOfStealthInChainMailHasDisadvantageAndUsesAHelp(t *testing.T) {
	t.Parallel()
	a, c := newMailArena(t)
	checkID := a.mustRequestGroup(t, nil).GetId()
	if view := a.groupView(t, a.caio); view.GetYourOption().GetMode() != playv1.RollModeKind_ROLL_MODE_KIND_DISADVANTAGE {
		t.Errorf("Toren's option = %v, want disadvantage (chain mail)", view.GetYourOption())
	}
	if _, err := a.rollGroupWith(t, a.caio, checkID, faces(12)); connect.CodeOf(err) != connect.CodeAborted {
		t.Fatalf("one typed die for a roll with disadvantage: error = %v, want aborted (read the options again)", err)
	}
	// A Help from Brisa for the Stealth check cancels the disadvantage: a normal roll.
	c.advance(t, "Brisa")
	given := helped(t)(c.helpCheck(t, a.bia, "Brisa", "Toren", stealthKey))
	if given == nil {
		t.Fatal("Brisa's Help was not given")
	}
	if _, err := a.rollGroupWith(t, a.caio, checkID, faces(12)); err != nil {
		t.Fatalf("RollGroupCheck() with the Help error = %v", err)
	}
	if hs := c.state(t, a.master).GetHelps(); len(hs) != 0 {
		t.Errorf("the Help after the group check roll = %v, want it used up", hs)
	}
}

// TestAGrappleFromHidingGivesThePositionAway (SRD 5.1, "Unseen Attackers and Targets": a grapple
// or a shove is an attack).
func TestAGrappleFromHidingGivesThePositionAway(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	tried := c.mustHide(t, a.caio, "Toren", cunningHide, 12)
	c.mustResolveHide(t, tried.GetId(), nil)
	if got := c.state(t, a.master).GetHiddenIds(); len(got) != 1 {
		t.Fatalf("hidden = %v, want Toren", got)
	}
	if _, err := c.grapple(t, a.caio, "Toren", c.hob, 12); err != nil {
		t.Fatalf("StartContest() error = %v", err)
	}
	for who, u := range map[string]*user{"the master": a.master, "Toren's player": a.caio} {
		if got := c.state(t, u).GetHiddenIds(); len(got) != 0 {
			t.Errorf("%s reads %v hidden after the grapple, want the hiding ended", who, got)
		}
	}
}

// TestACantripWithOnlyASomaticComponentKeepsTheCasterHidden (SRD 5.1: a verbal component is a
// noise; True Strike has only a somatic one).
func TestACantripWithOnlyASomaticComponentKeepsTheCasterHidden(t *testing.T) {
	t.Parallel()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:rogue", "race:human", 3, sneakScores(), []string{rapier}, nil)
		a.pens = a.ana.caster(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 3,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, nil, []string{"spell:true-strike"},
			nil, nil)
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, scores16(), []string{rapier}, nil)
	})
	c := a.arena(t, arenaPlan{})
	pens := c.id(t, "Pensantus")
	c.advance(t, "Pensantus")
	tried := c.mustHide(t, a.ana, "Pensantus", "", 14)
	c.mustResolveHide(t, tried.GetId(), nil)
	c.advance(t, "Goblin")
	c.advance(t, "Pensantus")
	a.mustCast(t, a.ana, c.refresh(t), "Pensantus", "spell:true-strike", nil, []*playv1.SpellTarget{{CombatantId: c.id(t, c.hob)}}, noCastRoll)
	if got := c.state(t, a.master).GetHiddenIds(); !slices.Equal(got, []string{pens}) {
		t.Errorf("hidden after a cantrip with no verbal component = %v, want Pensantus still hidden", got)
	}
}
