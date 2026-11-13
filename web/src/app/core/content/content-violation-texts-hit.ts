/** The first sentence the trigger's refusal says, and what the form says itself when a stored trigger is unknown. */
export const HIT_TRIGGER_PT = 'Escolha quando vale.';

/** What the server's reason at a field of a spell's "Efeito no próximo golpe" says in Portuguese (#577): the paths of
 * backend/internal/rules/weaponhitdamage.go (`checkWeaponHitDamage`, `spellHitRefusal`), under `table_spell.effect.modifiers[i]`.
 * Every refusal there has the reason `bad_value`, so the field alone tells which; the effect's own limit of modifiers is `limit`. */
export const HIT_FIELD_TEXTS: readonly { shape: RegExp; reason?: string; text: string }[] = [
  // The effect holds at most 8 modifiers (`maxSpellEffectItems`, overlay_spell_effect.go), this one among them.
  {
    shape: /^table_spell\.effect\.modifiers$/,
    reason: 'limit',
    text: 'Um efeito tem no máximo 8 modificadores. Tire algum antes de ligar este.',
  },
  {
    shape: /^table_spell\.effect\.modifiers\[\]\.trigger$/,
    text: `${HIT_TRIGGER_PT} “Cada golpe” e “alvo marcado” pedem concentração; o alvo marcado pede também o alvo Uma criatura.`,
  },
  { shape: /^table_spell\.effect\.modifiers\[\]\.die$/, text: 'O dado é d4, d6, d8, d10 ou d12.' },
  { shape: /^table_spell\.effect\.modifiers\[\]\.dice$/, text: 'De 1 a 10 dados.' },
  { shape: /^table_spell\.effect\.modifiers\[\]\.per_slot_dice$/, text: 'De 0 a 5 por nível.' },
  {
    shape: /^table_spell\.effect\.modifiers\[\]\.max_dice$/,
    text: 'De 1 a 20 dados, e nunca abaixo dos dados da magia.',
  },
  {
    shape: /^table_spell\.effect\.modifiers\[\]\.damage_type$/,
    text: 'Este tipo de dano não existe. Escolha da lista ou use o da arma.',
  },
  {
    shape: /^table_spell\.effect\.modifiers\[\]\.save$/,
    text: 'Escolha uma das seis habilidades, ou tire o teste.',
  },
  {
    shape: /^table_spell\.effect\.modifiers\[\]\.on_fail_conditions$/,
    text: 'Escolha a condição de quem falhar, ou tire o teste.',
  },
  // One condition of the list: the server says it is not an SRD condition, or that it is listed twice.
  {
    shape: /^table_spell\.effect\.modifiers\[\]\.on_fail_conditions\[\]$/,
    text: 'Esta condição não existe ou está repetida. Escolha cada condição uma vez.',
  },
  {
    shape: /^table_spell\.effect\.modifiers\[\]\.condition_duration$/,
    text: 'Escolha até quando dura. “Até o fim da magia” pede uma magia que dura; sem teste, não há o que durar.',
  },
  {
    shape: /^table_spell\.effect\.modifiers\[\]\.kind$/,
    text: 'Uma magia tem um só efeito no próximo golpe.',
  },
];
