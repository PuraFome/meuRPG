package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// Reliable Talent (SRD 5.1, Rogue level 11) counts a d20 of 9 or lower as 10 on an ability check that adds the proficiency
// bonus. It is the server's rule for every check it rolls: the group check, the roll of either side of a contest and the
// Stealth check of a Hide count it as the scene checks, the search and the hints do.

// newReliableSneaks is the party of newSneaks with a rogue of level 11 for Toren: proficient and expert in Acrobatics,
// Investigation, Perception and Stealth, and not in Athletics.
func newReliableSneaks(t *testing.T) *armed {
	t.Helper()
	return newArmedWith(t, func(a *armed) {
		a.toren = a.caio.rogue(t, a.campaignID, "Toren", 11)
		a.pens = a.ana.caster(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 3,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, nil, []string{fireBolt},
			[]string{magicMissileSpell}, []string{magicMissileSpell})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, scores16(), []string{rapier}, nil)
	})
}

// wantCounted fails unless the roll shows the d20 that came up and counts it as counted (0: as it came up, no mention of the
// feature), with the total using the number counted.
func wantCounted(t *testing.T, what string, roll *playv1.CheckRoll, face, counted int32) {
	t.Helper()
	if len(roll.GetFaces()) == 0 || roll.GetFaces()[0] != face {
		t.Errorf("%s: faces = %v, want [%d]", what, roll.GetFaces(), face)
		return
	}
	if counted == 0 {
		if roll.TreatedAs != nil || roll.GetTreatedAsSource() != "" || roll.GetTotal() != face+roll.GetModifier() {
			t.Errorf("%s: roll = %v, want the d20 as it came up and no mention of the feature", what, roll)
		}
		return
	}
	if roll.TreatedAs == nil || roll.GetTreatedAs() != counted || roll.GetTreatedAsSource() != "feature:reliable-talent" || roll.GetTotal() != counted+roll.GetModifier() {
		t.Errorf("%s: roll = %v, want the %d counted as %d by Reliable Talent", what, roll, face, counted)
	}
}

func TestReliableTalentOnAGroupCheck(t *testing.T) {
	t.Parallel()
	a := newReliableSneaks(t)
	asked := a.mustRequestGroup(t, nil) // Stealth, DC 12
	toren := memberOf(a.mustRollGroup(t, a.caio, asked.GetId(), 4), a.toren.GetId()).GetRoll()
	wantCounted(t, "Toren's 4 on Stealth", toren, 4, 10)
	// A character without the feature keeps the d20 as it came up.
	pens := memberOf(a.mustRollGroup(t, a.ana, asked.GetId(), 4), a.pens.GetId()).GetRoll()
	wantCounted(t, "Pensantus's 4 on Stealth", pens, 4, 0)
	a.mustCloseGroup(t, asked.GetId())
	// A skill the rogue is not proficient in gets nothing.
	other := a.mustRequestGroup(t, func(r *playv1.RequestGroupCheckRequest) { r.SkillKey = "skill:athletics" })
	athletics := memberOf(a.mustRollGroup(t, a.caio, other.GetId(), 4), a.toren.GetId()).GetRoll()
	wantCounted(t, "Toren's 4 on Athletics", athletics, 4, 0)
}

func TestReliableTalentOnHide(t *testing.T) {
	t.Parallel()
	a := newReliableSneaks(t)
	c := a.arena(t, arenaPlan{})
	attempt := c.mustHide(t, a.caio, "Toren", cunningHide, 4)
	wantCounted(t, "Toren's 4 on a Hide", attempt.GetRoll(), 4, 10)
}

func TestReliableTalentOnAContest(t *testing.T) {
	t.Parallel()
	a := newReliableSneaks(t)
	c := a.arena(t, arenaPlan{})
	c.advance(t, c.hob)
	c.h.roller.queue(10)
	started := c.mustStart(t, a.master, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, c.hob), c.id(t, "Toren"), playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE
	})
	// Toren answers with Acrobatics, a skill he is proficient in: the 4 counts as 10.
	answered := c.mustRespond(t, a.caio, started.GetContest().GetId(), playv1.ContestSkill_CONTEST_SKILL_ACROBATICS, 4)
	wantCounted(t, "Toren's 4 on Acrobatics", answered.GetContest().GetDefenderRoll(), 4, 10)
}

func TestNoReliableTalentOnAContestWithoutProficiency(t *testing.T) {
	t.Parallel()
	a := newReliableSneaks(t)
	c := a.arena(t, arenaPlan{})
	c.advance(t, c.hob)
	c.h.roller.queue(10)
	started := c.mustStart(t, a.master, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, c.hob), c.id(t, "Toren"), playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE
	})
	answered := c.mustRespond(t, a.caio, started.GetContest().GetId(), playv1.ContestSkill_CONTEST_SKILL_ATHLETICS, 4)
	wantCounted(t, "Toren's 4 on Athletics", answered.GetContest().GetDefenderRoll(), 4, 0)
}
