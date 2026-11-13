import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';

import { CombatantSchema } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { ContestTurnStateSchema } from '../../../../../gen/meurpg/play/v1/contest_types_pb';
import { TurnOptionsSchema } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { ActionGroups } from './action-groups';

// The first row of "Ações" (584-01, 584-02): what is left of the turn, "Mover", and "Permanecer na forma" after "Voltar".
function setup(own: Record<string, unknown> = {}, inputs: Record<string, unknown> = {}) {
  const fixture = TestBed.createComponent(ActionGroups);
  fixture.componentRef.setInput('options', create(TurnOptionsSchema, {}));
  fixture.componentRef.setInput(
    'own',
    create(CombatantSchema, {
      movementLeftFt: 40,
      movementLeftDft: 400,
      speedFt: 40,
      speedDft: 400,
      ...own,
    }),
  );
  for (const [k, v] of Object.entries(inputs)) {
    fixture.componentRef.setInput(k, v);
  }
  const events: string[] = [];
  fixture.componentInstance.move.subscribe(() => events.push('move'));
  fixture.componentInstance.stayForm.subscribe(() => events.push('stay'));
  fixture.componentInstance.leaveForm.subscribe(() => events.push('leave'));
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  return { el, events, fixture };
}
const text = (n: Element | null) => (n?.textContent ?? '').replace(/\s+/g, ' ').trim();
const label = (li: Element) => li.getAttribute('aria-label')?.replace(/ /g, ' ');

describe('ActionGroups: the economy row', () => {
  it('is the first thing in the panel, a list named "O que resta da vez", with the state in words', () => {
    const { el } = setup({ bonusActionUsed: true });
    const groups = el.querySelector('section.groups')!;
    const row = groups.querySelector('.econ')!;
    expect(row).not.toBeNull();
    // It comes before the tabs and stays while any of them is open.
    expect(row.compareDocumentPosition(groups.querySelector('app-tab-list')!)).toBe(
      Node.DOCUMENT_POSITION_FOLLOWING,
    );
    const list = row.querySelector('ul')!;
    expect(list.getAttribute('aria-label')).toBe('O que resta da vez');
    expect([...list.querySelectorAll('li')].map(label)).toEqual([
      'Ação disponível',
      'Ação bônus usada',
      'Reação disponível',
      'Movimento: 12,0 m',
    ]);
  });

  it('has "Mover" as its only button, and it opens the move page', () => {
    const { el, events } = setup();
    const buttons = [...el.querySelectorAll('.econ button')];
    expect(buttons.map(text)).toEqual(['open_withMover']);
    (buttons[0] as HTMLButtonElement).click();
    expect(events).toEqual(['move']);
  });

  it('turns "Mover" off while an answer is awaited, and leaves it out with nothing left to move', () => {
    const waiting = setup({}, { locked: 'Esperando a reação do mestre' });
    const move = waiting.el.querySelector<HTMLButtonElement>('.econ button')!;
    expect(move.getAttribute('aria-disabled') === 'true' || move.disabled).toBe(true);
    move.click();
    expect(waiting.events).toEqual([]);
    TestBed.resetTestingModule();
    const still = setup({ movementLeftFt: 0, movementLeftDft: 0 });
    expect(still.el.querySelector('.econ button')).toBeNull();
    expect([...still.el.querySelectorAll('.econ li')]).toHaveLength(3);
  });

  it('leaves "Mover" to the map header while the master acts for the character', () => {
    const acting = setup({}, { moveAside: true });
    expect(acting.el.querySelector('.econ button')).toBeNull();
    expect([...acting.el.querySelectorAll('.econ li')]).toHaveLength(4);
  });

  it('leaves "Mover" out for a grappled or a surprised character, and for a combat without a map', () => {
    const grappled = setup({}, { contest: create(ContestTurnStateSchema, { grappled: true }) });
    expect(grappled.el.querySelector('.econ button')).toBeNull();
    TestBed.resetTestingModule();
    const surprised = setup({}, { contest: create(ContestTurnStateSchema, { surprised: true }) });
    expect(surprised.el.querySelector('.econ button')).toBeNull();
    TestBed.resetTestingModule();
    const theatre = setup({}, { theatre: true });
    expect(theatre.el.querySelector('.econ button')).toBeNull();
    // With no map the walker keeps the count of the metres.
    expect([...theatre.el.querySelectorAll('.econ li')]).toHaveLength(4);
  });
});

describe('ActionGroups: "Permanecer na forma" after "Voltar"', () => {
  const stay = {
    detail: 'Sem magias · 12,0 m · gasta 1 uso (restam 1 de 2) e o tempo recomeça',
    reason: '',
  };
  const rows = (el: HTMLElement) => [...el.querySelectorAll('app-action-row')];
  const named = (el: HTMLElement, name: string) => rows(el).find((r) => text(r).startsWith(name))!;

  it('comes second, with a bolt and "Sem custo", and presses as a stay', () => {
    const { el, events } = setup({}, { beast: 'Lobo', stay });
    const order = rows(el)
      .map((r) => text(r.querySelector('.row__name, h4, h3, b') ?? r))
      .filter((t) => /Voltar à forma normal|Permanecer na forma/.test(t));
    expect(order.length).toBeGreaterThanOrEqual(2);
    expect(rows(el).indexOf(named(el, 'Voltar à forma normal'))).toBeLessThan(
      rows(el).indexOf(named(el, 'Permanecer na forma')),
    );
    const row = named(el, 'Permanecer na forma');
    expect(text(row)).toContain('Sem custo');
    expect(text(row)).toContain(
      'Sem magias · 12,0 m · gasta 1 uso (restam 1 de 2) e o tempo recomeça',
    );
    row.querySelector('button')!.click();
    expect(events).toEqual(['stay']);
  });

  it('is off, with the reason, with no use left or after the form ended, and spends nothing then', () => {
    for (const reason of [
      'Sem usos de Forma Selvagem · voltam no descanso curto ou longo',
      'A forma já acabou. Para virar fera de novo, use Forma Selvagem.',
    ]) {
      TestBed.resetTestingModule();
      const { el, events } = setup({}, { beast: 'Lobo', stay: { ...stay, reason } });
      const row = named(el, 'Permanecer na forma');
      expect(row.querySelector('button')?.getAttribute('aria-disabled')).toBe('true');
      expect(text(row)).toContain(reason);
      row.querySelector('button')!.click();
      expect(events).toEqual([]);
    }
  });

  it("is not there in the character's own shape", () => {
    const { el } = setup({}, { beast: '', stay: null });
    expect(text(el)).not.toContain('Permanecer na forma');
  });

  it('does not take the action: its row costs nothing even with the Ação and the Bônus spent', () => {
    const { el } = setup({ actionUsed: true, bonusActionUsed: true }, { beast: 'Lobo', stay });
    const row = named(el, 'Permanecer na forma');
    expect(row.querySelector('button')?.getAttribute('aria-disabled')).not.toBe('true');
  });
});
