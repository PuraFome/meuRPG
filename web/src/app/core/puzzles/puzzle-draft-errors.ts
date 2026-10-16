import {
  ANSWER_MAX,
  CIPHER_MESSAGE_MAX,
  CLUE_MAX,
  DC_MAX,
  DC_MIN,
  type Draft,
  HINT_MAX,
  KEYWORD_MAX,
  KEYWORD_MIN,
  MESSAGE_MAX,
  MINUTES_MAX,
  MOVES_LIMIT_MAX,
  NAME_MAX,
  PART_MAX,
  RIDDLE_MAX,
  STEPS_MAX,
  STEPS_MIN,
} from './puzzle-draft-model';
import { fold, hasCipherLetter, keywordLetters, keywordSwapsNothing } from './puzzle-text';

/** Characters as the server counts them: code points, not UTF-16 units. */
export function textLength(text: string): number {
  return [...text].length;
}

/** What is wrong in a draft, by field, in words. Empty when it can be sent. The server stays the authority. */
export interface DraftErrors {
  name?: string;
  clue?: string;
  /** By hint's index. */
  hints: Record<number, string>;
  message?: string;
  target?: string;
  lock?: string;
  riddle?: string;
  /** The answers' list as a whole; each answer's own problem is in `answerRows`. */
  answers?: string;
  answerRows: Record<number, string>;
  sequence?: string;
  cipherMessage?: string;
  cipherKey?: string;
  /** The skill check ("Escolha a perícia e a CD"). */
  check?: string;
  /** By part's index. */
  parts: Record<number, { owner?: string; text?: string }>;
  /** "Ao errar": what is missing for the chosen option. */
  wrong?: string;
}

function nameError(draft: Draft): string | undefined {
  const name = textLength(draft.name.trim());
  if (name === 0) {
    return 'Dê um nome ao quebra-cabeça.';
  }
  return name > NAME_MAX ? `O nome passa de ${NAME_MAX} letras: tem ${name}.` : undefined;
}

function clueError(draft: Draft): string | undefined {
  const n = textLength(draft.clue.trim());
  return n > CLUE_MAX ? `A pista passa de ${CLUE_MAX} letras: tem ${n}.` : undefined;
}

function hintErrors(draft: Draft, errors: DraftErrors): void {
  draft.hints.forEach((hint, i) => {
    const n = textLength(hint.trim());
    if (n === 0) {
      errors.hints[i] = 'Escreva a dica, ou tire esta.';
    } else if (n > HINT_MAX) {
      errors.hints[i] = `A dica passa de ${HINT_MAX} letras: tem ${n}.`;
    }
  });
}

function messageError(draft: Draft): string | undefined {
  const n = textLength(draft.solve.message.trim());
  return n > MESSAGE_MAX ? `A mensagem passa de ${MESSAGE_MAX} letras: tem ${n}.` : undefined;
}

/** What "Ao resolver" still needs to know about its target. */
function targetError(solve: Draft['solve']): string | undefined {
  switch (solve.choice) {
    case 'door':
      return solve.mapId === '' || solve.col < 0 || solve.row < 0
        ? 'Escolha a porta que se abre.'
        : undefined;
    case 'point':
      return solve.mapId === '' || solve.pointId === ''
        ? 'Escolha o ponto que aparece no mapa.'
        : undefined;
    case 'clue':
      return solve.clueId === '' ? 'Escolha a pista que vai a quem resolver.' : undefined;
    default:
      return undefined;
  }
}

function lockError(draft: Draft): string | undefined {
  return draft.kind === 'lock' && draft.lockStart.every((v, i) => v === draft.lockSolution[i])
    ? 'O começo é igual à solução. Gire uma roda para o começo ser outro.'
    : undefined;
}

export function draftErrors(draft: Draft): DraftErrors {
  const errors: DraftErrors = { hints: {}, answerRows: {}, parts: {} };
  const name = nameError(draft);
  if (name) {
    errors.name = name;
  }
  const clue = clueError(draft);
  if (clue) {
    errors.clue = clue;
  }
  hintErrors(draft, errors);
  const message = messageError(draft);
  if (message) {
    errors.message = message;
  }
  const target = targetError(draft.solve);
  if (target) {
    errors.target = target;
  }
  const lock = lockError(draft);
  if (lock) {
    errors.lock = lock;
  }
  if (draft.kind === 'riddle') {
    riddleErrors(draft, errors);
  } else if (draft.kind === 'sequence') {
    errors.sequence = sequenceError(draft);
  } else if (draft.kind === 'cipher') {
    cipherErrors(draft, errors);
  }
  errors.check = hintCheckError(draft);
  partsErrors(draft, errors);
  errors.wrong = wrongError(draft);
  return errors;
}

function riddleErrors(draft: Draft, errors: DraftErrors): void {
  const text = textLength(draft.riddleText.trim());
  if (text === 0) {
    errors.riddle = 'Escreva o enigma que os jogadores vão ler.';
  } else if (text > RIDDLE_MAX) {
    errors.riddle = `O enigma passa de ${RIDDLE_MAX} letras: tem ${text}.`;
  }
  if (draft.answers.length === 0) {
    errors.answers = 'Escreva pelo menos uma resposta aceita.';
  }
  const seen = new Set<string>();
  draft.answers.forEach((answer, i) => {
    const n = textLength(answer.trim());
    const folded = fold(answer);
    if (n > ANSWER_MAX) {
      errors.answerRows[i] = `Esta resposta passa de ${ANSWER_MAX} letras: tem ${n}.`;
    } else if (folded === '') {
      errors.answerRows[i] = 'Esta resposta não tem nenhuma letra ou número.';
    } else if (seen.has(folded)) {
      errors.answerRows[i] =
        'Esta resposta é igual a outra: maiúsculas, acentos e pontuação não contam.';
    }
    seen.add(folded);
  });
}

/** What is wrong with a sequence's steps, in words, or `undefined`. */
export function sequenceError(draft: Draft): string | undefined {
  if (draft.steps.length < STEPS_MIN) {
    return `Faltam passos: a sequência vai de ${STEPS_MIN} a ${STEPS_MAX}, e tem ${draft.steps.length}.`;
  }
  if (new Set(draft.steps).size < 2) {
    return 'Use pelo menos dois sinos diferentes: um sino só, tocado várias vezes, não é uma sequência.';
  }
  return undefined;
}

function cipherErrors(draft: Draft, errors: DraftErrors): void {
  const n = textLength(draft.cipherMessage.trim());
  if (n === 0) {
    errors.cipherMessage = 'Escreva a mensagem que os jogadores vão decifrar.';
  } else if (n > CIPHER_MESSAGE_MAX) {
    errors.cipherMessage = `A mensagem passa de ${CIPHER_MESSAGE_MAX} letras: tem ${n}.`;
  } else if (!hasCipherLetter(draft.cipherMessage)) {
    errors.cipherMessage = 'A mensagem precisa de pelo menos uma letra, de A a Z, para trocar.';
  }
  if (draft.cipherMethod === 'keyword') {
    const letters = keywordLetters(draft.keyword);
    if (letters.length < KEYWORD_MIN || letters.length > KEYWORD_MAX) {
      errors.cipherKey = `A palavra-chave precisa de ${KEYWORD_MIN} a ${KEYWORD_MAX} letras diferentes: tem ${letters.length}.`;
    } else if (keywordSwapsNothing(letters)) {
      errors.cipherKey = 'Essa palavra-chave não troca nenhuma letra. Escolha outra.';
    }
  }
}

function hintCheckError(draft: Draft): string | undefined {
  const check = draft.hintCheck;
  const dcText = check.dcText.trim();
  if (check.skillKey === '' && dcText === '') {
    return undefined;
  }
  if (check.skillKey === '') {
    return 'Escolha a perícia do teste, ou limpe a CD.';
  }
  const dc = /^\d{1,3}$/.test(dcText) ? Number(dcText) : NaN;
  if (!(dc >= DC_MIN && dc <= DC_MAX)) {
    return `A CD vai de ${DC_MIN} a ${DC_MAX}.`;
  }
  if (draft.hints.every((h) => h.trim() === '')) {
    return 'O teste dá dicas: escreva pelo menos uma dica acima.';
  }
  return undefined;
}

function partsErrors(draft: Draft, errors: DraftErrors): void {
  const owners = new Set<string>();
  draft.parts.forEach((part, i) => {
    const row: { owner?: string; text?: string } = {};
    const n = textLength(part.text.trim());
    if (n === 0) {
      row.text = 'Escreva a parte, ou tire esta.';
    } else if (n > PART_MAX) {
      row.text = `A parte passa de ${PART_MAX} letras: tem ${n}.`;
    }
    if (part.characterId !== '') {
      if (owners.has(part.characterId)) {
        row.owner = 'Cada personagem lê uma parte só.';
      }
      owners.add(part.characterId);
    }
    if (row.owner || row.text) {
      errors.parts[i] = row;
    }
  });
}

/** The field of "Ao errar" a refusal belongs to, from the draft (the client's own check) or from the server's `on_wrong.…` field name. */
export type WrongTarget = 'trap' | 'attempts' | 'moves' | 'minutes' | 'both' | '';

export function wrongTargetOf(field: string): WrongTarget {
  switch (field) {
    case 'on_wrong.trap':
      return 'trap';
    case 'on_wrong.attempts_per_player':
      return 'attempts';
    case 'on_wrong.max_moves':
      return 'moves';
    case 'on_wrong.time_limit_seconds':
      return 'minutes';
    default:
      return '';
  }
}

/** Which field of the chosen option is wrong, for the form's own check. */
export function wrongTarget(draft: Draft): WrongTarget {
  const wrong = draft.wrong;
  if (wrong.option === 'trap') {
    return wrong.mapId === '' || wrong.pointId === '' ? 'trap' : '';
  }
  if (wrong.option !== 'limits') {
    return '';
  }
  const moves = wrong.movesText.trim();
  const minutes = wrong.minutesText.trim();
  if (moves === '' && minutes === '') {
    return 'both';
  }
  if (
    moves !== '' &&
    !(/^\d{1,4}$/.test(moves) && Number(moves) >= 1 && Number(moves) <= MOVES_LIMIT_MAX)
  ) {
    return 'moves';
  }
  if (
    minutes !== '' &&
    !(/^\d{1,4}$/.test(minutes) && Number(minutes) >= 1 && Number(minutes) <= MINUTES_MAX)
  ) {
    return 'minutes';
  }
  return '';
}

function wrongError(draft: Draft): string | undefined {
  const wrong = draft.wrong;
  if (wrong.option === 'trap' && (wrong.mapId === '' || wrong.pointId === '')) {
    return 'Escolha a armadilha do mapa que dispara.';
  }
  if (wrong.option !== 'limits') {
    return undefined;
  }
  const moves = wrong.movesText.trim();
  const minutes = wrong.minutesText.trim();
  if (moves === '' && minutes === '') {
    return 'Ponha um limite de jogadas, de minutos, ou os dois.';
  }
  if (
    moves !== '' &&
    !(/^\d{1,4}$/.test(moves) && Number(moves) >= 1 && Number(moves) <= MOVES_LIMIT_MAX)
  ) {
    return `O limite de jogadas vai de 1 a ${MOVES_LIMIT_MAX}.`;
  }
  if (
    minutes !== '' &&
    !(/^\d{1,4}$/.test(minutes) && Number(minutes) >= 1 && Number(minutes) <= MINUTES_MAX)
  ) {
    return `O limite de tempo vai de 1 a ${MINUTES_MAX} minutos.`;
  }
  return undefined;
}

/** Whether a draft has nothing to fix. */
export function isValid(errors: DraftErrors): boolean {
  return (
    !errors.name &&
    !errors.clue &&
    !errors.message &&
    !errors.target &&
    !errors.lock &&
    !errors.riddle &&
    !errors.answers &&
    !errors.sequence &&
    !errors.cipherMessage &&
    !errors.cipherKey &&
    !errors.check &&
    !errors.wrong &&
    Object.keys(errors.hints).length === 0 &&
    Object.keys(errors.answerRows).length === 0 &&
    Object.keys(errors.parts).length === 0
  );
}
