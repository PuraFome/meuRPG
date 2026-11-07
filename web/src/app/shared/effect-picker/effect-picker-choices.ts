import type { ChoiceKind, FeatChoice } from '../../core/content/effect-choices';
import { choiceRef } from '../../core/content/effect-choices';
import type { SelectGroup, SelectOption } from '../form-fields/select-field';

/**
 * How the effect picker offers the feat's named choices (RN-23, MR-025). The fields that may hold `@name` are the server's
 * (`pickFields` in `rules/picks.go`); each writes the choice in the form its own list uses: "save.@habilidade" for a proficiency,
 * "damage-type:@tipo" for a damage type, "@habilidade" for an ability.
 */

/** The group of the feat's own choices, above the fixed values. */
export const CHOICE_GROUP = 'A escolha do talento';
export const FIXED_GROUP = 'Valores fixos';
export const CHOICE_GROUP_ICON = 'alt_route';
export const CHOICE_GROUP_NOTE = 'A escolha do talento aparece nos efeitos que vêm depois dela.';
/** The line under the list of a field in an option of the feat, where no choice is offered. */
export const OPTION_NOTE = 'As opções do talento não usam as escolhas dele: só valores fixos.';

export const CHOICE_GROUPS: Readonly<Record<string, SelectGroup>> = {
  [CHOICE_GROUP]: { icon: CHOICE_GROUP_ICON, accent: true },
};

/** One way a field writes a choice: the text before `@name`, how the entry starts, and the line beside the field once it is picked. */
interface RefForm {
  readonly prefix: string;
  readonly stem: string;
  readonly hint: (effect: number) => string;
}

const SAVE: RefForm = {
  prefix: 'save.',
  stem: 'Teste de resistência',
  hint: (n) => `Teste de resistência da habilidade escolhida no efeito ${n}.`,
};
const SCORE: RefForm = {
  prefix: 'score.',
  stem: 'Valor',
  hint: (n) => `Valor da habilidade escolhida no efeito ${n}.`,
};
const CHECK: RefForm = {
  prefix: 'check.',
  stem: 'Teste de habilidade',
  hint: (n) => `Testes da habilidade escolhida no efeito ${n}.`,
};
const ABILITY: RefForm = {
  prefix: '',
  stem: 'Escolha',
  hint: (n) => `A habilidade escolhida no efeito ${n}.`,
};
const DAMAGE: RefForm = {
  prefix: 'damage-type:',
  stem: 'Escolha',
  hint: (n) => `O tipo de dano escolhido no efeito ${n}.`,
};

const PROFICIENCY: RefForm = {
  ...SAVE,
  hint: (n) => `Proficiência na resistência da habilidade escolhida no efeito ${n}.`,
};

/** The fields that take a choice (the formulas take `mod("@name")` and `score("@name")`, see `formulaInsertions`), by the proto name. */
const REF_FIELDS: Readonly<Record<string, { kind: ChoiceKind; forms: readonly RefForm[] }>> = {
  proficiency: { kind: 'ability', forms: [PROFICIENCY] },
  target: { kind: 'ability', forms: [SCORE, SAVE] },
  targets: { kind: 'ability', forms: [CHECK, SAVE] },
  ability: { kind: 'ability', forms: [ABILITY] },
  attack_ability: { kind: 'ability', forms: [ABILITY] },
  damage_ability: { kind: 'ability', forms: [ABILITY] },
  damage_type: { kind: 'damage_type', forms: [DAMAGE] },
  damage_types: { kind: 'damage_type', forms: [DAMAGE] },
  except: { kind: 'damage_type', forms: [DAMAGE] },
};

/** The kind of choice a field may point at, or null when the field takes none. */
export function refKind(field: string): ChoiceKind | null {
  return REF_FIELDS[field]?.kind ?? null;
}

/** What the field writes for a form and a choice: "save.@habilidade". */
function refValue(form: RefForm, choice: FeatChoice): string {
  return `${form.prefix}${choiceRef(choice)}`;
}

/** An entry of the group: the choice written in the field's own words, with where it comes from at the right. */
export interface RefOption extends SelectOption {
  readonly hint: string;
}

/** The entries of the group "A escolha do talento" for a field: each form of the field for each choice, the forms in order. */
export function refOptions(
  field: string,
  choices: readonly FeatChoice[],
  sideOf: (choice: FeatChoice) => string,
): RefOption[] {
  const own = REF_FIELDS[field];
  if (!own) {
    return [];
  }
  return own.forms.flatMap((form) =>
    choices.map((c) => ({
      value: refValue(form, c),
      label: `${form.stem} · ${c.label}`,
      side: sideOf(c),
      group: CHOICE_GROUP,
      hint: form.hint(c.index + 1),
    })),
  );
}

/** "do efeito 1", and for a damage type its list: "do efeito 1 · ácido, elétrico ou fogo". */
export function choiceSide(choice: FeatChoice, damageTypes: readonly string[]): string {
  const base = `do efeito ${choice.index + 1}`;
  if (choice.kind !== 'damage_type' || damageTypes.length === 0) {
    return base;
  }
  const head = damageTypes.slice(0, -1).join(', ');
  const list = head === '' ? damageTypes[0] : `${head} ou ${damageTypes[damageTypes.length - 1]}`;
  return `${base} · ${list}`;
}

/** The fixed values of a field's list, under their own heading, once the choices are above them. */
export function fixedGroup<T extends SelectOption>(options: readonly T[]): T[] {
  return options.map((o) => ({ ...o, group: FIXED_GROUP }));
}

/** The text of the value a field holds, when it is one of the group's entries. */
export function pickedRef(options: readonly RefOption[], value: string): RefOption | undefined {
  return options.find((o) => o.value === value);
}

/** The text with `insert` written over the selection (at the end when there is none), and where the cursor goes after it. */
export function insertAt(
  text: string,
  insert: string,
  from: number | null | undefined,
  to: number | null | undefined,
): { next: string; caret: number } {
  const start = Math.min(from ?? text.length, text.length);
  const end = Math.min(to ?? start, text.length);
  return { next: text.slice(0, start) + insert + text.slice(end), caret: start + insert.length };
}
