import {
  AURA_REFUSED,
  AURA_REFUSED_PT,
  type EffectRow,
  type SpellEffectDraft,
  modifierIndexOf,
  rowFieldPath,
  readWhole,
  rowToModifiers,
} from './spell-effect-draft';

// ---- what the form says before the save ----

const RADIUS = /^\d+([.,]\d+)?$/;

/** The fields of a roll row the server may refuse. */
function rollFields(r: EffectRow): readonly string[] {
  if (r.rollMode === 'die') {
    return ['die', 'sign', 'applies_to'];
  }
  return r.rollMode === 'ability'
    ? ['value_ability', 'value_min', 'applies_to']
    : ['value', 'applies_to'];
}

/** The field of a row whose text is no number the save can send, or undefined. */
function unreadableField(r: EffectRow): string | undefined {
  const bad = (text: string) => Number.isNaN(readWhole(text));
  switch (r.kind) {
    case 'ac_bonus':
    case 'temp_hp':
    case 'check_bonus':
      return bad(r.value) ? 'value' : undefined;
    case 'roll':
      if (r.rollMode === 'die') {
        return bad(r.die) ? 'die' : undefined;
      }
      if (r.rollMode === 'ability') {
        return bad(r.valueMin) ? 'value_min' : undefined;
      }
      return bad(r.value) ? 'value' : undefined;
    case 'speed':
      return (r.speedMode === 'multiply' ? bad(r.percent) : !RADIUS.test(r.speedM.trim()))
        ? 'value'
        : undefined;
    default:
      return undefined;
  }
}

/** The paths of the rows' number fields (and the radius) whose text is no whole number the save can send, in the order of the form. */
export function effectUnreadable(d: SpellEffectDraft): string[] {
  if (!d.on) {
    return [];
  }
  const out: string[] = [];
  let at = 0;
  for (const r of d.rows) {
    const field = unreadableField(r);
    if (field) {
      out.push(`table_spell.effect.modifiers[${at}].${field}`);
    }
    at += rowToModifiers(r).length;
  }
  if (d.aura && !RADIUS.test(d.aura.radiusM.trim())) {
    out.push('table_spell.effect.aura.radius_ft');
  }
  return out;
}

/** What the form itself asks before the save (the server asks the same and more): the empty effect, the aura and its kinds. */
export interface EffectAdvice {
  readonly path: string;
  readonly text: string;
}

export function effectAdvice(d: SpellEffectDraft): EffectAdvice[] {
  if (!d.on) {
    return [];
  }
  const out: EffectAdvice[] = [];
  const written = d.rows.some((r) => rowToModifiers(r).length > 0);
  if (!written && d.conditions.length === 0) {
    out.push({
      path: 'table_spell.effect',
      text: 'Ponha ao menos uma condição ou uma mudança. Um efeito vazio não é salvo.',
    });
  }
  d.rows.forEach((r, i) => {
    if ((r.kind === 'advantage' || r.kind === 'roll') && r.rolls.length === 0) {
      out.push({
        path: rowFieldPath(d.rows, i, 'applies_to'),
        text: 'Escolha ao menos uma jogada.',
      });
    }
    if (r.kind === 'advantage' && r.rolls.some((w) => w !== 'attack') && r.abilities.length === 0) {
      out.push({
        path: rowFieldPath(d.rows, i, 'abilities', r.rolls.includes('attack') ? 1 : 0),
        text: 'Escolha ao menos uma habilidade.',
      });
    }
  });
  if (d.aura) {
    d.rows.forEach((r, i) => {
      if (r.kind !== '' && r.kind !== 'unknown' && AURA_REFUSED.includes(r.kind)) {
        out.push({
          path: `table_spell.effect.modifiers[${modifierIndexOf(d.rows, i)}].kind`,
          text: AURA_REFUSED_PT,
        });
      }
    });
  }
  return out;
}

/** The paths of the fields the block draws now (where a refusal of the server can land), in full. */
export function effectFieldPaths(d: SpellEffectDraft): string[] {
  if (!d.on) {
    return [];
  }
  const out = [
    'table_spell.effect',
    'table_spell.effect.modifiers',
    'table_spell.effect.visibility',
  ];
  out.push(
    ...(d.aura
      ? [
          'table_spell.effect.aura',
          'table_spell.effect.aura.radius_ft',
          'table_spell.effect.aura.affects',
        ]
      : ['table_spell.effect.applies', 'table_spell.effect.conditions']),
  );
  if (d.visibility === 'public') {
    out.push('table_spell.effect.player_label_pt');
  }
  let at = 0;
  for (const r of d.rows) {
    const p = `table_spell.effect.modifiers[${at}]`;
    const fields: Record<string, readonly string[]> = {
      ac_bonus: ['value'],
      temp_hp: ['value'],
      check_bonus: ['skill', 'value'],
      roll: rollFields(r),
      resistance: ['resistance_kind', 'damage_types'],
      speed: ['value'],
      advantage: ['applies_to'],
    };
    out.push(p, `${p}.kind`, ...(fields[r.kind] ?? []).map((f) => `${p}.${f}`));
    if (r.kind === 'advantage' && r.rolls.some((w) => w !== 'attack')) {
      out.push(
        `table_spell.effect.modifiers[${at + (r.rolls.includes('attack') ? 1 : 0)}].abilities`,
      );
    }
    if (rowToModifiers(r).length === 0 && (r.kind === 'advantage' || r.kind === 'roll')) {
      out.push(rowFieldPath(d.rows, d.rows.indexOf(r), 'applies_to'));
    }
    at += rowToModifiers(r).length;
  }
  return out;
}
