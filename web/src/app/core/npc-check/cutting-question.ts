import { HeldRollKind, type HeldRollOffer } from '../../../gen/meurpg/play/v1/combat_pb';
import type { CuttingWordsAnswered } from '../../../gen/meurpg/play/v1/resources_pb';
import { ofName } from './npc-check-picker';
import { article } from '../format/article';

/** What the server writes in the offer when the bard's player does not know the creature (`cutting.UnnamedLabel`). */
export const UNNAMED_LABEL = 'uma criatura';

/** The bard's question of an NPC's check, ready to draw. Nothing of the creature's roll is here: the offer has none (RN-10). */
export interface CuttingViewVm {
  readonly title: string;
  readonly subtitle: string;
  readonly lead: string;
  readonly text: string;
  readonly chips: readonly string[];
  readonly notes: readonly string[];
  readonly sides: number;
  readonly fieldLabel: string;
  readonly fieldHint: string;
}

/** The bard's result after a cut. */
export interface CuttingResultVm {
  readonly title: string;
  readonly subtitle: string;
  readonly tag: string;
  readonly face: number;
  readonly text: string;
  readonly chips: readonly string[];
  readonly note: string;
}

export function isCuttingOffer(offer: HeldRollOffer | null | undefined): offer is HeldRollOffer {
  return offer?.kind === HeldRollKind.NPC_CHECK && offer.question.case === 'cuttingWords';
}

/** The creature's name the bard may read, or `''` for "uma criatura". */
function nameOf(offer: HeldRollOffer): string {
  const label = offer.rollerLabel.trim();
  return label === '' || label === UNNAMED_LABEL ? '' : label;
}

/** "do Capitão bandido", or "da criatura" when the bard does not know the name. */
function ofCreature(offer: HeldRollOffer): string {
  const name = nameOf(offer);
  return name ? ofName(name) : 'da criatura';
}

function question(offer: HeldRollOffer) {
  return offer.question.case === 'cuttingWords' ? offer.question.value : undefined;
}

export function cuttingView(offer: HeldRollOffer): CuttingViewVm {
  const q = question(offer);
  const sides = q?.sides ?? 0;
  const name = nameOf(offer);
  return {
    title: name ? `${article(name) === 'a' ? 'A' : 'O'} ${name} faz um teste` : 'Uma criatura faz um teste',
    subtitle: 'Palavras de Interrupção · fora de combate',
    lead: 'Usar Palavras de Interrupção?',
    text: `Você tira o seu dado de Inspiração de Bardo (d${sides}) do teste ${ofCreature(offer)}. O resultado ainda não foi dito.`,
    chips: ['Gasta 1 Inspiração de Bardo', `Você tem ${q?.usesLeft ?? 0}`, 'Não gasta reação'],
    notes: [
      'Qual é o teste, o total e a CD ficam com o mestre.',
      'A cena espera a sua resposta. O mestre pode seguir sem ela.',
    ],
    sides,
    fieldLabel: `Número que saiu no seu d${sides} (1 a ${sides})`,
    fieldHint: `Role o seu d${sides} e digite o que saiu. Para deixar passar, não precisa rolar.`,
  };
}

/** The button: "Cortar", and "Cortar com 4" once a face is typed. */
export function cutLabel(face: number | null): string {
  return face === null ? 'Cortar' : `Cortar com ${face}`;
}

/** The face of the bard's die the player typed, `null` when it is not a whole number from 1 to the die. */
export function typedFace(text: string, sides: number): number | null {
  const t = text.trim();
  if (!/^\d{1,3}$/.test(t)) {
    return null;
  }
  const n = Number(t);
  return n >= 1 && n <= sides ? n : null;
}

export function resultView(offer: HeldRollOffer, answered: CuttingWordsAnswered): CuttingResultVm {
  const face = answered.die?.faces[0] ?? 0;
  const sides = answered.die?.diceSides ?? question(offer)?.sides ?? 0;
  return {
    title: 'Palavras de Interrupção usadas',
    subtitle: `O teste ${ofCreature(offer)} · fora de combate`,
    tag: `Você cortou: −${face}`,
    face,
    text: `O seu d${sides} saiu ${face} e foi tirado do teste ${ofCreature(offer)}.`,
    chips: ['Gastou 1 Inspiração de Bardo', `Restam ${answered.usesLeft}`],
    note: 'O total do teste e o resultado ficam com o mestre, que conta na cena.',
  };
}
