# Rules module: creatures, items and presets

The SRD 5.1 creatures, the magic items and the trap and light presets enter `rules` as data, like the rest of the content.
How the snapshot files are imported is in [Rules content](../rules-content.md); this page is what the engine does with them.
The services that serve them are in [Characters](characters.md#content-service).
For the engine around them see [Rules module](rules.md).

## Creatures

`srd51.Monster` holds a creature's size, type, armor class and what gives it, hit points and dice, speeds in feet, the six abilities, save and skill bonuses, vulnerabilities, resistances, immunities, condition immunities, senses, challenge rating and XP, traits, actions (with the attack bonus, the damage parts, the save DC and the Multiattack routines), reactions and legendary actions.
The Portuguese names are `monster:<index>` in `effects/names_pt.json`; the sheets' text stays in English, like the spells'.

Three importer readings deserve a warning.
When the SRD gives a damage choice (the longsword one- or two-handed), the first option is kept and the text has the rest.
The XP of a creature is the one of its challenge rating in the SRD table, not the source's, which has four creatures at half; a creature with challenge rating 0 and no XP keeps 0.
A save that 5e-database leaves only in the text ("Força CD 11 ou cai Derrubado") is read from the action's text.

| Function (`rules.Content`) | What it does |
| --- | --- |
| `ListCreatures(filter)`, `CreatureByKey(key)` | The list and one creature's sheet with Portuguese labels. The filter has a name query (Portuguese or English, without accents), type, size, minimum and maximum CR, no flying and no swimming; the list is sorted by Portuguese name. It refuses a CR that is not in the SRD, a minimum above the maximum and a size that does not exist. |
| `MonsterDerived(key)` | The creature as a `Derived` that combat already reads: abilities and modifiers, saves and skills, armor class, hit points, the walking speed in `SpeedWalkFt` and the others (`SpeedFlyFt`, `SpeedSwimFt`, `SpeedClimbFt`, `SpeedBurrowFt`, `Hover`), senses, passive Perception, the attacks (bonus and the first damage part; the full text goes in `Attack.Notes` for the master), Multiattack as `AttacksPerAction` (the largest routine) and the save actions in `SaveActions` with their DC. No spellcasting. |
| `SummonOptions(spell, level, build)`, `CheckSummon(...)` | What a summoning spell lets the caster pick with that slot, and the check of a choice (`ErrSummonCreature`, `ErrSummonCount`, `ErrSummonOption`, `ErrSummonCircle`, `ErrNotSummonSpell`). Casting time, ritual and concentration come from the spell's data. |
| `WildShapeLimitFor(build)`, `WildShapeForms(build)`, `WildShapeAllows(build, beast)`, `WildShapeDerived(sheet, beast)`, `CastsInBeastForm(derived)` | See [Wild Shape](#wild-shape). |

**A creature's armor class** comes from the armor it uses, and the note says what it wears and the other values ("15 com Armadura Arcana").

**Multiattack corrections.**
The Multiattack count (`MonsterDerived(...).AttacksPerAction`, the same number the character's creatures use on their turn) applies to any basic sheet linked to a creature, so a monster's turn options count the attacks of the Attack action, and NPCs made by "Criar NPC" follow it.
The master plays the monsters and has the last word, so no extra attack is refused to them.
The number comes from SRD 5.1 and not from the snapshot, which gets some wrong: `effects/corrections.json` corrects it (`creature_corrections`, a closed type: a known creature and field).
The same file corrects other stat-block fields, a spell's attack, saving throw and damage by slot level, and the spells a subclass always has, all applied over the snapshot at load (see [Rules content](../rules-content.md)).
`TestMultiattackCountsFollowTheSRDText` reads the Multiattack text of every creature and checks the number the engine uses, with an explicit list of the creatures whose text it cannot read, each with the reason.

## Summons

The `summon` effect is a closed type of `effects/spells.json`, next to `hp_pool`, `hp_threshold`, `zero_hp_target`, `flat_heal`, `temp_hp` and `max_hp`.
Loading refuses a creature, field, CR or attack mode that does not exist.
Either it lists the creatures (`creatures`, with `count`, `count_per_level` and what each may do, `attack`: `none`, `reaction` or `full`; `unlocks` for a feature that adds forms) or it gives CR bands of a creature type (`options` with `count` and `max_cr`, `multiplier_at_level`).

| Spell | What it summons |
| --- | --- |
| `spell:find-familiar` | The SRD's forms, one creature, a 1-hour ritual; it does not attack. The Pact of the Chain (`feature:pact-of-the-chain`) adds Imp, Pseudodragon, Quasit and Sprite and makes every warlock familiar attack, with the reaction only. |
| `spell:animate-dead` | Skeleton or Zombie: one creature, plus two per level above the 3rd. |
| `spell:conjure-animals` | Beasts only: 1 of CR 2, 2 of CR 1, 4 of CR 1/2 or 8 of CR 1/4, multiplied at the 5th, 7th and 9th level; concentration. |

Find Steed is not covered.
Whoever spends the slot and creates the creatures is the [play module](play/README.md); `rules` only says what is allowed.

## Wild Shape

The druid's `wild_shape` effect (in `effects/druid.json`, at levels 2, 4 and 8) carries `max_cr`, `no_fly` and `no_swim`: level 2 allows CR up to 1/4 without flying or swimming, level 4 up to 1/2 without flying, level 8 up to 1.
Loading refuses a `max_cr` that is not a CR and any other field.
`WildShapeForms(build)` lists the beasts the druid may assume now; `WildShapeAllows` checks one.

`WildShapeDerived(sheet, beast)` is the druid's sheet in beast form, by the SRD:

- armor class, hit points (a separate pool: `HitPointsMax` is the beast's), speeds, Strength, Dexterity and Constitution, attacks and senses come from the beast;
- Intelligence, Wisdom and Charisma, proficiency bonus, features and resources stay the character's;
- skill and saving throw proficiencies stay and gain the beast's, and where both have the same, the larger bonus wins;
- the character's darkvision only holds if the beast also has it (the larger range wins);
- no spells: `Spellcasting`, `Spells` and `PactMagic` come out empty, unless Beast Spells (`CastsInBeastForm`) applies.

## Magic items

The SRD 5.1 magic items (MR-044) enter `rules` as data, without effects: `srd51.MagicItem` has the key `item:<index>`, the SRD name, category, rarity (`common`, `uncommon`, `rare`, `very_rare`, `legendary`, `artifact` or `varies`), attunement and by whom, variants, and the English text.
The Portuguese names are `item:<index>` in `names_pt.json`, one per item and variant.
The gold values (SRD 5.2.1 table) belong to the treasure generator, not to the items.

**Families and variants.**
The SRD lists "Armor +1, +2 or +3" once, as a family, and again as three variants.
Each is a real item with its own key: the family holds `Variants` (the keys, in SRD order) and each variant holds `VariantOf` and its own rarity.
A family whose variants differ has the rarity `varies`.
The Crystal ball is the special case: a family that is also an item (`Standalone`, a fixed list in the importer), drawn as the common Crystal ball among the very rare and its variants among the legendary.

**Attunement.**
`Attunement` is true when the SRD header says "(requires attunement)"; `AttunementBy` keeps the restriction in English ("by a paladin") and `AttunementByPT` the Portuguese label (`attunement:<restriction>` in `names_pt.json`; loading refuses a restriction without a label and the reverse).
An attunement that applies to one property only stays in the text.

**What gets drawn.**
A flat list would overweight the families, so `MagicItemUnits(rarity)` returns units: each standalone item and one unit per (family, rarity), with that rarity's variants in `Options`.
Whoever draws picks a unit and, within it, an option: the Ring of Resistance is a rare unit with ten options.
A family is never a unit in a rarity its variants do not have, except the Crystal ball.

| Function (`rules.Content`) | What it does |
| --- | --- |
| `MagicItems()` | All entries (families and variants), sorted by Portuguese name, as `MagicItemEntry`. Copies. |
| `MagicItem(key)` | One entry with the SRD text in English (`Desc`) and our translation (`DescPT`, empty while missing: `DescPTMissing()`). |
| `MagicItemUnits(rarity)` | The units a treasure may contain at that rarity, sorted by the key's Portuguese name. Copies. |
| `MagicRarities()` | The six rarities, from most common to artifact. |

**Consumables.**
`effects/consumables.json` is handwritten: the potion and scroll categories and the single-use items (ammunition, arrow of slaying, the dusts, elemental gem, the manuals and tomes that lose their magic once read, and so on).
Loading refuses a category or item that does not exist, a repeated one and one the category already covers.
The SRD 5.2.1 value table halves a consumable, except the Spell scroll, which `SpellScroll` marks; the treasure generator applies it through `Content.MagicItemValue`.

## Trap and light presets

`effects/traps.json` (the SRD's eight sample traps, in our words and with the SRD's numbers, plus the severity table and the damage per level) and `effects/lights.json` (the SRD's lights, in feet) are handwritten and checked on load.
The Portuguese names are `trap:<key>` and `light:<key>`.
`Content.TrapPresets()`, `TrapPreset(key)`, `TrapSeverities()`, `TrapDamageByLevel()` and `LightPresets()` return deep copies.

- A trap has the effect parts of the design (an attack, damage that always hits, conditions that always apply, and a save with what happens on failure and on pass), the DC to notice and to find, the trigger (`enter` or `manual`), the area (1 to 4 squares) and the targets (`area` or `manual`).
- A pit has `fall_ft` (a fall of 1d6 per 10 ft) and whoever falls ends Prone.
- The SRD gives the DC to notice only for some traps; the DC to find is ours and `find_dc_ours` marks it (`TrapPreset.FindDCIsOurs`).
- Lights are circles with `bright_ft` and `dim_ft`.
  The bullseye lantern (a cone) is left out: the app only lights circles.

`ContentService.ListTrapPresets` and `ListLightPresets` deliver them to the master's map form.
A trap or light on a map keeps its own copy of the numbers (see the [maps module](maps/README.md)).
