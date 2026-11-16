package rules

import (
	"maps"
	"slices"
	"strings"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// A choice of features that offers the options of another feature (from_feature): the options are the named feature's
// own list (the SRD's or a table entry's), shared as one pool with every choice that offers the same options. The pool is
// worked out for the sheet that reads it: an option the table retired or switched off with its entry is no choice for a
// sheet that does not hold that entry, and a pick that names one reads as a key the content does not have.

// checkFromFeature checks a choice that offers the options of a feature and gives the compiled choice the list: the
// feature is a class or subclass feature, the key names a feature of the content that has options of its own, and the
// choice asks for no more than it lists.
func (b *overlayBuilder) checkFromFeature(site effectSite, e *Effect) error {
	switch {
	case b.optionEntry[site.owner] != "":
		return site.fail(".type", ReasonEffect, "an option does not ask for another choice")
	case e.Choice != "feature":
		return site.fail(".from_feature", ReasonValue, "only a choice of features offers the options of a feature")
	case !strings.HasPrefix(site.owner, featureKeyPrefix):
		return site.fail(".from_feature", ReasonValue, "only a class or subclass feature offers the options of another feature")
	case len(e.From) > 0 || e.Set != "":
		return site.fail(".from_feature", ReasonValue, "a choice offers a list, a set, a filter or the options of a feature, not several")
	}
	src, err := b.fromFeatureSource(site, e.FromFeature)
	if err != nil {
		return err
	}
	if e.Count > len(src.Options) {
		return site.fail(".count", ReasonValue, "a choice cannot ask for more than the %d options of %q", len(src.Options), e.FromFeature)
	}
	e.From = slices.Clone(src.Options)
	return nil
}

// fromFeatureSource is the feature a choice offers the options of, when it is one that may be offered.
func (b *overlayBuilder) fromFeatureSource(site effectSite, key string) (*srd51.Feature, error) {
	f := b.n.features[key]
	switch {
	case f == nil || !strings.HasPrefix(key, featureKeyPrefix):
		return nil, site.fail(".from_feature", ReasonReference, "%q is not a feature of the content", key)
	case f.Parent != "":
		return nil, site.fail(".from_feature", ReasonReference, "%q is an option, not a feature with options", key)
	case key == site.owner:
		return nil, site.fail(".from_feature", ReasonValue, "a feature offers its own options with no list: leave from_feature empty")
	case key == invocationsFeature:
		return nil, site.fail(".from_feature", ReasonValue, "the eldritch invocations are dealt by the class table: they are no list to offer")
	case b.n.fromFeature[key] != "":
		return nil, site.fail(".from_feature", ReasonReference, "%q offers another feature's options, which is no list to offer", key)
	case len(f.Options) == 0:
		return nil, site.fail(".from_feature", ReasonReference, "%q has no options", key)
	}
	return f, nil
}

// offerFromFeatures makes each feature whose choice offers the options of another feature the offerer of them, once
// every entry exists: the feature's options are the named feature's, and the content remembers which feature each of
// those options belongs to. A choice that names something else is left alone: compiling it says what is wrong.
func (b *overlayBuilder) offerFromFeatures() {
	n := b.n
	asks := map[string]Effect{}
	for _, p := range b.pending {
		for i := range p.effects {
			if e := p.effects[i]; isFeatureChoice(&e) && e.FromFeature != "" {
				asks[p.owner] = e
				n.fromFeature[p.owner] = e.FromFeature
			}
		}
	}
	for _, owner := range slices.Sorted(maps.Keys(asks)) {
		e := asks[owner]
		f, src := n.features[owner], n.features[e.FromFeature]
		if f == nil || src == nil || len(src.Options) == 0 || src.Parent != "" || n.fromFeature[e.FromFeature] != "" || len(f.Options) > 0 {
			continue
		}
		f.Options, f.OptionsChoose = slices.Clone(src.Options), e.Count
		for _, o := range src.Options {
			n.offeredBy[o] = append(n.offeredBy[o], owner)
			n.optionSource[o] = e.FromFeature
		}
	}
}

// optionHidden says the table keeps the option out of a sheet's pools: it is one of the options another feature offers,
// and it is retired or switched off itself, or the feature it belongs to is hidden and the sheet holds none of that
// feature's entries.
func (c *content) optionHidden(b Build, option string) bool {
	source := c.optionSource[option]
	if source == "" {
		return false
	}
	if c.archived[option] || c.off[option] {
		return true
	}
	return c.hidden(source) && !slices.ContainsFunc(c.entryOwners(source, 0), func(owner string) bool { return c.sheetHolds(b, owner) })
}

// visibleOptions are the options a sheet's pool offers: the list without what the table keeps out of it.
func (c *content) visibleOptions(b Build, options []string) []string {
	if len(c.optionSource) == 0 {
		return options
	}
	return slices.DeleteFunc(slices.Clone(options), func(o string) bool { return c.optionHidden(b, o) })
}
