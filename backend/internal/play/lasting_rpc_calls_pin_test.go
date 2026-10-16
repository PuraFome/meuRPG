package play

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The master's calls on a running effect: change its duration, switch who reads it, end it, take
// a target off it. Each scenario writes a transcript: what every call answered (the refusals with
// their code and message), the effect rows and the events written after it.

// rpcScript is the transcript of a scenario.
type rpcScript struct {
	t *testing.T
	a *armed
	b strings.Builder
}

func (a *armed) rpcScript(t *testing.T) *rpcScript {
	t.Helper()
	return &rpcScript{t: t, a: a}
}

// say writes the answer of a call: the refusal, or the message without its encounter.
func (s *rpcScript) say(name string, res proto.Message, err error) {
	s.t.Helper()
	fmt.Fprintf(&s.b, "## %s\n", name)
	if err != nil {
		var ce *connect.Error
		if errors.As(err, &ce) {
			fmt.Fprintf(&s.b, "refused %s: %s\n", ce.Code(), ce.Message())
		} else {
			fmt.Fprintf(&s.b, "refused: %v\n", err)
		}
		return
	}
	clone := proto.Clone(res)
	if f := clone.ProtoReflect().Descriptor().Fields().ByName("encounter"); f != nil {
		clone.ProtoReflect().Clear(f)
	}
	fmt.Fprintf(&s.b, "%s\n", s.a.rpcJSON(s.t, clone))
}

// note writes a line of text.
func (s *rpcScript) note(name, text string) {
	fmt.Fprintf(&s.b, "## %s\n%s", name, text)
	if !strings.HasSuffix(text, "\n") {
		s.b.WriteString("\n")
	}
}

// state writes the effect rows and the events of the kinds, as they are now.
func (s *rpcScript) state(name string, kinds ...string) {
	s.t.Helper()
	s.note(name+": rows", s.a.rpcClean(s.t, s.a.rpcRows(s.t)))
	if len(kinds) > 0 {
		s.note(name+": events", s.a.rpcEvents(s.t, kinds...))
	}
}

func (s *rpcScript) want(name string) { wantRPCGolden(s.t, name, s.b.String()) }

func (a *armed) rpcChange(t *testing.T, u *user, e *playv1.Encounter, effect string, dur *playv1.EffectDurationChoice, edit ...func(*playv1.ChangeLastingEffectDurationRequest)) (*playv1.ChangeLastingEffectDurationResponse, error) {
	t.Helper()
	a.rpcLabels(t) // remember the rows the call may take away
	req := &playv1.ChangeLastingEffectDurationRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey(), EffectId: effect, Duration: dur}
	for _, f := range edit {
		f(req)
	}
	res, err := u.lasting.ChangeLastingEffectDuration(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (a *armed) rpcVisibility(t *testing.T, u *user, e *playv1.Encounter, effect string, edit func(*playv1.SetLastingEffectVisibilityRequest)) (*playv1.SetLastingEffectVisibilityResponse, error) {
	t.Helper()
	a.rpcLabels(t) // remember the rows the call may take away
	req := &playv1.SetLastingEffectVisibilityRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey(), EffectId: effect}
	if edit != nil {
		edit(req)
	}
	res, err := u.lasting.SetLastingEffectVisibility(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (a *armed) rpcEnd(t *testing.T, u *user, e *playv1.Encounter, effect string, scope playv1.EffectEndScope, edit ...func(*playv1.EndLastingEffectRequest)) (*playv1.EndLastingEffectResponse, error) {
	t.Helper()
	a.rpcLabels(t) // remember the rows the call may take away
	req := &playv1.EndLastingEffectRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey(), EffectId: effect, Scope: scope}
	for _, f := range edit {
		f(req)
	}
	res, err := u.lasting.EndLastingEffect(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (a *armed) rpcRemove(t *testing.T, u *user, e *playv1.Encounter, effect, combatant string, edit ...func(*playv1.RemoveEffectTargetRequest)) (*playv1.RemoveEffectTargetResponse, error) {
	t.Helper()
	a.rpcLabels(t) // remember the rows the call may take away
	req := &playv1.RemoveEffectTargetRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey(), EffectId: effect, CombatantId: combatant}
	for _, f := range edit {
		f(req)
	}
	res, err := u.lasting.RemoveEffectTarget(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// rpcChoice is a duration the master picks.
func (a *armed) rpcChoice(t *testing.T, kind playv1.EffectDurationKind, rounds int32, anchor string) *playv1.EffectDurationChoice {
	t.Helper()
	d := &playv1.EffectDurationChoice{Kind: kind, Rounds: rounds}
	if anchor != "" {
		d.AnchorCombatantId = a.id(t, anchor)
	}
	return d
}

// rpcFight is the close fight with a casting of Bless by Pensantus on Toren and Brisa, a Hold
// Person on the Goblin, and a condition of the master's own on Brisa.
func (a *armed) rpcFight(t *testing.T) *playv1.Encounter {
	t.Helper()
	e := a.closeFight(t)
	a.rpcAdd(t, e, blessKey, []string{"Toren", "Brisa"}, a.rpcCaster(t, "Pensantus"), a.rpcDuration(t, rpcDurRounds, 10, "Pensantus"))
	a.rpcAdd(t, e, holdPerson, []string{"Goblin"}, a.rpcCaster(t, "Brisa"))
	a.rpcAdd(t, e, "condition:prone", []string{"Brisa"})
	return a.get(t, a.master)
}

// TestLastingRPCChangeTheDuration freezes ChangeLastingEffectDuration: every refusal and every
// kind of duration it writes.
func TestLastingRPCChangeTheDuration(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.rpcFight(t)
	s := a.rpcScript(t)
	bless, hold, prone := a.rpcEffectID(t, blessKey, "Toren"), a.rpcEffectID(t, holdPerson, "Goblin"), a.rpcEffectID(t, "condition:prone", "Brisa")
	rounds := func(n int32) *playv1.EffectDurationChoice { return a.rpcChoice(t, rpcDurRounds, n, "Pensantus") }

	// Who may call, and the request read before the transaction.
	res, err := a.rpcChange(t, a.caio, e, bless, rounds(3))
	s.say("a player", res, err)
	res, err = a.rpcChange(t, a.master, e, bless, rounds(3), func(r *playv1.ChangeLastingEffectDurationRequest) { r.IdempotencyKey = "no" })
	s.say("a bad key", res, err)
	res, err = a.rpcChange(t, a.master, e, bless, rounds(3), func(r *playv1.ChangeLastingEffectDurationRequest) { r.EncounterId = "no" })
	s.say("a bad combat id", res, err)
	res, err = a.rpcChange(t, a.master, e, "no", rounds(3))
	s.say("a bad effect id", res, err)
	res, err = a.rpcChange(t, a.master, e, newKey(), rounds(3))
	s.say("an effect that is not there", res, err)
	res, err = a.rpcChange(t, a.master, e, bless, rounds(3), func(r *playv1.ChangeLastingEffectDurationRequest) { r.EncounterId = newKey() })
	s.say("a combat that is not there", res, err)

	// The durations the master may not pick.
	res, err = a.rpcChange(t, a.master, e, bless, rounds(0))
	s.say("rounds 0", res, err)
	res, err = a.rpcChange(t, a.master, e, bless, rounds(601))
	s.say("rounds 601", res, err)
	res, err = a.rpcChange(t, a.master, e, bless, a.rpcChoice(t, rpcDurStartTurn, 0, ""))
	s.say("until the start of a turn with no anchor", res, err)
	res, err = a.rpcChange(t, a.master, e, bless, a.rpcChoice(t, rpcDurEndTurn, 0, ""))
	s.say("until the end of a turn with no anchor", res, err)
	res, err = a.rpcChange(t, a.master, e, bless, &playv1.EffectDurationChoice{Kind: rpcDurRounds, Rounds: 3, AnchorCombatantId: "no"})
	s.say("an anchor that is not an id", res, err)
	res, err = a.rpcChange(t, a.master, e, bless, &playv1.EffectDurationChoice{Kind: rpcDurRounds, Rounds: 3, AnchorCombatantId: newKey()})
	s.say("an anchor that is not in the combat", res, err)
	s.state("after the refusals", eventLastingChanged)

	// What it writes.
	res, err = a.rpcChange(t, a.master, e, bless, rounds(4))
	s.say("bless to 4 rounds", res, err)
	res, err = a.rpcChange(t, a.master, e, bless, a.rpcChoice(t, rpcDurEndTurn, 0, "Goblin"))
	s.say("bless until the end of the Goblin's turn", res, err)
	res, err = a.rpcChange(t, a.master, e, bless, a.rpcChoice(t, rpcDurStartTurn, 5, "Toren"))
	s.say("bless until the start of Toren's turn (the rounds are dropped)", res, err)
	res, err = a.rpcChange(t, a.master, e, bless, a.rpcChoice(t, rpcDurConc, 7, "Toren"))
	s.say("bless by concentration (the rounds and the anchor are dropped)", res, err)
	s.state("after bless", eventLastingChanged)
	res, err = a.rpcChange(t, a.master, e, hold, nil)
	s.say("hold person with no duration (keeps the kind)", res, err)
	res, err = a.rpcChange(t, a.master, e, hold, &playv1.EffectDurationChoice{})
	s.say("hold person with an unspecified duration", res, err)
	res, err = a.rpcChange(t, a.master, e, hold, a.rpcChoice(t, rpcDurDismissed, 3, ""))
	s.say("hold person until dismissed", res, err)
	res, err = a.rpcChange(t, a.master, e, hold, a.rpcChoice(t, rpcDurLongRest, 0, ""))
	s.say("hold person until a long rest", res, err)
	res, err = a.rpcChange(t, a.master, e, hold, rounds(2))
	s.say("hold person to 2 rounds", res, err)
	res, err = a.rpcChange(t, a.master, e, prone, a.rpcChoice(t, rpcDurRounds, 3, "Toren"))
	s.say("a condition with no caster to 3 rounds", res, err)
	key := newKey()
	again := func(r *playv1.ChangeLastingEffectDurationRequest) { r.IdempotencyKey = key }
	res, err = a.rpcChange(t, a.master, e, prone, rounds(9), again)
	s.say("the same request, first time", res, err)
	res, err = a.rpcChange(t, a.master, e, prone, rounds(9), again)
	s.say("the same request again", res, err)
	res, err = a.rpcChange(t, a.master, e, prone, rounds(8), again)
	s.say("the same key with another request", res, err)
	s.state("at the end", eventLastingChanged)
	s.want("change_duration")
}

// TestLastingRPCChangeTheDurationOfAHiddenEffect freezes the line of the log: it is the master's
// alone when the effect is hidden from the players or is on a hidden combatant.
func TestLastingRPCChangeTheDurationOfAHiddenEffect(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	a.rpcAdd(t, e, blessKey, []string{"Toren"}, a.rpcCaster(t, "Pensantus"), rpcVisibleLabel(false, "", playv1.EffectAudience_EFFECT_AUDIENCE_ALL))
	a.rpcAdd(t, e, "spell:bane", []string{"Brisa"}, rpcVisibleLabel(true, "", playv1.EffectAudience_EFFECT_AUDIENCE_OWNER))
	a.rpcAdd(t, e, "spell:faerie-fire", []string{"Pensantus"})
	s := a.rpcScript(t)
	for _, c := range []struct{ key, label string }{{blessKey, "Toren"}, {"spell:bane", "Brisa"}, {"spell:faerie-fire", "Pensantus"}} {
		res, err := a.rpcChange(t, a.master, e, a.rpcEffectID(t, c.key, c.label), a.rpcChoice(t, rpcDurRounds, 2, c.label))
		s.say("change "+c.key, res, err)
	}
	s.state("hidden", eventLastingChanged)
	s.want("change_duration_hidden")
}

// TestLastingRPCSetTheVisibility freezes SetLastingEffectVisibility.
func TestLastingRPCSetTheVisibility(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.rpcFight(t)
	s := a.rpcScript(t)
	bless, hold := a.rpcEffectID(t, blessKey, "Toren"), a.rpcEffectID(t, holdPerson, "Goblin")
	set := func(visible bool, audience playv1.EffectAudience, label string) func(*playv1.SetLastingEffectVisibilityRequest) {
		return func(r *playv1.SetLastingEffectVisibilityRequest) {
			r.PlayerVisible, r.Audience, r.PlayerLabel = visible, audience, label
		}
	}
	all, owner := playv1.EffectAudience_EFFECT_AUDIENCE_ALL, playv1.EffectAudience_EFFECT_AUDIENCE_OWNER

	res, err := a.rpcVisibility(t, a.caio, e, bless, set(true, all, ""))
	s.say("a player", res, err)
	res, err = a.rpcVisibility(t, a.master, e, bless, func(r *playv1.SetLastingEffectVisibilityRequest) { r.IdempotencyKey = "no" })
	s.say("a bad key", res, err)
	res, err = a.rpcVisibility(t, a.master, e, bless, func(r *playv1.SetLastingEffectVisibilityRequest) { r.EncounterId = "no" })
	s.say("a bad combat id", res, err)
	res, err = a.rpcVisibility(t, a.master, e, "no", set(true, all, ""))
	s.say("a bad effect id", res, err)
	res, err = a.rpcVisibility(t, a.master, e, newKey(), set(true, all, ""))
	s.say("an effect that is not there", res, err)
	res, err = a.rpcVisibility(t, a.master, e, bless, set(true, all, strings.Repeat("a", 31)))
	s.say("a label of 31 characters", res, err)
	res, err = a.rpcVisibility(t, a.master, e, bless, set(true, all, strings.Repeat("ã", 31)))
	s.say("a label of 31 letters with accents", res, err)
	res, err = a.rpcVisibility(t, a.master, e, bless, set(true, all, "  "+strings.Repeat("a", 30)+"  "))
	s.say("a label of 30 characters between spaces", res, err)
	s.state("after the refusals", eventLastingVisibility)

	res, err = a.rpcVisibility(t, a.master, e, bless, set(true, owner, "Abençoado"))
	s.say("bless to the owner alone with a label", res, err)
	res, err = a.rpcVisibility(t, a.master, e, bless, set(false, playv1.EffectAudience_EFFECT_AUDIENCE_UNSPECIFIED, "   "))
	s.say("bless hidden, an audience unspecified and a label of spaces", res, err)
	res, err = a.rpcVisibility(t, a.master, e, hold, nil)
	s.say("hold person with the empty request", res, err)
	res, err = a.rpcVisibility(t, a.master, e, hold, set(true, all, "Paralisado"))
	s.say("hold person visible to all with a label", res, err)
	key := newKey()
	again := func(r *playv1.SetLastingEffectVisibilityRequest) { r.IdempotencyKey = key; r.PlayerVisible = true }
	res, err = a.rpcVisibility(t, a.master, e, hold, again)
	s.say("the same request, first time", res, err)
	res, err = a.rpcVisibility(t, a.master, e, hold, again)
	s.say("the same request again", res, err)
	s.state("at the end", eventLastingVisibility)
	s.note("what the owner of Toren reads", a.rpcView(t, a.caio))
	s.note("what the owner of Pensantus reads", a.rpcView(t, a.ana))
	s.want("visibility")
}

// TestLastingRPCEndAnEffect freezes EndLastingEffect in a combat.
func TestLastingRPCEndAnEffect(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.rpcFight(t)
	s := a.rpcScript(t)
	this, group := playv1.EffectEndScope_EFFECT_END_SCOPE_THIS, playv1.EffectEndScope_EFFECT_END_SCOPE_CONCENTRATION_GROUP
	bless, hold, prone := a.rpcEffectID(t, blessKey, "Toren"), a.rpcEffectID(t, holdPerson, "Goblin"), a.rpcEffectID(t, "condition:prone", "Brisa")

	res, err := a.rpcEnd(t, a.caio, e, bless, this)
	s.say("a player", res, err)
	res, err = a.rpcEnd(t, a.master, e, bless, this, func(r *playv1.EndLastingEffectRequest) { r.IdempotencyKey = "no" })
	s.say("a bad key", res, err)
	res, err = a.rpcEnd(t, a.master, e, bless, playv1.EffectEndScope_EFFECT_END_SCOPE_UNSPECIFIED)
	s.say("a scope unspecified", res, err)
	res, err = a.rpcEnd(t, a.master, e, bless, playv1.EffectEndScope(99))
	s.say("a scope that is not one", res, err)
	res, err = a.rpcEnd(t, a.master, e, bless, this, func(r *playv1.EndLastingEffectRequest) { r.EncounterId = "no" })
	s.say("a bad combat id", res, err)
	res, err = a.rpcEnd(t, a.master, e, "no", this)
	s.say("a bad effect id", res, err)
	res, err = a.rpcEnd(t, a.master, e, newKey(), this)
	s.say("an effect that is not there", res, err)
	res, err = a.rpcEnd(t, a.master, e, bless, this, func(r *playv1.EndLastingEffectRequest) { r.EncounterId = newKey() })
	s.say("a combat that is not there", res, err)
	s.state("after the refusals", eventLastingEnded, eventConditionsSet)

	// Marks the master made by hand end by their id: loose:<combatant>:<condition>.
	for _, who := range []string{"Toren", "Pensantus"} {
		if _, err := a.conditions(t, a.master, e, who, []string{"condition:frightened", "condition:blinded"}, true, false); err != nil {
			t.Fatalf("SetCombatantConditions(%s) error = %v", who, err)
		}
	}
	torenID, pensID := a.id(t, "Toren"), a.id(t, "Pensantus")
	for _, c := range []struct{ name, id string }{
		{"no condition in the id", "loose:" + torenID},
		{"a condition Toren does not have", "loose:" + torenID + ":condition:prone"},
		{"a condition an effect gives", "loose:" + a.id(t, "Brisa") + ":condition:prone"},
		{"a combatant that is not there", "loose:" + newKey() + ":condition:blinded"},
		{"nothing after the prefix", "loose:"},
		{"a combatant that is not an id", "loose:no:condition:blinded"},
	} {
		res, err = a.rpcEnd(t, a.master, e, c.id, this)
		s.say("loose: "+c.name, res, err)
	}
	res, err = a.rpcEnd(t, a.master, e, "loose:"+torenID+":condition:frightened", group)
	s.say("a loose condition off Toren", res, err)
	key := newKey()
	loose := func(r *playv1.EndLastingEffectRequest) { r.IdempotencyKey = key }
	res, err = a.rpcEnd(t, a.master, e, "loose:"+pensID+":condition:blinded", this, loose)
	s.say("a loose condition off Pensantus", res, err)
	res, err = a.rpcEnd(t, a.master, e, "loose:"+pensID+":condition:blinded", this, loose)
	s.say("the same request again", res, err)
	s.state("after the loose conditions", eventLastingEnded, eventConditionsSet)

	// The effects themselves.
	res, err = a.rpcEnd(t, a.master, e, prone, this)
	s.say("a condition of the master", res, err)
	res, err = a.rpcEnd(t, a.master, e, bless, this)
	s.say("bless on two targets, THIS", res, err)
	res, err = a.rpcEnd(t, a.master, e, hold, group)
	s.say("hold person by concentration group", res, err)
	s.state("after the effects", eventLastingEnded, eventConditionsSet)
	s.want("end_effect")
}

// TestLastingRPCEndTheConcentrationGroup freezes the end of a whole concentration: everything the
// caster held goes with the concentration, and the caster stops concentrating.
func TestLastingRPCEndTheConcentrationGroup(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	a.rpcAdd(t, e, blessKey, []string{"Toren", "Brisa"}, a.rpcCaster(t, "Pensantus"))
	// Pensantus holds one concentration at a time: Hold Person replaces Bless in the rows.
	a.rpcAdd(t, e, holdPerson, []string{"Goblin"}, a.rpcCaster(t, "Pensantus"))
	a.rpcAdd(t, e, heroismKey, []string{"Brisa"}, a.rpcCaster(t, "Goblin"))
	a.rpcAdd(t, e, "spell:bane", []string{"Toren"}, a.rpcCaster(t, "Goblin"), func(r *playv1.AddLastingEffectRequest) { r.Concentration = new(false) })
	s := a.rpcScript(t)
	group := playv1.EffectEndScope_EFFECT_END_SCOPE_CONCENTRATION_GROUP
	s.state("before", eventConditionsSet)
	s.note("concentrating", a.rpcConcentrating(t))
	res, err := a.rpcEnd(t, a.master, e, a.rpcEffectID(t, "spell:bane", "Toren"), group)
	s.say("an effect that is not a concentration", res, err)
	s.note("concentrating", a.rpcConcentrating(t))
	res, err = a.rpcEnd(t, a.master, e, a.rpcEffectID(t, holdPerson, "Goblin"), group)
	s.say("the concentration of Pensantus", res, err)
	s.state("after", eventLastingEnded, eventConditionsSet)
	s.note("concentrating", a.rpcConcentrating(t))
	// The caster no longer concentrates: the group ends and nothing is said about a concentration.
	a.execSQL(t, `UPDATE combatants SET concentration_spell = NULL WHERE label = 'Goblin'`)
	res, err = a.rpcEnd(t, a.master, e, a.rpcEffectID(t, heroismKey, "Brisa"), group)
	s.say("a concentration the caster does not hold", res, err)
	s.state("at the end", eventLastingEnded, eventConditionsSet)
	s.want("end_concentration_group")
}

// rpcConcentrating prints the concentration of each combatant.
func (a *armed) rpcConcentrating(t *testing.T) string {
	t.Helper()
	rows, err := a.h.pool.Query(t.Context(), `SELECT label, COALESCE(concentration_spell, '-') FROM combatants ORDER BY label`)
	if err != nil {
		t.Fatalf("read the concentrations: %v", err)
	}
	var b strings.Builder
	for rows.Next() {
		var label, spell string
		if err := rows.Scan(&label, &spell); err != nil {
			t.Fatalf("scan a concentration: %v", err)
		}
		fmt.Fprintf(&b, "%s concentrates on %s\n", label, spell)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// TestLastingRPCEndingVelocidadeLeavesLethargy freezes what EndLastingEffect answers when the
// effect that ends makes another: created_effect_ids stays empty (a finding: the proto says it
// holds the ids of the lethargy of a Velocidade).
func TestLastingRPCEndingVelocidadeLeavesLethargy(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.closeFight(t)
	a.rpcAdd(t, e, "spell:haste", []string{"Toren"}, a.rpcCaster(t, "Pensantus"))
	s := a.rpcScript(t)
	s.state("before", eventLastingEnded)
	res, err := a.rpcEnd(t, a.master, e, a.rpcEffectID(t, "spell:haste", "Toren"), playv1.EffectEndScope_EFFECT_END_SCOPE_THIS)
	s.say("end haste", res, err)
	s.state("after", eventLastingEnded, eventConditionsSet)
	s.want("end_haste")
}

// TestLastingRPCRemoveATarget freezes RemoveEffectTarget.
func TestLastingRPCRemoveATarget(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.rpcFight(t)
	s := a.rpcScript(t)
	bless, hold := a.rpcEffectID(t, blessKey, "Toren"), a.rpcEffectID(t, holdPerson, "Goblin")
	toren, brisa, goblin := a.id(t, "Toren"), a.id(t, "Brisa"), a.id(t, "Goblin")

	res, err := a.rpcRemove(t, a.caio, e, bless, toren)
	s.say("a player", res, err)
	res, err = a.rpcRemove(t, a.master, e, bless, toren, func(r *playv1.RemoveEffectTargetRequest) { r.IdempotencyKey = "no" })
	s.say("a bad key", res, err)
	res, err = a.rpcRemove(t, a.master, e, bless, toren, func(r *playv1.RemoveEffectTargetRequest) { r.EncounterId = "no" })
	s.say("a bad combat id", res, err)
	res, err = a.rpcRemove(t, a.master, e, "no", toren)
	s.say("a bad effect id", res, err)
	res, err = a.rpcRemove(t, a.master, e, bless, "no")
	s.say("a bad combatant id", res, err)
	res, err = a.rpcRemove(t, a.master, e, newKey(), toren)
	s.say("an effect that is not there", res, err)
	res, err = a.rpcRemove(t, a.master, e, bless, goblin)
	s.say("a combatant that is not a target", res, err)
	res, err = a.rpcRemove(t, a.master, e, bless, newKey())
	s.say("a combatant that is not there", res, err)
	s.state("after the refusals", eventLastingEnded)

	res, err = a.rpcRemove(t, a.master, e, bless, brisa)
	s.say("Brisa off Bless", res, err)
	key := newKey()
	same := func(r *playv1.RemoveEffectTargetRequest) { r.IdempotencyKey = key }
	bless = a.rpcEffectID(t, blessKey, "Toren") // the id of a casting is the lowest of its rows
	res, err = a.rpcRemove(t, a.master, e, bless, toren, same)
	s.say("Toren off Bless, the last target", res, err)
	res, err = a.rpcRemove(t, a.master, e, bless, toren, same)
	s.say("the same request again", res, err)
	res, err = a.rpcRemove(t, a.master, e, hold, goblin)
	s.say("the Goblin off Hold Person", res, err)
	s.state("at the end", eventLastingEnded)
	s.want("remove_target")
}

// TestLastingRPCAfterTheCombatEnded freezes the refusal of every call on a combat that has ended.
func TestLastingRPCAfterTheCombatEnded(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.rpcFight(t)
	bless := a.rpcEffectID(t, blessKey, "Toren")
	toren := a.id(t, "Toren")
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	blocked := playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ENCOUNTER_ENDED
	_, err := a.rpcChange(t, a.master, e, bless, a.rpcChoice(t, rpcDurRounds, 2, "Toren"))
	wantEncounterBlocked(t, err, blocked)
	_, err = a.rpcVisibility(t, a.master, e, bless, nil)
	wantEncounterBlocked(t, err, blocked)
	_, err = a.rpcEnd(t, a.master, e, bless, playv1.EffectEndScope_EFFECT_END_SCOPE_THIS)
	wantEncounterBlocked(t, err, blocked)
	_, err = a.rpcRemove(t, a.master, e, bless, toren)
	wantEncounterBlocked(t, err, blocked)
	_, err = a.master.lasting.AddLastingEffect(t.Context(), connect.NewRequest(&playv1.AddLastingEffectRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey(), CatalogKey: blessKey, TargetIds: []string{toren},
	}))
	wantEncounterBlocked(t, err, blocked)
}
