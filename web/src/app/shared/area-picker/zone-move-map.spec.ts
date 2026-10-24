import { TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';

import type { Square } from '../../core/combat/combat-grid';
import { combatant, spellZone } from '../../core/combat/combat-testing';
import { settle } from '../../../test-settle';
import { CombatMap } from '../combat-map/combat-map';
import { AreaOverlay } from './area-overlay';
import { ZoneMoveMap } from './zone-move-map';

const zone = spellZone({
  id: 'z1',
  sizeFt: 5,
  origin: { col: 4, row: 4 },
  squares: [
    { col: 4, row: 4 },
    { col: 5, row: 4 },
  ],
});

const plain = (t: string | null | undefined) =>
  (t ?? '').replace(/ /g, ' ').replace(/\s+/g, ' ').trim();

async function setup(over: { master?: boolean; picked?: Square | null } = {}) {
  const fixture = TestBed.createComponent(ZoneMoveMap);
  const ref = fixture.componentRef;
  ref.setInput('image', { url: 'map.png', width: 1200, height: 800 });
  ref.setInput('columns', 30);
  ref.setInput('rows', 8);
  ref.setInput('combatants', [combatant({ id: 'g', label: 'Goblin', col: 1, row: 1 })]);
  ref.setInput('zone', zone);
  ref.setInput('master', over.master ?? false);
  ref.setInput('picked', over.picked ?? null);
  const picks: Square[] = [];
  const confirms: number[] = [];
  fixture.componentInstance.pick.subscribe((p) => picks.push(p));
  fixture.componentInstance.confirm.subscribe(() => confirms.push(1));
  await settle(fixture);
  const el = fixture.nativeElement as HTMLElement;
  const map = () => fixture.debugElement.query(By.directive(CombatMap));
  const choose = async (sq: Square) => {
    map().triggerEventHandler('choose', sq);
    await settle(fixture);
  };
  const live = () => plain(el.querySelector('[role="status"]')?.textContent);
  return { fixture, el, picks, confirms, choose, live, map };
}

describe('ZoneMoveMap', () => {
  it('draws the current footprint dimmed, and no new one before a square is picked', async () => {
    const { fixture, el } = await setup();
    const overlays = fixture.debugElement.queryAll(By.directive(AreaOverlay));
    expect(overlays).toHaveLength(1);
    expect(el.querySelector('.zm__now')).not.toBeNull();
  });

  it('draws the new footprint around the picked square', async () => {
    const { fixture } = await setup({ picked: { col: 8, row: 4 } });
    const overlays = fixture.debugElement.queryAll(By.directive(AreaOverlay));
    expect(overlays).toHaveLength(2);
    expect((overlays[1].componentInstance as AreaOverlay).squares().some((s) => s.col === 8)).toBe(
      true,
    );
  });

  it('says how to pick: the 18 m range for a player, no limit for the master', async () => {
    const player = await setup();
    expect(plain(player.el.querySelector('.hint')?.textContent)).toContain('até 18 m');
    const master = await setup({ master: true });
    expect(plain(master.el.querySelector('.hint')?.textContent)).toContain('Sem limite');
  });

  it('reports a click and speaks it', async () => {
    const { picks, choose, live } = await setup();
    await choose({ col: 8, row: 4 });
    expect(picks).toEqual([{ col: 8, row: 4 }]);
    expect(live()).toBe('Coluna 9, linha 5, a 6,0 m. Escolhido.');
  });

  it('refuses in words a square past the range for a player, and not for the master', async () => {
    const player = await setup({ picked: { col: 28, row: 4 } });
    await player.choose({ col: 28, row: 4 });
    expect(plain(player.el.querySelector('[role="alert"]')?.textContent)).toContain('Longe demais');
    const master = await setup({ master: true, picked: { col: 28, row: 4 } });
    await master.choose({ col: 28, row: 4 });
    expect(master.el.querySelector('[role="alert"]')).toBeNull();
  });

  it('confirms with Enter on the map, as the other pickers do', async () => {
    const { map, confirms, fixture } = await setup({ picked: { col: 8, row: 4 } });
    map().triggerEventHandler('confirm');
    await settle(fixture);
    expect(confirms).toHaveLength(1);
  });
});
