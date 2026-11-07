import type {
  TableContentViolation,
  TableEntry,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import { EFFECT_FIELD_LABELS } from './effect-field-copy';
import { KIND_NOUNS, KIND_WORDS, navOfKind } from './content-kinds';
import { violationText } from './content-violations';

/**
 * The entries the rules refuse (issue "left out entries"): the server builds the campaign's content without them
 * and tells the master which and why (`ListTableEntriesResponse.left_out`, first violation of each). This turns that
 * into what the list, the entry page and the campaign page say: a readable place ("Nível 1 · Vigília · efeito 1"),
 * the field in the editor's words ("O que muda"), the sentence of the violation, and what is out only because of it.
 * Pure functions, no screen: the rows are tested without one.
 */

/** Where in the entry a violation is, in the editor's words. */
export interface PathPlace {
  /** The parts of the place: "Nível 1", "Vigília", "efeito 1". Empty for the entry as a whole. */
  readonly parts: readonly string[];
  /** The label of the field the editor draws ("O que muda"), or `''` when the path names no field with a label. */
  readonly field: string;
}

/** The labels of the fields an entry has that are not an effect's (an effect's are `EFFECT_FIELD_LABELS`). */
const FIELD_LABELS: Readonly<Record<string, string>> = {
  name_pt: 'Nome',
  desc_pt: 'Descrição',
  hit_die: 'Dado de vida',
  saving_throws: 'Testes de resistência',
  skill_choose: 'Quantas perícias',
  skill_from: 'Perícias da lista',
  proficiencies: 'Proficiências',
  multiclass_proficiencies: 'Proficiências de multiclasse',
  multiclass_skill_choose: 'Perícias de multiclasse',
  minimums: 'Pré-requisito de multiclasse',
  any_of: 'Pré-requisito de multiclasse',
  subclass_level: 'Nível da subclasse',
  casting: 'Conjuração',
  list_from: 'Lista de magias',
  slots: 'Espaços de magia',
  cantrips_known: 'Truques conhecidos',
  spells_known: 'Magias conhecidas',
  prof_bonus: 'Bônus de proficiência',
  class_key: 'Classe',
  race_key: 'Raça',
  level: 'Nível',
  size: 'Tamanho',
  speed_ft: 'Deslocamento',
  darkvision_ft: 'Visão no escuro',
  ability_bonuses: 'Bônus de habilidade',
  choice_bonuses: 'Bônus à escolha',
  languages: 'Idiomas',
  language_choices: 'Idiomas à escolha',
  replaces_trait_keys: 'Traços que substitui',
  skills: 'Perícias',
  tools: 'Ferramentas',
  always_prepared: 'Magias sempre preparadas',
  spell_key: 'Magia',
  school_key: 'Escola',
  class_keys: 'Classes',
  prerequisite: 'Pré-requisito',
  set: 'Lista',
  offered_by: 'Oferecida por',
};

const TOKEN = /^([a-z_]+)(?:\[(\d+)\])?$/;

function camel(name: string): string {
  return name.replace(/_([a-z])/g, (_, c: string) => c.toUpperCase());
}

function asRecord(v: unknown): Record<string, unknown> | undefined {
  return typeof v === 'object' && v !== null ? (v as Record<string, unknown>) : undefined;
}

function nameOfNode(node: unknown, fallback: string): string {
  const name = asRecord(node)?.['namePt'];
  return typeof name === 'string' && name !== '' ? name : fallback;
}

/**
 * The place and the field of a violation's path in an entry: "table_class.levels[0].features[0].effects[0].target" in the
 * class Guardião do Vale is level 1, the feature Vigília, effect 1, the field "O que muda". The entry's own body names the
 * level and the feature, so the path needs no table per editor; a path it cannot read gives what it could.
 */
export function placeOfPath(entry: TableEntry, field: string): PathPlace {
  const tokens = field.split('.').slice(1);
  let cursor = asRecord(entry.body.value);
  const parts: string[] = [];
  let inEffect = false;
  let label = '';
  for (const raw of tokens) {
    const m = TOKEN.exec(raw);
    if (!m) {
      continue;
    }
    const name = m[1];
    const index = m[2] === undefined ? undefined : Number(m[2]);
    const list = cursor?.[camel(name)];
    const node =
      index === undefined ? list : Array.isArray(list) ? (list as unknown[])[index] : undefined;
    const own = (inEffect ? EFFECT_FIELD_LABELS[name] : undefined) ?? FIELD_LABELS[name];
    if (index !== undefined && name === 'levels') {
      const level = asRecord(node)?.['level'];
      parts.push(`Nível ${typeof level === 'number' && level > 0 ? level : index + 1}`);
    } else if (index !== undefined && (name === 'features' || name === 'traits')) {
      parts.push(
        nameOfNode(node, `${name === 'traits' ? 'traço' : 'característica'} ${index + 1}`),
      );
    } else if (name === 'feature') {
      parts.push(nameOfNode(node, 'característica'));
    } else if (index !== undefined && name === 'options') {
      parts.push(`opção ${nameOfNode(node, String(index + 1))}`);
    } else if (index !== undefined && name === 'effects') {
      parts.push(`efeito ${index + 1}`);
      inEffect = true;
    } else if (own) {
      label = own;
    }
    cursor = asRecord(node);
  }
  return { parts, field: label };
}

/** The sentence of a violation with its first letter small, to follow "O que muda:". */
function lowered(text: string): string {
  return /^[A-ZÀ-Ý][a-zà-ÿ]/.test(text) ? text.charAt(0).toLowerCase() + text.slice(1) : text;
}

/** An entry that is out only because the class or race it belongs to is out. */
export interface LeftOutDependent {
  readonly entry: TableEntry;
  /** "Saiu porque a classe dela, Guardião do Vale, está fora. Volta sozinha quando a classe voltar." */
  readonly text: string;
}

/** One entry the master has to fix, with what is out together with it. */
export interface LeftOutRow {
  readonly entry: TableEntry;
  /** The violation, with its key left as the entry's own. */
  readonly violation: TableContentViolation;
  readonly place: PathPlace;
  /** The Portuguese sentence of the violation, first letter small when it follows a place. */
  readonly text: string;
  /** "Nível 1 · Vigília · efeito 1", or `''`. */
  readonly placeLine: string;
  /** The place for a phone, where there is no room for the level: "Vigília, efeito 1". */
  readonly shortPlace: string;
  /** "Classe", "Talento": the word of the kind for the tag. */
  readonly kindWord: string;
  readonly dependents: readonly LeftOutDependent[];
  /** "1 ficha usa esta classe." or `''`. */
  readonly usingLine: string;
  /** "1 ficha usa", for a phone. */
  readonly usingShort: string;
}

export interface LeftOutModel {
  /** The entries to fix, in the list's order (kind, then name). */
  readonly rows: readonly LeftOutRow[];
  /** Every entry left out, dependents included: "3 entradas fora da campanha". */
  readonly count: number;
  /** The keys of every entry left out, for the tag on the list's rows. */
  readonly keys: ReadonlySet<string>;
}

export const NO_LEFT_OUT: LeftOutModel = { rows: [], count: 0, keys: new Set() };

/** The key of the class or race an entry belongs to, or `''` for one that stands alone. */
export function parentKeyOf(entry: TableEntry): string {
  const b = entry.body;
  switch (b.case) {
    case 'tableSubclass':
    case 'tableClassFeature':
      return b.value.classKey;
    case 'tableSubrace':
      return b.value.raceKey;
    default:
      return '';
  }
}

function parentWord(entry: TableEntry): string {
  return entry.body.case === 'tableSubrace' ? 'raça' : 'classe';
}

/** "A ficha que usa" lines: "1 ficha usa esta classe.", "3 fichas usam este talento.". */
export function usingLineOf(entry: TableEntry): string {
  const n = entry.charactersUsing;
  if (n <= 0) {
    return '';
  }
  const noun = KIND_NOUNS[entry.kind];
  const this_ = noun?.article === 'O' ? 'este' : 'esta';
  return `${n} ${n === 1 ? 'ficha usa' : 'fichas usam'} ${this_} ${noun?.noun ?? 'entrada'}.`;
}

/** "1 ficha usa", "3 fichas usam", or `''`. */
export function usingShortOf(entry: TableEntry): string {
  const n = entry.charactersUsing;
  return n > 0 ? `${n} ${n === 1 ? 'ficha usa' : 'fichas usam'}` : '';
}

/** What the page says of the entries the rules refuse: rows to fix, with what each took out with it. */
export function buildLeftOut(
  entries: readonly TableEntry[],
  leftOut: readonly TableContentViolation[],
): LeftOutModel {
  if (leftOut.length === 0) {
    return NO_LEFT_OUT;
  }
  const out = new Map(leftOut.map((v) => [v.key, v]));
  const byKey = new Map(entries.map((e) => [e.key, e]));
  const dependents = new Map<string, LeftOutDependent[]>();
  const rows: LeftOutRow[] = [];
  for (const entry of entries) {
    const violation = out.get(entry.key);
    if (!violation) {
      continue;
    }
    const parent = byKey.get(parentKeyOf(entry));
    if (parent && out.has(parent.key) && violation.reason === 'dangling_reference') {
      const list = dependents.get(parent.key) ?? [];
      const word = parentWord(entry);
      list.push({
        entry,
        text: `Saiu porque a ${word} dela, ${parent.namePt}, está fora. Volta sozinha quando a ${word} voltar.`,
      });
      dependents.set(parent.key, list);
      continue;
    }
    const place = placeOfPath(entry, violation.field);
    const ctx = { aOne: navOfKind(entry.kind)?.aOne ?? 'uma entrada' };
    rows.push({
      entry,
      violation,
      place,
      text: lowered(violationText(violation, ctx)),
      placeLine: place.parts.join(' · '),
      shortPlace: place.parts.filter((p) => !p.startsWith('Nível')).join(', '),
      kindWord: KIND_WORDS[entry.kind] ?? 'Entrada',
      dependents: [],
      usingLine: usingLineOf(entry),
      usingShort: usingShortOf(entry),
    });
  }
  const withDependents = rows.map((r) => ({ ...r, dependents: dependents.get(r.entry.key) ?? [] }));
  return { rows: withDependents, count: leftOut.length, keys: new Set(out.keys()) };
}

/** "3 entradas fora da campanha". */
export function leftOutTitle(count: number): string {
  return `${count} ${count === 1 ? 'entrada fora' : 'entradas fora'} da campanha`;
}

/** "Junto com ela: Juramento da Névoa (subclasse), que depende dela." The names are bold on the screen. */
export function togetherWith(row: LeftOutRow): {
  readonly names: readonly { name: string; word: string }[];
  readonly tail: string;
} {
  return {
    names: row.dependents.map((d) => ({
      name: d.entry.namePt,
      word: (KIND_WORDS[d.entry.kind] ?? '').toLowerCase(),
    })),
    tail: row.dependents.length === 1 ? 'que depende dela' : 'que dependem dela',
  };
}

/** Whether the entry the page shows is one of the left-out ones, and the violation to put on its editor. */
export function violationOfEntry(
  leftOut: readonly TableContentViolation[],
  key: string,
): TableContentViolation | undefined {
  return leftOut.find((v) => v.key === key);
}
