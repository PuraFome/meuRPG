package dungeon

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestWallsMask(t *testing.T) {
	d := mustGen(t, opts(48213, 31, 21))
	open, wall := WallsMask(d)
	for i := range d.Kinds {
		if open[i] != (d.Kinds[i] != KindRock) {
			t.Fatal("open mask differs from the grid")
		}
		if open[i] && wall[i] {
			t.Fatal("an open square is a wall")
		}
		x, y := i%d.Width, i/d.Width
		near := false
		for oy := -1; oy <= 1; oy++ {
			for ox := -1; ox <= 1; ox++ {
				if d.Open(x+ox, y+oy) {
					near = true
				}
			}
		}
		if !open[i] && wall[i] != near {
			t.Fatalf("square (%d,%d): wall %v, open neighbor %v", x, y, wall[i], near)
		}
	}
}

func TestMarkers(t *testing.T) {
	d := mustGen(t, opts(5, 51, 51))
	ms := Markers(d)
	if len(ms) != len(d.Doors)+len(d.Stairs) {
		t.Fatalf("%d markers for %d doors and %d stairs", len(ms), len(d.Doors), len(d.Stairs))
	}
	for _, m := range ms[len(d.Doors):] {
		if m.Type != "stairs_up" && m.Type != "stairs_down" {
			t.Fatalf("stair marker %q", m.Type)
		}
	}
}

func TestCorridorIDsFollowRowMajor(t *testing.T) {
	d := mustGen(t, opts(9, 61, 61))
	last := -1
	for _, c := range d.Corridors {
		first := c.FirstY*d.Width + c.FirstX
		if first <= last {
			t.Fatalf("corridor %d first square is not after the previous one", c.ID)
		}
		last = first
		if c.Length < 1 {
			t.Fatalf("corridor %d is empty", c.ID)
		}
	}
	sum := 0
	for _, c := range d.Corridors {
		sum += c.Length
	}
	n := 0
	for _, k := range d.Kinds {
		if k == KindCorridor {
			n++
		}
	}
	if sum != n {
		t.Fatalf("corridors cover %d squares of %d", sum, n)
	}
}

func TestPromptJSONShapeAndConnected(t *testing.T) {
	for seed := uint64(1); seed <= 60; seed++ {
		o := opts(seed, 51, 41)
		o.ExtraLoops = int(seed%2) * 30
		d := mustGen(t, o)
		raw, err := PromptJSON(d)
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]json.RawMessage
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		for _, k := range []string{"size", "style", "rooms", "doors", "corridors", "connections", "networks", "stairs", "entrance"} {
			if _, ok := doc[k]; !ok {
				t.Fatalf("no %q in the prompt", k)
			}
		}
		again, _ := PromptJSON(d)
		if string(raw) != string(again) {
			t.Fatal("the prompt is not deterministic")
		}
		p := PromptOf(d)
		if p.Size.WidthFt != d.Width*5 || len(p.Rooms) != len(d.Rooms) || len(p.Doors) != len(d.Doors) {
			t.Fatal("prompt sizes are wrong")
		}
		if err := roomGraphConnected(d); err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		// no pixel data, no randomness: only the listed fields
		if strings.Contains(string(raw), "pixel") || strings.Contains(string(raw), "seed") {
			t.Fatal("the prompt carries pixels or the seed")
		}
	}
}

func TestDoorJoinsAndExits(t *testing.T) {
	d := mustGen(t, opts(21, 61, 61))
	for _, dr := range d.Doors {
		if dr.RoomB == 0 && dr.Corridor == 0 {
			t.Fatalf("door %d joins no corridor", dr.ID)
		}
		if dr.RoomB != 0 && dr.Corridor != 0 {
			t.Fatalf("direct door %d has a corridor", dr.ID)
		}
		r, _ := d.Room(dr.RoomA)
		found := false
		for _, e := range r.Exits {
			if e.DoorID == dr.ID && e.Side == dr.Side {
				found = true
			}
		}
		if !found {
			t.Fatalf("door %d is not an exit of room %d on its %s side", dr.ID, dr.RoomA, dr.Side)
		}
	}
	ids := []int{}
	for _, r := range d.Rooms {
		ids = append(ids, r.ID)
	}
	if !slices.IsSorted(ids) {
		t.Fatal("rooms are not in ID order")
	}
}

func TestASCIISize(t *testing.T) {
	d := mustGen(t, opts(48213, 31, 21))
	lines := strings.Split(strings.TrimRight(ASCII(d), "\n"), "\n")
	if len(lines) != 21 || len(lines[0]) != 31 {
		t.Fatalf("%d lines of %d", len(lines), len(lines[0]))
	}
}

// roomGraphConnected is spec 5.12: the rooms, joined by direct doors and by
// the corridor networks they open onto, form one connected graph.
func roomGraphConnected(d *Dungeon) error {
	p := PromptOf(d)
	parent := map[int]int{}
	var find func(int) int
	find = func(a int) int {
		if parent[a] != a {
			parent[a] = find(parent[a])
		}
		return parent[a]
	}
	for _, r := range d.Rooms {
		parent[r.ID] = r.ID
	}
	for _, c := range p.Connections {
		if c.A >= c.B {
			return fmt.Errorf("bad connection %+v", c)
		}
		parent[find(c.A)] = find(c.B)
	}
	for _, n := range p.Networks {
		for _, r := range n.Rooms[1:] {
			parent[find(r)] = find(n.Rooms[0])
		}
	}
	roots := map[int]bool{}
	for _, r := range d.Rooms {
		roots[find(r.ID)] = true
	}
	if len(roots) != 1 {
		return fmt.Errorf("connections and networks leave %d separate groups of %d rooms", len(roots), len(d.Rooms))
	}
	return nil
}

func TestNetworksLinear(t *testing.T) {
	d := mustGen(t, opts(7, 61, 61))
	p := PromptOf(d)
	total := 0
	for k, n := range p.Networks {
		if n.ID != k+1 || len(n.Rooms) == 0 || len(n.Corridors) == 0 {
			t.Fatalf("bad network %+v", n)
		}
		total += n.LengthSquares
	}
	corr := 0
	for _, k := range d.Kinds {
		if k == KindCorridor {
			corr++
		}
	}
	if total != corr {
		t.Fatalf("networks cover %d corridor squares of %d", total, corr)
	}
}

// The slowest PromptOf the reviewer found (mask 01100 11001 11111 11001,
// sides 5..11, door density 25, loops 100) and the defaults, both at 199 x 399.
func promptWorst() []Options {
	a := opts(14343889915692936209, 199, 399)
	a.Mask = MaskCustom
	a.CustomMask = &CustomMask{5, 4, maskBits("01100", "11001", "11111", "11001")}
	a.RoomSideMin, a.RoomSideMax, a.DoorDensity, a.ExtraLoops = 5, 11, 25, 100
	return []Options{a, opts(7, 199, 399)}
}

func maskBits(rows ...string) []bool {
	var out []bool
	for _, r := range rows {
		for _, c := range r {
			out = append(out, c == '1')
		}
	}
	return out
}

func TestPromptOfIsFast(t *testing.T) {
	for _, o := range promptWorst() {
		d := mustGen(t, o)
		start := time.Now()
		PromptOf(d)
		if el := time.Since(start); measuring() && el > 20*time.Millisecond {
			t.Errorf("PromptOf took %v (budget 20 ms)", el)
		}
	}
}

func BenchmarkPromptOf(b *testing.B) {
	for i, o := range promptWorst() {
		o.SelfCheck = false
		d, err := Generate(o)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(fmt.Sprint("case", i), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				PromptOf(d)
			}
		})
	}
}
