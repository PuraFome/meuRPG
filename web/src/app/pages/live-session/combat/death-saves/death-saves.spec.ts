import { TestBed } from '@angular/core/testing';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CombatantKind, CombatantState } from '../../../../../gen/meurpg/play/v1/combat_pb';
import type { AttackDie } from '../../../../core/combat/combat-client-types';
import { combatant } from '../../../../core/combat/combat-testing';
import { DeathSaves } from './death-saves';

const brisaDown = combatant({
  id: 'b',
  label: 'Brisa',
  kind: CombatantKind.PLAYER,
  state: CombatantState.DOWN,
  hitPointsCurrent: 0,
  hitPointsMax: 27,
  mine: true,
  deathSaveDue: true,
});

describe('DeathSaves: the campaign’s dice mode (RN-18)', () => {
  function setup(diceMode: DiceMode) {
    const fixture = TestBed.createComponent(DeathSaves);
    fixture.componentRef.setInput('own', brisaDown);
    fixture.componentRef.setInput('diceMode', diceMode);
    fixture.componentRef.setInput('dicePreference', DicePreference.APP);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const rolls: AttackDie[] = [];
    fixture.componentInstance.roll.subscribe((d) => rolls.push(d));
    const button = (name: string) =>
      Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
        (b.textContent ?? '').includes(name),
      );
    return { fixture, el, button, rolls };
  }

  it('follows the campaign’s dice mode: forced app offers only "Rolar teste contra a morte" and emits no typed value', () => {
    const { fixture, el, button, rolls } = setup(DiceMode.APP);
    expect(button('Digitar o resultado')).toBeUndefined();
    expect(el.querySelector('input[type="text"]')).toBeNull();
    button('Rolar teste contra a morte')!.click();
    fixture.detectChanges();
    expect(rolls).toEqual([{ inApp: true }]);
  });

  it('follows the campaign’s dice mode: forced physical offers only the typed entry and emits the face', () => {
    const { fixture, el, button, rolls } = setup(DiceMode.PHYSICAL);
    expect(button('Rolar teste contra a morte')).toBeUndefined();
    const field = el.querySelector<HTMLInputElement>('input[type="text"]')!;
    field.value = '14';
    field.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    button('Confirmar')!.click();
    fixture.detectChanges();
    expect(rolls).toEqual([{ face: 14 }]);
  });

  it('follows the campaign’s dice mode: players choose gets both ways', () => {
    const { button } = setup(DiceMode.PLAYERS_CHOOSE);
    expect(button('Rolar teste contra a morte')).toBeTruthy();
    expect(button('Digitar o resultado')).toBeTruthy();
  });
});
