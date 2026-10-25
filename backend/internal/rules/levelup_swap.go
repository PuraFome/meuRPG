package rules

import (
	"slices"
)

// The groups of picks an Ability Score Improvement level may swap, when the table plays
// with the rule "Trocar uma escolha no aumento de atributo". Each names one of the
// choice groups the guided level-up knows: the cantrips of the class, or one of the
// option sets (the SRD's and the table's options of the set make the pool).
const (
	SwapGroupCantrip      = "cantrip"
	SwapGroupFightStyle   = "fighting_style"
	SwapGroupMetamagic    = "metamagic"
	SwapGroupInvocation   = "invocation"
	SwapGroupPactBoon     = "pact_boon"
	LevelUpReasonSwapPick = "swap_pick"
)

var swapGroupNames = map[string]string{
	SwapGroupCantrip:    "Truque",
	SwapGroupFightStyle: "Estilo de luta",
	SwapGroupMetamagic:  "Metamagia",
	SwapGroupInvocation: "Invocação mística",
	SwapGroupPactBoon:   "Dádiva do pacto",
}

// swapGroupOrder is the order the groups are offered in.
var swapGroupOrder = []string{SwapGroupCantrip, SwapGroupFightStyle, SwapGroupMetamagic, SwapGroupInvocation, SwapGroupPactBoon}

// SwapGroup is one group a level may swap one pick in: the picks the character has (what
// it may give up) and the pool it may take from, without the picks it already has.
type SwapGroup struct {
	Group  string
	NamePT string
	Picks  []NamedKey
	Pool   []NamedKey
}

// PickSwap is the swap the player asks for: From, a pick of the group, becomes To.
type PickSwap struct {
	Group, From, To string
}

// IsZero says nothing is swapped.
func (s PickSwap) IsZero() bool { return s == PickSwap{} }

// setOptions are the options of an option set that the features offering the picks in
// have: the SRD's and the table's.
func (c *content) setOptions(set string, picks []string) []string {
	def, ok := optionSetOf(set)
	if !ok {
		return nil
	}
	var out []string
	for _, f := range def.Offerers {
		feat := c.features[f]
		if feat == nil || !slices.ContainsFunc(feat.Options, func(k string) bool { return slices.Contains(picks, k) }) {
			continue
		}
		for _, k := range feat.Options {
			if !slices.Contains(out, k) {
				out = append(out, k)
			}
		}
	}
	return out
}

// swapGroups lists the groups of the sheet before a level up with something to swap.
func (c *content) swapGroups(b Build, classKey string) []SwapGroup {
	var out []SwapGroup
	for _, name := range swapGroupOrder {
		g := SwapGroup{Group: name, NamePT: swapGroupNames[name]}
		if name == SwapGroupCantrip {
			g.Picks = c.keysNamed(b.Cantrips, nil)
			for _, k := range c.sortedSpellKeys() {
				s := c.spells[k]
				if s.Level == 0 && c.onList(s, classKey) && !slices.Contains(b.Cantrips, k) {
					g.Pool = append(g.Pool, NamedKey{Key: k, NamePT: c.namePT(k)})
				}
			}
		} else {
			// The pool is what the features that offered the picks offer: a fighter's styles,
			// not the paladin's.
			opts := c.setOptions(name, b.FeatureChoices)
			g.Picks = c.keysNamed(b.FeatureChoices, opts)
			if len(g.Picks) > 0 {
				g.Pool = c.keysNamed(opts, nil)
				g.Pool = slices.DeleteFunc(g.Pool, func(n NamedKey) bool { return slices.Contains(b.FeatureChoices, n.Key) })
			}
		}
		if len(g.Picks) > 0 && len(g.Pool) > 0 {
			out = append(out, g)
		}
	}
	return out
}

// keysNamed names the keys, only the ones in among when it is not nil.
func (c *content) keysNamed(keys, among []string) []NamedKey {
	var out []NamedKey
	for _, k := range keys {
		if among != nil && !slices.Contains(among, k) {
			continue
		}
		out = append(out, NamedKey{Key: k, NamePT: c.namePT(k)})
	}
	return out
}

func (c *content) sortedSpellKeys() []string { return sortedKeys(c.spells) }

// applySwap gives up the pick and takes the other, in the lists the group lives in.
func applySwap(after *Build, s PickSwap) {
	if s.IsZero() {
		return
	}
	list := &after.FeatureChoices
	if s.Group == SwapGroupCantrip {
		list = &after.Cantrips
	}
	*list = slices.DeleteFunc(*list, func(k string) bool { return k == s.From })
	*list = append(*list, s.To)
}

// swapOf finds the swap the new sheet shows: a group of the offer in which the sheet gave
// up one pick (To is the pool option the sheet gained, "" when it gained none). found is
// false when no pick of an offered group was given up.
func swapOf(before, after Build, offer LevelUpOffer) (swap PickSwap, found bool) {
	for _, g := range offer.PickSwaps {
		was, now := before.FeatureChoices, after.FeatureChoices
		if g.Group == SwapGroupCantrip {
			was, now = before.Cantrips, after.Cantrips
		}
		var gone string
		for _, n := range g.Picks {
			if !slices.Contains(now, n.Key) {
				gone = n.Key
			}
		}
		if gone == "" {
			continue
		}
		swap = PickSwap{Group: g.Group, From: gone}
		for _, n := range g.Pool {
			if slices.Contains(now, n.Key) && !slices.Contains(was, n.Key) {
				swap.To = n.Key
			}
		}
		if swap.To == "" && g.Group != SwapGroupCantrip {
			continue // a pick lost with nothing in its place is not a swap
		}
		return swap, true
	}
	return PickSwap{}, false
}
