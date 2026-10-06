package dungeon

import "testing"

func worstCases() map[string]func(o *Options) {
	return map[string]func(o *Options){
		"tiled-200": func(o *Options) { o.Placement = PlacementTiled; o.RoomDensity = 200 },
		"tiled-smallest": func(o *Options) {
			o.Placement = PlacementTiled
			o.RoomDensity = 200
			o.RoomSideMin, o.RoomSideMax = 3, 3
			o.DoorDensity = 25
		},
		"spread-dense-small": func(o *Options) { o.RoomDensity = 200; o.RoomSideMin, o.RoomSideMax = 3, 5; o.DoorDensity = 25 },
		"spread-dense-med": func(o *Options) {
			o.RoomDensity = 200
			o.RoomSideMin, o.RoomSideMax = 3, 7
			o.DoorDensity = 25
			o.CorridorStyle = StyleLongRuns
		},
		"spread-default": func(*Options) {},
		"loops-donut":    func(o *Options) { o.RoomDensity = 200; o.Mask = MaskDonut; o.DoorDensity = 25; o.ExtraLoops = 100 },
	}
}

func BenchmarkWorst(b *testing.B) {
	for name, f := range worstCases() {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				o := opts(uint64(i%20+1), 199, 399)
				f(&o)
				if _, err := Generate(o); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestWorstCasePasses(t *testing.T) {
	for name, f := range worstCases() {
		maxp, disc, total := 0, 0, 0
		for seed := uint64(1); seed <= 20; seed++ {
			o := opts(seed, 199, 399)
			o.SelfCheck = true
			f(&o)
			var gp *gen
			d, err := generate(o, func(_ string, g *gen) { gp = g })
			if err != nil {
				t.Fatal(err)
			}
			maxp = max(maxp, gp.passes)
			disc += len(d.DiscardedRooms)
			total += len(d.Rooms) + len(d.DiscardedRooms)
		}
		if maxp > 40 || disc*20 > total {
			t.Errorf("%s: %d passes, %d of %d rooms discarded", name, maxp, disc, total)
		}
		t.Logf("%s: max passes %d, discarded rooms over 20 seeds %d", name, maxp, disc)
	}
}
