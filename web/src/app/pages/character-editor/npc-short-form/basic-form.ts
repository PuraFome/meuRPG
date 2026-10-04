import {
  AbstractControl,
  FormArray,
  FormBuilder,
  FormControl,
  FormGroup,
  ValidationErrors,
  ValidatorFn,
  Validators,
} from '@angular/forms';

import { DamageTypeKey, formatModifier } from '../../../core/characters/character-labels';
import { feetToMeters, metersToFeet } from '../../../core/units';
import { BasicAttackFormValue, BasicCharacterFormValue } from '../character-editor.types';
import { EditorField } from '../editor-labels';

/** The limits `characters.proto` documents for a basic sheet. The server
 * checks the same ones; the form only saves a round trip. */
export const MAX_ATTACKS = 3;
const DIE_SIDES = [4, 6, 8, 10, 12];
const MAX_SPEED_METERS = 90; // 300 ft
const WALK_STEP_METERS = 1.5;

/** One attack card. The signed numbers and the dice are typed as text, so
 * "+4", "4" and "−1" all work ("Aceita 2, +2 ou −1"); `basicFormToValue`
 * turns them into numbers. `rangeFt` has no field on screen yet: it rides
 * along so editing a sheet never loses it. */
export type BasicAttackFormGroup = FormGroup<{
  name: FormControl<string>;
  attackBonus: FormControl<string>;
  dice: FormControl<string>;
  damageBonus: FormControl<string>;
  damageType: FormControl<DamageTypeKey>;
  rangeFt: FormControl<number>;
}>;

export type BasicSheetFormGroup = FormGroup<{
  name: FormControl<string>;
  hitPointsMax: FormControl<number>;
  armorClass: FormControl<number>;
  speedWalkM: FormControl<number>;
  initiativeBonus: FormControl<string>;
  attacks: FormArray<BasicAttackFormGroup>;
  /** The free-text damage of a sheet saved before Etapa 6 that could not
   * become an attack; shown as a note, never edited. */
  legacyDamage: FormControl<string>;
  legacyAttackBonus: FormControl<number>;
  description: FormControl<string>;
  /** "Nível de desafio (ND)" and "XP ao derrotar" (E7-11): a new minion starts
   * at ND 0, 10 XP, so nobody is left without a number. */
  challengeRating: FormControl<string>;
  xpValue: FormControl<number>;
  /** The portrait's image ID (MR-031), carried as read: saving the short
   * form must never clear it. */
  portraitImageId: FormControl<string>;
}>;

/** "+2", "2", "-1" and "−1" (a real minus) as a number; `null` for
 * anything else, including an empty text and decimals. */
export function parseSigned(text: string): number | null {
  const clean = text.trim().replace('−', '-');
  return /^[+-]?\d{1,3}$/.test(clean) ? Number(clean) : null;
}

/** Valid when the text is a signed whole number within `min` to `max`. */
function signedWithin(min: number, max: number): ValidatorFn {
  return (control: AbstractControl): ValidationErrors | null => {
    const n = parseSigned(String(control.value ?? ''));
    return n !== null && n >= min && n <= max ? null : { signed: { min, max } };
  };
}

/** "NdS": 1 to 20 dice of 4, 6, 8, 10 or 12 faces. */
export function parseDice(text: string): { count: number; sides: number } | null {
  const m = /^(\d{1,2})\s*d\s*(\d{1,2})$/i.exec(text.trim());
  if (!m) return null;
  const count = Number(m[1]);
  const sides = Number(m[2]);
  return count >= 1 && count <= 20 && DIE_SIDES.includes(sides) ? { count, sides } : null;
}

const diceValidator: ValidatorFn = (control) =>
  parseDice(String(control.value ?? '')) ? null : { dice: true };

/** Metres in 1,5 m steps (5 ft), 0 to 90 m. */
const speedValidator: ValidatorFn = (control) => {
  const m = control.value;
  if (typeof m !== 'number' || !Number.isFinite(m)) return { required: true };
  if (m < 0 || m > MAX_SPEED_METERS) return { range: true };
  // Tenths, to keep 4,5 and 7,5 exact despite floating point.
  return Math.round(m * 10) % (WALK_STEP_METERS * 10) === 0 ? null : { step: true };
};

export function createAttackGroup(
  fb: FormBuilder,
  value?: Partial<BasicAttackFormValue>,
): BasicAttackFormGroup {
  const bonus = (n: number | undefined) => formatModifier(n ?? 0);
  return fb.nonNullable.group({
    name: [value?.name ?? '', [Validators.required, Validators.maxLength(40)]],
    attackBonus: [bonus(value?.attackBonus), [signedWithin(-10, 20)]],
    dice: [
      value?.damageDiceCount ? `${value.damageDiceCount}d${value.damageDiceSides}` : '1d6',
      [diceValidator],
    ],
    damageBonus: [bonus(value?.damageBonus), [signedWithin(-20, 40)]],
    damageType: [value?.damageType ?? ('' as DamageTypeKey), Validators.required],
    rangeFt: [value?.rangeFt ?? 0],
  });
}

export function createBasicForm(fb: FormBuilder): BasicSheetFormGroup {
  return fb.nonNullable.group({
    name: ['', [Validators.required, Validators.maxLength(80)]],
    hitPointsMax: [1, [Validators.required, Validators.min(1)]],
    armorClass: [10, [Validators.required, Validators.min(1)]],
    speedWalkM: [9, speedValidator],
    initiativeBonus: ['+0', signedWithin(-10, 20)],
    attacks: fb.array<BasicAttackFormGroup>([], Validators.maxLength(MAX_ATTACKS)),
    legacyDamage: [''],
    legacyAttackBonus: [0],
    description: ['', Validators.maxLength(2000)],
    challengeRating: ['0'],
    xpValue: [10, [Validators.required, Validators.min(0), Validators.max(1_000_000)]],
    portraitImageId: [''],
  });
}

/** Fills the form from a loaded sheet: one card per attack. */
export function patchBasicForm(
  fb: FormBuilder,
  form: BasicSheetFormGroup,
  value: BasicCharacterFormValue,
): void {
  form.controls.attacks.clear();
  for (const attack of value.attacks) {
    form.controls.attacks.push(createAttackGroup(fb, attack));
  }
  form.patchValue({
    name: value.name,
    hitPointsMax: value.hitPointsMax,
    armorClass: value.armorClass,
    speedWalkM: feetToMeters(value.speedFt),
    initiativeBonus: formatModifier(value.initiativeBonus),
    legacyDamage: value.legacyDamage,
    legacyAttackBonus: value.legacyAttackBonus,
    description: value.description,
    challengeRating: value.challengeRating,
    xpValue: value.xpValue,
    portraitImageId: value.portraitImageId,
  });
}

/** The form as the value the editor saves: numbers, feet. */
export function basicFormToValue(form: BasicSheetFormGroup): BasicCharacterFormValue {
  const raw = form.getRawValue();
  return {
    name: raw.name,
    hitPointsMax: raw.hitPointsMax,
    armorClass: raw.armorClass,
    speedFt: metersToFeet(raw.speedWalkM),
    initiativeBonus: parseSigned(raw.initiativeBonus) ?? 0,
    attacks: raw.attacks.map((a) => {
      const dice = parseDice(a.dice);
      return {
        name: a.name,
        attackBonus: parseSigned(a.attackBonus) ?? 0,
        damageDiceCount: dice?.count ?? 1,
        damageDiceSides: dice?.sides ?? 6,
        damageBonus: parseSigned(a.damageBonus) ?? 0,
        damageType: a.damageType,
        rangeFt: a.rangeFt,
      };
    }),
    legacyDamage: raw.legacyDamage,
    legacyAttackBonus: raw.legacyAttackBonus,
    description: raw.description,
    challengeRating: raw.challengeRating,
    xpValue: raw.xpValue,
    portraitImageId: raw.portraitImageId,
  };
}

const BASIC_FIELD_LABELS: readonly [string, string][] = [
  ['name', 'Nome do personagem'],
  ['hitPointsMax', 'Pontos de vida (máximo)'],
  ['armorClass', 'Classe de Armadura'],
  ['speedWalkM', 'Deslocamento'],
  ['initiativeBonus', 'Iniciativa'],
  ['xpValue', 'XP ao derrotar'],
  ['description', 'Descrição'],
];

const ATTACK_FIELD_LABELS: readonly [string, string][] = [
  ['name', 'Nome do ataque'],
  ['attackBonus', 'Bônus de ataque'],
  ['dice', 'Dados do dano'],
  ['damageBonus', 'Bônus do dano'],
  ['damageType', 'Tipo de dano'],
];

/** Every invalid control of the short form, in screen order, for the "fix
 * these first" notice; an attack's fields say which card ("Ataque 2: Dados
 * do dano"). */
export function invalidBasicFields(form: BasicSheetFormGroup): EditorField[] {
  const fields: EditorField[] = BASIC_FIELD_LABELS.filter(([path]) => form.get(path)?.invalid).map(
    ([path, label]) => ({ path, label, step: null }),
  );
  form.controls.attacks.controls.forEach((group, i) => {
    for (const [path, label] of ATTACK_FIELD_LABELS) {
      if (group.get(path)?.invalid) {
        fields.push({
          path: `attacks.${i}.${path}`,
          label: `Ataque ${i + 1}: ${label}`,
          step: null,
        });
      }
    }
  });
  return fields;
}
