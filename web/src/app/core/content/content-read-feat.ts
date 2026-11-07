import type { TableEffect, TableFeat } from '../../../gen/meurpg/rules/v1/table_content_pb';
import { type NameOf, type ReadItem, type ReadRow, featureItem } from './content-read-parts';
import { optionSetName } from './option-text';

/** What the player's pick is called after its count, by the kind of the choice ("1 habilidade", "2 tipos de dano"). */
const KIND_WORDS: Readonly<Record<string, readonly [string, string]>> = {
  ability: ['habilidade', 'habilidades'],
  damage_type: ['tipo de dano', 'tipos de dano'],
};

/** How much a choice of options asks: "1 de 4 opções" for the feat's own or marked ones, "1 estilo de luta" for a whole set. */
function optionsAsked(e: TableEffect, own: number): string {
  if (e.set !== '') {
    return `${e.count} ${optionSetName(e.set).toLowerCase()}`;
  }
  const offered = e.from.length > 0 ? e.from.length : own;
  return offered > 0 ? `${e.count} de ${offered} ${offered === 1 ? 'opção' : 'opções'}` : '';
}

/** The "Escolha" row of each choice of the feat that the player makes when taking it. */
export function featChoiceRows(f: TableFeat): ReadRow[] {
  return f.effects.flatMap((e): ReadRow[] => {
    if (e.type !== 'choice') {
      return [];
    }
    const words = KIND_WORDS[e.choice];
    const value = words
      ? `${e.count} ${e.count === 1 ? words[0] : words[1]}`
      : e.choice === 'feature'
        ? optionsAsked(e, f.options.length)
        : '';
    return value === '' ? [] : [{ label: 'Escolha', value }];
  });
}

/** The options the feat lists of its own, each with its text and what its effects say. */
export function featOptionItems(f: TableFeat, nameOf: NameOf): ReadItem[] {
  return f.options.filter((o) => o.namePt !== '').map((o) => featureItem(o, nameOf));
}
