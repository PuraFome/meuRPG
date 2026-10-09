/**
 * Local, hand-written mirrors of the wire vocabulary
 * `meurpg.characters.v1` defines (`proto/meurpg/characters/v1/characters.proto`).
 * Deliberately kept separate from the generated enums in
 * `../../../gen/meurpg/characters/v1/characters_pb` — every presentational
 * component in `pages/` is built and tested against these small local
 * types, never against `gen/` directly (see each page's `*.types.ts` top
 * comment). The one place that maps a generated `CharacterKind` /
 * `CharacterState` / `CharacterBlockedReason` value onto these strings is
 * each page's `*-source.live.ts` (phase 2) — nowhere else needs to change
 * if the wire enum ever adds a value these types don't have a case for yet.
 */

/** Mirrors `CharacterKind`. */
export type CharacterKind = 'player' | 'enemy' | 'boss' | 'minion' | 'story';

/** Mirrors `CharacterState`. `'pending'` is a character created through an
 * invite that requires approval, waiting for the master to approve or
 * reject it (RN-15, MR-024). */
export type CharacterState = 'draft' | 'locked' | 'dead' | 'pending';

/** Mirrors `Review`: where the master's review of a pending character stands (RN-15). */
export type ReviewStatus = 'awaiting' | 'changes_requested' | 'resubmitted';

/** `Character.kind` decides which sheet shape it carries — full for player,
 * enemy and boss; basic for minion and story (`CharacterSheet`'s `oneof`,
 * plan §4). */
export function isFullSheetKind(kind: CharacterKind): boolean {
  return kind === 'player' || kind === 'enemy' || kind === 'boss';
}

/** The six ability scores, in the fixed sheet order (Força through
 * Carisma — see `character-labels.ts`). */
export const ABILITY_KEYS = ['str', 'dex', 'con', 'int', 'wis', 'cha'] as const;
export type AbilityKey = (typeof ABILITY_KEYS)[number];

/** Mirrors `CharacterBlockedReason` (characters.proto): the detail
 * `CreateCharacter`, `UpdateCharacter` and `UpdateCharacterStory` attach to
 * a `failed_precondition` response. */
export type CharacterBlockedReason =
  | 'sheet_locked'
  | 'character_dead'
  | 'living_character_exists'
  | 'story_locked'
  | 'not_pending'
  | 'awaiting_approval'
  | 'no_changes_requested'
  | 'archived_content'
  | 'switched_off_content';
