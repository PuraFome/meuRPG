import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import { CombatantSchema } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { ExtraActionOptionSchema } from '../../../../../gen/meurpg/play/v1/lasting_effects_pb';
import {
  ActionOptionSchema,
  AttackKind,
  AttackOptionSchema,
  DisabledReasonCode,
  TurnOptionsSchema,
} from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { reasonText } from '../../../../core/combat/combat-options';
import { ActionGroups } from './action-groups';

const plain = (t: string | null | undefined) => (t ?? '').replace(/\s+/g, ' ').trim();

const longsword = create(AttackOptionSchema, {
  enabled: true,
  attack: {
    key: 'attack:longsword',
    name: 'Longsword',
    namePt: 'Espada longa',
    kind: AttackKind.WEAPON,
    attackBonus: 5,
    saveDc: 0,
    rangeFt: 5,
    damage: '1d8 + 3',
    damageTypePt: 'cortante',
  },
});

const haste = (over: object = {}) =>
  create(ExtraActionOptionSchema, {
    available: true,
    labelPt: 'Ação extra (Velocidade)',
    allowedActions: ['attack', 'dash', 'disengage', 'hide', 'use-an-object'],
    allowedTextPt: 'Só: Atacar (uma arma), Disparada, Desengajar, Esconder, Usar um objeto',
    ...over,
  });

describe('ActionGroups: the extra action an effect gives (RN-22)', () => {
  function setup(extra: ReturnType<typeof haste> | null, locked = '') {
    const fixture = TestBed.createComponent(ActionGroups);
    fixture.componentRef.setInput('options', create(TurnOptionsSchema, { attacks: [longsword] }));
    fixture.componentRef.setInput(
      'own',
      create(CombatantSchema, { movementLeftFt: 30, speedFt: 30 }),
    );
    fixture.componentRef.setInput('extraAction', extra);
    fixture.componentRef.setInput('locked', locked);
    const attacks: string[] = [];
    const standards: string[] = [];
    fixture.componentInstance.extraAttack.subscribe((k) => attacks.push(k));
    fixture.componentInstance.extraStandard.subscribe((k) => standards.push(k));
    fixture.detectChanges();
    return { el: fixture.nativeElement as HTMLElement, attacks, standards };
  }
  const section = (el: HTMLElement) =>
    el.querySelector<HTMLElement>('section[aria-labelledby="g-extra"]');
  const button = (el: HTMLElement, label: string) =>
    Array.from(
      el.querySelectorAll<HTMLButtonElement>('section[aria-labelledby="g-extra"] button'),
    ).find(
      (b) => plain(b.textContent).includes(label) || b.getAttribute('aria-label')?.includes(label),
    )!;

  it('draws no group without the effect', () => {
    expect(section(setup(null).el)).toBeNull();
  });

  it('draws its own group with the label, "Disponível" and what it may be', () => {
    const { el } = setup(haste());
    const g = section(el)!;
    expect(plain(g.querySelector('h3')?.textContent)).toBe('Ação extra (Velocidade)');
    expect(plain(g.textContent)).toContain('Disponível');
    expect(plain(g.textContent)).toContain(
      'Só: Atacar (uma arma), Disparada, Desengajar, Esconder, Usar um objeto',
    );
    expect(Array.from(g.querySelectorAll('.std__btn')).map((b) => plain(b.textContent))).toEqual([
      'Disparada',
      'Desengajar',
      'Esconder',
      'Usar um objeto',
    ]);
  });

  it('spends it on a weapon attack or on a standard action, by key', () => {
    const { el, attacks, standards } = setup(haste());
    button(el, 'Atacar com Espada longa').click();
    button(el, 'Disparada').click();
    button(el, 'Usar um objeto').click();
    expect(attacks).toEqual(['attack:longsword']);
    expect(standards).toEqual(['standard:dash', 'standard:use-an-object']);
  });

  it('keeps the buttons in place but off, with the reason, once it is used', () => {
    const { el, attacks, standards } = setup(
      haste({ available: false, reasonPt: 'Você já usou a ação extra neste turno.' }),
    );
    const g = section(el)!;
    expect(plain(g.textContent)).toContain('Usada');
    const why = g.querySelector('#extra-why')!;
    expect(plain(why.textContent)).toContain('Você já usou a ação extra neste turno.');
    const dash = button(el, 'Disparada');
    expect(dash.getAttribute('aria-disabled')).toBe('true');
    expect(dash.getAttribute('aria-describedby')).toBe('extra-why');
    dash.click();
    expect(standards).toEqual([]);
    expect(attacks).toEqual([]);
  });
});

describe('the reasons an effect writes (RN-22)', () => {
  const reason = (over: object) =>
    create(ActionOptionSchema, { reason: { code: DisabledReasonCode.UNSPECIFIED, ...over } })
      .reason;

  it('reads the sentence of the server, and the master the one with the cause', () => {
    const r = reason({ textPt: 'Você não pode agir.', textMasterPt: 'Paralisado por Orla.' });
    expect(reasonText(r)).toBe('Você não pode agir.');
    expect(reasonText(r, true)).toBe('Paralisado por Orla.');
    expect(reasonText(reason({ textPt: 'Indisponível: Impedido.' }), true)).toBe(
      'Indisponível: Impedido.',
    );
  });

  it('keeps the words of the other codes', () => {
    expect(reasonText(reason({ code: DisabledReasonCode.ACTION_USED }))).toBe('Ação já usada');
  });
});
