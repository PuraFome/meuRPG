import { create } from '@bufbuild/protobuf';

import { HookResultSchema } from '../../../gen/meurpg/play/v1/combat_pb';
import { hookEffects, hookSentence, hookStandUpSentence } from './hook-text';

/** Invented hooks: "Fôlego da Mesa" gives temporary hit points on a rest, "Reserva de Teste" stands the character up. */
const hook = (over: object = {}) =>
  create(HookResultSchema, { sourceNamePt: 'Fôlego da Mesa', ...over });

describe('what an on_event hook did, in words (the web prints what arrives)', () => {
  it('lists the hit points, the temporary ones and the uses given back or spent', () => {
    expect(hookEffects(hook({ healed: 5, tempHpGained: 3 }))).toEqual([
      'recupera 5 PV',
      'ganha 3 PV temporários',
    ]);
    expect(hookEffects(hook({ restored: 1, restoredResource: 'reserva_de_teste' }))).toEqual([
      'devolve 1 uso de reserva de teste',
    ]);
    expect(hookEffects(hook({ spent: 2, spentResource: 'reserva_de_teste' }))).toEqual([
      'gasta 2 usos de reserva de teste',
    ]);
  });

  it('says nothing for a number that did not arrive', () => {
    expect(hookEffects(hook())).toEqual([]);
  });

  it('marks the dice of a heal as an average when the numbers are an estimate', () => {
    expect(hookEffects(hook({ healed: 5, tempHpGained: 3 }), true)).toEqual([
      'recupera 5 PV (em média)',
      'ganha 3 PV temporários',
    ]);
  });

  it('names the feature that has it, or says "Uma característica" when the line cannot', () => {
    expect(hookSentence(hook({ tempHpGained: 3 }))).toBe(
      'Fôlego da Mesa entrou em ação: ganha 3 PV temporários',
    );
    expect(hookSentence(hook({ sourceNamePt: '', tempHpGained: 3 }))).toBe(
      'Uma característica entrou em ação: ganha 3 PV temporários',
    );
  });

  it('says who it stood up and with how many hit points', () => {
    expect(hookStandUpSentence(hook({ healed: 6 }), 'Toren')).toBe(
      'Fôlego da Mesa levantou Toren com 6 PV',
    );
    expect(hookStandUpSentence(hook({ healed: 0 }), 'Toren')).toBe('Fôlego da Mesa entrou em ação');
  });
});
