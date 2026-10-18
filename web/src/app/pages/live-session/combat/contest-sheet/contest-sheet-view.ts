import type { Combatant } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  type CheckOption,
  type ContestAttackOption,
  type ContestSkillOption,
  type ContestView,
  type ManeuverGrappleOption,
  type RollNote,
  ContestKind,
  ContestPurpose,
  ContestSkill,
  ContestStatus,
  ContestTargetReason,
  RollModeKind,
} from '../../../../../gen/meurpg/play/v1/contest_types_pb';
import { reasonText } from '../../../../core/combat/combat-options';
import type { CheckDie, StartContestInput } from '../../../../core/combat/contest-client';
import {
  sheetTitle,
  sheetTitleWith,
  isSettled,
  named,
  sizeWord,
  whoIs,
} from '../../../../core/combat/contest-view';
import type { Who } from '../../../../core/combat/contest-view';
import { joinDots, tight } from '../../../../core/format/text';
import { metersText } from '../../../../core/units';

/** Where the sheet is: choosing the target, rolling, waiting for the other side, choosing what a won shove does, or the result. */
export type Stage = 'target' | 'roll' | 'wait' | 'choice' | 'result';

export const GRAPPLE_STEPS = ['Alvo', 'Disputa', 'Resultado'] as const;
export const ESCAPE_STEPS = ['Teste', 'Resultado'] as const;
/** The step each stage of a grapple or a shove is on (Resultado is the last). */
export const GRAPPLE_AT: Record<Stage, number> = {
  target: 0,
  roll: 1,
  wait: 1,
  choice: 2,
  result: 2,
};
export const ESCAPE_AT: Record<Stage, number> = {
  target: 0,
  roll: 0,
  wait: 0,
  choice: 1,
  result: 1,
};

/** What a check rolls, from what the server sent for it: the modifier, the mode and the circumstances behind the mode. */
export function checkOf(option: CheckOption | ContestSkillOption | undefined): {
  modifier: number;
  mode: RollModeKind;
  notes: readonly RollNote[];
} {
  return {
    modifier: option?.modifier ?? 0,
    mode: option?.mode ?? RollModeKind.NORMAL,
    notes: option?.notes ?? [],
  };
}

/** The stage of a contest the page already knows: the result, the choice of a won shove, or the wait. */
export function stageOf(c: ContestView): Stage {
  if (isSettled(c)) {
    return 'result';
  }
  if (c.status === ContestStatus.AWAITING_OUTCOME && c.youChoose) {
    return 'choice';
  }
  return 'wait';
}

/** The creatures the player can pick (those out of reach or off the map are not listed), the ones too big dashed. */
export function targetRowsOf(
  attack: ContestAttackOption | undefined,
  people: readonly Combatant[],
) {
  return (attack?.targets ?? [])
    .filter(
      (t) =>
        t.reason !== ContestTargetReason.OUT_OF_REACH &&
        t.reason !== ContestTargetReason.NOT_PLACED,
    )
    .map((t) => {
      const c = people.find((x) => x.id === t.combatantId);
      return {
        id: t.combatantId,
        who: whoIs(c),
        label: c?.label ?? 'Alguém',
        sub: tight(
          joinDots(
            [sizeWord(t.size), t.distanceFt > 0 ? `a ${metersText(t.distanceFt)}` : ''].filter(
              Boolean,
            ),
          ),
        ),
        blocked: t.eligible ? '' : 'Grande demais: no máximo um tamanho acima do seu.',
      };
    });
}

/** "Alvo: no máximo um tamanho acima do seu (Médio), ao alcance." */
export function limitTextOf(size: string): string {
  return `Alvo: no máximo um tamanho acima do seu${size ? ` (${size})` : ''}, ao alcance.`;
}

/** Why "Rolar a disputa" is off, in words: the missing reminder first. */
export function whyOf(needsHand: boolean, free: boolean, hasTarget: boolean): string {
  if (needsHand && !free) {
    return 'Confirme que tem uma mão livre';
  }
  return hasTarget ? '' : 'Escolha o alvo';
}

/** What a maneuver says under its name: its die and uses, or why it is off. */
export function maneuverWhy(o: ManeuverGrappleOption): string {
  if (o.enabled) {
    return `d${o.sides} · ${o.usesLeft} ${o.usesLeft === 1 ? 'uso' : 'usos'} · ação bônus`;
  }
  return o.noMeleeHit ? 'Precisa de um acerto corpo a corpo neste turno.' : reasonText(o.reason);
}

/** The sheet's title: the purpose, then who it is against once the target is chosen. */
export function titleOf(
  escape: boolean,
  purpose: ContestPurpose,
  stage: Stage,
  other: Who,
): string {
  if (escape) {
    return sheetTitle(ContestPurpose.ESCAPE);
  }
  return stage === 'target' ? sheetTitle(purpose) : sheetTitleWith(purpose, named(other));
}

/** The line under the title. */
export function subtitleOf(escape: boolean, stage: Stage): string {
  if (escape) {
    return 'Ação';
  }
  if (stage === 'target') {
    return 'Ação · substitui um ataque';
  }
  return stage === 'choice' ? 'Resultado' : 'Disputa';
}

/** The request that starts the contest, as the sheet sends it: a plain grapple, shove or escape, or a grapple that a table maneuver starts. */
export function startInputOf(
  data: {
    readonly campaignId: string;
    readonly encounterId: string;
    readonly initiatorId: string;
    readonly purpose: ContestPurpose;
  },
  targetId: string,
  skill: ContestSkill,
  die: CheckDie,
  maneuverKey: string,
  maneuverFace: number,
): StartContestInput {
  return {
    campaignId: data.campaignId,
    encounterId: data.encounterId,
    initiatorId: data.initiatorId,
    targetId,
    purpose: data.purpose,
    kind: ContestKind.CONTEST,
    skill,
    die,
    ...(maneuverKey ? { maneuverKey, maneuverFace } : {}),
  };
}
