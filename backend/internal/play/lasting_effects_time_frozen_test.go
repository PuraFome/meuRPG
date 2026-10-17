package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The characterization of AdvanceGameTime: who calls, the key, the seconds, the session, the
// combat that blocks it, the effects that end, and the same request again. See lasting_core_frozen_test.go.

func (a *armed) coreAdvance(t *testing.T, u *user, edit func(*playv1.AdvanceGameTimeRequest)) (*playv1.AdvanceGameTimeResponse, error) {
	t.Helper()
	req := &playv1.AdvanceGameTimeRequest{CampaignId: a.campaignID, IdempotencyKey: newKey(), Seconds: 60}
	edit(req)
	res, err := u.lasting.AdvanceGameTime(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// seconds is the left time of the effect rows of the characters, in the order of the labels.
func (a *armed) coreSecondsLeft(t *testing.T) string {
	t.Helper()
	return a.coreCharacterEffects(t, a.toren.GetId()) + a.coreCharacterEffects(t, a.pens.GetId()) + a.coreCharacterEffects(t, a.bri.GetId())
}

func TestLastingCoreAdvanceGameTime(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	var l coreLog
	try := func(name string, u *user, edit func(*playv1.AdvanceGameTimeRequest)) {
		t.Helper()
		res, err := a.coreAdvance(t, u, edit)
		if err != nil {
			l.say("%s -> %s", name, coreRefusal(err))
			return
		}
		l.say("%s -> ended %d", name, res.GetEffectsEnded())
	}
	none := func(*playv1.AdvanceGameTimeRequest) {}
	try("by a player", a.caio, none)
	try("a bad key", a.master, func(r *playv1.AdvanceGameTimeRequest) { r.IdempotencyKey = "not-a-key" })
	try("0 seconds", a.master, func(r *playv1.AdvanceGameTimeRequest) { r.Seconds = 0 })
	try("-1 second", a.master, func(r *playv1.AdvanceGameTimeRequest) { r.Seconds = -1 })
	try("a day and a second", a.master, func(r *playv1.AdvanceGameTimeRequest) { r.Seconds = 24*3600 + 1 })
	try("a bad key and 0 seconds", a.master, func(r *playv1.AdvanceGameTimeRequest) { r.IdempotencyKey, r.Seconds = "x", 0 })
	// The effects: two end in a minute, one in ten, one stays for the day.
	for _, row := range []struct {
		char    string
		key     string
		seconds int
	}{{a.toren.GetId(), blessKey, 60}, {a.pens.GetId(), guidanceKey, 60}, {a.bri.GetId(), blessKey, 600}, {a.bri.GetId(), longstriderKey, 86400}} {
		a.execSQL(t, `INSERT INTO character_effects (campaign_id, character_id, group_id, source_key, source_kind, duration_kind, seconds_left)
			VALUES ($1, $2, gen_random_uuid(), $3, 'spell', 'rounds', $4)`, a.campaignID, row.char, row.key, row.seconds)
	}
	l.say("before:\n%s", a.coreSecondsLeft(t))
	first := &playv1.AdvanceGameTimeRequest{CampaignId: a.campaignID, IdempotencyKey: newKey(), Seconds: 60}
	for i := range 2 {
		res, err := a.master.lasting.AdvanceGameTime(t.Context(), connect.NewRequest(first))
		if err != nil {
			t.Fatalf("AdvanceGameTime(%d) error = %v", i, err)
		}
		l.say("a minute, call %d -> ended %d\n%s", i+1, res.Msg.GetEffectsEnded(), a.coreSecondsLeft(t))
	}
	// The key again for another amount of time, and for another kind of change.
	_, err := a.coreAdvance(t, a.master, func(r *playv1.AdvanceGameTimeRequest) { r.IdempotencyKey, r.Seconds = first.GetIdempotencyKey(), 61 })
	l.say("the same key for 61 seconds -> %s", coreRefusal(err))
	_, err = a.master.lasting.AdvanceGameTime(t.Context(), connect.NewRequest(&playv1.AdvanceGameTimeRequest{
		CampaignId: a.campaignID, IdempotencyKey: "0123456789abcdef0123456789abcdef", Seconds: 6,
	}))
	l.say("a key with no hyphens -> %s", coreRefusal(err))
	_, err = a.master.lasting.AdvanceGameTime(t.Context(), connect.NewRequest(&playv1.AdvanceGameTimeRequest{
		CampaignId: a.campaignID, IdempotencyKey: "01234567-89ab-cdef-0123-456789abcdef", Seconds: 6,
	}))
	l.say("the same key written with hyphens -> %s", coreRefusal(err))
	// A combat that runs blocks it.
	e := a.closeFight(t)
	try("a combat running", a.master, none)
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	try("the combat ended, a day", a.master, func(r *playv1.AdvanceGameTimeRequest) { r.Seconds = 24 * 3600 })
	l.say("after a day:\n%s", a.coreSecondsLeft(t))
	l.say("events:\n%s", a.coreEvents(t, eventGameTimeAdvanced))
	// No session open.
	a.master.end(t, a.master.liveSession(t, a.campaignID).GetGameSession())
	try("no session open", a.master, none)
	wantLastingGolden(t, "lasting_core", "time_advance", a.coreNames(t, l.String()))
}
