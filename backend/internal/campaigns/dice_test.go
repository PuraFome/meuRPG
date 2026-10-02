package campaigns

import (
	"testing"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
)

func TestEffectiveDiceMode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		mode DiceMode
		pref DicePreference
		want RollsIn
	}{
		{DiceModePlayersChoose, DicePreferenceApp, RollsInApp},
		{DiceModePlayersChoose, DicePreferencePhysical, RollsPhysical},
		{DiceModeApp, DicePreferenceApp, RollsInApp},
		{DiceModeApp, DicePreferencePhysical, RollsInApp}, // forced: the choice is ignored
		{DiceModePhysical, DicePreferenceApp, RollsPhysical},
		{DiceModePhysical, DicePreferencePhysical, RollsPhysical},
		{DiceModePlayersChoose, "", RollsInApp}, // no preference: the default
	}
	for _, tt := range tests {
		if got := EffectiveDiceMode(tt.mode, tt.pref); got != tt.want {
			t.Errorf("EffectiveDiceMode(%q, %q) = %q, want %q", tt.mode, tt.pref, got, tt.want)
		}
	}
}

// TestRN18_DiceSettings follows RN-18 from the defaults to the effective
// mode a player gets.
func TestRN18_DiceSettings(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	player := h.newUser("Jogador")
	other := h.newUser("Outro")
	campaign := master.createCampaign(t, "Mirathel")
	id := campaign.GetId()
	_, token := master.createInvite(t, id, MaxInviteUses, 0)
	player.join(t, token)
	other.join(t, token)
	ctx := t.Context()

	get := func(u *user) *campaignsv1.Campaign {
		t.Helper()
		res, err := u.api.GetCampaign(ctx, connect.NewRequest(&campaignsv1.GetCampaignRequest{CampaignId: id}))
		if err != nil {
			t.Fatalf("GetCampaign() error = %v", err)
		}
		return res.Msg.GetCampaign()
	}
	effective := func(u *user) RollsIn {
		t.Helper()
		got, err := h.service.PlayerDiceMode(ctx, id, u.id)
		if err != nil {
			t.Fatalf("PlayerDiceMode() error = %v", err)
		}
		return got
	}

	// Defaults: players choose, everyone prefers the app.
	if c := get(player); c.GetDiceMode() != campaignsv1.DiceMode_DICE_MODE_PLAYERS_CHOOSE || c.GetMyDicePreference() != campaignsv1.DicePreference_DICE_PREFERENCE_APP {
		t.Errorf("defaults = %v / %v, want players_choose / app", c.GetDiceMode(), c.GetMyDicePreference())
	}
	if got := effective(player); got != RollsInApp {
		t.Errorf("default effective mode = %q, want app", got)
	}

	// A player chooses their own dice; the master sees it, another player
	// does not, and the choice counts while players choose.
	res, err := player.api.SetMyDicePreference(ctx, connect.NewRequest(&campaignsv1.SetMyDicePreferenceRequest{
		CampaignId: id, Preference: campaignsv1.DicePreference_DICE_PREFERENCE_PHYSICAL,
	}))
	if err != nil || res.Msg.GetPreference() != campaignsv1.DicePreference_DICE_PREFERENCE_PHYSICAL {
		t.Fatalf("SetMyDicePreference() = %v, %v, want physical", res, err)
	}
	if got := get(player).GetMyDicePreference(); got != campaignsv1.DicePreference_DICE_PREFERENCE_PHYSICAL {
		t.Errorf("GetCampaign my_dice_preference = %v, want physical", got)
	}
	if got := effective(player); got != RollsPhysical {
		t.Errorf("effective mode = %q, want physical", got)
	}
	if got := effective(other); got != RollsInApp {
		t.Errorf("another player's effective mode = %q, want app", got)
	}
	members := func(u *user) map[string]campaignsv1.DicePreference {
		t.Helper()
		res, err := u.api.ListMembers(ctx, connect.NewRequest(&campaignsv1.ListMembersRequest{CampaignId: id}))
		if err != nil {
			t.Fatalf("ListMembers() error = %v", err)
		}
		out := map[string]campaignsv1.DicePreference{}
		for _, m := range res.Msg.GetMembers() {
			out[m.GetUserId()] = m.GetDicePreference()
		}
		return out
	}
	if got := members(master)[player.id]; got != campaignsv1.DicePreference_DICE_PREFERENCE_PHYSICAL {
		t.Errorf("master sees preference %v, want physical", got)
	}
	if got := members(other)[player.id]; got != campaignsv1.DicePreference_DICE_PREFERENCE_UNSPECIFIED {
		t.Errorf("a player sees preference %v, want it hidden", got)
	}

	// The master forces the app: the preference is kept but ignored.
	mres, err := master.api.SetCampaignDiceMode(ctx, connect.NewRequest(&campaignsv1.SetCampaignDiceModeRequest{
		CampaignId: id, Mode: campaignsv1.DiceMode_DICE_MODE_APP,
	}))
	if err != nil || mres.Msg.GetMode() != campaignsv1.DiceMode_DICE_MODE_APP {
		t.Fatalf("SetCampaignDiceMode() = %v, %v, want app", mres, err)
	}
	if got := get(player); got.GetDiceMode() != campaignsv1.DiceMode_DICE_MODE_APP || got.GetMyDicePreference() != campaignsv1.DicePreference_DICE_PREFERENCE_PHYSICAL {
		t.Errorf("after forcing the app: %v / %v, want app / physical kept", got.GetDiceMode(), got.GetMyDicePreference())
	}
	if got := effective(player); got != RollsInApp {
		t.Errorf("effective mode with the app forced = %q, want app", got)
	}

	// Forcing physical applies to a player who prefers the app too.
	if _, err := master.api.SetCampaignDiceMode(ctx, connect.NewRequest(&campaignsv1.SetCampaignDiceModeRequest{
		CampaignId: id, Mode: campaignsv1.DiceMode_DICE_MODE_PHYSICAL,
	})); err != nil {
		t.Fatalf("SetCampaignDiceMode() error = %v", err)
	}
	if got := effective(other); got != RollsPhysical {
		t.Errorf("effective mode with physical forced = %q, want physical", got)
	}

	// The master has a preference too.
	if _, err := master.api.SetMyDicePreference(ctx, connect.NewRequest(&campaignsv1.SetMyDicePreferenceRequest{
		CampaignId: id, Preference: campaignsv1.DicePreference_DICE_PREFERENCE_PHYSICAL,
	})); err != nil {
		t.Errorf("the master's SetMyDicePreference() error = %v", err)
	}
}

func TestSetDiceValidation(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	id := master.createCampaign(t, "Mirathel").GetId()
	ctx := t.Context()

	for _, mode := range []campaignsv1.DiceMode{campaignsv1.DiceMode_DICE_MODE_UNSPECIFIED, campaignsv1.DiceMode(99)} {
		_, err := master.api.SetCampaignDiceMode(ctx, connect.NewRequest(&campaignsv1.SetCampaignDiceModeRequest{CampaignId: id, Mode: mode}))
		wantCode(t, "SetCampaignDiceMode()", err, connect.CodeInvalidArgument)
	}
	for _, pref := range []campaignsv1.DicePreference{campaignsv1.DicePreference_DICE_PREFERENCE_UNSPECIFIED, campaignsv1.DicePreference(99)} {
		_, err := master.api.SetMyDicePreference(ctx, connect.NewRequest(&campaignsv1.SetMyDicePreferenceRequest{CampaignId: id, Preference: pref}))
		wantCode(t, "SetMyDicePreference()", err, connect.CodeInvalidArgument)
	}
}

// TestDiceEnumsMatchTheDatabase checks that every enum value has a database
// value and maps back, like the XP modes.
func TestDiceEnumsMatchTheDatabase(t *testing.T) {
	t.Parallel()
	for mode, dbValue := range diceModeToDB {
		if diceModeFromDB[dbValue] != mode {
			t.Errorf("%v -> %q does not map back", mode, dbValue)
		}
	}
	for pref, dbValue := range dicePreferenceToDB {
		if dicePreferenceFromDB[dbValue] != pref {
			t.Errorf("%v -> %q does not map back", pref, dbValue)
		}
	}
	if len(diceModeToDB) != 3 || len(dicePreferenceToDB) != 2 {
		t.Errorf("want 3 modes and 2 preferences, have %d and %d", len(diceModeToDB), len(dicePreferenceToDB))
	}
}
