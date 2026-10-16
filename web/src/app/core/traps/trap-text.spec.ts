import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';

import {
  MapPointKind,
  MapPointSchema,
  TrapRevealHow,
  TrapState,
} from '../../../gen/meurpg/maps/v1/maps_pb';
import {
  Ability,
  TrapEffectSchema,
  TrapPassOutcome,
  TrapSaveApplies,
  TrapTargets,
  TrapTrigger,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import {
  areaWords,
  clockOf,
  effectLine,
  effectText,
  firstLine,
  isPinKind,
  knownReason,
  trapDcLine,
  trapPoints,
  trapStateIcon,
  trapStateWord,
  trapVisibility,
  treasurePoints,
  triggerWord,
} from './trap-text';

const plain = (t: string) => t.replace(/\u00a0/g, ' ');

describe('trap text', () => {
  it('names the state, with the time of a firing', () => {
    expect(trapStateWord(TrapState.ARMED)).toBe('Armada');
    expect(plain(trapStateWord(TrapState.TRIGGERED, new Date(2026, 9, 4, 21, 31)))).toBe(
      'Disparada às 21:31',
    );
    expect(trapStateWord(TrapState.TRIGGERED)).toBe('Disparada');
    expect(trapStateWord(TrapState.DISARMED)).toBe('Desarmada');
  });

  it('says who knows the trap', () => {
    const secret = create(MapPointSchema, {
      kind: MapPointKind.TRAP,
      trap: { state: TrapState.ARMED },
    });
    expect(trapVisibility(secret).label).toBe('Só você vê');
    const some = create(MapPointSchema, {
      trapRevealedTo: [
        {
          characterId: 'a',
          characterName: 'Toren',
          how: TrapRevealHow.MASTER,
          at: timestampFromDate(new Date()),
        },
      ],
    });
    expect(trapVisibility(some)).toEqual({ kind: 'some', label: 'Só Toren sabe' });
    expect(trapVisibility(create(MapPointSchema, { revealed: true })).kind).toBe('all');
    expect(
      trapVisibility(create(MapPointSchema, { trap: { state: TrapState.TRIGGERED } })).label,
    ).toBe('Visível para todos');
  });

  it('gives the reason a character already knows it', () => {
    expect(knownReason(TrapRevealHow.SEARCHED)).toBe('já achou esta armadilha');
    expect(knownReason(TrapRevealHow.NOTICED)).toBe('já notou esta armadilha');
    expect(knownReason(TrapRevealHow.MASTER)).toBe('você já revelou');
  });

  it('writes the effect from the parts the server sent', () => {
    const effect = create(TrapEffectSchema, {
      damage: [
        { dice: '2d6', damageTypeKey: 'damage-type:bludgeoning', damageTypePt: 'concussão' },
      ],
      save: {
        ability: Ability.CONSTITUTION,
        dc: 15,
        appliesTo: TrapSaveApplies.CAUGHT,
        onPass: TrapPassOutcome.HALF,
        onFail: {
          damage: [{ dice: '2d10', damageTypeKey: 'damage-type:poison', damageTypePt: 'veneno' }],
        },
      },
      targets: TrapTargets.MANUAL,
    });
    const text = plain(effectText(effect));
    expect(text).toContain('Dano: 2d6 de concussão');
    expect(text).toContain(
      'Resistência de Constituição CD 15: 2d10 de veneno; quem passa leva metade do dano',
    );
    expect(text).toContain('Você escolhe quem é atingido.');
    expect(effectText(undefined)).toBe('');
  });

  it('draws the area words and knows the pin kinds', () => {
    expect(plain(areaWords(2))).toBe('área de 2 × 2');
    expect(areaWords(1)).toBe('um quadrado só');
    expect(isPinKind(MapPointKind.TREASURE)).toBe(true);
    expect(isPinKind(MapPointKind.SCENE)).toBe(false);
  });
});

describe('trap text, every part of an effect', () => {
  const effectOf = (init: Parameters<typeof create<typeof TrapEffectSchema>>[1]) =>
    plain(effectText(create(TrapEffectSchema, init)));

  it('writes an attack with its bonus, its count and its damage, or "sem dano"', () => {
    expect(
      effectOf({
        attack: {
          bonus: 8,
          count: 3,
          damage: { dice: '1d4', damageTypeKey: 'x', damageTypePt: 'perfurante' },
        },
      }),
    ).toBe('Ataque +8 (3 ataques): 1d4 de dano perfurante.');
    expect(effectOf({ attack: { bonus: 5, count: 1 } })).toBe('Ataque +5: sem dano.');
  });

  it('calls a damage type by its name unless it is an adjective, and "dano" when it has none', () => {
    expect(effectOf({ damage: [{ dice: '1d6', damageTypePt: 'cortante' }] })).toBe(
      'Dano: 1d6 de dano cortante.',
    );
    expect(effectOf({ damage: [{ dice: '1d6', damageTypePt: 'fogo' }] })).toBe(
      'Dano: 1d6 de fogo.',
    );
    expect(effectOf({ damage: [{ dice: '1d6' }] })).toBe('Dano: 1d6 de dano.');
    expect(
      effectOf({
        damage: [
          { dice: '1d6', damageTypePt: 'fogo' },
          { dice: '2d4', damageTypePt: 'contundente' },
        ],
      }),
    ).toBe('Dano: 1d6 de fogo e 2d4 de dano contundente.');
  });

  it('writes a save: who it applies to, the failure (damage and condition), and the pass', () => {
    expect(
      effectOf({
        save: {
          ability: Ability.DEXTERITY,
          dc: 13,
          appliesTo: TrapSaveApplies.HIT,
          onPass: TrapPassOutcome.NONE,
          onFail: {
            damage: [{ dice: '3d6', damageTypePt: 'fogo' }],
            condition: { conditionPt: 'caído' },
          },
        },
      }),
    ).toBe(
      'Resistência de Destreza CD 13 (de quem foi atingido): 3d6 de fogo e fica caído; quem passa leva nada.',
    );
    expect(
      effectOf({
        save: { ability: Ability.UNSPECIFIED, dc: 10, onPass: TrapPassOutcome.HALF },
      }),
    ).toBe('Resistência de habilidade CD 10: sem efeito; quem passa leva metade do dano.');
    expect(
      effectOf({
        save: {
          ability: Ability.WISDOM,
          dc: 11,
          onFail: { condition: { conditionPt: 'amedrontado' } },
        },
      }),
    ).toContain('Sabedoria CD 11: fica amedrontado;');
  });

  it('lists the conditions it leaves, and is empty when it only describes', () => {
    expect(effectOf({ conditions: [{ conditionPt: 'cego' }, { conditionPt: 'surdo' }] })).toBe(
      'Deixa cego e surdo.',
    );
    expect(effectOf({})).toBe('');
    expect(effectOf({ targets: TrapTargets.MANUAL })).toBe('');
  });

  it('joins the parts with a period and adds the master choice at the end', () => {
    expect(
      effectOf({
        damage: [{ dice: '1d6', damageTypePt: 'fogo' }],
        conditions: [{ conditionPt: 'cego' }],
        targets: TrapTargets.MANUAL,
      }),
    ).toBe('Dano: 1d6 de fogo. Deixa cego. Você escolhe quem é atingido.');
  });
});

describe('trap text, the card lines', () => {
  const trapPoint = (init: Parameters<typeof create<typeof MapPointSchema>>[1]) =>
    create(MapPointSchema, { kind: MapPointKind.TRAP, ...init });

  it('picks the traps and the treasures out of the points', () => {
    const pts = [
      trapPoint({}),
      create(MapPointSchema, { kind: MapPointKind.TREASURE }),
      create(MapPointSchema, { kind: MapPointKind.SCENE }),
    ];
    expect(trapPoints(pts)).toHaveLength(1);
    expect(treasurePoints(pts)).toHaveLength(1);
  });

  it('gives the glyph of each state and the word of each trigger', () => {
    expect(trapStateIcon(TrapState.TRIGGERED)).toBe('warning');
    expect(trapStateIcon(TrapState.DISARMED)).toBe('check_circle');
    expect(trapStateIcon(TrapState.ARMED)).toBe('crisis_alert');
    expect(triggerWord(TrapTrigger.MANUAL)).toBe('Manual');
    expect(triggerWord(TrapTrigger.ENTER)).toBe('Ao entrar na área');
  });

  it('says a fired or revealed trap is public, and counts the names that are known', () => {
    expect(trapVisibility(trapPoint({ trap: { state: TrapState.TRIGGERED } })).kind).toBe('all');
    expect(trapVisibility(trapPoint({ revealed: true })).kind).toBe('all');
    const one = trapVisibility(
      trapPoint({ trapRevealedTo: [{ characterId: 'a', characterName: 'Toren' }] }),
    );
    expect(one).toEqual({ kind: 'some', label: 'Só Toren sabe' });
    const two = trapVisibility(
      trapPoint({
        trapRevealedTo: [
          { characterId: 'a', characterName: 'Toren' },
          { characterId: 'b', characterName: 'Brisa' },
        ],
      }),
    );
    expect(plain(two.label)).toBe('Só Toren e Brisa sabem');
    expect(trapVisibility(trapPoint({ trapRevealedTo: [{ characterId: 'a' }] })).label).toBe(
      'Alguns jogadores sabem',
    );
  });

  it('writes the effect for a cell, calling a pit damage a fall', () => {
    const effect = { damage: [{ dice: '2d6', damageTypePt: 'contundente' }] };
    expect(plain(effectLine(trapPoint({ trap: { presetKey: 'trap:spikes', effect } })))).toBe(
      'Dano: 2d6 de dano contundente',
    );
    expect(plain(effectLine(trapPoint({ trap: { presetKey: 'trap:pit', effect } })))).toBe(
      'Queda: 2d6 de dano contundente',
    );
    expect(effectLine(trapPoint({}))).toBe('');
  });

  it('writes the line of a closed card, with or without a DC to notice', () => {
    expect(plain(trapDcLine(trapPoint({ trap: { noticeDc: 15, findDc: 12 } })))).toContain(
      'CD para notar 15',
    );
    expect(
      plain(
        trapDcLine(trapPoint({ trap: { noticeDc: 0, findDc: 12, trigger: TrapTrigger.MANUAL } })),
      ),
    ).toContain('sem CD para notar');
    expect(trapDcLine(trapPoint({}))).toBe('');
  });

  it('takes the first line of a description and the clock of a firing', () => {
    expect(firstLine('  Corredor norte \nsegunda linha')).toBe('Corredor norte');
    expect(clockOf(undefined)).toBeNull();
    const at = new Date(2026, 9, 4, 21, 31);
    expect(clockOf(timestampFromDate(at))?.getTime()).toBe(at.getTime());
  });
});
