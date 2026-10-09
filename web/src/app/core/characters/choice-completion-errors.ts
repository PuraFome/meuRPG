import { Code, ConnectError } from '@connectrpc/connect';

import {
  ChoiceRefusalReason,
  ChoiceRefusalSchema,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { describeCharacterError } from './character-errors';

/**
 * What a refused "Salvar escolhas" says on the page that completes a locked sheet's open choices (PM-05): by the typed
 * `ChoiceRefusal` reason, with the label of the choice the server names, never by its message. Any other error is the
 * sheet's usual wording (`describeCharacterError`).
 */
export function describeCompletionError(err: unknown): string {
  const connectErr = ConnectError.from(err, Code.Unavailable);
  const [refusal] =
    connectErr.code === Code.FailedPrecondition ? connectErr.findDetails(ChoiceRefusalSchema) : [];
  if (!refusal) {
    return describeCharacterError(err);
  }
  const label = refusal.issues.find((i) => i.labelPt !== '')?.labelPt ?? '';
  const which = label === '' ? 'Uma escolha' : `A escolha “${label}”`;
  switch (refusal.reason) {
    case ChoiceRefusalReason.CHOICES_MISSING:
      return label === ''
        ? 'Falta terminar uma escolha: confira as opções e os textos que ela pede.'
        : `Falta terminar a escolha “${label}”: confira as opções e os textos que ela pede.`;
    case ChoiceRefusalReason.PREREQUISITE_UNMET:
      return `${which} tem uma opção que pede algo que a ficha ainda não tem. Escolha outra.`;
    case ChoiceRefusalReason.CHOICE_NOT_OFFERED:
      return `${which} tem uma opção que a ficha não oferece, ou opções demais. Confira o que foi marcado.`;
    case ChoiceRefusalReason.CHOICE_ALREADY_MADE:
      return `${which} já foi feita e não muda mais. Recarregue a página para ver o que sobrou.`;
    case ChoiceRefusalReason.CHOICE_NOT_OPEN:
      return `${which} não está mais em aberto nesta ficha. Recarregue a página para ver o que sobrou.`;
    default:
      return 'As regras não aceitaram essas escolhas. Confira cada uma.';
  }
}
