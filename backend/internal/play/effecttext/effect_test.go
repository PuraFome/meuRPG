package effecttext

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

const fireResistance = "item:potion-of-resistance-fire"

func srd(t *testing.T) *rules.Content {
	t.Helper()
	c, err := rules.LoadSRD()
	if err != nil {
		t.Fatalf("LoadSRD() error = %v", err)
	}
	return c
}

func TestEffectTitleIsThePotionsElseTheSources(t *testing.T) {
	t.Parallel()
	c := srd(t)
	if got := EffectTitle(c, fireResistance, "Poção de resistência"); got != "Resistência a fogo" {
		t.Errorf("EffectTitle(potion) = %q, want the potion's own", got)
	}
	if got := EffectTitle(c, "spell:bless", "Bênção"); got != "Bênção" {
		t.Errorf("EffectTitle(spell) = %q, want the source's", got)
	}
}

func TestTotalDurationIsToldOnlyForALengthTheCatalogGave(t *testing.T) {
	t.Parallel()
	c := srd(t)
	for name, tc := range map[string]struct {
		key, kind    string
		caster, time bool
		want         string
	}{
		"a potion drunk":                    {fireResistance, "item", false, true, "por 1 hora"},
		"a spell cast by a character":       {"spell:bless", "spell", true, true, "por 1 minuto"},
		"a spell the master put on":         {"spell:bless", "spell", false, true, ""},
		"an effect of the master's own":     {"effect:dodging", "master", false, true, ""},
		"an effect with no fixed length":    {fireResistance, "item", false, false, ""},
		"a spell with no length of its own": {"spell:fireball", "spell", true, true, ""},
		"a feature's clock":                 {"feature:wild-shape", "feature", false, true, ""},
	} {
		if got := TotalDuration(c, tc.key, tc.kind, tc.caster, tc.time); got != tc.want {
			t.Errorf("%s: TotalDuration() = %q, want %q", name, got, tc.want)
		}
	}
}

func TestGameTimeSaysWhatIsLeftOrWhyThereIsNoClock(t *testing.T) {
	t.Parallel()
	secs := func(n int32) *int32 { return &n }
	for name, tc := range map[string]struct {
		left          *int32
		concentration bool
		want          string
	}{
		"until dismissed": {nil, false, "Dura até o mestre encerrar"},
		"concentration":   {nil, true, "Dura enquanto quem a conjurou se concentrar"},
		"a minute":        {secs(60), false, "Dura 1 minuto"},
		"seconds":         {secs(54), false, "Dura 54 segundos"},
		"hours":           {secs(8 * 3600), true, "Dura 8 horas"},
		"mixed":           {secs(3600 + 125), false, "Dura 1 hora e 2 minutos e 5 segundos"},
		"nothing left":    {secs(0), false, "Dura menos de um segundo"},
	} {
		if got := GameTime(tc.left, tc.concentration); got != tc.want {
			t.Errorf("%s: GameTime() = %q, want %q", name, got, tc.want)
		}
	}
}
