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
        { key: P('special'), namePt: 'Especial' },
        { key: P('heavy'), namePt: 'Pesada' },
        { key: P('finesse'), namePt: 'Acuidade' },
        { key: P('monk'), namePt: 'Monge' },
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
    expect(text(el)).toContain('No livro: 4 lb.');
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

  const chipNames = (el: HTMLElement) =>
    Array.from(el.querySelectorAll('button.tchip')).map((b) =>
      text(b)
        .replace(/^\s*(lock|check)\s/, '')
        .trim(),
    );

  it('lists the properties in the board order: by name, Especial last', () => {
    const { el } = setup();
    expect(chipNames(el)).toEqual([
      'Acuidade',
      'Alcance',
      'Arremesso',
      'Leve',
      'Munição',
      'Pesada',
      'Versátil',
      'Especial',
    ]);
  });

  it('leaves the Monge property out: it is not an SRD 5.1 property (canvas H, #580 S2)', () => {
    const { el } = setup();
    expect(chipNames(el)).not.toContain('Monge');
  });

  it('labels the category and the use, and says what the category means for the attack', () => {
    const { el } = setup();
    const labels = Array.from(el.querySelectorAll('.seg__label, legend, label')).map((l) =>
      text(l).trim(),
    );
    expect(labels).toContain('Categoria');
    expect(labels).toContain('Uso');
    expect(text(el)).toContain(
      'Quem tem proficiência em armas simples soma o bônus de proficiência no ataque.',
    );
  });

  it('names the field "Nome da arma" and counts its letters', () => {
    const { el } = setup();
    expect(text(el)).toContain('Nome da arma');
    expect(text(el)).toContain('14/60');
  });

  it('calls the two-handed damage "Com duas mãos", and says why it is missing without Versátil', async () => {
    const { fixture, el } = setup();
    expect(text(field(el, 'table_weapon.two_handed_damage').closest('app-text-field')!)).toContain(
      'Com duas mãos',
    );
    expect(text(el)).not.toContain('Dano com duas mãos');
    Array.from(el.querySelectorAll<HTMLButtonElement>('button.tchip'))
      .find((b) => text(b).includes('Versátil'))!
      .click();
    await settle(fixture, 2);
    expect(text(el)).toContain('Sem Versátil, não há dano com duas mãos.');
  });

  it('writes the distances in feet under the fields that take meters', () => {
    const { el } = setup();
    expect(text(el)).toContain('20 pés.');
    expect(text(el)).toContain('60 pés: até lá, com desvantagem.');
  });

  it('puts the distances right under the damage, before the properties', () => {
    const { el } = setup();
    const order = Array.from(
      el.querySelectorAll(
        '[data-field="table_weapon.damage"], [data-field="table_weapon.normal_range_ft"], .tchips',
      ),
    ).map((n) => n.getAttribute('data-field') ?? 'chips');
    expect(order).toEqual([
      'table_weapon.damage',
      'table_weapon.normal_range_ft',
      'table_weapon.properties',
    ]);
  });

  it('says how many sheets use the weapon, and that a change reaches the locked ones', () => {
    const { fixture, el } = setup();
    expect(text(el)).not.toContain('Em uso por');
    const e = stored();
    e.charactersUsing = 1;
    fixture.componentRef.setInput('entry', e);
    fixture.detectChanges();
    expect(text(el)).toContain('Em uso por 1 ficha');
    expect(el.querySelector('.card-note mat-icon')!.textContent).toBe('open_in_full');
    expect(el.querySelector('.preview__name mat-icon')!.textContent).toBe('menu_book');
    expect(text(el)).toContain(
      'Mudar o dano ou as propriedades vale na hora, também nos ataques das fichas travadas.',
    );
  });

  it('says on a ranged weapon what board 580-11 says: where the players pick it', () => {
    const { fixture, el } = setup();
    const e = stored();
    e.charactersUsing = 1;
    fixture.componentRef.setInput('entry', e);
    const ranged = Array.from(el.querySelectorAll<HTMLLabelElement>('label.seg__item')).find((l) =>
      text(l).includes('À distância'),
    )!;
    ranged.querySelector('input')!.click();
    fixture.detectChanges();
    expect(text(el.querySelector('.card-note')!)).toContain(
      'Os jogadores a escolhem no editor da ficha, na lista de armas, com as do SRD.',
    );
  });

  it('hides the damage row of the preview until there is a damage, and shows Arremesso plain', () => {
    const { fixture, el } = setup(false);
    const preview = () => text(el.querySelector('.preview')!);
    expect(preview()).not.toContain('Dano');
    type(el, 'table_weapon.damage', '1d4');
    fixture.detectChanges();
    expect(preview()).toContain('Dano');
    const stored1 = setup();
    const p = text(stored1.el.querySelector('.preview')!);
    expect(p).toContain('Arremesso');
    expect(p).not.toContain('Arremesso (');
  });

  it('refuses Leve with Pesada as soon as they are both on', async () => {
    const { fixture, el } = setup(false);
    for (const n of ['Leve', 'Pesada']) {
      Array.from(el.querySelectorAll<HTMLButtonElement>('button.tchip'))
        .find((b) => text(b).includes(n))!
        .click();
      await settle(fixture, 2);
    }
    expect(text(el)).toContain('Leve e Pesada não vão juntas. Tire uma.');
  });

  it('counts the Leve and Pesada clash with the server refusals, and shows "—" in the empty two-handed damage', async () => {
    const { fixture, el } = setup();
    for (const n of ['Leve', 'Pesada']) {
      Array.from(el.querySelectorAll<HTMLButtonElement>('button.tchip'))
        .find((b) => text(b).includes(n))!
        .click();
      await settle(fixture, 2);
    }
    save.mockRejectedValue(
      refusal(
        ['table_weapon.damage', 'bad_value'],
        ['table_weapon.two_handed_damage', 'bad_value'],
        ['table_weapon.long_range_ft', 'bad_range'],
      ),
    );
    click(el, 'Salvar arma');
    await settle(fixture, 3);
    expect(text(el)).toContain('4 campos precisam de ajuste');
    expect(
      (field(el, 'table_weapon.two_handed_damage') as HTMLInputElement).getAttribute('placeholder'),
    ).toBe('—');
  });
});
