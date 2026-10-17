package play

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// These tests freeze what CastSpell does today, scenario by scenario: every refusal with its code and
// message, and for each cast that goes through the answer, the events it writes (in order, the combatants
// named by label), the combatants' economy and hit points, the damage and the effects it opens and the
// questions it holds. They are the net under the split of castSpell into phases.
// testdata/cast_spell.golden holds what each scenario reads; run with UPDATE_GOLDEN=1 to write the file
// again, and read the diff.

const castSpellGolden = "testdata/cast_spell.golden"

// spellJSON is a message as sorted JSON, with the ids replaced by labels.
func spellJSON(m proto.Message, names map[string]string, omit ...string) string {
	if m == nil || !m.ProtoReflect().IsValid() {
		return ""
	}
	c := proto.Clone(m)
	for _, f := range omit {
		if fd := c.ProtoReflect().Descriptor().Fields().ByName(protoreflect.Name(f)); fd != nil {
			c.ProtoReflect().Clear(fd)
		}
	}
	raw, err := protojson.Marshal(c)
	if err != nil {
		return err.Error()
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return err.Error()
	}
	out, _ := json.Marshal(v)
	return uuidPattern.ReplaceAllStringFunc(string(out), func(id string) string {
		if l, ok := names[id]; ok {
			return l
		}
		return "?"
	})
}

// castTimePattern finds the instants an event keeps, which differ from one run to the next.
var castTimePattern = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?Z`)

// castStateQueries are the tables a cast writes to or holds a question in, one query for each; the ids
// are left out and the labels of the combatants are joined in.
var castStateQueries = []struct{ name, query string }{
	{"combatant", `SELECT label, action_used, bonus_action_used, reaction_used, dashed, hidden, COALESCE(hp_current, -1), COALESCE(hp_temp, -1), hp_max_bonus, defeated,
		COALESCE(conditions::TEXT, ''), COALESCE(effect_conditions::TEXT, ''), COALESCE(concentration_spell, '-'), spell_cast, bonus_spell_cast, dismissed, COALESCE(slots_used::TEXT, ''),
		extra_action_used, action_attack_key IS NOT NULL FROM combatants ORDER BY label`},
	{"damage", `SELECT COALESCE(a.label, '-'), t.label, p.attack_key, p.status, p.critical, p.dice_count, p.dice_sides, p.dice_bonus, p.damage_type, p.half, p.healing,
		COALESCE(p.attack_total, 0), COALESCE(p.attack_armor_class, 0), p.cast_id IS NOT NULL, COALESCE(p.roll_total, 0), COALESCE(p.applied_amount, 0)
		FROM pending_damages p JOIN combatants t ON t.id = p.target_id LEFT JOIN combatants a ON a.id = p.attacker_id`},
	{"vitals", `SELECT b.label, v.hit_points_current, v.hit_points_temporary, v.spell_slots_used::TEXT, v.pact_slots_used FROM character_vitals v JOIN combatants b ON b.character_id = v.character_id`},
	{"state", `SELECT c.label, s.kind, COALESCE(s.source_key, '-'), s.concentration, COALESCE(s.condition_keys::TEXT, ''), COALESCE(s.duration_kind, '-'), COALESCE(s.save_dc, 0), s.amount
		FROM combatant_states s JOIN combatants c ON c.id = s.combatant_id`},
	{"effect", `SELECT b.label, e.source_key, e.concentration, COALESCE(e.condition_keys::TEXT, ''), e.duration_kind, COALESCE(e.seconds_left, 0) FROM character_effects e JOIN combatants b ON b.character_id = e.character_id`},
	{"window", `SELECT kind, status, COALESCE(closed_reason, '-'), step FROM reaction_windows`},
	{"hold", `SELECT kind, state, actor_is_master FROM reaction_holds`},
	{"roll hold", `SELECT face, modifier, round, answer_key IS NOT NULL, COALESCE(lucky_index, -1) FROM roll_holds`},
	{"inspiration hold", `SELECT kind, faces::TEXT, modifier, total, counted, physical, COALESCE(lucky_index, -1) FROM inspiration_holds`},
	{"reveal", `SELECT spell_key, array_length(combatant_ids, 1), state FROM hidden_reveals`},
	{"rider", `SELECT kind, round, COALESCE(choice, '-'), used FROM hit_riders`},
	{"cast", `SELECT spell_key, slot_level, status, concentrating, secret FROM spell_casts`},
}

// castReport is what a scenario read after its cast: the answer or the refusal, the events written since
// `after` and the state the cast left.
func (a *armed) castReport(t *testing.T, after int, res *playv1.CastSpellResponse, err error) string {
	t.Helper()
	e := a.get(t, a.master)
	names := labelsOf(e)
	var lines []string
	if err != nil {
		lines = append(lines, fmt.Sprintf("refused %v: %v", connect.CodeOf(err), err.Error()[strings.Index(err.Error(), ": ")+2:]))
	} else {
		lines = append(lines,
			"cast "+spellJSON(res.GetCast(), names, "cast_id"),
			"area "+spellJSON(res.GetArea(), names),
			"inspiration "+spellJSON(res.GetInspirationOffer(), names, "hold_id"),
			"lucky "+spellJSON(res.GetLuckyOffer(), names, "hold_id"),
			"summoned "+fmt.Sprint(len(res.GetSummonedCombatantIds())),
			"hidden hits "+spellJSON(&playv1.CastSpellResponse{HiddenHits: res.GetHiddenHits()}, names),
			"pending reveal "+fmt.Sprint(res.GetPendingRevealId() != ""),
			"held "+fmt.Sprint(res.GetCast() == nil && res.GetInspirationOffer() == nil && res.GetLuckyOffer() == nil),
		)
	}
	for _, l := range a.eventTrace(t, e, after) {
		lines = append(lines, "event "+castTimePattern.ReplaceAllString(l, "<time>"))
	}
	var state []string
	for _, q := range castStateQueries {
		for _, r := range a.rowsOf(t, q.query) {
			state = append(state, q.name+" "+r)
		}
	}
	lines = append(lines, a.printedState(t, err != nil, state)...)
	return strings.Join(lines, "\n")
}

// goldenScenario is one scenario of the golden: run plays it and reads what it left.
// castPlay is what a scenario does after its setup: the combat it ran on, the number of events written
// before the cast and the cast's answer.
type castPlay func(t *testing.T) (a *armed, before int, res *playv1.CastSpellResponse, err error)

// castOn is a scenario that plays its own fight.
func castOn(name string, play castPlay) goldenScenario {
	return goldenScenario{name: name, run: func(t *testing.T) string {
		t.Helper()
		a, before, res, err := play(t)
		return a.castReport(t, before, res, err)
	}}
}

// castCall calls CastSpell as u with a request that edit changes after it is built from the caster, the
// spell, the slot and the targets by label.
func (a *armed) castCall(t *testing.T, u *user, e *playv1.Encounter, caster, spell string, slot *playv1.SpellSlot, targets []*playv1.SpellTarget, edit func(*playv1.CastSpellRequest)) (*playv1.CastSpellResponse, error) {
	t.Helper()
	return a.castKey(t, u, e, caster, spell, slot, targets, edit, newKey())
}

// castStand puts each combatant on its square in the order of the labels, so that the events of the
// scenario read the same on every run (a.stand ranges over a map).
func (a *armed) castStand(t *testing.T, at map[string][2]int32) {
	t.Helper()
	labels := make([]string, 0, len(at))
	for label := range at {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	for _, label := range labels {
		a.put(t, label, at[label][0], at[label][1])
	}
}

// castRaw is castCall without the area a test would place for the spell.
func (a *armed) castRaw(t *testing.T, u *user, e *playv1.Encounter, caster, spell string, slot *playv1.SpellSlot, targets []*playv1.SpellTarget, edit func(*playv1.CastSpellRequest)) (*playv1.CastSpellResponse, error) {
	t.Helper()
	req := &playv1.CastSpellRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CasterId: a.id(t, caster), SpellKey: spell, Slot: slot, Targets: targets, IdempotencyKey: newKey(),
	}
	edit(req)
	res, err := u.combat.CastSpell(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// castFight is a scenario on the casters' fight with a goblin and the Capitao: edit changes the request of
// the cast, and the other fields say the spell, who casts it and as whom.
type castFight struct {
	as      func(a *armed) *user
	caster  string
	spell   string
	slot    *playv1.SpellSlot
	targets []string
	edit    func(*playv1.CastSpellRequest)
	// turn is who is on turn before the cast ("" keeps Pensantus).
	turn string
	// raw sends the request as edit leaves it, without the area a test would place.
	raw bool
	// before runs on the fight before the cast: queued dice, a changed sheet.
	before func(t *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter
}

func asAna(a *armed) *user { return a.ana }

func asBia(a *armed) *user { return a.bia }

func asCaio(a *armed) *user { return a.caio }

// scenario plays the fight and the cast.
func (f castFight) scenario(name string) goldenScenario {
	return castOn(name, func(t *testing.T) (*armed, int, *playv1.CastSpellResponse, error) {
		t.Helper()
		a := newCasters(t)
		e := a.castersFight(t, 1)
		if f.turn != "" {
			e = a.passTo(t, e, f.turn)
		}
		if f.before != nil {
			e = f.before(t, a, e)
		}
		edit := f.edit
		if edit == nil {
			edit = noCastRoll
		}
		before := a.markCall(t)
		if f.raw {
			res, err := a.castRaw(t, f.as(a), e, f.caster, f.spell, f.slot, a.at(t, f.targets...), edit)
			return a, before, res, err
		}
		res, err := a.castCall(t, f.as(a), e, f.caster, f.spell, f.slot, a.at(t, f.targets...), edit)
		return a, before, res, err
	})
}

// castRefused is a scenario of a cast the checks refuse, on the casters' fight; it names what is refused.
func castRefused(name string, f castFight) goldenScenario { return f.scenario("refused " + name) }

// pensantus is the usual caster of the golden: Pensantus (Ana) casts the spell at the goblin.
func pensantus(spell string, slot *playv1.SpellSlot, edit func(*playv1.CastSpellRequest)) castFight {
	return castFight{as: asAna, caster: "Pensantus", spell: spell, slot: slot, targets: []string{"Goblin"}, edit: edit}
}

// TestCastSpellIsFrozen runs every scenario on a fight of its own and compares what each reads with the
// golden.
func TestCastSpellIsFrozen(t *testing.T) { //nolint:tparallel // runGolden waits for its parallel scenarios, then compares the golden
	t.Parallel()
	runGolden(t, castSpellGolden, castSpellScenarios())
}

// castSpellScenarios are the scenarios of the golden, in the order they are listed.
func castSpellScenarios() []goldenScenario {
	var out []goldenScenario
	out = append(out, castRequestScenarios()...)
	out = append(out, castRefusalScenarios()...)
	out = append(out, castThroughScenarios()...)
	out = append(out, castWindowScenarios()...)
	out = append(out, castSorcererScenarios()...)
	out = append(out, castHeldScenarios()...)
	out = append(out, castSummonScenarios()...)
	out = append(out, castOtherScenarios()...)
	out = append(out, castCaveScenarios()...)
	out = append(out, castEffectScenarios()...)
	return out
}

// brisa is the cleric of the golden: Brisa (Bia) casts the spell at the goblin on her own turn.
func brisa(spell string, slot *playv1.SpellSlot, edit func(*playv1.CastSpellRequest)) castFight {
	return castFight{as: asBia, caster: "Brisa", turn: "Brisa", spell: spell, slot: slot, targets: []string{"Goblin"}, edit: edit}
}

func (f castFight) to(targets ...string) castFight {
	f.targets = targets
	return f
}

func (f castFight) by(as func(a *armed) *user) castFight {
	f.as = as
	return f
}

func (f castFight) first(before func(t *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter) castFight {
	f.before = before
	return f
}

// queued is a fight step that queues the dice the cast will roll.
func queued(faces ...int) func(t *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter {
	return func(_ *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter {
		a.h.roller.queue(faces...)
		return e
	}
}

// hurtTo sets the hit points of a player's character before the cast.
func hurtTo(label string, hp int32) func(t *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter {
	return func(t *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter {
		t.Helper()
		c := map[string]*charactersv1.Character{"Toren": a.toren, "Pensantus": a.pens, "Brisa": a.bri}[label]
		a.correct(t, c, hpIs(hp))
		return e
	}
}

// castFirst casts a spell as Pensantus (a request that has to go through) before the scenario's own.
func castFirst(spell string, slot *playv1.SpellSlot, roll func(*playv1.CastSpellRequest), targets ...string) func(t *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter {
	return func(t *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter {
		t.Helper()
		if roll == nil {
			roll = noCastRoll
		}
		a.mustCast(t, a.ana, e, "Pensantus", spell, slot, a.at(t, targets...), roll)
		return a.get(t, a.master)
	}
}

// defeat takes a combatant to 0 hit points with the master's correction.
func defeat(label string) func(t *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter {
	return func(t *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter {
		t.Helper()
		if _, err := a.adjustHP(t, e, label, func(r *playv1.AdjustCombatantHitPointsRequest) {
			r.Change = &playv1.AdjustCombatantHitPointsRequest_Damage{Damage: 200}
		}); err != nil {
			t.Fatalf("AdjustCombatantHitPoints(%s) error = %v", label, err)
		}
		return a.get(t, a.master)
	}
}

func d20Face(face int32) func(*playv1.CastSpellRequest) {
	return func(r *playv1.CastSpellRequest) { r.Roll = &playv1.CastSpellRequest_D20Face{D20Face: face} }
}

func d20Faces(faces ...int32) func(*playv1.CastSpellRequest) {
	return func(r *playv1.CastSpellRequest) { r.Roll, r.D20Faces = nil, faces }
}

// castRequestScenarios are the refusals of the request's own fields.
func castRequestScenarios() []goldenScenario {
	long := strings.Repeat("x", 101)
	eleven := make([]string, 11)
	for i := range eleven {
		eleven[i] = "Goblin"
	}
	return []goldenScenario{
		castRefused("an empty spell key", pensantus("", slotOfLevel(1), nil)),
		castRefused("a spell key of 101 characters", pensantus(long, slotOfLevel(1), nil)),
		castRefused("a key that is not a UUID", pensantus(magicMissileSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) { r.IdempotencyKey = "not-a-key" })),
		castRefused("an encounter that is not a UUID", pensantus(magicMissileSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) { r.EncounterId = "nope" })),
		castRefused("a caster that is not a UUID", pensantus(magicMissileSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) { r.CasterId = "nope" })),
		castRefused("eleven targets", pensantus(sleepSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) {
			for range 11 {
				r.Targets = append(r.Targets, &playv1.SpellTarget{CombatantId: r.GetCasterId()})
			}
		})),
		castRefused("a target that is not a UUID", pensantus(magicMissileSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) {
			r.Targets = []*playv1.SpellTarget{{CombatantId: "nope"}}
		})),
		castRefused("a target twice", pensantus(magicMissileSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) {
			r.Targets = append(r.Targets, r.Targets[0])
		})),
		castRefused("negative darts", pensantus(magicMissileSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) { r.Targets[0].Darts = -1 })),
		castRefused("twenty-one darts", pensantus(magicMissileSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) { r.Targets[0].Darts = 21 })),
		castRefused("roll_in_app false", pensantus(sleepSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) {
			r.Roll = &playv1.CastSpellRequest_RollInApp{RollInApp: false}
		})),
		castRefused("d20_face of 0", pensantus(sleepSpell, slotOfLevel(1), d20Face(0))),
		castRefused("d20_face of 21", pensantus(sleepSpell, slotOfLevel(1), d20Face(21))),
		castRefused("a typed face out of range", pensantus(sleepSpell, slotOfLevel(1), d20Faces(4, 22))),
		castRefused("three typed faces", pensantus(sleepSpell, slotOfLevel(1), d20Faces(4, 9, 12))),
		castRefused("reveal_hidden from a player", pensantus(burningHands, slotOfLevel(1), func(r *playv1.CastSpellRequest) {
			reveal := true
			r.RevealHidden = &reveal
			aimToward(1, 0)(r)
		})),
		castRefused("reveal_hidden from a player before anything else", pensantus("", slotOfLevel(1), func(r *playv1.CastSpellRequest) {
			reveal := false
			r.RevealHidden = &reveal
		})),
		castRefused("a Revivify the caster does not have", pensantus(revivifyKey, slotOfLevel(3), nil)),
		castRefused("eleven targets as the master", pensantus(sleepSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) {
			for range 11 {
				r.Targets = append(r.Targets, &playv1.SpellTarget{CombatantId: r.GetCasterId()})
			}
		}).by(asMasterOf)),
		castOn("ten goblins asleep", func(t *testing.T) (*armed, int, *playv1.CastSpellResponse, error) {
			t.Helper()
			a := newCasters(t)
			e := a.castersFight(t, 10)
			before := a.markCall(t)
			res, err := a.castCall(t, a.ana, e, "Pensantus", sleepSpell, slotOfLevel(1), nil, together(poolInApp, aimAt(7, 5)))
			return a, before, res, err
		}),
	}
}

func asMasterOf(a *armed) *user { return a.master }

// slotsAllUsed spends every spell slot of a player's character before the cast.
func slotsAllUsed(label string) func(t *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter {
	return func(t *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter {
		t.Helper()
		c := map[string]*charactersv1.Character{"Toren": a.toren, "Pensantus": a.pens, "Brisa": a.bri}[label]
		if _, err := a.h.pool.Exec(t.Context(), `UPDATE character_vitals SET spell_slots_used = '{9,9,9}' WHERE character_id = $1`, c.GetId()); err != nil {
			t.Fatalf("spend the slots: %v", err)
		}
		return e
	}
}

// deadToren leaves Toren dead (three failed death saves), and confirmed when confirm is set.
func deadToren(confirm bool) func(t *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter {
	return func(t *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter {
		t.Helper()
		a.correct(t, a.toren, hpIs(0))
		if _, err := a.h.pool.Exec(t.Context(), `UPDATE combatants SET death_failures = 3 WHERE id = $1`, a.id(t, "Toren")); err != nil {
			t.Fatalf("set the failed saves: %v", err)
		}
		if confirm {
			if _, err := a.master.combat.ConfirmDeath(t.Context(), connect.NewRequest(&playv1.ConfirmDeathRequest{
				CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Toren"), IdempotencyKey: newKey(),
			})); err != nil {
				t.Fatalf("ConfirmDeath() error = %v", err)
			}
		}
		return a.get(t, a.master)
	}
}

// originAt places the area of the spell on a square.
func aimAt(col, row int32) func(*playv1.CastSpellRequest) {
	return func(r *playv1.CastSpellRequest) {
		r.Area = &playv1.CastSpellRequest_Origin{Origin: &playv1.SpellOrigin{Col: col, Row: row}}
	}
}

// toward points the area of the spell from the caster.
func aimToward(dx, dy int32) func(*playv1.CastSpellRequest) {
	return func(r *playv1.CastSpellRequest) {
		r.Area = &playv1.CastSpellRequest_Direction{Direction: &playv1.SpellDirection{Dx: dx, Dy: dy}}
	}
}

// both applies the edits to the request, one after the other.
func together(edits ...func(*playv1.CastSpellRequest)) func(*playv1.CastSpellRequest) {
	return func(r *playv1.CastSpellRequest) {
		for _, e := range edits {
			e(r)
		}
	}
}

func rollMode(mode playv1.RollMode, reason string) func(*playv1.CastSpellRequest) {
	return func(r *playv1.CastSpellRequest) { r.RollMode, r.ModeReason = mode, reason }
}

func damageType(key string) func(*playv1.CastSpellRequest) {
	return func(r *playv1.CastSpellRequest) { r.DamageTypeKey = key }
}

func (f castFight) unplaced() castFight {
	f.raw = true
	return f
}

func (f castFight) turnOf(label string) castFight {
	f.turn = label
	return f
}

// castRefusalScenarios are the refusals the checks of the transaction give.
func castRefusalScenarios() []goldenScenario {
	unknown := "00000000-0000-4000-8000-000000000001"
	return []goldenScenario{
		castRefused("a caster of another player", pensantus(magicMissileSpell, slotOfLevel(1), nil).by(asBia)),
		castRefused("a caster that is not in the combat", pensantus(magicMissileSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) { r.CasterId = unknown })),
		castRefused("a target that is not in the combat", pensantus(magicMissileSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) {
			r.Targets = []*playv1.SpellTarget{{CombatantId: unknown, Darts: 3}}
		})),
		castRefused("out of turn", brisa(cureWounds, slotOfLevel(1), nil).to("Toren").turnOf("")),
		castRefused("a fighter with no spells", castFight{as: asCaio, caster: "Toren", turn: "Toren", spell: magicMissileSpell, slot: slotOfLevel(1), targets: []string{"Goblin"}}),
		castRefused("a spell the caster does not know", pensantus(fireballSpell, slotOfLevel(3), nil)),
		castRefused("a cantrip that is an attack roll", pensantus(fireBolt, nil, nil)),
		castRefused("a defeated target", pensantus(magicMissileSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) { r.Targets[0].Darts = 3 }).first(defeat("Goblin"))),
		castRefused("a defeated player's character as a target", pensantus(holdPerson, slotOfLevel(2), nil).to("Toren").first(deadToren(true))),
		castRefused("a reaction spell", pensantus(shieldSpell, slotOfLevel(1), nil).to()),
		castRefused("a second action spell", pensantus(magicMissileSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) { r.Targets[0].Darts = 3 }).first(castFirst(sleepSpell, slotOfLevel(1), poolInApp, "Goblin"))),
		castRefused("a second bonus action spell", brisa(healingWord, slotOfLevel(1), nil).to("Toren").first(func(t *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter {
			t.Helper()
			a.mustCast(t, a.bia, e, "Brisa", healingWord, slotOfLevel(1), a.at(t, "Toren"), noCastRoll)
			return a.get(t, a.master)
		})),
		castRefused("a leveled spell after a bonus action spell", brisa(cureWounds, slotOfLevel(1), nil).to("Toren").first(func(t *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter {
			t.Helper()
			a.mustCast(t, a.bia, e, "Brisa", healingWord, slotOfLevel(1), a.at(t, "Toren"), noCastRoll)
			return a.get(t, a.master)
		})),
		castRefused("a bonus action spell after a leveled spell", brisa(healingWord, slotOfLevel(1), nil).to("Toren").first(func(t *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter {
			t.Helper()
			a.mustCast(t, a.bia, e, "Brisa", cureWounds, slotOfLevel(1), a.at(t, "Toren"), noCastRoll)
			return a.get(t, a.master)
		})),
		castRefused("no free slot", pensantus(sleepSpell, slotOfLevel(1), together(poolInApp, aimAt(7, 5))).first(slotsAllUsed("Pensantus"))),
		castRefused("a cantrip with a slot", brisa(sacredFlame, slotOfLevel(1), nil)),
		castRefused("a leveled spell with no slot", pensantus(sleepSpell, nil, together(poolInApp, aimAt(7, 5)))),
		castRefused("a slot below the spell's level", brisa(cureWounds, slotOfLevel(0), nil).to("Toren")),
		castRefused("a slot above the ninth", pensantus(sleepSpell, slotOfLevel(10), together(poolInApp, aimAt(7, 5)))),
		castRefused("a slot the caster has not got", pensantus(sleepSpell, slotOfLevel(5), together(poolInApp, aimAt(7, 5)))),
		castRefused("a pact slot the caster has not got", pensantus(sleepSpell, &playv1.SpellSlot{Level: 1, Pact: true}, together(poolInApp, aimAt(7, 5)))),
		castRefused("a damage type the spell does not offer", pensantus(sleepSpell, slotOfLevel(1), together(poolInApp, aimAt(7, 5), damageType("damage-type:fire")))),
		pensantus(burningHands, slotOfLevel(1), together(aimToward(1, 0), abilityOf("ability:strength"))).scenario("an ability key sent with burning hands"),
		castRefused("a healing out of touch reach", brisa(cureWounds, slotOfLevel(1), nil)),
		castRefused("a healing on a dead target", brisa(cureWounds, slotOfLevel(1), nil).to("Toren").first(deadToren(false))),
		castRefused("a healing on a confirmed dead target", brisa(cureWounds, slotOfLevel(1), nil).to("Toren").first(deadToren(true))),
		castRefused("a summon sent to a spell that does not summon", pensantus(sleepSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) {
			r.Roll, r.Summon = &playv1.CastSpellRequest_RollInApp{RollInApp: true}, &playv1.SummonChoice{Option: 1}
		})),
		castRefused("an area for a spell that has none", pensantus(magicMissileSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) {
			r.Targets[0].Darts = 3
			r.Area = &playv1.CastSpellRequest_Origin{Origin: &playv1.SpellOrigin{Col: 7, Row: 5}}
		})),
		castRefused("an area outside the grid", pensantus(burningHands, slotOfLevel(1), func(r *playv1.CastSpellRequest) {
			r.Area = &playv1.CastSpellRequest_Direction{Direction: &playv1.SpellDirection{Dx: 5, Dy: 5}}
		})),
		castRefused("an area with no origin or direction", pensantus(webSpell, slotOfLevel(2), nil).unplaced()),
		castRefused("darts that do not add up", pensantus(magicMissileSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) { r.Targets[0].Darts = 2 })),
		castRefused("a dart with no target", pensantus(magicMissileSpell, slotOfLevel(1), nil).to()),
		castRefused("a target with no dart", pensantus(magicMissileSpell, slotOfLevel(1), func(r *playv1.CastSpellRequest) {
			r.Targets = []*playv1.SpellTarget{{CombatantId: r.Targets[0].CombatantId, Darts: 3}, {CombatantId: r.GetCasterId()}}
		})),
		castRefused("darts for another spell", pensantus(holdPerson, slotOfLevel(2), func(r *playv1.CastSpellRequest) { r.Targets[0].Darts = 1 })),
		castRefused("a spell with no target", pensantus(holdPerson, slotOfLevel(2), nil).to()),
		castRefused("two targets for a single target spell", pensantus(holdPerson, slotOfLevel(2), nil).to("Goblin", "Capitão Goblin")),
		castRefused("a target out of range", pensantus(holdPerson, slotOfLevel(2), nil).first(func(t *testing.T, a *armed, _ *playv1.Encounter) *playv1.Encounter {
			t.Helper()
			a.put(t, "Goblin", 19, 9)
			return a.get(t, a.master)
		})),
		castRefused("a target behind total cover", pensantus(holdPerson, slotOfLevel(2), nil).first(func(t *testing.T, a *armed, _ *playv1.Encounter) *playv1.Encounter {
			t.Helper()
			if _, err := a.h.pool.Exec(t.Context(), `UPDATE combatants SET cover_mark = 'total' WHERE label = 'Goblin'`); err != nil {
				t.Fatalf("mark the cover: %v", err)
			}
			return a.get(t, a.master)
		})),
		castRefused("a spell attack with a pool", brisa(guidingBolt, slotOfLevel(1), poolSum(5))),
		castRefused("a spell attack with no roll", brisa(guidingBolt, slotOfLevel(1), nil)),
		castRefused("a spell attack with a face for several targets", brisa(guidingBolt, slotOfLevel(1), d20Face(14)).to("Goblin", "Capitão Goblin").by(asMasterOf)),
		castRefused("a spell attack at two targets as a player", brisa(guidingBolt, slotOfLevel(1), poolInApp).to("Goblin", "Capitão Goblin")),
		castRefused("a pool spell with no roll", pensantus(sleepSpell, slotOfLevel(1), nil)),
		castRefused("a pool spell with a d20 face", pensantus(sleepSpell, slotOfLevel(1), d20Face(10))),
		castRefused("a spell attack in app for a player who rolls physical dice", brisa(guidingBolt, slotOfLevel(1), poolInApp).first(func(t *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter {
			t.Helper()
			a.setPhysical(t, a.bia)
			return e
		})),
		castRefused("a spell attack with a face for a player who rolls in the app", brisa(guidingBolt, slotOfLevel(1), d20Face(14)).first(func(t *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter {
			t.Helper()
			a.forceDice(t, campaignsv1.DiceMode_DICE_MODE_APP)
			return e
		})),
	}
}

// missiles sets the darts of each target of the request.
func missiles(darts ...int32) func(*playv1.CastSpellRequest) {
	return func(r *playv1.CastSpellRequest) {
		for i, d := range darts {
			r.Targets[i].Darts = d
		}
	}
}

// castThroughScenarios are the casts that go through.
func castThroughScenarios() []goldenScenario {
	return []goldenScenario{
		pensantus(magicMissileSpell, slotOfLevel(1), missiles(3)).scenario("magic missile with three darts"),
		pensantus(magicMissileSpell, slotOfLevel(2), missiles(2, 2)).to("Goblin", "Capitão Goblin").scenario("magic missile from a second level slot at two targets"),
		pensantus(magicMissileSpell, slotOfLevel(1), missiles(3)).by(asMasterOf).scenario("magic missile as the master for the player"),
		pensantus(magicMissileSpell, slotOfLevel(1), missiles(1, 1, 1)).to("Goblin", "Capitão Goblin", "Toren").scenario("magic missile one dart at each of three"),
		pensantus(sleepSpell, slotOfLevel(1), together(poolInApp, aimAt(7, 5))).scenario("sleep rolled in the app"),
		pensantus(sleepSpell, slotOfLevel(2), together(poolSum(30), aimAt(7, 5))).scenario("sleep with a typed pool from a second level slot"),
		pensantus(sleepSpell, slotOfLevel(1), together(poolInApp, aimAt(4, 4))).by(asMasterOf).scenario("sleep as the master"),
		pensantus(burningHands, slotOfLevel(1), aimToward(1, 0)).first(queued(5, 12, 9)).scenario("burning hands that reaches a goblin and an ally"),
		pensantus(burningHands, slotOfLevel(2), aimToward(-1, -1)).first(queued(15)).scenario("burning hands from a second level slot, saved"),
		pensantus(burningHands, slotOfLevel(1), aimToward(1, 0)).by(asMasterOf).first(queued(3, 4)).scenario("burning hands as the master"),
		pensantus(holdPerson, slotOfLevel(2), nil).first(queued(4)).scenario("hold person"),
		pensantus(holdPerson, slotOfLevel(2), nil).to("Capitão Goblin").first(queued(19)).scenario("hold person saved"),
		pensantus(holdPerson, slotOfLevel(2), nil).first(func(t *testing.T, a *armed, e *playv1.Encounter) *playv1.Encounter {
			t.Helper()
			e = castFirst(webSpell, slotOfLevel(2), nil, "Goblin")(t, a, e)
			a.passTo(t, e, "Toren")
			return a.passTo(t, e, "Pensantus")
		}).scenario("a second concentration spell"),
		pensantus(webSpell, slotOfLevel(2), nil).to("Goblin", "Capitão Goblin").first(queued(6, 17)).scenario("web at two creatures"),
		brisa(sacredFlame, nil, nil).first(queued(3)).scenario("sacred flame failed"),
		brisa(sacredFlame, nil, nil).first(queued(18)).scenario("sacred flame saved"),
		brisa(sacredFlame, nil, nil).to("Capitão Goblin").by(asMasterOf).first(queued(9)).scenario("sacred flame as the master"),
		brisa(guidingBolt, slotOfLevel(1), d20Face(15)).scenario("guiding bolt that hits"),
		brisa(guidingBolt, slotOfLevel(1), d20Face(2)).scenario("guiding bolt that misses"),
		brisa(guidingBolt, slotOfLevel(1), d20Face(20)).scenario("guiding bolt with a natural 20"),
		brisa(guidingBolt, slotOfLevel(1), d20Face(1)).scenario("guiding bolt with a natural 1"),
		brisa(guidingBolt, slotOfLevel(1), poolInApp).first(queued(14)).scenario("guiding bolt rolled in the app"),
		brisa(guidingBolt, slotOfLevel(2), d20Faces(3, 17)).scenario("guiding bolt with a pair of faces and no mode asked"),
		castRefused("advantage asked by a player", brisa(guidingBolt, slotOfLevel(2), together(d20Faces(3, 17), rollMode(playv1.RollMode_ROLL_MODE_ADVANTAGE, "a bênção")))),
		castRefused("advantage asked with no reason", brisa(guidingBolt, slotOfLevel(2), together(d20Faces(3, 17), rollMode(playv1.RollMode_ROLL_MODE_ADVANTAGE, ""))).by(asMasterOf)),
		brisa(guidingBolt, slotOfLevel(2), together(d20Faces(3, 17), rollMode(playv1.RollMode_ROLL_MODE_ADVANTAGE, "a bênção"))).by(asMasterOf).scenario("guiding bolt with advantage asked by the master and a pair of faces"),
		brisa(guidingBolt, slotOfLevel(1), together(d20Faces(17, 3), rollMode(playv1.RollMode_ROLL_MODE_DISADVANTAGE, "Brisa está cansada"))).scenario("guiding bolt with disadvantage asked and a reason"),
		brisa(guidingBolt, slotOfLevel(1), together(poolInApp, rollMode(playv1.RollMode_ROLL_MODE_ADVANTAGE, "a bênção"))).by(asMasterOf).first(queued(4, 16)).scenario("guiding bolt with advantage rolled in the app"),
		brisa(guidingBolt, slotOfLevel(1), d20Face(11)).to("Capitão Goblin").scenario("guiding bolt against armor class 18"),
		brisa(guidingBolt, slotOfLevel(1), poolInApp).to("Goblin", "Capitão Goblin").by(asMasterOf).first(queued(14, 9)).scenario("guiding bolt at two targets as the master"),
		brisa(guidingBolt, slotOfLevel(1), d20Face(14)).by(asMasterOf).scenario("guiding bolt as the master with a face"),
		brisa(cureWounds, slotOfLevel(1), nil).to("Toren").first(hurtTo("Toren", 5)).scenario("cure wounds"),
		brisa(cureWounds, slotOfLevel(2), nil).to("Toren").first(hurtTo("Toren", 5)).scenario("cure wounds from a second level slot"),
		brisa(cureWounds, slotOfLevel(1), nil).to("Toren").scenario("cure wounds on a whole creature"),
		brisa(healingWord, slotOfLevel(1), nil).to("Toren").first(hurtTo("Toren", 5)).scenario("healing word"),
		brisa(cureWounds, slotOfLevel(1), nil).to("Pensantus").by(asMasterOf).first(hurtTo("Pensantus", 4)).scenario("cure wounds as the master"),
		brisa(cureWounds, slotOfLevel(1), nil).to("Toren").first(hurtTo("Toren", 0)).scenario("cure wounds on a creature at zero"),
		brisa(cureWounds, slotOfLevel(1), nil).to("Brisa").first(hurtTo("Brisa", 3)).scenario("cure wounds on the caster"),
		brisa(sacredFlame, nil, nil).first(queued(2)).by(asMasterOf).turnOf("Toren").scenario("a cantrip as the master out of turn"),
	}
}
