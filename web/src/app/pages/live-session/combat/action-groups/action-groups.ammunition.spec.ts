import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import { CombatantSchema } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  AttackKind,
  AttackOptionSchema,
  TurnOptionsSchema,
} from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { ActionGroups } from './action-groups';

const plain = (t: string | null | undefined) => (t ?? '').replace(/\s+/g, ' ').trim();

function bow(extra: Record<string, unknown> = {}, key = 'equipment:longbow') {
  return create(AttackOptionSchema, {
    enabled: true,
    attack: {
      key,
      name: 'Longbow',
      namePt: 'Arco longo',
      kind: AttackKind.WEAPON,
      attackBonus: 5,
      rangeFt: 150,
      longRangeFt: 600,
      damage: '1d8 + 2',
      damageTypePt: 'perfurante',
      ...extra,
    },
  });
}
const melee = create(AttackOptionSchema, {
  enabled: true,
  attack: {
    key: 'equipment:longbow:melee',
    name: 'Longbow, melee',
    namePt: 'Arco longo, corpo a corpo',
    kind: AttackKind.WEAPON,
    improvised: true,
    attackBonus: 3,
    rangeFt: 5,
    damage: '1d4 + 3',
    damageTypePt: 'concussão',
  },
});

function setup(attacks: ReturnType<typeof bow>[]) {
  const own = create(CombatantSchema, { id: 'me', label: 'Tor', mine: true });
  const fixture = TestBed.createComponent(ActionGroups);
  fixture.componentRef.setInput('options', create(TurnOptionsSchema, { attacks }));
  fixture.componentRef.setInput('own', own);
  const pressed: string[] = [];
  fixture.componentInstance.attack.subscribe((k: string) => pressed.push(k));
  fixture.detectChanges();
  return { el: fixture.nativeElement as HTMLElement, fixture, pressed };
}
const button = (el: HTMLElement, name: string) =>
  el.querySelector<HTMLButtonElement>(`button[aria-label="${name}"]`)!;
const tag = (el: HTMLElement) => el.querySelector<HTMLElement>('[data-testid="ammo-tag"]');

describe('ActionGroups: the ammunition on the attack row', () => {
  it('shows no tag and an open button for a kind the app does not count', () => {
    const { el } = setup([bow()]);
    expect(tag(el)).toBeNull();
    expect(button(el, 'Atacar com Arco longo').getAttribute('aria-disabled')).not.toBe('true');
  });

  it('counts the pieces beside the name, with the button open', () => {
    const { el, pressed } = setup([bow({ ammunitionLeft: 7 })]);
    expect(plain(tag(el)?.textContent)).toBe('7 flechas');
    expect(tag(el)?.classList.contains('row__ammo--warning')).toBe(false);
    button(el, 'Atacar com Arco longo').click();
    expect(pressed).toEqual(['equipment:longbow']);
  });

  it('warns at 3 or fewer, in words that say it and not by color alone', () => {
    const { el } = setup([bow({ ammunitionLeft: 3 })]);
    expect(plain(tag(el)?.textContent)).toContain('3 flechas');
    expect(tag(el)?.classList.contains('row__ammo--warning')).toBe(true);
    expect(tag(el)?.querySelector('mat-icon')?.textContent).toBe('warning');
  });

  it('turns "Atacar" off at 0 with the reason in sight and wired to the button', () => {
    const { el, pressed } = setup([bow({ ammunitionLeft: 0 })]);
    expect(plain(tag(el)?.textContent)).toBe('0 flechas');
    expect(tag(el)?.classList.contains('row__ammo--empty')).toBe(true);
    const off = button(el, 'Atacar com Arco longo');
    expect(off.getAttribute('aria-disabled')).toBe('true');
    const why = el.querySelector<HTMLElement>('.row__why')!;
    expect(plain(why.textContent)).toContain('Sem flechas: o tiro não sai.');
    expect(off.getAttribute('aria-describedby')).toBe(why.id);
    off.click();
    expect(pressed).toEqual([]);
  });

  it('gives the improvised melee line its own row, with the note, no tag and an open button', () => {
    const { el, pressed } = setup([bow({ ammunitionLeft: 0 }), melee]);
    const open = button(el, 'Atacar com Arco longo, corpo a corpo');
    expect(open.getAttribute('aria-disabled')).not.toBe('true');
    const rows = [...el.querySelectorAll('app-action-row')];
    const row = rows.find((r) => r.contains(open))!;
    expect(plain(row.textContent)).toContain('Improvisado: não gasta flecha.');
    expect(plain(row.textContent)).toContain('1d4 + 3 concussão');
    expect(row.querySelector('[data-testid="ammo-tag"]')).toBeNull();
    open.click();
    expect(pressed).toEqual(['equipment:longbow:melee']);
  });

  it('follows the options: a new count after a shot changes the tag and lifts the block', () => {
    const { el, fixture } = setup([bow({ ammunitionLeft: 1 })]);
    expect(plain(tag(el)?.textContent)).toContain('1 flecha');
    fixture.componentRef.setInput(
      'options',
      create(TurnOptionsSchema, { attacks: [bow({ ammunitionLeft: 0 })] }),
    );
    fixture.detectChanges();
    expect(plain(tag(el)?.textContent)).toBe('0 flechas');
    expect(button(el, 'Atacar com Arco longo').getAttribute('aria-disabled')).toBe('true');
    fixture.componentRef.setInput(
      'options',
      create(TurnOptionsSchema, { attacks: [bow({ ammunitionLeft: 20 })] }),
    );
    fixture.detectChanges();
    expect(button(el, 'Atacar com Arco longo').getAttribute('aria-disabled')).not.toBe('true');
  });

  it('says once, under the list, what spends ammunition, and only while a kind is counted', () => {
    const counted = setup([bow({ ammunitionLeft: 5 }), melee]);
    expect(plain(counted.el.querySelector('[data-testid="ammo-rule"]')?.textContent)).toContain(
      'Cada tiro gasta uma flecha: o Ataque Extra gasta duas.',
    );
    TestBed.resetTestingModule();
    const none = setup([bow()]);
    expect(none.el.querySelector('[data-testid="ammo-rule"]')).toBeNull();
  });
});
