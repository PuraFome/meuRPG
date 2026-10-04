package rules

import (
	"fmt"
	"io/fs"
	"slices"
	"strings"
)

// Light presets (MR-036, Etapa 9, D6): the radii of the SRD 5.1's lights, from
// its equipment and spell texts, in effects/lights.json, written by hand and
// checked by the loader. They fill the master's form for a light on the map
// and the light a character carries. The bullseye lantern, which lights a
// cone, is left out: the app lights circles only.

// LightPreset is a source of light the SRD describes.
type LightPreset struct {
	// Key is "light:<name>"; NamePT is its Portuguese name.
	Key    string
	NamePT string
	// BrightFt is the radius of its bright light, and DimFt how much further
	// its dim light goes ("bright light in a 20-foot radius and dim light for
	// an additional 20 feet" is 20 and 20), both in feet.
	BrightFt, DimFt int
	// DurationPT says how long it lasts, as text for the master.
	DurationPT string
}

type lightsFile struct {
	Lights []struct {
		Key        string `json:"key"`
		BrightFt   int    `json:"bright_ft"`
		DimFt      int    `json:"dim_ft"`
		DurationPT string `json:"duration_pt"`
	} `json:"lights"`
}

// loadLights reads and checks effects/lights.json.
func (c *content) loadLights(fsys fs.FS) error {
	const name = "effects/lights.json"
	var f lightsFile
	if err := readJSON(fsys, name, &f); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, in := range f.Lights {
		key := "light:" + in.Key
		switch {
		case in.Key == "" || strings.ContainsAny(in.Key, " :") || seen[key]:
			return fmt.Errorf("%s: light key %q is empty, has a space or colon, or is repeated", name, in.Key)
		case in.BrightFt < 0 || in.DimFt < 0 || in.BrightFt+in.DimFt == 0:
			return fmt.Errorf("%s: %s needs a radius: bright_ft and dim_ft can't be negative, nor both 0", name, key)
		case in.BrightFt%5 != 0 || in.DimFt%5 != 0:
			return fmt.Errorf("%s: %s: the radii are whole squares of 5 ft", name, key)
		case strings.TrimSpace(in.DurationPT) == "":
			return fmt.Errorf("%s: %s has no duration_pt", name, key)
		case c.namesPT[key] == "":
			return fmt.Errorf("%s: %s has no Portuguese name in effects/names_pt.json", name, key)
		}
		seen[key] = true
		c.lights = append(c.lights, LightPreset{Key: key, NamePT: c.namesPT[key], BrightFt: in.BrightFt, DimFt: in.DimFt, DurationPT: in.DurationPT})
	}
	for k := range c.namesPT {
		if strings.HasPrefix(k, "light:") && !seen[k] {
			return fmt.Errorf("effects/names_pt.json names %q, which is not in lights.json", k)
		}
	}
	return nil
}

// LightPresets returns the SRD's lights (Vela, Tocha, Lâmpada, Lanterna
// coberta, a magia Luz, Chama Contínua, Luz do Dia), in the order of
// effects/lights.json. The caller gets a copy.
func (c *Content) LightPresets() []LightPreset {
	return slices.Clone(c.c.lights)
}

// LightPreset returns the preset with a key such as "light:torch".
func (c *Content) LightPreset(key string) (LightPreset, bool) {
	for _, l := range c.c.lights {
		if l.Key == key {
			return l, true
		}
	}
	return LightPreset{}, false
}
