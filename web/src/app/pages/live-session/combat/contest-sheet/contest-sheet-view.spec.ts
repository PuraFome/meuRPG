import { create } from '@bufbuild/protobuf';

import {
  ContestAttackOptionSchema,
  ContestPurpose,
  ContestStatus,
  ContestTargetReason,
  ContestTargetSchema,
  ManeuverGrappleOptionSchema,
  RollModeKind,
} from '../../../../../gen/meurpg/play/v1/contest_types_pb';
import { CreatureSize } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { combatant } from '../../../../core/combat/combat-testing';
import {
  checkOf,
  limitTextOf,
  maneuverWhy,
  stageOf,
  subtitleOf,
  targetRowsOf,
  titleOf,
  whyOf,
} from './contest-sheet-view';

describe('contest sheet view', () => {
  it('reads what a check rolls, with defaults', () => {
    expect(checkOf(undefined)).toEqual({ modifier: 0, mode: RollModeKind.NORMAL, notes: [] });
    expect(
      checkOf({ modifier: 3, mode: RollModeKind.ADVANTAGE, notes: [] } as never).modifier,
    ).toBe(3);
  });

  it('puts a contest on its stage', () => {
    expect(stageOf({ status: ContestStatus.RESOLVED } as never)).toBe('result');
    expect(stageOf({ status: ContestStatus.AWAITING_OUTCOME, youChoose: true } as never)).toBe(
      'choice',
    );
    expect(stageOf({ status: ContestStatus.AWAITING_OUTCOME, youChoose: false } as never)).toBe(
      'wait',
    );
  });

  it('lists the targets that are at reach, naming the ones too big or unknown', () => {
    const attack = create(ContestAttackOptionSchema, {
      targets: [
        create(ContestTargetSchema, {
          combatantId: 'a',
          size: CreatureSize.MEDIUM,
          distanceFt: 5,
          eligible: true,
        }),
        create(ContestTargetSchema, { combatantId: 'b', eligible: false }),
        create(ContestTargetSchema, { combatantId: 'c', reason: ContestTargetReason.OUT_OF_REACH }),
        create(ContestTargetSchema, { combatantId: 'd', reason: ContestTargetReason.NOT_PLACED }),
      ],
    });
    const rows = targetRowsOf(attack, [combatant({ id: 'a', label: 'Hobgoblin' })]);
    expect(rows.map((r) => r.id)).toEqual(['a', 'b']);
    expect(rows[0].label).toBe('Hobgoblin');
    expect(rows[1].label).toBe('Alguém');
    expect(rows[1].blocked).toContain('Grande demais');
    expect(targetRowsOf(undefined, [])).toEqual([]);
  });

  it('words the limit, why the roll is off, and the maneuvers', () => {
    expect(limitTextOf('')).toBe('Alvo: no máximo um tamanho acima do seu, ao alcance.');
    expect(limitTextOf('Médio')).toContain('(Médio)');
    expect(whyOf(true, false, true)).toBe('Confirme que tem uma mão livre');
    expect(whyOf(false, false, false)).toBe('Escolha o alvo');
    expect(whyOf(true, true, true)).toBe('');
    const m = (over: object) =>
      create(ManeuverGrappleOptionSchema, { sides: 8, usesLeft: 1, ...over });
    expect(maneuverWhy(m({ enabled: true }))).toBe('d8 · 1 uso · ação bônus');
    expect(maneuverWhy(m({ enabled: true, usesLeft: 2 }))).toContain('2 usos');
    expect(maneuverWhy(m({ noMeleeHit: true }))).toContain('acerto corpo a corpo');
    expect(maneuverWhy(m({}))).toBe('');
  });

  it('titles and subtitles the sheet by purpose and stage', () => {
    const other = { label: 'Hobgoblin', player: false };
    expect(titleOf(true, ContestPurpose.ESCAPE, 'roll', other)).toBe('Escapar');
    expect(titleOf(false, ContestPurpose.GRAPPLE, 'target', other)).toBe('Agarrar');
    expect(titleOf(false, ContestPurpose.GRAPPLE, 'roll', other)).toBe('Agarrar o Hobgoblin');
    expect(subtitleOf(true, 'roll')).toBe('Ação');
    expect(subtitleOf(false, 'target')).toBe('Ação · substitui um ataque');
    expect(subtitleOf(false, 'choice')).toBe('Resultado');
    expect(subtitleOf(false, 'wait')).toBe('Disputa');
  });
});
