import type { StageNpc } from '../../../gen/meurpg/play/v1/scene_pb';

/** How many NPCs the stage holds (the server refuses a fifth, MR-031). */
export const STAGE_LIMIT = 4;

/**
 * The letters a portrait without an image shows: the first two letters of a
 * one-word name ("Aldo" is "AL") and the first letter of each of the first
 * two words of a longer one ("Capitão Goblin" is "CG").
 */
export function initialsOf(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean);
  const letters =
    words.length >= 2
      ? `${firstLetter(words[0])}${firstLetter(words[1])}`
      : [...(words[0] ?? '')].slice(0, 2).join('');
  return letters.toLocaleUpperCase('pt-BR') || '?';
}

function firstLetter(word: string): string {
  return [...word][0] ?? '';
}

/** "2 de 4 em cena", with the numbers tied to their words (no-break spaces). */
export function stageCount(n: number): string {
  return `${n}\u00a0de\u00a0${STAGE_LIMIT} em cena`;
}

/** The sentence under a full stage, in the helper style of the rest of the
 * page: it comes before the dashed button and says why it is off. */
export const STAGE_FULL_REASON = `A cena comporta ${STAGE_LIMIT} NPCs. Tire um para pôr outro.`;

/** "Mira", "Mira e Aldo", "Mira, Aldo e Barão Ivo". */
export function joinNames(names: readonly string[]): string {
  if (names.length <= 1) {
    return names[0] ?? '';
  }
  return `${names.slice(0, -1).join(', ')} e ${names[names.length - 1]}`;
}

/**
 * What a screen reader says when the stage moves, as the artboard's note
 * words it: "Mira entrou na cena.", "Capitão Goblin saiu da cena.", "Aldo
 * fala.", "Ninguém fala.". Several changes read at once are joined in one
 * sentence per kind ("Aldo e Barão Ivo entraram na cena."), so the reader
 * never falls behind. Empty when nothing a person can hear changed. The
 * speaker is only named when the one who spoke did not just leave.
 */
export function stageAnnouncement(prev: readonly StageNpc[], next: readonly StageNpc[]): string {
  const before = new Set(prev.map((n) => n.id));
  const after = new Set(next.map((n) => n.id));
  const came = next.filter((n) => !before.has(n.id)).map((n) => n.name);
  const went = prev.filter((n) => !after.has(n.id)).map((n) => n.name);

  const parts: string[] = [];
  if (came.length > 0) {
    parts.push(`${joinNames(came)} ${came.length === 1 ? 'entrou' : 'entraram'} na cena.`);
  }
  if (went.length > 0) {
    parts.push(`${joinNames(went)} ${went.length === 1 ? 'saiu' : 'saíram'} da cena.`);
  }
  const was = prev.find((n) => n.speaking);
  const now = next.find((n) => n.speaking);
  if (now && now.id !== was?.id) {
    parts.push(`${now.name} fala.`);
  } else if (!now && was && after.has(was.id)) {
    parts.push('Ninguém fala.');
  }
  return parts.join(' ');
}
