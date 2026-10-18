# Rules module: table content (`Content.With`)

The content a master registers enters the engine as a layer over the SRD (MR-025, RN-23, ADR-0018): `(*Content).With(Overlay)` returns a new `*Content`, the SRD plus the table's entries, and never changes what it received.
`rules` stays pure; who stores the entries is [Characters](characters-table-content.md), and the editors are in the web app.
The code is in `rules/overlay*.go`.
For the engine around it see [Rules module](rules.md).

## The overlay

| Piece | What it does |
| --- | --- |
| `Overlay` | The revision, the keys switched off, the entries of seven kinds (`TableClass`, `TableSubclass`, `TableRace`, `TableSubrace`, `TableBackground`, `TableSpell`, `TableFeat`) and `Strict`, the keys being written now. Each entry starts with a `TableEntry` (key, Portuguese name, `Archived`, the revision and date of its last change) and carries the features (`TableFeature`: key, name, text, effects). The fields follow what the editors fill in, so the protos map one to one. |
| Keys | Every table key is `<kind>:<name>@mesa`, the name 1 to 60 characters of `[a-z0-9-]`; a feature's key starts `feature:`, `trait:` or `background-feature:`. `With` refuses a key without `@mesa`, of another kind, malformed or already taken (even by another entry's feature). Because the prefix is the SRD's, every place that decides by prefix works for table keys. |
| Reserved words | `With` refuses (`reserved_key`) a key, of an entry or a feature, with `magical-secrets`, `wild-shape` or `ability-score-improvement` in the name, or ending in `-spellcasting`, because the engine, `play` and the web react to those words in SRD keys. The keys the engine writes carry the kind (`feature:class-<name>-spellcasting@mesa`, `feature:subclass-<name>-spellcasting@mesa`, `feature:class-<name>-ability-score-improvement-<n>@mesa`), so a class and a subclass of the same name never collide. |
| Copy-on-write | The SRD is a singleton many requests read at once. `With` copies (shallowly) only the maps it changes, adds the entries, rebuilds the indexes and the catalogue, and leaves the values shared. A table subclass of an SRD class enters a copy of the class with the new subclass list; the same for a race's subraces. Two contents made from the same SRD do not see each other, and a `Content` that already has a table layer does not accept another. |
| Same loaders | Formulas go through the usual compiler (`classLevel("cavaleiro-runico@mesa")` works) and effects through the SRD's `compileEffect`; a repeated formula is compiled once per `With`. |
| Archived | An `Archived` entry keeps holding everywhere (`Derive`, `Validate`, names, level-up); the catalogue marks it. Refusing an archived key as a new choice needs the earlier sheet, so it belongs to the server: `Content.ArchivedKeys(build)` lists the archived keys a sheet uses and `TableKeys(build)` the `@mesa` keys. |
| Switched off | `Overlay.Off` carries the keys the master switched off for players. See [Switches](#switches-and-hidden-keys). |
| Missing key | A sheet that cites an `@mesa` key the content lacks opens with an `unknown_key` `Issue`; nothing panics. `Validate` refuses it like any unknown key. |
| Version | `srd51@<commit>+fx.<n>+mesa.<revision>`. The SRD part does not change; the revision is per campaign, so a content cache is indexed by campaign and version, never the version alone. |
| Errors | `*OverlayError{Key, Field, Reason, Message}`. `Field` is the path in the entry as the caller sent it (`classes[2].levels[4].features[0].effects[1].formula`); `With` sorts entries by key so the result does not depend on order, but the indexes are the caller's. `Reason` is a stable code: `limit`, `bad_key`, `duplicate_key`, `reserved_key`, `bad_name`, `bad_text`, `dangling_reference`, `forbidden_effect`, `bad_formula`, `bad_table`, `bad_casting`, `bad_value`, `overlay`. The server turns each into a field violation and the web into Portuguese. `OverlayError.More` carries the rest of an entry's violations, so a class reports every one at once. |

## Limits

All checked in the first pass, before anything compiles; the error names the limit.

| What | Limit |
| --- | --- |
| Entries (of the seven kinds; features do not count) | 300 (`MaxOverlayEntries`) |
| Features | 60 per class and 60 per subclass, counted apart (`MaxTableFeatures`); the features the engine writes do not count |
| Traits | 60 per race and per subrace (`MaxTraits`) |
| Options in a feature's option list | 60 (`MaxFeatureOptions`) |
| Effects | 20 per feature (`MaxFeatureEffects`); 2,000 in the set, the effects the engine writes included (`MaxOverlayEffects`) |
| Distinct formulas | 500 in the set; a repeated one counts once (`MaxOverlayFormulas`) |
| Items in a list of an effect | 20 |
| `choice` count | 1 to 18 (`MaxChoiceCount`) |
| Name | 80 characters; a name over it is `bad_name` |
| Text | 4,000 characters a paragraph (`bad_text`), 50 paragraphs (`limit`) |

The 64 KiB per entry is a storage limit, checked by the server.
Without these budgets, hundreds of classes of 60 features of 20 effects each cost seconds and gigabytes.
The ADR-0018 budget for a realistic content (300 entries) is about 25 ms and 4 MB per `With`; measure with `go test -run '^$' -bench BenchmarkWith -benchmem ./internal/rules` (without `-race`), and the memory a content retains with `MEURPG_MEASURE=1 go test ./internal/rules -run TestWithMemory -v`.
Cache and memory limits of the live source are in [Characters](characters-table-content.md#the-live-source).

## The closed menu of effects

A table feature has effects of these types only; anything else is `forbidden_effect`.

| Type | Notes |
| --- | --- |
| `modifier`, `proficiency`, `sense`, `roll_mode`, `extra_attack`, `note` | As in [Effects](rules.md#effects). |
| `resource` | With uses and recharge. The resource may not have the name of an SRD one. |
| `grant_action` | May carry `resource`: the name of a `resource` effect of the same entry, its class, or an SRD resource. Each use spends one and the action is off with none left. Refused as `bad_reference` when nothing defines it. |
| `ranged_attack` | Checked at its fields: `applies` is `spell` or `weapon`; `range_multiplier` is 2 or 3 and only on `spell`; `no_long_range_disadvantage` only on `weapon`; an effect that sets none of the three switches is refused at `.type`; `tags` at `.tags`; the switches on any other type are stray fields. |
| `extra_damage` | See [Extra damage](#extra-damage). |
| `choice` | Picks among SRD options (a key the SRD has) or, for kind `feature`, among the option list of a table feature. Kinds: `skill`, `expertise`, `cantrip`, `spell`, `language`, `tool`, `feature`, `feat`. A `feature` choice never asks for more than the options it lists. |
| `ability_increase` | Feats only. `count` different abilities from `from`, `value` 1 or 2; at most one per feat. The sheet keeps the result in `ExtraAbilityBonuses`. |
| `superiority_die` | See [Maneuvers](#maneuvers). |
| Granted spell | A `note` with `spells` (like the SRD's Infernal Legacy): the character knows the cantrip, or a spell once per day when the same feature has a `resource`. A granted spell is on the sheet's list, does not count in the class numbers and needs no slot. Only a `note` grants spells. |

Never allowed: `handler` (not even one that exists, since a handler reads columns of an SRD class a table class lacks), `spellcasting` and `wild_shape` (the engine writes casting from the class), `replaces`, `beast_spells`, `song_of_rest`.
A text-only feature always holds.

**Option lists.**
A class or subclass feature may carry `TableFeature.options` (a table's maneuvers).
Each option is a feature of its own (name, text, effects) with a server-made key (`feature:<stem><option name>@mesa`) and the offering feature as its `Parent`.
An option is in no level row: it is gained only when picked, and applies (its effects, its action under the parent) only while the character has the parent.
A `feature` choice with an empty `from` offers all the feature's options; on another feature of the same entry, `from` may list option keys (a later level's "two more").
The choice engine deals one pool of picks to choices that share options, in level order, so options an earlier choice took are blocked in the later one, and a pick whose parent is gone is reported as no longer valid.
Refused, each at its field: options on a trait or a background feature, nested options, options with no `choice` of features, a repeated option name or `from` key, a `from` key of another entry's list, a `choice` inside an option, more options than the limit.

### Maneuvers

`superiority_die` (menu name "Dado de manobra"; fields `applies`, `resource`, `value`, `ability`, `economy`, `text_pt`; `rules/maneuvers.go`) is a die from a resource.
`applies` is one of `damage`, `attack`, `grapple`, `reduce_melee_damage`.
Checked at its fields:

- the resource is one the entry or its class defines (`.resource`, `bad_reference`);
- `value` compiles;
- a reduction names `ability` and costs the `reaction`; a grapple costs a `bonus_action` or nothing; a die added to an attack or damage costs nothing (`.economy`, `.ability`, `bad_value`).

`Derive` fills `Derived.Maneuvers` from the effects of what the character has (picked options included), with the die's sides worked out for the level, and only when the resource exists at that level; a `value` that gives fewer than 2 or more than 100 sides is ignored with a notice.
An option with a `superiority_die` does not list its `grant_action`, so the resource is not spent twice.
How combat spends the die (damage, grapple, reduction) is in the [play module](play/README.md).

### Extra damage

`extra_damage` (menu name "Dano extra"; fields `applies`, `value`, `resource`, `when`, `text_pt`; `rules/extradamage.go`) is fixed damage a feature adds to a weapon hit, once per turn, when the target is what `applies` asks.
Checked at its fields:

- `applies` is `any_target` or `larger_target` (`.applies`, `bad_value`);
- `value` is required and compiles (`.value`, `bad_value`, `bad_formula`);
- `resource`, when set, is one the entry or its class defines (`.resource`, `bad_reference`);
- any field the type does not read is refused for the entry being written (`economy`, `ability`...), and a feature has at most one (`.type`, `forbidden_effect`).

`Derive` fills `Derived.ExtraDamages` with the amount worked out; a resource the character does not have at that level leaves the effect out.
The attack card offers it after a hit as a choosable part ([Attacks and damage](play/attacks-and-damage.md)).

## The class

The ADR-0018 contract: hit die (6, 8, 10 or 12), two saves, skills, proficiencies, multiclass prerequisites, the subclass level (3 if unsaid) and exactly 20 table rows (proficiency bonus, which defaults to the SRD's, the features, cantrips, spells known and slots).
The ability score improvement levels default to 4, 8, 12, 16 and 19; the server writes the features `<class>-ability-score-improvement-<n>@mesa`, which the engine finds by key like the SRD's.

**Casting.** `none`, `full`, `half` or `pact`; the ability; `known` or `prepared`; and the list: an SRD class's (or a table class's own), plus the spells that cite the class.
The number of prepared spells is a formula (default `max(1, mod + level)`, with half the level for the half caster).
The "Conjuração" feature is written by the engine.
Before the level casting starts the columns are zero; after, they have slots (a single slot level for pact).

**Third caster.**
It is the casting of a subclass (`CastingThird`), as in the SRD: the parent class does not cast, and the subclass brings the table of cantrips, spells and slots for each level from the level it starts to 20.
The multiclass rule (`multiclassSlots`) counts the full caster's whole level, half of the half caster's and a third (rounded down) of the third caster's, and reads the slots from the SRD full caster table, never a table class's.
Alone, the caster uses its own table.

**Subclass.**
It carries the parent class (SRD's or the table's, which never changes), the features by class level, the optional third-caster casting and the always-prepared spells by class level, which do not count against the prepared limit.
The choice level is the parent class's.
The SRD's Fiend patron is the exception: its spells are an expanded list (`expanded_list_corrections` in `effects/corrections.json`), so they only join the warlock's list, to choose from, and count against the spells known.

## Race, subrace, background

- **Race and subrace** (2014 format): ability bonuses, size, speed, darkvision (a sense of the race itself), languages and how many to choose, and the traits as features with effects from the menu.
  "+2 and +1 of your choice" is the master's option (`ChoiceBonuses`): the player puts the bonuses in the manual ones and `Derive` warns (`race_bonus`) while they do not cover.
  A subrace adds bonuses and traits.
- **Background:** two skills, tools, languages to choose, the equipment as text and the feature, usually a note.
  A table background's equipment reaches `Derive` (`background_equipment_pt`).

## Feat

`TableFeat` (`feat:<name>@mesa`) has a name, text, a `FeatPrerequisite` and effects (the feat owns them; it has no features).
The prerequisite is all of: minimum scores that must all be met, scores of which one is enough, a proficiency key, "can cast at least one spell", a race or subrace key, a total level.
`Content.Feats`, `FeatOptions(build)` and `CheckFeat` answer who qualifies and what they lack (`FeatUnmet`).
The SRD's feat (Grappler, from `5e-SRD-Feats.json`, effects in `effects/feats.json`) is in the same `feats` map.
`Build.Feats` are the keys a character took: `Derive` adds their effects and lists each among the features (source "Talento").
A character that no longer meets the prerequisite of a feat it holds keeps the feat on the sheet with the issue `feat_prerequisite`, and its effects do not apply until it meets it again.

## Spell

A table spell has spell level, school, casting time, range, components (with the material), duration, concentration, ritual, classes, text and "at higher levels", plus the target (`SpellTarget`): one creature, several (with one more per level above), an area (cone, cube, cylinder, line or sphere, in feet, in steps of 5; an area takes no spell attack) or only the caster.
Time, range, duration and components have input-only types without the fields `With` would ignore.
Optionally it has an attack, a saving throw (half or nothing), damage (extra dice per level above or per cantrip tier) and healing, whose dice are plain (`8d6`, no bonus) and of a die the table rolls (d4, d6, d8, d10, d12, d20 or d100, the dice `platform/dice` rolls).
`With` writes the tables in the SRD format and uses the same constructor, so `SpellDetails` equals the SRD spells' (`DamageAt`, `HealAt`) and combat resolves a table spell the same way.
The target is `SpellDetails.Target` (`IsArea()`, `AnyNumber()`, `MaxTargets(level, spellLevel)`); the engine stores the size in feet and writes the label (`LabelPT`, "Cone de 4,5 m").
How an SRD spell's target is decided is in [Characters](characters-table-content.md#spell-targets-and-the-magias-page).
A spell never crosses between cantrip and leveled spell on an edit (the sheet stores them in different lists); the server refuses that change.

The spell-list question "is this spell on this class's list?" has one answer, `content.onList` (`spell.classes` plus the list the class reuses); `Derive` and level-up use it.
The catalogue answers from the other side, `spellClasses`.
`Validate` does not look at lists: an off-list spell is a `Derive` `Issue`.
Combat uses `Spellcasting.SpellList`, the list the caster reads: the class's or, for a third caster, the class the subclass casts from.

## Switches and hidden keys

The master switches classes, subclasses, races, subraces, backgrounds, spells and feats on or off per campaign, SRD and table alike; everything is on by default.
`Overlay.Off` carries the switched-off keys to `With`, which stores them on a copy and ignores a key that is not switchable (`Content.Switchable`).

- `Content.Off(key)` is the key's own switch.
- `Content.Hidden(key)` is what the player does not receive: archived, switched off, or the subclass or subrace of a class or race that is switched off or archived.
  The children keep their own switch, so turning the class back on returns each subclass as it was.
- A parent only hides the children of whoever does not have it: `Content.OffKeys(build)` lists the switched-off keys a sheet uses, and a subclass (or subrace) under a switched-off class (or race) counts only when the sheet does not have that class (or race).
  A sheet that already has the switched-off class keeps reaching its subclass level; a new sheet cannot take the class, so not the subclass either.
- `Content.AnyHidden()` lets a caller that only filters skip the work when nothing is hidden.

## "A classe mudou"

A change to table content takes effect at once, also on locked sheets.
`Content.ChangedSince(build, savedRevision, baselines)` lists the table entries a sheet uses that changed after the revision it was saved at; each entry stores the revision and date of its own last change (an archive is not a change).
`Derive` ties every notice to the table entries it depends on (`Issue.Keys`: the entry of the field it points to, and the entries the number is computed from, such as the classes in a skill count or all the keys in a formula that failed), so the server can show, for each changed entry, only the new notices whose keys include it.

`Issue` also carries `ChangeMessage` and `ChangeSubject`: the sentence of the same notice told as a change, without the entry's name, and the class or subclass it is about.
Only notices tied to a table entry have one.
The server builds the final sentence where the changed entry is known (`changeSentence`): the name enters only when the sentence is about the very entry that changed.
The sentences:

| Subject | Sentence |
| --- | --- |
| Skills | "Guardião do Vale agora dá 2 perícias no nível 1; esta ficha tem 3." (with another skill source: "O total de perícias para escolher agora é 4; esta ficha tem 3.") |
| Cantrips | "Mago agora conhece 3 truques; esta ficha tem 4." |
| Spells known | "… agora conhece 5 magias; esta ficha tem 6." |
| Spells prepared | "… agora prepara 4 magias; esta ficha tem 5." (the third caster is named by its subclass; with several sources: "O total de magias preparadas agora é 4; esta ficha tem 5.") |
| Subclass level | "Guardião do Vale agora escolhe a subclasse no nível 5; esta ficha tem nível 3 nela." |
| Multiclass | "Guardião do Vale agora pede outras habilidades para multiclasse; os desta ficha não cumprem." |
| SRD option no longer offered | "Guardião do Vale agora não oferece a escolha Estilo de Luta: Defesa; ela não vale mais nesta ficha." The notice is tied to the entry whose feature offers options of the same SRD set; if none offers it any more, there is no entry to blame and it does not appear in "A classe mudou". |

A sentence says what the entry gives today and what the sheet has, in the singular when it is 1.
Other notices (a spell that left the class list) use their own sentence.
The neutral sentence in `DerivedSheet.issues` keeps the notice's identity (`code`, field and sentence) and holds in a draft.

## The class editor's data

Two reads give the class editor everything it needs, so the browser computes nothing:

- **`TableDefaults`** returns the SRD proficiency bonus per level (+2 at levels 1 to 4 up to +6 at 17 to 20), the ability score improvement levels, the subclass level (3) and a 20-row table per casting type: none, full, half, pact and a third (the subclass's), each "knows" or "prepares".
  Each row is a `TableClassLevel` ready to be the class's `levels` (features empty): bonus, cantrips known, spells known and the nine slot counts.
  The numbers are read from the SRD itself, never from the campaign's content: full and third use the Wizard/Cleric's, half the Paladin's (starts at level 2), pact the Warlock's, "knows" the cantrips and spells known of the Sorcerer (full), the Ranger (half) and the Warlock; "prepares" has no spells known.
  The third caster, which SRD 5.1 lacks, is our default: the full table at a third of the level, rounded up, 2 cantrips (3 from level 10) and, for one that knows spells, 3 at levels 3 and 4 and one more every two levels; the master changes whatever they want.
- **`EffectMenu`** returns the twelve effect types of the menu with their Portuguese names and sentences, each one's fields (required or not, the field's kind and list, a number's limits, `always_shown` for an optional field the editor never folds away), and every closed list with Portuguese names: modifier targets, modes, proficiency targets, levels, advantage modes and targets, senses, recharges, action economies, choice kinds, `ranged_attack_applies`, skills, languages, tools and tag prefixes.
  It also carries the SRD option lists a "one option from a list" choice can offer, the formula functions with a sentence each, the class indexes `classLevel` accepts (SRD and table) and the limits.
  A test puts each value of each list in an effect and checks that `With` accepts it, so the menu never offers what the server refuses; it offers a subset of what the engine understands.

A field an effect type does not read is refused only when an entry is written (`Overlay.Strict`); in an entry already stored it is ignored on read, so a lost field never makes a campaign unreadable.
Every refusal of a class or subclass comes out with the exact field the editor draws, in the proto's names and with its indexes (`table_class.levels[4].slots[2]`, `table_class.casting.list_from`, `table_class.skill_from[2]`, `table_subclass.always_prepared[1].spell_key`), and a stable reason.

## Quality gate

ADR-0018 requires that a table class works end to end.
`TestLevelUpSweepTable` levels from 1 to 20, doing exactly what the options ask, one table class of each casting type (none, full that prepares, full that knows, half, pact) with subclasses (one with features, another with always-prepared spells) and the third casters of two SRD classes (the Fighter's Eldritch Knight, which knows, and the Rogue's Arcane Trickster, which prepares).
`TestLevelUpSweepTableMulticlass` repeats it with each of them together with an SRD class, in both orders.
No step may fail `CheckLevelUp` or `Validate`, leave an `Issue` or panic.
`TestDeriveTableCharacterGolden` pins a table character's sheet at levels 1, 5 and 11 (`testdata/golden/table-character.json`; rewrite with `go test ./internal/rules -update`).

## The area a spell keeps on the map

`TableSpell.Zone` (`TableSpellZone`: `Trigger` of `ZoneStartOfTurn`, `ZoneFirstEntry` or `ZoneBoth`, and `FollowsCaster`) is checked by `checkSpellZone` and kept in `Content.spellZones` (`Content.SpellZone`). Refused at `.zone.trigger` (not one of the three), `.zone.follows_caster` (needs the range Self if it follows, a distance if it does not) and `.zone` (a cantrip; a target that is not a sphere, a cylinder or a cube; not exactly one damage type; a spell attack or healing; neither a timed duration nor concentration). The combat reads it through `link.Spell.Zone` ([Spell zones](play/spell-zones.md)).
