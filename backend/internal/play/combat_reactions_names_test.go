package play

import (
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

const (
	daggerKey   = "equipment:dagger"
	shortbowKey = "equipment:shortbow"
)

// TestShieldPromptNamesTheAttackerOnlyWhenSeen: the prompt says who attacked and
// with what ("Capitão Goblin · Cimitarra") and names the spell ("Escudo Arcano"),
// to the master and to the target's player, unless the attacker is hidden from
// the player (RN-20).
func TestShieldPromptNamesTheAttackerOnlyWhenSeen(t *testing.T) {
	t.Parallel()
	t.Run("a visible attacker", func(t *testing.T) {
		t.Parallel()
		a := newCasters(t)
		e := a.castersFightNPCFirst(t)
		a.mustAttack(t, a.master, e, "Capitão Goblin", sword, "Pensantus", d20(9))
		for who, u := range map[string]*user{"the target's player": a.ana, "the master": a.master} {
			ps := a.get(t, u).GetReactionPrompts()
			if len(ps) != 1 || ps[0].GetAttackerLabel() != "Capitão Goblin" || ps[0].GetAttackNamePt() == "" || ps[0].GetSpellNamePt() != "Escudo Arcano" {
				t.Errorf("%s's prompt = %v, want the attacker, its attack and \"Escudo Arcano\"", who, ps)
			}
		}
	})
	t.Run("a hidden attacker", func(t *testing.T) {
		t.Parallel()
		a := newCasters(t)
		e := a.castersFightNPCFirst(t)
		if _, err := a.master.combat.SetCombatantHidden(t.Context(), connect.NewRequest(&playv1.SetCombatantHiddenRequest{
			CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Capitão Goblin"), IdempotencyKey: newKey(), Hidden: true,
		})); err != nil {
			t.Fatalf("SetCombatantHidden() error = %v", err)
		}
		a.mustAttack(t, a.master, e, "Capitão Goblin", sword, "Pensantus", d20(9))
		ps := a.get(t, a.ana).GetReactionPrompts()
		if len(ps) != 1 || ps[0].GetAttackerLabel() != "" || ps[0].GetAttackNamePt() != "" || ps[0].GetSpellNamePt() != "Escudo Arcano" {
			t.Errorf("the player's prompt = %v, want no attacker and no attack, but the spell's name", ps)
		}
		ps = a.get(t, a.master).GetReactionPrompts()
		if len(ps) != 1 || ps[0].GetAttackerLabel() != "Capitão Goblin" || ps[0].GetAttackNamePt() == "" {
			t.Errorf("the master's prompt = %v, want the attacker and its attack", ps)
		}
	})
}

// TestConcentrationSpellComesWithItsName: everyone who sees the combatant reads
// the Portuguese name of the spell it concentrates on, with no second read.
func TestConcentrationSpellComesWithItsName(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.castersFight(t, 1)
	a.mustCast(t, a.ana, e, "Pensantus", webSpell, slotOfLevel(2), nil, noCastRoll)
	for who, u := range map[string]*user{"another player": a.caio, "the master": a.master} {
		c := byLabel(t, a.get(t, u), "Pensantus")
		if c.GetConcentrationSpell() != webSpell || c.GetConcentrationSpellNamePt() != "Teia" {
			t.Errorf("%s sees %q / %q, want the key and \"Teia\"", who, c.GetConcentrationSpell(), c.GetConcentrationSpellNamePt())
		}
	}
}

// TestOpportunityAttackWithAThrownMeleeWeapon: a dagger is a melee weapon
// (SRD) and can make an opportunity attack, with the melee reach; a bow cannot.
func TestOpportunityAttackWithAThrownMeleeWeapon(t *testing.T) {
	t.Parallel()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe, daggerKey, shortbowKey}, nil)
		a.pens = a.ana.caster(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 3,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, nil, []string{fireBolt},
			[]string{shieldSpell}, []string{shieldSpell})
		a.bri = a.bia.caster(t, a.campaignID, "Brisa", "class:cleric", "race:human", 3,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 16, Charisma: 8}, []string{maceKey}, nil, nil, []string{cureWounds})
	})
	e := a.castersFight(t, 1) // Pensantus on turn; Toren is next to the goblin
	opts, err := a.master.combat.GetTurnOptions(t.Context(), connect.NewRequest(&playv1.GetTurnOptionsRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Toren")}))
	if err != nil {
		t.Fatalf("GetTurnOptions() error = %v", err)
	}
	melee := map[string]bool{}
	for _, o := range opts.Msg.GetOptions().GetAttacks() {
		melee[o.GetAttack().GetKey()] = o.GetAttack().GetMelee()
	}
	if !melee[battleaxe] || !melee[daggerKey] || melee[shortbowKey] {
		t.Errorf("melee flags = %v, want the axe and the dagger melee, the bow not", melee)
	}
	opp, err := a.attackAs(t, a.caio, e, "Toren", daggerKey, "Goblin", d20(15), true)
	if err != nil {
		t.Fatalf("an opportunity attack with a dagger: error = %v", err)
	}
	if !byLabel(t, opp.GetEncounter(), "Toren").GetReactionUsed() {
		t.Errorf("the dagger's opportunity attack did not spend the reaction")
	}
	a.h.roller.queue(2)
	a.mustDamage(t, a.caio, e, opp.GetPendingDamage().GetId(), inAppDamage)
	a.mustEndTurn(t, a.ana, e) // Toren's turn: the reaction is back
	a.mustEndTurn(t, a.caio, e)
	_, err = a.attackAs(t, a.caio, e, "Toren", shortbowKey, "Goblin", d20(15), true)
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("an opportunity attack with a bow = %v, want invalid_argument", err)
	}
}

// TestAdjustingVitalsInACombatRaisesItsRevision: the master's correction of a
// player's hit points changes what the combat shows ("Caído"), so the combat's
// revision goes up in the same transaction and every stream hears of it.
func TestAdjustingVitalsInACombatRaisesItsRevision(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.castersFight(t, 1)
	before := a.get(t, a.caio).GetRevision()
	if _, err := a.master.play.AdjustCharacterVitals(t.Context(), connect.NewRequest(&playv1.AdjustCharacterVitalsRequest{
		CampaignId: a.campaignID, CharacterId: a.toren.GetId(), IdempotencyKey: newKey(), HitPointsCurrent: proto.Int32(0),
	})); err != nil {
		t.Fatalf("AdjustCharacterVitals() error = %v", err)
	}
	after := a.get(t, a.caio)
	if after.GetRevision() <= before {
		t.Errorf("revision after = %d, before = %d; want it raised", after.GetRevision(), before)
	}
	if got := byLabel(t, after, "Toren").GetState(); got != playv1.CombatantState_COMBATANT_STATE_DOWN {
		t.Errorf("Toren's state = %v, want DOWN", got)
	}
	_ = e
}
