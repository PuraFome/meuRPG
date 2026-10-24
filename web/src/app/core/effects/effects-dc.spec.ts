import { create } from '@bufbuild/protobuf';

import {
  EffectPhase,
  EffectSavePromptSchema,
  LastingEffectSchema,
} from '../../../gen/meurpg/play/v1/lasting_effects_pb';
import { combatCards, effectSaveView } from './effects';

const card = (phase: EffectPhase, dc?: number) =>
  create(LastingEffectSchema, {
    id: 'e1',
    sourceNamePt: 'Imobilizar Pessoa',
    ...(phase === EffectPhase.END
      ? { endSave: { ability: 'wis', abilityNamePt: 'Sabedoria', phase, dc } }
      : { startSave: { ability: 'dex', abilityNamePt: 'Destreza', phase, dc } }),
  });

describe('the save DC on the cards of "Seus efeitos"', () => {
  it('adds the DC to the end-of-turn save when the server sent it', () => {
    expect(combatCards([card(EffectPhase.END, 14)])[0].save).toBe(
      'No fim de cada turno seu: teste de resistência de Sabedoria (CD 14).',
    );
  });

  it('adds the DC to the start-of-turn save when the server sent it', () => {
    expect(combatCards([card(EffectPhase.START, 17)])[0].save).toBe(
      'No começo de cada turno seu: teste de resistência de Destreza (CD 17).',
    );
  });

  it('says what it said before when the DC is absent or 0, and never makes one up', () => {
    expect(combatCards([card(EffectPhase.END)])[0].save).toBe(
      'No fim de cada turno seu: teste de resistência de Sabedoria.',
    );
    expect(combatCards([card(EffectPhase.START, 0)])[0].save).toBe(
      'No começo de cada turno seu: teste de resistência de Destreza.',
    );
  });
});

describe('the save DC on the question of an effect save', () => {
  const prompt = (dc?: number) =>
    create(EffectSavePromptSchema, {
      effectId: 'e1',
      abilityNamePt: 'Sabedoria',
      phase: EffectPhase.END,
      mode: 'normal',
      textPt: 'Teste de resistência de Sabedoria. Se passar, o efeito acaba sobre você.',
      dc,
    });

  it('carries "CD 14" when the prompt has the DC', () => {
    expect(effectSaveView(prompt(14), 3).dc).toBe('CD 14');
  });

  it('carries nothing when the prompt has none, or 0', () => {
    expect(effectSaveView(prompt(), 3).dc).toBe('');
    expect(effectSaveView(prompt(0), 3).dc).toBe('');
  });
});
