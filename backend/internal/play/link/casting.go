package link

// The reads casting outside a combat needs from the characters module (SRD 5.1,
// "Spellcasting"). Like the rest of this package they are plain types: the module
// that fills them and the one that reads them never import each other.

// CastSlot is a free spell slot a spell can be cast with.
type CastSlot struct {
	// Level is the slot's level, Pact says it is a pact magic slot, and Free how many
	// slots of this kind are free.
	Level int
	Pact  bool
	Free  int
}

// HealFeatures are the features of the healer that change a healing spell (the Life
// Domain: Disciple of Life, Blessed Healer and Supreme Healing).
type HealFeatures struct {
	Disciple, Blessed, Supreme bool
}

// Effects a spell can have that the server applies, as OutsideSpell.Effect says.
const (
	// EffectNarrated: the server applies nothing; the log records the cast.
	EffectNarrated = "narrated"
	// EffectHeal: the spell restores hit points (dice, or a fixed amount for Heal).
	EffectHeal = "heal"
	// EffectTempHP: the spell gives temporary hit points (False Life).
	EffectTempHP = "temp_hp"
	// EffectMaxHP: the spell raises the hit point maximum (Aid).
	EffectMaxHP = "max_hp"
	// EffectArmorClass: the spell sets the base armor class (Mage Armor).
	EffectArmorClass = "armor_class"
	// EffectSummon: the spell brings creatures.
	EffectSummon = "summon"
)

// OutsideSpell is a spell as a character casts it outside a combat: what the sheet
// says about it, the ways to cast it, how long it takes and lasts, and what the
// server applies. Spell is the same resolution a combat uses (CombatRoster.CombatSpell),
// at the slot level asked for.
type OutsideSpell struct {
	Spell Spell
	// Known, Prepared and CanRitual are what the sheet says about the spell; Ritual
	// is whether it carries the ritual tag.
	Known, Prepared, CanRitual, Ritual bool
	// CanCast says the spell can be cast the normal way: a cantrip, or a prepared
	// spell with a free slot, listed in Slots.
	CanCast bool
	Slots   []CastSlot
	// CastMinutes is how long casting takes (0: an action, a bonus action or a reaction)
	// and RitualMinutes how long the ritual version takes.
	CastMinutes, RitualMinutes int
	// Lasts says the spell stays after the cast; DurationSeconds is its timed duration
	// (0 when it has none to count) and RestEnds the rest that ends it by its length
	// ("short", "long" or "").
	Lasts           bool
	DurationSeconds int
	RestEnds        string
	// Effect is one of the Effect* constants.
	Effect string
	// Healing are the healer's features.
	Healing HealFeatures
	// NPC says the caster is an NPC: it has no slots to spend.
	NPC bool
}

// MageArmor is what Mage Armor would do for a character: Wears says it wears armor (the
// spell asks for a creature that does not), AC the armor class it would have with the
// spell on it. Applies is false for a character with no sheet to work it out from (an
// NPC with a basic sheet).
type MageArmor struct {
	Applies bool
	Wears   bool
	AC      int
}
