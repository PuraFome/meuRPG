package play

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The effects on the characters outside a combat (character_effects): how each viewer reads them,
// how the master ends them, how game time and a long rest take them away, and how they move into a
// combat and back out of it (RN-22).

// rpcGive has the master put an effect on one character, with the edits.
func (a *armed) rpcGive(t *testing.T, key string, who *charactersv1.Character, edit ...func(*playv1.AddCharacterEffectRequest)) string {
	t.Helper()
	return a.mustGiveEffect(t, key, []*charactersv1.Character{who}, edit...)
}

// rpcSeconds is a duration of rounds in game time.
func rpcSeconds(n int32) func(*playv1.AddCharacterEffectRequest) {
	return func(r *playv1.AddCharacterEffectRequest) {
		r.DurationKind, r.Seconds = playv1.EffectDurationKind_EFFECT_DURATION_KIND_ROUNDS, n
	}
}

// rpcCharDuration is a duration of another kind.
func rpcCharDuration(kind playv1.EffectDurationKind) func(*playv1.AddCharacterEffectRequest) {
	return func(r *playv1.AddCharacterEffectRequest) { r.DurationKind = kind }
}

// rpcCharVisible makes the effect visible or not, to an audience, with a label.
func rpcCharVisible(visible bool, audience playv1.EffectAudience, label string) func(*playv1.AddCharacterEffectRequest) {
	return func(r *playv1.AddCharacterEffectRequest) {
		r.PlayerVisible, r.Audience, r.PlayerLabel = &visible, audience, label
	}
}

// rpcTempHP gives the character temporary hit points.
func (a *armed) rpcTempHP(t *testing.T, c *charactersv1.Character, n int32) {
	t.Helper()
	a.correct(t, c, func(r *playv1.AdjustCharacterVitalsRequest) { r.HitPointsTemporary = &n })
}

// rpcOutsideParty puts effects of every kind on the party outside a combat.
func (a *armed) rpcOutsideParty(t *testing.T) {
	t.Helper()
	a.mustCastOut(t, a.bia, a.bri, blessKey, slotOfLevel(1), false, []*charactersv1.Character{a.toren, a.pens})
	a.mustCastOut(t, a.ana, a.pens, mageArmorKey, slotOfLevel(1), false, []*charactersv1.Character{a.pens})
	all, owner := playv1.EffectAudience_EFFECT_AUDIENCE_ALL, playv1.EffectAudience_EFFECT_AUDIENCE_OWNER
	a.rpcGive(t, heroismKey, a.bri, rpcSeconds(120))
	a.rpcGive(t, guidanceKey, a.pens)
	a.rpcGive(t, enhanceAbilityKey, a.toren, func(r *playv1.AddCharacterEffectRequest) { r.AbilityKey = "str" })
	a.rpcGive(t, longstriderKey, a.toren, rpcCharDuration(playv1.EffectDurationKind_EFFECT_DURATION_KIND_LONG_REST))
	a.rpcGive(t, "condition:poisoned", a.toren, rpcCharVisible(false, all, ""))
	a.rpcGive(t, "condition:blinded", a.pens, rpcCharVisible(true, owner, "Cegueira"), rpcSeconds(61))
	a.rpcGive(t, "condition:prone", a.bri, rpcCharVisible(true, all, "No chão"), rpcSeconds(3661))
	a.rpcGive(t, "condition:charmed", a.bri, rpcSeconds(1))
	a.rpcGive(t, "condition:deafened", a.pens, rpcSeconds(7200))
}

// rpcSortedCharacterEffects lists the effects of the campaign as u reads them, in the order of
// the character and the key (the rows of one casting are made at one instant).
func (a *armed) rpcSortedCharacterEffects(t *testing.T, u *user) *playv1.ListCharacterEffectsResponse {
	t.Helper()
	res, err := u.lasting.ListCharacterEffects(t.Context(), connect.NewRequest(&playv1.ListCharacterEffectsRequest{CampaignId: a.campaignID}))
	if err != nil {
		t.Fatalf("ListCharacterEffects() error = %v", err)
	}
	slices.SortStableFunc(res.Msg.Effects, func(x, y *playv1.CharacterEffect) int {
		return strings.Compare(x.GetCharacterName()+x.GetSourceKey(), y.GetCharacterName()+y.GetSourceKey())
	})
	return res.Msg
}

// TestLastingRPCListsTheEffectsOnTheCharacters freezes what each viewer reads of the effects on
// the characters outside a combat.
func TestLastingRPCListsTheEffectsOnTheCharacters(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	a.rpcOutsideParty(t)
	for name, u := range map[string]*user{"master": a.master, "toren": a.caio, "pens": a.ana, "bri": a.bia} {
		effects := a.rpcSortedCharacterEffects(t, u)
		got := a.rpcJSON(t, effects)
		if name == "master" { // the master's rows are the same cards as the owners'; keep the rules only
			got = a.rpcRuleJSON(t, effects)
		}
		wantLastingGolden(t, "lasting_rpc", "characters_"+name, got)
	}
	// A character that is gone from the campaign's table (reserved or deleted) is read by nobody.
	a.execSQL(t, `INSERT INTO character_effects (campaign_id, character_id, group_id, source_key, source_kind, duration_kind)
		SELECT campaign_id, id, gen_random_uuid(), 'effect:LEAKCANARY', 'master', 'until_dismissed' FROM characters WHERE name = 'Goblin'`)
	if got := len(a.rpcSortedCharacterEffects(t, a.master).GetEffects()); got == 0 {
		t.Fatalf("the master reads no effect")
	}
	_, err := a.h.anonymous().lasting.ListCharacterEffects(t.Context(), connect.NewRequest(&playv1.ListCharacterEffectsRequest{CampaignId: a.campaignID}))
	wantCode(t, "a stranger lists the effects", err, connect.CodeUnauthenticated)
}

// TestLastingRPCEndsAnEffectOnACharacter freezes EndLastingEffect outside a combat.
func TestLastingRPCEndsAnEffectOnACharacter(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	a.rpcOutsideParty(t)
	a.rpcTempHP(t, a.bri, 6)
	s := a.rpcScript(t)
	e := &playv1.Encounter{} // no combat: the call has no encounter id
	this, group := playv1.EffectEndScope_EFFECT_END_SCOPE_THIS, playv1.EffectEndScope_EFFECT_END_SCOPE_CONCENTRATION_GROUP
	effectOf := func(key, who string) string {
		for _, f := range a.rpcSortedCharacterEffects(t, a.master).GetEffects() {
			if f.GetSourceKey() == key && f.GetCharacterName() == who {
				return f.GetId()
			}
		}
		t.Fatalf("%s has no %s", who, key)
		return ""
	}
	end := func(id string, scope playv1.EffectEndScope, edit ...func(*playv1.EndLastingEffectRequest)) (*playv1.EndLastingEffectResponse, error) {
		return a.rpcEnd(t, a.master, e, id, scope, edit...)
	}

	guidance := effectOf(guidanceKey, "Pensantus")
	res, err := a.rpcEnd(t, a.caio, e, guidance, this)
	s.say("a player", res, err)
	res, err = end(guidance, this, func(r *playv1.EndLastingEffectRequest) { r.IdempotencyKey = "no" })
	s.say("a bad key", res, err)
	res, err = end(guidance, playv1.EffectEndScope_EFFECT_END_SCOPE_UNSPECIFIED)
	s.say("a scope unspecified", res, err)
	res, err = end("no", this)
	s.say("a bad effect id", res, err)
	res, err = end(newKey(), this)
	s.say("an effect that is not there", res, err)
	s.note("rows before", a.rpcCharRows(t))

	// The temporary hit points the spell gave go with it.
	res, err = end(effectOf(heroismKey, "Brisa"), this)
	s.say("Heroism, THIS", res, err)
	s.note("Brisa's temporary hit points", a.vitalsLine(t, a.bri))
	key := newKey()
	same := func(r *playv1.EndLastingEffectRequest) { r.IdempotencyKey = key }
	res, err = end(guidance, this, same)
	s.say("Guidance, THIS", res, err)
	res, err = end(guidance, this, same)
	s.say("the same request again", res, err)
	res, err = end(guidance, this)
	s.say("Guidance again, with a key of its own", res, err)
	// A casting ends whole, and the cast that held it with it.
	cast := a.casts(t, a.bia).GetActive()
	s.note("casts before", castLines(cast))
	res, err = end(effectOf(blessKey, "Pensantus"), group)
	s.say("Bless, CONCENTRATION_GROUP", res, err)
	s.note("casts after", castLines(a.casts(t, a.bia).GetActive())+"log: "+castLines(a.casts(t, a.bia).GetLog()))
	res, err = end(effectOf(mageArmorKey, "Pensantus"), group)
	s.say("Mage Armor, CONCENTRATION_GROUP (not a concentration)", res, err)
	s.note("rows after", a.rpcCharRows(t))
	s.note("events", a.rpcEvents(t, eventLastingEnded))
	s.want("outside_end")
}

// castLines prints the casts: the spell and the status.
func castLines(casts []*playv1.OutsideCast) string {
	var b strings.Builder
	for _, c := range casts {
		b.WriteString(c.GetSpellKey() + " " + c.GetStatus().String() + "\n")
	}
	return b.String()
}

// vitalsLine prints the temporary hit points of a character.
func (a *armed) vitalsLine(t *testing.T, c *charactersv1.Character) string {
	t.Helper()
	v := a.vitals(t, c)
	return fmt.Sprintf("%s has %d hit points and %d temporary\n", c.GetName(), v.GetHitPointsCurrent(), v.GetHitPointsTemporary())
}

// TestLastingRPCEndingWithNoOpenSession freezes the refusal of ending an effect on a character
// when the table has no open game session.
func TestLastingRPCEndingWithNoOpenSession(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	id := a.rpcGive(t, guidanceKey, a.toren)
	session := a.master.liveSession(t, a.campaignID).GetGameSession()
	a.master.end(t, session)
	_, err := a.rpcEnd(t, a.master, &playv1.Encounter{}, id, playv1.EffectEndScope_EFFECT_END_SCOPE_THIS)
	wantCode(t, "end an effect with no open session", err, connect.CodeFailedPrecondition)
}
