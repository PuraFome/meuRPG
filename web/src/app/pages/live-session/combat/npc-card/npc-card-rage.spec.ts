import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import {
  CombatantEffectSchema,
  CombatantStateKind,
  ConditionSourceSchema,
  StatePhase,
} from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import { CombatState } from '../../../../core/combat/combat-state';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { NpcCard } from './npc-card';

describe("NpcCard: the NPC's states and rage", () => {
  function setup(over: Parameters<typeof combatant>[0]) {
    TestBed.configureTestingModule({ providers: [{ provide: CombatClient, useValue: {} }] });
    const subject = combatant(over);
    const fixture = TestBed.createComponent(NpcCard);
    fixture.componentRef.setInput('campaignId', 'c');
    fixture.componentRef.setInput(
      'encounter',
      encounter({ combatants: [subject, combatant({ id: 'kai', label: 'Kai' })] }),
    );
    fixture.componentRef.setInput('subject', subject);
    fixture.componentRef.setInput('state', { apply: vi.fn() } as unknown as CombatState);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  const raging = {
    id: 'orc',
    label: 'Orc',
    attackedHostileSinceLastTurn: true,
    tookDamageSinceLastTurn: false,
    states: [
      create(CombatantEffectSchema, {
        id: 's',
        kind: CombatantStateKind.RAGE,
        labelPt: 'Em fúria',
        effectPt: 'Resistência a dano físico.',
      }),
    ],
  };

  it('shows the chip in the header and a button that ends the rage', () => {
    const { fixture, el } = setup(raging);
    const ended: string[] = [];
    fixture.componentInstance.endRage.subscribe((id) => ended.push(id));
    expect(el.querySelector('.card__states button')?.textContent?.trim()).toBe('Em fúria');
    expect(el.querySelector('.card__rage')?.textContent).toContain('Atacou um hostil: sim');
    Array.from(el.querySelectorAll('button'))
      .find((b) => b.textContent?.includes('Encerrar fúria'))!
      .click();
    expect(ended).toEqual(['orc']);
  });

  it('says where a condition of the NPC comes from', () => {
    const { el } = setup({
      id: 'orc',
      label: 'Orc',
      conditions: ['condition:stunned'],
      conditionNamesPt: ['Atordoado'],
      conditionSources: [
        create(ConditionSourceSchema, {
          conditionKey: 'condition:stunned',
          sourceLabel: 'Kai',
          endsCombatantId: 'kai',
          endsPhase: StatePhase.END_OF_TURN,
        }),
      ],
    });
    expect(el.querySelector('.card__states')?.textContent?.replace(/\s+/g, ' ')).toContain(
      'Atordoado , por Kai, até o fim do turno de Kai',
    );
  });

  it('has no rage button for an NPC that is not raging', () => {
    const { el } = setup({ id: 'orc', label: 'Orc' });
    expect(el.textContent).not.toContain('Encerrar fúria');
  });
});
