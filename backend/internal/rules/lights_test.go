package rules

import (
	"testing"
	"testing/fstest"
)

func TestLightPresetsOfTheSRD(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	// Radii in feet, "bright light in a N-foot radius and dim light for an
	// additional M feet" is N and M (5e-database equipment and spells).
	want := []LightPreset{
		{Key: "light:candle", NamePT: "Vela", BrightFt: 5, DimFt: 5, DurationPT: "1 hora"},
		{Key: "light:torch", NamePT: "Tocha", BrightFt: 20, DimFt: 20, DurationPT: "1 hora"},
		{Key: "light:lamp", NamePT: "Lâmpada", BrightFt: 15, DimFt: 30, DurationPT: "6 horas por frasco de óleo"},
		{Key: "light:hooded-lantern", NamePT: "Lanterna coberta", BrightFt: 30, DimFt: 30, DurationPT: "6 horas por frasco de óleo"},
		{Key: "light:light-spell", NamePT: "Luz", BrightFt: 20, DimFt: 20, DurationPT: "1 hora"},
		{Key: "light:continual-flame", NamePT: "Chama Contínua", BrightFt: 20, DimFt: 20, DurationPT: "até ser dissipada"},
		{Key: "light:daylight", NamePT: "Luz do Dia", BrightFt: 60, DimFt: 60, DurationPT: "1 hora"},
	}
	got := c.LightPresets()
	if len(got) != len(want) {
		t.Fatalf("LightPresets() has %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("LightPresets()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
	if p, ok := c.LightPreset("light:torch"); !ok || p.NamePT != "Tocha" {
		t.Errorf("LightPreset(torch) = %+v, %v", p, ok)
	}
	// The bullseye lantern lights a cone, which the app does not draw.
	if _, ok := c.LightPreset("light:bullseye-lantern"); ok {
		t.Error("the bullseye lantern is left out")
	}
	got[0].NamePT = "x"
	if c.LightPresets()[0].NamePT == "x" {
		t.Error("LightPresets shares its slice")
	}
}

func TestLoadLightsRefuses(t *testing.T) {
	t.Parallel()
	load := func(body string) error {
		c := &content{namesPT: map[string]string{"light:a": "A"}}
		return c.loadLights(fstest.MapFS{"effects/lights.json": {Data: []byte(body)}})
	}
	if err := load(`{"lights":[{"key":"a","bright_ft":20,"dim_ft":20,"duration_pt":"1 hora"}]}`); err != nil {
		t.Fatalf("a good file: %v", err)
	}
	bad := map[string]string{
		"a negative radius":    `{"lights":[{"key":"a","bright_ft":-5,"dim_ft":20,"duration_pt":"1 hora"}]}`,
		"a negative dim":       `{"lights":[{"key":"a","bright_ft":20,"dim_ft":-5,"duration_pt":"1 hora"}]}`,
		"no radius at all":     `{"lights":[{"key":"a","bright_ft":0,"dim_ft":0,"duration_pt":"1 hora"}]}`,
		"half a square":        `{"lights":[{"key":"a","bright_ft":7,"dim_ft":5,"duration_pt":"1 hora"}]}`,
		"no duration":          `{"lights":[{"key":"a","bright_ft":20,"dim_ft":20,"duration_pt":""}]}`,
		"no Portuguese name":   `{"lights":[{"key":"b","bright_ft":20,"dim_ft":20,"duration_pt":"1 hora"}]}`,
		"a repeated key":       `{"lights":[{"key":"a","bright_ft":20,"dim_ft":20,"duration_pt":"1"},{"key":"a","bright_ft":5,"dim_ft":5,"duration_pt":"1"}]}`,
		"an unknown field":     `{"lights":[{"key":"a","bright_ft":20,"dim_ft":20,"duration_pt":"1 hora","cone":true}]}`,
		"a name with no light": `{"lights":[]}`,
	}
	for name, body := range bad {
		if err := load(body); err == nil {
			t.Errorf("%s: the loader accepted it", name)
		}
	}
}
