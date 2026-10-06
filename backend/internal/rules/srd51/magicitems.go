package srd51

// MagicItem is a magic item of the SRD 5.1 (5e-SRD-Magic-Items.json), for the
// treasure the master hands out (MR-044). Its key is "item:<index>", such as
// "item:bag-of-holding". Text is the SRD's English, as for the spells.
//
// Families: the SRD lists "Armor, +1, +2, or +3" once as a family, and then
// again as three variants. A family has Variants (the keys of its variants);
// each variant has VariantOf (the key of its family) and its own Rarity.
// Variants are entries of their own, so a list can show either.
type MagicItem struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	// Category is "armor", "weapon", "ammunition", "potion", "scroll", "ring",
	// "rod", "staff", "wand" or "wondrous-item".
	Category string `json:"category"`
	// Rarity is "common", "uncommon", "rare", "very_rare", "legendary",
	// "artifact", or "varies": a family whose variants have different
	// rarities, such as the Potion of Healing. A variant always has a rarity
	// of its own.
	Rarity string `json:"rarity"`
	// Attunement says the item requires attunement; AttunementBy is the SRD's
	// restriction ("by a spellcaster"), or "".
	Attunement   bool   `json:"attunement,omitempty"`
	AttunementBy string `json:"attunement_by,omitempty"`
	// Standalone marks a family that is also an item of its own, at the
	// family's Rarity (the plain Crystal Ball, "a very rare item", whose
	// variants are legendary).
	Standalone bool `json:"standalone,omitempty"`
	// Variants lists the keys of a family's variants, in the SRD's order.
	Variants []string `json:"variants,omitempty"`
	// VariantOf is the key of the family a variant belongs to.
	VariantOf string `json:"variant_of,omitempty"`
	// Desc is the SRD's text, one paragraph per line; the first line is the
	// SRD's type-and-rarity line ("Wondrous item, uncommon (requires
	// attunement)").
	Desc []string `json:"desc"`
}
