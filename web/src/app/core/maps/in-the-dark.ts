import type {
  ListCharactersInTheDarkResponse,
  MapToken,
} from '../../../gen/meurpg/maps/v1/maps_pb';
import { lightKeyName } from './carried-light';

/** A player character that sees nothing where it stands (no light, no darkvision), as the master's read names it. */
export interface DarkCharacter {
  readonly characterId: string;
  readonly name: string;
  readonly col: number;
  readonly row: number;
}

export function darkCharacters(res: ListCharactersInTheDarkResponse): DarkCharacter[] {
  return res.characters.map((c) => ({
    characterId: c.characterId,
    name: c.characterName,
    col: c.col,
    row: c.row,
  }));
}

/** The amber notice of the master's session map card: the lead word, the sentence and, for several, the cause (B3-06 states 2 and 3). */
export interface DarkNotice {
  readonly many: boolean;
  readonly lead: string;
  readonly text: string;
  readonly detail: string;
}

const LEAD = 'No escuro:';

/** The notice for who is in the dark; `null` for nobody, because an empty warning never appears. */
export function darkNotice(list: readonly DarkCharacter[]): DarkNotice | null {
  if (list.length === 0) {
    return null;
  }
  if (list.length === 1) {
    return {
      many: false,
      lead: LEAD,
      text: `${list[0].name} não enxerga onde está (sem luz e sem visão no escuro).`,
      detail: '',
    };
  }
  return {
    many: true,
    lead: LEAD,
    text: `${list.length} personagens não enxergam onde estão.`,
    detail: 'Sem luz e sem visão no escuro.',
  };
}

/** The one-line form for the map page's painting mode: "No escuro: Brisa, Jabuti."; empty for nobody. */
export function darkLine(list: readonly DarkCharacter[]): string {
  return list.length === 0 ? '' : `${LEAD} ${darkNames(list)}`;
}

/** "Brisa, Jabuti.": the names the one-line form says after its lead word. */
export function darkNames(list: readonly DarkCharacter[]): string {
  return `${list.map((c) => c.name).join(', ')}.`;
}

/** What the page knows at one moment: the light each player character carries, and who was in the dark. */
export interface DarkSnapshot {
  readonly lights: ReadonlyMap<string, { readonly name: string; readonly key: string }>;
  readonly dark: ReadonlySet<string>;
}

export function darkSnapshot(
  tokens: readonly MapToken[],
  dark: readonly DarkCharacter[],
): DarkSnapshot {
  return {
    lights: new Map(
      tokens
        .filter((t) => !t.creatureId)
        .map((t) => [t.characterId, { name: t.name, key: t.carriedLight }] as const),
    ),
    dark: new Set(dark.map((c) => c.characterId)),
  };
}

/** The name of a light inside a sentence: "tocha", "lanterna coberta", and "luz" for one with no name of its own. */
function inSentence(key: string): string {
  const name = lightKeyName(key).toLocaleLowerCase('pt-BR');
  return name === 'uma luz' ? 'luz' : name;
}

/**
 * The status messages between two reads (B3-06 states 8 and 11): a carried light that went out ("A tocha de bilbo
 * apagou. A luz de bilbo ficou “Nenhuma”."), and a character who lit one and left the dark ("Brisa acendeu a tocha.
 * Não está mais no escuro."). Nothing to compare with (`before` null) says nothing.
 */
export function darkAnnouncements(before: DarkSnapshot | null, after: DarkSnapshot): string[] {
  if (before === null) {
    return [];
  }
  const said: string[] = [];
  for (const [id, was] of before.lights) {
    const now = after.lights.get(id);
    if (!now) {
      continue;
    }
    if (was.key !== '' && now.key === '') {
      said.push(
        `A ${inSentence(was.key)} de ${was.name} apagou. A luz de ${was.name} ficou “Nenhuma”.`,
      );
    } else if (before.dark.has(id) && !after.dark.has(id) && now.key !== '') {
      said.push(`${now.name} acendeu a ${inSentence(now.key)}. Não está mais no escuro.`);
    }
  }
  return said;
}
