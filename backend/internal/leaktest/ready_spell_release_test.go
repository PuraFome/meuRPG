package leaktest

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The targets of a held spell (#857) are listed in its readier's options, which only the readier's player and the master
// read (RN-10). Ana is refused the options of Toren, and nothing of the held spell is in what she or the TV reads.
func TestTheTargetsOfAHeldSpellAreTheOwnersAndTheMasters(t *testing.T) {
	w := newWorld(t)
	w.onTorenTurnAsWizard()
	toren := w.combatant(w.toren)
	for _, n := range readiedSpellWords {
		w.secrets.add(&canary{needle: n, kind: "readied-spell-targets", readers: names([]*person{w.caio})})
	}
	must(w.caio.ready.ReadyAction(t.Context(), rq(&playv1.ReadyActionRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: toren.GetId(), IdempotencyKey: newKey(),
		TriggerText: w.secrets.marker("readied-spell-targets-trigger", w.caio), Kind: playv1.ReadiedKind_READIED_KIND_SPELL, SpellKey: "spell:hold-person", SlotLevel: 2,
	})))
	options := func(p *person) (*playv1.GetTurnOptionsResponse, error) {
		res, err := p.combat.GetTurnOptions(t.Context(), rq(&playv1.GetTurnOptionsRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: toren.GetId()}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	for _, p := range []*person{w.master, w.caio} {
		res, err := options(p)
		if err != nil {
			t.Fatalf("%s reads Toren's options: %v", p.name, err)
		}
		held := false
		for _, st := range res.GetSpellTargets() {
			held = held || (st.GetSpellKey() == "spell:hold-person" && len(st.GetTargets()) > 0)
		}
		if !held {
			t.Errorf("%s's options list no targets of the held Hold Person: the canary is unreachable", p.name)
		}
	}
	if _, err := options(w.ana); err == nil {
		t.Error("Ana read the options of Toren while he holds a spell")
	}
	w.inspectPlayers(t, w.ana, w.pending)
	w.readScreen(t, "after Toren readied a spell and his options were read", w.tvReader(), nil)
}
