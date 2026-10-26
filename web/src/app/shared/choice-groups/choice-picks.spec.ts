import { create } from '@bufbuild/protobuf';

import { ChoiceGroupSchema, ChoiceSchema } from '../../../gen/meurpg/characters/v1/characters_pb';
import { missingSentence, pendingLabels } from './choice-picks';

describe('the open choices of a sheet', () => {
  const choice = (labelPt: string, missing: number, advisory = false) =>
    create(ChoiceSchema, { key: labelPt, labelPt, picks: 1, missing, advisory });
  const groups = [
    create(ChoiceGroupSchema, {
      choices: [
        choice('Estilo de Luta (Guerreiro, nível 1)', 1),
        choice('Idiomas (Acólito)', 2, true),
        choice('Ancestral (Draconato)', 0),
      ],
    }),
  ];

  it('lists every open choice, the advisory ones too, for the mark on the step', () => {
    expect(pendingLabels(groups)).toEqual([
      'Estilo de Luta (Guerreiro, nível 1)',
      'Idiomas (Acólito)',
    ]);
  });

  it('says only the choices that stop the sheet under "Criar personagem"', () => {
    expect(pendingLabels(groups, false)).toEqual(['Estilo de Luta (Guerreiro, nível 1)']);
    expect(missingSentence(groups)).toBe('Falta uma escolha: Estilo de Luta (Guerreiro, nível 1).');
  });

  it('says nothing when only a language or a tool is open', () => {
    expect(
      missingSentence([
        create(ChoiceGroupSchema, { choices: [choice('Idioma (Humano)', 1, true)] }),
      ]),
    ).toBe('');
  });
});
