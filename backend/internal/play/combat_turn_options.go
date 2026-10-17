package play

import (
	"context"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// What a combatant can do now (GetTurnOptions), in phases. The read finds the combatant as the caller sees the
// fog (RN-10: a player never learns of what they cannot see), works the options out and disables them with
// the reason the combatant cannot act, and lists, for each attack and spell, who it can target as the viewer
// sees them. The phases are those of a turnOptionsFlow, each one of the reads the call has always made in that
// order: the golden of combat_actions_rest_frozen_options_test.go reads it, by viewer.

// turnOptionsCall is what GetTurnOptions reads from its request, checked.
type turnOptionsCall struct {
	encounterID, combatantID string
}

// readTurnOptionsCall reads the request, in the order the answers are given: the ids.
func readTurnOptionsCall(req *playv1.GetTurnOptionsRequest) (turnOptionsCall, error) {
	var call turnOptionsCall
	var err error
	if call.encounterID, err = parseCombatID(req.GetEncounterId(), "encounter"); err != nil {
		return call, err
	}
	call.combatantID, err = parseCombatID(req.GetCombatantId(), "combatant")
	return call, err
}

// GetTurnOptions implements playv1connect.CombatServiceHandler.
func (s *Service) GetTurnOptions(
	ctx context.Context,
	req *connect.Request[playv1.GetTurnOptionsRequest],
) (*connect.Response[playv1.GetTurnOptionsResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	call, err := readTurnOptionsCall(req.Msg)
	if err != nil {
		return nil, err
	}
	res, err := newTurnOptionsFlow(s, m, call).run(ctx)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

// turnOptionsFlow is one run of the read: what each phase reads, and the next one uses.
type turnOptionsFlow struct {
	s    *Service
	m    authz.Membership
	call turnOptionsCall

	d     *encounterData
	enc   playdb.Encounter
	v     combatViewer
	sight *fogSight
	who   playdb.Combatant
	opts  *rulesv1.TurnOptions
	// code is why the combatant cannot act now, or UNSPECIFIED.
	code rulesv1.DisabledReasonCode
	// terrain is the one the viewer plans on.
	terrain grid.Terrain
	modes   turnModes
	res     *playv1.GetTurnOptionsResponse
}

func newTurnOptionsFlow(s *Service, m authz.Membership, call turnOptionsCall) *turnOptionsFlow {
	return &turnOptionsFlow{s: s, m: m, call: call}
}

// run is the read: it answers the options and the targets.
func (f *turnOptionsFlow) run(ctx context.Context) (*playv1.GetTurnOptionsResponse, error) {
	phases := []func(context.Context) error{f.locate, f.options, f.plan, f.weigh, f.attackTargets, f.spellTargets, f.pending, f.contests}
	for _, phase := range phases {
		if err := phase(ctx); err != nil {
			return nil, err
		}
	}
	return f.res, nil
}

// locate finds the combatant as the caller sees the fog, and checks the caller may act for it.
func (f *turnOptionsFlow) locate(ctx context.Context) error {
	_, d, err := f.s.readEncounter(ctx, f.m.CampaignID, f.call.encounterID)
	if err != nil {
		return err
	}
	f.d, f.enc = d, d.enc
	if f.v, f.sight, err = f.s.viewerWith(ctx, f.m, f.enc, d.cs); err != nil {
		return f.s.dbError(ctx, "work out what the player sees", err)
	}
	if f.who, err = findCombatant(d.cs, f.call.combatantID, f.v); err != nil {
		return err
	}
	return f.v.mayAct(f.who)
}

// options works out the options of the combatant, and disables them all with the reason it cannot act.
func (f *turnOptionsFlow) options(ctx context.Context) error {
	var err error
	if f.opts, err = f.s.optionsOf(ctx, nil, f.m.CampaignID, f.who); err != nil {
		return f.s.dbError(ctx, "work out the turn options", err)
	}
	if err := f.s.attachMetamagic(ctx, nil, f.m.CampaignID, f.who, f.opts); err != nil {
		return f.s.dbError(ctx, "work out the Metamagic", err)
	}
	if f.code, err = f.s.gate(ctx, nil, f.m.CampaignID, f.enc, f.who); err != nil {
		return f.s.dbError(ctx, "check the combatant's turn", err)
	}
	if f.code != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_UNSPECIFIED {
		disableAll(f.opts, f.code)
		f.s.explainEffectReason(f.d, f.who, f.v, f.opts, f.code, f.s.namesFor(ctx, f.m.CampaignID))
	}
	return nil
}

// plan reads the terrain the viewer plans on and the table's rules, and starts the answer. The cover a player
// is shown, and the targets it makes untargetable, come from the terrain they know and the creatures they see.
func (f *turnOptionsFlow) plan(ctx context.Context) error {
	terrain, err := f.s.terrainOf(ctx, nil, f.m.CampaignID, f.enc)
	if err != nil {
		return f.s.dbError(ctx, "read the terrain", err)
	}
	known, err := f.sight.knownTerrain(ctx, nil, f.v)
	if err != nil {
		return f.s.dbError(ctx, "read the terrain the player knows", err)
	}
	f.terrain = planOn(f.v, terrain, known)
	table, err := f.s.tableRules(ctx, nil, f.m.CampaignID)
	if err != nil {
		return f.s.dbError(ctx, "read the table's rules", err)
	}
	f.res = &playv1.GetTurnOptionsResponse{Options: f.opts, YourTurn: actsNow(f.enc, f.who), CriticalRule: criticalRuleProto(table)}
	f.s.effectTurnInfo(f.d, f.who, f.v, f.res, f.s.namesFor(ctx, f.m.CampaignID))
	return nil
}

// weigh works out the roll modes of the turn. The master's turn options carry no sight (they plan on the real
// terrain), but the mode of an attack depends on who the players see (an NPC they cannot see attacks with
// advantage, SRD 5.1): RollAttack reads it for the master too, so the suggestion must, or the typed d20 count
// is wrong.
func (f *turnOptionsFlow) weigh(ctx context.Context) error {
	modeSight := f.sight
	if modeSight == nil && f.v.master {
		var err error
		if modeSight, err = f.s.fogSightOf(ctx, f.m.CampaignID, f.enc); err != nil {
			return f.s.dbError(ctx, "work out what the players see", err)
		}
	}
	var err error
	if f.modes, err = f.s.turnModes(ctx, f.m.CampaignID, f.d, modeSight, f.v, f.who); err != nil {
		return f.s.dbError(ctx, "work out the roll modes", err)
	}
	return nil
}

// attackTargets lists who each attack can target.
func (f *turnOptionsFlow) attackTargets(context.Context) error {
	for _, a := range f.opts.GetAttacks() {
		if a.GetAttack().GetSaveDc() > 0 {
			continue // a saving throw, not an attack roll: the spells slice
		}
		targets := targetsFor(f.terrain, f.d.cs, f.who, f.v, reachFt(a.GetAttack().GetRangeFt(), a.GetAttack().GetLongRangeFt()), isTheatre(f.enc))
		f.modes.annotate(a.GetAttack().GetKey(), targets)
		f.modes.dropIgnoredCover(a.GetAttack().GetKey(), targets)
		f.res.AttackTargets = append(f.res.AttackTargets, &playv1.AttackTargets{AttackKey: a.GetAttack().GetKey(), Targets: targets})
	}
	return nil
}

// spellTargets lists who each spell can target, and the targets of the resources.
func (f *turnOptionsFlow) spellTargets(ctx context.Context) error {
	scene := spellTargetScene{terrain: f.terrain, who: f.who, cs: f.d.cs, v: f.v, theatre: isTheatre(f.enc), modes: f.modes}
	var err error
	if f.res.SpellTargets, err = f.s.spellTargetsFor(ctx, f.m.CampaignID, scene, f.opts); err != nil {
		return f.s.dbError(ctx, "work out the spell targets", err)
	}
	f.res.ResourceTargets = resourceTargetsFor(f.terrain, f.d.cs, f.who, f.v, f.opts, f.enc)
	return nil
}

// pending lists the damage the combatant's hits left waiting, the ones the viewer may know of (RN-10).
func (f *turnOptionsFlow) pending(ctx context.Context) error {
	open, err := f.s.queries.ListOpenPendingDamages(ctx, f.enc.ID)
	if err != nil {
		return f.s.dbError(ctx, "list the pending damage", err)
	}
	for _, p := range open {
		if conv.OrZero(p.AttackerID) == f.who.ID && pendingVisible(p, f.d.cs, f.v) {
			f.res.PendingDamages = append(f.res.PendingDamages, f.s.pendingView(ctx, f.m.CampaignID, p, f.d.cs, f.v))
		}
	}
	return nil
}

// contests lists the contests (a grapple, a shove) the combatant can start, and where the turn stands on them.
func (f *turnOptionsFlow) contests(ctx context.Context) error {
	var err error
	if f.res.ContestAttackOptions, f.res.ContestState, err = f.s.contestOptionsFor(ctx, f.m, f.d, f.v, f.who, f.opts, f.code); err != nil {
		return f.s.dbError(ctx, "work out the contests of the turn", err)
	}
	return nil
}
