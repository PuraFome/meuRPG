import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import {
  CombatantEffectSchema,
  CombatantStateKind,
} from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import { combatant } from '../../../../core/combat/combat-testing';
import { RageStatus } from './rage-status';
import { isFrenzied, isRaging } from './rage';

const raging = (attacked: boolean, damaged: boolean) =>
  combatant({
    id: 't',
    label: 'Toren',
    attackedHostileSinceLastTurn: attacked,
    tookDamageSinceLastTurn: damaged,
    states: [
      create(CombatantEffectSchema, {
        id: 's',
        kind: CombatantStateKind.RAGE,
        labelPt: 'Em fúria',
      }),
    ],
  });

function setup(subject: ReturnType<typeof combatant>, canEnd = true) {
  const fixture = TestBed.createComponent(RageStatus);
  fixture.componentRef.setInput('subject', subject);
  fixture.componentRef.setInput('canEnd', canEnd);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement };
}

describe('RageStatus', () => {
  it('says whether the raging character attacked and took damage', () => {
    const { el } = setup(raging(true, false));
    expect(el.querySelector('[role="status"]')?.textContent?.replace(/\s+/g, ' ')).toContain(
      'Atacou um hostil: sim · Sofreu dano: não',
    );
  });

  it('ends the rage from the button', () => {
    const { fixture, el } = setup(raging(false, false));
    const ended: unknown[] = [];
    fixture.componentInstance.endRage.subscribe(() => ended.push(true));
    const button = el.querySelector<HTMLButtonElement>('button')!;
    expect(button.textContent).toContain('Encerrar fúria');
    button.click();
    expect(ended).toHaveLength(1);
  });

  it('hides the button when the rage cannot be ended now', () => {
    const { el } = setup(raging(false, false), false);
    expect(el.querySelector('button')).toBeNull();
    expect(el.textContent).toContain('Atacou um hostil');
  });

  it('draws nothing for a character that is not raging', () => {
    const calm = combatant({ id: 'c', label: 'Calma' });
    const { el } = setup(calm);
    expect(el.textContent?.trim()).toBe('');
    expect(isRaging(calm)).toBe(false);
    expect(isRaging(raging(false, false))).toBe(true);
  });

  it('says "Em frenesi" when the rage is a frenzy, and only then', () => {
    const plain = raging(false, false);
    expect(setup(plain).el.textContent).not.toContain('Em frenesi');
    expect(isFrenzied(plain)).toBe(false);
    const frenzy = combatant({
      id: 'b',
      label: 'Brum',
      states: [
        create(CombatantEffectSchema, {
          id: 's',
          kind: CombatantStateKind.RAGE,
          labelPt: 'Em frenesi',
          frenzy: true,
        }),
      ],
    });
    expect(isFrenzied(frenzy)).toBe(true);
    expect(setup(frenzy).el.textContent).toContain('Em frenesi');
  });
});
