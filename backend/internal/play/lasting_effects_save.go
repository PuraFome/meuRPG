package play

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
	"github.com/PuraFome/meuRPG/backend/internal/rules/reaction"
)

// The saving throw an effect asks (RN-22): Imobilizar Pessoa and Riso Histérico at the end
// of the target's turn ("o alvo pode fazer outro teste de resistência"), Teia at the
// start of it. It is a reaction window of its own kind (PM-04), answered with
// RollEffectSave: the target's player rolls in the app or types the d20, or leaves it to
// the master; the master rolls for an NPC, or skips the save (a table option the SRD does
// not give). While a window is open the turn does not move (reactionGate), and the turn
// that ended waits for it to pass (releaseHeldTurn).

// defaultEffectDC is the DC an effect asks when nobody gave one (a master's effect with
// no caster): the SRD's easy-to-hard middle is 10 to 15 and the table says the rest.
const defaultEffectDC = 10

// openEffectSave opens the saving throw of an effect on its target at a phase of the turn,
// once for the turn: a repeat finds the window open and opens none. damage says the
// window opened because the target took damage (advantage on the roll).
func (s *Service) openEffectSave(ctx context.Context, c *combatTx, group string, st playdb.CombatantState, target playdb.Combatant, phase string, damage bool) (bool, error) {
	open, err := openWindowsOf(ctx, c)
	if err != nil {
		return false, err
	}
	if slices.ContainsFunc(open, func(w playdb.ReactionWindow) bool {
		t := windowTriggerOf(w)
		return reaction.Kind(w.Kind) == reaction.EffectSave && t.Effect == st.ID && t.Phase == phase && (t.OnDamage == damage || !damage)
	}) {
		return false, nil
	}
	trigger := windowTrigger{Actor: deref(st.SourceID), Target: target.ID, Key: deref(st.SourceKey), Effect: st.ID, Phase: phase, OnDamage: damage}
	_, err = s.openWindows(ctx, c, group, nil, []windowSpec{{kind: reaction.EffectSave, reactor: &target, trigger: trigger}})
	return err == nil, err
}

// closeEffectWindows closes the saving throws an effect row had open: it ended, or its
// caster lost the concentration.
func (s *Service) closeEffectWindows(ctx context.Context, c *combatTx, st playdb.CombatantState, reason string) error {
	open, err := openWindowsOf(ctx, c)
	if err != nil {
		return err
	}
	for _, w := range open {
		if reaction.Kind(w.Kind) != reaction.EffectSave || windowTriggerOf(w).Effect != st.ID {
			continue
		}
		if _, err := s.closeWindow(ctx, c, w, windowClosed, windowReasonOf(reason), windowOutcome{}, ""); err != nil {
			return err
		}
	}
	return nil
}

// effectDie is a die an effect adds to or takes from a roll, with the face it showed.
type effectDie struct {
	Key   string
	Faces int
	Sign  int
	Face  int
}

// effectDiceFor lists the dice the effects on a combatant add to a roll of the kind
// (rules.RollAppliesAttack or rules.RollAppliesSave), in the order of the effects.
func effectDiceFor(states map[string][]playdb.CombatantState, id, applies string) []effectDie {
	var out []effectDie
	for _, st := range effectsOn(states, id) {
		for _, d := range combat.EffectDice(deref(st.SourceKey), effectModifiers(st), applies) {
			out = append(out, effectDie{Key: d.Source, Faces: d.Faces, Sign: d.Sign})
		}
	}
	return out
}

// rollEffectDice rolls the dice an effect adds to a roll. With the app's dice the server
// rolls them; with physical dice the player typed the faces, one for each, in the same
// order. It returns the dice with their faces and the number they add.
func (s *Service) rollEffectDice(in rollInput, dd []effectDie, typed []int32) ([]effectDie, int, error) {
	if len(dd) == 0 {
		return nil, 0, nil
	}
	if !in.inApp && len(typed) != len(dd) {
		return nil, 0, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("the roll takes %d more die(s): type their faces in extra_die_faces", len(dd)))
	}
	total := 0
	out := make([]effectDie, len(dd))
	for i, d := range dd {
		face := 0
		if in.inApp {
			r, err := dice.Roll(s.roller, dice.Expr{Count: 1, Sides: d.Faces})
			if err != nil {
				return nil, 0, fmt.Errorf("roll an effect's die: %w", err)
			}
			face = r.Faces[0]
		} else {
			face = int(typed[i])
			if face < 1 || face > d.Faces {
				return nil, 0, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("extra_die_faces[%d] must be 1 to %d", i, d.Faces))
			}
		}
		d.Face = face
		out[i] = d
		total += d.Sign * face
	}
	return out, total, nil
}

// extraDiceProto writes the dice an effect added, for who reads the roll: the effect's name,
// or "Outra fonte" with no key when the master hides the effect from the players (RN-20).
func (s *Service) extraDiceProto(dd []effectDie, reveal func(key string) bool, names func(string) string) []*playv1.ExtraDie {
	out := make([]*playv1.ExtraDie, 0, len(dd))
	for _, d := range dd {
		e := &playv1.ExtraDie{SourceKey: d.Key, SourceNamePt: names(d.Key), Faces: clamp32(d.Faces, 0, 100), Sign: clamp32(d.Sign, -1, 1), Face: clamp32(d.Face, 0, 100)}
		if !reveal(d.Key) {
			e.SourceKey, e.SourceNamePt = "", "Outra fonte"
		}
		out = append(out, e)
	}
	return out
}

// effectSaveRollOf reads how an effect's saving throw is answered.
func effectSaveRollOf(msg *playv1.RollEffectSaveRequest) (in rollInput, delegate, skip bool, err error) {
	bad := func(text string) error { return connect.NewError(connect.CodeInvalidArgument, errors.New(text)) }
	switch roll := msg.GetRoll().(type) {
	case *playv1.RollEffectSaveRequest_RollInApp:
		if !roll.RollInApp {
			return in, false, false, bad("roll_in_app must be true")
		}
		in.inApp = true
	case *playv1.RollEffectSaveRequest_D20Face:
		in.typed = int(roll.D20Face)
		if in.typed < 1 || in.typed > 20 {
			return in, false, false, bad("d20_face must be 1 to 20")
		}
		if msg.SecondD20Face != nil {
			in.typedFaces = []int{in.typed, int(msg.GetSecondD20Face())}
		}
	case *playv1.RollEffectSaveRequest_DelegateToMaster:
		if !roll.DelegateToMaster {
			return in, false, false, bad("delegate_to_master must be true")
		}
		delegate = true
	case *playv1.RollEffectSaveRequest_Skip:
		if !roll.Skip {
			return in, false, false, bad("skip must be true")
		}
		skip = true
	default:
		return in, false, false, bad("set roll_in_app, d20_face, delegate_to_master or skip")
	}
	return in, delegate, skip, nil
}

// RollEffectSave implements playv1connect.LastingEffectServiceHandler.
func (s *Service) RollEffectSave(
	ctx context.Context,
	req *connect.Request[playv1.RollEffectSaveRequest],
) (*connect.Response[playv1.RollEffectSaveResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	encID, err := parseCombatID(req.Msg.GetEncounterId(), "encounter")
	if err != nil {
		return nil, err
	}
	winID, err := parseCombatID(req.Msg.GetWindowId(), "window")
	if err != nil {
		return nil, err
	}
	in, delegate, skip, err := effectSaveRollOf(req.Msg)
	if err != nil {
		return nil, err
	}
	typedExtra := req.Msg.GetExtraDieFaces()

	var made actionEvent
	var result *playv1.EffectSaveResult
	res, err := s.write(ctx, combatWrite{m: m, key: key, hash: idem.Hash(req.Msg), kind: eventLastingSaved, encounterID: encID}, func(c *combatTx) (any, error) {
		result = nil
		if err := notEnded(c.enc); err != nil {
			return nil, err
		}
		w, reactor, err := s.windowToAnswer(ctx, c, m, winID)
		if err != nil {
			return nil, err
		}
		if reaction.Kind(w.Kind) != reaction.EffectSave {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("this window is not an effect's saving throw: use AnswerReaction"))
		}
		t := windowTriggerOf(w)
		master := m.Role == authz.RoleMaster
		switch {
		case skip && !master:
			return nil, errWindowDenied()
		case delegate && master:
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("the master rolls it: use roll_in_app or d20_face"))
		case t.Handed && !master:
			return nil, errWindowDenied() // the roll was left to the master
		}
		if delegate {
			return nil, s.handSaveToMaster(ctx, c, w, t)
		}
		st, err := c.q.GetLastingEffect(ctx, playdb.GetLastingEffectParams{ID: t.Effect, EncounterID: c.enc.ID})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errNotYourTurnToAnswer() // the window closes with its effect: never reached
		}
		if err != nil {
			return nil, fmt.Errorf("find the effect: %w", err)
		}
		content, err := s.contentOf(ctx, c)
		if err != nil {
			return nil, err
		}
		def, _ := s.effectDef(content, deref(st.SourceKey))
		ability := deref(st.EndSaveAbility)
		if t.Phase == combat.PhaseStart {
			ability = deref(st.StartSaveAbility)
		}
		dc := derefInt32(st.SaveDc)
		if dc == 0 {
			dc = defaultEffectDC
		}
		ev := actionEvent{Round: c.enc.Round, Secret: s.effectHiddenFrom(st, []playdb.Combatant{reactor}), Actor: reactor.ID, Key: deref(st.SourceKey)}
		le := &lastingEvent{Key: deref(st.SourceKey), Change: "saved", Targets: []string{reactor.ID}, Caster: deref(st.SourceID), Ability: ability, DC: dc, Effects: []string{st.ID}}
		out := windowOutcome{ByMaster: master, Used: true}
		saved := false
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		switch {
		case skip:
			le.Change = "skipped"
			saved = false
			result = &playv1.EffectSaveResult{Skipped: true}
		default:
			if !master {
				if err := s.mustRollThisWay(ctx, c.tx, m, in); err != nil {
					return nil, err
				}
			}
			states, err := s.readStates(ctx, c.tx, c.enc.ID)
			if err != nil {
				return nil, err
			}
			sheet, err := s.sheetOf(ctx, c.tx, c.session.CampaignID, reactor)
			if err != nil {
				return nil, err
			}
			creature := creatureFacts(reactor, states, sheet.Traits)
			sources := combat.SaveMode(combat.SaveScene{Creature: creature, Ability: ability, EffectVisible: true})
			if t.OnDamage { // Riso Histérico: advantage on the save the damage asks (SRD)
				sources = append(sources, combat.Source{Kind: combat.SourceEffectSave, Effect: combat.ModeAdvantage})
			}
			mode := combat.Resolve(sources)
			save, err := s.saveOf(ctx, c.tx, c.session.CampaignID, reactor, ability)
			if err != nil {
				return nil, err
			}
			result = &playv1.EffectSaveResult{Modifier: clamp32(save.Bonus, math.MinInt32, math.MaxInt32)}
			if combat.AutoFailsSave(creature, ability) {
				result.AutoFail, le.Change = true, "failed"
			} else {
				d20, err := s.d20With(in, save.Bonus, mode)
				if err != nil {
					return nil, err
				}
				extra, delta, err := s.rollEffectDice(in, effectDiceFor(states, reactor.ID, rules.RollAppliesSave), typedExtra)
				if err != nil {
					return nil, err
				}
				total := d20.Total + delta
				saved = combat.SaveSucceeded(total, int(dc))
				for _, f := range d20.Faces {
					result.D20Faces = append(result.D20Faces, clamp32(f, 1, 20))
				}
				result.D20, result.Total, result.Physical, result.Saved = clamp32(d20.Face(), 1, 20), clamp32(total, math.MinInt32, math.MaxInt32), d20.Physical, saved
				result.ExtraDice = s.extraDiceProto(extra, func(string) bool { return true }, s.namesFor(ctx, c.session.CampaignID))
				le.D20, le.Total = result.D20, result.Total
				ev.D20, ev.Modifier, ev.Total, ev.Physical = result.D20, result.Modifier, result.Total, d20.Physical
			}
			if !saved && le.Change != "failed" {
				le.Change = "failed"
			}
			if saved {
				le.Change = "saved"
			}
			result.Saved = saved
		}
		if master {
			result.Dc = &dc
		}
		if err := s.resolveEffectSave(ctx, c, cs, content, def, st, reactor, t.Phase, saved, skip, &result, le); err != nil {
			return nil, err
		}
		if _, err := s.closeWindow(ctx, c, w, windowAnswered, reaction.ReasonNone, out, ""); err != nil {
			return nil, err
		}
		ev.Lasting = le
		if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
			return nil, fmt.Errorf("touch the encounter: %w", err)
		}
		c.characterID = &reactor.CharacterID
		made = ev
		return ev, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "roll an effect's saving throw", err)
	}
	out, err := s.finish(ctx, m, res, func(ctx context.Context, d *encounterData) {
		s.publishEncounterChanged(ctx, m.CampaignID, d.enc)
		s.publishLogChanged(ctx, m.CampaignID, d.enc.ID, !made.Secret)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.RollEffectSaveResponse{Encounter: out, Result: result}), nil
}

// resolveEffectSave does what an answered saving throw does to its effect: a pass ends it on
// the target (Imobilizar Pessoa, Riso Histérico), a failed save at the start of the turn adds
// what the effect says (the Teia that restrains). A skipped save does nothing.
func (s *Service) resolveEffectSave(ctx context.Context, c *combatTx, cs []playdb.Combatant, content *rules.Content, def *rules.EffectDef, st playdb.CombatantState, target playdb.Combatant, phase string, saved, skipped bool, result **playv1.EffectSaveResult, le *lastingEvent) error {
	if skipped || def == nil {
		return nil
	}
	save := def.EndSave
	if phase == combat.PhaseStart {
		save = def.StartSave
	}
	if save == nil {
		return nil
	}
	r := *result
	switch {
	case saved && save.OnPass == "end":
		r.EffectEnded = true
		r.TextPt = def.NamePT + " acabou."
		if def.NamePT == "" {
			r.TextPt = "O efeito acabou."
		}
		return s.endEffectRows(ctx, c, cs, []playdb.CombatantState{st}, endSaved)
	case !saved && save.OnFail != "":
		next, ok := content.CombatEffect(save.OnFail)
		if !ok {
			return nil
		}
		var caster *playdb.Combatant
		if j := slices.IndexFunc(cs, func(o playdb.Combatant) bool { return o.ID == deref(st.SourceID) }); j >= 0 {
			caster = &cs[j]
		}
		made, err := s.addEffects(ctx, c, cs, effectSpec{
			key: save.OnFail, sourceKind: "spell", def: next, caster: caster, group: deref(st.GroupID), targets: []playdb.Combatant{target},
			dur: durationSpec{Kind: next.Duration.Kind}, concentration: st.Concentration, dc: st.SaveDc,
		})
		if err != nil {
			return err
		}
		return s.addedEvent(ctx, c, cs, made, save.OnFail)
	}
	return nil
}
