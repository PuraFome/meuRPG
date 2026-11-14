import type { TableSpellEffect } from '../../../gen/meurpg/rules/v1/table_content_pb';
import { rangeMeters } from '../units';
import {
  type AuraAffectsChoice,
  type AuraDraft,
  type EffectRow,
  ROLL_WORDS,
  type RollWord,
  effectToDraft,
  feetOf,
  whole,
} from './spell-effect-draft';

// ---- the card's labels: a twin of rules.EffectTagsPT (the server writes them for the players; the editor has no call to ask) ----

const ROLL_NAMES_PT: Record<RollWord, string> = {
  attack: 'ataques',
  save: 'resistências',
  check: 'testes',
};
const ABILITY_TAGS_PT: Record<string, string> = {
  str: 'FOR',
  dex: 'DES',
  con: 'CON',
  int: 'INT',
  wis: 'SAB',
  cha: 'CAR',
};
const RES_VERB_PT = {
  resist: 'Resistência a',
  immune: 'Imunidade a',
  vulnerable: 'Vulnerabilidade a',
} as const;
const AFFECTS_PT: Record<AuraAffectsChoice, string> = {
  self: 'só quem a carrega',
  allies: 'aliados',
  enemies: 'inimigos',
};

function joinPt(words: readonly string[]): string {
  return words.length <= 1
    ? words.join('')
    : `${words.slice(0, -1).join(', ')} e ${words[words.length - 1]}`;
}

function signed(n: number): string {
  return n >= 0 ? `+${n}` : `${n}`;
}

/** "3 m" or "4,5 m": the card's way of writing feet (`rules.MetersPT`). */
export function metersPt(ft: number): string {
  return `${ft > 0 ? rangeMeters(ft) : '0'} m`;
}

function rollTag(r: EffectRow, rolls: string): string {
  if (r.rollMode === 'ability') {
    return `Modificador de ${ABILITY_TAGS_PT[r.valueAbility] ?? r.valueAbility} (mín. ${signed(whole(r.valueMin))}) em ${rolls}`;
  }
  if (r.rollMode === 'die') {
    return `${r.sign < 0 ? '-' : '+'}1d${whole(r.die)} em ${rolls}`;
  }
  return `${signed(whole(r.value))} em ${rolls}`;
}

/** The label of one row, as the server's `EffectTagsPT` writes it, `damageName` naming a damage-type key. Empty while the row is not readable. */
export function rowTags(r: EffectRow, damageName: (key: string) => string): string[] {
  const rolls = joinPt(ROLL_WORDS.filter((w) => r.rolls.includes(w)).map((w) => ROLL_NAMES_PT[w]));
  const abilities =
    r.abilities.length > 0 ? ` (${joinPt(r.abilities.map((a) => ABILITY_TAGS_PT[a] ?? a))})` : '';
  switch (r.kind) {
    case 'ac_bonus':
      return [`CA ${signed(whole(r.value))}`];
    case 'temp_hp':
      return [`PV temporários: ${whole(r.value)}`];
    case 'check_bonus':
      return [`${signed(whole(r.value))} em testes de perícia`];
    case 'roll':
      return [rollTag(r, rolls)];
    case 'resistance': {
      const only = r.damageTypes.length === 1 ? r.damageTypes[0] : '';
      const what =
        only === 'all'
          ? 'todo dano'
          : only === 'nonmagical_bps'
            ? 'dano de armas não mágicas'
            : r.damageTypes.map((k) => damageName(k).toLowerCase()).join(', ');
      return [`${RES_VERB_PT[r.resistance]} ${what}`];
    }
    case 'speed':
      return [
        r.speedMode === 'add'
          ? `Deslocamento +${metersPt(feetOf(r.speedM))}`
          : `Deslocamento ${whole(r.percent)}%`,
      ];
    case 'advantage':
      return ROLL_WORDS.filter((w) => r.rolls.includes(w)).map((w) => {
        const word = w === 'attack' ? 'ataques' : w === 'save' ? 'resistências' : 'testes';
        return `${r.advantage === 'adv' ? 'Vantagem' : 'Desvantagem'} em ${word}${w === 'attack' ? '' : abilities}`;
      });
    default:
      return [];
  }
}

export function auraTag(a: AuraDraft): string {
  return `Aura de ${metersPt(feetOf(a.radiusM))}: ${AFFECTS_PT[a.affects]}`;
}

/** The line of a spell's effect in what a reader sees ("Nos alvos: +1d4 em ataques e resistências, enquanto durar."), with the stored effect's own labels. */
export function effectReadItem(
  e: TableSpellEffect | undefined,
  nameOf: (key: string) => string,
): { readonly title: string; readonly text: string } | undefined {
  if (!e) {
    return undefined;
  }
  const d = effectToDraft(e);
  const parts = [
    ...(d.aura ? [auraTag(d.aura)] : []),
    ...d.conditions.map(nameOf),
    ...d.rows.flatMap((r) => rowTags(r, nameOf)),
  ];
  if (parts.length === 0) {
    return undefined;
  }
  const title = d.aura
    ? 'Em volta de quem conjura'
    : d.applies === 'failed_save'
      ? 'Em quem falhar no teste'
      : d.applies === 'hit'
        ? 'Em quem o ataque acertar'
        : 'Nos alvos';
  return { title, text: `${parts.join(', ')}, enquanto durar.` };
}
