import { timestampDate } from '@bufbuild/protobuf/wkt';

import type { SceneClue } from '../../../gen/meurpg/maps/v1/maps_pb';
import { tight } from '../format/text';
import { formatClock } from '../../shared/session-time/session-time';

/** A scene has at most 30 clues (maps.proto, `AddSceneClue`). */
export const CLUE_LIMIT = 30;
/** One clue: 1 to 500 characters, one line. */
export const CLUE_MAX = 500;
/** "Ganchos e anotações": up to 4.000 characters. */
export const HOOKS_MAX = 4000;

/** A player character the master may give a clue to. */
export interface CluePlayer {
  readonly id: string;
  readonly name: string;
  /** The player's display name, or `''`. */
  readonly playerName: string;
}

/** Characters count as the server counts them: code points, not UTF-16 units. */
export function textLength(text: string): number {
  return [...text].length;
}

/** What is wrong with a clue's text, in words, or `''` when it is fine. The
 * server stays the authority; this is said first, under the field. */
export function clueTextError(text: string): string {
  const length = textLength(text.trim());
  if (length === 0) {
    return `Escreva a pista antes de salvar. Ela pode ter até ${CLUE_MAX} caracteres.`;
  }
  if (length > CLUE_MAX) {
    return tight(
      `A pista passa de ${CLUE_MAX} caracteres: tem ${length}, tire ${length - CLUE_MAX}.`,
    );
  }
  return '';
}

/** Pasted line breaks become spaces: a clue is one line. */
export function oneLine(text: string): string {
  return text.replace(/\s*[\r\n]+\s*/g, ' ');
}

/** "3 pistas", "1 pista". */
export function clueCount(n: number): string {
  return `${n} ${n === 1 ? 'pista' : 'pistas'}`;
}

/** "Mais 28 pistas na lista." for a list folded after two rows. */
export function moreClues(hidden: number): string {
  return `Mais ${hidden} ${hidden === 1 ? 'pista' : 'pistas'} na lista.`;
}

export type ClueAudienceKind = 'all' | 'some' | 'none';

export interface ClueAudience {
  readonly kind: ClueAudienceKind;
  /** The characters who have it, by name. */
  readonly names: readonly string[];
  readonly label: string;
}

function nameOf(clue: SceneClue, id: string, players: readonly CluePlayer[]): string {
  const given = clue.revealedTo.find((r) => r.characterId === id)?.characterName;
  return given || (players.find((p) => p.id === id)?.name ?? 'Personagem');
}

/**
 * Who has a clue, in words: "Todos" when every player character has it,
 * "Só Brisa" / "Só Brisa e Toren" for some, "Ninguém ainda" for none. `players`
 * are the campaign's player characters; empty while they load, when "Todos"
 * can't be told (the names are listed instead).
 */
export function clueAudience(clue: SceneClue, players: readonly CluePlayer[]): ClueAudience {
  const have = new Set(clue.revealedTo.map((r) => r.characterId));
  if (have.size === 0) {
    return { kind: 'none', names: [], label: 'Ninguém ainda' };
  }
  const names = [...have].map((id) => nameOf(clue, id, players));
  if (players.length > 0 && players.every((p) => have.has(p.id))) {
    return { kind: 'all', names, label: 'Todos' };
  }
  return { kind: 'some', names, label: `Só ${listNames(names)}` };
}

/** "Brisa", "Brisa e Toren", "Brisa, Toren e Aldo". */
export function listNames(names: readonly string[]): string {
  if (names.length <= 1) {
    return names[0] ?? '';
  }
  return `${names.slice(0, -1).join(', ')} e ${names[names.length - 1]}`;
}

/** The latest time one of the recipients got the clue, "21:26". */
export function revealedClock(clue: SceneClue): string {
  let last = 0;
  for (const r of clue.revealedTo) {
    if (r.revealedAt) {
      last = Math.max(last, timestampDate(r.revealedAt).getTime());
    }
  }
  return last > 0 ? formatClock(new Date(last)) : '';
}

/** The open scene's state line: "Revelada para todos às 21:20", "Revelada só
 * para Brisa às 21:26" (a name, never "a Brisa": the data has no gender). */
export function revealedLine(clue: SceneClue, players: readonly CluePlayer[]): string {
  const audience = clueAudience(clue, players);
  if (audience.kind === 'none') {
    return audience.label;
  }
  const at = revealedClock(clue);
  const who = audience.kind === 'all' ? 'para todos' : `só para ${listNames(audience.names)}`;
  // The time never splits from "às" (a no-break space).
  return `Revelada ${who}${at ? ` às\u00a0${at}` : ''}`;
}

/** The players who do not have the clue yet. */
export function playersWithout(
  clue: SceneClue,
  players: readonly CluePlayer[],
): readonly CluePlayer[] {
  const have = new Set(clue.revealedTo.map((r) => r.characterId));
  return players.filter((p) => !have.has(p.id));
}

/** The reveal button's words, by who is checked: "Revelar a pista" (nobody),
 * "Revelar para Brisa", "Revelar para 2 jogadores", "Revelar para todos" (only
 * when the checked are every player of the campaign) and "Revelar aos outros"
 * (everyone who does not have it yet is checked): never "todos" when someone
 * already had the clue. `open` is how many can still receive it, `players` how
 * many the campaign has. */
export function revealLabel(
  picked: readonly CluePlayer[],
  open: number,
  players: number = open,
): string {
  if (picked.length === 0) {
    return 'Revelar a pista';
  }
  if (picked.length === 1) {
    return `Revelar para ${picked[0].name}`;
  }
  if (picked.length === open) {
    return open === players ? 'Revelar para todos' : 'Revelar aos outros';
  }
  return `Revelar para ${picked.length} jogadores`;
}

/** The sentence under the list in the reveal dialog. */
export function revealSummary(
  picked: readonly CluePlayer[],
  open: number,
  players: number = open,
): string {
  if (picked.length === 0) {
    return 'Ninguém marcado. Escolha quem recebe a pista.';
  }
  if (picked.length === open && picked.length > 1) {
    return open === players
      ? `A pista vai para todos os ${players} jogadores.`
      : `A pista vai para os outros ${open} jogadores.`;
  }
  return tight(
    `A pista vai para ${picked.length} de ${open} ${open === 1 ? 'jogador' : 'jogadores'}: ${listNames(picked.map((p) => p.name))}.`,
  );
}
