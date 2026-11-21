package play

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math"
	"slices"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// Channel Divinity (SRD 5.1, Cleric): Preserve Life of the Life Domain restores hit points, and Turn Undead
// makes the undead near the cleric save or flee. Both spend the action and a use of the resource, as every feature
// action does (combat_take_action.go); these are what each one does besides.
const (
	preserveLifeAction = "feature:channel-divinity-preserve-life"
	turnUndeadAction   = "feature:channel-divinity-turn-undead"
	// channelRangeFt is how far from the cleric both reach.
	channelRangeFt = 30
	// preserveLifePerLevel is the hit points Preserve Life restores for each level of the cleric.
	preserveLifePerLevel = 5
	// turnedRounds is the minute a turned undead stays turned.
	turnedRounds = 10
	turnedEffect = "effect:turned"
)

// preserveHeal is the hit points the cleric gives one creature.
type preserveHeal struct {
	target string
	points int32
}

// channelEvent is what a Channel Divinity action did, kept on the action's event for the log, the answer and the
// undo.
type channelEvent struct {
	Heals []channelHeal `json:"heals,omitempty"`
	Turns []channelTurn `json:"turns,omitempty"`
	DC    int32         `json:"dc,omitempty"`
	// Effects are the rows of the effect "turned" the action made, on Targets, which the undo takes away.
	Effects []string `json:"effects,omitempty"`
	Targets []string `json:"targets,omitempty"`
	// UnderFog says whether the action was made with the fog on. The record of each entry (fogSeen) tells who saw it
	// when the fog was on, so it is written only as false, for an action on a map with no fog. An older event may
	// carry it as true, and then lists no entry to a player that has no record.
	UnderFog *bool `json:"under_fog,omitempty"`
}

// channelHeal is what Preserve Life gave one creature: what was asked, what the limit left, and the hit points
// before, with what an undo needs.
type channelHeal struct {
	Target      string      `json:"target_id"`
	Label       string      `json:"label"`
	Asked       int32       `json:"asked"`
	Healed      int32       `json:"healed"`
	After       int32       `json:"after"`
	Before      *hpState    `json:"before,omitempty"`
	DeathBefore *deathState `json:"death_before,omitempty"`
	// HiddenAtAction says the master had the creature hidden at the action.
	HiddenAtAction bool `json:"hidden_at_action,omitempty"`
	fogSeen
}

// channelTurn is the saving throw of one undead against Turn Undead.
type channelTurn struct {
	Target   string `json:"target_id"`
	Label    string `json:"label"`
	D20      int32  `json:"d20"`
	Modifier int32  `json:"modifier"`
	Total    int32  `json:"total"`
	Turned   bool   `json:"turned,omitempty"`
	// HiddenAtAction says the master had the undead hidden at the action.
	HiddenAtAction bool `json:"hidden_at_action,omitempty"`
	fogSeen
	// countedRoll is the number the d20 counted as when a minimum raised it: D20 is what came up and Total uses this one.
	countedRoll
}

// readPreserveLife reads the hit points asked of Preserve Life: one entry for each creature, 1 or more points.
func readPreserveLife(req *playv1.TakeActionRequest) ([]preserveHeal, error) {
	if len(req.GetPreserveLife()) > maxCombatants {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("preserve_life must have at most %d entries", maxCombatants))
	}
	var out []preserveHeal
	seen := make(map[string]struct{}, len(req.GetPreserveLife()))
	for _, h := range req.GetPreserveLife() {
		id, err := parseCombatID(h.GetCombatantId(), "combatant")
		if err != nil {
			return nil, err
		}
		if h.GetHitPoints() < 1 || h.GetHitPoints() > maxChannelPoints {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("hit_points must be 1 to 9999"))
		}
		if _, dup := seen[id]; dup {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a creature appears once in preserve_life"))
		}
		seen[id] = struct{}{}
		out = append(out, preserveHeal{target: id, points: h.GetHitPoints()})
	}
	return out, nil
}

const maxChannelPoints = 9999

// preserveLife is Preserve Life: it restores the hit points asked to the creatures within 30 ft, each to no more
// than half of its maximum, up to five times the cleric's level in all. An undead and a construct are not restored.
func (f *takeActionFlow) preserveLife(ctx context.Context, sheet link.Sheet) error {
	limit := clamp32(preserveLifePerLevel*sheet.ClericLevel, 0, math.MaxInt32)
	asked := f.call.preserveLife
	var total int32
	for _, h := range asked {
		total += h.points
	}
	if total > limit {
		return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_CHANNEL_DIVINITY_TOO_MUCH,
			fmt.Sprintf("Preserve Life restores up to %d hit points in all", limit))
	}
	cs, err := f.c.q.ListCombatants(ctx, f.c.enc.ID)
	if err != nil {
		return fmt.Errorf("list the combatants: %w", err)
	}
	may := f.namable(cs)
	if len(asked) == 0 { // a request that names no one: the points go to whoever is hurt the most
		if asked, err = f.neediest(ctx, may, limit); err != nil {
			return err
		}
	}
	ev := &channelEvent{}
	if f.c.sight == nil {
		ev.UnderFog = new(false)
	}
	for _, h := range asked {
		target, err := f.preserveTarget(ctx, may, h.target)
		if err != nil {
			return err
		}
		heal, err := f.restore(ctx, target, h.points)
		if err != nil {
			return err
		}
		heal.HiddenAtAction = target.Hidden
		ev.Heals = append(ev.Heals, heal)
	}
	f.made.Channel = ev
	return nil
}

// namable are the creatures the caller may name for Preserve Life, and the ones it shares the points among: the ones in the
// combat that the answer's viewer sees. A request names no other, and one that does is refused as it is for a creature that
// is not in the combat.
func (f *takeActionFlow) namable(cs []playdb.Combatant) []playdb.Combatant {
	return slices.DeleteFunc(slices.Clone(cs), func(c playdb.Combatant) bool { return c.Dismissed || !f.out.v.sees(c) })
}

// neediest shares the points among the cleric's side: the creatures of `may` within 30 ft that are not undead or constructs
// and are below half of their maximum, the lowest in proportion first, each as far as that half.
func (f *takeActionFlow) neediest(ctx context.Context, may []playdb.Combatant, points int32) ([]preserveHeal, error) {
	type need struct {
		id   string
		room int32
		hurt float64
	}
	var needs []need
	for _, c := range may {
		if c.Side != f.who.Side {
			continue
		}
		if _, err := f.preserveTarget(ctx, may, c.ID); err != nil {
			continue // too far, an undead or a construct: not a target
		}
		now, most, err := f.s.hitPointsNow(ctx, f.c, f.m.CampaignID, c)
		if err != nil {
			return nil, err
		}
		if room := most/2 - now; room > 0 && most > 0 {
			needs = append(needs, need{id: c.ID, room: room, hurt: float64(now) / float64(most)})
		}
	}
	slices.SortStableFunc(needs, func(a, b need) int { return cmp.Compare(a.hurt, b.hurt) })
	var out []preserveHeal
	for _, n := range needs {
		if points <= 0 {
			break
		}
		give := min(n.room, points)
		out = append(out, preserveHeal{target: n.id, points: give})
		points -= give
	}
	return out, nil
}

// preserveTarget finds a creature Preserve Life may restore: one of `may` (namable), within 30 ft of the cleric, that
// is not an undead or a construct.
func (f *takeActionFlow) preserveTarget(ctx context.Context, may []playdb.Combatant, id string) (playdb.Combatant, error) {
	i := slices.IndexFunc(may, func(c playdb.Combatant) bool { return c.ID == id })
	if i < 0 {
		return playdb.Combatant{}, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_CHANNEL_DIVINITY_TARGET, "that creature is not in the combat")
	}
	target := may[i]
	if d, known := distanceFt(f.who, target); known && d > channelRangeFt {
		return target, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_CHANNEL_DIVINITY_TARGET, target.Label+" is more than 30 ft away")
	}
	kind, err := f.creatureType(ctx, target)
	if err != nil {
		return target, err
	}
	if kind == "undead" || kind == "construct" {
		return target, errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_CHANNEL_DIVINITY_TARGET, "Preserve Life cannot restore an undead or a construct")
	}
	return target, nil
}

// creatureType is the SRD type of a combatant's creature ("undead"), empty for a character.
func (f *takeActionFlow) creatureType(ctx context.Context, c playdb.Combatant) (string, error) {
	sheet, err := f.s.sheetOf(ctx, f.c.tx, f.m.CampaignID, c)
	if err != nil {
		return "", err
	}
	return sheet.Traits.CreatureType, nil
}

// restore gives the creature the hit points asked, no further than half of its maximum.
func (f *takeActionFlow) restore(ctx context.Context, target playdb.Combatant, asked int32) (channelHeal, error) {
	now, most, err := f.s.hitPointsNow(ctx, f.c, f.m.CampaignID, target)
	if err != nil {
		return channelHeal{}, err
	}
	room := most/2 - now
	heal := channelHeal{Target: target.ID, Label: target.Label, Asked: asked, After: now}
	if room <= 0 {
		return heal, nil
	}
	hit, _, err := f.s.healCombatant(ctx, f.c, target, min(asked, room))
	if err != nil {
		return heal, err
	}
	heal.Healed, heal.Before, heal.DeathBefore = hit.Amount, hit.Before, hit.DeathBefore
	if hit.After != nil {
		heal.After = hit.After.HP
	}
	return heal, nil
}

// hitPointsNow is the current hit points and the maximum of a combatant: a creature's own, a character's vitals.
func (s *Service) hitPointsNow(ctx context.Context, c *combatTx, campaignID string, who playdb.Combatant) (current, maximum int32, err error) {
	if holdsHP(who) {
		return hpOf(who).HP, conv.OrZero(who.HpMax), nil
	}
	v, err := s.vitals.GetVitalsTx(ctx, c.tx, campaignID, who.CharacterID)
	if err != nil {
		return 0, 0, fmt.Errorf("read the hit points: %w", err)
	}
	return v.GetHitPointsCurrent(), v.GetHitPointsMax(), nil
}

// turnUndead is Turn Undead: each undead within 30 ft that can see or hear the cleric makes a Wisdom saving throw
// against the cleric's DC, and the ones that fail are turned for a minute (the effect "turned").
func (f *takeActionFlow) turnUndead(ctx context.Context, sheet link.Sheet) error {
	cs, err := f.c.q.ListCombatants(ctx, f.c.enc.ID)
	if err != nil {
		return fmt.Errorf("list the combatants: %w", err)
	}
	states, err := f.s.readStatesAt(ctx, f.c.tx, f.c.enc.ID, nil)
	if err != nil {
		return err
	}
	dc := clamp32(sheet.ChannelDC, 0, math.MaxInt32)
	ev := &channelEvent{DC: dc}
	if f.c.sight == nil {
		ev.UnderFog = new(false)
	}
	var turned []playdb.Combatant
	for _, target := range cs {
		if target.ID == f.who.ID || target.Defeated || target.Dismissed || !f.hearsTheCleric(target) {
			continue
		}
		undead, err := f.creatureSave(ctx, target, states, dc)
		if err != nil || undead == nil {
			if err != nil {
				return err
			}
			continue
		}
		undead.HiddenAtAction = target.Hidden
		ev.Turns = append(ev.Turns, *undead)
		if undead.Turned {
			turned = append(turned, target)
		}
	}
	f.made.Channel = ev
	return f.turn(ctx, cs, turned, ev)
}

// hearsTheCleric says the combatant is within 30 ft of the cleric and neither blind nor deaf: it can see or hear it.
func (f *takeActionFlow) hearsTheCleric(c playdb.Combatant) bool {
	if d, known := distanceFt(f.who, c); known && d > channelRangeFt {
		return false
	}
	return !slices.Contains(c.Conditions, "condition:blinded") || !slices.Contains(c.Conditions, "condition:deafened")
}

// creatureSave rolls the Wisdom saving throw of an undead against the DC; nil for a creature that is not one.
func (f *takeActionFlow) creatureSave(ctx context.Context, target playdb.Combatant, states map[string][]playdb.CombatantState, dc int32) (*channelTurn, error) {
	sheet, err := f.s.sheetOf(ctx, f.c.tx, f.m.CampaignID, target)
	if err != nil {
		return nil, err
	}
	if sheet.Traits.CreatureType != "undead" {
		return nil, nil
	}
	save, err := f.s.saveOf(ctx, f.c.tx, f.m.CampaignID, target, "wis")
	if err != nil {
		return nil, err
	}
	sun, err := f.s.sunlightOf(ctx, f.c.q, f.c.enc)
	if err != nil {
		return nil, err
	}
	creature := sun.facts(creatureFacts(target, states, sheet.Traits), target)
	mode := combat.Resolve(combat.SaveMode(combat.SaveScene{Creature: creature, Ability: "wis", EffectVisible: true}))
	d20, err := f.s.d20With(rollInput{inApp: true}, save.Bonus, mode)
	if err != nil {
		return nil, err
	}
	return turnedSave(target, save.Bonus, d20, dc, floorFor(sheet.Traits, rules.RollAppliesSave, false)), nil
}

// turnedSave is the save of an undead against Turn Undead: the d20 that counts is counted by the floor (a table's roll_minimum),
// the faces stay as they came up, and the total decides whether it is turned.
func turnedSave(target playdb.Combatant, bonus int, d20 d20Roll, dc int32, floor rollFloor) *channelTurn {
	d20 = floorD20(d20, bonus, floor)
	return &channelTurn{
		Target: target.ID, Label: target.Label, D20: clamp32(d20.Face(), 1, 20), Modifier: clamp32(bonus, math.MinInt32, math.MaxInt32),
		Total: conv.Int32(d20.Total), Turned: !combat.SaveSucceeded(d20.Total, int(dc)), countedRoll: countedOf(d20),
	}
}

// turn puts the effect "turned" on the undead that failed, for a minute, and keeps its rows for the undo.
func (f *takeActionFlow) turn(ctx context.Context, cs []playdb.Combatant, turned []playdb.Combatant, ev *channelEvent) error {
	if len(turned) == 0 {
		return nil
	}
	content, err := f.s.contentOf(ctx, f.c)
	if err != nil {
		return err
	}
	def, _ := f.s.effectDef(content, turnedEffect)
	caster := f.who
	rows, err := f.s.addEffects(ctx, f.c, cs, effectSpec{
		key: turnedEffect, sourceKind: "feature", def: def, caster: &caster, targets: turned,
		dur: durationSpec{Kind: rules.EffectDurationRounds, Rounds: turnedRounds, Anchor: caster.ID},
	})
	if err != nil {
		return err
	}
	for _, r := range rows {
		ev.Effects = append(ev.Effects, r.ID)
	}
	for _, t := range turned {
		ev.Targets = append(ev.Targets, t.ID)
	}
	return nil
}

// channelReader is what a reader of a Channel Divinity result reads: reads says whether the entry for a creature is theirs
// (the creature, whether the master had it hidden at the action, and the record of who saw it), and numbers whether the
// combat view gives them that creature's hit points as numbers.
type channelReader struct {
	reads   func(target string, hiddenAtAction bool, seen fogSeen) bool
	numbers func(target string) bool
}

// atTheCall is the reader of the answer to the action: the entries of the creatures the viewer sees, that the master
// had not hidden at the action.
func (v combatViewer) atTheCall(byID map[string]playdb.Combatant) channelReader {
	return channelReader{
		reads:   func(id string, hidden bool, _ fogSeen) bool { return v.seesAtAction(byID[id], hidden) },
		numbers: v.numbersOf(byID),
	}
}

// numbersOf says, for a creature of byID, whether the viewer reads its hit points as numbers (readsHitPointsOf).
func (v combatViewer) numbersOf(byID map[string]playdb.Combatant) func(string) bool {
	return func(id string) bool {
		c, found := byID[id]
		return found && v.readsHitPointsOf(c)
	}
}

// channelProto is what a Channel Divinity action did, for the answer and the log: the heals and the saves that the reader
// reads, each heal with the hit points after only where the reader reads that creature's as numbers.
func channelProto(ev *channelEvent, r channelReader) *playv1.ChannelDivinityResult {
	if ev == nil {
		return nil
	}
	out := &playv1.ChannelDivinityResult{SaveDc: ev.DC}
	for _, h := range ev.Heals {
		if !r.reads(h.Target, h.HiddenAtAction, h.fogSeen) {
			continue
		}
		heal := &playv1.ChannelDivinityHeal{CombatantId: h.Target, Label: h.Label, Asked: h.Asked, Healed: h.Healed}
		if r.numbers(h.Target) {
			heal.HitPointsAfterKnown = new(h.After)
		}
		out.Heals = append(out.Heals, heal)
	}
	for _, t := range ev.Turns {
		if !r.reads(t.Target, t.HiddenAtAction, t.fogSeen) {
			continue
		}
		turn := &playv1.ChannelDivinityTurn{CombatantId: t.Target, Label: t.Label, D20: t.D20, Modifier: t.Modifier, Total: t.Total, Turned: t.Turned}
		if t.CountedAs != 0 {
			turn.TreatedAs, turn.TreatedAsSource = new(t.CountedAs), legacySource(t.CountedBy)
		}
		out.Turns = append(out.Turns, turn)
	}
	return out
}
