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
} from '../../../gen/meurpg/rules/v1/rules_pb';
import {
  areaWords,
  effectText,
  isPinKind,
  knownReason,
  trapStateWord,
  trapVisibility,
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
