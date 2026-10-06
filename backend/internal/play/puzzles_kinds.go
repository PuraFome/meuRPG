package play

import (
	"errors"
	"fmt"
	"math/bits"
	"slices"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules/puzzle"
)

// A puzzleKind is everything the service needs to know about one kind of puzzle:
// how to check what the master wrote, how to start it, what a move does, when it is
// solved and what the fewest moves are. The rest of the service (storage, runs,
// idempotency, "Ao resolver", the hints, who sees what) never looks inside a kind.
// Slice 10.7b adds the riddle, the sequence and the cipher: one more type here, and
// a case in kindOf.
type puzzleKind interface {
	// id is the kind as the API names it.
	id() playv1.PuzzleKind
	// stored is the kind as puzzles.kind keeps it.
	stored() string
	// generates says whether the start is drawn from a seed (lights, pillars) or is
	// the master's choice (the lock).
	generates() bool
	// build checks what the master wrote and returns its stored forms: the start is
	// drawn from seed when the kind generates it, and taken from in.start otherwise.
	build(in puzzleInput, seed int64) (puzzleBuilt, error)
	// generate draws a start for the seed from a built puzzle (never for a kind that
	// does not generate).
	generate(d puzzleDef, seed int64) (*playv1.PuzzleState, error)
	// move applies a relative move to the state and returns the new state and what
	// changed (the indices of the cells, wheels or pillars). It is a pure function
	// of the two: a replay in any order gives the same state.
	move(d puzzleDef, state *playv1.PuzzleState, mv *playv1.PuzzleMove) (*playv1.PuzzleState, []int32, error)
	// solved says whether the state is the solution.
	solved(d puzzleDef, state *playv1.PuzzleState) (bool, error)
	// minimum is the fewest moves from the state, and one way to make them.
	minimum(d puzzleDef, state *playv1.PuzzleState) (*playv1.PuzzleMinimum, error)
	// playerConfig is the config as a player may read it.
	playerConfig(cfg *playv1.PuzzleConfig) *playv1.PuzzleConfig
	// symbols are the faces of the wheels or the pillars, in turning order.
	symbols(cfg *playv1.PuzzleConfig) []*playv1.PuzzleSymbol
}

// puzzleBuilt is a puzzle's stored forms, checked.
type puzzleBuilt struct {
	config   *playv1.PuzzleConfig
	solution *playv1.PuzzleSolution
	start    *playv1.PuzzleState
	seed     int64
}

// kindOf finds the kind of a config, nil for a config with none or one this server
// does not know.
func kindOf(cfg *playv1.PuzzleConfig) puzzleKind {
	switch cfg.GetKind().(type) {
	case *playv1.PuzzleConfig_Lights:
		return lightsKind{}
	case *playv1.PuzzleConfig_Lock:
		return lockKind{}
	case *playv1.PuzzleConfig_Pillars:
		return pillarsKind{}
	}
	return nil
}

// rulesError turns an error of package rules/puzzle into the typed refusal the app
// reads. field is the request field to blame.
func rulesError(err error, field string) error {
	var reason playv1.PuzzleInvalidReason
	switch {
	case errors.Is(err, puzzle.ErrSize):
		reason = playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_SIZE
	case errors.Is(err, puzzle.ErrSymbols):
		reason = playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_SYMBOLS
	case errors.Is(err, puzzle.ErrLinks):
		reason = playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_LINKS
	case errors.Is(err, puzzle.ErrSolved):
		reason = playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_START_SOLVED
	case errors.Is(err, puzzle.ErrUnsolvable):
		reason = playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_UNSOLVABLE
	case errors.Is(err, puzzle.ErrMove):
		reason = playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_MOVE
	default:
		return err
	}
	return puzzleInvalid(reason, field, err.Error())
}

func wrongKind(field string) error {
	return puzzleInvalid(playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_KIND, field, field+" is not of the puzzle's kind")
}

// seedOf is the generator's seed for a stored one (bit for bit).
func seedOf(seed int64) uint64 { return uint64(seed) } //nolint:gosec // the seed is only a number to start the generator

func int32s(in []int) []int32 {
	out := make([]int32, len(in))
	for i, v := range in {
		out[i] = clamp32(v, 0, 1<<20)
	}
	return out
}

func ints(in []int32) []int {
	out := make([]int, len(in))
	for i, v := range in {
		out[i] = int(v)
	}
	return out
}

func symbolsOf(in []puzzle.Symbol) []*playv1.PuzzleSymbol {
	out := make([]*playv1.PuzzleSymbol, len(in))
	for i, s := range in {
		out[i] = &playv1.PuzzleSymbol{Key: s.Key, NamePt: s.NamePT}
	}
	return out
}

// --- "Apagar as luzes" ---

type lightsKind struct{}

func (lightsKind) id() playv1.PuzzleKind { return playv1.PuzzleKind_PUZZLE_KIND_LIGHTS }
func (lightsKind) stored() string        { return "lights" }
func (lightsKind) generates() bool       { return true }

func lightsSize(cfg *playv1.PuzzleConfig) int { return int(cfg.GetLights().GetSize()) }

func lightsState(n int, state uint64) *playv1.PuzzleState {
	return &playv1.PuzzleState{Kind: &playv1.PuzzleState_Lights{Lights: &playv1.LightsState{Lit: puzzle.LightsToBools(n, state)}}}
}

// lightsBits reads the board of a state.
func lightsBits(n int, state *playv1.PuzzleState) (uint64, error) {
	if state.GetLights() == nil {
		return 0, wrongKind("state")
	}
	bitsOf, err := puzzle.LightsFromBools(n, state.GetLights().GetLit())
	if err != nil {
		return 0, rulesError(err, "state")
	}
	return bitsOf, nil
}

func (k lightsKind) build(in puzzleInput, seed int64) (puzzleBuilt, error) {
	if in.config.GetLights() == nil {
		return puzzleBuilt{}, wrongKind("config")
	}
	n := lightsSize(in.config)
	start, err := puzzle.LightsStart(n, seedOf(seed))
	if err != nil {
		return puzzleBuilt{}, rulesError(err, "config.lights.size")
	}
	return puzzleBuilt{
		config:   &playv1.PuzzleConfig{Kind: &playv1.PuzzleConfig_Lights{Lights: &playv1.LightsConfig{Size: in.config.GetLights().GetSize()}}},
		solution: &playv1.PuzzleSolution{},
		start:    lightsState(n, start),
		seed:     seed,
	}, nil
}

func (lightsKind) generate(d puzzleDef, seed int64) (*playv1.PuzzleState, error) {
	n := lightsSize(d.config)
	start, err := puzzle.LightsStart(n, seedOf(seed))
	if err != nil {
		return nil, rulesError(err, "config.lights.size")
	}
	return lightsState(n, start), nil
}

func (lightsKind) move(d puzzleDef, state *playv1.PuzzleState, mv *playv1.PuzzleMove) (*playv1.PuzzleState, []int32, error) {
	if mv.GetLights() == nil {
		return nil, nil, wrongKind("move")
	}
	n := lightsSize(d.config)
	cur, err := lightsBits(n, state)
	if err != nil {
		return nil, nil, err
	}
	next, changed, err := puzzle.LightsPress(n, cur, int(mv.GetLights().GetRow()), int(mv.GetLights().GetCol()))
	if err != nil {
		return nil, nil, rulesError(err, "move.lights")
	}
	var cells []int32
	for i := range n * n {
		if changed>>i&1 == 1 {
			cells = append(cells, clamp32(i, 0, 1<<20))
		}
	}
	return lightsState(n, next), cells, nil
}

func (lightsKind) solved(d puzzleDef, state *playv1.PuzzleState) (bool, error) {
	cur, err := lightsBits(lightsSize(d.config), state)
	return puzzle.LightsSolved(cur), err
}

func (lightsKind) minimum(d puzzleDef, state *playv1.PuzzleState) (*playv1.PuzzleMinimum, error) {
	n := lightsSize(d.config)
	cur, err := lightsBits(n, state)
	if err != nil {
		return nil, err
	}
	sol, ok, err := puzzle.LightsMinimum(n, cur)
	if err != nil {
		return nil, rulesError(err, "config.lights.size")
	}
	out := &playv1.PuzzleMinimum{Solvable: ok}
	if !ok {
		return out, nil
	}
	out.Moves = clamp32(sol.Presses, 0, 64)
	for i := range n * n {
		if sol.Cells>>i&1 == 1 {
			out.Path = append(out.Path, &playv1.PuzzleMove{Kind: &playv1.PuzzleMove_Lights{Lights: &playv1.LightsMove{Row: clamp32(i/n, 0, 7), Col: clamp32(i%n, 0, 7)}}})
		}
	}
	if bits.OnesCount64(sol.Cells) != sol.Presses {
		return nil, fmt.Errorf("play: a minimum of %d presses names %d cells", sol.Presses, bits.OnesCount64(sol.Cells))
	}
	return out, nil
}

func (lightsKind) playerConfig(cfg *playv1.PuzzleConfig) *playv1.PuzzleConfig { return cfg }
func (lightsKind) symbols(*playv1.PuzzleConfig) []*playv1.PuzzleSymbol        { return nil }

// --- the combination lock ---

type lockKind struct{}

func (lockKind) id() playv1.PuzzleKind { return playv1.PuzzleKind_PUZZLE_KIND_LOCK }
func (lockKind) stored() string        { return "lock" }
func (lockKind) generates() bool       { return false }

func lockAlphabet(cfg *playv1.PuzzleConfig) puzzle.Alphabet {
	switch cfg.GetLock().GetAlphabet() {
	case playv1.PuzzleAlphabet_PUZZLE_ALPHABET_DIGITS:
		return puzzle.Digits
	case playv1.PuzzleAlphabet_PUZZLE_ALPHABET_LETTERS:
		return puzzle.Letters
	case playv1.PuzzleAlphabet_PUZZLE_ALPHABET_RUNES:
		return puzzle.Runes
	}
	return 0
}

func lockState(wheels []int) *playv1.PuzzleState {
	return &playv1.PuzzleState{Kind: &playv1.PuzzleState_Lock{Lock: &playv1.LockState{Wheels: int32s(wheels)}}}
}

func (lockKind) build(in puzzleInput, _ int64) (puzzleBuilt, error) {
	cfg := in.config.GetLock()
	if cfg == nil {
		return puzzleBuilt{}, wrongKind("config")
	}
	if in.solution.GetLock() == nil {
		return puzzleBuilt{}, puzzleInvalid(playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_KIND, "solution", "a lock needs its solution")
	}
	if in.start.GetLock() == nil {
		return puzzleBuilt{}, puzzleInvalid(playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_KIND, "start", "a lock needs its start")
	}
	solution, start := ints(in.solution.GetLock().GetWheels()), ints(in.start.GetLock().GetWheels())
	if err := puzzle.ValidateLock(int(cfg.GetWheels()), lockAlphabet(in.config), solution, start); err != nil {
		field := "config.lock"
		switch {
		case errors.Is(err, puzzle.ErrSolved):
			field = "start"
		case errors.Is(err, puzzle.ErrSymbols):
			field = "solution"
		}
		return puzzleBuilt{}, rulesError(err, field)
	}
	return puzzleBuilt{
		config:   &playv1.PuzzleConfig{Kind: &playv1.PuzzleConfig_Lock{Lock: &playv1.LockConfig{Wheels: cfg.GetWheels(), Alphabet: cfg.GetAlphabet()}}},
		solution: &playv1.PuzzleSolution{Kind: &playv1.PuzzleSolution_Lock{Lock: &playv1.LockSolution{Wheels: in.solution.GetLock().GetWheels()}}},
		start:    lockState(start),
	}, nil
}

func (lockKind) generate(puzzleDef, int64) (*playv1.PuzzleState, error) {
	return nil, errors.New("play: a lock has no generated start")
}

func (lockKind) move(d puzzleDef, state *playv1.PuzzleState, mv *playv1.PuzzleMove) (*playv1.PuzzleState, []int32, error) {
	if mv.GetLock() == nil {
		return nil, nil, wrongKind("move")
	}
	if state.GetLock() == nil {
		return nil, nil, wrongKind("state")
	}
	next, err := puzzle.LockTurn(lockAlphabet(d.config), ints(state.GetLock().GetWheels()), int(mv.GetLock().GetWheel()), int(mv.GetLock().GetDelta()))
	if err != nil {
		return nil, nil, rulesError(err, "move.lock")
	}
	return lockState(next), []int32{mv.GetLock().GetWheel()}, nil
}

func (lockKind) solved(d puzzleDef, state *playv1.PuzzleState) (bool, error) {
	if state.GetLock() == nil || d.solution.GetLock() == nil {
		return false, wrongKind("state")
	}
	return puzzle.LockSolved(ints(state.GetLock().GetWheels()), ints(d.solution.GetLock().GetWheels())), nil
}

func (lockKind) minimum(d puzzleDef, state *playv1.PuzzleState) (*playv1.PuzzleMinimum, error) {
	if state.GetLock() == nil || d.solution.GetLock() == nil {
		return nil, wrongKind("state")
	}
	moves, path := puzzle.LockMinimum(lockAlphabet(d.config), ints(state.GetLock().GetWheels()), ints(d.solution.GetLock().GetWheels()))
	out := &playv1.PuzzleMinimum{Solvable: true, Moves: clamp32(moves, 0, 1<<20)}
	for _, st := range path {
		out.Path = append(out.Path, &playv1.PuzzleMove{Kind: &playv1.PuzzleMove_Lock{Lock: &playv1.LockMove{Wheel: clamp32(st.Wheel, 0, 6), Delta: clamp32(st.Delta, -1, 1)}}})
	}
	return out, nil
}

func (lockKind) playerConfig(cfg *playv1.PuzzleConfig) *playv1.PuzzleConfig { return cfg }
func (lockKind) symbols(cfg *playv1.PuzzleConfig) []*playv1.PuzzleSymbol {
	return symbolsOf(lockAlphabet(cfg).Symbols())
}

// --- the turning symbols ---

type pillarsKind struct{}

func (pillarsKind) id() playv1.PuzzleKind { return playv1.PuzzleKind_PUZZLE_KIND_PILLARS }
func (pillarsKind) stored() string        { return "pillars" }
func (pillarsKind) generates() bool       { return true }

// pillarsRules is the rules package's view of a config.
func pillarsRules(cfg *playv1.PuzzleConfig) puzzle.Pillars {
	c := cfg.GetPillars()
	p := puzzle.Pillars{Count: int(c.GetPillars()), Glyphs: int(c.GetSymbols())}
	for _, l := range c.GetLinks() {
		p.Links = append(p.Links, ints(l.GetAlsoTurns()))
	}
	return p
}

func pillarsState(state []int) *playv1.PuzzleState {
	return &playv1.PuzzleState{Kind: &playv1.PuzzleState_Pillars{Pillars: &playv1.PillarsState{Pillars: int32s(state)}}}
}

func (pillarsKind) build(in puzzleInput, seed int64) (puzzleBuilt, error) {
	cfg := in.config.GetPillars()
	if cfg == nil {
		return puzzleBuilt{}, wrongKind("config")
	}
	if in.solution.GetPillars() == nil {
		return puzzleBuilt{}, puzzleInvalid(playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_KIND, "solution", "the pillars need their mural")
	}
	p := pillarsRules(in.config)
	if err := p.Validate(); err != nil {
		field := "config.pillars"
		if errors.Is(err, puzzle.ErrLinks) {
			field = "config.pillars.links"
		}
		return puzzleBuilt{}, rulesError(err, field)
	}
	target := ints(in.solution.GetPillars().GetPillars())
	if err := p.CheckState(target); err != nil {
		return puzzleBuilt{}, rulesError(err, "solution")
	}
	start, err := p.Start(target, seedOf(seed))
	if err != nil {
		return puzzleBuilt{}, rulesError(err, "config.pillars")
	}
	if _, _, ok, err := p.Minimum(start, target); err != nil || !ok {
		return puzzleBuilt{}, rulesError(errors.Join(puzzle.ErrUnsolvable, err), "solution")
	}
	links := make([]*playv1.PillarLinks, len(cfg.GetLinks()))
	for i, l := range cfg.GetLinks() {
		links[i] = &playv1.PillarLinks{AlsoTurns: slices.Clone(l.GetAlsoTurns())}
	}
	return puzzleBuilt{
		config:   &playv1.PuzzleConfig{Kind: &playv1.PuzzleConfig_Pillars{Pillars: &playv1.PillarsConfig{Pillars: cfg.GetPillars(), Symbols: cfg.GetSymbols(), Links: links}}},
		solution: &playv1.PuzzleSolution{Kind: &playv1.PuzzleSolution_Pillars{Pillars: &playv1.PillarsSolution{Pillars: slices.Clone(in.solution.GetPillars().GetPillars())}}},
		start:    pillarsState(start),
		seed:     seed,
	}, nil
}

func (pillarsKind) generate(d puzzleDef, seed int64) (*playv1.PuzzleState, error) {
	start, err := pillarsRules(d.config).Start(ints(d.solution.GetPillars().GetPillars()), seedOf(seed))
	if err != nil {
		return nil, rulesError(err, "config.pillars")
	}
	return pillarsState(start), nil
}

func (pillarsKind) move(d puzzleDef, state *playv1.PuzzleState, mv *playv1.PuzzleMove) (*playv1.PuzzleState, []int32, error) {
	if mv.GetPillars() == nil {
		return nil, nil, wrongKind("move")
	}
	if state.GetPillars() == nil {
		return nil, nil, wrongKind("state")
	}
	next, changed, err := pillarsRules(d.config).Turn(ints(state.GetPillars().GetPillars()), int(mv.GetPillars().GetPillar()), int(mv.GetPillars().GetDelta()))
	if err != nil {
		return nil, nil, rulesError(err, "move.pillars")
	}
	return pillarsState(next), int32s(changed), nil
}

func (pillarsKind) solved(d puzzleDef, state *playv1.PuzzleState) (bool, error) {
	if state.GetPillars() == nil || d.solution.GetPillars() == nil {
		return false, wrongKind("state")
	}
	return puzzle.PillarsSolved(ints(state.GetPillars().GetPillars()), ints(d.solution.GetPillars().GetPillars())), nil
}

func (pillarsKind) minimum(d puzzleDef, state *playv1.PuzzleState) (*playv1.PuzzleMinimum, error) {
	if state.GetPillars() == nil || d.solution.GetPillars() == nil {
		return nil, wrongKind("state")
	}
	moves, path, ok, err := pillarsRules(d.config).Minimum(ints(state.GetPillars().GetPillars()), ints(d.solution.GetPillars().GetPillars()))
	if err != nil {
		return nil, rulesError(err, "config.pillars")
	}
	out := &playv1.PuzzleMinimum{Solvable: ok}
	if !ok {
		return out, nil
	}
	out.Moves = clamp32(moves, 0, 1<<20)
	for _, st := range path {
		out.Path = append(out.Path, &playv1.PuzzleMove{Kind: &playv1.PuzzleMove_Pillars{Pillars: &playv1.PillarsMove{Pillar: clamp32(st.Pillar, 0, 6), Delta: clamp32(st.Delta, -1, 1)}}})
	}
	return out, nil
}

// playerConfig keeps the links: the players read how the pillars turn together
// ("Girar um pilar gira também o da esquerda e o da direita", artboard E10-06), as
// they read the mural; the challenge is the moves, not guessing the rules.
func (pillarsKind) playerConfig(cfg *playv1.PuzzleConfig) *playv1.PuzzleConfig { return cfg }

func (pillarsKind) symbols(cfg *playv1.PuzzleConfig) []*playv1.PuzzleSymbol {
	return symbolsOf(puzzle.Glyphs(int(cfg.GetPillars().GetSymbols())))
}
