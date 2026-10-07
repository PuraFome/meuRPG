package leaktest

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// vision reads the fog map's view of p: a player gets their own character's.
func (w *world) vision(p *person, mapID string) *mapsv1.GetMapVisionResponse {
	return must(p.maps.GetMapVision(w.t.Context(), connect.NewRequest(&mapsv1.GetMapVisionRequest{CampaignId: w.campaign, MapId: mapID})))
}

// picture draws a view: B bright, d dim, g grey, # a wall seen, r remembered, a
// space unseen (the oracle's alphabet, see maps/fog_test.go).
func picture(res *mapsv1.GetMapVisionResponse) []string {
	cols, rows := int(res.GetGridColumns()), int(res.GetGridRows())
	var out []string
	for row := range rows {
		var b strings.Builder
		for col := range cols {
			i := row*cols + col
			code := res.GetStates()[i/2] >> (4 * (i % 2)) & 0xf
			ch, ok := map[byte]byte{4: 'B', 3: 'd', 2: 'g', 1: '#', 5: 'r'}[code]
			if !ok {
				ch = ' '
			}
			b.WriteByte(ch)
		}
		out = append(out, b.String())
	}
	return out
}

// sees says whether the view shows a square (seen now or remembered).
func sees(res *mapsv1.GetMapVisionResponse, sq grid.Square) bool {
	cols := int(res.GetGridColumns())
	i := sq.Row*cols + sq.Col
	code := res.GetStates()[i/2] >> (4 * (i % 2)) & 0xf
	return code >= 2
}

// assertFogShape checks what the canaries of the fog map assume: which squares Ana's character
// sees, which Caio's sees, and that nobody sees the guard room. A leak test whose fixture drifted
// (a wall moved, a character stands elsewhere) would call a legitimate read a leak, or a leak
// legitimate, so it stops here with a clear message instead.
func (w *world) assertFogShape(t *testing.T) {
	t.Helper()
	ana, caio := w.vision(w.ana, w.fogMap), w.vision(w.caio, w.fogMap)
	check := func(who string, v *mapsv1.GetMapVisionResponse, col, row int, want bool) {
		if got := sees(v, grid.Square{Col: col, Row: row}); got != want {
			t.Fatalf("the fixture's fog is not what the test assumes: %s sees (%d,%d) = %v, want %v\n%s", who, col, row, got, want, strings.Join(picture(v), "\n"))
		}
	}
	for _, sq := range [][2]int{{2, 7}, {2, 9}, {4, 7}, {4, 8}, {4, 10}, {6, 7}, {5, 7}} { // the west: Ana's
		check("Ana", ana, sq[0], sq[1], true)
		check("Caio", caio, sq[0], sq[1], false)
	}
	for _, sq := range [][2]int{{10, 12}} { // the south room: Caio's
		check("Caio", caio, sq[0], sq[1], true)
		check("Ana", ana, sq[0], sq[1], false)
	}
	for _, sq := range [][2]int{{19, 3}, {20, 4}, {21, 3}, {22, 2}, {18, 6}} { // the guard room: nobody's
		check("Ana", ana, sq[0], sq[1], false)
		check("Caio", caio, sq[0], sq[1], false)
	}
}
