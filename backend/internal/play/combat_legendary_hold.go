package play

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
	"github.com/PuraFome/meuRPG/backend/internal/rules/reaction"
)

// Legendary Resistance on the saves a player's action makes (SRD 5.1, "Legendary Resistance").
// A cast with a saving throw that targets a creature with the trait and a use left waits in a window of the master's alone (reaction.LegendaryResistance).
// It waits whether that creature's save passed or failed, so the wait tells a player nothing about the roll.
// The window is held with the request as any held action is (combat_reaction_hold.go).
// The hold keeps the saves as they were rolled (holdData.Saves), so the replay lands the same d20 and the same dice: nothing is rolled twice.
// The window offers only the failed saves.
// The master turns the failures he chooses into successes, each one spending a use in the replay's own transaction (resistSpellSave), and the others stay as they were.
// When every such creature passed, the window offers nothing and the master's PASS lets the cast go on.
// A cast that targets no creature with the trait and a use left is never held.
// A player reads the wait any held action gives ("Esperando o mestre") and, after, an ordinary save result.

// errResistHeld ends the transaction of a cast that was held for the window: the cast's writes are rolled
// back and the hold stands for it.
var errResistHeld = errors.New("the cast waits for the master's Legendary Resistance")

// resistOffer is one failed save of a window: the creature and the roll that failed.
// A save that passed is never offered.
type resistOffer struct {
	Who   string `json:"who"`
	D20   int32  `json:"d20,omitempty"`
	Total int32  `json:"total,omitempty"`
	DC    int32  `json:"dc,omitempty"`
}

// keptSave is a target's saving throw as the hold keeps it: every number the replay needs to land the very
// same roll.
type keptSave struct {
	Target string   `json:"target"`
	Roll   saveRoll `json:"roll"`
}

// keptSave is the save the held cast rolled for the target, nil when it kept none (or when no cast is replayed).
func (st *replayState) keptSave(id string) *saveRoll {
	if st == nil {
		return nil
	}
	for i := range st.saves {
		if st.saves[i].Target == id {
			return &st.saves[i].Roll
		}
	}
	return nil
}

// resists says the master turned the target's failed save into a success.
func (st *replayState) resists(id string) bool {
	return st != nil && st.resisted[id]
}

// resistScope is the savepoint under which a cast that may be held for the window is made: the writes of a
// cast that turns out to wait are rolled back to it, and the combat's in-memory state with them.
type resistScope struct {
	sp     pgx.Tx
	before combatTx
	// capable are the targets with the trait and a use left, whose save holds the cast.
	capable map[string]bool
}

// guardResistance opens the savepoint of a cast that may be held.
// That is a spell with a saving throw that has, among its targets, a creature with Legendary Resistance and a use left whose save the master has not decided yet.
// The replay of an answered window has the savepoint too, so a replay that is dropped leaves nothing.
// A spell cast by a creature that is not a player's character, and any other cast, has no savepoint and is not held.
func (f *castFlow) guardResistance(ctx context.Context) (*resistScope, error) {
	if f.caster.Kind != kindPlayer || f.sp.SaveAbility == "" || f.sp.AttackType != "" || f.sp.HP != nil || f.laysZone() {
		return nil, nil
	}
	capable := map[string]bool{}
	replaying := false
	var undecided []playdb.Combatant
	for _, t := range f.targs {
		if f.c.replay.keptSave(t.ID) != nil {
			replaying = true // decided already: the replay of the window's answer
			continue
		}
		undecided = append(undecided, t)
	}
	resist, err := f.s.resistancesOf(ctx, f.c, undecided)
	if err != nil {
		return nil, err
	}
	for _, t := range undecided {
		capable[t.ID] = resist[t.ID].left > 0
	}
	if !replaying && !anyTrue(capable) {
		return nil, nil
	}
	sp, err := f.c.tx.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("guard the cast: %w", err)
	}
	return &resistScope{sp: sp, before: *f.c, capable: capable}, nil
}

func anyTrue(m map[string]bool) bool {
	for _, v := range m {
		if v {
			return true
		}
	}
	return false
}

// laysZone says the cast keeps an area on the map: the saves of the creatures it catches are another path.
func (f *castFlow) laysZone() bool {
	_, ok := zoneDefOfSpell(f.sp)
	return ok && f.sp.ZoneLaid
}

// resistance is a creature's uses of Legendary Resistance a day and what it has left.
type resistance struct{ perDay, left int }

// resistancesOf is the Legendary Resistance of each combatant, by ID: zero for a combatant without the
// trait. The trait works while the creature is incapacitated. The stat block keys are read in one call, the
// content once, and what each spent is what its row says, so the cost does not grow with the targets.
// The rows must be read in this change, before anything bumps creature_spent.
func (s *Service) resistancesOf(ctx context.Context, c *combatTx, whos []playdb.Combatant) (map[string]resistance, error) {
	out := make(map[string]resistance, len(whos))
	npcs := slices.DeleteFunc(slices.Clone(whos), func(who playdb.Combatant) bool { return who.Kind != kindNPC })
	if len(npcs) == 0 {
		return out, nil
	}
	keys, err := s.monsterKeysOf(ctx, c, npcs)
	if err != nil {
		return nil, err
	}
	content, err := s.contentOf(ctx, c)
	if err != nil {
		return nil, err
	}
	for _, who := range npcs {
		if p, _ := content.LegendaryProfile(keys[who.ID]); keys[who.ID] != "" && p.ResistancePerDay > 0 {
			out[who.ID] = resistance{p.ResistancePerDay, usesLeft(p.ResistancePerDay, parseSpent(who.CreatureSpent).count(spentLegendaryResistance))}
		}
	}
	return out, nil
}

// monsterKeysOf is monsterKeyOf for several combatants at once, by combatant ID: the keys of those that keep
// none on the combatant are read in one call.
func (s *Service) monsterKeysOf(ctx context.Context, c *combatTx, whos []playdb.Combatant) (map[string]string, error) {
	keys := make(map[string]string, len(whos))
	var unkeyed []string
	for _, who := range whos {
		if key := conv.OrZero(who.MonsterKey); key != "" {
			keys[who.ID] = key
		} else {
			unkeyed = append(unkeyed, who.CharacterID)
		}
	}
	if len(unkeyed) == 0 {
		return keys, nil
	}
	chars, err := s.roster.CombatCharacters(ctx, c.tx, c.session.CampaignID, unkeyed)
	if err != nil {
		return nil, err
	}
	byCharacter := make(map[string]string, len(chars))
	for _, ch := range chars {
		byCharacter[ch.ID] = ch.MonsterKey
	}
	for _, who := range whos {
		if keys[who.ID] == "" {
			keys[who.ID] = byCharacter[who.CharacterID]
		}
	}
	return keys, nil
}

// settle ends the guard: the cast that waits is already rolled back, a cast that failed is rolled back to the
// savepoint (so a replay that is dropped leaves nothing), and one that went through keeps its writes.
func (g *resistScope) settle(ctx context.Context, f *castFlow, payload any, err error) (any, error) {
	switch {
	case g == nil:
		return payload, err
	case errors.Is(err, errResistHeld):
		return f.out.made, nil
	case err != nil:
		g.rollback(ctx, f.c)
		return payload, err
	}
	if err := g.sp.Commit(ctx); err != nil {
		return nil, fmt.Errorf("release the guard of the cast: %w", err)
	}
	return payload, nil
}

// rollback undoes what the cast wrote since the savepoint, and what it told the change's state.
func (g *resistScope) rollback(ctx context.Context, c *combatTx) {
	_ = g.sp.Rollback(ctx) // the transaction decides what a failed rollback means: the next statement fails with it
	*c = g.before
}

// offerResistance holds the cast when a target has the trait and a use left, whether its save passed or failed.
// The writes of the cast are rolled back, and the request waits in a window of the master's with the saves as they were rolled.
// The window offers the failed saves of those targets, and none when they all passed.
func (f *castFlow) offerResistance(ctx context.Context) error {
	g := f.resist
	if g == nil {
		return nil
	}
	var offers []resistOffer
	var kept []keptSave
	var held []string
	for _, h := range f.out.made.Hits {
		if h.Save == nil {
			continue
		}
		kept = append(kept, keptSave{Target: h.Target, Roll: *h.Save})
		if !g.capable[h.Target] {
			continue
		}
		held = append(held, h.Target)
		if !h.Save.Saved {
			offers = append(offers, resistOffer{Who: h.Target, D20: h.Save.D20, Total: h.Save.Total, DC: h.Save.DC})
		}
	}
	if len(held) == 0 {
		return nil
	}
	g.rollback(ctx, f.c)
	f.out.vitals, f.out.hook = nil, nil
	ev := actionEvent{Round: f.c.enc.Round, Secret: f.caster.Hidden, Actor: f.caster.ID, Key: f.call.spellKey}
	for _, t := range f.targs {
		ev.Secret = ev.Secret || t.Hidden
	}
	spec := windowSpec{kind: reaction.LegendaryResistance, trigger: windowTrigger{
		Actor: f.caster.ID, Key: f.call.spellKey, Spell: f.call.spellKey, Level: conv.Int32(f.slotLevel), Roll: "cast", Resist: offers, Held: held,
	}}
	ev, err := f.s.holdAction(ctx, f.c, f.m, "cast", f.caster, f.req, holdData{Saves: kept}, []windowSpec{spec}, ev)
	if err != nil {
		return err
	}
	if err := touch(ctx, f.c); err != nil {
		return err
	}
	f.c.characterID = &f.caster.CharacterID
	f.out.made = ev
	return errResistHeld
}

// savedDice are the dice of a target's saving throw: the d20, the one Lucky rolled again, and the dice the
// effects on the target add.
type savedDice struct {
	d20        d20Roll
	lucky      *luckyReroll
	extra      []effectDie
	extraTotal int
}

// saveDice rolls the dice of a target's saving throw against a spell, or lands the dice the held cast rolled
// for it: the replay of a window's answer never rolls a save twice.
func (s *Service) saveDice(c *combatTx, target playdb.Combatant, save link.Save, mode combat.RollMode, sheet link.Sheet, effects []effectDie) (savedDice, error) {
	if k := c.replay.keptSave(target.ID); k != nil {
		out, ok := keptDice(k, save.Bonus, mode, effects)
		if !ok {
			return savedDice{}, errKeptDiceMismatch(target.ID)
		}
		return out, nil
	}
	d20, err := s.d20With(rollInput{inApp: true}, save.Bonus, mode)
	if err != nil {
		return savedDice{}, err
	}
	// Nobody is asked about this save: a halfling player's 1 is rolled again by the app (combat_lucky.go).
	d20, lucky, err := s.luckySave(d20, save.Bonus, mode, luckySaver(target, sheet.Traits))
	if err != nil {
		return savedDice{}, err
	}
	// Bênção and Perdição: a d4 on the target's saving throw.
	extra, extraTotal, err := s.rollEffectDice(rollInput{inApp: true}, effects, nil)
	if err != nil {
		return savedDice{}, err
	}
	return savedDice{d20: d20, lucky: lucky, extra: extra, extraTotal: extraTotal}, nil
}

// errKeptDiceMismatch is the refusal of the replay of a held save whose kept dice no longer fit it: the hold
// is dropped, with nothing spent, like any held action that cannot happen any more.
func errKeptDiceMismatch(target string) error {
	return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("the held save of %s does not fit the save as it stands", target))
}

// keptDice rebuilds the dice of a save from the roll the hold kept: the faces in the order they were rolled,
// the die that counted, the die Lucky rolled again and the faces of the effects' dice (which the state still
// lists, in the same order). It answers false when the roll does not fit the save as it stands: the replay then
// fails and the hold is dropped, since a save is never rolled twice.
func keptDice(k *saveRoll, bonus int, mode combat.RollMode, effects []effectDie) (savedDice, bool) {
	if k == nil || k.Auto || k.Sculpted || k.D20 == 0 || len(k.Extra) != len(effects) {
		return savedDice{}, false
	}
	faces := []int{int(k.D20)}
	if k.D20B != 0 {
		faces = intFaces(pairOf(k.D20, k.D20B, k.Counted))
	}
	if len(faces) != mode.Dice() || int(k.Counted) >= len(faces) {
		return savedDice{}, false
	}
	idx := int(k.Counted)
	out := savedDice{d20: d20Roll{Faces: faces, Index: idx, Total: faces[idx] + bonus}, lucky: k.Lucky, extra: make([]effectDie, len(effects))}
	for i, d := range effects {
		d.Face = k.Extra[i].Face
		out.extra[i] = d
		out.extraTotal += d.Sign * d.Face
	}
	return out, true
}

func intFaces(in []int32) []int {
	out := make([]int, len(in))
	for i, f := range in {
		out[i] = int(f)
	}
	return out
}

// resistSpellSave turns the failed save of a target into a success when the master chose to in the window of
// the held cast, spending the use in the replay's transaction. A save that passed, or a target the master left
// alone, keeps the result it had.
func (s *Service) resistSpellSave(ctx context.Context, c *combatTx, target playdb.Combatant, hit *castHit, saved bool) (bool, error) {
	if saved || hit.Save == nil || !c.replay.resists(target.ID) {
		return saved, nil
	}
	r, err := s.legendaryResistOpen(ctx, c, target, true)
	if err != nil {
		return false, err
	}
	resisted, left, err := r.resists(ctx, c, target, false)
	if err != nil || !resisted {
		return false, err
	}
	hit.Save.Saved, hit.Save.Resisted, hit.Save.ResistedLeft = true, true, left
	return true, nil
}

// resistedBack gives back the uses of Legendary Resistance a cast spent (its undo).
func (u *undoer) resistedBack(ctx context.Context, _ playdb.Combatant, ev actionEvent) error {
	for _, h := range ev.Hits {
		if _, here := u.find(h.Target); !here || h.Save == nil || !h.Save.Resisted {
			continue // a creature that left the combat has nothing to put back
		}
		if _, err := bumpCreatureSpent(ctx, u.c.q, h.Target, spentLegendaryResistance, -1); err != nil {
			return fmt.Errorf("give the Legendary Resistance back: %w", err)
		}
	}
	return nil
}

// answerLegendaryResistance answers the master's window: USE names the creatures whose failed save becomes a
// success (each must be offered and have a use left, or nothing is spent), PASS leaves every save as it was
// rolled. The uses are spent by the replay of the held cast, in this very transaction.
func (s *Service) answerLegendaryResistance(ctx context.Context, c *combatTx, w playdb.ReactionWindow, use bool, ids []string, got *answered) error {
	t := windowTriggerOf(w)
	re := &reactionEvent{Kind: string(reaction.LegendaryResistance), Window: w.ID, ByMaster: true, Used: use}
	got.ev = actionEvent{Round: c.enc.Round, Secret: true, Actor: t.Actor, Reaction: re}
	got.result = &playv1.ReactionResult{Kind: playv1.ReactionKind_REACTION_KIND_LEGENDARY_RESISTANCE, ByMaster: true, Used: use}
	chosen, err := s.resistChoice(ctx, c, t, use, ids)
	if err != nil {
		return err
	}
	out := windowOutcome{ByMaster: true, Used: use, Saved: chosen}
	if _, err := s.closeWindow(ctx, c, w, windowAnswered, reaction.ReasonNone, out, ""); err != nil {
		return err
	}
	if use {
		got.result.Result = &playv1.ReactionResult_LegendaryResistance{LegendaryResistance: &playv1.LegendaryResistanceResult{CombatantIds: chosen}}
	}
	return nil
}

// resistChoice checks the creatures a USE answer names: at least one, each offered by the window, once, with a
// use left. A PASS names none.
func (s *Service) resistChoice(ctx context.Context, c *combatTx, t windowTrigger, use bool, ids []string) ([]string, error) {
	if !use {
		return nil, nil
	}
	if len(ids) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("creature_ids must name the creatures whose save becomes a success"))
	}
	cs, err := listCombatants(ctx, c)
	if err != nil {
		return nil, err
	}
	var chosen []string
	for _, id := range ids {
		if _, ok := combatantByID(cs, id); !ok || !offered(t, id) || slices.Contains(chosen, id) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("creature_ids must name each creature of the window once"))
		}
		chosen = append(chosen, id)
	}
	whos := make([]playdb.Combatant, len(chosen))
	for i, id := range chosen {
		whos[i], _ = combatantByID(cs, id)
	}
	resist, err := s.resistancesOf(ctx, c, whos)
	if err != nil {
		return nil, err
	}
	for _, who := range whos {
		if resist[who.ID].left == 0 {
			return nil, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_LEGENDARY_RESISTANCE_NONE_LEFT, "the creature has no Legendary Resistance left")
		}
	}
	return chosen, nil
}

func offered(t windowTrigger, id string) bool {
	for _, o := range t.Resist {
		if o.Who == id {
			return true
		}
	}
	return false
}

// resistGone says no creature that holds the window is in the combat any more.
// Those are the targets with the trait and a use left, whatever their saves rolled, so the window lasts the same either way.
// A window stored without them falls back to the creatures it offers.
func resistGone(t windowTrigger, byID map[string]playdb.Combatant) bool {
	held := t.Held
	if len(held) == 0 {
		for _, o := range t.Resist {
			held = append(held, o.Who)
		}
	}
	for _, id := range held {
		if _, ok := byID[id]; ok {
			return false
		}
	}
	return len(held) > 0
}

// resistancePrompt is the master's question of a Legendary Resistance window: each failed save with the
// roll that failed and the uses its creature has left, and nothing when every save passed.
// Nobody else is shown the window.
func (wv *windowView) resistancePrompt(w playdb.ReactionWindow, out *playv1.ReactionWindow) error {
	t := windowTriggerOf(w)
	content, err := wv.s.roster.RulesContent(wv.ctx, nil, wv.m.CampaignID)
	if err != nil {
		return wv.s.dbError(wv.ctx, "read the rules content", err)
	}
	p := &playv1.LegendaryResistancePrompt{}
	for _, o := range t.Resist {
		who, ok := wv.byID[o.Who]
		if !ok {
			continue
		}
		key, err := wv.s.monsterKeyIn(wv.ctx, nil, wv.m.CampaignID, who)
		if err != nil {
			return wv.s.dbError(wv.ctx, "read the stat block key", err)
		}
		profile, _ := content.LegendaryProfile(key)
		spent := creatureSpentOf(who.CreatureSpent)
		p.Offers = append(p.Offers, &playv1.LegendaryResistanceOffer{
			CombatantId: who.ID, Label: who.Label, D20: o.D20, Total: o.Total, Dc: o.DC,
			UsesLeft: conv.Int32(usesLeft(profile.ResistancePerDay, spent[spentLegendaryResistance])), UsesPerDay: conv.Int32(profile.ResistancePerDay),
		})
	}
	out.Prompt = &playv1.ReactionWindow_LegendaryResistance{LegendaryResistance: p}
	return nil
}
