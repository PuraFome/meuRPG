package play

import (
	"errors"
	"fmt"
	"math/bits"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/puzzle"
)

// A puzzleKind is everything the service needs to know about one kind of puzzle:
// how to check what the master wrote, how to start it, what a move does, when it is
// solved and what the fewest moves are. The rest of the service (storage, runs,
// idempotency, "Ao resolver", the hints, who sees what) never looks inside a kind.
// The riddle, the sequence and the cipher (slice 10.7b) are three more types here and
// three more cases in kindOf and kindByStored: what makes them different from the
// first three is only that a move can be wrong (judges), which the rest of the service
// handles for all kinds at once ("Ao errar").
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
	// judges says whether a move of this kind can be wrong (an answer, a bell). The
	// lights, the lock and the pillars cannot: whatever a player does is a move toward
	// the solution or away from it, never a mistake. A trap and the attempts of "Ao
	// errar" only make sense for a kind that judges.
	judges() bool
	// move applies a move to the state and returns the new state, what changed (the
	// indices of the cells, wheels or pillars) and, for a kind that judges, whether the
	// move was wrong. It is a pure function of the two: for the kinds that move
	// relatively, a replay in any order gives the same state.
	move(d puzzleDef, state *playv1.PuzzleState, mv *playv1.PuzzleMove) (moveOutcome, error)
	// solved says whether the state is the solution.
	solved(d puzzleDef, state *playv1.PuzzleState) (bool, error)
	// minimum is the fewest moves from the state, and one way to make them.
	minimum(d puzzleDef, state *playv1.PuzzleState) (*playv1.PuzzleMinimum, error)
	// playerConfig is the config as a player may read it.
	playerConfig(cfg *playv1.PuzzleConfig) *playv1.PuzzleConfig
	// symbols are the faces of the wheels or the pillars, in turning order.
	symbols(cfg *playv1.PuzzleConfig) []*playv1.PuzzleSymbol
}

// moveOutcome is what a move did: the state it left, what changed in it, and, for a
// kind that judges, whether it was wrong and, for the sequence, which step it was for
// (from 1).
type moveOutcome struct {
	state   *playv1.PuzzleState
	changed []int32
	wrong   bool
	step    int32
	// solved is set by a kind whose answer is judged on the move itself, not left in the
	// state (the riddle and the cipher: a right answer solves it, and the state has
	// nowhere to say so). The other kinds are solved when solved(state) says so.
	solved bool
}

// gatedKind is a kind whose moves wait for something besides the state: the sequence
// waits for the master to have played it, and for the play to end. gate says why a move
// is refused now, or nil.
type gatedKind interface {
	gate(d puzzleDef, run playdb.PuzzleRun, now time.Time) error
}

// puzzleBuilt is a puzzle's stored forms, checked.
type puzzleBuilt struct {
	config   *playv1.PuzzleConfig
	solution *playv1.PuzzleSolution
	start    *playv1.PuzzleState
	seed     int64
	// What every kind has the same (set by buildPuzzle, never by a kind): the hint
	// check, the split information and "Ao errar", checked.
	hintCheck *playv1.PuzzleHintCheck
	parts     []*playv1.PuzzlePart
	onWrong   *playv1.PuzzleOnWrong
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
	case *playv1.PuzzleConfig_Riddle:
		return riddleKind{}
	case *playv1.PuzzleConfig_Sequence:
		return sequenceKind{}
	case *playv1.PuzzleConfig_Cipher:
		return cipherKind{}
	}
	return nil
}

// allKinds is every kind, for finding one by its stored name.
var allKinds = []puzzleKind{lightsKind{}, lockKind{}, pillarsKind{}, riddleKind{}, sequenceKind{}, cipherKind{}}

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
	case errors.Is(err, puzzle.ErrAnswers):
		reason = playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_ANSWERS
	case errors.Is(err, puzzle.ErrCipher):
		reason = playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_CIPHER
	case errors.Is(err, puzzle.ErrSequence):
		reason = playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_SYMBOLS
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

func (lightsKind) judges() bool { return false }

func (lightsKind) move(d puzzleDef, state *playv1.PuzzleState, mv *playv1.PuzzleMove) (moveOutcome, error) {
	if mv.GetLights() == nil {
		return moveOutcome{}, wrongKind("move")
	}
	n := lightsSize(d.config)
	cur, err := lightsBits(n, state)
	if err != nil {
		return moveOutcome{}, err
	}
	next, changed, err := puzzle.LightsPress(n, cur, int(mv.GetLights().GetRow()), int(mv.GetLights().GetCol()))
	if err != nil {
		return moveOutcome{}, rulesError(err, "move.lights")
	}
	var cells []int32
	for i := range n * n {
		if changed>>i&1 == 1 {
			cells = append(cells, clamp32(i, 0, 1<<20))
		}
	}
	return moveOutcome{state: lightsState(n, next), changed: cells}, nil
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

func (lockKind) judges() bool { return false }

func (lockKind) move(d puzzleDef, state *playv1.PuzzleState, mv *playv1.PuzzleMove) (moveOutcome, error) {
	if mv.GetLock() == nil {
		return moveOutcome{}, wrongKind("move")
	}
	if state.GetLock() == nil {
		return moveOutcome{}, wrongKind("state")
	}
	next, err := puzzle.LockTurn(lockAlphabet(d.config), ints(state.GetLock().GetWheels()), int(mv.GetLock().GetWheel()), int(mv.GetLock().GetDelta()))
	if err != nil {
		return moveOutcome{}, rulesError(err, "move.lock")
	}
	return moveOutcome{state: lockState(next), changed: []int32{mv.GetLock().GetWheel()}}, nil
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

func (pillarsKind) judges() bool { return false }

func (pillarsKind) move(d puzzleDef, state *playv1.PuzzleState, mv *playv1.PuzzleMove) (moveOutcome, error) {
	if mv.GetPillars() == nil {
		return moveOutcome{}, wrongKind("move")
	}
	if state.GetPillars() == nil {
		return moveOutcome{}, wrongKind("state")
	}
	next, changed, err := pillarsRules(d.config).Turn(ints(state.GetPillars().GetPillars()), int(mv.GetPillars().GetPillar()), int(mv.GetPillars().GetDelta()))
	if err != nil {
		return moveOutcome{}, rulesError(err, "move.pillars")
	}
	return moveOutcome{state: pillarsState(next), changed: int32s(changed)}, nil
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

// --- the riddle ---

type riddleKind struct{}

func (riddleKind) id() playv1.PuzzleKind { return playv1.PuzzleKind_PUZZLE_KIND_RIDDLE }
func (riddleKind) stored() string        { return "riddle" }
func (riddleKind) generates() bool       { return false }
func (riddleKind) judges() bool          { return true }

// maxRiddleText is the longest riddle, in characters.
const maxRiddleText = 500

func riddleState() *playv1.PuzzleState {
	return &playv1.PuzzleState{Kind: &playv1.PuzzleState_Riddle{Riddle: &playv1.RiddleState{}}}
}

func (riddleKind) build(in puzzleInput, _ int64) (puzzleBuilt, error) {
	cfg := in.config.GetRiddle()
	if cfg == nil {
		return puzzleBuilt{}, wrongKind("config")
	}
	if in.solution.GetRiddle() == nil {
		return puzzleBuilt{}, puzzleInvalid(playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_KIND, "solution", "a riddle needs its answers")
	}
	text, err := checkedText(cfg.GetText(), 1, maxRiddleText, playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_TEXT, "config.riddle.text")
	if err != nil {
		return puzzleBuilt{}, err
	}
	answers := in.solution.GetRiddle().GetAnswers()
	if err := puzzle.ValidateAnswers(answers); err != nil {
		return puzzleBuilt{}, rulesError(err, "solution.riddle.answers")
	}
	trimmed := make([]string, len(answers))
	for i, a := range answers {
		trimmed[i] = strings.TrimSpace(a)
	}
	return puzzleBuilt{
		config:   &playv1.PuzzleConfig{Kind: &playv1.PuzzleConfig_Riddle{Riddle: &playv1.RiddleConfig{Text: text}}},
		solution: &playv1.PuzzleSolution{Kind: &playv1.PuzzleSolution_Riddle{Riddle: &playv1.RiddleSolution{Answers: trimmed}}},
		start:    riddleState(),
	}, nil
}

func (riddleKind) generate(puzzleDef, int64) (*playv1.PuzzleState, error) {
	return nil, errors.New("play: a riddle has no generated start")
}

// typedText checks what a player typed: 1 to MaxTyped characters. It names the move's
// field, never the text.
func typedText(raw, field string) error {
	if n := utf8.RuneCountInString(strings.TrimSpace(raw)); n < 1 || n > puzzle.MaxTyped {
		return puzzleInvalid(playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_MOVE, field, fmt.Sprintf("%s must have 1 to %d characters", field, puzzle.MaxTyped))
	}
	return nil
}

func (riddleKind) move(d puzzleDef, state *playv1.PuzzleState, mv *playv1.PuzzleMove) (moveOutcome, error) {
	if mv.GetRiddle() == nil {
		return moveOutcome{}, wrongKind("move")
	}
	if state.GetRiddle() == nil || d.solution.GetRiddle() == nil {
		return moveOutcome{}, wrongKind("state")
	}
	if err := typedText(mv.GetRiddle().GetAnswer(), "move.riddle.answer"); err != nil {
		return moveOutcome{}, err
	}
	right := puzzle.Matches(d.solution.GetRiddle().GetAnswers(), mv.GetRiddle().GetAnswer())
	return moveOutcome{state: state, wrong: !right, solved: right}, nil
}

// solved is always false: the riddle's state is empty, and a right answer says so in
// the move's outcome.
func (riddleKind) solved(_ puzzleDef, _ *playv1.PuzzleState) (bool, error) { return false, nil }

func (riddleKind) minimum(_ puzzleDef, _ *playv1.PuzzleState) (*playv1.PuzzleMinimum, error) {
	return &playv1.PuzzleMinimum{Solvable: true, Moves: 1}, nil
}

func (riddleKind) playerConfig(cfg *playv1.PuzzleConfig) *playv1.PuzzleConfig { return cfg }
func (riddleKind) symbols(*playv1.PuzzleConfig) []*playv1.PuzzleSymbol        { return nil }

// --- the sequence ---

type sequenceKind struct{}

func (sequenceKind) id() playv1.PuzzleKind { return playv1.PuzzleKind_PUZZLE_KIND_SEQUENCE }
func (sequenceKind) stored() string        { return "sequence" }
func (sequenceKind) generates() bool       { return false }
func (sequenceKind) judges() bool          { return true }

func sequenceState(progress int) *playv1.PuzzleState {
	return &playv1.PuzzleState{Kind: &playv1.PuzzleState_Sequence{Sequence: &playv1.SequenceState{Progress: clamp32(progress, 0, puzzle.MaxSteps)}}}
}

func (sequenceKind) build(in puzzleInput, _ int64) (puzzleBuilt, error) {
	cfg := in.config.GetSequence()
	if cfg == nil {
		return puzzleBuilt{}, wrongKind("config")
	}
	if in.solution.GetSequence() == nil {
		return puzzleBuilt{}, puzzleInvalid(playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_KIND, "solution", "a sequence needs its steps")
	}
	steps := ints(in.solution.GetSequence().GetSteps())
	if err := puzzle.ValidateSequence(int(cfg.GetBells()), steps); err != nil {
		field := "config.sequence.bells"
		switch {
		case errors.Is(err, puzzle.ErrSymbols), errors.Is(err, puzzle.ErrSequence):
			field = "solution.sequence.steps"
		case len(steps) < puzzle.MinSteps || len(steps) > puzzle.MaxSteps:
			field = "solution.sequence.steps"
		}
		return puzzleBuilt{}, rulesError(err, field)
	}
	return puzzleBuilt{
		// The number of steps is the solution's: whatever the request said is ignored.
		config:   &playv1.PuzzleConfig{Kind: &playv1.PuzzleConfig_Sequence{Sequence: &playv1.SequenceConfig{Bells: cfg.GetBells(), Steps: clamp32(len(steps), 0, puzzle.MaxSteps)}}},
		solution: &playv1.PuzzleSolution{Kind: &playv1.PuzzleSolution_Sequence{Sequence: &playv1.SequenceSolution{Steps: slices.Clone(in.solution.GetSequence().GetSteps())}}},
		start:    sequenceState(0),
	}, nil
}

func (sequenceKind) generate(puzzleDef, int64) (*playv1.PuzzleState, error) {
	return nil, errors.New("play: a sequence has no generated start")
}

func (sequenceKind) move(d puzzleDef, state *playv1.PuzzleState, mv *playv1.PuzzleMove) (moveOutcome, error) {
	if mv.GetSequence() == nil {
		return moveOutcome{}, wrongKind("move")
	}
	if state.GetSequence() == nil || d.solution.GetSequence() == nil {
		return moveOutcome{}, wrongKind("state")
	}
	steps := ints(d.solution.GetSequence().GetSteps())
	bell := int(mv.GetSequence().GetBell())
	if bell < 0 || bell >= int(d.config.GetSequence().GetBells()) {
		return moveOutcome{}, puzzleInvalid(playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_MOVE, "move.sequence.bell", "move.sequence.bell is not one of the bells")
	}
	next, wrong, _, step, err := puzzle.SequenceStrike(steps, int(state.GetSequence().GetProgress()), bell)
	if err != nil {
		return moveOutcome{}, rulesError(err, "move.sequence")
	}
	return moveOutcome{state: sequenceState(next), wrong: wrong, step: clamp32(step, 0, puzzle.MaxSteps)}, nil
}

func (sequenceKind) solved(d puzzleDef, state *playv1.PuzzleState) (bool, error) {
	if state.GetSequence() == nil || d.solution.GetSequence() == nil {
		return false, wrongKind("state")
	}
	return int(state.GetSequence().GetProgress()) >= len(d.solution.GetSequence().GetSteps()), nil
}

func (sequenceKind) minimum(d puzzleDef, state *playv1.PuzzleState) (*playv1.PuzzleMinimum, error) {
	if state.GetSequence() == nil || d.solution.GetSequence() == nil {
		return nil, wrongKind("state")
	}
	steps := d.solution.GetSequence().GetSteps()
	done := min(max(int(state.GetSequence().GetProgress()), 0), len(steps))
	out := &playv1.PuzzleMinimum{Solvable: true, Moves: clamp32(len(steps)-done, 0, puzzle.MaxSteps)}
	for _, b := range steps[done:] {
		out.Path = append(out.Path, &playv1.PuzzleMove{Kind: &playv1.PuzzleMove_Sequence{Sequence: &playv1.SequenceMove{Bell: b}}})
	}
	return out, nil
}

func (sequenceKind) playerConfig(cfg *playv1.PuzzleConfig) *playv1.PuzzleConfig { return cfg }
func (sequenceKind) symbols(cfg *playv1.PuzzleConfig) []*playv1.PuzzleSymbol {
	return symbolsOf(puzzle.Bells(int(cfg.GetSequence().GetBells())))
}

// gate: a sequence is struck only after the master has played it, and not while it
// plays: what the players repeat is what they watched.
func (sequenceKind) gate(d puzzleDef, run playdb.PuzzleRun, now time.Time) error {
	if run.Plays == 0 {
		return puzzleBlocked(playv1.PuzzleBlockedReason_PUZZLE_BLOCKED_REASON_SEQUENCE_NOT_PLAYED, "the master has not played the sequence yet")
	}
	if playing, _ := sequencePlaying(d, run, now); playing {
		return puzzleBlocked(playv1.PuzzleBlockedReason_PUZZLE_BLOCKED_REASON_SEQUENCE_PLAYING, "the sequence is being played")
	}
	return nil
}

// sequencePlaying says whether a play is running at now, and how many steps it has
// revealed (the first at once) and how long until the next change.
func sequencePlaying(d puzzleDef, run playdb.PuzzleRun, now time.Time) (bool, sequenceReveal) {
	n := len(d.solution.GetSequence().GetSteps())
	if run.PlayStartedAt == nil || n == 0 {
		return false, sequenceReveal{}
	}
	shown, playing, next := puzzle.SequencePlayback(n, now.Sub(*run.PlayStartedAt))
	return playing, sequenceReveal{shown: shown, next: next}
}

// sequenceReveal is how far a play has gone.
type sequenceReveal struct {
	shown int
	next  time.Duration
}

// --- the cipher ---

type cipherKind struct{}

func (cipherKind) id() playv1.PuzzleKind { return playv1.PuzzleKind_PUZZLE_KIND_CIPHER }
func (cipherKind) stored() string        { return "cipher" }
func (cipherKind) generates() bool       { return false }
func (cipherKind) judges() bool          { return true }

func cipherState() *playv1.PuzzleState {
	return &playv1.PuzzleState{Kind: &playv1.PuzzleState_Cipher{Cipher: &playv1.CipherState{}}}
}

// cipherKey is the rules package's view of a solution's key.
func cipherKey(sol *playv1.CipherSolution) puzzle.CipherKey {
	return puzzle.CipherKey{Shift: int(sol.GetShift()), Keyword: sol.GetKeyword()}
}

func (cipherKind) build(in puzzleInput, _ int64) (puzzleBuilt, error) {
	cfg := in.config.GetCipher()
	if cfg == nil {
		return puzzleBuilt{}, wrongKind("config")
	}
	sol := in.solution.GetCipher()
	if sol == nil {
		return puzzleBuilt{}, puzzleInvalid(playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_KIND, "solution", "a cipher needs its message and key")
	}
	message := strings.TrimSpace(sol.GetMessage())
	ciphertext, err := puzzle.Encipher(message, cipherKey(sol))
	if err != nil {
		return puzzleBuilt{}, rulesError(err, "solution.cipher")
	}
	stored := &playv1.CipherSolution{Message: message}
	if sol.GetKeyword() != "" {
		stored.Method = &playv1.CipherSolution_Keyword{Keyword: strings.TrimSpace(sol.GetKeyword())}
	} else {
		stored.Method = &playv1.CipherSolution_Shift{Shift: sol.GetShift()}
	}
	return puzzleBuilt{
		config:   &playv1.PuzzleConfig{Kind: &playv1.PuzzleConfig_Cipher{Cipher: &playv1.CipherConfig{Ciphertext: ciphertext, KeyClueId: cfg.GetKeyClueId()}}},
		solution: &playv1.PuzzleSolution{Kind: &playv1.PuzzleSolution_Cipher{Cipher: stored}},
		start:    cipherState(),
	}, nil
}

func (cipherKind) generate(puzzleDef, int64) (*playv1.PuzzleState, error) {
	return nil, errors.New("play: a cipher has no generated start")
}

func (cipherKind) move(d puzzleDef, state *playv1.PuzzleState, mv *playv1.PuzzleMove) (moveOutcome, error) {
	if mv.GetCipher() == nil {
		return moveOutcome{}, wrongKind("move")
	}
	if state.GetCipher() == nil || d.solution.GetCipher() == nil {
		return moveOutcome{}, wrongKind("state")
	}
	if err := typedText(mv.GetCipher().GetText(), "move.cipher.text"); err != nil {
		return moveOutcome{}, err
	}
	right := puzzle.Matches([]string{d.solution.GetCipher().GetMessage()}, mv.GetCipher().GetText())
	return moveOutcome{state: state, wrong: !right, solved: right}, nil
}

// solved is always false: see riddleKind.solved.
func (cipherKind) solved(_ puzzleDef, _ *playv1.PuzzleState) (bool, error) { return false, nil }

func (cipherKind) minimum(_ puzzleDef, _ *playv1.PuzzleState) (*playv1.PuzzleMinimum, error) {
	return &playv1.PuzzleMinimum{Solvable: true, Moves: 1}, nil
}

// playerConfig is the ciphered message and nothing else: which clue holds the key stays
// with the master (a player reads PuzzleRun.has_key_clue, and key_clue_id once found).
func (cipherKind) playerConfig(cfg *playv1.PuzzleConfig) *playv1.PuzzleConfig {
	return &playv1.PuzzleConfig{Kind: &playv1.PuzzleConfig_Cipher{Cipher: &playv1.CipherConfig{Ciphertext: cfg.GetCipher().GetCiphertext()}}}
}
func (cipherKind) symbols(*playv1.PuzzleConfig) []*playv1.PuzzleSymbol { return nil }
