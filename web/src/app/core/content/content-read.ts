import type { TableClass, TableClassLevel, TableEffect, TableEntry, TableFeature, TableRace } from '../../../gen/meurpg/rules/v1/table_content_pb';
import { joinDots, tight } from '../format/text';
import { feetToMeters, formatMeters } from '../units';
import { SIZE_WORDS } from './content-kinds';
import { ABILITY_FIELDS, type Bonuses, bonusText, noBonuses } from './feature-draft';
import type { Ability } from '../../../gen/meurpg/rules/v1/rules_pb';
import type { CatalogAbility } from './catalog';
import { previewRows, spellToDraft } from './spell-draft';
import { slotsText } from './class-draft';

/**
 * What a player reads of an entry (MR-025, question 83: "every option that is on, in full, with the numbers and the
 * effects"), and what the master reads where the app has no editor yet (classes and subclasses, 10.12). Pure: the page
 * hands over the entry and a way to name a key, and gets rows and sections of plain Portuguese back. Nothing here changes
 * what the server sent; a field the entry does not carry is not drawn.
 */

export interface ReadRow {
  readonly label: string;
  readonly value: string;
}

export interface ReadItem {
  readonly title: string;
  readonly text: string;
}

export interface ReadSection {
  readonly title: string;
  readonly items: readonly ReadItem[];
}

export interface EntryRead {
  readonly rows: readonly ReadRow[];
  readonly sections: readonly ReadSection[];
  /** Text of the entry itself (a spell's description, a subclass's). */
  readonly text: readonly string[];
}

const RECHARGE_WORDS: Readonly<Record<string, string>> = {
  short_rest: 'voltam num descanso curto',
  long_rest: 'voltam num descanso longo',
  dawn: 'voltam ao amanhecer',
  none: 'não voltam sozinhos',
};

const SENSE_WORDS: Readonly<Record<string, string>> = {
  darkvision: 'Visão no escuro',
  blindsight: 'Visão às cegas',
  tremorsense: 'Sentido sísmico',
  truesight: 'Visão verdadeira',
};

export type NameOf = (key: string) => string;

/** The way of casting as a row says it: "Metade · Sabedoria · preparadas". */
const READ_CASTING: Readonly<Record<string, string>> = { full: 'Completa', half: 'Metade', pact: 'Pacto', third: 'Um terço' };

/** A sentence ends with a stop: "Enxergam longe" becomes "Enxergam longe." (a text that already ends with one is left). */
export function ensureStop(text: string): string {
  const t = text.trim();
  return t === '' || /[.!?…:;]$/.test(t) ? t : `${t}.`;
}

/** One effect in a line, for the ones the player can read without the master's menu; '' for the others (their text carries them). */
export function effectLine(e: TableEffect, nameOf: NameOf): string {
  switch (e.type) {
    case 'proficiency': {
      const what = e.proficiency.startsWith('skill:') && e.proficiency !== 'skill:*' ? nameOf(e.proficiency) : '';
      return what ? `Proficiência em ${what}` : '';
    }
    case 'sense':
      return e.rangeFt > 0 ? tight(`${SENSE_WORDS[e.sense] ?? e.sense} ${formatMeters(feetToMeters(e.rangeFt))}`) : '';
    case 'resource':
      // Only a plain number says well in a line; a formula (prof()) is left to the feature's own text.
      return /^\d+$/.test(e.max) ? `${e.max} ${e.max === '1' ? 'uso' : 'usos'}, ${RECHARGE_WORDS[e.recharge] ?? ''}`.replace(/, $/, '') : '';
    case 'extra_attack':
      return e.count > 0 ? `Ataque extra: ${e.count} ataques` : '';
    default:
      return '';
  }
}

/** A feature as the player reads it: its text, and the effects it has that a line can say. */
export function featureItem(f: TableFeature, nameOf: NameOf, prefix = ''): ReadItem {
  const said = f.descPt.join(' ').toLowerCase();
  // An effect's line is added only when the trait's own text does not already say it.
  const lines = f.effects.map((e) => effectLine(e, nameOf)).filter((l) => l !== '' && !said.includes(l.toLowerCase()));
  const text = [...f.descPt, ...lines].map(ensureStop).join(' ');
  return { title: `${prefix}${f.namePt}`, text };
}

function bonusesOf(b: TableRace['abilityBonuses']): Bonuses {
  const out = noBonuses();
  for (const a of ABILITY_FIELDS) {
    out[a] = b?.[a] ?? 0;
  }
  return out;
}

/** The abilities with their names, from the catalog (`ability:strength`...). */
function abilitiesOf(nameOf: NameOf): CatalogAbility[] {
  return ABILITY_FIELDS.map((field, i) => ({ field, ability: (i + 1) as Ability, name: nameOf(`ability:${field}`) }));
}

function metres(ft: number): string {
  return formatMeters(feetToMeters(ft));
}

function names(keys: readonly string[], nameOf: NameOf): string {
  return keys.map(nameOf).join(', ');
}

/** "Metade · Sabedoria · preparadas" (and "· rituais"): how a class or a third caster's subclass casts. */
function castingLine(c: NonNullable<TableClass['casting']>, abilityName: (a: number) => string): string {
  return joinDots(
    [READ_CASTING[c.kind] ?? c.kind, abilityName(c.ability), c.preparation === 'prepared' ? 'preparadas' : 'conhecidas', c.ritual ? 'rituais' : ''].filter((x) => x !== ''),
  );
}

/** One row of a class's table in a line: "+3 · Ataque extra, Vigília · 2 truques · 4 de 1º, 2 de 2º". What the server stored, in words. */
function levelLine(lv: TableClassLevel, pact: boolean, marks: readonly string[] = []): string {
  const slots = slotsText({ profBonus: lv.profBonus, cantrips: lv.cantripsKnown, spells: lv.spellsKnown, slots: [...lv.slots] }, pact);
  return [
    lv.profBonus > 0 ? `+${lv.profBonus}` : '',
    [...lv.features.map((f) => f.namePt), ...marks].join(', '),
    lv.cantripsKnown > 0 ? `${lv.cantripsKnown} ${lv.cantripsKnown === 1 ? 'truque' : 'truques'}` : '',
    lv.spellsKnown > 0 ? `${lv.spellsKnown} ${lv.spellsKnown === 1 ? 'magia conhecida' : 'magias conhecidas'}` : '',
    slots,
  ]
    .filter((x) => x !== '')
    .join(' · ');
}

export function readEntry(entry: TableEntry, nameOf: NameOf, subclassLevelOf: (classKey: string) => number = () => 0): EntryRead {
  const abilityName = (a: number): string => nameOf(`ability:${a}`);
  const rows: ReadRow[] = [];
  const sections: ReadSection[] = [];
  let text: readonly string[] = [];
  switch (entry.body.case) {
    case 'tableRace': {
      const r = entry.body.value;
      const choice = r.choiceBonuses.length > 0 ? `${r.choiceBonuses.map((n) => `+${n}`).join(' e ')} à escolha` : '';
      const fixed = bonusText(bonusesOf(r.abilityBonuses), abilitiesOf(nameOf));
      rows.push({ label: 'Habilidades', value: [fixed === 'Nenhum' ? '' : fixed, choice].filter((x) => x).join(', ') || 'Nenhum' });
      rows.push({ label: 'Tamanho', value: SIZE_WORDS[r.size] ?? r.size });
      rows.push({ label: 'Deslocamento', value: metres(r.speedFt) });
      if (r.darkvisionFt > 0) rows.push({ label: 'Visão no escuro', value: metres(r.darkvisionFt) });
      const langs = [names(r.languages, nameOf), r.languageChoices > 0 ? `${r.languageChoices} à escolha` : ''].filter((x) => x).join(', ');
      if (langs) rows.push({ label: 'Idiomas', value: langs });
      if (r.traits.length > 0) sections.push({ title: 'Traços', items: r.traits.map((t) => featureItem(t, nameOf)) });
      break;
    }
    case 'tableSubrace': {
      const s = entry.body.value;
      if (s.raceKey) rows.push({ label: 'Sub-raça de', value: nameOf(s.raceKey) });
      const fixed = bonusText(bonusesOf(s.abilityBonuses), abilitiesOf(nameOf));
      rows.push({ label: 'Habilidades', value: fixed });
      if (s.traits.length > 0) sections.push({ title: 'Traços', items: s.traits.map((t) => featureItem(t, nameOf)) });
      break;
    }
    case 'tableBackground': {
      const b = entry.body.value;
      rows.push({ label: 'Perícias', value: names(b.skills, nameOf) || '—' });
      if (b.tools.length > 0) rows.push({ label: 'Ferramentas', value: names(b.tools, nameOf) });
      if (b.languageChoices > 0) rows.push({ label: 'Idiomas à escolha', value: String(b.languageChoices) });
      if (b.equipmentPt) rows.push({ label: 'Equipamento', value: b.equipmentPt });
      if (b.feature && b.feature.namePt) sections.push({ title: 'Característica', items: [featureItem(b.feature, nameOf)] });
      break;
    }
    case 'tableSpell': {
      const s = entry.body.value;
      const d = spellToDraft(s);
      for (const row of previewRows(d, nameOf)) rows.push(row);
      const classes = names(s.classKeys, nameOf);
      if (classes) rows.push({ label: 'Classes', value: classes });
      text = [...s.descPt, ...s.higherLevelPt];
      break;
    }
    case 'tableClass': {
      const c = entry.body.value;
      rows.push({ label: 'Dado de vida', value: `d${c.hitDie}` });
      rows.push({ label: 'Testes de resistência', value: c.savingThrows.map(abilityName).join(', ') });
      if (c.skillChoose > 0) rows.push({ label: 'Perícias', value: `${c.skillChoose} de ${c.skillFrom.length}: ${names(c.skillFrom, nameOf)}` });
      const armor = c.proficiencies.filter((k) => k.includes('armor') || k.includes('shield')).map(nameOf);
      const weapons = c.proficiencies.filter((k) => k.includes('weapon')).map(nameOf);
      const other = c.proficiencies.filter((k) => !k.includes('armor') && !k.includes('shield') && !k.includes('weapon')).map(nameOf);
      if (armor.length > 0) rows.push({ label: 'Armaduras', value: armor.join(', ') });
      if (weapons.length > 0) rows.push({ label: 'Armas', value: weapons.join(', ') });
      if (other.length > 0) rows.push({ label: 'Outras proficiências', value: other.join(', ') });
      if (c.casting) {
        rows.push({ label: 'Conjuração', value: castingLine(c.casting, abilityName) });
        rows.push({ label: 'Lista de magias', value: c.casting.listFrom ? `A lista do ${nameOf(c.casting.listFrom)}` : 'A própria lista da classe' });
      }
      rows.push({ label: 'Escolhe a subclasse', value: `no nível ${c.subclassLevel || 3}` });
      const need = (m: TableClass['minimums']): string =>
        ABILITY_FIELDS.filter((a) => (m?.[a] ?? 0) > 0).map((a) => `${nameOf(`ability:${a}`)} ${m?.[a]}`).join(', ');
      const all = need(c.minimums);
      const any = need(c.anyOf);
      if (all) rows.push({ label: 'Para multiclasse', value: all });
      if (any) rows.push({ label: 'Para multiclasse (uma delas)', value: any });
      const items: ReadItem[] = [];
      c.levels.forEach((lv, i) => lv.features.forEach((f) => items.push(featureItem(f, nameOf, `Nível ${i + 1} · `))));
      if (items.length > 0) sections.push({ title: 'Características', items });
      // The table in words, level by level (the numbers the server stored; a 0 proficiency bonus is the SRD's and is not written).
      if (c.levels.length > 0) {
        const choose = c.subclassLevel || 3;
        sections.push({
          title: 'Tabela dos níveis',
          items: c.levels.map((lv, i) => {
            const marks = [c.asiLevels.includes(i + 1) ? 'Incremento no Valor de Habilidade' : '', choose === i + 1 ? 'Escolha de subclasse' : ''].filter((x) => x !== '');
            return { title: `Nível ${i + 1}`, text: levelLine(lv, c.casting?.kind === 'pact', marks) };
          }),
        });
      }
      break;
    }
    case 'tableSubclass': {
      const s = entry.body.value;
      rows.push({ label: 'Subclasse de', value: nameOf(s.classKey) });
      const chosen = s.level || subclassLevelOf(s.classKey);
      if (chosen > 0) rows.push({ label: 'Escolhida no nível', value: String(chosen) });
      if (s.casting) {
        rows.push({ label: 'Conjuração', value: castingLine(s.casting, abilityName) });
        if (s.casting.listFrom) rows.push({ label: 'Lista de magias', value: `A lista do ${nameOf(s.casting.listFrom)}` });
      }
      text = s.descPt;
      const items: ReadItem[] = [];
      s.levels.forEach((lv) => lv.features.forEach((f) => items.push(featureItem(f, nameOf, `Nível ${lv.level} · `))));
      if (items.length > 0) sections.push({ title: 'Características', items });
      if (s.casting) {
        const from = s.casting.startLevel > 0 ? s.casting.startLevel : 3;
        const table = s.levels.filter((lv) => lv.level >= from).map((lv) => ({ title: `Nível ${lv.level}`, text: levelLine({ profBonus: 0, cantripsKnown: lv.cantripsKnown, spellsKnown: lv.spellsKnown, slots: lv.slots, features: [] } as unknown as TableClassLevel, false) }));
        if (table.length > 0) sections.push({ title: 'Conjuração por nível', items: table });
      }
      if (s.alwaysPrepared.length > 0) {
        sections.push({
          title: 'Sempre preparadas',
          items: s.alwaysPrepared.map((a) => ({ title: `Nível ${a.classLevel}`, text: nameOf(a.spellKey) })),
        });
      }
      break;
    }
    default:
      break;
  }
  return { rows, sections, text };
}
