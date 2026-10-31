/** The sentences of the import cards that the live region says as well. The card and the region read the same text. */
const NOTHING_LEFT = 'Nenhuma campanha, imagem ou personagem ficou para trás.';
const RETRY_UNKNOWN =
  '“Tentar de novo” é seguro: se a campanha já foi criada, você recebe a mesma, sem repetir.';
const RETRY_KNOWN = '“Tentar de novo” usa as partes que já foram enviadas.';

/** The fixed sentences, for the template. */
export const SPEECH = {
  nothingLeft: NOTHING_LEFT,
  retryUnknown: RETRY_UNKNOWN,
  retryKnown: RETRY_KNOWN,
} as const;

/** The limit card: what it says, in the order it says it. */
export function capSentence(max: number): string {
  return max > 0
    ? `Você já tem ${max} campanhas. Este é o limite por conta.`
    : 'Você já tem o máximo de campanhas. Este é o limite por conta.';
}

/** The failed card: the title, the cause, that nothing was left behind when it is known, and the hint under the card. */
export function failureSentence(failure: {
  readonly title: string;
  readonly cause: string;
  readonly createdNothing: boolean;
  readonly unknown: boolean;
}): string {
  return [
    failure.title,
    failure.cause,
    failure.createdNothing ? NOTHING_LEFT : '',
    failure.unknown ? RETRY_UNKNOWN : RETRY_KNOWN,
  ]
    .filter((t) => t !== '')
    .join(' ');
}
