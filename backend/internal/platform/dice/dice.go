// Package dice parses dice expressions ("1d20+6"), rolls them and checks
// the result of dice rolled at the table (RN-18, MR-013, MR-014).
//
// The rules module is pure (ADR-0008), so it never rolls: the play module
// rolls here and passes the result in. Rolling goes through the Roller
// interface, so tests use a fixed sequence of faces (Fixed) and production
// uses the operating system's random source (Crypto).
package dice

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Limits of an expression. A table never rolls more than a handful of
// dice; the cap keeps a request from asking for a million.
const (
	MaxDice     = 100
	MaxModifier = 100
)

// validSides are the dice that exist at a table.
var validSides = []int{4, 6, 8, 10, 12, 20, 100}

// Expr is a parsed dice expression: Count dice of Sides faces, plus Modifier.
type Expr struct {
	Count    int
	Sides    int
	Modifier int
}

// Result is the outcome of a roll.
type Result struct {
	Expr Expr
	// Faces are the faces rolled, one per die. Empty for a physical roll
	// (Physical): the player only typed the sum.
	Faces    []int
	Modifier int
	// Total is the sum of the faces plus the modifier.
	Total int
	// Physical is true when the player rolled real dice and typed the sum.
	Physical bool
}

// Errors returned by Parse and Physical. Callers map them to
// invalid_argument.
var (
	ErrSyntax   = errors.New("dice: use an expression like 1d20+6")
	ErrCount    = fmt.Errorf("dice: the number of dice must be 1 to %d", MaxDice)
	ErrSides    = errors.New("dice: the dice can have 4, 6, 8, 10, 12, 20 or 100 sides")
	ErrModifier = fmt.Errorf("dice: the modifier must be between -%d and %d", MaxModifier, MaxModifier)
	ErrSum      = errors.New("dice: the typed result is out of range for these dice")
)

// exprPattern is the whole grammar: an optional count, "d", the sides and
// an optional signed modifier, with optional spaces around the sign.
var exprPattern = regexp.MustCompile(`^(\d{1,4})?d(\d{1,4})(?:\s*([+-])\s*(\d{1,4}))?$`)

// Parse reads an expression such as "d20", "1d20+6", "2d6 + 2" or "1d4 - 1".
// Case and surrounding spaces do not matter. Anything else is an error.
func Parse(s string) (Expr, error) {
	m := exprPattern.FindStringSubmatch(strings.ToLower(strings.TrimSpace(s)))
	if m == nil {
		return Expr{}, ErrSyntax
	}
	e := Expr{Count: 1}
	// The pattern only lets digits through, so Atoi cannot fail.
	if m[1] != "" {
		e.Count, _ = strconv.Atoi(m[1])
	}
	e.Sides, _ = strconv.Atoi(m[2])
	if m[4] != "" {
		e.Modifier, _ = strconv.Atoi(m[4])
		if m[3] == "-" {
			e.Modifier = -e.Modifier
		}
	}
	switch {
	case e.Count < 1 || e.Count > MaxDice:
		return Expr{}, ErrCount
	case !slices.Contains(validSides, e.Sides):
		return Expr{}, ErrSides
	case e.Modifier < -MaxModifier || e.Modifier > MaxModifier:
		return Expr{}, ErrModifier
	}
	return e, nil
}

// String writes the expression the way it is read: "2d6+2", "1d20-1".
func (e Expr) String() string {
	switch {
	case e.Modifier > 0:
		return fmt.Sprintf("%dd%d+%d", e.Count, e.Sides, e.Modifier)
	case e.Modifier < 0:
		return fmt.Sprintf("%dd%d%d", e.Count, e.Sides, e.Modifier)
	}
	return fmt.Sprintf("%dd%d", e.Count, e.Sides)
}

// Min is the lowest total the expression can give.
func (e Expr) Min() int { return e.Count + e.Modifier }

// Max is the highest total the expression can give.
func (e Expr) Max() int { return e.Count*e.Sides + e.Modifier }

// Roller gives the face of one die: a number from 1 to sides.
type Roller interface {
	Roll(sides int) (int, error)
}

// Roll rolls the expression with r.
func Roll(r Roller, e Expr) (Result, error) {
	res := Result{Expr: e, Modifier: e.Modifier, Total: e.Modifier, Faces: make([]int, 0, e.Count)}
	for range e.Count {
		face, err := r.Roll(e.Sides)
		if err != nil {
			return Result{}, fmt.Errorf("roll a d%d: %w", e.Sides, err)
		}
		res.Faces = append(res.Faces, face)
		res.Total += face
	}
	return res, nil
}

// Physical checks the sum a player typed after rolling real dice (RN-18,
// Q38): the player types the sum of the dice, without the modifier, and the
// app adds the modifier. The sum must be reachable with these dice, between
// Count and Count*Sides. The faces are unknown, so Faces is empty.
func Physical(e Expr, typedSum int) (Result, error) {
	if typedSum < e.Count || typedSum > e.Count*e.Sides {
		return Result{}, ErrSum
	}
	return Result{Expr: e, Modifier: e.Modifier, Total: typedSum + e.Modifier, Physical: true}, nil
}

// Crypto rolls with crypto/rand, so a roll cannot be predicted or steered.
type Crypto struct{}

// Roll implements Roller. rand.Int draws a uniform number below its bound
// (rejection sampling inside), so no face is more likely than another.
func (Crypto) Roll(sides int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(sides)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()) + 1, nil
}

// Fixed is a Roller for tests: it returns the given faces in order, and
// fails when they run out or a face does not fit the die.
type Fixed struct {
	Faces []int
	next  int
}

// Roll implements Roller.
func (f *Fixed) Roll(sides int) (int, error) {
	if f.next >= len(f.Faces) {
		return 0, errors.New("dice: Fixed ran out of faces")
	}
	face := f.Faces[f.next]
	f.next++
	if face < 1 || face > sides {
		return 0, fmt.Errorf("dice: Fixed face %d does not fit a d%d", face, sides)
	}
	return face, nil
}
