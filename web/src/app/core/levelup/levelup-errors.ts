import { Code, ConnectError } from '@connectrpc/connect';

import {
  CharacterBlockedReason,
  CharacterBlockedSchema,
  LevelUpRefusalReason,
  LevelUpRefusalSchema,
  type LevelUpRefusal,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { describeConnectError } from '../connect/connect-errors';
import { formatInt, tight } from '../format/text';
import type { StepKey } from './levelup-flow';

/** The step a refused rule belongs to, so the message can send the player there. */
export function refusalStep(reason: LevelUpRefusalReason, field = ''): StepKey | null {
  switch (reason) {
    case LevelUpRefusalReason.ARCHIVED_CHOICE:
    case LevelUpRefusalReason.SWITCHED_OFF_CHOICE:
      // A retired option sits in the step of the field it is chosen in.
      return /class|feature_choice/.test(field) ? 'picks' : /spell|cantrip/.test(field) ? 'spells' : null;
    case LevelUpRefusalReason.ABILITY_NOT_DUE:
    case LevelUpRefusalReason.ABILITY_SHAPE:
    case LevelUpRefusalReason.ABILITY_ABOVE_20:
      return 'abilities';
    case LevelUpRefusalReason.HIT_POINTS:
    case LevelUpRefusalReason.HIT_POINTS_RULE:
    case LevelUpRefusalReason.HIT_POINT_ROLL_MISSING:
    case LevelUpRefusalReason.HIT_POINT_ROLL_OTHER_CLASS:
      return 'hp';
    case LevelUpRefusalReason.SUBCLASS:
    case LevelUpRefusalReason.FEATURE_CHOICE:
    case LevelUpRefusalReason.SKILLS:
    case LevelUpRefusalReason.EXPERTISE:
      return 'picks';
    case LevelUpRefusalReason.CANTRIPS:
    case LevelUpRefusalReason.SPELLS:
    case LevelUpRefusalReason.PREPARED:
      return 'spells';
    default:
      return null;
  }
}

/** What a rule the level broke says, by its reason (never by the server's message). */
export function refusalMessage(refusal: Pick<LevelUpRefusal, 'reason'>): string {
  switch (refusal.reason) {
    case LevelUpRefusalReason.ARCHIVED_CHOICE:
      return 'O mestre arquivou uma das opções que você escolheu, e ela não vale mais como escolha nova. Volte e escolha outra.';
    case LevelUpRefusalReason.SWITCHED_OFF_CHOICE:
      return 'O mestre desligou uma das opções que você escolheu para os jogadores. Volte e escolha outra.';
    case LevelUpRefusalReason.CLASS:
      return 'Esse nível não vale para a classe escolhida. Volte para a ficha e comece de novo.';
    case LevelUpRefusalReason.MAX_LEVEL:
      return 'Este personagem já está no nível máximo.';
    case LevelUpRefusalReason.LOCKED_FIELD:
      return 'Algo que o nível não muda ficou diferente na ficha. Volte para a ficha e comece de novo.';
    case LevelUpRefusalReason.ABILITY_NOT_DUE:
      return 'Este nível não dá incremento no valor de habilidade.';
    case LevelUpRefusalReason.ABILITY_SHAPE:
      return 'O aumento é de +2 em uma habilidade, ou de +1 em duas.';
    case LevelUpRefusalReason.ABILITY_ABOVE_20:
      return 'Nenhuma habilidade passa de 20. Escolha outra.';
    case LevelUpRefusalReason.HIT_POINTS:
      return 'O resultado do dado de vida está fora do que o dado permite.';
    case LevelUpRefusalReason.HIT_POINTS_RULE:
      return 'A mesa decidiu como se ganham os pontos de vida do nível: use o jeito que ela deixa.';
    case LevelUpRefusalReason.SUBCLASS:
      return 'Escolha a subclasse do nível.';
    case LevelUpRefusalReason.CANTRIPS:
      return 'Escolha todos os truques novos do nível, nem mais nem menos.';
    case LevelUpRefusalReason.SPELLS:
      return 'Escolha todas as magias novas do nível, nem mais nem menos.';
    case LevelUpRefusalReason.PREPARED:
      return 'As magias preparadas só ganham vagas novas: nenhuma sai.';
    case LevelUpRefusalReason.FEATURE_CHOICE:
      return 'Escolha todas as opções das novas características.';
    case LevelUpRefusalReason.SKILLS:
      return 'Escolha todas as perícias novas do nível.';
    case LevelUpRefusalReason.EXPERTISE:
      return 'Escolha todas as especializações novas do nível.';
    case LevelUpRefusalReason.SHEET_ISSUE:
      return 'Alguma das escolhas não vale para a ficha, como uma magia fora da lista da classe ou preparada demais. Confira as magias.';
    case LevelUpRefusalReason.HIT_POINT_ROLL_MISSING:
      return 'Role o dado de vida antes de confirmar.';
    case LevelUpRefusalReason.HIT_POINT_ROLL_OTHER_CLASS:
      return 'O dado deste nível já foi rolado para outra classe.';
    case LevelUpRefusalReason.SHEET_NEEDS_MASTER:
      return 'A ficha tem um problema que nenhuma escolha resolve. Peça ao mestre para corrigir a ficha.';
    default:
      return 'As regras não aceitaram as escolhas. Confira cada passo.';
  }
}

/** Why the character cannot go through the flow (or roll its die) now. */
export function blockedMessage(reason: CharacterBlockedReason | undefined): string {
  switch (reason) {
    case CharacterBlockedReason.CANNOT_LEVEL_UP:
      return 'Este personagem não pode subir de nível agora: o mestre ainda não marcou um marco, ou o XP não chegou.';
    case CharacterBlockedReason.CHARACTER_DEAD:
      return 'Esse personagem está morto e não sobe de nível.';
    case CharacterBlockedReason.AWAITING_APPROVAL:
      return 'Esse personagem ainda espera a aprovação do mestre.';
    case CharacterBlockedReason.DICE_FORCED_PHYSICAL:
      return 'Neste jogo todos rolam os próprios dados: digite o número que saiu.';
    case CharacterBlockedReason.DICE_FORCED_IN_APP:
      return 'Neste jogo todos rolam no app: use "Rolar no app".';
    case CharacterBlockedReason.HIT_POINTS_AVERAGE_ONLY:
      return 'A mesa usa a média nos pontos de vida: o dado não é rolado.';
    default:
      return 'Não foi possível concluir a ação agora.';
  }
}

/** What a failed call of the flow means for the screen. */
export type LevelUpFailure =
  /** The sheet changed under the player: read it again. */
  | { readonly kind: 'stale'; readonly message: string }
  /** The rules refused the choices, with the step to go to. */
  | { readonly kind: 'refusal'; readonly message: string; readonly step: StepKey | null }
  /** The character cannot level up (any more): no choice fixes it. */
  | { readonly kind: 'blocked'; readonly message: string; readonly reason: CharacterBlockedReason | undefined }
  | { readonly kind: 'other'; readonly message: string };

/**
 * Sorts a failed call of the guided level-up by its code and typed detail:
 * `aborted` is a stale revision, `failed_precondition` carries either the rule
 * that was broken (`LevelUpRefusal`) or why the character cannot level up
 * (`CharacterBlocked`), and the rest are plain codes.
 */
export function describeLevelUpFailure(err: unknown): LevelUpFailure {
  const e = ConnectError.from(err, Code.Unavailable);
  if (e.code === Code.Aborted) {
    return {
      kind: 'stale',
      message: 'A ficha mudou enquanto você escolhia: o mestre mexeu nela ou entrou XP. Leia a ficha de novo e confirme.',
    };
  }
  if (e.code === Code.FailedPrecondition) {
    const [refusal] = e.findDetails(LevelUpRefusalSchema);
    if (refusal) {
      return { kind: 'refusal', message: refusalMessage(refusal), step: refusalStep(refusal.reason, refusal.field) };
    }
    const [blocked] = e.findDetails(CharacterBlockedSchema);
    return { kind: 'blocked', message: blockedMessage(blocked?.reason), reason: blocked?.reason };
  }
  return {
    kind: 'other',
    message: describeConnectError(e, {
      [Code.PermissionDenied]: 'Só o jogador dono do personagem sobe o nível. O mestre edita a ficha.',
      [Code.NotFound]: 'Esse personagem não existe, ou você não pode vê-lo.',
      [Code.InvalidArgument]: 'Alguma escolha não vale. Confira cada passo.',
    }),
  };
}

/**
 * Why a character cannot level up now, in words that say what is missing: a milestone the master has not marked, or
 * the XP still to come, from the sheet's own numbers (the campaign's mode is known). A character at level 20 is done.
 */
export function cannotLevelUpMessage(mode: XpMode, xp: number, nextLevelXp: number, level: number): string {
  if (level >= 20) {
    return 'Este personagem já está no nível máximo.';
  }
  if (mode === XpMode.MILESTONES) {
    return 'Falta o mestre marcar um marco para este personagem. Quando ele marcar, o botão aparece na ficha.';
  }
  if (nextLevelXp > xp) {
    return tight(`Faltam ${formatInt(nextLevelXp - xp)} XP para o nível ${level + 1} (${formatInt(nextLevelXp)} XP). Quando o XP chegar, o botão aparece na ficha.`);
  }
  return blockedMessage(CharacterBlockedReason.CANNOT_LEVEL_UP);
}
