import type { Creature, CreatureDamageModifier } from '../../../gen/meurpg/rules/v1/rules_pb';
import { signed } from '../../core/creatures/creature-format';
import { formatInt, tight } from '../../core/format/text';
import { metersText } from '../../core/units';

/** The numbers of "PV": the creature's own (the owner's and the master's), or the book's. */
export interface StatBlockHp {
  readonly current: number;
  readonly max: number;
}

export interface Tile {
  readonly label: string;
  readonly value: string;
  readonly of?: string;
  /** A small line under the value: "gibão de peles", "7d10 + 21", "40 pés", "450 XP" (the bestiary's page). */
  readonly note?: string;
}

export interface LinePart {
  readonly text: string;
  readonly en?: boolean;
}

export interface Line {
  readonly term: string;
  readonly parts: LinePart[];
}

export interface Group {
  readonly title: string;
  readonly entries: readonly { name: string; text: string; usage: string }[];
}

/** The hit points tile: the creature's own, or the book's (the average, with the dice written out on the bestiary's page). */
function hpTile(c: Creature, hp: StatBlockHp | null, full: boolean): Tile {
  if (hp) {
    return { label: 'PV', value: String(hp.current), of: `de ${hp.max}` };
  }
  if (full) {
    return {
      label: 'PV',
      value: String(c.hitPoints),
      note: c.hitPointsRoll.replace(/([+-])/g, '\u00a0$1\u00a0'),
    };
  }
  return {
    label: 'PV',
    value: String(c.hitPoints),
    of: c.hitPointsRoll ? `(${c.hitPointsRoll})` : '',
  };
}

/** One tile per speed the creature has; a creature with none still shows its walking speed. */
function speedTiles(c: Creature, full: boolean): Tile[] {
  const speeds: [number, string][] = [
    [c.speedWalkFt, 'Deslocamento'],
    [c.speedFlyFt, 'Voo'],
    [c.speedSwimFt, 'Natação'],
    [c.speedClimbFt, 'Escalada'],
    [c.speedBurrowFt, 'Escavação'],
  ];
  const anySpeed = speeds.some(([ft]) => ft > 0);
  return speeds
    .filter(([ft, label]) => ft > 0 || (!anySpeed && label === 'Deslocamento'))
    .map(([ft, label]) => ({
      label,
      value: tight(metersText(ft)),
      note: full && ft > 0 ? `${ft}\u00a0pés` : undefined,
    }));
}

export function tilesOf(c: Creature, hp: StatBlockHp | null, full: boolean): readonly Tile[] {
  const out: Tile[] = [
    {
      label: 'CA',
      value: String(c.armorClass),
      note: full ? c.armorClassNote || c.armorClassLabelPt : undefined,
    },
    hpTile(c, hp, full),
    ...speedTiles(c, full),
  ];
  if (full) {
    out.push({
      label: 'Nível de desafio',
      value: c.summary?.challengeRating ?? '0',
      note: `${formatInt(c.summary?.xp ?? 0)} XP`,
    });
  }
  return out;
}

const bonusText = (list: Creature['savingThrows']) =>
  list.map((b) => `${b.namePt} ${signed(b.bonus)}`).join(', ');

/** The lines that exist only when the creature has something to say (saves, skills, vulnerabilities...). */
function optionalLines(c: Creature): Line[] {
  const out: Line[] = [];
  const line = (term: string, text: string) => out.push({ term, parts: [{ text, en: false }] });
  if (c.savingThrows.length > 0) {
    line('Testes de resistência', bonusText(c.savingThrows));
  }
  if (c.skills.length > 0) {
    line('Perícias', bonusText(c.skills));
  }
  const modifiers: [string, readonly CreatureDamageModifier[]][] = [
    ['Vulnerável a', c.vulnerabilities],
    ['Resistente a', c.resistances],
    ['Imune a', c.immunities],
  ];
  for (const [term, list] of modifiers) {
    if (list.length > 0) {
      // The damage types are ours, in Portuguese; the book's note ("from nonmagical weapons") stays English.
      out.push({ term, parts: list.flatMap((m, i) => modifierParts(m, i > 0)) });
    }
  }
  if (c.conditionImmunities.length > 0) {
    line('Condições', `imune a ${c.conditionImmunities.map((k) => k.namePt).join(', ')}`);
  }
  return out;
}

export function linesOf(c: Creature, full: boolean): readonly Line[] {
  const line = (term: string, text: string, en = false): Line => ({ term, parts: [{ text, en }] });
  const senses = c.senses.map((s) => tight(`${s.namePt} ${metersText(s.rangeFt)}`));
  return [
    ...optionalLines(c),
    line('Sentidos', [...senses, `Percepção passiva ${c.passivePerception}`].join(', ')),
    line('Idiomas', c.languages || '—', c.languages !== ''),
    full
      ? line('Bônus de proficiência', signed(c.proficiencyBonus))
      : line('Desafio', c.summary?.challengeRating ?? '0'),
  ];
}

export function groupsOf(c: Creature): readonly Group[] {
  const groups: [string, Group['entries']][] = [
    ['Características', c.traits],
    ['Ações', c.actions],
    ['Reações', c.reactions],
    ['Ações lendárias', c.legendaryActions],
  ];
  return groups
    .filter(([, entries]) => entries.length > 0)
    .map(([title, entries]) => ({ title, entries }));
}

/** "Fogo, Gelo" (ours) plus the book's note ("from nonmagical weapons", in English), after "; " when it is not the first. */
function modifierParts(m: CreatureDamageModifier, more: boolean): { text: string; en?: boolean }[] {
  const types = m.types.map((t) => t.namePt).join(', ');
  const out: { text: string; en?: boolean }[] = [];
  if (types) {
    out.push({ text: `${more ? '; ' : ''}${types}${m.note ? ' ' : ''}` });
  }
  if (m.note) {
    out.push({ text: `${types || !more ? '' : '; '}${m.note}`, en: true });
  }
  return out;
}
