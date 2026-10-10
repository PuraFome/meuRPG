import { spellLevelLabel } from '../../core/characters/character-labels';
import { AbilityKey, CharacterState } from '../../core/characters/characters.types';
import {
  CoinsVm,
  IssueVm,
  PactSlotsVm,
  ReviewVm,
  SheetSpellVm,
  SpellcastingVm,
} from './character-sheet.types';

/**
 * Display helpers of the sheet page only (the "Ficha de papel" layout,
 * docs/design.md). Every one of them formats a number or a stable code the
 * server sent; none computes a rule (ADR-0008) or parses a display string.
 */

/** The official sheet's three-letter ability abbreviations, shown next to
 * each skill (For, Des, Con, Int, Sab, Car). */
export const ABILITY_ABBREVIATIONS: Record<AbilityKey, string> = {
  str: 'For',
  dex: 'Des',
  con: 'Con',
  int: 'Int',
  wis: 'Sab',
  cha: 'Car',
};

/** "29/09/2026", local time: the header's lock and death lines show the day
 * only. */
/** Joins a number and the unit after it ("10 PO", "15 m", "2 kg") with a no-break space, so a line never
 * ends between them. Display only: the stored text is untouched. */
export function keepUnitsTogether(text: string): string {
  return text.replace(/(\d)\s+(PO|PP|PE|PL|PC|m|kg|pés)(?![\p{L}])/gu, '$1\u00a0$2');
}

export function formatDate(date: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${pad(date.getDate())}/${pad(date.getMonth() + 1)}/${date.getFullYear()}`;
}

const SHORT_MONTHS = [
  'jan.',
  'fev.',
  'mar.',
  'abr.',
  'mai.',
  'jun.',
  'jul.',
  'ago.',
  'set.',
  'out.',
  'nov.',
  'dez.',
];

/** "8 de out., 21h10", local time: when the master asked for changes, or the player sent the sheet again. */
export function formatWhen(date: Date): string {
  const minutes = String(date.getMinutes()).padStart(2, '0');
  return `${date.getDate()} de ${SHORT_MONTHS[date.getMonth()]}, ${date.getHours()}h${minutes}`;
}

/** The one-word state tag (docs/design.md, StatusTag): "Pendente", not the
 * lifecycle's full "Pendente de aprovação", which the notice under the
 * header spells out. */
export function stateTagLabel(state: CharacterState): string {
  switch (state) {
    case 'draft':
      return 'Rascunho';
    case 'pending':
      return 'Pendente';
    case 'locked':
      return 'Travada';
    case 'dead':
      return 'Morto';
  }
}

/** The bold first phrase of a rules issue in the notice under the header:
 * what kind of problem it is, from the issue's stable `code`
 * (`rules.proto`'s `Issue.code`, lower_snake_case) and, for the armour, its
 * `field` path. The server's `message` follows it and says the details. */
export function issueTitle(issue: IssueVm): string {
  switch (issue.code) {
    case 'armor_proficiency':
      return issue.field === 'full.shield'
        ? 'Escudo sem proficiência.'
        : 'Armadura sem proficiência.';
    case 'spell_not_on_list':
      return issue.field.startsWith('full.prepared_spell_keys')
        ? 'Magia fora do grimório.'
        : 'Magia fora da lista.';
    case 'spell_count':
    case 'spell_level':
      return 'Magias a revisar.';
    case 'skill_count':
      return 'Perícias a revisar.';
    case 'expertise':
      return 'Especialização a revisar.';
    case 'missing':
      return 'Falta uma escolha.';
    case 'unknown_key':
      return 'Escolha inválida.';
    case 'subclass_level':
      return 'Subclasse cedo demais.';
    case 'multiclass_prerequisite':
      return 'Pré-requisito de multiclasse.';
    case 'hit_point_rolls':
      return 'Rolagens de pontos de vida.';
    case 'score_above_20':
      return 'Habilidade acima de 20.';
    case 'level':
      return 'Nível acima do máximo.';
    case 'formula':
      return 'Efeito ignorado.';
    default:
      return 'Pendência de regra.';
  }
}

export interface SpellSlotRow {
  readonly level: number;
  /** "1º nível". */
  readonly label: string;
  readonly count: number;
  /** "4 espaços", the circles' accessible name. */
  readonly countLabel: string;
}

/** One row per spell level with at least one slot, in order (index 0 of
 * `FullSheetVm.spellSlots` is level 1). */
export function spellSlotRows(slots: readonly number[]): SpellSlotRow[] {
  return slots
    .map((count, i) => ({
      level: i + 1,
      label: spellLevelLabel(i + 1),
      count,
      countLabel: count === 1 ? '1 espaço' : `${count} espaços`,
    }))
    .filter((row) => row.count > 0);
}

/** The row of a Warlock's Pact Magic slots: "Espaços do pacto", all of one level; null without any. */
export function pactSlotRow(pact: PactSlotsVm | null): SpellSlotRow | null {
  return pact && pact.count > 0
    ? {
        level: pact.level,
        label: spellLevelLabel(pact.level),
        count: pact.count,
        countLabel: pact.count === 1 ? '1 espaço' : `${pact.count} espaços`,
      }
    : null;
}

/** "Até 3 truques e 7 magias preparadas." (or "... 5 magias conhecidas." for a
 * class that knows its spells), from the class's own limits; `''` when the
 * class has none. */
export function spellLimitsText(sc: SpellcastingVm): string {
  const parts: string[] = [];
  if (sc.cantripsKnown > 0) {
    parts.push(sc.cantripsKnown === 1 ? '1 truque' : `${sc.cantripsKnown} truques`);
  }
  if (sc.spellsPreparedMax > 0) {
    parts.push(
      sc.spellsPreparedMax === 1
        ? '1 magia preparada'
        : `${sc.spellsPreparedMax} magias preparadas`,
    );
  }
  if (sc.spellsKnownMax > 0) {
    parts.push(
      sc.spellsKnownMax === 1 ? '1 magia conhecida' : `${sc.spellsKnownMax} magias conhecidas`,
    );
  }
  return parts.length === 0 ? '' : `Até ${parts.join(' e ')}.`;
}

export interface CoinEntry {
  readonly amount: number;
  /** "PO". */
  readonly abbreviation: string;
  /** "peças de ouro", for the abbreviation's `title`. */
  readonly name: string;
}

/** The coins the character carries, platinum first, skipping zeros; empty
 * when there are none ("Sem moedas"). */
export function coinEntries(coins: CoinsVm): CoinEntry[] {
  const all: CoinEntry[] = [
    { amount: coins.pp, abbreviation: 'PL', name: 'peças de platina' },
    { amount: coins.gp, abbreviation: 'PO', name: 'peças de ouro' },
    { amount: coins.ep, abbreviation: 'PE', name: 'peças de electro' },
    { amount: coins.sp, abbreviation: 'PP', name: 'peças de prata' },
    { amount: coins.cp, abbreviation: 'PC', name: 'peças de cobre' },
  ];
  return all.filter((c) => c.amount > 0);
}

/** The tag of a pending character's header, with the master's review in it: text and icon, never only colour. */
export function pendingTag(review: ReviewVm | null): { label: string; icon: string } {
  switch (review?.status) {
    case 'changes_requested':
      return { label: 'Pendente · ajustes pedidos', icon: 'edit' };
    case 'resubmitted':
      return { label: 'Pendente · reenviado', icon: 'task_alt' };
    default:
      return { label: 'Pendente', icon: 'schedule' };
  }
}

/** One spell of the sheet's list, with what its row says besides the name. */
export interface SpellRow {
  readonly spell: SheetSpellVm;
  /** "Ritual", "Concentração", "Reação", from the spell's own data, in that order. */
  readonly tags: readonly string[];
  /**
   * Where the spell stands for a class that prepares: "Preparada" (can be cast today), "No grimório"
   * (a Wizard's spellbook spell not prepared) or "Conhecida" (any other not prepared). `''` for a
   * cantrip and for a class that knows its spells: they are all castable, so the row says nothing.
   */
  readonly state: '' | 'Preparada' | 'No grimório' | 'Conhecida';
}

/** The spells of one level: "Truques", "1º nível"... */
export interface SpellGroup {
  readonly level: number;
  readonly label: string;
  readonly rows: readonly SpellRow[];
}

function spellTags(s: SheetSpellVm): string[] {
  const tags: string[] = [];
  if (s.ritual) {
    tags.push('Ritual');
  }
  if (s.concentration) {
    tags.push('Concentração');
  }
  if (s.reaction) {
    tags.push('Reação');
  }
  return tags;
}

/**
 * The sheet's spells grouped by level, cantrips first. With a class that prepares (`spellsPreparedMax`),
 * every spell above a cantrip says whether it is prepared; a class that knows its spells says nothing
 * (all are castable). The order inside a level is the server's.
 */
export function spellGroups(
  spells: readonly SheetSpellVm[],
  casters: readonly SpellcastingVm[],
): SpellGroup[] {
  const prepares = casters.some((c) => c.spellsPreparedMax > 0);
  const book = casters.some((c) => c.spellbook);
  const byLevel = new Map<number, SpellRow[]>();
  for (const spell of spells) {
    const state =
      !prepares || spell.level === 0
        ? ''
        : spell.prepared
          ? 'Preparada'
          : book
            ? 'No grimório'
            : 'Conhecida';
    const rows = byLevel.get(spell.level) ?? [];
    rows.push({ spell, tags: spellTags(spell), state });
    byLevel.set(spell.level, rows);
  }
  return [...byLevel.keys()]
    .sort((a, b) => a - b)
    .map((level) => ({
      level,
      label: level === 0 ? 'Truques' : spellLevelLabel(level),
      rows: byLevel.get(level) ?? [],
    }));
}

/** "7 preparadas e 3 no grimório." for a class that prepares; `''` for one that knows its spells. */
export function spellStateSummary(groups: readonly SpellGroup[]): string {
  const rows = groups.flatMap((g) => g.rows);
  const prepared = rows.filter((r) => r.state === 'Preparada').length;
  const book = rows.filter((r) => r.state === 'No grimório').length;
  const known = rows.filter((r) => r.state === 'Conhecida').length;
  const parts: string[] = [];
  if (prepared + book + known > 0) {
    parts.push(prepared === 1 ? '1 preparada' : `${prepared} preparadas`);
  }
  if (book > 0) {
    parts.push(book === 1 ? '1 só no grimório' : `${book} só no grimório`);
  }
  if (known > 0) {
    parts.push(known === 1 ? '1 só conhecida' : `${known} só conhecidas`);
  }
  return parts.length === 0 ? '' : `${parts.join(' e ')}.`;
}
