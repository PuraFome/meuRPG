// Package link holds the small plain types that cross the borders between
// the play module and the modules it reads while it runs a combat: the
// characters module (who can fight, with which numbers) and the maps module
// (the grid, the battle point, the tokens).
//
// Modules never import each other's code (docs/architecture.md), and the
// numbers a combat needs are not API messages, so the types both sides
// agree on live here, in a package that imports nothing of the project.
// Package play declares the interfaces (CombatRoster, MapKeeper) that use
// them; cmd/api connects the real services.
package link

import (
	"errors"
	"time"
)

// What a Wild Shape change refuses with (MR-037): the characters module returns
// them, the play module turns them into the typed refusal the app reads.
var (
	// ErrBeastNotAllowed: the beast is not one the character's Wild Shape allows
	// (its level, the fly and swim limits, not a beast), or it has no Wild Shape.
	ErrBeastNotAllowed = errors.New("the beast is not one the character's Wild Shape allows")
	// ErrAlreadyInWildShape: the character is in a beast form already.
	ErrAlreadyInWildShape = errors.New("the character is in a beast form already")
)

// Character is what a combat needs to know about a character that fights:
// who it is and the numbers that start a combatant (MR-013). Numbers come
// from the sheet, derived by the rules module for a full sheet, and read
// as they are for a basic one.
type Character struct {
	ID   string
	Name string
	// Player is true for a player's character, false for an NPC.
	Player bool
	// PlayerUserID is the account that plays a player's character. Empty for
	// an NPC, and for a character whose player deleted their account (RN-16).
	PlayerUserID string
	// Reserved is true for a character the master gave back to the reserve (MR-049):
	// nobody owns it and no player may see it. Only SessionCharacters sets it; the
	// other reads leave a reserved character out.
	Reserved bool
	// InitiativeBonus is added to the d20 for the initiative.
	InitiativeBonus int
	// SpeedFt is the walking speed, in feet (5 ft per square of the grid).
	SpeedFt int
	// HitPointsMax is the maximum hit points. A combatant takes it only for
	// an NPC: a player character's hit points are in its vitals.
	HitPointsMax int
	// XPValue is the XP an NPC gives when defeated (its sheet's xp_value), 0
	// for a player's character (MR-016).
	XPValue int
	// PortraitImageID is the gallery image of an NPC's portrait (MR-031), ""
	// for none and for a player's character.
	PortraitImageID string
	// Size is the creature's size, one of "tiny", "small", "medium", "large",
	// "huge" or "gargantuan": a character's race size, or a basic sheet's Tamanho.
	// "" means medium (MR-034, RN-21).
	Size string
	// SpeedFlyFt is the fly speed in feet, 0 for none.
	SpeedFlyFt int
	// JumpLongDFt and JumpHighDFt are the long and high jump with a running start,
	// in tenths of a foot (rules/combat.JumpLimits); 0 for a sheet with no
	// Strength. Standing, each is half.
	JumpLongDFt, JumpHighDFt int
	// CombatOnly marks the NPC the app keeps for a creature's monsters (RN-29):
	// never a participant, a stage NPC or a token.
	CombatOnly bool
	// CastsInBeastForm says the character keeps its spells in a beast shape
	// (a druid of level 18, Beast Spells); every other character has none there.
	CastsInBeastForm bool
	// MonsterKey and ChallengeRating are the SRD creature an NPC was made from
	// ("monster:bandit") and its ND ("1/8"), "" for an NPC the master typed: the
	// master's view of a monster.
	MonsterKey, ChallengeRating string
}

// PartyMember is a player's character as an encounter's budget reads it (MR-043):
// its name and its total level.
type PartyMember struct {
	ID, Name string
	Level    int
}

// Grid is a map's battle grid: squares of 1.5 m (5 ft) across the image's
// width and down its height. The zero Grid means the map has none.
type Grid struct {
	Columns int32
	Rows    int32
}

// OK says whether the map has a grid.
func (g Grid) OK() bool { return g.Columns > 0 && g.Rows > 0 }

// BattlePoint is a battle point of a map: where its fight happens.
type BattlePoint struct {
	// MapID is the map the point is on.
	MapID string
	// TargetMapID is the map of the fight, when the master chose one; empty
	// when the fight happens on the session's current map.
	TargetMapID string
	// XBP and YBP are where the point is on its map, in basis points of the
	// image's width and height (0 to 10000).
	XBP, YBP int32
}

// TokenPosition is where a character's token stands on a map, in basis
// points of the image's width and height (0 to 10000).
type TokenPosition struct {
	// CharacterID is the character whose token it is; empty for a creature's.
	CharacterID string
	// CreatureID is the creature whose token it is (MR-037); empty for a
	// character's own token.
	CreatureID string
	XBP, YBP   int32
}

// Sheet is what the actions of a combat need from a character's sheet: the
// armor class an attack must reach, the attacks it can make and the standard
// actions it has (MR-012, MR-014). The characters module fills it from the
// derived full sheet, or from the structured attacks of a basic one.
type Sheet struct {
	// ArmorClass is the number a d20 plus the attack bonus must reach. It
	// never goes to a player (RN-20).
	ArmorClass int
	// Attacks are the weapon attacks and the attack cantrips, in sheet order.
	Attacks []Attack
	// Actions are the standard actions every creature has.
	Actions []Action
	// AttacksPerAction is how many attacks the Attack action makes: 1, or more
	// with Extra Attack.
	AttacksPerAction int
	// CriticalRange is the lowest natural d20 that is a critical hit with a
	// weapon attack: 20, 19 with Improved Critical, 18 with Superior Critical.
	// 0 (a basic sheet) is 20.
	CriticalRange int
	// BrutalCriticalDice is how many weapon damage dice a critical hit with a melee
	// weapon attack rolls on top of the doubled ones (the barbarian's Brutal
	// Critical: 1, 2 or 3 by level); 0 without it.
	BrutalCriticalDice int
	// Metamagic are the Metamagic options the sorcerer knows (feature keys), ChaMod
	// is its Charisma modifier, and BardicDie the size of the die its Bardic
	// Inspiration gives, 0 for a character with none.
	Metamagic []string
	ChaMod    int
	BardicDie int
	// TwoWeaponFighting says the character has the fighting style.
	TwoWeaponFighting bool
	// FeatureActions are the actions the sheet's class and race features grant
	// (rules.Derived.Actions): their key, name, economy and resource.
	FeatureActions []FeatureAction
	// FighterLevel is what Retomar o fôlego adds to its d10: the character's
	// fighter level, 0 for anyone who is not a fighter.
	FighterLevel int
	// Senses are the special senses the sheet or stat block gives (darkvision...),
	// which an NPC sees with on a map with the fog of war (MR-036). The zero value
	// is plain sight.
	Senses Senses
	// Traits are the features the rolls and the damage of a combat read.
	Traits Traits
}

// Traits are what a sheet has that changes how its owner rolls and what its
// damage carries: the barbarian's Rage, Reckless Attack and Danger Sense, the
// rogue's Sneak Attack, the paladin's Divine Smite, the ranger's Hunter's Mark and
// Colossus Slayer, the fighting style that rerolls low damage dice, a monster's
// Pack Tactics, the armor that cancels Rage and the damage resistances.
type Traits struct {
	// BarbarianLevel and RogueLevel are the class levels, 0 without the class.
	BarbarianLevel, RogueLevel int
	// Rage, RecklessAttack and DangerSense are the barbarian's features, and Frenzy
	// the Berserker's level 3 feature.
	Rage, RecklessAttack, DangerSense, Frenzy bool
	// SneakAttackDice is the dice of Sneak Attack, 0 without the feature.
	SneakAttackDice int
	// DivineSmite, ImprovedDivineSmite and ColossusSlayer are the features of those
	// names, and HuntersMark says the character has the spell.
	DivineSmite, ImprovedDivineSmite, ColossusSlayer, HuntersMark bool
	// GreatWeaponFighting is the fighting style.
	GreatWeaponFighting bool
	// PackTactics is a monster's trait.
	PackTactics bool
	// HeavyArmor says the armor worn is heavy: Rage gives none of its benefits then.
	HeavyArmor bool
	// StealthDisadvantage says the armor worn gives disadvantage on Dexterity (Stealth).
	StealthDisadvantage bool
	// Resistances are the damage resistances the features and traits give.
	Resistances []Resistance
	// CreatureType is the SRD type of a monster ("undead", "fiend", "beast"); empty
	// for a character.
	CreatureType string
}

// Resistance is a damage resistance of a feature or trait: its key and name, the
// damage types it halves ("damage-type:fire") and when ("" always, "rage").
type Resistance struct {
	Source, NamePT string
	DamageTypes    []string
	While          string
}

// Senses are a creature's special senses, as a range in feet (0 for none).
type Senses struct {
	DarkvisionFt, BlindsightFt, TruesightFt int
}

// Attack is one attack of a sheet, with real dice.
type Attack struct {
	// Key identifies it in the sheet: a weapon or spell key, "basic:0" for the
	// first attack of a basic sheet.
	Key string
	// Name is its Portuguese name.
	Name string
	// Save says it asks for a saving throw instead of an attack roll, which
	// the spells slice handles.
	Save bool
	// Spell says it is a cantrip, cast with the whole action: Extra Attack
	// belongs to weapon attacks only.
	Spell bool
	// ToHit is added to the d20.
	ToHit int
	// DiceCount d DiceSides plus DiceBonus is the damage. DiceCount 0 is a flat
	// number.
	DiceCount, DiceSides, DiceBonus int
	// DamageType is a content key such as "damage-type:slashing".
	DamageType string
	// RangeFt is the reach or normal range, LongRangeFt the long range, both
	// in feet; 0 when the sheet says none (a melee attack reaches 5 ft).
	RangeFt, LongRangeFt int
	// Melee says it is a melee weapon, thrown or not: the only kind an
	// opportunity attack can use, with the melee reach.
	Melee bool
	// Beams is how many attack rolls the cantrip makes in one action (Eldritch
	// Blast: 2, 3 and 4 beams from character levels 5, 11 and 17); 0 or 1 for
	// anything else.
	Beams int
	// Light is a light melee weapon. Unarmed is the unarmed strike, and
	// MartialArts says the Martial Arts strike goes with the attack. AbilityMod
	// is the ability modifier inside DiceBonus. The bonus action attacks read
	// them.
	Light, Unarmed, MartialArts bool
	AbilityMod                  int
	// Ability is the ability the attack uses ("str", "dex"...), Finesse and TwoHanded
	// the weapon's properties, and Weapon says it is a weapon attack (an unarmed
	// strike too) and not a spell.
	Ability            string
	Finesse, TwoHanded bool
	Weapon             bool
}

// Action is a standard action: its key ("standard:dash") and Portuguese name.
type Action struct {
	Key, Name string
}

// FeatureAction is an action a feature grants ("feature:second-wind").
type FeatureAction struct {
	Key, Name string
	// Economy is "action", "bonus_action", "reaction", "free" or "movement".
	Economy string
	// Resource is the key of the resource each use spends, or "".
	Resource string
	// Pool says the resource is a pool of points (Cura pelas mãos), not a count
	// of uses: using the action spends no point.
	Pool bool
	// Standard is the standard action it performs ("standard:dash" for Cunning
	// Action's Dash), or "".
	Standard string
}

// Dice is a roll: Count d Sides plus Bonus (Count 0 is a flat number), with the
// damage type for a damage.
type Dice struct {
	Count, Sides, Bonus int
	// DamageType is a key such as "damage-type:fire"; empty for a heal.
	DamageType string
}

// Spell is what a cast needs from a spell and the caster's sheet, resolved at
// the slot level the spell is cast with (MR-014).
type Spell struct {
	Key, Name string
	// Level is the spell's own level, 0 for a cantrip.
	Level int
	// Economy is "action", "bonus_action" or "reaction"; "" for a casting time
	// too long for a fight.
	Economy       string
	Concentration bool
	// Verbal says the spell has a verbal component: the noise gives a hidden caster's
	// position away.
	Verbal bool
	// RangeKind is "self", "touch", "ranged", "sight", "unlimited" or "special",
	// and RangeFt the distance of a ranged spell.
	RangeKind string
	RangeFt   int
	// AttackType is "melee", "ranged" or "": a spell attack, with ToHit added to
	// the d20.
	AttackType string
	ToHit      int
	// SaveAbility is the ability of the saving throw ("dex"), "" when there is
	// none; SaveOnSuccess is "half", "none" or "other"; SaveDC the caster's DC.
	SaveAbility   string
	SaveOnSuccess string
	SaveDC        int
	// CasterDC is the caster's spell save DC, whether or not the spell asks for a saving
	// throw: the DC of the saves an effect that lasts asks later (Web, Hold Person).
	CasterDC int
	// Damages are the spell's damage at the slot level, one part for each damage
	// type it deals (Ice Storm has two), empty when it has none the engine can
	// roll; Heal is its healing with the caster's spellcasting modifier already in
	// Bonus, nil when it does not heal.
	Damages []Dice
	Heal    *Dice
	// DamageChoice says what the caster picks among the damage types: "scale"
	// (every part is dealt, the higher-slot dice go to the picked type),
	// "alternative" (only the picked type is dealt) or "" (nothing to pick).
	// DamageTypes are the types the caster may pick, and Damages already follow
	// the pick the spell was resolved with.
	DamageChoice string
	DamageTypes  []string
	// Area says the spell hits every creature in an area: any number of targets.
	// ExtraTargetPerLevel says it takes one more target for each slot level
	// above its own.
	Area                bool
	ExtraTargetPerLevel bool
	// TargetCount is how many targets the spell takes at its own level (0 when the
	// spell says only Area or nothing: the old rules apply), and TargetPerLevel
	// how many more it takes for each slot level above its own. A table spell
	// says both itself ("três criaturas, uma a mais por nível"); an SRD spell's
	// come from the same fields of rules.SpellTarget.
	TargetCount    int
	TargetPerLevel int
	// CasterOnly says there is nobody to pick: the spell reaches the caster alone,
	// or no creature at all (a point, an object, a place), whatever its range.
	CasterOnly bool
	// HP is set for a spell that reads hit points (Sono, Palavra de Poder...):
	// Damage and Heal are nil for it, and the cast applies HP instead.
	HP *HPEffect
	// Summon says the spell brings creatures (Convocar Familiar, Animar os
	// Mortos, Conjurar Animais; MR-037): the cast takes a choice of creatures
	// instead of targets.
	Summon bool
	// IgnoresCover says the spell's saving throw gets no benefit from cover (Chama
	// Sagrada, SRD 5.1); the total-cover targeting refusal stays.
	IgnoresCover bool
	// AreaShape is the form of the spell's area ("sphere", "cylinder", "cone", "line"
	// or "cube"), AreaSizeFt its radius (sphere, cylinder), length (cone, line) or
	// side (cube) in feet, and AreaWidthFt a line's width: empty and 0 for a spell
	// that is not an area with a shape. SpreadsAroundCorners says the area is the
	// part of the shape connected to its origin, not what the origin sees.
	AreaShape            string
	AreaSizeFt           int
	AreaWidthFt          int
	SpreadsAroundCorners bool
}

// HPEffect is what a spell that reads hit points does at the slot level, from
// the rules' effects/spells.json (rules.SpellEffect). Kind is the rules'
// SpellKind: "hp_pool", "hp_threshold", "zero_hp_target", "flat_heal", "temp_hp"
// or "max_hp".
type HPEffect struct {
	Kind string
	// Pool is the dice of an hp_pool spell, with the extra dice of the slot
	// level already in Count.
	Pool Dice
	// Condition is the key given to each creature affected; empty for none.
	Condition string
	// Threshold is the most hit points a creature may have, and Dies says the
	// spell kills it instead (an hp_threshold spell).
	Threshold int
	Dies      bool
	// Heal is a flat_heal's healing at the slot level, and Ends the conditions
	// it ends.
	Heal int
	Ends []string
	// Amount is what a temp_hp spell adds to its Pool dice (the spell's dice are
	// in Pool, with no bonus) and what a max_hp spell raises the maximum by, at
	// the slot level.
	Amount int
}

// Save is a creature's saving throw: the bonus added to the d20, and whether
// the sheet has one at all. A basic-sheet NPC has none: Known is false and the
// Bonus 0.
type Save struct {
	Bonus int
	Known bool
}

// ReactionStats is what a combatant's sheet or stat block says about its
// reactions (PM-04): the spells it can cast and its slots, the numbers of each
// reaction and the features that give them. A player's character spends from its
// vitals; for anyone else the slots are the sheet's or the stat block's total and
// the combat counts what a fight spent.
type ReactionStats struct {
	// Spells are the keys of the spells it can cast now: every cantrip, the spells
	// it knows or has prepared, the always-prepared ones and, for a stat block, the
	// ones its Spellcasting trait lists.
	Spells []string
	// SlotsTotal are its slots by spell level (index 0 the 1st), PactLevel and
	// PactSlots the warlock's pact slots.
	SlotsTotal           [9]int
	PactLevel, PactSlots int
	// CastingMod is the spellcasting ability modifier (what a Counterspell check
	// adds), SaveDC the spell save DC.
	CastingMod, SaveDC int
	// InfernalLegacy says Hellish Rebuke can be cast through the tiefling's trait
	// (level 3), once a long rest, as a 2nd-level spell; its DC is the Charisma one.
	InfernalLegacy bool
	LegacyDC       int
	// UncannyDodge says the combatant has the rogue's feature.
	UncannyDodge bool
	// Deflect says it has Deflect Missiles; MonkLevel and DexMod are its numbers.
	Deflect           bool
	MonkLevel, DexMod int
	// Proficiency is the proficiency bonus (the Deflect Missiles throw back adds it).
	Proficiency int
	// CuttingWords says it has the bard's feature, with its bard level (the die)
	// and the bard's "Perguntar" setting: "all", "attacks" (the default) or "never".
	CuttingWords bool
	BardLevel    int
	CuttingAsk   string
	// CharmImmune says the stat block is immune to being charmed (Cutting Words
	// does nothing to it).
	CharmImmune bool
	// StatBlock says the combatant is a creature's stat block: its slots come from
	// the trait and the combat counts them.
	StatBlock bool
	// ResourceMax are the uses its features give at its level, by resource key
	// ("bardic_inspiration", "ki", "infernal_legacy"): the combat counts what an NPC
	// spent; a player's character spends from its vitals.
	ResourceMax map[string]int
}

// Named is a content key with its Portuguese name.
type Named struct {
	Key, NamePT string
}

// Turn is what a combatant used in the current turn, for working out what it
// can still do.
type Turn struct {
	ActionUsed, BonusActionUsed, ReactionUsed bool
	// AttacksMade is how many attacks the Attack action made this turn.
	AttacksMade int
	// AttackKey is the attack the Attack action made last this turn ("" if none)
	// and FlurryLeft the unarmed strikes of Flurry of Blows left: what the bonus
	// action attacks read.
	AttackKey  string
	FlurryLeft int
	// FrenzyReady says the combatant is in a frenzied rage that began in an earlier
	// turn (the Berserker's Frenzy): a melee weapon attack is its bonus action.
	FrenzyReady bool
	// Dashed says the Dash action doubled the speed.
	Dashed bool
	// ActionSurged says Action Surge was used this turn (once per turn, whatever
	// the uses left).
	ActionSurged bool
	// SpellCast and BonusSpellCast say which kinds of spell were cast this turn
	// (see combat.TurnState).
	SpellCast, BonusSpellCast bool
	// SpeedFt is the combatant's best speed in a combat (walking, or flying when
	// it can), and MovementUsedFt the feet walked this turn, rounded down.
	SpeedFt, MovementUsedFt int
	// MovementUsedDFt is the movement used this turn in tenths of a foot, which is
	// what the server charges (RN-21); LastMoveDFt the length of the last move on
	// foot this turn, the running start of a jump; Disengaged says the Disengage
	// action was taken.
	MovementUsedDFt, LastMoveDFt int
	// JumpLongDFt and JumpHighDFt are the combatant's jump limits with a running
	// start, in tenths of a foot, as kept on the combatant (copied from the sheet
	// when it joined, like its speed): the turn options show exactly what the
	// server enforces. Standing, each is half.
	JumpLongDFt, JumpHighDFt int
	Disengaged               bool
}

// Scene is an RP scene (MR-015): a SCENE point of a map as the session opens
// it, with the master's actions on it. The DCs are in it, so it never goes to
// a player as it is.
type Scene struct {
	PointID string
	// Name and Description are the point's: the players read both in the open
	// scene.
	Name, Description string
	// Actions are in the order the master put them.
	Actions []SceneAction
	// Hooks is the master's private "Ganchos e anotações" (MR-029), and Clues
	// the clues the master prepared. They never go to a player (RN-20).
	Hooks string
	Clues []SceneClue
	// Images are the gallery images attached to the scene (MR-015), in the
	// master's order. They never go to a player (RN-10).
	Images []SceneImage
	// ShowDC is the master's "Mostrar a CD aos jogadores" switch: when it is
	// on, a player gets each DC and the pass or fail of their own rolls.
	ShowDC bool
}

// SceneImage is a gallery image attached to a scene.
type SceneImage struct {
	ID, Name string
	// ShowsWholeMap is GalleryImage.shows_whole_map.
	ShowsWholeMap bool
}

// SceneClue is a clue of a scene, with who has it.
type SceneClue struct {
	ID, Text string
	// RevealedTo are the players it was revealed to, oldest first.
	RevealedTo []ClueRecipient
}

// ClueRecipient is a player's character a clue was revealed to.
type ClueRecipient struct {
	CharacterID string
	RevealedAt  time.Time
}

// SceneAction is one check of a scene.
type SceneAction struct {
	ID string
	// Key is "skill:investigation", "ability:str" or "save:wis".
	Key string
	// Name is the master's name for it, "" for none.
	Name string
	// DC is the difficulty class, 0 for none.
	DC int
	// MaxAttempts is how many times each player's character may roll it while
	// the scene is open: 1 to 5, and 0 means unlimited.
	MaxAttempts int
}

// SceneOption is a scene check with one character's numbers on it (the rules
// engine's SceneOption).
type SceneOption struct {
	// Known says the sheet has this check; a basic sheet has no skills, so its
	// character has no numbers for a scene.
	Known bool
	// CheckName is the check's name in Portuguese: "Investigação".
	CheckName string
	// Bonus is added to the d20.
	Bonus int
	// Passive is the character's passive value, when HasPassive.
	Passive    int
	HasPassive bool
	// ReliableTalent says a d20 of this check counts as at least 10 (the
	// character has Reliable Talent and the check adds the proficiency bonus).
	ReliableTalent bool
}

// Creature is a creature of a player's character (MR-037, Etapa 9) as a
// combat reads it: an SRD creature the table named, with its own hit points.
// The stat block is rules content; what a combat needs of it is copied in.
type Creature struct {
	ID string
	// CharacterID is the owner, and OwnerUserID the owner's player ("" when the
	// player left).
	CharacterID, OwnerUserID string
	// MonsterKey is the SRD creature, "monster:wolf", and Name what the table
	// calls it (1 to 40 characters).
	MonsterKey, Name string
	// Source is "familiar", "animate_dead", "conjure_animals" or "master".
	Source string
	// Attack is "none", "reaction" or "full" (rules.SummonAttack*).
	Attack string
	// GroupID is shared by the creatures of one casting.
	GroupID string
	// DependsOnConcentration says it lasts only while the caster concentrates.
	DependsOnConcentration bool
	// HitPointsCurrent and HitPointsMax are its own.
	HitPointsCurrent, HitPointsMax int
	// InitiativeBonus is its Dexterity modifier and SpeedFt its best speed, in
	// feet: what a combatant copies when it joins.
	InitiativeBonus, SpeedFt int
	// SpeedFlyFt is its fly speed, 0 for none; Size its size ("tiny" to
	// "gargantuan"); JumpLongDFt and JumpHighDFt its jump limits with a running
	// start, in tenths of a foot (rules/combat.JumpLimits of its Strength). What a
	// combatant copies when it joins, as a character's.
	SpeedFlyFt  int
	Size        string
	JumpLongDFt int
	JumpHighDFt int
}

// CreatureSpec is one creature a casting makes: the SRD creature, the name the
// player gave it ("" for the default) and what it may do on its own.
type CreatureSpec struct {
	MonsterKey, Name, Attack string
}

// Summon is a casting to record: the creatures a spell made for a character.
type Summon struct {
	CampaignID, CharacterID string
	// Source is "familiar", "animate_dead" or "conjure_animals".
	Source string
	// GroupID is the id the creatures of the casting share.
	GroupID string
	// Concentration says the creatures last only while the caster concentrates.
	Concentration bool
	Creatures     []CreatureSpec
}

// SummonResult is what recording a casting did: the creatures it made, in
// order, and the ones it dismissed (a new familiar replaces the old one).
type SummonResult struct {
	Created, Replaced []Creature
}

// SummonedForm is a creature a casting may make, as the rules check it.
type SummonedForm struct {
	MonsterKey, Attack string
}

// SummonSpell is what a summoning spell asks and what the character has of
// it, from the spell's data and the sheet.
type SummonSpell struct {
	// Known says the spell is on the character's sheet (a wizard's spellbook
	// counts), Prepared that it can be cast with a slot today, and CanRitual
	// that the character may cast it as a ritual: the spell is a ritual and the
	// character's class casts rituals.
	Known, Prepared, CanRitual bool
	// Ritual, Concentration and CastingUnit come from the spell: Find Familiar
	// is a ritual that takes 1 hour, Conjure Animals takes 1 action and needs
	// concentration. CastingUnit is a rules.Cast* constant.
	Ritual, Concentration bool
	CastingUnit           string
	// Level is the spell's own circle.
	Level int
	// Creatures are the creatures the choice made, with what each may do.
	Creatures []SummonedForm
}

// CreatureState is where a combat leaves a creature: its hit points and whether
// it is out of the fight (0 hit points).
type CreatureState struct {
	ID       string
	HP       int
	Defeated bool
}

// CreatureChanges says what keeping creatures in step with a combat did.
type CreatureChanges struct {
	// Dismissed are the creatures that were defeated and are dismissed now;
	// Revived the ones that were dismissed as defeated and an undo healed.
	Dismissed, Revived []Creature
}

// MonsterHitPoints are the hit points of an SRD creature: its average, and the
// dice it rolls them with ("2d8+2" is 2, 8, 2). DiceCount is 0 when the
// creature's dice are not known, and then the average is all there is.
type MonsterHitPoints struct {
	Average                         int
	DiceCount, DiceSides, DiceBonus int
}

// DeadCharacter is a player's character the master marked dead: a target Revivify may
// reach outside a combat, and the master's "Reviver".
type DeadCharacter struct {
	ID   string
	Name string
	// PlayerUserID is the account that plays it, empty when the player left (RN-16).
	PlayerUserID string
	// RevivifyBlocked is the master's switch "Revivificar não funciona nesta morte".
	RevivifyBlocked bool
}
