package encounter

import (
	"cmp"
	"errors"
	"slices"
)

// ErrNoCreature is what Generate answers when no creature of the pool fits: none
// has the type, or none is within the rating cap and the budget.
var ErrNoCreature = errors.New("encounter: no creature fits the options")

// The limits of a generated encounter. A kind of creature never has more than
// DefaultMaxPerKind copies (one "Pôr no combate" adds up to 10), and the whole
// encounter never has more than DefaultMaxTotal creatures, so a combat of 40 has
// room left for the party.
const (
	DefaultMaxPerKind = 10
	DefaultMaxTotal   = 24
	// attempts is how many leaders the generator tries before it keeps the encounter
	// that spent the most. Each try draws from the seed, so the answer still depends
	// on the seed alone.
	attempts = 48
)

// Options say what to generate.
type Options struct {
	// Budget is the XP of the band asked for, for the whole party. The encounter
	// never costs more.
	Budget int
	// MaxCR is the highest rating a creature may have, in eighths (MaxCR).
	MaxCR int
	// Type limits the creatures to an SRD type ("humanoid"); "" for any.
	Type string
	// Seed makes the choice: the same pool, options and seed give the same encounter.
	Seed uint32
	// MaxPerKind and MaxTotal limit the counts; 0 takes the defaults above.
	MaxPerKind, MaxTotal int
}

// Result is a generated encounter: the leader first, then the group by falling XP.
type Result struct {
	Entries []Entry
	// XP is what the encounter costs, never above Options.Budget.
	XP int
}

// Generate builds an encounter of one leader and a group, from the pool.
//
// The idea, in short. The leader is a single creature, strong but not the whole
// budget. The group is one kind of creature, or two, never stronger than the
// leader, and takes the rest: the first kind gets 50 to 70 % of what is left when
// there are two (all of it when there is one), the second what is still left,
// and then the dearest kinds are added while they still fit. Everything is drawn
// from the seed. The generator makes several such tries and keeps the one that
// spent the most ("spend as much of your XP budget as you can without going over",
// SRD 5.2.1), the first of them on a tie. To reach the band asked even for a
// lopsided party, the odd tries draw the leader from every candidate (the even
// ones from those that cost an eighth to six tenths of the budget), and a group
// kind is drawn, when it can, from those able to spend its share in the room
// there is (drawKind).
func Generate(pool []Creature, o Options) (Result, error) {
	perKind, total := o.MaxPerKind, o.MaxTotal
	if perKind <= 0 {
		perKind = DefaultMaxPerKind
	}
	if total <= 0 {
		total = DefaultMaxTotal
	}
	// The candidates, in key order, so the pool's own order never changes the answer.
	var cands []Creature
	for _, c := range pool {
		if c.XP > 0 && c.XP <= o.Budget && c.CR <= o.MaxCR && (o.Type == "" || c.Type == o.Type) {
			cands = append(cands, c)
		}
	}
	if len(cands) == 0 {
		return Result{}, ErrNoCreature
	}
	slices.SortFunc(cands, func(a, b Creature) int { return cmp.Compare(a.Key, b.Key) })

	leaders := pick(cands, func(c Creature) bool { return c.XP*8 >= o.Budget && c.XP*10 <= o.Budget*6 })
	if len(leaders) == 0 {
		leaders = pick(cands, func(c Creature) bool { return c.XP*10 <= o.Budget*6 })
	}
	if len(leaders) == 0 {
		leaders = cands
	}

	r := newRNG(o.Seed)
	var best Result
	for try := range attempts {
		// Half of the tries (the odd ones) draw the leader from every candidate: in a
		// party where the usual shares do not add up (a low budget and only dear
		// creatures, or the other way round) the leader that fills the band may be
		// the dearest or the cheapest one.
		pool := leaders
		if try%2 == 1 {
			pool = cands
		}
		leader := pool[r.intn(len(pool))]
		res := fill(cands, leader, o.Budget, perKind, total, &r)
		if best.Entries == nil || res.XP > best.XP {
			best = res
		}
		if best.XP == o.Budget {
			break
		}
	}
	return best, nil
}

func pick(cs []Creature, keep func(Creature) bool) []Creature {
	var out []Creature
	for _, c := range cs {
		if keep(c) {
			out = append(out, c)
		}
	}
	return out
}

// fill is one try: the leader, then the group.
func fill(cands []Creature, leader Creature, budget, perKind, total int, r *rng) Result {
	left := budget - leader.XP
	// The group is never stronger than the leader. A pool whose only creature is the
	// leader's own kind makes the group of that kind.
	group := pick(cands, func(c Creature) bool { return c.Key != leader.Key && c.CR <= leader.CR && c.XP <= left })
	if len(group) == 0 {
		group = pick(cands, func(c Creature) bool { return c.Key == leader.Key && c.XP <= left })
	}

	counts := map[string]int{leader.Key: 1}
	byKey := map[string]Creature{leader.Key: leader}
	used := 1
	add := func(c Creature, n int) {
		n = min(n, left/c.XP, perKind-counts[c.Key], total-used)
		if n <= 0 {
			return
		}
		counts[c.Key] += n
		byKey[c.Key] = c
		used += n
		left -= n * c.XP
	}

	if len(group) > 0 {
		two := len(group) > 1 && r.intn(2) == 0
		share := left
		if two {
			share = left * (50 + r.intn(21)) / 100
		}
		first := drawKind(group, "", share, perKind, total-used, r)
		kinds := []Creature{first}
		add(first, share/first.XP)
		if two {
			second := drawKind(group, first.Key, left, perKind, total-used, r)
			kinds = append(kinds, second)
			add(second, left/second.XP)
		}
		// Top up: the dearest kind first, as many as still fit.
		slices.SortFunc(kinds, func(a, b Creature) int { return cmp.Or(cmp.Compare(b.XP, a.XP), cmp.Compare(a.Key, b.Key)) })
		for _, c := range kinds {
			add(c, left/c.XP)
		}
	}

	res := Result{XP: budget - left}
	res.Entries = append(res.Entries, Entry{Key: leader.Key, Count: counts[leader.Key]})
	rest := make([]Creature, 0, len(counts))
	for k := range counts {
		if k != leader.Key {
			rest = append(rest, byKey[k])
		}
	}
	slices.SortFunc(rest, func(a, b Creature) int { return cmp.Or(cmp.Compare(b.XP, a.XP), cmp.Compare(a.Key, b.Key)) })
	for _, c := range rest {
		res.Entries = append(res.Entries, Entry{Key: c.Key, Count: counts[c.Key]})
	}
	return res
}

// drawKind draws a kind of creature from the group, never the one named but. It leans
// toward the kinds that can spend the XP in need within the room (a kind costs XP each
// and at most room copies): a kind of 10 XP cannot spend 1 000 XP in 10 copies, so
// while a dearer kind can, the cheap one is not drawn. When none can, the dearer half of the kinds is drawn from.
func drawKind(group []Creature, but string, need, perKind, room int, r *rng) Creature {
	room = min(perKind, room)
	var open, fit []Creature
	for _, c := range group {
		if c.Key == but {
			continue
		}
		open = append(open, c)
		if c.XP*room >= need {
			fit = append(fit, c)
		}
	}
	if len(fit) == 0 {
		// None can spend it all: the dearest half, which spend the most.
		top := 0
		for _, c := range open {
			top = max(top, c.XP)
		}
		fit = pick(open, func(c Creature) bool { return c.XP*2 >= top })
	}
	return fit[r.intn(len(fit))]
}
