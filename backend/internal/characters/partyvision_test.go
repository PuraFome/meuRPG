package characters

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/vision"
)

// The derived senses reach the fog as ranges: Devil's Sight is its own range, apart from darkvision, for the map and for the combat's sheet.
func TestSensesOfCarriesDevilsSightApartFromDarkvision(t *testing.T) {
	t.Parallel()
	derived := []rules.Sense{{Key: "darkvision", RangeFt: 60}, {Key: "devils-sight", RangeFt: 120}}
	want := vision.Senses{DarkvisionFt: 60, DevilsSightFt: 120}
	if got := sensesOf(derived); got != want {
		t.Errorf("sensesOf() = %+v, want %+v", got, want)
	}
	if got := senseRanges(derived); got.DarkvisionFt != 60 || got.DevilsSightFt != 120 {
		t.Errorf("senseRanges() = %+v, want darkvision 60 and Devil's Sight 120", got)
	}
	if got := sensesOf([]rules.Sense{{Key: "darkvision", RangeFt: 60}}); got.DevilsSightFt != 0 {
		t.Errorf("a sheet without the invocation has Devil's Sight %d", got.DevilsSightFt)
	}
}
