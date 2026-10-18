package leaktest

import (
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The free dice roller (MR-052, RN-10): a player's roll goes to the whole table, and the master's
// "Só eu" roll reaches no player at all: not a read, not the stream, not the summary, not a count.
// A roll has no text the master could write a marker in, so the needle is the roll's id (the id of
// its session event): a player's answer that holds it has the secret roll. The roll of Ana's is the
// control: every player finds it in their own reads, and the master finds both.

// freeRoll rolls 2d6+3 as p, with the master's "Só eu" when secret, and returns the roll.
func (w *world) freeRoll(p *person, secret bool) *playv1.FreeRoll {
	req := &playv1.RollFreeDiceRequest{CampaignId: w.campaign, Sides: 6, Count: 2, Modifier: 3, Secret: secret, IdempotencyKey: newKey()}
	return must(p.play.RollFreeDice(w.t.Context(), rq(req))).GetRoll()
}

// registerFreeRolls says who may read each roll's id: the master's secret roll nobody else, Ana's
// roll the whole table.
func (w *world) registerFreeRolls(kind string, secret, table *playv1.FreeRoll) {
	w.secrets.id(kind+"-secret", secret.GetId())
	w.secrets.add(&canary{needle: table.GetId(), kind: kind + "-table-id", public: true})
}

// buildFreeRolls makes the rolls the reads are asked against: one of the master's "Só eu" and one
// of Ana's for the table.
func (w *world) buildFreeRolls() {
	w.registerFreeRolls("free-roll", w.freeRoll(w.master, true), w.freeRoll(w.ana, false))
}

// freeRollsOnTheStream makes the same two rolls while the streams are open: Ana's reaches the master
// and both players, and the master's reaches nobody.
func (w *world) freeRollsOnTheStream() {
	w.registerFreeRolls("free-roll-live", w.freeRoll(w.master, true), w.freeRoll(w.ana, false))
}
