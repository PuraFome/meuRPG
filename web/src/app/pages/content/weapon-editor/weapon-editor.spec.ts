import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import { WeaponCategory } from '../../../../gen/meurpg/rules/v1/rules_pb';
import {
  EffectMenuListSchema,
  TableContentKind,
  TableContentRefusalSchema,
  TableContentViolationSchema,
  TableWeaponSchema,
} from '../../../../gen/meurpg/rules/v1/table_content_pb';
import { TableContentClient } from '../../../core/content/content-client';
import { catalog, entry } from '../../../core/content/content-testing';
import { menuResponse } from '../../../core/content/content-testing-menu';
import { EffectMenuVm } from '../../../core/content/effect-draft';
import { WeaponEditor } from './weapon-editor';
import { settle } from '../../../../test-settle';

const P = (k: string) => `weapon-property:${k}`;

function weaponMenu(): EffectMenuVm {
  const res = menuResponse();
  res.lists.push(
    create(EffectMenuListSchema, {
      name: 'weapon_properties',
      values: [
        { key: P('versatile'), namePt: 'Versátil' },
        { key: P('light'), namePt: 'Leve' },
        { key: P('reach'), namePt: 'Alcance' },
        { key: P('ammunition'), namePt: 'Munição' },
        { key: P('thrown'), namePt: 'Arremesso' },
      ],
    }),
  );
  return new EffectMenuVm(res);
}

function refusal(...violations: [string, string][]) {
  return new ConnectError('refused', Code.InvalidArgument, undefined, [
    {
      desc: TableContentRefusalSchema,
      value: create(TableContentRefusalSchema, {
        violations: violations.map(([field, reason]) =>
          create(TableContentViolationSchema, { field, reason }),
        ),
      }),
    },
  ]);
}

describe('WeaponEditor (#580 S2, board 580-10)', () => {
  const save = vi.fn();
  const stored = () =>
    entry(TableContentKind.WEAPON, 'Lança de Cobre', {
      body: {
        case: 'tableWeapon',
        value: create(TableWeaponSchema, {
          namePt: 'Lança de Cobre',
          category: WeaponCategory.SIMPLE,
          damage: '1d6',
          damageType: 'damage-type:fire',
          twoHandedDamage: '1d8',
          properties: [P('versatile'), P('thrown')],
          normalRangeFt: 20,
          longRangeFt: 60,
          weightOz: 64,
          priceCp: 500,
        }),
      },
    });

  function setup(existing = true) {
    save.mockReset();
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({
      providers: [{ provide: TableContentClient, useValue: { save } }],
    });
    const fixture = TestBed.createComponent(WeaponEditor);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput('catalog', catalog());
    fixture.componentRef.setInput('menu', weaponMenu());
    if (existing) fixture.componentRef.setInput('entry', stored());
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  const text = (e: Element) => (e.textContent ?? '').replace(/ /g, ' ').replace(/\s+/g, ' ');
  const field = (el: HTMLElement, path: string) =>
    el.querySelector<HTMLElement>(`[data-field="${path}"]`)!;
  const type = (el: HTMLElement, path: string, value: string) => {
    const i = field(el, path) as HTMLInputElement;
    i.value = value;
    i.dispatchEvent(new Event('input'));
  };
  const click = (el: HTMLElement, label: string) =>
    Array.from(el.querySelectorAll<HTMLButtonElement>('button'))
      .find((b) => text(b).includes(label))!
      .click();

  it('shows the stored weapon in meters, kilograms and coins', () => {
    const { el } = setup();
    expect((field(el, 'table_weapon.normal_range_ft') as HTMLInputElement).value).toBe('6');
    expect((field(el, 'table_weapon.long_range_ft') as HTMLInputElement).value).toBe('18');
    expect((field(el, 'table_weapon.weight_oz') as HTMLInputElement).value).toBe('2');
    expect((field(el, 'table_weapon.price_cp') as HTMLInputElement).value).toBe('5');
    expect(text(el)).toContain('No livro: 4 lb');
  });

  it('asks for the two-handed damage only while the weapon is Versátil', async () => {
    const { fixture, el } = setup();
    expect(field(el, 'table_weapon.two_handed_damage')).toBeTruthy();
    const chips = Array.from(el.querySelectorAll<HTMLButtonElement>('button.tchip')).find((b) =>
      (b.textContent ?? '').includes('Versátil'),
    )!;
    chips.click();
    await settle(fixture, 2);
    expect(field(el, 'table_weapon.two_handed_damage')).toBeFalsy();
  });

  it('turns meters, kilograms and coin back into feet, ounces and copper on save', async () => {
    const { fixture, el } = setup();
    type(el, 'table_weapon.weight_oz', '1,5');
    type(el, 'table_weapon.price_cp', '12');
    fixture.detectChanges();
    save.mockResolvedValue({ entry: stored(), affected: [] });
    click(el, 'Salvar arma');
    await settle(fixture, 3);
    const w = save.mock.calls[0][2].value;
    expect([w.normalRangeFt, w.longRangeFt, w.weightOz, w.priceCp]).toEqual([20, 60, 48, 1200]);
  });

  it('does not save a weight that is no number, and lands on it', async () => {
    const { fixture, el } = setup();
    type(el, 'table_weapon.weight_oz', 'pesada');
    fixture.detectChanges();
    click(el, 'Salvar arma');
    await settle(fixture, 3);
    expect(save).not.toHaveBeenCalled();
    expect(document.activeElement).toBe(field(el, 'table_weapon.weight_oz'));
    expect(text(field(el, 'table_weapon.weight_oz').closest('app-text-field')!)).toContain(
      'Escreva um número',
    );
  });

  it('says what the combination of properties is as soon as it is on the form', async () => {
    const { fixture, el } = setup(false);
    Array.from(el.querySelectorAll<HTMLButtonElement>('button.tchip'))
      .find((b) => (b.textContent ?? '').includes('Munição'))!
      .click();
    await settle(fixture, 2);
    expect(text(el)).toContain('Munição só numa arma à distância.');
  });

  it('puts the server refusal of the damage at the damage field, in Portuguese', async () => {
    const { fixture, el } = setup();
    save.mockRejectedValue(refusal(['table_weapon.damage', 'bad_value']));
    click(el, 'Salvar arma');
    await settle(fixture, 3);
    expect(text(field(el, 'table_weapon.damage').closest('app-text-field')!)).toContain(
      'Use dados de d4',
    );
  });
});
