import { create } from '@bufbuild/protobuf';
import { vi } from 'vitest';

import {
  AreaPlacement,
  CoverDegree,
  CoverSource,
  DartsAtSlotSchema,
  SpellTargetsSchema,
  TargetInReachSchema,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  ContestAttackOptionKind,
  ContestAttackOptionSchema,
  ContestTargetReason,
  ContestTargetSchema,
} from '../../../../../gen/meurpg/play/v1/contest_types_pb';
import { AttackSchema, SlotChoiceSchema } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import type { AttackSheetData } from '../attack-sheet/attack-sheet-types';
import type { CastSheetData } from '../cast-sheet/cast-sheet-types';
import { TargetFlow, type TargetFlowDeps } from './target-flow';
import { TargetMode } from './target-mode';

const who = (id: string, over: Record<string, unknown> = {}) =>
  create(TargetInReachSchema, { combatantId: id, label: `Goblin ${id}`, ...over });

function flow(over: Partial<TargetFlowDeps> = {}) {
  const reveal = vi.fn();
  const mode = new TargetMode((run) => run());
  const deps: TargetFlowDeps = {
    onMap: () => true,
    placed: () => true,
    reveal,
    origin: () => null,
    ...over,
  };
  return { flow: new TargetFlow(mode, deps), mode, reveal };
}

const attackData = (targets = [who('1'), who('2', { tooFar: true })]) =>
  ({
    attack: create(AttackSchema, {
      key: 'spell:fire-bolt',
      name: 'Fire Bolt',
      namePt: 'Raio de Fogo',
      attackBonus: 5,
      damage: '1d10',
      damageTypePt: 'fogo',
      rangeFt: 120,
    }),
    targets,
  }) as unknown as AttackSheetData;

const castData = (over: Record<string, unknown> = {}) =>
  ({
    spellKey: 'spell:magic-missile',
    name: 'Mísseis Mágicos',
    casterId: 'me',
    level: 1,
    cantripDice: '',
    attackBonus: 0,
    slots: [create(SlotChoiceSchema, { level: 1, free: 2 })],
    usage: [],
    pact: null,
    map: null,
    targets: create(SpellTargetsSchema, {
      spellKey: 'spell:magic-missile',
      targets: [who('1'), who('2'), who('me', { label: 'Pensantus' })],
      darts: [create(DartsAtSlotSchema, { slotLevel: 1, darts: 3 })],
    }),
    ...over,
  }) as unknown as CastSheetData;

describe('TargetFlow: an attack', () => {
  it('starts the choice on the map, brings the map into view and dims what the server leaves out', () => {
    const { flow: f, mode, reveal } = flow();
    expect(f.attack(attackData(), vi.fn())).toBe(true);
    expect(reveal).toHaveBeenCalled();
    expect(mode.session()?.name).toBe('Raio de Fogo');
    expect(mode.session()?.detail).toBe('+5 para acertar · 1d10 de fogo');
    expect(mode.marks()?.map((m) => m.reason)).toEqual(['', 'fora de alcance']);
  });

  it('opens the sheet with the tapped target, or with none for the list', () => {
    const open = vi.fn();
    const { flow: f, mode } = flow();
    f.attack(attackData(), open);
    mode.tap('1');
    mode.confirm();
    expect(open).toHaveBeenLastCalledWith('1');
    f.attack(attackData(), open);
    mode.list();
    expect(open).toHaveBeenLastCalledWith(null);
  });

  it('keeps the sheet when there is no map, nobody to tap or nobody on the map', () => {
    expect(flow({ onMap: () => false }).flow.attack(attackData(), vi.fn())).toBe(false);
    expect(flow().flow.attack(attackData([]), vi.fn())).toBe(false);
    expect(
      flow().flow.attack(
        attackData([
          who('1', { untargetable: true, cover: CoverDegree.TOTAL, coverSource: CoverSource.MAP }),
        ]),
        vi.fn(),
      ),
    ).toBe(false);
    expect(flow({ placed: () => false }).flow.attack(attackData(), vi.fn())).toBe(false);
  });
});

describe('TargetFlow: a spell', () => {
  it('hands out the darts of the lowest slot, a tap each, and gives the sheet the count of each target', () => {
    const open = vi.fn();
    const { flow: f, mode } = flow();
    expect(f.cast(castData(), 'darts', open)).toBe(true);
    expect(mode.session()?.rule).toMatchObject({ kind: 'darts', max: 3 });
    expect(mode.marks()?.map((m) => m.id)).toEqual(['1', '2']);
    mode.tap('1');
    mode.tap('1');
    mode.tap('2');
    mode.confirm();
    expect(open).toHaveBeenCalledWith({
      chosen: [],
      dealt: new Map([
        ['1', 2],
        ['2', 1],
      ]),
    });
  });

  it('chooses one creature for a spell with one target', () => {
    const open = vi.fn();
    const { flow: f, mode } = flow();
    const data = castData({
      spellKey: 'spell:cure-wounds',
      targets: create(SpellTargetsSchema, {
        spellKey: 'spell:cure-wounds',
        maxTargets: 1,
        targets: [who('1'), who('me', { label: 'Pensantus' })],
      }),
    });
    expect(f.cast(data, 'heal', open)).toBe(true);
    expect(mode.marks()?.map((m) => m.label)).toEqual(['Goblin 1', 'Pensantus (você)']);
    expect(mode.session()?.foe).toBe(false);
    mode.tap('me');
    mode.confirm();
    expect(open).toHaveBeenCalledWith({ chosen: ['me'], dealt: new Map() });
  });

  it('keeps the sheet for an area, a teleport and a spell on the caster', () => {
    const { flow: f } = flow();
    const area = create(SpellTargetsSchema, {
      spellKey: 'spell:fireball',
      maxTargets: 0,
      placement: AreaPlacement.POINT,
      targets: [who('1')],
    });
    expect(f.cast(castData({ targets: area }), 'save', vi.fn())).toBe(false);
    expect(f.cast(castData({ targets: undefined }), 'plain', vi.fn())).toBe(false);
    const self = create(SpellTargetsSchema, {
      spellKey: 'spell:shield',
      maxTargets: 0,
      targets: [who('me')],
    });
    expect(f.cast(castData({ targets: self }), 'plain', vi.fn())).toBe(false);
    const jump = create(SpellTargetsSchema, {
      spellKey: 'spell:misty-step',
      maxTargets: 1,
      teleportRangeFt: 30,
      targets: [who('1')],
    });
    expect(f.cast(castData({ targets: jump }), 'plain', vi.fn())).toBe(false);
  });
});

describe('TargetFlow: a grapple or a shove', () => {
  it('offers the creatures at reach, dims the ones that are too big and opens the sheet with the tapped one', () => {
    const open = vi.fn();
    const option = create(ContestAttackOptionSchema, {
      kind: ContestAttackOptionKind.GRAPPLE,
      targets: [
        create(ContestTargetSchema, { combatantId: '1', eligible: true }),
        create(ContestTargetSchema, { combatantId: '2', eligible: false }),
        create(ContestTargetSchema, {
          combatantId: '3',
          eligible: true,
          reason: ContestTargetReason.OUT_OF_REACH,
        }),
      ],
    });
    const { flow: f, mode } = flow();
    expect(f.contest(option, 'Agarrar', (id) => `Goblin ${id}`, open)).toBe(true);
    expect(mode.marks()?.map((m) => [m.id, m.reason])).toEqual([
      ['1', ''],
      ['2', 'grande demais'],
    ]);
    mode.tap('1');
    mode.confirm();
    expect(open).toHaveBeenCalledWith('1');
  });
});
