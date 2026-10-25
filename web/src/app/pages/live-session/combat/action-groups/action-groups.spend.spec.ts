import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';

import { CombatantSchema } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  ActionEconomy,
  ActionOptionSchema,
  DisabledReasonCode,
  TurnOptionsSchema,
} from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { ActionGroups } from './action-groups';

/** Invented table features: one spends 1 to 3 points of a pool to heal 1d6 per point, one spends exactly 2 for a bonus action. */
function feature(
  key: string,
  namePt: string,
  over: { cost?: number; costMax?: number; usesLeft?: number; enabled?: boolean } = {},
) {
  const enabled = over.enabled ?? true;
  return create(ActionOptionSchema, {
    action: {
      key,
      namePt,
      economy: ActionEconomy.BONUS_ACTION,
      resourceKey: 'reserva_de_teste',
      cost: over.cost ?? 0,
      costMax: over.costMax ?? 0,
      heal: { count: 1, sides: 6 },
      healPerPoint: { count: 1, sides: 6 },
    },
    usesLeft: over.usesLeft ?? 5,
    enabled,
    reason: enabled ? undefined : { code: DisabledReasonCode.NO_USES },
  });
}

describe('ActionGroups: a table feature that spends several points (quantos?)', () => {
  function setup(...features: ReturnType<typeof feature>[]) {
    const fixture = TestBed.createComponent(ActionGroups);
    fixture.componentRef.setInput(
      'options',
      create(TurnOptionsSchema, { featureActions: features }),
    );
    fixture.componentRef.setInput(
      'own',
      create(CombatantSchema, { movementLeftFt: 25, speedFt: 25 }),
    );
    const plain: string[] = [];
    const spends: { key: string; points: number }[] = [];
    fixture.componentInstance.feature.subscribe((k) => plain.push(k));
    fixture.componentInstance.spend.subscribe((s) => spends.push(s));
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const use = (name: string) =>
      el.querySelector<HTMLButtonElement>(`button[aria-label="Usar ${name}"]`)!;
    const press = (b: HTMLButtonElement) => {
      b.click();
      fixture.detectChanges();
    };
    const ask = () => el.querySelector('app-spend-ask');
    return { el, plain, spends, use, press, ask };
  }

  const FOLEGO = () => feature('feature:folego-da-mesa', 'Fôlego da Mesa', { costMax: 3 });

  it('asks "Quantos?" in place instead of using it, with the range and the live line', () => {
    const { plain, spends, use, press, ask } = setup(FOLEGO());
    expect(ask()).toBeNull();
    press(use('Fôlego da Mesa'));
    expect(ask()?.textContent).toContain('Quantos?');
    expect(ask()?.textContent).toContain('de 1 a 3');
    expect(ask()?.querySelector('[role="status"]')?.textContent?.trim()).toBe('Gasta 1 · cura 1d6');
    expect(plain).toEqual([]);
    expect(spends).toEqual([]);
  });

  it('narrows the range to the uses left', () => {
    const { use, press, ask } = setup(
      feature('feature:folego-da-mesa', 'Fôlego da Mesa', { costMax: 3, usesLeft: 2 }),
    );
    press(use('Fôlego da Mesa'));
    expect(ask()?.textContent).toContain('de 1 a 2');
  });

  it('sends the number picked with the key of the action, and closes the question', () => {
    const { plain, spends, use, press, ask, el } = setup(FOLEGO());
    press(use('Fôlego da Mesa'));
    press(el.querySelector<HTMLButtonElement>('button[aria-label="Gastar um a mais"]')!);
    press(el.querySelector<HTMLButtonElement>('button[aria-label="Gastar um a mais"]')!);
    press(
      el.querySelector<HTMLButtonElement>('button[aria-label="Usar Fôlego da Mesa gastando 3"]')!,
    );
    expect(spends).toEqual([{ key: 'feature:folego-da-mesa', points: 3 }]);
    expect(plain).toEqual([]);
    expect(ask()).toBeNull();
  });

  it('"Cancelar" closes the question and sends nothing', () => {
    const { plain, spends, use, press, ask, el } = setup(FOLEGO());
    press(use('Fôlego da Mesa'));
    const cancel = [...el.querySelectorAll<HTMLButtonElement>('app-spend-ask button')].find(
      (b) => b.textContent?.trim() === 'Cancelar',
    )!;
    press(cancel);
    expect(ask()).toBeNull();
    expect(plain).toEqual([]);
    expect(spends).toEqual([]);
  });

  it('says what a use costs in the row, and what is left', () => {
    const { el } = setup(FOLEGO());
    expect(el.querySelector('.row__detail')?.textContent?.trim()).toBe('Gasta de 1 a 3 · restam 5');
  });

  it('is off with "Faltam usos" when fewer are left than the cost, and a tap does nothing', () => {
    const { el, plain, spends, use, press, ask } = setup(
      feature('feature:folego-da-mesa', 'Fôlego da Mesa', { cost: 2, costMax: 4, usesLeft: 1 }),
    );
    const button = use('Fôlego da Mesa');
    expect(button.getAttribute('aria-disabled')).toBe('true');
    expect(el.querySelector('.row__why')?.textContent).toContain('Faltam usos');
    press(button);
    expect(ask()).toBeNull();
    expect(plain).toEqual([]);
    expect(spends).toEqual([]);
  });

  it('leaves a fixed-cost action as it was: the tap uses it at once, with no question', () => {
    const { plain, spends, use, press, ask, el } = setup(
      feature('feature:grito-de-teste', 'Grito de Teste', { cost: 2, usesLeft: 5 }),
    );
    expect(el.querySelector('.row__detail')?.textContent?.trim()).toBe('Gasta 2 · restam 5');
    press(use('Grito de Teste'));
    expect(ask()).toBeNull();
    expect(plain).toEqual(['feature:grito-de-teste']);
    expect(spends).toEqual([]);
  });

  it('leaves an ordinary one-use feature with its usual line', () => {
    const { el } = setup(feature('feature:outro', 'Outro Traço', { usesLeft: 2 }));
    expect(el.querySelector('.row__detail')?.textContent?.trim()).toBe('2 usos');
  });
});
