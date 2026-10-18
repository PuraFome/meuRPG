package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// A spell that keeps its area on the map lays it only when the cast chooses a point (origin). A cast without one is
// the spell as it was before the zones: its targets, its effects, its concentration, and no zone. Moonbeam and
// Cloudkill were placed at a point already, so without one they are refused as they were.

func (c *cave) castAt(t *testing.T, u *user, caster, spell string, slot int32, origin *playv1.SpellOrigin, targets ...string) (*playv1.CastSpellResponse, error) {
	t.Helper()
	return c.castArea(t, u, caster, spell, slotOfLevel(slot), origin, nil, func(r *playv1.CastSpellRequest) {
		for _, l := range targets {
			r.Targets = append(r.Targets, &playv1.SpellTarget{CombatantId: c.id(t, l)})
		}
	})
}

func TestWebCastWithoutAnOriginIsTheWebAsBeforeAndLaysNoZone(t *testing.T) {
	t.Parallel()
	c := zoneCave(t)
	e := c.zoneFight(t)
	c.passTo(t, e, "Pensantus")
	if _, err := c.castAt(t, c.ana, "Pensantus", webKey, 2, nil, "Ogro"); err != nil {
		t.Fatalf("CastSpell(Web) with only targets error = %v", err)
	}
	enc := c.get(t, c.master)
	if len(enc.GetZones()) != 0 {
		t.Errorf("zones = %v, want none: no origin was chosen", enc.GetZones())
	}
	if cardOf(t, byLabel(t, enc, "Ogro"), webKey) == nil {
		t.Error("the ogre has no Web card: the spell's effect on its target is as it was")
	}
	if !strings.Contains(asJSONAny(t, byLabel(t, enc, "Pensantus")), webKey) {
		t.Error("Pensantus does not concentrate on Web")
	}
}

func TestWebCastWithAnOriginLaysTheZone(t *testing.T) {
	t.Parallel()
	c := zoneCave(t)
	e := c.zoneFight(t)
	c.passTo(t, e, "Pensantus")
	if _, err := c.castAt(t, c.ana, "Pensantus", webKey, 2, at(9, 7)); err != nil {
		t.Fatalf("CastSpell(Web) with an origin error = %v", err)
	}
	if len(zonesOf(c.get(t, c.master), webKey)) != 1 {
		t.Error("no zone after a cast with an origin")
	}
}

func TestSpiritGuardiansCastWithoutAnOriginIsTheSpellAsBeforeAndLaysNoZone(t *testing.T) {
	t.Parallel()
	c := zoneCave(t)
	c.zoneFight(t)
	c.h.roller.queue(5)
	res, err := c.castAt(t, c.caio, "Toren", spiritGuardiansKey, 3, nil, "Ogro")
	if err != nil {
		t.Fatalf("CastSpell(Spirit Guardians) with only targets error = %v", err)
	}
	if len(c.get(t, c.master).GetZones()) != 0 {
		t.Error("a zone was laid without an origin")
	}
	if tg := res.GetCast().GetTargets(); len(tg) != 1 || tg[0].GetSave() == nil || tg[0].GetPendingDamageId() == "" {
		t.Errorf("targets = %v, want the ogre's save and damage at the cast, as before the zones", tg)
	}
}

func TestSpiritGuardiansCastWithAnOriginLaysTheZoneAroundTheCaster(t *testing.T) {
	t.Parallel()
	c := zoneCave(t)
	c.zoneFight(t)
	if _, err := c.castAt(t, c.caio, "Toren", spiritGuardiansKey, 3, at(5, 7)); err != nil {
		t.Fatalf("CastSpell(Spirit Guardians) with an origin error = %v", err)
	}
	if z := c.oneZone(t, c.master, spiritGuardiansKey); z.GetOrigin().GetCol() != 5 {
		t.Errorf("zone = %v, want it centered on the caster at (5,7)", z)
	}
}

func TestMoonbeamAndCloudkillWithoutAnOriginAreRefusedAsBefore(t *testing.T) {
	t.Parallel()
	c := zoneCave(t)
	e := c.zoneFight(t)
	c.passTo(t, e, "Pensantus")
	_, err := c.castAt(t, c.ana, "Pensantus", cloudkillKey, 5, nil, "Ogro")
	if connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), "placed at a point") {
		t.Errorf("Cloudkill without an origin: error = %v, want invalid_argument, placed at a point", err)
	}
	if _, err := c.castAt(t, c.ana, "Pensantus", cloudkillKey, 5, at(16, 7)); err != nil {
		t.Fatalf("Cloudkill with an origin error = %v", err)
	}
	if len(zonesOf(c.get(t, c.master), cloudkillKey)) != 1 {
		t.Error("no cloud after a cast with an origin")
	}
	d := druidCave(t)
	d.zoneFight(t)
	if _, err := d.castAt(t, d.caio, "Toren", moonbeamKey, 2, nil, "Ogro"); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("Moonbeam without an origin: error = %v, want invalid_argument", err)
	}
}

func TestATableZoneSpellWithoutAnOriginIsTheSpellAsBeforeAndWithOneLaysTheZone(t *testing.T) {
	t.Parallel()
	c := tableZoneCave(t)
	e := c.zoneFight(t)
	c.passTo(t, e, "Pensantus")
	c.h.roller.queue(5)
	if _, err := c.castAt(t, c.ana, "Pensantus", tableFollowing, 2, nil); err != nil {
		t.Fatalf("a table zone spell with no origin error = %v", err)
	}
	if len(c.get(t, c.master).GetZones()) != 0 {
		t.Error("a zone was laid without an origin")
	}
	c.nextTurnOf(t, "Pensantus")
	if _, err := c.castAt(t, c.ana, "Pensantus", tableFollowing, 2, at(12, 7)); err != nil {
		t.Fatalf("a table zone spell with an origin error = %v", err)
	}
	if len(zonesOf(c.get(t, c.master), tableFollowing)) != 1 {
		t.Error("no zone after a cast with an origin")
	}
}

// The request the e2e specs send: Web with an empty list of targets and no point.
func TestWebCastWithNoTargetsAndNoOriginConcentrates(t *testing.T) {
	t.Parallel()
	c := zoneCave(t)
	e := c.zoneFight(t)
	c.passTo(t, e, "Pensantus")
	if _, err := c.castAt(t, c.ana, "Pensantus", webKey, 2, nil); err != nil {
		t.Fatalf("CastSpell(Web) with no targets and no origin error = %v", err)
	}
	enc := c.get(t, c.master)
	if len(enc.GetZones()) != 0 || !strings.Contains(asJSONAny(t, byLabel(t, enc, "Pensantus")), webKey) {
		t.Errorf("zones = %v; want none, and Pensantus concentrating on Web", enc.GetZones())
	}
}
