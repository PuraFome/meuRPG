import { TestBed } from '@angular/core/testing';

import { SpellZoneVisibility } from '../../../../gen/meurpg/play/v1/combat_pb';
import { spellZone } from '../../../core/combat/combat-testing';
import { zoneView, type ZoneView } from '../../../core/combat/zones';
import { ZoneLayer } from './zone-layer';

const square = (col: number, row: number) => ({ col, row });
function view(over: Parameters<typeof spellZone>[0]): ZoneView {
  return zoneView(
    spellZone({ squares: [square(4, 3), square(5, 3), square(4, 4), square(5, 4)], ...over }),
    { master: true, combatants: [] },
  );
}

function mount(zones: ZoneView[]) {
  TestBed.resetTestingModule();
  const fixture = TestBed.createComponent(ZoneLayer);
  fixture.componentRef.setInput('columns', 20);
  fixture.componentRef.setInput('rows', 10);
  fixture.componentRef.setInput('zones', zones);
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('ZoneLayer', () => {
  it('draws nothing without a zone', () => {
    expect(mount([]).querySelector('svg')).toBeNull();
  });

  it('is decorative: the panel list is the accessible version', () => {
    const el = mount([view({ id: 'a' })]);
    expect(el.getAttribute('aria-hidden')).toBe('true');
  });

  it('draws the footprint of each zone from its squares, with a pattern and an outline', () => {
    const el = mount([
      view({ id: 'a' }),
      view({ id: 'b', spellKey: 'spell:web', spellNamePt: 'Teia' }),
    ]);
    const zones = el.querySelectorAll('g.zl__zone');
    expect(zones.length).toBe(2);
    const first = zones[0];
    expect(first.querySelector('.zl__fill')?.getAttribute('d')).toMatch(/^M/);
    expect(first.querySelector('.zl__pat')?.getAttribute('fill')).toMatch(/^url\(#.+\)$/);
    expect(first.querySelector('.zl__edge')?.getAttribute('d')).toMatch(/^M/);
    // two spells, two patterns
    expect(zones[1].querySelector('.zl__pat')?.getAttribute('fill')).not.toBe(
      first.querySelector('.zl__pat')?.getAttribute('fill'),
    );
  });

  it('names each zone with its spell and caster', () => {
    const el = mount([view({ id: 'a' })]);
    expect((el.querySelector('.zl__label')?.textContent ?? '').replace(/\s+/g, ' ')).toContain(
      'Raio Lunar · Bartolo',
    );
  });

  it('draws the zone of a hidden caster dashed, with its label marked', () => {
    const el = mount([view({ id: 'a', visibility: SpellZoneVisibility.MASTER_ONLY })]);
    expect(el.querySelector('g.zl__zone--dashed')).not.toBeNull();
    expect(el.querySelector('.zl__label--hidden')).not.toBeNull();
  });

  it('marks a concentration zone on its label', () => {
    expect(mount([view({ id: 'a' })]).querySelector('.zl__label mat-icon')).not.toBeNull();
    expect(
      mount([view({ id: 'a', concentration: false })]).querySelector('.zl__label mat-icon'),
    ).toBeNull();
  });

  it("puts the label over the zone's top edge, centered, in percent of the map", () => {
    const label = mount([view({ id: 'a' })]).querySelector<HTMLElement>('.zl__label')!;
    expect(label.style.left).toBe('25%'); // (4 + 6) / 2 over 20
    expect(label.style.top).toBe('30%'); // row 3 over 10
  });

  it('puts the label under a zone that touches the top of the map', () => {
    const label = mount([
      view({ id: 'a', squares: [square(2, 0), square(3, 0)] }),
    ]).querySelector<HTMLElement>('.zl__label')!;
    expect(label.classList.contains('zl__label--below')).toBe(true);
    expect(label.style.top).toBe('10%');
  });

  it('skips a zone with no squares', () => {
    expect(mount([view({ id: 'a', squares: [] })]).querySelector('g.zl__zone')).toBeNull();
  });
});
