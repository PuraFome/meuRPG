package dungeon

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite testdata/golden.json")

func hashOf(d *Dungeon) string {
	b, err := json.Marshal(d)
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func mustGen(t testing.TB, o Options) *Dungeon {
	t.Helper()
	o.SelfCheck = true
	d, err := Generate(o)
	if err != nil {
		t.Fatalf("Generate(%+v): %v", o, err)
	}
	if err := check(d); err != nil {
		t.Fatalf("check: %v", err)
	}
	return d
}

func opts(seed uint64, w, h int) Options {
	o := DefaultOptions(seed)
	o.Width, o.Height = w, h
	return o
}

var allMasks = []Mask{MaskNone, MaskDonut, MaskPlus, MaskLShape, MaskEllipse, MaskDiamond, MaskCustom}

func customMask(seed uint64) *CustomMask {
	r := newRNG(seed)
	m := &CustomMask{Width: 5, Height: 4, On: make([]bool, 20)}
	for i := range m.On {
		m.On[i] = r.intn(4) != 0
	}
	return m
}

// comboFor builds a varied option set from a counter, deterministically.
func comboFor(k uint64) Options {
	r := newRNG(k * 7919)
	sizes := []int{15, 16, 17, 21, 22, 25, 31, 33, 40, 51, 61, 81}
	o := DefaultOptions(r.next())
	if k%97 == 0 {
		o.Seed = 0
	}
	o.Width = sizes[r.intn(len(sizes))]
	o.Height = sizes[r.intn(len(sizes))]
	o.Mask = allMasks[r.intn(len(allMasks))]
	o.MaskHole = 20 + r.intn(41)
	o.CustomMask = customMask(r.next())
	o.RoomSideMin = 3 + r.intn(6)
	o.RoomSideMax = o.RoomSideMin + r.intn(12)
	o.AspectLimit = []float64{1, 1.5, 3, 6}[r.intn(4)]
	o.RoomDensity = []int{10, 50, 100, 150, 200}[r.intn(5)]
	o.Placement = []Placement{PlacementSpread, PlacementTiled}[r.intn(2)]
	o.CorridorStyle = []CorridorStyle{StyleTwisty, StyleMeandering, StyleLongRuns}[r.intn(3)]
	o.DoorDensity = []int{25, 100, 200}[r.intn(3)]
	o.DoorMix = []DoorMix{MixOpen, MixTypical, MixSecured, MixParanoid}[r.intn(4)]
	o.DeadendRemoval = []int{0, 60, 100}[r.intn(3)]
	o.Stairs = r.intn(9)
	o.ExtraLoops = []int{0, 30}[r.intn(2)]
	return o
}

// Property tests: every invariant of spec 5 over many option combinations.
func TestPropertiesOverManyOptions(t *testing.T) {
	n := 1500
	if testing.Short() {
		n = 300
	}
	noSpace, discarded, withLoops, multiRoom := 0, 0, 0, 0
	for k := 0; k < n; k++ {
		o := comboFor(uint64(k))
		o.SelfCheck = true
		d, err := Generate(o)
		if errors.Is(err, ErrNoSpace) {
			noSpace++
			continue
		}
		if err != nil {
			t.Fatalf("combo %d %+v: %v", k, o, err)
		}
		if err := check(d); err != nil {
			t.Fatalf("combo %d: %v", k, err)
		}
		if len(d.DiscardedRooms) > 0 {
			discarded++
		}
		if o.ExtraLoops > 0 {
			withLoops++
		}
		if len(d.Rooms) > 1 {
			multiRoom++
		}
		if k%5 == 0 {
			if err := roomGraphConnected(d); err != nil {
				t.Fatalf("combo %d: %v", k, err)
			}
		}
		// the same input twice is byte-identical
		if k%10 == 0 {
			d2, _ := Generate(o)
			if hashOf(d) != hashOf(d2) {
				t.Fatalf("combo %d is not deterministic", k)
			}
		}
	}
	t.Logf("%d combos: %d ErrNoSpace, %d with discarded rooms, %d with loops, %d with 2+ rooms", n, noSpace, discarded, withLoops, multiRoom)
	if noSpace > n/4 {
		t.Fatalf("too many ErrNoSpace: %d of %d", noSpace, n)
	}
}

// Every mask, both placements, every corridor style and door mix, stairs 0..8,
// dead-end removal 0/60/100, extra loops 0/30, at odd and even sizes.
func TestPropertiesFullGrid(t *testing.T) {
	seed := uint64(1)
	count := 0
	for _, mask := range allMasks {
		for _, pl := range []Placement{PlacementSpread, PlacementTiled} {
			for _, st := range []CorridorStyle{StyleTwisty, StyleMeandering, StyleLongRuns} {
				for _, mix := range []DoorMix{MixOpen, MixTypical, MixSecured, MixParanoid} {
					seed++
					o := opts(seed, 41+int(seed%2), 33)
					o.Mask, o.CustomMask, o.Placement, o.CorridorStyle, o.DoorMix = mask, customMask(seed), pl, st, mix
					o.Stairs = int(seed % 9)
					o.DeadendRemoval = []int{0, 60, 100}[seed%3]
					o.ExtraLoops = []int{0, 30}[seed%2]
					o.SelfCheck = true
					if _, err := Generate(o); err != nil && !errors.Is(err, ErrNoSpace) {
						t.Fatalf("%+v: %v", o, err)
					}
					count++
				}
			}
		}
	}
	t.Logf("%d combinations", count)
}

func TestBigSizes(t *testing.T) {
	for _, s := range [][2]int{{199, 199}, {198, 120}, {199, 399}, {121, 121}, {16, 399}} {
		for seed := uint64(1); seed <= 2; seed++ {
			o := opts(seed, s[0], s[1])
			o.ExtraLoops = int(seed-1) * 30
			d := mustGen(t, o)
			if d.Width%2 == 0 || d.Height%2 == 0 {
				t.Fatalf("size %dx%d is even", d.Width, d.Height)
			}
		}
	}
}

func TestDeadendsZeroRemovesNone(t *testing.T) {
	for seed := uint64(1); seed <= 30; seed++ {
		o := opts(seed, 41, 41)
		o.DeadendRemoval = 0
		o.SelfCheck = true
		before, after := -1, -1
		count := func(g *gen) int {
			n := 0
			for i := range g.kind {
				if g.kind[i] == KindCorridor && g.openDeg(i) == 1 {
					n++
				}
			}
			return n
		}
		_, err := generate(o, func(stage string, g *gen) {
			switch stage {
			case "stairs":
				before = count(g)
			case "deadends":
				after = count(g)
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		if before != after || before < 0 {
			t.Fatalf("seed %d: dead ends %d before the phase, %d after", seed, before, after)
		}
	}
}

func TestDeadendsFullRemoval(t *testing.T) {
	// check() enforces it; make sure the cases are not trivial: there were dead ends to cut.
	cut := 0
	for seed := uint64(1); seed <= 30; seed++ {
		o := opts(seed, 51, 51)
		o.DeadendRemoval = 100
		before := 0
		o.SelfCheck = true
		_, err := generate(o, func(stage string, g *gen) {
			if stage == "stairs" {
				for i := range g.kind {
					if g.kind[i] == KindCorridor && g.openDeg(i) == 1 && !g.protected[i] {
						before++
					}
				}
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		cut += before
	}
	if cut == 0 {
		t.Fatal("no dead end was ever cut")
	}
}

// Golden tests: fixed seeds and options, with a pinned hash per version.
type goldenCase struct {
	Name string
	Opts func() Options
}

func goldenCases() []goldenCase {
	mk := func(name string, f func(o *Options)) goldenCase {
		return goldenCase{name, func() Options {
			o := DefaultOptions(48213)
			f(&o)
			return o
		}}
	}
	return []goldenCase{
		mk("default-51", func(*Options) {}),
		mk("small-31x21", func(o *Options) { o.Width, o.Height = 31, 21 }),
		mk("smallest-15", func(o *Options) { o.Width, o.Height = 15, 15; o.Seed = 1 }),
		mk("tiled-61", func(o *Options) { o.Width, o.Height = 61, 61; o.Placement = PlacementTiled }),
		mk("donut-paranoid", func(o *Options) { o.Mask = MaskDonut; o.DoorMix = MixParanoid; o.Seed = 7 }),
		mk("plus-long-runs", func(o *Options) { o.Mask = MaskPlus; o.CorridorStyle = StyleLongRuns; o.Seed = 99 }),
		mk("l-shape-twisty", func(o *Options) { o.Mask = MaskLShape; o.CorridorStyle = StyleTwisty; o.Width = 60 }),
		mk("ellipse-loops", func(o *Options) { o.Mask = MaskEllipse; o.ExtraLoops = 30; o.Width, o.Height = 71, 45 }),
		mk("diamond-stairs8", func(o *Options) { o.Mask = MaskDiamond; o.Stairs = 8; o.DeadendRemoval = 100 }),
		mk("custom", func(o *Options) { o.Mask = MaskCustom; o.CustomMask = customMask(5); o.Seed = 3 }),
		mk("keep-all-dead-ends", func(o *Options) { o.DeadendRemoval = 0; o.Stairs = 0 }),
		mk("seed0", func(o *Options) { o.Seed = 0 }),
		mk("seedmax", func(o *Options) { o.Seed = math.MaxUint64 }),
		mk("wide-199x399", func(o *Options) { o.Width, o.Height = 199, 399; o.Seed = 2026 }),
	}
}

func TestGolden(t *testing.T) {
	got := map[string]string{}
	for _, c := range goldenCases() {
		got[c.Name] = hashOf(mustGen(t, c.Opts()))
	}
	const path = "testdata/golden.json"
	if *update {
		b, _ := json.MarshalIndent(map[string]any{"version": Version, "hashes": got}, "", "  ")
		if err := os.WriteFile(path, append(b, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var want struct {
		Version int               `json:"version"`
		Hashes  map[string]string `json:"hashes"`
	}
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if want.Version != Version {
		t.Fatalf("golden file is for version %d, code is %d: pin the new version's hashes with -update", want.Version, Version)
	}
	for name, h := range got {
		if want.Hashes[name] != h {
			t.Errorf("%s: hash changed (%s, was %s). If the output change is intended, bump Version and run with -update", name, h, want.Hashes[name])
		}
	}
	if len(want.Hashes) != len(got) {
		t.Errorf("golden has %d cases, test has %d", len(want.Hashes), len(got))
	}
}

// 5.13: changing `stairs` leaves the rooms identical; changing `door_mix` moves
// no door (and leaves the whole grid and the stairs as they were).
func TestStreamIndependence(t *testing.T) {
	roomBounds := func(d *Dungeon) string {
		var sb strings.Builder
		for _, r := range d.Rooms {
			fmt.Fprintf(&sb, "%d:%d,%d,%d,%d;", r.ID, r.X, r.Y, r.Width, r.Height)
		}
		return sb.String()
	}
	for seed := uint64(1); seed <= 60; seed++ {
		base := opts(seed, 51, 41)
		base.ExtraLoops = int(seed%2) * 30
		ref := mustGen(t, base)
		for st := 0; st <= 8; st++ {
			o := base
			o.Stairs = st
			if got := roomBounds(mustGen(t, o)); got != roomBounds(ref) {
				t.Fatalf("seed %d: stairs=%d moved the rooms", seed, st)
			}
		}
		// door_mix draws only from the door kinds stream: no door moves.
		for _, mix := range []DoorMix{MixOpen, MixTypical, MixSecured, MixParanoid} {
			o := base
			o.DoorMix = mix
			d := mustGen(t, o)
			if !slices.Equal(d.Kinds, ref.Kinds) || !slices.Equal(d.RoomIDs, ref.RoomIDs) || !slices.Equal(d.Stairs, ref.Stairs) || len(d.Doors) != len(ref.Doors) {
				t.Fatalf("seed %d: door_mix %s changed the grid", seed, mix)
			}
			for i := range d.Doors {
				a, b := d.Doors[i], ref.Doors[i]
				if a.X != b.X || a.Y != b.Y || a.Axis != b.Axis || a.RoomA != b.RoomA || a.RoomB != b.RoomB {
					t.Fatalf("seed %d: door_mix %s moved door %d", seed, mix, i+1)
				}
			}
		}
	}
}

func TestDoorMixChangesKinds(t *testing.T) {
	count := func(mix DoorMix) (secret, trapped, total int) {
		for seed := uint64(1); seed <= 20; seed++ {
			o := opts(seed, 61, 61)
			o.DoorMix = mix
			for _, d := range mustGen(t, o).Doors {
				total++
				if d.Kind == DoorSecret {
					secret++
				}
				if d.Trapped {
					trapped++
				}
			}
		}
		return
	}
	so, to, no := count(MixOpen)
	sp, tp, np := count(MixParanoid)
	if to != 0 {
		t.Fatalf("open mix has %d trapped doors", to)
	}
	if tp == 0 || sp*no <= so*np {
		t.Fatalf("paranoid should have more secret and trapped doors: %d/%d secret, %d trapped", sp, np, tp)
	}
}

func TestSmallestGridNeverPanics(t *testing.T) {
	for _, mask := range allMasks {
		for _, pl := range []Placement{PlacementSpread, PlacementTiled} {
			for seed := uint64(0); seed < 12; seed++ {
				o := opts(seed, 15, 15)
				o.Mask, o.Placement, o.CustomMask = mask, pl, customMask(seed)
				o.Stairs, o.ExtraLoops, o.DoorDensity = 8, 100, 200
				o.SelfCheck = true
				if _, err := Generate(o); err != nil && !errors.Is(err, ErrNoSpace) {
					t.Fatalf("%s %s seed %d: %v", mask, pl, seed, err)
				}
			}
		}
	}
	// With no mask the smallest grid always gets a room.
	for seed := uint64(0); seed < 50; seed++ {
		d := mustGen(t, opts(seed, 15, 15))
		if len(d.Rooms) == 0 {
			t.Fatal("no room")
		}
	}
}

func TestErrNoSpace(t *testing.T) {
	o := opts(1, 51, 51)
	o.Mask = MaskCustom
	o.CustomMask = &CustomMask{Width: 3, Height: 3, On: make([]bool, 9)}
	if _, err := Generate(o); !errors.Is(err, ErrNoSpace) {
		t.Fatalf("all-off custom mask: %v", err)
	}
	// A room larger than the grid can hold.
	o = opts(1, 15, 15)
	o.RoomSideMin, o.RoomSideMax = 15, 15
	if _, err := Generate(o); !errors.Is(err, ErrNoSpace) {
		t.Fatalf("15 side room on a 15 grid: %v", err)
	}
}

func TestOneRoomOnly(t *testing.T) {
	o := opts(5, 15, 15)
	o.RoomSideMin, o.RoomSideMax = 13, 13
	o.Stairs = 0
	d := mustGen(t, o)
	if len(d.Rooms) != 1 || len(d.Doors) != 0 {
		t.Fatalf("%d rooms, %d doors", len(d.Rooms), len(d.Doors))
	}
	cx, cy := d.Rooms[0].Center()
	if d.Entrance.X != cx || d.Entrance.Y != cy || d.Entrance.OnStairs {
		t.Fatalf("entrance %+v, center %d,%d", d.Entrance, cx, cy)
	}
	o.Stairs = 3
	d = mustGen(t, o)
	if d.StairsPlaced == 0 || d.Stairs[0].InRoom != 1 || !d.Entrance.OnStairs {
		t.Fatalf("room-site stairs: %+v", d.Stairs)
	}
}

func TestRoomSidesEqual(t *testing.T) {
	for _, pl := range []Placement{PlacementSpread, PlacementTiled} {
		o := opts(11, 61, 61)
		o.RoomSideMin, o.RoomSideMax, o.Placement = 7, 7, pl
		d := mustGen(t, o)
		if len(d.Rooms) < 3 {
			t.Fatalf("%s: %d rooms", pl, len(d.Rooms))
		}
		for _, r := range d.Rooms {
			if r.Width != 7 || r.Height != 7 {
				t.Fatalf("room %d is %dx%d", r.ID, r.Width, r.Height)
			}
		}
	}
}

func TestTiledDensity200IsAs100(t *testing.T) {
	a, b := opts(3, 51, 51), opts(3, 51, 51)
	a.Placement, b.Placement = PlacementTiled, PlacementTiled
	a.RoomDensity, b.RoomDensity = 100, 200
	da, db := mustGen(t, a), mustGen(t, b)
	if !slices.Equal(da.Kinds, db.Kinds) || len(da.Rooms) != len(db.Rooms) {
		t.Fatal("tiled 200 differs from 100")
	}
}

func TestTiledEqualSidesTile(t *testing.T) {
	o := opts(8, 51, 51)
	o.Placement, o.RoomSideMin, o.RoomSideMax = PlacementTiled, 5, 5
	d := mustGen(t, o)
	if len(d.Rooms) < 20 {
		t.Fatalf("only %d rooms tile a 51 grid with 5-square rooms", len(d.Rooms))
	}
}

func TestSeedExtremes(t *testing.T) {
	for _, seed := range []uint64{0, 1, math.MaxUint64, math.MaxUint64 - 1, 1 << 63} {
		mustGen(t, opts(seed, 41, 41))
	}
}

func TestOptionsRejected(t *testing.T) {
	cases := []struct {
		option string
		mod    func(o *Options)
	}{
		{"version", func(o *Options) { o.Version = 0 }},
		{"version", func(o *Options) { o.Version = Version + 1 }},
		{"width", func(o *Options) { o.Width = 14 }},
		{"width", func(o *Options) { o.Width = 200 }},
		{"height", func(o *Options) { o.Height = 400 }},
		{"height", func(o *Options) { o.Height = 3 }},
		{"mask", func(o *Options) { o.Mask = "blob" }},
		{"mask_hole", func(o *Options) { o.Mask = MaskDonut; o.MaskHole = 19 }},
		{"mask_hole", func(o *Options) { o.Mask = MaskDonut; o.MaskHole = 61 }},
		{"custom_mask", func(o *Options) { o.Mask = MaskCustom }},
		{"custom_mask", func(o *Options) { o.Mask = MaskCustom; o.CustomMask = &CustomMask{2, 3, make([]bool, 6)} }},
		{"custom_mask", func(o *Options) { o.Mask = MaskCustom; o.CustomMask = &CustomMask{60, 3, make([]bool, 180)} }},
		{"custom_mask", func(o *Options) { o.Mask = MaskCustom; o.CustomMask = &CustomMask{3, 3, make([]bool, 8)} }},
		{"room_side_min", func(o *Options) { o.RoomSideMin = 2 }},
		{"room_side_min", func(o *Options) { o.RoomSideMin = 16 }},
		{"room_side_max", func(o *Options) { o.RoomSideMax = 2 }},
		{"room_side_max", func(o *Options) { o.RoomSideMax = 32 }},
		{"aspect_limit", func(o *Options) { o.AspectLimit = 0.9 }},
		{"aspect_limit", func(o *Options) { o.AspectLimit = 6.1 }},
		{"aspect_limit", func(o *Options) { o.AspectLimit = math.NaN() }},
		{"room_density", func(o *Options) { o.RoomDensity = 9 }},
		{"room_density", func(o *Options) { o.RoomDensity = 201 }},
		{"placement", func(o *Options) { o.Placement = "x" }},
		{"corridor_style", func(o *Options) { o.CorridorStyle = "" }},
		{"door_density", func(o *Options) { o.DoorDensity = 24 }},
		{"door_density", func(o *Options) { o.DoorDensity = 201 }},
		{"door_mix", func(o *Options) { o.DoorMix = "none" }},
		{"deadend_removal", func(o *Options) { o.DeadendRemoval = -1 }},
		{"deadend_removal", func(o *Options) { o.DeadendRemoval = 101 }},
		{"stairs", func(o *Options) { o.Stairs = 9 }},
		{"stairs", func(o *Options) { o.Stairs = -1 }},
		{"extra_loops", func(o *Options) { o.ExtraLoops = 101 }},
	}
	for _, c := range cases {
		o := DefaultOptions(1)
		c.mod(&o)
		_, err := Generate(o)
		var oe *OptionError
		if !errors.As(err, &oe) || oe.Option != c.option || !errors.Is(err, ErrInvalidOption) {
			t.Errorf("%s: got %v", c.option, err)
		} else if !strings.Contains(err.Error(), c.option) {
			t.Errorf("%s: the message does not name it: %v", c.option, err)
		}
	}
}

func TestEvenSizesNormalised(t *testing.T) {
	o := opts(1, 50, 24)
	o.RoomSideMin, o.RoomSideMax = 4, 10
	d := mustGen(t, o)
	if d.Width != 49 || d.Height != 23 || d.Options.Width != 49 || d.Options.Height != 23 {
		t.Fatalf("size %dx%d", d.Width, d.Height)
	}
	if d.Options.RoomSideMin != 5 || d.Options.RoomSideMax != 9 {
		t.Fatalf("sides %d..%d", d.Options.RoomSideMin, d.Options.RoomSideMax)
	}
	o.RoomSideMin, o.RoomSideMax = 14, 14
	d = mustGen(t, o)
	if d.Options.RoomSideMin != 15 || d.Options.RoomSideMax != 15 {
		t.Fatalf("sides %d..%d", d.Options.RoomSideMin, d.Options.RoomSideMax)
	}
	// 15 is the smallest; 16 goes down to it.
	d = mustGen(t, opts(1, 16, 16))
	if d.Width != 15 {
		t.Fatalf("width %d", d.Width)
	}
}

func TestMasksBlockSquares(t *testing.T) {
	for _, mask := range []Mask{MaskDonut, MaskPlus, MaskLShape, MaskEllipse, MaskDiamond, MaskCustom} {
		o := opts(4, 61, 61)
		o.Mask, o.CustomMask = mask, customMask(4)
		d := mustGen(t, o)
		blocked := 0
		for _, in := range d.Mask {
			if !in {
				blocked++
			}
		}
		if blocked == 0 || blocked == len(d.Mask) {
			t.Fatalf("%s blocks %d squares", mask, blocked)
		}
	}
	d := mustGen(t, opts(4, 61, 61))
	for _, in := range d.Mask {
		if !in {
			t.Fatal("mask none blocks a square")
		}
	}
}

func TestDonutGeometry(t *testing.T) {
	o := opts(2, 51, 51)
	o.Mask, o.MaskHole = MaskDonut, 40
	d := mustGen(t, o)
	// hole 20 wide, from column 15
	if d.Mask[25*51+25] || !d.Mask[25*51+14] || d.Mask[25*51+15] || d.Mask[25*51+34] || !d.Mask[25*51+35] {
		t.Fatal("donut hole is not where the spec puts it")
	}
}

func TestReachableAndNoOrphans(t *testing.T) {
	// A tight sweep for the connectivity phase: dense rooms, few doors.
	discarded := 0
	for seed := uint64(1); seed <= 150; seed++ {
		o := opts(seed, 41, 41)
		o.Placement = PlacementTiled
		o.DoorDensity = 25
		o.RoomSideMin, o.RoomSideMax = 3, 5
		d := mustGen(t, o)
		discarded += len(d.DiscardedRooms)
	}
	t.Logf("%d rooms discarded over 150 dense levels", discarded)
}

func TestExtraLoopsAddCycles(t *testing.T) {
	// A cycle means more open adjacencies than a tree: compare open-neighbor edges.
	edges := func(d *Dungeon) (e, nodes int) {
		for i, k := range d.Kinds {
			if k == KindRock {
				continue
			}
			nodes++
			x, y := i%d.Width, i/d.Width
			if d.Open(x+1, y) {
				e++
			}
			if d.Open(x, y+1) {
				e++
			}
		}
		return
	}
	more := 0
	for seed := uint64(1); seed <= 20; seed++ {
		a, b := opts(seed, 61, 61), opts(seed, 61, 61)
		b.ExtraLoops = 100
		da, db := mustGen(t, a), mustGen(t, b)
		ea, _ := edges(da)
		eb, _ := edges(db)
		if len(db.Rooms) != len(da.Rooms) {
			t.Fatal("loops changed the rooms")
		}
		if eb > ea {
			more++
		}
	}
	if more < 10 {
		t.Fatalf("extra_loops 100 added connections in only %d of 20 levels", more)
	}
}

func TestStairsAreAtDeadEndsOrRoomCorners(t *testing.T) {
	corr, room := 0, 0
	for seed := uint64(1); seed <= 60; seed++ {
		o := opts(seed, 41, 41)
		// Tiled small rooms leave few dead ends, so the room corners are needed.
		o.Placement, o.Stairs, o.DeadendRemoval = PlacementTiled, 8, 100
		o.RoomSideMin, o.RoomSideMax = 3, 5
		d := mustGen(t, o)
		for _, s := range d.Stairs {
			if s.InRoom == 0 {
				corr++
			} else {
				room++
			}
		}
	}
	if corr == 0 || room == 0 {
		t.Fatalf("stairs: %d in corridors, %d in rooms", corr, room)
	}
}

func TestDefaultsAreValid(t *testing.T) {
	d := mustGen(t, DefaultOptions(1))
	if d.Width != 51 || d.Height != 51 || d.Version != 1 || len(d.Rooms) < 3 {
		t.Fatalf("%dx%d, %d rooms", d.Width, d.Height, len(d.Rooms))
	}
}

// A fuzz over the options: no input may panic; a rejected input says which
// option; an accepted one passes every invariant.
func FuzzOptions(f *testing.F) {
	f.Add(uint64(1), 31, 21, 0, 40, 3, 11, 3.0, 100, 0, 0, 100, 1, 60, 2, 0)
	f.Add(uint64(48213), 15, 15, 6, 20, 13, 13, 1.0, 10, 1, 2, 25, 3, 100, 8, 100)
	f.Add(uint64(0), 199, 61, 3, 60, 15, 31, 6.0, 200, 1, 1, 200, 0, 0, 0, 30)
	f.Fuzz(func(t *testing.T, seed uint64, w, h, mask, hole, smin, smax int, aspect float64, rd, pl, style, dd, mix, de, stairs, loops int) {
		pick := func(i, n int) int { return ((i % n) + n) % n }
		o := Options{
			Seed: seed, Version: 1, Width: w, Height: h,
			Mask:     []Mask{MaskNone, MaskDonut, MaskPlus, MaskLShape, MaskEllipse, MaskDiamond, MaskCustom, "bogus"}[pick(mask, 8)],
			MaskHole: hole, CustomMask: customMask(seed),
			RoomSideMin: smin, RoomSideMax: smax, AspectLimit: aspect, RoomDensity: rd,
			Placement:     []Placement{PlacementSpread, PlacementTiled, "bogus"}[pick(pl, 3)],
			CorridorStyle: []CorridorStyle{StyleTwisty, StyleMeandering, StyleLongRuns, "bogus"}[pick(style, 4)],
			DoorDensity:   dd, DoorMix: []DoorMix{MixOpen, MixTypical, MixSecured, MixParanoid, "bogus"}[pick(mix, 5)],
			DeadendRemoval: de, Stairs: stairs, ExtraLoops: loops, SelfCheck: true,
		}
		// keep the grids small so a run is fast
		if o.Width > 120 {
			o.Width = 15 + o.Width%100
		}
		if o.Height > 120 {
			o.Height = 15 + o.Height%100
		}
		d, err := Generate(o)
		if err != nil {
			var oe *OptionError
			if !errors.As(err, &oe) && !errors.Is(err, ErrNoSpace) {
				t.Fatalf("unexpected error: %v", err)
			}
			return
		}
		if err := check(d); err != nil {
			t.Fatal(err)
		}
	})
}

func BenchmarkGenerate(b *testing.B) {
	for _, s := range [][2]int{{31, 31}, {51, 51}, {121, 121}, {199, 199}, {199, 399}} {
		b.Run(fmt.Sprintf("%dx%d", s[0], s[1]), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := Generate(opts(uint64(i), s[0], s[1])); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// A group that fails on the first pass is deferred and links on a later one
// (spec 3.6.3 steps 4 to 6). Door lanes joining the maze (3.5) and links from
// both sides (3.6.3 b) leave few orphans, so the fixture is a dense level of
// small rooms with few doors.
func TestDeferredGroupLinksOnALaterPass(t *testing.T) {
	found := uint64(0)
	for seed := uint64(1); seed <= 200 && found == 0; seed++ {
		o := opts(seed, 41, 41)
		o.RoomDensity, o.RoomSideMin, o.RoomSideMax, o.DoorDensity = 200, 3, 5, 25
		o.SelfCheck = true
		later := 0
		d, err := generate(o, func(stage string, g *gen) {
			if stage == "grid" {
				g.onLater = func(int) { later++ }
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		if later > 0 {
			found = seed
			if err := check(d); err != nil {
				t.Fatal(err)
			}
		}
	}
	if found == 0 {
		t.Fatal("no seed in 1..200 links a group on a later pass")
	}
	t.Logf("seed %d (41x41, density 200, sides 3..5, door density 25)", found)
}

// A group that could not link links once another group's discard has freed
// squares (spec 3.6.3 step 5). Random levels almost never need this any more,
// so the case is built by hand, in one row: room 1 (the main component, with a
// door and a lane corridor), room 2 (the front group: its own lane, nothing it
// can link to) and room 3 (sharing a wall with room 2, so its only way out is
// through room 2's squares). Room 2 is discarded; room 3 then links through the
// freed squares.
func TestGroupLinksAfterAnotherIsDiscarded(t *testing.T) {
	o, err := normalize(opts(1, 19, 15))
	if err != nil {
		t.Fatal(err)
	}
	g := newGen(o)
	for y := 0; y < g.h; y++ {
		for x := 0; x < g.w; x++ {
			g.blocked[g.idx(x, y)] = y <= 3 || y >= 9 || x >= 17
		}
	}
	g.anyBlocked = true
	g.blocked[g.idx(7, 7)] = true // room 2 has no free node on its west side
	g.addRoom(1, 5, 3, 3)         // room 1, the main component
	g.addRoom(9, 5, 3, 3)         // room 2
	g.addRoom(13, 5, 3, 3)        // room 3, sharing the wall at x=12 with room 2
	for _, sq := range [][2]int{{4, 5}, {8, 5}} {
		g.kind[g.idx(sq[0], sq[1])] = KindDoor
	}
	for _, sq := range [][2]int{{5, 5}, {7, 5}} { // the lanes
		g.kind[g.idx(sq[0], sq[1])] = KindCorridor
		g.lane[g.idx(sq[0], sq[1])] = true
	}
	g.directPairs[pairKey(2, 3)] = struct{}{} // no direct door between rooms 2 and 3
	afterDiscard := 0
	g.onLater = func(discards int) {
		if discards == 1 {
			afterDiscard++
		}
	}
	g.connect()
	g.compact()
	if len(g.discarded) != 1 || g.discarded[0] != 2 {
		t.Fatalf("discarded %v, want [2]", g.discarded)
	}
	if len(g.rooms) != 2 || afterDiscard != 1 {
		t.Fatalf("%d rooms kept, %d links after the discard", len(g.rooms), afterDiscard)
	}
	c := g.labelComponents()
	if c.comp[g.idx(2, 6)] != c.comp[g.idx(14, 6)] {
		t.Fatal("room 3 is not connected to room 1 after the discard")
	}
}

// The reviewer's repro: with door lanes stuck as one-square stubs only 2 of 500
// rooms survived (seed 1, 199 x 399, density 200, sides 3, door density 200).
func TestLanesJoinTheMaze(t *testing.T) {
	o := opts(1, 199, 399)
	o.RoomDensity, o.RoomSideMin, o.RoomSideMax, o.DoorDensity = 200, 3, 3, 200
	d := mustGen(t, o)
	if len(d.Rooms) < 300 || len(d.DiscardedRooms) > len(d.Rooms)/20 {
		t.Fatalf("%d rooms kept, %d discarded", len(d.Rooms), len(d.DiscardedRooms))
	}
}

// Links from both sides (3.6.3 b): this pocketed main component used to give
// 6 of 500 rooms after 421 passes and a connectivity phase of 260 ms or more.
func TestLinksFromBothSides(t *testing.T) {
	o := opts(11175377858218570949, 199, 399)
	o.Mask, o.CustomMask = MaskCustom, &CustomMask{5, 4, maskBits("00111", "10110", "11111", "11101")}
	o.RoomSideMin, o.RoomSideMax, o.CorridorStyle = 3, 9, StyleTwisty
	o.DoorDensity, o.DeadendRemoval, o.ExtraLoops = 25, 0, 30
	o.SelfCheck = true
	start := time.Now()
	d, err := Generate(o)
	el := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Rooms) < 100 || len(d.DiscardedRooms) > len(d.Rooms)/10 {
		t.Fatalf("%d rooms kept, %d discarded", len(d.Rooms), len(d.DiscardedRooms))
	}
	if measuring() && el > 250*time.Millisecond {
		t.Fatalf("Generate took %v (ceiling 250 ms)", el)
	}
	t.Logf("%d rooms kept, %d discarded, %v", len(d.Rooms), len(d.DiscardedRooms), el)
}

// A cut loop that stops after a failed draw leaves a dead end the scan must not
// draw for again (spec 4.5): no square ever fails two draws.
func TestDeadEndScanDrawsOncePerNode(t *testing.T) {
	failedTwice, draws := 0, 0
	for seed := uint64(1); seed <= 30; seed++ {
		o := opts(seed, 81, 81)
		o.DeadendRemoval = 60
		failed := map[int]bool{}
		if _, err := generate(o, func(stage string, g *gen) {
			if stage == "grid" {
				g.onDeadEndDraw = func(node int, ok bool) {
					draws++
					if !ok {
						if failed[node] {
							failedTwice++
						}
						failed[node] = true
					}
				}
			}
		}); err != nil {
			t.Fatal(err)
		}
	}
	if draws == 0 || failedTwice != 0 {
		t.Fatalf("%d draws, %d squares failed twice", draws, failedTwice)
	}
}

// TestSweep is the big sweep (20 000 combinations: every mask, placement,
// style and mix, stairs 0 to 8, dead ends 0/60/100, loops 0/30/100, seeds 0 and
// the maximum). It is slow; run it with DUNGEON_SWEEP=1.
func TestSweep(t *testing.T) {
	if os.Getenv("DUNGEON_SWEEP") == "" {
		t.Skip("set DUNGEON_SWEEP=1")
	}
	noSpace, disc, rooms := 0, 0, 0
	for k := uint64(0); k < 20000; k++ {
		o := comboFor(k + 100000)
		switch k % 200 {
		case 0:
			o.Seed = 0
		case 1:
			o.Seed = math.MaxUint64
		}
		o.ExtraLoops = []int{0, 30, 100}[k%3]
		o.SelfCheck = true
		d, err := Generate(o)
		if errors.Is(err, ErrNoSpace) {
			noSpace++
			continue
		}
		if err != nil {
			t.Fatalf("combo %d %+v: %v", k, o, err)
		}
		if k%7 == 0 {
			if err := roomGraphConnected(d); err != nil {
				t.Fatalf("combo %d: %v", k, err)
			}
		}
		disc += len(d.DiscardedRooms)
		rooms += len(d.Rooms) + len(d.DiscardedRooms)
	}
	t.Logf("20000 combos: %d ErrNoSpace, %d of %d rooms discarded", noSpace, disc, rooms)
}
