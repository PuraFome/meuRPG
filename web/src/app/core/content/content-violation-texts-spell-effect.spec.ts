import { violationText } from './content-violations';

const CTX = { aOne: 'uma magia' };

// The paths and reasons of backend/internal/rules/overlay_spell_effect.go.
const CASES: [field: string, reason: string, text: RegExp][] = [
  ['table_spell.effect.player_label_pt', 'limit', /até 30 caracteres/],
  ['table_spell.effect.visibility', 'bad_value', /quem vê o efeito/],
  ['table_spell.effect.applies', 'bad_value', /Quem falhar no teste.*pede um teste/],
  ['table_spell.effect.conditions', 'limit', /No máximo 8 condições/],
  ['table_spell.effect.conditions[2]', 'bad_key', /não é do SRD/],
  ['table_spell.effect.modifiers', 'limit', /No máximo 8 mudanças/],
  ['table_spell.effect.modifiers[1].kind', 'bad_value', /Incapacitado, Paralisado/],
  ['table_spell.effect.modifiers[0].value', 'bad_value', /PV temporários de 1 a 1.000/],
  ['table_spell.effect.modifiers[0].applies_to', 'bad_value', /ao menos uma jogada/],
  ['table_spell.effect.modifiers[3].abilities', 'bad_value', /ao menos uma habilidade/],
  ['table_spell.effect.modifiers[0].damage_types', 'bad_value', /ao menos um tipo de dano/],
  ['table_spell.effect.modifiers[0]', 'bad_value', /Confira esta mudança/],
  ['table_spell.effect.aura.radius_ft', 'bad_value', /de 1,5 m a 36 m/],
  ['table_spell.effect.aura.affects', 'bad_value', /quem a aura alcança/],
  ['table_spell.effect.aura', 'bad_value', /Só quem conjura.*não vale CA/],
  ['table_spell.effect', 'bad_value', /ao menos uma condição ou uma mudança/],
];

describe('the words of a table spell effect refusal (#579)', () => {
  it.each(CASES)('%s (%s)', (field, reason, text) => {
    expect(violationText({ field, reason }, CTX)).toMatch(text);
  });
});
