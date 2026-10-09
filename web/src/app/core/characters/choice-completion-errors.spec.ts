import { Code, ConnectError } from '@connectrpc/connect';

import {
  CharacterBlockedReason,
  CharacterBlockedSchema,
  ChoiceRefusalReason,
  ChoiceRefusalSchema,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { describeCompletionError } from './choice-completion-errors';

function refused(reason: ChoiceRefusalReason, labelPt = ''): ConnectError {
  return new ConnectError('refused', Code.FailedPrecondition, undefined, [
    { desc: ChoiceRefusalSchema, value: { reason, issues: [{ labelPt }] } },
  ]);
}

describe('describeCompletionError', () => {
  it('names the choice the server refused, by the typed reason', () => {
    const label = 'Estilo de Luta (Guerreiro, nível 1)';
    expect(describeCompletionError(refused(ChoiceRefusalReason.CHOICES_MISSING, label))).toContain(
      `“${label}”`,
    );
    expect(
      describeCompletionError(refused(ChoiceRefusalReason.PREREQUISITE_UNMET, label)),
    ).toContain('pede algo que a ficha ainda não tem');
    expect(
      describeCompletionError(refused(ChoiceRefusalReason.CHOICE_NOT_OFFERED, label)),
    ).toContain('não oferece');
    expect(
      describeCompletionError(refused(ChoiceRefusalReason.CHOICE_ALREADY_MADE, label)),
    ).toContain('já foi feita');
    expect(describeCompletionError(refused(ChoiceRefusalReason.CHOICE_NOT_OPEN, label))).toContain(
      'não está mais em aberto',
    );
  });

  it('still speaks without a label', () => {
    expect(describeCompletionError(refused(ChoiceRefusalReason.CHOICE_NOT_OPEN))).toMatch(
      /^Uma escolha não está mais em aberto/,
    );
  });

  it("leaves the sheet's own refusals to the sheet's words", () => {
    const dead = new ConnectError('blocked', Code.FailedPrecondition, undefined, [
      {
        desc: CharacterBlockedSchema,
        value: { reason: CharacterBlockedReason.CHARACTER_DEAD },
      },
    ]);
    expect(describeCompletionError(dead)).toContain('morto');
    expect(describeCompletionError(new ConnectError('x', Code.Aborted))).toContain('A ficha mudou');
  });
});
