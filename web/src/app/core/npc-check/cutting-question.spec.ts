import { create } from '@bufbuild/protobuf';

import { DiceRollSchema, HeldRollKind } from '../../../gen/meurpg/play/v1/combat_pb';
import { CuttingWordsAnsweredSchema } from '../../../gen/meurpg/play/v1/resources_pb';
import { cutLabel, cuttingView, isCuttingOffer, resultView, typedFace } from './cutting-question';
import { cuttingOffer } from './npc-check-testing';

describe('isCuttingOffer', () => {
  it('is the bard question of an NPC check, and nothing else', () => {
    expect(isCuttingOffer(cuttingOffer())).toBe(true);
    expect(isCuttingOffer(cuttingOffer({ kind: HeldRollKind.GROUP_CHECK }))).toBe(false);
    expect(
      isCuttingOffer(cuttingOffer({ question: { case: 'lucky', value: { luckyIndex: 0 } } })),
    ).toBe(false);
    expect(isCuttingOffer(undefined)).toBe(false);
  });
});

describe('cuttingView', () => {
  it('names the creature the bard knows, with the die and the uses left', () => {
    const v = cuttingView(cuttingOffer());
    expect(v.title).toBe('Capitão bandido faz um teste');
    expect(v.subtitle).toBe('Palavras de Interrupção · fora de combate');
    expect(v.lead).toBe('Usar Palavras de Interrupção?');
    expect(v.text).toBe(
      'Você tira o seu dado de Inspiração de Bardo (d8) do teste de Capitão bandido. O resultado ainda não foi dito.',
    );
    expect(v.chips).toEqual(['Gasta 1 Inspiração de Bardo', 'Você tem 3', 'Não gasta reação']);
    expect(v.notes).toEqual([
      'Qual é o teste, o total e a CD ficam com o mestre.',
      'A cena espera a sua resposta. O mestre pode seguir sem ela.',
    ]);
  });

  it('reads "uma criatura", with no name, when the bard does not know it', () => {
    const v = cuttingView(cuttingOffer({ rollerLabel: 'uma criatura' }));
    expect(v.title).toBe('Uma criatura faz um teste');
    expect(v.text).toBe(
      'Você tira o seu dado de Inspiração de Bardo (d8) do teste da criatura. O resultado ainda não foi dito.',
    );
    expect(cuttingView(cuttingOffer({ rollerLabel: '' })).title).toBe('Uma criatura faz um teste');
  });

  it('never carries the d20, the bonus, the total or a DC, and no clock or "Sem efeito"', () => {
    const v = cuttingView(cuttingOffer());
    const all = JSON.stringify(v);
    expect(all).not.toMatch(/d20|bônus|total do|CD \d|Sem efeito|segundos| s\b/);
  });

  it("asks for the face of the bard's own die when the table rolls physical dice", () => {
    const v = cuttingView(cuttingOffer());
    expect(v.fieldLabel).toBe('Número que saiu no seu d8 (1 a 8)');
    expect(v.fieldHint).toBe(
      'Role o seu d8 e digite o que saiu. Para deixar passar, não precisa rolar.',
    );
    expect(
      cuttingView(
        cuttingOffer({ question: { case: 'cuttingWords', value: { sides: 6, usesLeft: 1 } } }),
      ).fieldLabel,
    ).toBe('Número que saiu no seu d6 (1 a 6)');
  });
});

describe('cutLabel and typedFace', () => {
  it('writes the number on the button once it is typed', () => {
    expect(cutLabel(null)).toBe('Cortar');
    expect(cutLabel(4)).toBe('Cortar com 4');
  });

  it('takes a face from 1 to the die', () => {
    expect(typedFace('4', 8)).toBe(4);
    expect(typedFace('8', 8)).toBe(8);
    expect(typedFace('9', 8)).toBeNull();
    expect(typedFace('0', 8)).toBeNull();
    expect(typedFace('x', 8)).toBeNull();
  });
});

describe('resultView', () => {
  const die = create(DiceRollSchema, { diceCount: 1, diceSides: 8, faces: [4], total: 4 });

  it("says what the bard cut and what is left, and never the creature's roll", () => {
    const v = resultView(
      cuttingOffer(),
      create(CuttingWordsAnsweredSchema, { cut: true, die, usesLeft: 2 }),
    );
    expect(v.title).toBe('Palavras de Interrupção usadas');
    expect(v.subtitle).toBe('O teste de Capitão bandido · fora de combate');
    expect(v.tag).toBe('Você cortou: −4');
    expect(v.face).toBe(4);
    expect(v.text).toBe('O seu d8 saiu 4 e foi tirado do teste de Capitão bandido.');
    expect(v.chips).toEqual(['Gastou 1 Inspiração de Bardo', 'Restam 2']);
    expect(v.note).toBe('O total do teste e o resultado ficam com o mestre, que conta na cena.');
  });

  it('reads "da criatura" when the bard did not know the name', () => {
    const v = resultView(
      cuttingOffer({ rollerLabel: 'uma criatura' }),
      create(CuttingWordsAnsweredSchema, { cut: true, die, usesLeft: 0 }),
    );
    expect(v.text).toBe('O seu d8 saiu 4 e foi tirado do teste da criatura.');
    expect(v.subtitle).toBe('O teste da criatura · fora de combate');
    expect(v.chips[1]).toBe('Restam 0');
  });
});
