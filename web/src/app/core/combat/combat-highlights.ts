import {
  type CharacterHighlights,
  type HighlightCategory,
  HighlightKind,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { joinNames } from '../play/stage-view';

const NBSP = ' ';

/** What a category is called, the Material icon beside it and the line under
 * it when the number alone does not say what it counts (the Tanque). */
const KINDS: Record<number, { label: string; icon: string; sub?: string }> = {
  [HighlightKind.MOST_DAMAGE]: { label: 'Mais dano causado', icon: 'bolt' },
  [HighlightKind.MOST_HEALING]: { label: 'Mais cura', icon: 'favorite' },
  [HighlightKind.TANK]: { label: 'Tanque', icon: 'shield', sub: 'mais dano recebido' },
  [HighlightKind.FINAL_BLOW]: { label: 'Golpe final', icon: 'target' },
  [HighlightKind.CRITICAL_HITS]: { label: 'Acertos críticos', icon: 'casino' },
  // Only the session's summary has it: the checks passed in scenes that showed their DC.
  [HighlightKind.CHECKS_PASSED]: { label: 'Mais testes passados fora do combate', icon: 'task_alt' },
};

/** The number with its unit, tied by a no-break space: "23 de dano", "9 de
 * cura", "2 inimigos", "1 crítico". */
export function highlightValue(kind: HighlightKind, value: number): string {
  switch (kind) {
    case HighlightKind.MOST_HEALING:
      return `${value}${NBSP}de${NBSP}cura`;
    case HighlightKind.FINAL_BLOW:
      return `${value}${NBSP}${value === 1 ? 'inimigo' : 'inimigos'}`;
    case HighlightKind.CRITICAL_HITS:
      return `${value}${NBSP}${value === 1 ? 'crítico' : 'críticos'}`;
    case HighlightKind.CHECKS_PASSED:
      return `${value}${NBSP}${value === 1 ? 'teste' : 'testes'}`;
    default:
      return `${value}${NBSP}de${NBSP}dano`;
  }
}

/** One tile of "Destaques do combate". */
export interface HighlightTile {
  readonly kind: HighlightKind;
  readonly label: string;
  readonly icon: string;
  /** Under the label: "mais dano recebido" for the Tanque. */
  readonly sub: string;
  /** "23 de dano": the largest text of the tile. */
  readonly value: string;
  /** Everyone who has that number: "Toren", "Pensantus e Toren". */
  readonly names: string;
  /** More than one: the line says "cada um". */
  readonly tie: boolean;
  /** The winners' character IDs, to mark the reader ("Você"). */
  readonly characterIds: readonly string[];
}

/**
 * The categories that have a winner, in the order the server sends them (the
 * order of `HighlightKind`); a category where everybody has 0 is not there.
 * For the master's session summary, `tried` says how many checks each winner
 * rolled, and the "Mais testes passados" tile says "de 5 tentados" when the
 * winners rolled the same number (a tie of different numbers says nothing).
 */
export function highlightTiles(
  res: { readonly categories: readonly HighlightCategory[] },
  tried: ReadonlyMap<string, number> = new Map(),
): HighlightTile[] {
  return res.categories.map((c) => {
    const kind = KINDS[c.kind] ?? KINDS[HighlightKind.MOST_DAMAGE];
    const counts = new Set(c.winners.map((w) => tried.get(w.characterId)));
    const [count] = counts;
    const triedLine =
      c.kind === HighlightKind.CHECKS_PASSED && counts.size === 1 && count !== undefined
        ? `de${NBSP}${count}${NBSP}${count === 1 ? 'tentado' : 'tentados'}`
        : '';
    return {
      kind: c.kind,
      label: kind.label,
      icon: kind.icon,
      sub: triedLine || (kind.sub ?? ''),
      value: highlightValue(c.kind, c.value),
      names: joinNames(c.winners.map((w) => w.name)),
      tie: c.winners.length > 1,
      characterIds: c.winners.map((w) => w.characterId),
    };
  });
}

/** One row of the master's "Números de cada jogador": every number, zeros
 * included. */
export interface HighlightRow {
  readonly characterId: string;
  readonly name: string;
  readonly damageDealt: number;
  readonly healingDone: number;
  readonly damageTaken: number;
  readonly finalBlows: number;
  readonly criticalHits: number;
}

export function highlightRows(characters: readonly CharacterHighlights[]): HighlightRow[] {
  return characters.map((c) => ({
    characterId: c.characterId,
    name: c.name,
    damageDealt: c.damageDealt,
    healingDone: c.healingDone,
    damageTaken: c.damageTaken,
    finalBlows: c.finalBlows,
    criticalHits: c.criticalHits,
  }));
}

/** One tile of "Seu resultado": a number of the reader's own character. */
export interface OwnNumber {
  readonly label: string;
  readonly value: string;
}

/**
 * The reader's own numbers, for a player: the server sends a player exactly
 * one row of the table, their own character's, zeros included. Empty when
 * there is none (the character was not in the combat).
 */
export function ownNumbers(characters: readonly CharacterHighlights[], characterId: string): OwnNumber[] {
  const mine = characters.find((c) => c.characterId === characterId) ?? (characters.length === 1 ? characters[0] : undefined);
  if (!mine) {
    return [];
  }
  return [
    { label: 'Dano causado', value: String(mine.damageDealt) },
    { label: 'Dano recebido', value: String(mine.damageTaken) },
    { label: 'Golpes finais', value: String(mine.finalBlows) },
    { label: 'Cura', value: String(mine.healingDone) },
  ];
}
