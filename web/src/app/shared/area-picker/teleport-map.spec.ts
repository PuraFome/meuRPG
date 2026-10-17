import { TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';

import { CombatantKind } from '../../../gen/meurpg/play/v1/combat_pb';
import type { Square } from '../../core/combat/combat-grid';
import { combatant } from '../../core/combat/combat-testing';
import { settle } from '../../../test-settle';
import { CombatMap } from '../combat-map/combat-map';
import { TeleportMap } from './teleport-map';

const REACH_FT = 30;

const caster = combatant({
  id: 'p',
  label: 'Pensantus',
  kind: CombatantKind.PLAYER,
  mine: true,
  col: 2,
  row: 4,
});
const goblin = combatant({ id: 'g1', label: 'Goblin 1', col: 3, row: 4 });

const plain = (t: string | null | undefined) =>
  (t ?? '')
    .replace(/\u00a0/g, ' ')
    .replace(/\s+/g, ' ')
    .trim();

async function setup(over: { master?: boolean } = {}) {
  const fixture = TestBed.createComponent(TeleportMap);
  const ref = fixture.componentRef;
  ref.setInput('image', { url: 'map.png', width: 1200, height: 800 });
  ref.setInput('columns', 12);
  ref.setInput('rows', 8);
  ref.setInput('combatants', [caster, goblin]);
  ref.setInput('casterId', 'p');
  ref.setInput('reachFt', REACH_FT);
  ref.setInput('master', over.master ?? false);
  const picks: (Square | null)[] = [];
  fixture.componentInstance.pick.subscribe((p) => picks.push(p));
  await settle(fixture);
  const el = fixture.nativeElement as HTMLElement;
  const group = el.querySelector<HTMLElement>('[role="group"][tabindex="0"]')!;
  const choose = async (sq: Square) => {
    fixture.debugElement.query(By.directive(CombatMap)).triggerEventHandler('choose', sq);
    await settle(fixture);
  };
  const key = async (name: string) => {
    group.dispatchEvent(new KeyboardEvent('keydown', { key: name, bubbles: true }));
    await settle(fixture);
  };
  const live = () => plain(el.querySelector('[role="status"]')?.textContent);
  return { fixture, el, group, picks, choose, key, live };
}

describe('TeleportMap', () => {
  it('says how to pick and is one focus stop of the map', async () => {
    const { el, group } = await setup();
    expect(plain(el.querySelector('.hint')?.textContent)).toContain('para onde você vai, até 9 m');
    expect(group.getAttribute('aria-label')).toContain('Mapa de batalha');
  });

  it('picks a free square in reach with a click and says it in the live region', async () => {
    const { picks, choose, live } = await setup();
    await choose({ col: 6, row: 4 });
    expect(picks).toEqual([{ col: 6, row: 4 }]);
    expect(live()).toBe('Coluna 7, linha 5, a 6,0 m. Escolhido.');
  });

  it('refuses a square beyond the reach with "Longe demais" and picks nothing', async () => {
    const { el, picks, choose, live } = await setup();
    await choose({ col: 10, row: 4 });
    expect(picks).toEqual([null]);
    expect(plain(el.querySelector('[role="alert"] p')?.textContent)).toBe('Longe demais');
    expect(live()).toContain('Longe demais.');
  });

  it('refuses the square of a creature the player sees', async () => {
    const { el, picks, choose } = await setup();
    await choose({ col: 3, row: 4 });
    expect(picks).toEqual([null]);
    expect(plain(el.querySelector('[role="alert"] p')?.textContent)).toBe(
      'Esse espaço não está disponível',
    );
  });

  it('moves the square with the arrows from the caster and picks it, without a pointer', async () => {
    const { picks, key, live } = await setup();
    await key('ArrowDown');
    await key('ArrowRight');
    expect(picks).toEqual([
      { col: 2, row: 5 },
      { col: 3, row: 5 },
    ]);
    expect(live()).toBe('Coluna 4, linha 6, a 1,5 m. Escolhido.');
  });

  it('Enter says the chosen square again', async () => {
    const { key, live } = await setup();
    await key('ArrowUp');
    await key('Enter');
    expect(live()).toBe('Coluna 3, linha 4, a 1,5 m. Escolhido.');
  });

  it('lets the master try any square, and says nothing is tinted', async () => {
    const { picks, choose } = await setup({ master: true });
    await choose({ col: 11, row: 7 });
    expect(picks).toEqual([{ col: 11, row: 7 }]);
  });
});
