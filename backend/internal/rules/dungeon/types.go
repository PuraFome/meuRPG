package dungeon

// Version is the current algorithm version (spec 4.6). Bump it, and keep the
// old one behind the same function while a saved map needs it, whenever a
// fix or a tuned constant changes the output or the draw order.
const Version = 1

// The phase constants of the per-phase streams (spec 4.2). They are part of
// the contract of Version 1: the stream of a phase is
// SplitMix64(seed XOR constant). The mask phase draws nothing (the
// silhouette is integer geometry) but keeps its constant.
const (
	phaseMask     uint64 = 0x6D5A2C91E0B473F1
	phaseRooms    uint64 = 0xC3B1F5A7094D2E68
	phaseDoors    uint64 = 0x1F8E7D3A5B6C90A4
	phaseCorridor uint64 = 0xA94C0E62D7F158B3
	phaseConnect  uint64 = 0x5E27B9C4813AF60D
	phaseStairs   uint64 = 0xF0183D6A2C95E7B1
	phaseDeadend  uint64 = 0x38D6A1F94E0B5C72
	// phaseKinds is the "door kinds" stream: every door kind and trapped draw,
	// in the order the doors are made (3.4, then 3.6.3, then 3.6.4).
	phaseKinds uint64 = 0x84C2E7195AD03F6B
)

// Kind is the base kind of a square (spec 1.1). A square excluded by the
// shape mask is Rock here; the silhouette is in Dungeon.Mask.
type Kind uint8

// The base kinds.
const (
	KindRock Kind = iota
	KindRoom
	KindCorridor
	KindDoor
)

func (k Kind) String() string {
	switch k {
	case KindRock:
		return "rock"
	case KindRoom:
		return "room"
	case KindCorridor:
		return "corridor"
	case KindDoor:
		return "door"
	}
	return "?"
}

// DoorKind is what a door is, from the most open to the most closed, which
// is the order of the kind draw.
type DoorKind uint8

// The door kinds (spec 1.1); they map onto the states of RN-26.
const (
	DoorArchway DoorKind = iota
	DoorClosed
	DoorBarred
	DoorLocked
	DoorSecret
)

func (k DoorKind) String() string {
	switch k {
	case DoorArchway:
		return "archway"
	case DoorClosed:
		return "closed"
	case DoorBarred:
		return "barred"
	case DoorLocked:
		return "locked"
	case DoorSecret:
		return "secret"
	}
	return "?"
}

// canBeTrapped: only a closed, locked or secret door may carry the flag.
func (k DoorKind) canBeTrapped() bool {
	return k == DoorClosed || k == DoorLocked || k == DoorSecret
}

// Side is a side of a room, and also a direction (a stair's facing). The
// order, north, east, south, west, is the fixed order of the spec.
type Side uint8

// The sides, clockwise from the north.
const (
	North Side = iota
	East
	South
	West
)

func (s Side) String() string {
	switch s {
	case North:
		return "north"
	case East:
		return "east"
	case South:
		return "south"
	case West:
		return "west"
	}
	return "?"
}

// Axis is the orientation of a door's wall (spec 1.3).
type Axis uint8

// The axes.
const (
	// HorizontalWall: the wall runs east-west and the door is crossed
	// moving north-south.
	HorizontalWall Axis = iota
	VerticalWall
)

func (a Axis) String() string {
	if a == HorizontalWall {
		return "horizontal_wall"
	}
	return "vertical_wall"
}

// StairKind is up or down.
type StairKind uint8

// The stair kinds.
const (
	StairUp StairKind = iota
	StairDown
)

func (k StairKind) String() string {
	if k == StairUp {
		return "stairs_up"
	}
	return "stairs_down"
}

// Exit is a door on a room's wall ring.
type Exit struct {
	Side    Side
	X, Y    int
	DoorID  int
	Kind    DoorKind
	Trapped bool
	// Other is the room on the far side when the door opens directly into
	// another room; 0 when it opens into a corridor.
	Other int
}

// Room is a rectangle of floor (spec 1.2). Bounds are the floor only; the
// wall ring around it is one square wider on every side.
type Room struct {
	ID            int
	X, Y          int
	Width, Height int
	// Exits are ordered by side (north, east, south, west) and position.
	Exits []Exit
}

// Center is the middle square (exact, the sides are odd).
func (r Room) Center() (x, y int) { return r.X + r.Width/2, r.Y + r.Height/2 }

// Area in squares.
func (r Room) Area() int { return r.Width * r.Height }

// Door is a marker on a wall-line square (spec 1.1, 1.3).
type Door struct {
	// ID counts from 1 in row-major order of the final grid.
	ID      int
	X, Y    int
	Kind    DoorKind
	Trapped bool
	Axis    Axis
	// RoomA is the room at the north (or west) end when that end is a room,
	// otherwise the room at the other end; Side is the side of RoomA's ring
	// the door sits on. RoomB is the other room of a direct door, 0 when the
	// door opens into a corridor, in which case Corridor is its ID.
	RoomA, RoomB int
	Side         Side
	Corridor     int
}

// Stair is a stairway overlay (spec 1.1).
type Stair struct {
	X, Y int
	Kind StairKind
	// Facing is the direction in which a creature walks onto the stair. For
	// a stair at a corridor's dead end it is away from the corridor; for the
	// fallback stair in a room corner (the "room site") it is west at the
	// west corners and east at the east ones.
	Facing Side
	// InRoom is the room's ID for a room site, 0 for a corridor stair.
	InRoom int
}

// Entrance is the level's arrival point (spec 1.4).
type Entrance struct {
	X, Y int
	// Room is the room it is in, or 0; Corridor the corridor, or 0.
	Room, Corridor int
	// OnStairs is true when it coincides with the first (up) stair.
	OnStairs bool
}

// CorridorEnd is where a corridor ends.
type CorridorEnd struct {
	// Type is "door", "dead_end" or "junction".
	Type   string
	DoorID int
	X, Y   int
}

// Corridor is a maximal run of corridor squares between junctions, dead
// ends and doors; a junction square (three or more open neighbors) is a
// corridor of its own, of one square (spec 1.5 d). IDs count from 1 in
// row-major order of the first square of each run.
type Corridor struct {
	ID             int
	FirstX, FirstY int
	Length         int
	Junction       bool
	// Junctions is how many distinct junction squares the run touches.
	Junctions int
	Ends      []CorridorEnd
}

// Dungeon is a generated level.
type Dungeon struct {
	// Version is the algorithm version that made it.
	Version int
	// Options are the effective options: sizes odd, room sides normalised.
	Options       Options
	Width, Height int
	// Kinds is the base kind of every square, row-major (index y*Width+x).
	Kinds []Kind
	// RoomIDs is the room ID of every room-floor square, 0 elsewhere.
	RoomIDs []uint16
	// CorridorIDs is the Corridor ID of every corridor square, 0 elsewhere.
	CorridorIDs []uint16
	// Mask is true where the shape mask lets the level be (false = blocked).
	Mask []bool
	// Rooms in ID order. A discarded room leaves a gap in the IDs.
	Rooms []Room
	// Doors in ID order, Corridors in ID order.
	Doors     []Door
	Corridors []Corridor
	// Stairs in placement order; the first is up and is the entrance.
	Stairs   []Stair
	Entrance Entrance
	// DiscardedRooms are the IDs of the rooms that could not be linked and
	// were erased (spec 3.6.3), ascending.
	DiscardedRooms []int
	// StairsPlaced can be less than Options.Stairs; that is not an error.
	StairsPlaced int
}

// Kind of the square at (x, y); KindRock outside the grid.
func (d *Dungeon) Kind(x, y int) Kind {
	if x < 0 || y < 0 || x >= d.Width || y >= d.Height {
		return KindRock
	}
	return d.Kinds[y*d.Width+x]
}

// Open reports whether a creature can stand at (x, y): room, corridor or door.
func (d *Dungeon) Open(x, y int) bool { return d.Kind(x, y) != KindRock }

// Room returns the room with that ID.
func (d *Dungeon) Room(id int) (Room, bool) {
	for _, r := range d.Rooms {
		if r.ID == id {
			return r, true
		}
	}
	return Room{}, false
}

// StairAt returns the stair on a square.
func (d *Dungeon) StairAt(x, y int) (Stair, bool) {
	for _, s := range d.Stairs {
		if s.X == x && s.Y == y {
			return s, true
		}
	}
	return Stair{}, false
}
