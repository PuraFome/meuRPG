// Package cutting holds the rules of Cutting Words ("Palavras de Interrupção", SRD 5.1, College of Lore) on an ability check a creature
// of the master's side makes outside a combat, over plain values: which bard is asked, whether it sees the creature, what the creature is
// called to it, the order of the questions and what the check comes to. The questions, the dice and what they spend are the play
// package's.
package cutting

import (
	"slices"

	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
	"github.com/PuraFome/meuRPG/backend/internal/rules/reaction"
)

// UnnamedLabel is how a bard's question calls a creature the bard's player does not see.
const UnnamedLabel = "uma criatura"

// Bard is a bard of the party as the question of a check reads it.
type Bard struct {
	// Feature says the character has Cutting Words.
	Feature bool
	// Ask is the bard's "Perguntar" setting: "all", "attacks" (the default, also what "" means) or "never".
	Ask string
	// HitPoints are the current hit points: a bard at 0 is unconscious.
	HitPoints int
	// UsesLeft are the uses of Bardic Inspiration the bard has left.
	UsesLeft int
}

// MayCut says the bard may cut a check at all: it is the setting "todos" that takes part in checks (the default asks about attacks
// only), and the bard must be awake. It holds at the answer as it holds at the question, so a bard at 0 hit points or whose setting is
// not "todos" cuts nothing; only the pass is left.
func (b Bard) MayCut() bool {
	return b.Feature && b.Ask == "all" && b.HitPoints > 0
}

// Asks says the bard is asked about a check: the bard may cut it and has a use of Bardic Inspiration left. Outside a combat no
// reaction is spent, so none is asked about.
func (b Bard) Asks() bool {
	return b.MayCut() && b.UsesLeft > 0
}

// Die is the size of the bard's die at the bard level (the Bardic Inspiration die).
func Die(bardLevel int) int { return reaction.CuttingWordsDie(bardLevel) }

// Sight is what decides whether the bard sees the creature and the creature hears the bard (Cutting Words needs both), and whether the
// bard's question may name it.
type Sight struct {
	// Mapped says the creature and the bard both have a token on the grid of the session's current map: the map decides, and Box is not read.
	Mapped bool
	// DistanceFt is the distance between the two tokens, in feet.
	DistanceFt int
	// Seen says the bard's player sees the creature's square (the fog of the map; true on a map with no fog).
	Seen bool
	// TokenHidden says the master hid the creature's token on the map: no player sees it, whatever the box says and whether or not the
	// bard has a token too.
	TokenHidden bool
	// Box is the master's "Os bardos veem a criatura e ela os ouve" for a table with no placement of both.
	Box bool
	// InScene says the bard's player is one of the viewers of the scene the table is in (the whole table when no scene is open or it is
	// open to everyone). Both paths read it: a player the scene is not open to sees nothing of the creature, whatever the map measures
	// and the box says.
	InScene bool
	// OnStage says the creature is on the stage of the open scene for the bard's player (an NPC the players see).
	OnStage bool
}

// Sees says the bard sees the creature within the 60 feet of the reactions and the creature hears it. A creature whose token the master
// hid is seen by nobody, first of all, and so is every creature by a bard whose player is outside the viewers of the scene. Outside a
// combat nothing says a creature is deafened, so it is the master's box that answers when the map does not measure it.
func (s Sight) Sees() bool {
	switch {
	case s.TokenHidden, !s.InScene:
		return false
	case s.Mapped:
		return s.DistanceFt <= reaction.RangeFt && s.Seen
	}
	return s.Box
}

// Named says the bard's player sees the creature itself, so its question may say who it is: the creature is in the bard's sight on the
// map, or, with no map, it is an NPC the players see on the stage. The master's box alone does not say the player sees it.
func (s Sight) Named() bool {
	if s.Mapped {
		return s.Sees()
	}
	return s.Sees() && s.OnStage
}

// Label is what a bard's question calls the creature.
func Label(name string, named bool) string {
	if named && name != "" {
		return name
	}
	return UnnamedLabel
}

// State is where a bard's question stands.
type State int

// The states of a question: open (or waiting for the bards before it), the die taken off, let go, and closed with no answer.
const (
	Waiting State = iota
	Cut
	Passed
	Closed
)

// The states of a question as the rows and the history spell them.
const (
	StateAsked  = "asked"
	StateCut    = "cut"
	StatePassed = "passed"
	StateClosed = "closed"
)

// ParseState reads the state a row keeps: anything but cut, passed and closed is a question still waiting.
func ParseState(s string) State {
	switch s {
	case StateCut:
		return Cut
	case StatePassed:
		return Passed
	case StateClosed:
		return Closed
	}
	return Waiting
}

// Current is the index of the question that is open now: the first still waiting, in the order the bards are asked, -1 when none is.
func Current(states []State) int {
	return slices.Index(states, Waiting)
}

// Settle is what the check comes to: the total with every die the bards took off. A check is not a damage, so the total may fall below
// zero. cut is the sum of the dice.
func Settle(total int, dice []int) (settled, cut int) {
	for _, d := range dice {
		cut += d
	}
	return total - cut, cut
}

var abilities = []string{"ability:str", "ability:dex", "ability:con", "ability:int", "ability:wis", "ability:cha"}

var skills = []string{
	"skill:acrobatics", "skill:animal-handling", "skill:arcana", "skill:athletics", "skill:deception", "skill:history", "skill:insight",
	"skill:intimidation", "skill:investigation", "skill:medicine", "skill:nature", "skill:perception", "skill:performance",
	"skill:persuasion", "skill:religion", "skill:sleight-of-hand", "skill:stealth", "skill:survival",
}

// ValidCheck says the key is an ability check or a skill check of the SRD ("skill:deception", "ability:cha"); a saving throw is not one.
func ValidCheck(key string) bool {
	return slices.Contains(abilities, key) || slices.Contains(skills, key)
}

// CheckKeys are the checks a creature may be rolled for: the six abilities in the order of the sheet, then the skills in alphabetical order.
func CheckKeys() []string {
	return append(slices.Clone(abilities), skills...)
}

// Token is where a character stands on the session's current map.
type Token struct {
	Sq     grid.Square
	Hidden bool
	// Unmeasured says the map has no grid: the token is on the map (and may be hidden) but stands on no square.
	Unmeasured bool
}

// Place is where the creature stands for the question of a check: the tokens of the session's current map (none with no map or no
// grid), and the creature's id among them.
type Place struct {
	Tokens map[string]Token
	NPC    string
}

// Measures says the map places both the creature and the bard on its grid: the map decides what the bard sees, and the map measures the distance and the fog, and the master's box is not read. The tokens of a map with no grid place nobody.
func (p Place) Measures(bard string) bool {
	npc, okN := p.Tokens[p.NPC]
	b, okB := p.Tokens[bard]
	return okN && okB && !npc.Unmeasured && !b.Unmeasured
}

// SightOf is what a bard's question reads of the creature. Its own token comes first: hidden, nobody sees it (the bard needs no token
// for that, and the map needs no grid). Then the map when it measures both (seen says the bard's player sees the creature's square),
// the master's box otherwise. A bard whose player is outside the viewers of the scene (inScene false) sees nothing on either path.
// onStage says the creature is on the stage the bard's player sees.
func (p Place) SightOf(bard string, box bool, seen func(grid.Square) bool, onStage, inScene bool) Sight {
	npcTok, bardTok := p.Tokens[p.NPC], p.Tokens[bard]
	if p.Measures(bard) {
		return Sight{Mapped: true, DistanceFt: grid.RangeFt(bardTok.Sq, npcTok.Sq), Seen: seen(npcTok.Sq), TokenHidden: npcTok.Hidden, InScene: inScene}
	}
	return Sight{Box: box, OnStage: onStage, InScene: inScene, TokenHidden: npcTok.Hidden}
}

// The reasons no bard is asked about a check, as the event of the check keeps them.
const (
	NobodyNoBard      = "no_bard"
	NobodyCharmImmune = "charm_immune"
	NobodyTokenHidden = "token_hidden"
	NobodySees        = "nobody_sees"
)

// TokenHidden says the master hid the creature's token on the session's map.
func (p Place) TokenHidden() bool { return p.Tokens[p.NPC].Hidden }

// WhyNobody is why no bard is asked about a check, "" when one is: the first that applies. bards is how many bards of the party could cut a check
// at all (see Bard.Asks), asked how many of them are asked once the creature and what they see of it are read.
func WhyNobody(bards, asked int, charmImmune, tokenHidden bool) string {
	switch {
	case asked > 0:
		return ""
	case bards == 0:
		return NobodyNoBard
	case charmImmune:
		return NobodyCharmImmune
	case tokenHidden:
		return NobodyTokenHidden
	}
	return NobodySees
}

// Move is what a check that waits needs as things stand when it is read or answered.
type Move int

// The moves: leave it, close every question as a skip (the creature is hidden), let the question at the index pass (its bard is not
// in play), or settle it as the last answer does (no question is left open).
const (
	Stand Move = iota
	Skip
	Pass
	Finish
)

// NextMove says what a check that waits needs now. states are the questions in the order they are asked, inPlay says for each whether its
// bard is a character in play of a member, and tokenHidden whether the creature's token is hidden on the map the check was rolled on. The index is
// the question to let pass, -1 for any other move. A hidden creature closes the check whatever the bards are.
func NextMove(states []State, inPlay []bool, tokenHidden bool) (Move, int) {
	cur := Current(states)
	switch {
	case tokenHidden:
		return Skip, -1
	case cur < 0:
		return Finish, -1
	case cur < len(inPlay) && !inPlay[cur]:
		return Pass, cur
	}
	return Stand, -1
}
