import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import { MapPointKind, MapPointSchema, TrapState } from '../../../../../gen/meurpg/maps/v1/maps_pb';
import { TrapTrigger } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { TrapCard } from './trap-card';

const point = (state: TrapState, extra = {}) =>
  create(MapPointSchema, {
    id: 'p',
    kind: MapPointKind.TRAP,
    name: 'Fosso escondido',
    description: 'No corredor',
    trap: { state, noticeDc: 15, findDc: 15, areaSize: 2, trigger: TrapTrigger.ENTER, presetKey: 'trap:hidden-pit', effect: { damage: [{ dice: '2d6', damageTypeKey: 'damage-type:bludgeoning', damageTypePt: 'concussão' }] } },
    ...extra,
  });

function setup(state: TrapState, open = true, extra = {}) {
  const fixture = TestBed.createComponent(TrapCard);
  fixture.componentRef.setInput('point', point(state, extra));
  fixture.componentRef.setInput('open', open);
  const calls: string[] = [];
  const c = fixture.componentInstance;
  c.toggle.subscribe(() => calls.push('toggle'));
  c.reveal.subscribe(() => calls.push('reveal'));
  c.fire.subscribe(() => calls.push('fire'));
  c.extend.subscribe(() => calls.push('extend'));
  c.disarm.subscribe(() => calls.push('disarm'));
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const button = (t: string) => Array.from(el.querySelectorAll('button')).find((b) => b.textContent?.includes(t))!;
  return { fixture, el, calls, button };
}

describe('TrapCard', () => {
  it('an open armed card has the DCs, the trigger, the effect without a final period ("Queda" for a pit) and three actions', () => {
    const { el, button, calls } = setup(TrapState.ARMED);
    const text = el.textContent!.replace(/\s+/g, ' ');
    expect(text).toContain('Armada');
    expect(text).toContain('Só você vê');
    expect(text).toContain('Ao entrar na área');
    expect(text).toContain('Queda: 2d6 de concussão');
    expect(text).not.toContain('concussão.');
    for (const name of ['Revelar para…', 'Disparar…', 'Desarmar']) {
      button(name).click();
    }
    expect(calls).toEqual(['reveal', 'fire', 'disarm']);
  });

  it('the toggle says "Ver detalhes" and "Esconder detalhes", never a trap verb', () => {
    const closed = setup(TrapState.ARMED, false);
    expect(closed.button('Ver detalhes')).toBeTruthy();
    expect(closed.el.textContent).not.toContain('Abrir a armadilha');
    expect(setup(TrapState.ARMED).button('Esconder detalhes')).toBeTruthy();
  });

  it('a fired card offers "Marcar como desarmada" and "Disparar em mais alguém" (only when it can be extended)', () => {
    const { fixture, el, button, calls } = setup(TrapState.TRIGGERED);
    expect(el.textContent).toContain('Disparada');
    expect(el.textContent).toContain('Visível para todos');
    expect(Array.from(el.querySelectorAll('button')).some((b) => b.textContent?.includes('Disparar em mais alguém'))).toBe(false);
    fixture.componentRef.setInput('canExtend', true);
    fixture.detectChanges();
    button('Disparar em mais alguém').click();
    button('Marcar como desarmada').click();
    expect(calls).toEqual(['extend', 'disarm']);
  });

  it('a disarmed card has no action; one everybody already sees disables "Revelar para…" with the reason', () => {
    const { el } = setup(TrapState.DISARMED);
    expect(el.textContent).toContain('Desarmada');
    expect(el.querySelectorAll('.tc__actions button')).toHaveLength(0);
    const seen = setup(TrapState.ARMED, true, { revealed: true });
    expect(seen.button('Revelar para…').getAttribute('aria-disabled')).toBe('true');
    expect(seen.el.textContent).toContain('Todos já veem esta armadilha.');
  });
});
