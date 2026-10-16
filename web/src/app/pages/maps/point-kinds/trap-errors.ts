import { POINT_DESCRIPTION_MAX, POINT_NAME_MAX } from '../point-panel/point-draft';
import { TrapPassOutcome, TrapSaveApplies } from '../../../../gen/meurpg/rules/v1/rules_pb';
import type { AttackDraft, SaveDraft, TrapDraft } from './trap-draft-types';

export interface TrapErrors {
  readonly name?: string;
  readonly description?: string;
  readonly noticeDc?: string;
  readonly findDc?: string;
  readonly attackBonus?: string;
  readonly attackCount?: string;
  readonly attackDice?: string;
  /** By part: `damage:0`, `fail:1`, `condition:0`, `saveDc`, `save` (the save as a whole). */
  readonly parts: Readonly<Record<string, string>>;
}

/** The most dice of one roll the server takes. */
const DICE_COUNT_MAX = 20;
const DICE = /^(\d{1,2})d(4|6|8|10|12)$/;
const DICE_MESSAGE = 'Use dados como 2d6 (de d4 a d12) ou um número de 1 a 100.';

/** Whether `text` is a die the server takes: 1 to 20 dice of d4 to d12, or a flat 1 to 100. */
export function validDice(text: string): boolean {
  const t = text.trim();
  const m = DICE.exec(t);
  if (m) {
    const n = Number(m[1]);
    return n >= 1 && n <= DICE_COUNT_MAX;
  }
  return /^\d{1,3}$/.test(t) && Number(t) >= 1 && Number(t) <= 100;
}

function wholeIn(text: string, min: number, max: number): boolean {
  return /^\d{1,3}$/.test(text.trim()) && Number(text) >= min && Number(text) <= max;
}

const CONDITION_DURATION_MAX = 60;
const DC_MAX = 30;
const ATTACK_BONUS_MAX = 20;
const ATTACK_COUNT_MAX = 10;
const DURATION_MESSAGE = `Use até ${CONDITION_DURATION_MAX} caracteres.`;
const DC_MESSAGE = `Use uma CD de 1 a ${DC_MAX}.`;

/** What is wrong with the name, or `undefined`. */
function nameError(raw: string): string | undefined {
  const name = raw.trim();
  if (name === '') {
    return 'Dê um nome ao ponto.';
  }
  if ([...name].length > POINT_NAME_MAX) {
    return `Use até ${POINT_NAME_MAX} caracteres.`;
  }
  return /\p{Cc}/u.test(name) ? 'Use um nome numa linha só.' : undefined;
}

/** The fields of the form that are not a part of the effect. */
function fieldErrors(d: TrapDraft): Omit<TrapErrors, 'parts'> {
  const errors: { -readonly [K in keyof TrapErrors]: TrapErrors[K] } = { parts: {} };
  const name = nameError(d.name);
  if (name) {
    errors.name = name;
  }
  if ([...d.description].length > POINT_DESCRIPTION_MAX) {
    errors.description = 'Use até 2.000 caracteres.';
  }
  if (d.noticeDc.trim() !== '' && !wholeIn(d.noticeDc, 1, DC_MAX)) {
    errors.noticeDc = 'Use uma CD de 1 a 30, ou deixe vazio.';
  }
  if (!wholeIn(d.findDc, 1, DC_MAX)) {
    errors.findDc = DC_MESSAGE;
  }
  return errors;
}

/** The attack's bonus, count and damage. */
function attackErrors(attack: AttackDraft): Omit<TrapErrors, 'parts'> {
  const errors: { -readonly [K in keyof TrapErrors]: TrapErrors[K] } = { parts: {} };
  if (!wholeIn(attack.bonus, 0, ATTACK_BONUS_MAX)) {
    errors.attackBonus = `Use de 0 a ${ATTACK_BONUS_MAX}.`;
  }
  if (!wholeIn(attack.count, 1, ATTACK_COUNT_MAX)) {
    errors.attackCount = `Use de 1 a ${ATTACK_COUNT_MAX}.`;
  }
  if (!validDice(attack.damage.dice)) {
    errors.attackDice = DICE_MESSAGE;
  }
  return errors;
}

/** What is wrong with the save as a whole: nothing happens on a failure, "metade" with no damage, "atingido" with no attack. */
function saveRuleError(save: SaveDraft, hasAttack: boolean): string | undefined {
  let message: string | undefined;
  if (save.failDamage.length === 0 && !save.failCondition) {
    message = 'Escolha o que acontece a quem falha: dano ou uma condição.';
  } else if (save.onPass === TrapPassOutcome.HALF && save.failDamage.length === 0) {
    message = 'Metade do dano precisa de dano para quem falha.';
  }
  if (save.appliesTo === TrapSaveApplies.HIT && !hasAttack) {
    message =
      'Só quem foi atingido faz a resistência: acrescente um ataque ou escolha quem foi pego.';
  }
  return message;
}

function saveErrors(save: SaveDraft, hasAttack: boolean, parts: Record<string, string>): void {
  if (!wholeIn(save.dc, 1, DC_MAX)) {
    parts['saveDc'] = DC_MESSAGE;
  }
  save.failDamage.forEach((x, i) => {
    if (!validDice(x.dice)) {
      parts[`fail:${i}`] = DICE_MESSAGE;
    }
  });
  if (save.failCondition && [...save.failCondition.duration].length > CONDITION_DURATION_MAX) {
    parts['failCondition'] = DURATION_MESSAGE;
  }
  const rule = saveRuleError(save, hasAttack);
  if (rule) {
    parts['save'] = rule;
  }
}

/** The server's rules (maps.proto: the numbers are checked against the rules content), so a field can say what is wrong. */
export function trapErrors(d: TrapDraft): TrapErrors {
  const parts: Record<string, string> = {};
  d.damage.forEach((x, i) => {
    if (!validDice(x.dice)) {
      parts[`damage:${i}`] = DICE_MESSAGE;
    }
  });
  d.conditions.forEach((c, i) => {
    if ([...c.duration].length > CONDITION_DURATION_MAX) {
      parts[`condition:${i}`] = DURATION_MESSAGE;
    }
  });
  if (d.save) {
    saveErrors(d.save, d.attack !== null, parts);
  }
  return { ...fieldErrors(d), ...(d.attack ? attackErrors(d.attack) : {}), parts };
}

export function hasTrapErrors(e: TrapErrors): boolean {
  return Object.keys(e).some((k) => k !== 'parts') || Object.keys(e.parts).length > 0;
}
