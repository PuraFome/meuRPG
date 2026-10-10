package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// useExtra attacks with the extra action of an effect.
func useExtra(r *playv1.RollAttackRequest) { r.UseExtraAction = true }

// Haste (SRD 5.1): +2 armor class, double speed, advantage on Dexterity saves and one extra
// action each turn (one weapon attack, Dash, Disengage, Hide or Use an Object); when it ends the
// creature can neither move nor act for a turn.
func TestHasteGivesAnExtraActionAndLethargyFollowsIt(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	before := byLabel(t, e, "Toren")
	a.mustAddEffect(t, e, "spell:haste", []string{"Toren"}, a.rounds(t, 1, "Toren"))
	hasted := byLabel(t, a.get(t, a.master), "Toren")
	if hasted.GetArmorClassBonus() != before.GetArmorClassBonus()+2 || hasted.GetSpeedDft() != before.GetSpeedDft()*2 {
		t.Errorf("Toren hasted = AC bonus %d speed %d, want %d and %d", hasted.GetArmorClassBonus(), hasted.GetSpeedDft(), before.GetArmorClassBonus()+2, before.GetSpeedDft()*2)
	}
	opts := a.mustOptions(t, a.caio, e, "Toren")
	if x := opts.GetExtraAction(); x == nil || !x.GetAvailable() {
		t.Fatalf("the extra action = %v, want one available", x)
	}
	// The action of the turn is untouched by the extra attack.
	a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", func(r *playv1.RollAttackRequest) { d20(15)(r); useExtra(r) })
	if _, err := a.attack(t, a.caio, e, "Toren", battleaxe, "Goblin", func(r *playv1.RollAttackRequest) { d20(15)(r); useExtra(r) }); err == nil {
		t.Error("a second extra action in the same turn worked")
	} else {
		wantBlockedBy(t, "the second extra action", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_EXTRA_ACTION_UNAVAILABLE)
	}
	if got := a.mustOptions(t, a.caio, e, "Toren").GetExtraAction(); got.GetAvailable() {
		t.Errorf("the extra action still reads available after it was spent: %v", got)
	}
	// Not a spell: a cast with the extra action is refused (the request has no such field), and the
	// standard action Cast a Spell is not on the list.
	if _, err := a.master.combat.TakeAction(t.Context(), connect.NewRequest(&playv1.TakeActionRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Toren"), ActionKey: "standard:dodge", IdempotencyKey: newKey(), UseExtraAction: true,
	})); err == nil {
		t.Error("Dodge with the extra action worked")
	}
	// The effect ends at the start of Toren's turn in round 2; the turn is the lethargy's.
	e = a.turnOf(t, "Toren")
	if f := cardOf(t, byLabel(t, e, "Toren"), "spell:haste"); f != nil {
		t.Fatalf("Haste is still on after its time: %v", f)
	}
	lethargic := a.mustOptions(t, a.caio, e, "Toren")
	if len(lethargic.GetEffectNotes()) == 0 {
		t.Error("the lethargy has no note on the turn")
	}
	if _, err := a.attack(t, a.caio, e, "Toren", battleaxe, "Goblin", d20(15)); err == nil {
		t.Error("a lethargic Toren attacked")
	}
	if got := byLabel(t, e, "Toren").GetMovementLeftDft(); got != 0 {
		t.Errorf("a lethargic Toren has %d movement left, want 0", got)
	}
}

// Bless adds a d4 to an attack roll (SRD 5.1); with physical dice the player types it.
func TestBlessAddsADieToAnAttackAndPhysicalDiceTypeIt(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	a.mustAddEffect(t, e, "spell:bless", []string{"Toren"}, a.rounds(t, 10, "Toren"))
	// A physical d20 without the d4 is refused, with it the roll takes the die.
	if _, err := a.attack(t, a.caio, e, "Toren", battleaxe, "Goblin", d20(10)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("an attack without the die: err = %v, want invalid_argument", err)
	}
	res := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", func(r *playv1.RollAttackRequest) { d20(10)(r); r.ExtraDieFaces = []int32{3} })
	roll := res.GetRoll().GetD20()
	if got := roll.GetExtraDice(); len(got) != 1 || got[0].GetFace() != 3 || got[0].GetFaces() != 4 {
		t.Errorf("the roll's extra dice = %v, want a d4 of 3", got)
	}
	if want := roll.GetModifier() + 10 + 3; roll.GetTotal() != want {
		t.Errorf("the total = %d, want d20 10 + modifier %d + 3 = %d", roll.GetTotal(), roll.GetModifier(), want)
	}
}

// Bane takes a d4 off.
func TestBaneTakesADieOffAnAttack(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	a.mustAddEffect(t, e, "spell:bane", []string{"Toren"}, a.rounds(t, 10, "Toren"))
	res := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", func(r *playv1.RollAttackRequest) { d20(10)(r); r.ExtraDieFaces = []int32{4} })
	roll := res.GetRoll().GetD20()
	if got := roll.GetExtraDice(); len(got) != 1 || got[0].GetSign() != -1 {
		t.Fatalf("the roll's extra dice = %v, want a subtracted d4", got)
	}
	if want := roll.GetModifier() + 10 - 4; roll.GetTotal() != want {
		t.Errorf("the total = %d, want %d", roll.GetTotal(), want)
	}
}

// Haste (SRD 5.1): "when the spell ends, the target can't move or take actions until after its
// next turn". The spell also ends when its caster, who concentrates, dies or leaves the combat, and the
// lethargy follows then too.
func TestHastesLethargyFollowsWhenTheCasterDies(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	a.mustAddEffect(t, e, "spell:haste", []string{"Toren"}, func(r *playv1.AddLastingEffectRequest) { r.CasterId = a.id(t, "Pensantus") })
	if f := cardOf(t, byLabel(t, a.get(t, a.master), "Toren"), "spell:haste"); f == nil {
		t.Fatal("Toren has no Haste")
	}
	// The caster dies (exhaustion 6, then the master's confirmation).
	if _, err := a.setExhaustion(t, e, "Pensantus", 6, 0, true); err != nil {
		t.Fatalf("SetExhaustion(6) error = %v", err)
	}
	if _, err := a.master.combat.ConfirmDeath(t.Context(), connect.NewRequest(&playv1.ConfirmDeathRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Pensantus"), IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("ConfirmDeath() error = %v", err)
	}
	toren := byLabel(t, a.get(t, a.master), "Toren")
	if cardOf(t, toren, "spell:haste") != nil {
		t.Error("Haste is still on Toren after its caster left")
	}
	if cardOf(t, toren, "effect:lethargy") == nil {
		t.Errorf("Toren's effects = %v, want the lethargy", toren.GetEffects())
	}
}
