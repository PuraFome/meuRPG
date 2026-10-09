import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import { CombatantKind } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  CombatantEffectSchema,
  CombatantStateKind,
} from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { TurnPanel } from './turn-panel';

describe("TurnPanel: the player's own states and rage", () => {
  function setup(rage: boolean) {
    const toren = combatant({
      id: 'toren',
      label: 'Toren',
      kind: CombatantKind.PLAYER,
      mine: true,
      attackedHostileSinceLastTurn: false,
      tookDamageSinceLastTurn: true,
      states: rage
        ? [
            create(CombatantEffectSchema, {
              id: 's',
              kind: CombatantStateKind.RAGE,
              labelPt: 'Em fúria',
              effectPt: 'Vantagem em Força.',
            }),
            create(CombatantEffectSchema, {
              id: 'r',
              kind: CombatantStateKind.RECKLESS,
              labelPt: 'Ataque descuidado',
              effectPt: 'Vantagem no ataque, e os ataques contra você também.',
            }),
          ]
        : [],
    });
    const fixture = TestBed.createComponent(TurnPanel);
    fixture.componentRef.setInput(
      'encounter',
      encounter({ combatants: [toren], currentCombatantId: 'toren' }),
    );
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  it('shows the chips under the turn, the two facts and "Encerrar fúria"', () => {
    const { fixture, el } = setup(true);
    const chips = Array.from(el.querySelectorAll('.turn__states button'), (b) =>
      b.textContent?.trim(),
    );
    expect(chips).toEqual(['Em fúria', 'Ataque descuidado']);
    expect(el.querySelector('.turn__rage')?.textContent?.replace(/\s+/g, ' ')).toContain(
      'Atacou um hostil: não · Sofreu dano: sim',
    );
    const ended: string[] = [];
    fixture.componentInstance.endRage.subscribe((id) => ended.push(id));
    Array.from(el.querySelectorAll('button'))
      .find((b) => b.textContent?.includes('Encerrar fúria'))!
      .click();
    expect(ended).toEqual(['toren']);
  });

  it('shows neither without a state', () => {
    const { el } = setup(false);
    expect(el.querySelector('.turn__states button')).toBeNull();
    expect(el.textContent).not.toContain('Encerrar fúria');
  });
});
