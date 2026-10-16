import { type FormKind, FORM_KINDS, protoKindOf } from '../../../core/puzzles/puzzle-draft';
import { kindIcon, kindName } from '../../../core/puzzles/puzzle-format';
import type { PickOption } from '../fields/pick-group';

/** What each kind says of itself under its name, in the kind picker. */
const KIND_SUB: Readonly<Record<FormKind, string>> = {
  lights: 'Uma grade de luzes. Cada toque troca a luz e as quatro vizinhas.',
  lock: 'Rodas de dígitos, letras ou runas. Você escolhe a solução.',
  pillars: 'Pilares com símbolos que giram, para copiar um mural.',
  riddle: 'Um enigma que os jogadores respondem por escrito.',
  sequence: 'Sinos que você toca e os jogadores repetem, na mesma ordem.',
  cipher: 'Uma mensagem de letras trocadas, que eles decifram.',
};

export const KIND_OPTIONS: readonly PickOption<FormKind>[] = FORM_KINDS.map((kind) => ({
  value: kind,
  title: kindName(protoKindOf(kind)),
  icon: kindIcon(protoKindOf(kind)),
  sub: KIND_SUB[kind],
}));
