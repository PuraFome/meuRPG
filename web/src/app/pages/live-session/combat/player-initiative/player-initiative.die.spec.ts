import { ComponentFixture, TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CombatantKind } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { RollDieSchema } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import type { InitiativeManeuver } from '../../../../core/combat/combat-client-types';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { PlayerInitiative } from './player-initiative';

const me = combatant({
  id: 'me',
  label: 'Sálvia',
  kind: CombatantKind.PLAYER,
  mine: true,
  controlledByMe: true,
  initiativeBonus: 2,
});
const die = (applies: string) =>
  create(RollDieSchema, {
    key: `feature:${applies}@mesa`,
    namePt: `Dado de ${applies}`,
    applies,
    sides: 6,
    resource: 'dados',
  });

describe('PlayerInitiative: a table die on the initiative (MR-025)', () => {
  let fixture: ComponentFixture<PlayerInitiative>;
  let el: HTMLElement;
  const picks: (InitiativeManeuver | null)[] = [];
  const faces: number[] = [];

  function setup(preference: DicePreference) {
    picks.length = 0;
    faces.length = 0;
    fixture = TestBed.createComponent(PlayerInitiative);
    fixture.componentRef.setInput('encounter', encounter({ combatants: [me] }));
    fixture.componentRef.setInput('diceMode', DiceMode.PLAYERS_CHOOSE);
    fixture.componentRef.setInput('dicePreference', preference);
    fixture.componentRef.setInput('rollDice', [die('initiative'), die('check'), die('save')]);
    fixture.componentInstance.maneuverChange.subscribe((m) => picks.push(m));
    fixture.componentInstance.typeFace.subscribe((f) => faces.push(f));
    fixture.detectChanges();
    el = fixture.nativeElement as HTMLElement;
  }
  const type = (selector: string, text: string) => {
    const input = el.querySelector<HTMLInputElement>(selector)!;
    input.value = text;
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
  };
  const tick = () => {
    el.querySelector<HTMLInputElement>('app-roll-die-offer input')!.click();
    fixture.detectChanges();
  };
  const button = (text: string) =>
    Array.from(el.querySelectorAll('button')).find((b) =>
      b.textContent?.includes(text),
    ) as HTMLButtonElement;

  const off = () => button('Confirmar').getAttribute('aria-disabled') === 'true';

  it('offers only the initiative die, and tells the page which one is ticked', () => {
    setup(DicePreference.APP);
    const offer = el.querySelector('app-roll-die-offer')!;
    expect(offer.querySelectorAll('input')).toHaveLength(1);
    expect(offer.textContent).toContain('Usar Dado de initiative');
    expect(offer.textContent).toContain('d6 · gasta 1 uso de dados');
    tick();
    expect(picks.at(-1)).toEqual({ key: 'feature:initiative@mesa' });
    tick();
    expect(picks.at(-1)).toBeNull();
  });

  it('holds the typed d20 until the ticked die has its face, then sends both', () => {
    setup(DicePreference.PHYSICAL);
    tick();
    button('Digitar o resultado').click();
    fixture.detectChanges();
    type('#face', '12');
    expect(off()).toBe(true);
    expect(el.textContent).toContain('Resultado do d6 (Dado de initiative)');
    type('#die-face', '7');
    expect(off()).toBe(true);
    type('#die-face', '4');
    expect(picks.at(-1)).toEqual({ key: 'feature:initiative@mesa', face: 4 });
    button('Confirmar').click();
    expect(faces).toEqual([12]);
  });
});
