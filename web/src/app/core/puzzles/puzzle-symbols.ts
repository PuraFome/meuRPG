import { PuzzleAlphabet } from '../../../gen/meurpg/play/v1/puzzles_pb';

/** One face of a wheel or a pillar: a stable key the app draws, and the Portuguese name written beside it. */
export interface SymbolFace {
  readonly key: string;
  readonly namePt: string;
}

/**
 * The faces the create form shows before the server has made the puzzle (it sends `symbols` for a puzzle that exists, and
 * the screens use those). The lists are the table's own, drawn for this app: the eight runes of a lock and the six glyphs
 * of the pillars. Only the order matters to the rules, and the server owns it; these are display copies, so a face the
 * server names differently always wins on a live screen.
 */
export const RUNES: readonly SymbolFace[] = [
  { key: 'moon', namePt: 'Lua' },
  { key: 'flame', namePt: 'Chama' },
  { key: 'wave', namePt: 'Onda' },
  { key: 'root', namePt: 'Raiz' },
  { key: 'eye', namePt: 'Olho' },
  { key: 'star', namePt: 'Estrela' },
  { key: 'stone', namePt: 'Pedra' },
  { key: 'leaf', namePt: 'Folha' },
];

export const GLYPHS: readonly SymbolFace[] = [
  { key: 'raven', namePt: 'Corvo' },
  { key: 'wolf', namePt: 'Lobo' },
  { key: 'serpent', namePt: 'Serpente' },
  { key: 'owl', namePt: 'Coruja' },
  { key: 'deer', namePt: 'Cervo' },
  { key: 'fish', namePt: 'Peixe' },
];

/**
 * The eight bells of a sequence (E10-12), the first `bells` of them in play. As for the runes, the server owns the list and
 * sends it in `PuzzleRun.symbols`; these are the form's own copy, to draw the bells before a puzzle exists.
 */
export const BELLS: readonly SymbolFace[] = [
  { key: 'round', namePt: 'Sino redondo' },
  { key: 'tall', namePt: 'Sino alto' },
  { key: 'wide', namePt: 'Sino largo' },
  { key: 'small', namePt: 'Sino pequeno' },
  { key: 'cracked', namePt: 'Sino rachado' },
  { key: 'thin', namePt: 'Sino fino' },
  { key: 'bent', namePt: 'Sino torto' },
  { key: 'deep', namePt: 'Sino grave' },
];

/** The first `count` bells: the faces of a sequence of `count` bells. */
export function bellFaces(count: number): readonly SymbolFace[] {
  return BELLS.slice(0, Math.max(0, Math.min(count, BELLS.length)));
}

const DIGITS: readonly SymbolFace[] = [...'0123456789'].map((c) => ({ key: c, namePt: c }));
const LETTERS: readonly SymbolFace[] = [...'ABCDEFGHIJKLMNOPQRSTUVWXYZ'].map((c) => ({
  key: c,
  namePt: c,
}));

/** The faces of a lock's alphabet, in the order a wheel turns through them. */
export function alphabetFaces(alphabet: PuzzleAlphabet): readonly SymbolFace[] {
  switch (alphabet) {
    case PuzzleAlphabet.DIGITS:
      return DIGITS;
    case PuzzleAlphabet.LETTERS:
      return LETTERS;
    default:
      return RUNES;
  }
}

/** The first `count` glyphs: the faces of a pillar that turns through `count` symbols. */
export function pillarFaces(count: number): readonly SymbolFace[] {
  return GLYPHS.slice(0, Math.max(0, Math.min(count, GLYPHS.length)));
}

/**
 * Our own simple stroke drawings, on a 24 × 24 grid (stroke 1.8, round). A digit or a letter has none: it is drawn as
 * the character. The name is always written beside the drawing, so a drawing is never the only way to tell faces apart.
 */
export const GLYPH_PATHS: Readonly<Record<string, { readonly d: string; readonly fill?: string }>> =
  {
    // The runes.
    moon: { d: 'M12 4a8 8 0 1 0 0 16a8 8 0 0 0 0-16zM12 4v16', fill: 'M12 4a8 8 0 0 1 0 16z' },
    flame: { d: 'M12 3c1 4 5 6 5 11a5 5 0 0 1-10 0c0-2 1-3 2-4 0 2 1 3 2 3 0-4-1-6 1-10z' },
    wave: { d: 'M3 9c2-3 4-3 6 0s4 3 6 0 4-3 6 0M3 15c2-3 4-3 6 0s4 3 6 0 4-3 6 0' },
    root: { d: 'M12 3v18M12 9l-5-4M12 9l5-4M12 14l-6 5M12 14l6 5' },
    eye: {
      d: 'M2 12c3-5 7-7 10-7s7 2 10 7c-3 5-7 7-10 7s-7-2-10-7zM12 9a3 3 0 1 0 0 6a3 3 0 0 0 0-6z',
      fill: 'M12 11a1 1 0 1 0 0 2a1 1 0 0 0 0-2z',
    },
    star: { d: 'M12 3l2.7 5.6 6.1.9-4.4 4.3 1 6.1L12 17l-5.4 2.9 1-6.1-4.4-4.3 6.1-.9z' },
    stone: { d: 'M5 16l2-8 6-3 6 4 1 7-5 3H8zM9 10l4 2 3-2M13 12l-1 6' },
    leaf: { d: 'M5 19C5 10 10 5 19 5c0 9-5 14-14 14zM5 19l9-9' },
    // The pillars' glyphs.
    raven: { d: 'M3 14c3-6 8-8 13-6l4-1-2 3c0 5-4 9-9 9-3 0-5-2-6-5zM15 11h.01M20 7l-2 2' },
    // Tall pointed ears and a long snout ending in a nose: a narrow face, nothing round.
    wolf: {
      d: 'M5.5 2l4 7h5l4-7 1.5 9.5-4 4V21h-8v-5.5l-4-4zM8.5 12l2.4 1.2M15.5 12l-2.4 1.2',
      fill: 'M10.5 19.5h3L12 21.3z',
    },
    serpent: {
      d: 'M17 5c-6-2-10 1-8 4s7 2 7 6-5 5-9 3M17 5l3 1-3 2',
      fill: 'M17 6.5a.6.6 0 1 0 0 1.2a.6.6 0 0 0 0-1.2z',
    },
    // A round face, two big round eyes with their pupils, a small beak.
    owl: {
      d: 'M12 3a9 9 0 1 0 0 18a9 9 0 0 0 0-18zM5.6 11a3.1 3.1 0 1 0 6.2 0a3.1 3.1 0 1 0-6.2 0M12.2 11a3.1 3.1 0 1 0 6.2 0a3.1 3.1 0 1 0-6.2 0',
      fill: 'M7.6 11a1.1 1.1 0 1 0 2.2 0a1.1 1.1 0 1 0-2.2 0M14.2 11a1.1 1.1 0 1 0 2.2 0a1.1 1.1 0 1 0-2.2 0M12 14.4l-1.4 2.6h2.8z',
    },
    deer: { d: 'M9 20l-1-6v-3h8v3l-1 6zM8 11L5 5M5 5L3 6M5 5l1-2M16 11l3-6M19 5l2 1M19 5l-1-2' },
    fish: { d: 'M3 12c4-6 10-6 14 0-4 6-10 6-14 0zM17 12l4-4v8zM8 11h.01' },
    // The bells: eight shapes that differ by their outline (a width, a height, a crack, a lean), never by color.
    round: {
      d: 'M4.5 16c0-6.5 2.8-10 7.5-10s7.5 3.5 7.5 10zM3.5 16h17M12 6V3.5',
      fill: 'M12 18.3a1.4 1.4 0 1 0 0 2.8a1.4 1.4 0 0 0 0-2.8z',
    },
    tall: {
      d: 'M9.2 3.5h5.6l2.2 12.5H7zM6 16h12',
      fill: 'M12 18.3a1.4 1.4 0 1 0 0 2.8a1.4 1.4 0 0 0 0-2.8z',
    },
    wide: {
      d: 'M2.5 15.5c0-4.5 4-7.5 9.5-7.5s9.5 3 9.5 7.5zM2 15.5h20M12 8V5.5',
      fill: 'M12 17.8a1.4 1.4 0 1 0 0 2.8a1.4 1.4 0 0 0 0-2.8z',
    },
    small: {
      d: 'M8 15c0-4.2 1.4-6.8 4-6.8s4 2.6 4 6.8zM7 15h10M12 8.2V6',
      fill: 'M12 17a1.1 1.1 0 1 0 0 2.2a1.1 1.1 0 0 0 0-2.2z',
    },
    cracked: {
      d: 'M4.5 16c0-6.5 2.8-10 7.5-10s7.5 3.5 7.5 10zM3.5 16h17M12 6.5l-2 3.2 3 2.2-2 3.8',
      fill: 'M12 18.3a1.4 1.4 0 1 0 0 2.8a1.4 1.4 0 0 0 0-2.8z',
    },
    thin: {
      d: 'M10.2 3.5h3.6l1.2 12.5H9zM7.5 16h9',
      fill: 'M12 18.3a1.2 1.2 0 1 0 0 2.4a1.2 1.2 0 0 0 0-2.4z',
    },
    bent: {
      d: 'M4 16L8.2 7.5C9.2 5.5 10.6 4.5 12.8 4.5c2.4 0 3.7 1.4 4.4 3.6L20 16zM3 16h18',
      fill: 'M12.8 18.3a1.4 1.4 0 1 0 0 2.8a1.4 1.4 0 0 0 0-2.8z',
    },
    deep: {
      d: 'M8.5 3.5h7l1 6.5c0 3 3.5 3.8 3.5 6.5H4c0-2.7 3.5-3.5 3.5-6.5zM3.5 16.5h17',
      fill: 'M12 18.8a1.4 1.4 0 1 0 0 2.8a1.4 1.4 0 0 0 0-2.8z',
    },
  };
