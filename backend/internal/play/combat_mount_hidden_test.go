package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// mountOptionsOf is TurnOptions.mount of the combatant as u reads it.
func (c *cave) mountOptionsOf(t *testing.T, u *user, label string) *rulesv1.MountOptions {
	t.Helper()
	res, err := u.combat.GetTurnOptions(t.Context(), connect.NewRequest(&playv1.GetTurnOptionsRequest{CampaignId: c.campaignID, EncounterId: c.get(t, c.master).GetId(), CombatantId: c.id(t, label)}))
	if err != nil {
		t.Fatalf("GetTurnOptions(%s) error = %v", label, err)
	}
	return res.Msg.GetOptions().GetMount()
}

// offered says whether the mount options list the combatant as a mount.
func (c *cave) offered(t *testing.T, o *rulesv1.MountOptions, label string) *rulesv1.MountChoice {
	t.Helper()
	for _, m := range o.GetMounts() {
		if m.GetCombatantId() == c.id(t, label) {
			return m
		}
	}
	return nil
}

// TestMountedCombat_AHiddenRiderOnAVisibleMountLeaksNothing: a player who tries to mount a pony that a hidden creature rides, or whose
// own mount rides a hidden creature, is answered as if the pair did not exist: the same refusal as for a combatant that is no mount,
// no id of the hidden one, and the options do not list the mount. The master reads the real refusal (the positive control).
func TestMountedCombat_AHiddenRiderOnAVisibleMountLeaksNothing(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		setup  func(t *testing.T, c *cave) // the hidden creature rides the Ogro, or the Ogro rides it
		reason playv1.EncounterBlockedReason
	}{
		"a hidden rider on the mount": {func(t *testing.T, c *cave) { c.sitOn(t, "Goblin 1", "Ogro") }, reasonMountTaken},
		"a mount that rides a hidden mount": {func(t *testing.T, c *cave) {
			c.mustMove(t, c.master, "Goblin 1", 8, 7)
			c.sitOn(t, "Ogro", "Goblin 1")
		}, reasonAlreadyRiding},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := newCave(t)
			c.fight(t)
			c.steed(t)
			tc.setup(t, c)
			c.hide(t, "Goblin 1")
			hidden := c.id(t, "Goblin 1")

			_, err := c.mountAs(t, c.master, "Toren", "Ogro")
			b := wantBlockedBy(t, "the master mounting the pair's mount", err, tc.reason)
			if len(b.GetCombatantIds()) != 1 {
				t.Errorf("the master's refusal names %v, want one combatant", b.GetCombatantIds())
			}
			if m := c.offered(t, c.mountOptionsOf(t, c.master, "Toren"), "Ogro"); m == nil || m.GetRefusal() == rulesv1.MountRefusal_MOUNT_REFUSAL_UNSPECIFIED {
				t.Errorf("the master's options list the Ogro as %v, want it with a refusal", m)
			}

			_, err = c.mountAs(t, c.caio, "Toren", "Ogro")
			b = wantBlockedBy(t, "a player mounting the pair's mount", err, reasonNotAMount)
			for _, id := range b.GetCombatantIds() {
				if id == hidden {
					t.Errorf("the refusal names the hidden combatant %s", id)
				}
			}
			if m := c.offered(t, c.mountOptionsOf(t, c.caio, "Toren"), "Ogro"); m != nil {
				t.Errorf("a player's options list the Ogro (%v) while a hidden creature is in its pair", m)
			}
		})
	}
}
