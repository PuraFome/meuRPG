package play

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
	"uuid"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/idem"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// The master's calls on the effects that last (RN-22): the table of "Efeitos em jogo", "Adicionar
// efeito", "Mudar a duração", "Os jogadores veem este efeito", "Encerrar" and "Tirar de um alvo".
// Every one is the master's (a player gets not_found for anything of it, RN-10) and every write
// takes an idempotency key.

// The limits of what the master writes.
const (
	maxEffectTargets = 10
	maxEffectRounds  = 600
	maxEffectLabel   = 30
	// looseConditionPrefix is the id of a condition the master marked by hand, which the table lists
	// as an effect with no origin and no end: "condition:<combatant id>:<condition key>".
	looseConditionPrefix = "loose:"
)

// ListLastingEffects implements playv1connect.LastingEffectServiceHandler.
func (s *Service) ListLastingEffects(
	ctx context.Context,
	req *connect.Request[playv1.ListLastingEffectsRequest],
) (*connect.Response[playv1.ListLastingEffectsResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	// Without a combat only the catalog comes: what the master gives a character between fights.
	if req.Msg.GetEncounterId() == "" {
		content, err := s.roster.RulesContent(ctx, nil, m.CampaignID)
		if err != nil {
			return nil, s.dbError(ctx, "read the rules", err)
		}
		ev := &effectViewer{s: s, m: m, v: combatViewer{master: true}, content: content, names: s.namesFor(ctx, m.CampaignID)}
		return connect.NewResponse(&playv1.ListLastingEffectsResponse{Catalog: ev.catalog()}), nil
	}
	encID, err := parseCombatID(req.Msg.GetEncounterId(), "encounter")
	if err != nil {
		return nil, err
	}
	_, d, err := s.readEncounter(ctx, m.CampaignID, encID)
	if err != nil {
		return nil, err
	}
	names := s.namesFor(ctx, m.CampaignID)
	ev, err := s.effectViewerFor(ctx, m, d, combatViewer{master: true}, names)
	if err != nil {
		return nil, err
	}
	res := &playv1.ListLastingEffectsResponse{Round: d.enc.Round, CurrentCombatantId: deref(d.enc.CurrentCombatantID)}
	groups := groupsOf(d.states)
	for _, g := range groups {
		res.Effects = append(res.Effects, ev.card(g, ""))
	}
	res.Effects = append(res.Effects, ev.looseConditions()...)
	res.TurnClock = ev.turnClock(groups)
	res.Concentrations = ev.concentrations(groups)
	res.Catalog = ev.catalog()
	return connect.NewResponse(res), nil
}

// looseConditions lists the conditions the master marked by hand as effects with no origin and
// no end.
func (ev *effectViewer) looseConditions() []*playv1.LastingEffect {
	var out []*playv1.LastingEffect
	for _, c := range ev.d.cs {
		for _, k := range c.Conditions {
			if slices.Contains(c.EffectConditions, k) || k == conditionExhaustion {
				continue
			}
			info, _ := ev.content.ConditionInfo(k)
			out = append(out, &playv1.LastingEffect{
				Id: looseConditionPrefix + c.ID + ":" + k, EncounterId: ev.d.enc.ID, TargetIds: []string{c.ID}, TargetLabels: []string{c.Label},
				SourceKind: playv1.EffectSourceKind_EFFECT_SOURCE_KIND_CONDITION, SourceKey: k, SourceNamePt: ev.names(k), OriginPt: "Do mestre",
				ConditionKeys: []string{k}, ConditionNamesPt: []string{ev.names(k)}, DurationKind: playv1.EffectDurationKind_EFFECT_DURATION_KIND_UNTIL_DISMISSED,
				ClockTextPt: "Dura até o mestre encerrar.", EndTextPt: "Até o mestre encerrar.", PlayerVisible: info.Visibility == rules.EffectVisibilityPublic,
				Audience: audienceProto(map[bool]string{true: "all", false: "owner"}[info.Visibility == rules.EffectVisibilityPublic]), ChangesPt: info.ChangesPT,
				PlayersSeePt: map[bool]string{true: ev.names(k), false: ""}[info.Visibility == rules.EffectVisibilityPublic],
			})
		}
	}
	return out
}

// turnClock is what the clock does from the turn that is running on, in the order of the
// initiative: the saving throws and the effects that end, over the next two rounds.
func (ev *effectViewer) turnClock(groups []effectGroup) []*playv1.TurnClockEntry {
	type keyed struct {
		round int32
		idx   int
		phase int
		e     *playv1.TurnClockEntry
	}
	var all []keyed
	order := func(id string) int {
		return slices.IndexFunc(ev.d.cs, func(c playdb.Combatant) bool { return c.ID == id })
	}
	round := max(ev.d.enc.Round, 1)
	add := func(r int32, who, text string, phase string, save bool, effect string) {
		i := order(who)
		if i < 0 {
			return
		}
		p := 0
		if phase == combat.PhaseEnd {
			p = 1
		}
		all = append(all, keyed{r, i, p, &playv1.TurnClockEntry{
			Round: r, CombatantId: who, CombatantLabel: ev.byID[who].Label, Phase: phaseProto(phase), IsSave: save, EffectId: effect, TextPt: text,
		}})
	}
	for _, g := range groups {
		f := g.first()
		name := ev.names(deref(f.SourceKey))
		casterLabel := ev.byID[deref(f.SourceID)].Label
		var targets []string
		for _, st := range g.rows {
			targets = append(targets, ev.byID[st.CombatantID].Label)
		}
		if e := endsAtOf(f); e.Timed() && e.CombatantID != "" {
			phase := e.Phase
			text := fmt.Sprintf("%s em %s acaba.", name, strings.Join(targets, ", "))
			if len(targets) == 1 && f.SourceID == nil {
				text = name + " acaba."
			}
			add(int32(e.Round), e.CombatantID, text, phase, false, f.ID) //nolint:gosec // a round
		}
		for _, st := range g.rows {
			dc := derefInt32(st.SaveDc)
			if dc == 0 {
				dc = defaultEffectDC
			}
			from := " contra " + name
			if casterLabel != "" {
				from += ", de " + casterLabel
			}
			if st.EndSaveAbility != nil {
				add(nextTurnRound(ev.d.enc, ev.d.cs, st.CombatantID), st.CombatantID, fmt.Sprintf("Teste de %s (CD %d)%s.", ev.names(*st.EndSaveAbility), dc, from), combat.PhaseEnd, true, st.ID)
			}
			if st.StartSaveAbility != nil {
				add(nextTurnRound(ev.d.enc, ev.d.cs, st.CombatantID), st.CombatantID, fmt.Sprintf("Teste de %s (CD %d)%s.", ev.names(*st.StartSaveAbility), dc, from), combat.PhaseStart, true, st.ID)
			}
		}
	}
	slices.SortStableFunc(all, func(a, b keyed) int {
		switch {
		case a.round != b.round:
			return int(a.round - b.round)
		case a.idx != b.idx:
			return a.idx - b.idx
		}
		return a.phase - b.phase
	})
	var out []*playv1.TurnClockEntry
	for _, k := range all {
		if k.round >= round {
			out = append(out, k.e)
		}
	}
	return out
}

// concentrations lists the casters that concentrate on an effect and the effects they hold.
func (ev *effectViewer) concentrations(groups []effectGroup) []*playv1.ConcentrationEntry {
	var out []*playv1.ConcentrationEntry
	for _, g := range groups {
		f := g.first()
		if !f.Concentration || f.SourceID == nil {
			continue
		}
		i := slices.IndexFunc(out, func(e *playv1.ConcentrationEntry) bool { return e.CasterId == *f.SourceID })
		if i < 0 {
			caster := ev.byID[*f.SourceID]
			out = append(out, &playv1.ConcentrationEntry{CasterId: caster.ID, CasterLabel: caster.Label, SpellKey: deref(caster.ConcentrationSpell), SpellNamePt: ev.names(deref(caster.ConcentrationSpell))})
			i = len(out) - 1
		}
		for _, st := range g.rows {
			out[i].EffectIds = append(out[i].EffectIds, st.ID)
		}
	}
	return out
}

// catalog lists what the master may add: the spells that last, the 14 conditions (exhaustion has
// its own call) and what the app adds itself (the web that burns).
func (ev *effectViewer) catalog() []*playv1.CatalogEffect {
	var out []*playv1.CatalogEffect
	for _, key := range []string{"spell:hold-person", "spell:bless", "spell:bane", "spell:haste", "spell:web", "spell:faerie-fire", "spell:hideous-laughter", "spell:guidance", "spell:resistance", "spell:enhance-ability", "spell:heroism", "spell:longstrider", "spell:mage-armor", "spell:pass-without-trace", "spell:shield-of-faith"} {
		def, ok := ev.content.CombatSpellEffect(key)
		if !ok {
			continue
		}
		e := &playv1.CatalogEffect{Key: key, NamePt: ev.names(key), SourceKind: playv1.EffectSourceKind_EFFECT_SOURCE_KIND_SPELL, Concentration: def.Concentration, HasCaster: true}
		e.DefaultDurationKind, e.DefaultRounds = playv1.EffectDurationKind_EFFECT_DURATION_KIND_UNTIL_DISMISSED, 0
		if r := ev.content.SpellEffectRounds(key); r > 0 {
			e.DefaultDurationKind, e.DefaultRounds = playv1.EffectDurationKind_EFFECT_DURATION_KIND_ROUNDS, clamp32(r, 0, maxEffectRounds)
		} else if def.Concentration {
			e.DefaultDurationKind = playv1.EffectDurationKind_EFFECT_DURATION_KIND_CONCENTRATION
		}
		out = append(out, e)
	}
	for _, key := range ev.content.CombatEffectKeys() {
		def, _ := ev.content.CombatEffect(key)
		if def.Kind != "master" {
			continue
		}
		out = append(out, &playv1.CatalogEffect{
			Key: key, NamePt: ev.names(key), SourceKind: playv1.EffectSourceKind_EFFECT_SOURCE_KIND_MASTER,
			DefaultDurationKind: durationKindProto(def.Duration.Kind), DefaultRounds: clamp32(def.Duration.Rounds, 0, maxEffectRounds),
		})
	}
	var conds []string
	conds = append(conds, ev.content.ConditionKeys()...)
	slices.SortFunc(conds, func(a, b string) int { return strings.Compare(ev.names(a), ev.names(b)) })
	for _, k := range conds {
		if k == "condition:exhaustion" {
			continue
		}
		out = append(out, &playv1.CatalogEffect{Key: k, NamePt: ev.names(k), SourceKind: playv1.EffectSourceKind_EFFECT_SOURCE_KIND_CONDITION, DefaultDurationKind: playv1.EffectDurationKind_EFFECT_DURATION_KIND_UNTIL_DISMISSED})
	}
	return out
}

// lastingDurationOf reads the duration the master picked.
func lastingDurationOf(ch *playv1.EffectDurationChoice, fallback durationSpec) (durationSpec, error) {
	if ch == nil || ch.GetKind() == playv1.EffectDurationKind_EFFECT_DURATION_KIND_UNSPECIFIED {
		return fallback, nil
	}
	d := durationSpec{Kind: durationKindOf(ch.GetKind()), Rounds: ch.GetRounds()}
	if raw := ch.GetAnchorCombatantId(); raw != "" {
		id, err := parseCombatID(raw, "combatant")
		if err != nil {
			return d, err
		}
		d.Anchor = id
	}
	bad := func(text string) error { return connect.NewError(connect.CodeInvalidArgument, errors.New(text)) }
	switch d.Kind {
	case rules.EffectDurationRounds:
		if d.Rounds < 1 || d.Rounds > maxEffectRounds {
			return d, bad("duration.rounds must be 1 to 600")
		}
	case rules.EffectDurationUntilStartOfTurnOf, rules.EffectDurationUntilEndOfTurnOf:
		if d.Anchor == "" {
			return d, bad("duration.anchor_combatant_id is the combatant whose turn ends it")
		}
		d.Rounds = 0
	default:
		d.Rounds, d.Anchor = 0, ""
	}
	return d, nil
}

// lastingInput is what AddLastingEffect reads from its request before it opens the transaction:
// the key, the combat, and the combatants the effect is about.
type lastingInput struct {
	key         string
	encounterID string
	targetIDs   []string
	casterID    string
}

// The limits of the saving throw's difficulty class the master may set.
const (
	minEffectDC = 1
	maxEffectDC = 40
)

// parseLastingInput checks the request: the key, the combat, the targets and the caster, the
// difficulty class and the label.
func parseLastingInput(msg *playv1.AddLastingEffectRequest) (lastingInput, error) {
	var in lastingInput
	key, err := parseKey(msg.GetIdempotencyKey())
	if err != nil {
		return in, err
	}
	in.key = key
	if in.encounterID, err = parseCombatID(msg.GetEncounterId(), "encounter"); err != nil {
		return in, err
	}
	if in.targetIDs, err = parseTargetIDs(msg.GetTargetIds()); err != nil {
		return in, err
	}
	if raw := msg.GetCasterId(); raw != "" {
		if in.casterID, err = parseCombatID(raw, "combatant"); err != nil {
			return in, err
		}
	}
	if dc := msg.SaveDc; dc != nil && (*dc < minEffectDC || *dc > maxEffectDC) {
		return in, errInvalidEffect("save_dc must be 1 to 40")
	}
	return in, checkEffectLabel(msg.GetPlayerLabel())
}

// parseTargetIDs reads the targets: one to ten combatants, none twice.
func parseTargetIDs(raw []string) ([]string, error) {
	if n := len(raw); n < 1 || n > maxEffectTargets {
		return nil, errInvalidEffect("target_ids must have 1 to 10 combatants")
	}
	var ids []string
	for _, r := range raw {
		id, err := parseCombatID(r, "combatant")
		if err != nil {
			return nil, err
		}
		if slices.Contains(ids, id) {
			return nil, errInvalidEffect("target_ids repeats a combatant")
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// AddLastingEffect implements playv1connect.LastingEffectServiceHandler.
func (s *Service) AddLastingEffect(
	ctx context.Context,
	req *connect.Request[playv1.AddLastingEffectRequest],
) (*connect.Response[playv1.AddLastingEffectResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	in, err := parseLastingInput(req.Msg)
	if err != nil {
		return nil, err
	}
	var made []playdb.CombatantState
	var secret bool
	res, err := s.write(ctx, combatWrite{m: m, key: in.key, hash: idem.Hash(req.Msg), kind: eventLastingAdded, encounterID: in.encounterID}, func(c *combatTx) (any, error) {
		made = nil
		rows, ev, err := s.addLastingEffectTx(ctx, c, m, in, req.Msg)
		if err != nil {
			return nil, err
		}
		made, secret = rows, ev.Secret
		return ev, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "add an effect", err)
	}
	out, err := s.finish(ctx, m, res, func(ctx context.Context, d *encounterData) {
		s.publishEncounterChanged(ctx, m.CampaignID, d.enc)
		s.publishLogChanged(ctx, m.CampaignID, d.enc.ID, !secret)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.AddLastingEffectResponse{Encounter: out, Effects: s.addedCards(ctx, m, res, made)}), nil
}

// addLastingEffectTx is the change AddLastingEffect makes inside the combat's transaction: read
// and validate (the combat, the combatants), resolve the effect, write it and the concentration,
// and answer the effect it made and the event that tells it.
func (s *Service) addLastingEffectTx(ctx context.Context, c *combatTx, m authz.Membership, in lastingInput, msg *playv1.AddLastingEffectRequest) ([]playdb.CombatantState, actionEvent, error) {
	var none actionEvent
	if err := notEnded(c.enc); err != nil {
		return nil, none, err
	}
	cs, err := c.q.ListCombatants(ctx, c.enc.ID)
	if err != nil {
		return nil, none, fmt.Errorf("list the combatants: %w", err)
	}
	targets, caster, err := lastingPartiesOf(cs, in, viewerOf(m))
	if err != nil {
		return nil, none, err
	}
	content, err := s.contentOf(ctx, c)
	if err != nil {
		return nil, none, err
	}
	spec, opts, err := s.planLastingEffect(ctx, c, content, cs, msg, targets, caster)
	if err != nil {
		return nil, none, err
	}
	if err := s.holdConcentration(ctx, c, spec); err != nil {
		return nil, none, err
	}
	rows, err := s.putLastingEffects(ctx, c, cs, spec, opts)
	if err != nil {
		return nil, none, err
	}
	ev := addedPayload(c, rows, spec.key)
	c.characterID = &targets[0].CharacterID
	if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
		return nil, none, fmt.Errorf("touch the encounter: %w", err)
	}
	return rows, ev, nil
}

// lastingPartiesOf finds the targets and the caster among the combatants, as the viewer sees them.
func lastingPartiesOf(cs []playdb.Combatant, in lastingInput, v combatViewer) ([]playdb.Combatant, *playdb.Combatant, error) {
	var targets []playdb.Combatant
	for _, id := range in.targetIDs {
		t, err := findCombatant(cs, id, v)
		if err != nil {
			return nil, nil, err
		}
		targets = append(targets, t)
	}
	if in.casterID == "" {
		return targets, nil, nil
	}
	cb, err := findCombatant(cs, in.casterID, v)
	if err != nil {
		return nil, nil, err
	}
	return targets, &cb, nil
}

// lastingSource is what the catalog says about an effect: how long it lasts when the master says
// nothing, and what the cast decided about its modifiers.
type lastingSource struct {
	fallback durationSpec
	opts     castOpts
}

// planLastingEffect resolves the request into the spec of the effect: the catalog entry, the
// duration, the concentration, who reads it and the label.
func (s *Service) planLastingEffect(ctx context.Context, c *combatTx, content *rules.Content, cs []playdb.Combatant, msg *playv1.AddLastingEffectRequest, targets []playdb.Combatant, caster *playdb.Combatant) (effectSpec, castOpts, error) {
	catalogKey := msg.GetCatalogKey()
	spec := effectSpec{key: catalogKey, caster: caster, targets: targets, dc: msg.SaveDc, audience: ""}
	var src lastingSource
	var err error
	switch catalogKindOf(catalogKey) {
	case catalogCondition:
		src, err = lastingConditionSource(content, &spec)
	case catalogSpell:
		src, err = s.lastingSpellSource(ctx, c, content, &spec, msg.GetAbilityKey())
	default:
		src, err = lastingMasterSource(content, &spec)
	}
	if err != nil {
		return spec, src.opts, err
	}
	err = applyLastingChoices(&spec, cs, msg, src.fallback)
	return spec, src.opts, err
}

// lastingConditionSource is a condition: until dismissed, read as the condition says.
func lastingConditionSource(content *rules.Content, spec *effectSpec) (lastingSource, error) {
	info, err := conditionOfCatalog(content, spec.key)
	if err != nil {
		return lastingSource{}, err
	}
	spec.sourceKind, spec.conditions = "master", []string{spec.key}
	spec.audience = info.Visibility
	return lastingSource{fallback: durationSpec{Kind: rules.EffectDurationUntilDismissed}}, nil
}

// lastingSpellSource is a spell's effect: its modifiers worked out from the caster, and the spell's
// own duration.
func (s *Service) lastingSpellSource(ctx context.Context, c *combatTx, content *rules.Content, spec *effectSpec, abilityKey string) (lastingSource, error) {
	src := lastingSource{fallback: durationSpec{Kind: rules.EffectDurationUntilDismissed}}
	def, err := spellEffectOfCatalog(content, spec.key)
	if err != nil {
		return src, err
	}
	spec.sourceKind, spec.def, spec.concentration = "spell", def, def.Concentration && spec.caster != nil
	ability, err := s.abilityChoiceOf(ctx, c, spec.key, abilityKey)
	if err != nil {
		return src, err
	}
	casterCharacter := ""
	if spec.caster != nil {
		casterCharacter = spec.caster.CharacterID
	}
	if src.opts, err = s.castOptsOf(ctx, c, def, ability, casterCharacter); err != nil {
		return src, err
	}
	if spec.modifiers, _, err = s.modifiersFor(ctx, c, def, "", src.opts); err != nil {
		return src, err
	}
	if r := content.SpellEffectRounds(spec.key); r > 0 {
		src.fallback = durationSpec{Kind: rules.EffectDurationRounds, Rounds: int32(r)} //nolint:gosec // 10 rounds a minute
	} else if def.Concentration {
		src.fallback = durationSpec{Kind: rules.EffectDurationConcentration}
	}
	return src, nil
}

// lastingMasterSource is an effect of the master's own, with the duration the catalog gives it.
func lastingMasterSource(content *rules.Content, spec *effectSpec) (lastingSource, error) {
	def, err := masterEffectOfCatalog(content, spec.key)
	if err != nil {
		return lastingSource{}, err
	}
	spec.sourceKind, spec.def = "master", def
	return lastingSource{fallback: durationSpec{Kind: def.Duration.Kind, Rounds: int32(def.Duration.Rounds)}}, nil //nolint:gosec // a small number of the file
}

// applyLastingChoices puts what the master chose on the spec: the duration (checked against the
// combatants), the concentration, who sees the effect and the label.
func applyLastingChoices(spec *effectSpec, cs []playdb.Combatant, msg *playv1.AddLastingEffectRequest, fallback durationSpec) error {
	var err error
	if spec.dur, err = lastingDurationOf(msg.GetDuration(), fallback); err != nil {
		return err
	}
	if spec.dur.Anchor != "" && !slices.ContainsFunc(cs, func(o playdb.Combatant) bool { return o.ID == spec.dur.Anchor }) {
		return errCombatantNotFound()
	}
	if msg.Concentration != nil && spec.caster != nil && spec.def != nil && spec.sourceKind == "spell" {
		spec.concentration = msg.GetConcentration()
	}
	if v := msg.PlayerVisible; v != nil {
		spec.visible = v
	}
	spec.audience = chosenAudience(msg.GetAudience(), spec.audience)
	spec.label = msg.GetPlayerLabel()
	return nil
}

// holdConcentration makes the effect the caster's concentration. One concentration for each
// caster: the new spell ends the old one and what it held.
func (s *Service) holdConcentration(ctx context.Context, c *combatTx, spec effectSpec) error {
	caster := spec.caster
	if !spec.concentration || caster == nil {
		return nil
	}
	if caster.ConcentrationSpell != nil && *caster.ConcentrationSpell != spec.key {
		stop := actionEvent{Round: c.enc.Round, Secret: caster.Hidden, Actor: caster.ID}
		if err := s.stopConcentrating(ctx, c, *caster, &stop); err != nil {
			return err
		}
		if err := insertEvent(ctx, c, eventConditionsSet, &c.actorUserID, nil, stop); err != nil {
			return err
		}
	}
	if err := c.q.SetCombatantConcentration(ctx, playdb.SetCombatantConcentrationParams{ID: caster.ID, ConcentrationSpell: &spec.key}); err != nil {
		return fmt.Errorf("set the concentration: %w", err)
	}
	return nil
}

// putLastingEffects writes the effect on the targets. An effect that sets a base armor class is a
// record for each target, with the class worked out for it, and none for a target that wears armor.
func (s *Service) putLastingEffects(ctx context.Context, c *combatTx, cs []playdb.Combatant, spec effectSpec, opts castOpts) ([]playdb.CombatantState, error) {
	if !hasBaseAC(spec.def) {
		return s.addEffects(ctx, c, cs, spec)
	}
	// The base armor class is the target's own (13 + its Dexterity).
	var rows []playdb.CombatantState
	spec.group = uuid.New().String()
	for _, t := range spec.targets {
		mods, ok, err := s.modifiersForTarget(ctx, c, spec.def, t, opts)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		one := spec
		one.targets, one.modifiers = []playdb.Combatant{t}, mods
		made, err := s.addEffects(ctx, c, cs, one)
		if err != nil {
			return nil, err
		}
		rows = append(rows, made...)
	}
	return rows, nil
}

// addedCards are the cards of the effects the call made, as the master reads them; read after the
// commit, and none when the read fails or the call made nothing.
func (s *Service) addedCards(ctx context.Context, m authz.Membership, res combatResult, made []playdb.CombatantState) []*playv1.LastingEffect {
	if len(made) == 0 {
		return nil
	}
	d, err := s.encounterDataOf(ctx, res)
	if err != nil {
		return nil
	}
	ev, err := s.effectViewerFor(ctx, m, d, combatViewer{master: true}, s.namesFor(ctx, m.CampaignID))
	if err != nil {
		return nil
	}
	var cards []*playv1.LastingEffect
	for _, g := range groupsOf(d.states) {
		if slices.ContainsFunc(g.rows, func(st playdb.CombatantState) bool {
			return slices.ContainsFunc(made, func(o playdb.CombatantState) bool { return o.ID == st.ID })
		}) {
			cards = append(cards, ev.card(g, ""))
		}
	}
	return cards
}

// addedPayload is the event of a casting of an effect that began: one line for it.
func addedPayload(c *combatTx, rows []playdb.CombatantState, key string) actionEvent {
	var targets, ids []string
	for _, st := range rows {
		targets, ids = append(targets, st.CombatantID), append(ids, st.ID)
	}
	ev := actionEvent{Round: c.enc.Round}
	if len(rows) == 0 {
		return ev
	}
	first := rows[0]
	var rounds int32
	if first.EndsRound != nil {
		rounds = max(*first.EndsRound-max(c.enc.Round, 1), 0)
	}
	ev.Actor, ev.Target = deref(first.SourceID), targets[0]
	ev.Secret = !first.PlayerVisible || first.Audience == "owner"
	ev.Lasting = &lastingEvent{Key: key, Change: "added", Targets: targets, Caster: deref(first.SourceID), Rounds: rounds, Effects: ids}
	return ev
}

// encounterDataOf reads the combat a change was about, after the commit.
func (s *Service) encounterDataOf(ctx context.Context, res combatResult) (*encounterData, error) {
	pctx, stop := afterCommit(ctx)
	defer stop()
	enc, err := s.queries.GetEncounterInSession(pctx, playdb.GetEncounterInSessionParams{GameSessionID: res.session.ID, ID: res.encounterID})
	if err != nil {
		return nil, err
	}
	return loadEncounter(pctx, s.queries, enc)
}

// groupRowsOf are the rows of the casting an effect id belongs to: every target of it. A row that
// is not in the combat is not found, the same for one that never was.
func groupRowsOf(rows []playdb.CombatantState, id string) ([]playdb.CombatantState, error) {
	i := slices.IndexFunc(rows, func(st playdb.CombatantState) bool { return st.ID == id })
	if i < 0 {
		return nil, errEffectNotFound()
	}
	f := rows[i]
	return slices.DeleteFunc(slices.Clone(rows), func(st playdb.CombatantState) bool {
		return deref(st.GroupID) != deref(f.GroupID) || deref(st.SourceKey) != deref(f.SourceKey)
	}), nil
}

// ChangeLastingEffectDuration implements playv1connect.LastingEffectServiceHandler.
func (s *Service) ChangeLastingEffectDuration(
	ctx context.Context,
	req *connect.Request[playv1.ChangeLastingEffectDurationRequest],
) (*connect.Response[playv1.ChangeLastingEffectDurationResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
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
	effID, err := parseCombatID(req.Msg.GetEffectId(), "effect")
	if err != nil {
		return nil, errEffectNotFound()
	}
	var secret bool
	var changed []playdb.CombatantState
	res, err := s.write(ctx, combatWrite{m: m, key: key, hash: idem.Hash(req.Msg), kind: eventLastingChanged, encounterID: encID}, func(c *combatTx) (any, error) {
		changed = nil
		if err := notEnded(c.enc); err != nil {
			return nil, err
		}
		all, err := c.q.ListLastingEffects(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the effects: %w", err)
		}
		rows, err := groupRowsOf(all, effID)
		if err != nil {
			return nil, err
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		var caster *playdb.Combatant
		if i := slices.IndexFunc(cs, func(o playdb.Combatant) bool { return o.ID == deref(rows[0].SourceID) }); i >= 0 {
			caster = &cs[i]
		}
		dur, err := lastingDurationOf(req.Msg.GetDuration(), durationSpec{Kind: deref(rows[0].DurationKind)})
		if err != nil {
			return nil, err
		}
		if dur.Anchor != "" && !slices.ContainsFunc(cs, func(o playdb.Combatant) bool { return o.ID == dur.Anchor }) {
			return nil, errCombatantNotFound()
		}
		var targets []string
		for _, st := range rows {
			i := slices.IndexFunc(cs, func(o playdb.Combatant) bool { return o.ID == st.CombatantID })
			if i < 0 {
				continue
			}
			round, who, phase := endsOf(c.enc, cs, dur, caster, cs[i])
			row, err := c.q.SetLastingEffectEnds(ctx, playdb.SetLastingEffectEndsParams{ID: st.ID, DurationKind: &dur.Kind, EndsRound: round, EndsCombatantID: who, EndsPhase: phase})
			if err != nil {
				return nil, fmt.Errorf("change the duration: %w", err)
			}
			changed = append(changed, row)
			targets = append(targets, st.CombatantID)
		}
		first := changed[0]
		secret = s.effectHiddenFrom(first, targetsOf(cs, targets))
		var rounds int32
		if first.EndsRound != nil {
			rounds = max(*first.EndsRound-max(c.enc.Round, 1), 0)
		}
		c.characterID = &cs[0].CharacterID
		if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
			return nil, fmt.Errorf("touch the encounter: %w", err)
		}
		return actionEvent{
			Round: c.enc.Round, Secret: secret, Actor: deref(first.SourceID), Target: targets[0],
			Lasting: &lastingEvent{Key: deref(first.SourceKey), Change: "duration", Targets: targets, Rounds: rounds, Effects: []string{first.ID}},
		}, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "change an effect's duration", err)
	}
	out, err := s.finish(ctx, m, res, func(ctx context.Context, d *encounterData) {
		s.publishEncounterChanged(ctx, m.CampaignID, d.enc)
		s.publishLogChanged(ctx, m.CampaignID, d.enc.ID, !secret)
	})
	if err != nil {
		return nil, err
	}
	resp := &playv1.ChangeLastingEffectDurationResponse{Encounter: out}
	resp.Effect = s.cardAfter(ctx, m, res, effID)
	return connect.NewResponse(resp), nil
}

// cardAfter reads the effect a change was about, as the master reads it now; nil when it is gone.
func (s *Service) cardAfter(ctx context.Context, m authz.Membership, res combatResult, id string) *playv1.LastingEffect {
	d, err := s.encounterDataOf(ctx, res)
	if err != nil {
		return nil
	}
	ev, err := s.effectViewerFor(ctx, m, d, combatViewer{master: true}, s.namesFor(ctx, m.CampaignID))
	if err != nil {
		return nil
	}
	for _, g := range groupsOf(d.states) {
		if slices.ContainsFunc(g.rows, func(st playdb.CombatantState) bool { return st.ID == id }) {
			return ev.card(g, "")
		}
	}
	return nil
}

// SetLastingEffectVisibility implements playv1connect.LastingEffectServiceHandler.
func (s *Service) SetLastingEffectVisibility(
	ctx context.Context,
	req *connect.Request[playv1.SetLastingEffectVisibilityRequest],
) (*connect.Response[playv1.SetLastingEffectVisibilityResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
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
	effID, err := parseCombatID(req.Msg.GetEffectId(), "effect")
	if err != nil {
		return nil, errEffectNotFound()
	}
	label := strings.TrimSpace(req.Msg.GetPlayerLabel())
	if utf8.RuneCountInString(label) > maxEffectLabel {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("player_label must have at most 30 characters"))
	}
	audience := "all"
	if req.Msg.GetAudience() == playv1.EffectAudience_EFFECT_AUDIENCE_OWNER {
		audience = "owner"
	}
	res, err := s.write(ctx, combatWrite{m: m, key: key, hash: idem.Hash(req.Msg), kind: eventLastingVisibility, encounterID: encID}, func(c *combatTx) (any, error) {
		if err := notEnded(c.enc); err != nil {
			return nil, err
		}
		all, err := c.q.ListLastingEffects(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the effects: %w", err)
		}
		rows, err := groupRowsOf(all, effID)
		if err != nil {
			return nil, err
		}
		var lbl *string
		if label != "" {
			lbl = &label
		}
		for _, st := range rows {
			if _, err := c.q.SetLastingEffectVisibility(ctx, playdb.SetLastingEffectVisibilityParams{ID: st.ID, PlayerVisible: req.Msg.GetPlayerVisible(), Audience: audience, PlayerLabel: lbl}); err != nil {
				return nil, fmt.Errorf("set the visibility: %w", err)
			}
		}
		if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
			return nil, fmt.Errorf("touch the encounter: %w", err)
		}
		// A line for the master alone: a switch is nothing the table reads.
		return actionEvent{
			Round: c.enc.Round, Secret: true, Actor: deref(rows[0].SourceID), Target: rows[0].CombatantID,
			Lasting: &lastingEvent{Key: deref(rows[0].SourceKey), Change: "visibility", Effects: []string{rows[0].ID}},
		}, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "set an effect's visibility", err)
	}
	out, err := s.finish(ctx, m, res, s.changed(m.CampaignID))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.SetLastingEffectVisibilityResponse{Encounter: out, Effect: s.cardAfter(ctx, m, res, effID)}), nil
}

// EndLastingEffect implements playv1connect.LastingEffectServiceHandler.
func (s *Service) EndLastingEffect(
	ctx context.Context,
	req *connect.Request[playv1.EndLastingEffectRequest],
) (*connect.Response[playv1.EndLastingEffectResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return nil, err
	}
	key, err := parseKey(req.Msg.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	scope := req.Msg.GetScope()
	if scope != playv1.EffectEndScope_EFFECT_END_SCOPE_THIS && scope != playv1.EffectEndScope_EFFECT_END_SCOPE_CONCENTRATION_GROUP {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("scope must be THIS or CONCENTRATION_GROUP"))
	}
	if req.Msg.GetEncounterId() == "" {
		return s.endCharacterEffect(ctx, m, req.Msg, key)
	}
	encID, err := parseCombatID(req.Msg.GetEncounterId(), "encounter")
	if err != nil {
		return nil, err
	}
	rawID := req.Msg.GetEffectId()
	var ended int32
	var made []string
	res, err := s.write(ctx, combatWrite{m: m, key: key, hash: idem.Hash(req.Msg), kind: eventLastingEnded, encounterID: encID}, func(c *combatTx) (any, error) {
		ended, made = 0, nil
		if err := notEnded(c.enc); err != nil {
			return nil, err
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		if strings.HasPrefix(rawID, looseConditionPrefix) {
			return s.endLooseCondition(ctx, c, cs, rawID, &ended)
		}
		effID, err := parseCombatID(rawID, "effect")
		if err != nil {
			return nil, errEffectNotFound()
		}
		all, err := c.q.ListLastingEffects(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the effects: %w", err)
		}
		rows, err := groupRowsOf(all, effID)
		if err != nil {
			return nil, err
		}
		first := rows[0]
		reason := endByMaster
		if scope == playv1.EffectEndScope_EFFECT_END_SCOPE_CONCENTRATION_GROUP && first.Concentration && first.SourceID != nil {
			// The whole concentration: the caster stops concentrating and everything it held ends.
			rows = slices.DeleteFunc(all, func(st playdb.CombatantState) bool { return !st.Concentration || deref(st.SourceID) != *first.SourceID })
			if i := slices.IndexFunc(cs, func(o playdb.Combatant) bool { return o.ID == *first.SourceID && o.ConcentrationSpell != nil }); i >= 0 {
				if err := c.q.SetCombatantConcentration(ctx, playdb.SetCombatantConcentrationParams{ID: cs[i].ID}); err != nil {
					return nil, fmt.Errorf("end the concentration: %w", err)
				}
				stop := actionEvent{Round: c.enc.Round, Secret: cs[i].Hidden, Actor: cs[i].ID, ConcBefore: *cs[i].ConcentrationSpell, ConcEnded: *cs[i].ConcentrationSpell}
				if err := insertEvent(ctx, c, eventConditionsSet, &c.actorUserID, nil, stop); err != nil {
					return nil, err
				}
			}
			reason = endConcentration
		}
		before := len(rows)
		if err := s.endEffectRows(ctx, c, cs, rows, reason); err != nil {
			return nil, err
		}
		ended = clamp32(before, 0, 100)
		c.characterID = &cs[0].CharacterID
		if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
			return nil, fmt.Errorf("touch the encounter: %w", err)
		}
		return actionEvent{
			Round: c.enc.Round, Secret: s.effectHiddenFrom(first, targetsOf(cs, []string{first.CombatantID})), Actor: deref(first.SourceID), Target: first.CombatantID,
			Lasting: &lastingEvent{Key: deref(first.SourceKey), Change: "ended", Reason: reason, Effects: []string{first.ID}},
		}, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "end an effect", err)
	}
	out, err := s.finish(ctx, m, res, func(ctx context.Context, d *encounterData) {
		s.publishEncounterChanged(ctx, m.CampaignID, d.enc)
		s.publishLogChanged(ctx, m.CampaignID, d.enc.ID, false)
		s.publishTurnChanged(ctx, m.CampaignID, d)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.EndLastingEffectResponse{Encounter: out, Ended: ended, CreatedEffectIds: made}), nil
}

// looseConditionParts are the two parts of a loose condition's id: the combatant and the condition.
const looseConditionParts = 2

// endLooseCondition takes a condition the master marked by hand off a combatant: the id names it.
func (s *Service) endLooseCondition(ctx context.Context, c *combatTx, cs []playdb.Combatant, id string, ended *int32) (any, error) {
	parts := strings.SplitN(strings.TrimPrefix(id, looseConditionPrefix), ":", 2)
	if len(parts) != looseConditionParts {
		return nil, errEffectNotFound()
	}
	who, err := findCombatant(cs, parts[0], combatViewer{master: true})
	if err != nil {
		return nil, errEffectNotFound()
	}
	cond := parts[1]
	if !slices.Contains(who.Conditions, cond) || slices.Contains(who.EffectConditions, cond) {
		return nil, errEffectNotFound()
	}
	next := slices.DeleteFunc(slices.Clone(who.Conditions), func(k string) bool { return k == cond })
	if err := c.q.SetCombatantConditions(ctx, playdb.SetCombatantConditionsParams{ID: who.ID, Conditions: next}); err != nil {
		return nil, fmt.Errorf("take the condition off: %w", err)
	}
	*ended = 1
	c.characterID = &who.CharacterID
	var err2 error
	if c.enc, err2 = c.q.TouchEncounter(ctx, c.enc.ID); err2 != nil {
		return nil, fmt.Errorf("touch the encounter: %w", err2)
	}
	return actionEvent{Round: c.enc.Round, Secret: true, Actor: who.ID, CondSet: true, Conditions: next, CondBefore: who.Conditions}, nil
}

// RemoveEffectTarget implements playv1connect.LastingEffectServiceHandler.
func (s *Service) RemoveEffectTarget(
	ctx context.Context,
	req *connect.Request[playv1.RemoveEffectTargetRequest],
) (*connect.Response[playv1.RemoveEffectTargetResponse], error) {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
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
	effID, err := parseCombatID(req.Msg.GetEffectId(), "effect")
	if err != nil {
		return nil, errEffectNotFound()
	}
	combID, err := parseCombatID(req.Msg.GetCombatantId(), "combatant")
	if err != nil {
		return nil, errEffectNotFound()
	}
	res, err := s.write(ctx, combatWrite{m: m, key: key, hash: idem.Hash(req.Msg), kind: eventLastingEnded, encounterID: encID}, func(c *combatTx) (any, error) {
		if err := notEnded(c.enc); err != nil {
			return nil, err
		}
		all, err := c.q.ListLastingEffects(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the effects: %w", err)
		}
		rows, err := groupRowsOf(all, effID)
		if err != nil {
			return nil, err
		}
		i := slices.IndexFunc(rows, func(st playdb.CombatantState) bool { return st.CombatantID == combID })
		if i < 0 {
			return nil, errEffectNotFound()
		}
		cs, err := c.q.ListCombatants(ctx, c.enc.ID)
		if err != nil {
			return nil, fmt.Errorf("list the combatants: %w", err)
		}
		row := rows[i]
		if err := s.endEffectRows(ctx, c, cs, []playdb.CombatantState{row}, endByMaster); err != nil {
			return nil, err
		}
		c.characterID = &cs[0].CharacterID
		if c.enc, err = c.q.TouchEncounter(ctx, c.enc.ID); err != nil {
			return nil, fmt.Errorf("touch the encounter: %w", err)
		}
		return actionEvent{
			Round: c.enc.Round, Secret: s.effectHiddenFrom(row, targetsOf(cs, []string{combID})), Actor: deref(row.SourceID), Target: combID,
			Lasting: &lastingEvent{Key: deref(row.SourceKey), Change: "ended", Reason: endByMaster, Targets: []string{combID}, Effects: []string{row.ID}},
		}, nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "take a target off an effect", err)
	}
	out, err := s.finish(ctx, m, res, func(ctx context.Context, d *encounterData) {
		s.publishEncounterChanged(ctx, m.CampaignID, d.enc)
		s.publishLogChanged(ctx, m.CampaignID, d.enc.ID, false)
		s.publishTurnChanged(ctx, m.CampaignID, d)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&playv1.RemoveEffectTargetResponse{Encounter: out}), nil
}
