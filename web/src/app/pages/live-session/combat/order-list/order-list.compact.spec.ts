import { TestBed } from '@angular/core/testing';

import { CombatantKind, CoverDegree } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { OrderList } from './order-list';

const rows = [
  combatant({
    id: 'g1',
    label: 'Goblin 1',
    hitPointsCurrent: 7,
    hitPointsMax: 7,
    initiative: 14,
    armorClass: 15,
  }),
  combatant({
    id: 'pen',
    label: 'Pensantus',
    kind: CombatantKind.PLAYER,
    characterId: 'pen-c',
    hitPointsCurrent: 23,
    hitPointsMax: 23,
    initiative: 12,
  }),
  combatant({
    id: 'cul',
    label: 'Cultista',
    hidden: true,
    hitPointsCurrent: 9,
    hitPointsMax: 9,
    initiative: 7,
    armorClass: 12,
  }),
];

describe('OrderList compact rows (C1-02, Main)', () => {
  function setup(list = rows, inputs: Record<string, unknown> = {}) {
    const fixture = TestBed.createComponent(OrderList);
    fixture.componentRef.setInput(
      'encounter',
      encounter({ combatants: list, currentCombatantId: 'g1' }),
    );
    fixture.componentRef.setInput('adjustable', new Set(['pen-c']));
    for (const [k, v] of Object.entries(inputs)) {
      fixture.componentRef.setInput(k, v);
    }
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const plain = (n?: Element | null) => (n?.textContent ?? '').replace(/\u00a0/g, ' ').trim();
    const head = (label: string) =>
      Array.from(el.querySelectorAll<HTMLButtonElement>('.row__head')).find((h) =>
        h.getAttribute('aria-label')?.startsWith(label),
      )!;
    return { fixture, el, head, plain };
  }

  it('draws a closed row of one line, with the name, the initiative and armor class, the hit points and a bar', () => {
    const { el, head, plain } = setup();
    expect(head('Goblin 1').getAttribute('aria-expanded')).toBe('false');
    expect(head('Goblin 1').getAttribute('aria-label')).toBe('Goblin 1, é a vez, 7 de 7 PV');
    const first = el.querySelector('li.row')!;
    expect(plain(first.querySelector('.row__sub'))).toBe('Inic. 14 · CA 15');
    expect(first.querySelector('.row__hp-n')?.textContent?.replace(/\s+/g, '')).toBe('7/7');
    expect(first.querySelector('.row__bar')).not.toBeNull();
    expect(el.querySelector('.row__actions')).toBeNull();
  });

  it('says the state word before the initiative on the line of a hidden one', () => {
    const { el, plain } = setup();
    const line = Array.from(el.querySelectorAll('li.row .row__sub')).map((n) => plain(n));
    expect(line).toEqual(['Inic. 14 · CA 15', 'Inic. 12', 'Escondido · inic. 7']);
  });

  it('opens the row in place on a tap, with "Dano/Cura", "Condições" and "Mais", and closes it on the next', () => {
    const { fixture, el, head } = setup();
    head('Pensantus').click();
    fixture.detectChanges();
    expect(head('Pensantus').getAttribute('aria-expanded')).toBe('true');
    const open = el.querySelector('li.row[data-combatant="pen"] .row__open')!;
    expect(head('Pensantus').getAttribute('aria-controls')).toBe(open.id);
    const names = Array.from(open.querySelectorAll('.row__actions button'), (b) =>
      b.getAttribute('aria-label'),
    );
    expect(names).toEqual([
      'Dano ou cura em Pensantus',
      'Condições de Pensantus',
      'Mais ações para Pensantus',
    ]);
    head('Pensantus').click();
    fixture.detectChanges();
    expect(el.querySelector('.row__open')).toBeNull();
  });

  it('says "Revelar aos jogadores" in the opened row of a hidden NPC', () => {
    const { fixture, el, head } = setup();
    head('Cultista').click();
    fixture.detectChanges();
    expect(el.textContent).toContain('Só você vê este combatente.');
    expect(
      Array.from(el.querySelectorAll('button')).some((b) =>
        b.textContent?.includes('Revelar aos jogadores'),
      ),
    ).toBe(true);
  });

  it('says the cover line in the opened row, where "Marcar cobertura" is', () => {
    const { fixture, el, head } = setup(rows, {
      coverAgainst: new Map([['g1', { cover: CoverDegree.HALF, source: 1 }]]),
      turnLabel: 'Pensantus',
    });
    expect(el.querySelector('app-row-cover')).toBeNull();
    head('Goblin 1').click();
    fixture.detectChanges();
    expect(el.querySelector('app-row-cover')).not.toBeNull();
  });

  it('shows the count of combatants only when the order is long', () => {
    expect(setup().el.querySelector('.order__count')).toBeNull();
    const many = Array.from({ length: 9 }, (_, i) =>
      combatant({ id: `c${i}`, label: `Cultista ${i}`, hitPointsCurrent: 9, hitPointsMax: 9 }),
    );
    const { el } = setup(many);
    expect(el.querySelector('.order__count')?.textContent?.trim()).toBe('9');
    expect(el.querySelector('.order__count')?.getAttribute('aria-label')).toBe('9 combatentes');
  });

  it('has the fold button only where the layout can fold it, and folds to a strip of tokens', () => {
    expect(setup().el.querySelector('[aria-label="Recolher a ordem de iniciativa"]')).toBeNull();
    const { fixture, el } = setup(rows, { foldable: true });
    const fold = el.querySelector<HTMLButtonElement>(
      '[aria-label="Recolher a ordem de iniciativa"]',
    )!;
    fold.click();
    fixture.detectChanges();
    expect(fixture.componentInstance.folded()).toBe(true);
    expect(el.querySelector('.order')).toBeNull();
    const tokens = Array.from(el.querySelectorAll<HTMLButtonElement>('.strip__token'), (t) =>
      t.getAttribute('aria-label'),
    );
    expect(tokens).toEqual([
      'Goblin 1, é a vez, 7 de 7 PV',
      'Pensantus, 23 de 23 PV',
      'Cultista, Escondido, 9 de 9 PV',
    ]);
    expect(el.querySelector('[aria-label="Ordem de iniciativa, recolhida"]')).not.toBeNull();
  });

  it('unfolds with the row of the token that was tapped open', () => {
    const { fixture, el } = setup(rows, { foldable: true, folded: true });
    el.querySelectorAll<HTMLButtonElement>('.strip__token')[1].click();
    fixture.detectChanges();
    expect(fixture.componentInstance.folded()).toBe(false);
    expect(
      el.querySelector('li.row[data-combatant="pen"] .row__head')?.getAttribute('aria-expanded'),
    ).toBe('true');
  });

  it('keeps a row open while its question is', () => {
    const { fixture, el } = setup();
    const cmp = fixture.componentInstance as unknown as { ask(id: string): void };
    cmp.ask('cul');
    fixture.detectChanges();
    expect(
      el.querySelector('li.row[data-combatant="cul"] .row__head')?.getAttribute('aria-expanded'),
    ).toBe('true');
    expect(el.querySelector('[role="alertdialog"]')).not.toBeNull();
  });
});
