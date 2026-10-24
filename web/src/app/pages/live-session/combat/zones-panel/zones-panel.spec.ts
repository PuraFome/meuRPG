import { TestBed } from '@angular/core/testing';

import {
  SpellZoneMovement,
  SpellZoneVisibility,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { SpellAreaShape } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { combatant, spellZone } from '../../../../core/combat/combat-testing';
import { zoneViews } from '../../../../core/combat/zones';
import { encounter } from '../../../../core/combat/combat-testing';
import { ZonesPanel } from './zones-panel';

const plain = (t: string | null | undefined) => (t ?? '').replace(/\s+/g, ' ').trim();

function mount(zones: ReturnType<typeof spellZone>[], over: { master?: boolean } = {}) {
  const master = over.master ?? true;
  const views = zoneViews(
    encounter({
      combatants: [
        combatant({ id: 'tor', label: 'Tor' }),
        combatant({ id: 'jabuti', label: 'Jabuti' }),
      ],
      zones,
    }),
    master,
  );
  TestBed.resetTestingModule();
  const fixture = TestBed.createComponent(ZonesPanel);
  fixture.componentRef.setInput('zones', views);
  fixture.componentRef.setInput('master', master);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const setZones = (next: ReturnType<typeof spellZone>[]) => {
    fixture.componentRef.setInput(
      'zones',
      zoneViews(encounter({ combatants: [], zones: next }), master),
    );
    fixture.detectChanges();
  };
  return { fixture, el, setZones };
}

const click = (el: Element | null, f: { detectChanges: () => void }) => {
  (el as HTMLElement).click();
  f.detectChanges();
};
const button = (el: HTMLElement, text: string) =>
  [...el.querySelectorAll('button')].find((b) => plain(b.textContent).includes(text)) ?? null;

describe('ZonesPanel', () => {
  it('draws nothing for an empty list', () => {
    const { el } = mount([]);
    expect(el.querySelector('section')).toBeNull();
    expect(plain(el.textContent)).toBe('');
  });

  it('is a section named by its heading, with the zones as a list', () => {
    const { el } = mount([spellZone({ id: 'a' }), spellZone({ id: 'b', spellNamePt: 'Teia' })]);
    const section = el.querySelector('section')!;
    const h = el.querySelector('#' + section.getAttribute('aria-labelledby'))!;
    expect(plain(h.textContent)).toBe('Zonas ativas (2)');
    expect(section.querySelectorAll('ul > li').length).toBe(2);
  });

  it('reads a card: title, area, save and damage, rounds, concentration and who is inside', () => {
    const { el } = mount([spellZone({ id: 'a', occupantIds: ['tor', 'jabuti'] })]);
    const card = plain(el.querySelector('li')!.textContent);
    expect(card).toContain('Raio Lunar · Bartolo');
    expect(card).toContain(
      'Cilindro de 1,5 m de raio · Constituição CD 14 · 2d10 radiante, metade se passar',
    );
    expect(card).toContain('restam 9 rodadas');
    expect(card).toContain('Concentração de Bartolo');
    expect(card).toContain('Aqui: Tor, Jabuti');
  });

  it('says "sem teste", the move per turn and the notes', () => {
    const { el } = mount([
      spellZone({
        id: 'a',
        spellNamePt: 'Nuvem Mortal',
        shape: SpellAreaShape.SPHERE,
        sizeFt: 20,
        saveAbility: '',
        movesPerTurnFt: 10,
        notes: ['restrains'],
      }),
    ]);
    const card = plain(el.querySelector('li')!.textContent);
    expect(card).toContain('anda 3 m por turno');
    expect(card).toContain('sem teste');
    expect(card).toContain('contido');
  });

  it('never prints "restam 0": a zone with no count has no chip', () => {
    const { el } = mount([spellZone({ id: 'a', roundsLeft: 0 })]);
    expect(plain(el.textContent)).not.toContain('restam');
  });

  it('shows a zone with hidden details as name, area and caster only', () => {
    const { el } = mount(
      [
        spellZone({
          id: 'a',
          detailsHidden: true,
          saveAbility: '',
          damageDice: '',
          roundsLeft: 0,
          occupantIds: ['tor'],
        }),
      ],
      { master: false },
    );
    const card = plain(el.querySelector('li')!.textContent);
    expect(card).toBe('Raio Lunar · Bartolo Cilindro de 1,5 m de raio');
    expect(el.querySelector('button')).toBeNull();
  });

  it('marks the zone of a hidden caster "Só você vê"', () => {
    const { el } = mount([spellZone({ id: 'a', visibility: SpellZoneVisibility.MASTER_ONLY })]);
    expect(plain(el.querySelector('li')!.textContent)).toContain('Só você vê');
  });

  it('offers "Mover" only for a zone that moves by an action, and says which zone to the page', () => {
    const { fixture, el } = mount([
      spellZone({ id: 'a' }),
      spellZone({ id: 'b', spellNamePt: 'Teia', movement: SpellZoneMovement.FIXED }),
    ]);
    const moves = [...el.querySelectorAll('button')].filter((b) =>
      plain(b.textContent).endsWith('Mover'),
    );
    expect(moves.length).toBe(1);
    const seen: string[] = [];
    fixture.componentInstance.move.subscribe((id: string) => seen.push(id));
    click(moves[0], fixture);
    expect(seen).toEqual(['a']);
  });

  it("keeps the buttons of the master out of a player's panel", () => {
    const { el } = mount([spellZone({ id: 'a' })], { master: false });
    expect(button(el, 'Encerrar')).toBeNull();
    expect(button(el, 'Mover')).toBeNull();
  });

  it('asks before ending a zone, and "Não" closes the question without a call', () => {
    const { fixture, el } = mount([spellZone({ id: 'a' })]);
    const ended: string[] = [];
    fixture.componentInstance.end.subscribe((id: string) => ended.push(id));
    click(button(el, 'Encerrar zona'), fixture);
    expect(plain(el.textContent)).toContain('Encerrar a zona?');
    click(button(el, 'Não'), fixture);
    expect(plain(el.textContent)).not.toContain('Encerrar a zona?');
    expect(ended).toEqual([]);
  });

  it('ends the zone only on "Sim, encerrar"', () => {
    const { fixture, el } = mount([
      spellZone({ id: 'a' }),
      spellZone({ id: 'b', spellNamePt: 'Teia' }),
    ]);
    const ended: string[] = [];
    fixture.componentInstance.end.subscribe((id: string) => ended.push(id));
    click(el.querySelectorAll('li')[1].querySelector('button.zp__end'), fixture);
    expect(el.querySelectorAll('.zp__confirm').length).toBe(1);
    click(button(el, 'Sim, encerrar'), fixture);
    expect(ended).toEqual(['b']);
  });

  it('puts the focus on "Não" when the question opens, and says a refusal in place', async () => {
    const { fixture, el } = mount([spellZone({ id: 'a' })]);
    document.body.appendChild(el);
    click(button(el, 'Encerrar zona'), fixture);
    await fixture.whenStable();
    expect(document.activeElement).toBe(button(el, 'Não'));
    fixture.componentRef.setInput('error', 'Só o mestre encerra uma zona.');
    fixture.detectChanges();
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('Só o mestre encerra');
    el.remove();
  });

  it('holds the buttons while a call is in the air', () => {
    const { fixture, el } = mount([spellZone({ id: 'a' })]);
    fixture.componentRef.setInput('busy', true);
    fixture.detectChanges();
    expect((button(el, 'Encerrar zona') as HTMLButtonElement).disabled).toBe(true);
  });

  it('keeps every button at least 44 px tall', () => {
    const { el } = mount([spellZone({ id: 'a' })]);
    for (const b of el.querySelectorAll('button')) {
      expect(b.classList.contains('zp__btn')).toBe(true);
    }
  });

  it('announces a zone created or ended in a polite region, not on the first read', async () => {
    const { fixture, el, setZones } = mount([spellZone({ id: 'a' })]);
    const live = el.querySelector('[aria-live="polite"]')!;
    await fixture.whenStable();
    expect(plain(live.textContent)).toBe('');
    setZones([spellZone({ id: 'a' }), spellZone({ id: 'b', spellNamePt: 'Teia' })]);
    await fixture.whenStable();
    fixture.detectChanges();
    expect(plain(live.textContent)).toBe('Zona criada: Teia · Bartolo.');
    setZones([]);
    await fixture.whenStable();
    fixture.detectChanges();
    expect(plain(live.textContent)).toContain('Zonas encerradas:');
  });

  it('keeps the live region when the list is empty, so the first zone is announced', async () => {
    const { fixture, el, setZones } = mount([]);
    const live = el.querySelector('[aria-live="polite"]')!;
    setZones([spellZone({ id: 'a' })]);
    await fixture.whenStable();
    fixture.detectChanges();
    expect(plain(live.textContent)).toBe('Zona criada: Raio Lunar · Bartolo.');
  });
});
