package dungeon

import (
	"fmt"
	"slices"
)

// finish is spec 3.9: remove doors to nowhere, check that no room lost every
// exit, and derive the doors, exits, corridors, stairs and entrance from the
// final grid.
func (g *gen) finish() (*Dungeon, error) {
	w, h := g.w, g.h
	// 1. A door whose two neighbors across its wall are not both open is a
	// door to nowhere (dead-end removal cut its lane): it becomes rock. The
	// axis follows from the parity: a door at (odd, even) is in a horizontal
	// wall and is crossed north-south; at (even, odd), the other way.
	for i, k := range g.kind {
		if k != KindDoor {
			continue
		}
		x, y := i%w, i/w
		a, b := g.idx(x, y-1), g.idx(x, y+1)
		if x%2 == 0 {
			a, b = g.idx(x-1, y), g.idx(x+1, y)
		}
		if g.kind[a] == KindRock || g.kind[b] == KindRock {
			g.kind[i] = KindRock
			g.doorKind[i], g.doorTrap[i] = DoorArchway, false
		}
	}
	// 2. A room left with no exit is discarded (spec 3.9.2; with 3.6 in place
	// and leaves-only pruning it should not happen, so SelfCheck fails loudly).
	if len(g.rooms) > 1 {
		for _, rm := range g.rooms {
			if g.hasExit(rm) {
				continue
			}
			if g.o.SelfCheck {
				return nil, fmt.Errorf("%w: room %d has no exit", ErrSelfCheck, rm.id)
			}
			for y := rm.y; y < rm.y+rm.h; y++ {
				for x := rm.x; x < rm.x+rm.w; x++ {
					g.kind[g.idx(x, y)], g.rid[g.idx(x, y)] = KindRock, 0
				}
			}
			rm.discarded = true
			g.roomByID[rm.id] = nil
			g.discarded = append(g.discarded, rm.id)
			g.stairs = slices.DeleteFunc(g.stairs, func(s Stair) bool { return s.InRoom == rm.id })
			for k := 0; k < min(2, len(g.stairs)); k++ {
				g.stairs[k].Kind = StairKind(k) // the first is up, the second down
			}
		}
	}
	d := &Dungeon{
		Version: Version, Options: g.o, Width: w, Height: h,
		Kinds: g.kind, RoomIDs: g.rid, Mask: make([]bool, len(g.kind)),
		CorridorIDs: make([]uint16, len(g.kind)),
	}
	// 3. Blocked squares are rock; the silhouette stays in Mask.
	for i, b := range g.blocked {
		d.Mask[i] = !b
	}
	g.compact()
	d.DiscardedRooms = slices.Clone(g.discarded)
	slices.Sort(d.DiscardedRooms)
	d.StairsPlaced = len(g.stairs)
	d.Stairs = slices.Clone(g.stairs)

	// Doors, in row-major order of the final grid.
	for i, k := range g.kind {
		if k != KindDoor {
			continue
		}
		x, y := i%w, i/w
		dr := Door{ID: len(d.Doors) + 1, X: x, Y: y, Kind: g.doorKind[i], Trapped: g.doorTrap[i]}
		a, b := g.idx(x, y-1), g.idx(x, y+1)
		sa, sb := South, North // the side of the room at a, at b
		dr.Axis = HorizontalWall
		if x%2 == 0 {
			a, b = g.idx(x-1, y), g.idx(x+1, y)
			sa, sb = East, West
			dr.Axis = VerticalWall
		}
		ra, rb := int(g.rid[a]), int(g.rid[b])
		switch {
		case ra > 0 && rb > 0:
			dr.RoomA, dr.RoomB, dr.Side = ra, rb, sa
		case ra > 0:
			dr.RoomA, dr.Side = ra, sa
		default:
			dr.RoomA, dr.Side = rb, sb
		}
		d.Doors = append(d.Doors, dr)
	}
	// Rooms and their exits (4: a direct door shows in both rooms).
	for _, rm := range g.rooms {
		out := Room{ID: rm.id, X: rm.x, Y: rm.y, Width: rm.w, Height: rm.h}
		for side := North; side <= West; side++ {
			for p := 0; p < rm.sideLen(side); p++ {
				t := rm.thresholdAt(side, p)
				di := g.idx(t.doorX, t.doorY)
				if g.kind[di] != KindDoor {
					continue
				}
				dr := d.Doors[doorIndex(d.Doors, t.doorX, t.doorY)]
				other := 0
				if g.kind[g.idx(t.beyondX, t.beyondY)] == KindRoom {
					other = int(g.rid[g.idx(t.beyondX, t.beyondY)])
				}
				out.Exits = append(out.Exits, Exit{Side: side, X: t.doorX, Y: t.doorY, DoorID: dr.ID, Kind: dr.Kind, Trapped: dr.Trapped, Other: other})
			}
		}
		d.Rooms = append(d.Rooms, out)
	}
	g.deriveCorridors(d)
	for i := range d.Doors {
		dr := &d.Doors[i]
		if dr.RoomB == 0 {
			bx, by := corridorEnd(d, *dr)
			dr.Corridor = int(d.CorridorIDs[by*w+bx])
		}
	}
	// The entrance (1.4): the first stair, else the center of the lowest room.
	if len(d.Stairs) > 0 {
		s := d.Stairs[0]
		d.Entrance = Entrance{X: s.X, Y: s.Y, Room: s.InRoom, OnStairs: true}
		if s.InRoom == 0 {
			d.Entrance.Corridor = int(d.CorridorIDs[g.idx(s.X, s.Y)])
		}
	} else {
		cx, cy := d.Rooms[0].Center()
		d.Entrance = Entrance{X: cx, Y: cy, Room: d.Rooms[0].ID}
	}
	return d, nil
}

// deriveCorridors labels the corridor squares (spec 1.5 d): a maximal run of
// corridor squares that are not junctions is one corridor; a junction square
// (three or more open neighbors) is a corridor of one square. IDs follow the
// row-major order of the first square.
func (g *gen) deriveCorridors(d *Dungeon) {
	w := g.w
	junction := func(i int) bool { return g.kind[i] == KindCorridor && g.openDeg(i) >= 3 }
	var q, touched []int
	nCorr := 0
	for _, k := range g.kind {
		if k == KindCorridor {
			nCorr++
		}
	}
	// every corridor's Ends, back to back (sized from the corridor squares: a run
	// is rarely shorter than 3, and each has about one end)
	pool := make([]CorridorEnd, 0, nCorr/2+8)
	span := make([][2]int, 0, nCorr/2+8)
	d.Corridors = make([]Corridor, 0, nCorr/2+8)
	for i0, k := range g.kind {
		if k != KindCorridor || d.CorridorIDs[i0] != 0 {
			continue
		}
		c := Corridor{ID: len(d.Corridors) + 1, FirstX: i0 % w, FirstY: i0 / w}
		id := uint16(c.ID) //nolint:gosec // G115: corridor IDs fit uint16 (at most ~20 000 nodes)
		d.CorridorIDs[i0] = id
		q = append(q[:0], i0)
		c.Junction = junction(i0)
		touched = touched[:0]
		start := len(pool)
		for head := 0; head < len(q); head++ {
			u := q[head]
			x, y := u%w, u/w
			if !c.Junction && g.openDeg(u) == 1 {
				// the one neighbor decides: a door is a door end, else a dead end
				for dd := range 4 {
					v := g.idx(x+dx[dd], y+dy[dd])
					switch g.kind[v] {
					case KindDoor:
						pool = append(pool, CorridorEnd{Type: "door", X: x + dx[dd], Y: y + dy[dd]})
					case KindCorridor:
						pool = append(pool, CorridorEnd{Type: "dead_end", X: x, Y: y})
					}
				}
			}
			for dd := range 4 {
				v := g.idx(x+dx[dd], y+dy[dd])
				switch {
				case g.kind[v] == KindDoor && g.openDeg(u) != 1:
					pool = append(pool, CorridorEnd{Type: "door", X: x + dx[dd], Y: y + dy[dd]})
				case g.kind[v] == KindCorridor && !c.Junction && junction(v):
					if !slices.Contains(touched, v) {
						touched = append(touched, v)
						pool = append(pool, CorridorEnd{Type: "junction", X: v % w, Y: v / w})
					}
				case g.kind[v] == KindCorridor && !c.Junction && !junction(v) && d.CorridorIDs[v] == 0:
					d.CorridorIDs[v] = id
					q = append(q, v)
				}
			}
		}
		c.Length = len(q)
		c.Junctions = len(touched)
		span = append(span, [2]int{start, len(pool)})
		d.Corridors = append(d.Corridors, c)
	}
	for ci := range d.Corridors {
		sp := span[ci]
		if sp[1] > sp[0] {
			d.Corridors[ci].Ends = pool[sp[0]:sp[1]:sp[1]]
		}
		for ei := range d.Corridors[ci].Ends {
			e := &d.Corridors[ci].Ends[ei]
			if e.Type == "door" {
				e.DoorID = d.Doors[doorIndex(d.Doors, e.X, e.Y)].ID
			}
		}
	}
}

// doorIndex finds the door on a square; the doors are in row-major order.
func doorIndex(doors []Door, x, y int) int {
	i, _ := slices.BinarySearchFunc(doors, [2]int{x, y}, func(d Door, p [2]int) int {
		if d.Y != p[1] {
			return d.Y - p[1]
		}
		return d.X - p[0]
	})
	return i
}

// hasExit reports whether a door is open on the room's ring.
func (g *gen) hasExit(rm *room) bool {
	for side := North; side <= West; side++ {
		for p := 0; p < rm.sideLen(side); p++ {
			t := rm.thresholdAt(side, p)
			if g.kind[g.idx(t.doorX, t.doorY)] == KindDoor {
				return true
			}
		}
	}
	return false
}
