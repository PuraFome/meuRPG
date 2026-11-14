import { ChangeDetectionStrategy, Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import {
  type AttunementEntryVm,
  MagicCategories,
  magicCategoryIcon,
} from '../../../core/characters/attunement';
import type { FullSheetVm } from '../character-sheet.types';
import { EquipmentPanel } from './equipment-panel';

const flat = (n: Element | null | undefined) => n?.textContent?.replace(/\s+/g, ' ').trim();

const entry = (over: Partial<AttunementEntryVm>): AttunementEntryVm => ({
  id: 'm1',
  key: 'ring',
  namePt: 'Anel de proteção',
  kind: 'magic',
  attuned: false,
  asks: true,
  restrictionPt: '',
  restriction: 'none',
  sameItemAttuned: false,
  ...over,
});

const piece = (key: string, namePt: string, equipped = true) => ({
  key,
  namePt,
  kind: 'weapon' as const,
  equipped,
  twoHanded: false,
  armorClassIfToggled: 0,
  count: 1,
});

function sheetWith(entries: AttunementEntryVm[], gear: unknown[] = []): FullSheetVm {
  return {
    armorClass: 14,
    gear,
    equipment: [],
    backgroundEquipment: '',
    coins: { cp: 0, sp: 0, ep: 0, gp: 0, pp: 0 },
    shieldIdle: false,
    attunement: { entries, attuned: entries.filter((e) => e.asks && e.attuned).length },
  } as unknown as FullSheetVm;
}

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [EquipmentPanel],
  template: `
    <app-equipment-panel
      [sheet]="sheet()"
      [viewer]="viewer()"
      characterName="Tor"
      [canEquip]="true"
      [canManageMagic]="manage()"
      [canChangeAttunement]="change()"
      [attunementIsClosed]="closed()"
      [magicNotice]="notice()"
      (attunementOpened)="opened = opened + 1"
      (magicAdding)="adding = adding + 1"
      (magicRemoving)="removed.push($event.id)"
    />
  `,
})
class Host {
  readonly sheet = signal(sheetWith([]));
  readonly viewer = signal<'owner' | 'master'>('owner');
  readonly manage = signal(false);
  readonly change = signal(false);
  readonly closed = signal(false);
  readonly notice = signal('');
  opened = 0;
  adding = 0;
  readonly removed: string[] = [];
}

function render(init: (h: Host) => void = () => undefined) {
  const fixture = TestBed.createComponent(Host);
  init(fixture.componentInstance);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  return { fixture, host: fixture.componentInstance, el };
}

const CATEGORY_OF = new Map([
  ['ring', 'armor'],
  ['cloak', 'wondrous-item'],
]);

describe('EquipmentPanel, the magic items and the attunement', () => {
  beforeEach(() =>
    TestBed.configureTestingModule({
      providers: [
        {
          provide: MagicCategories,
          useValue: { iconOf: (k: string) => magicCategoryIcon(CATEGORY_OF.get(k)) },
        },
      ],
    }),
  );

  it('shows neither the header nor the list when nothing is magic and nobody can add', () => {
    const { el } = render();
    expect(el.querySelector('.att-head')).toBeNull();
    expect(el.querySelector('.magic')).toBeNull();
  });

  it('counts what is attuned in the header and words each row as a pill', () => {
    const { el } = render((h) =>
      h.sheet.set(
        sheetWith([
          entry({ attuned: true }),
          entry({ id: 'm2', key: 'cloak', namePt: 'Manto élfico' }),
          entry({ id: 'm3', key: 'bag', namePt: 'Bolsa de carga', asks: false }),
        ]),
      ),
    );
    expect(flat(el.querySelector('.att-head'))).toContain('Sintonizados: 1 de 3');
    expect(flat(el.querySelector('#magic-heading'))).toContain('Itens mágicos 3');
    const rows = Array.from(el.querySelectorAll('.magic-row'), flat);
    expect(rows[0]).toContain('Anel de proteçãoSintonizado');
    expect(rows[1]).toContain('Manto élficoPede sintonização');
    expect(rows[2]).toContain('Bolsa de carga');
    expect(rows[2]).not.toContain('sintonização');
  });

  it('says who fails what an item asks, the same words as the rest sheet', () => {
    const unmet = entry({ restrictionPt: 'Só para um conjurador', restriction: 'notMet' });
    const owner = render((h) => h.sheet.set(sheetWith([unmet])));
    expect(flat(owner.el.querySelector('.magic-row__line'))).toContain(
      'Só para um conjurador: você não cumpre.',
    );
    expect(owner.el.querySelector('.magic-row__line mat-icon')?.textContent).toBe('block');

    const master = render((h) => {
      h.viewer.set('master');
      h.sheet.set(sheetWith([unmet]));
    });
    expect(flat(master.el.querySelector('.magic-row__line'))).toContain(
      'Só para um conjurador: Tor não cumpre.',
    );
  });

  it('draws a condition of use with the moon, not with the block that means a refusal', () => {
    const { el } = render((h) =>
      h.sheet.set(sheetWith([entry({ restrictionPt: 'Só ao ar livre, à noite' })])),
    );
    expect(flat(el.querySelector('.magic-row__line'))).toContain('Só ao ar livre, à noite.');
    expect(el.querySelector('.magic-row__line mat-icon')?.textContent).toBe('dark_mode');
  });

  it("draws the icon of the item's category by its key", () => {
    const { el } = render((h) =>
      h.sheet.set(
        sheetWith([
          entry({}),
          entry({ id: 'm2', key: 'cloak', namePt: 'Manto' }),
          entry({ id: 'm3', key: 'unknown', namePt: 'Outro' }),
        ]),
      ),
    );
    const icons = Array.from(el.querySelectorAll('.magic-row__icon'), (i) => i.textContent);
    expect(icons).toEqual(['shield', 'diamond', 'diamond']);
  });

  it('offers "Mudar" while the window is open, and "Muda num descanso" when it is not', () => {
    const open = render((h) => {
      h.sheet.set(sheetWith([entry({})]));
      h.change.set(true);
    });
    const button = open.el.querySelector<HTMLButtonElement>('.att-head__button')!;
    expect(button.getAttribute('aria-label')).toBe('Mudar sintonização');
    button.click();
    expect(open.host.opened).toBe(1);
    expect(open.el.querySelector('.att-head__later')).toBeNull();

    const shut = render((h) => h.sheet.set(sheetWith([entry({})])));
    expect(flat(shut.el.querySelector('.att-head__later'))).toBe('Muda num descanso');
    expect(shut.el.querySelector('.att-head__button')).toBeNull();
  });

  it('says neither for a dead character', () => {
    const { el } = render((h) => {
      h.sheet.set(sheetWith([entry({})]));
      h.closed.set(true);
    });
    expect(el.querySelector('.att-head__later')).toBeNull();
    expect(el.querySelector('.att-head__button')).toBeNull();
  });

  it('gives whoever manages the items "Adicionar item mágico" and a "Remover item" menu on each row', () => {
    const { el, host } = render((h) => {
      h.sheet.set(sheetWith([entry({})]));
      h.manage.set(true);
    });
    el.querySelector<HTMLButtonElement>('.magic__add')!.click();
    expect(host.adding).toBe(1);
    const more = el.querySelector<HTMLButtonElement>('.magic-row__more')!;
    expect(more.getAttribute('aria-label')).toBe('Remover item · Anel de proteção');
    more.click();
    expect(host.removed).toEqual(['m1']);
  });

  it('says "Nenhum item mágico." for a manager with an empty list, and gives the others no controls', () => {
    const manager = render((h) => h.manage.set(true));
    expect(flat(manager.el.querySelector('.magic'))).toContain('Nenhum item mágico.');
    const other = render((h) => h.sheet.set(sheetWith([entry({})])));
    expect(other.el.querySelector('.magic__add')).toBeNull();
    expect(other.el.querySelector('.magic-row__more')).toBeNull();
  });

  it('keeps the last add or remove in a live region', () => {
    const { el } = render((h) => {
      h.sheet.set(sheetWith([entry({})]));
      h.notice.set('Item mágico adicionado.');
    });
    expect(flat(el.querySelector('.magic [role=status]'))).toBe('Item mágico adicionado.');
  });

  it('marks a gear row with the chip of its attunement: attuned, or asking for it', () => {
    const { el } = render((h) =>
      h.sheet.set(
        sheetWith(
          [
            entry({
              id: 'bonus:equipment:rapier',
              kind: 'piece',
              gearKey: 'equipment:rapier',
              attuned: true,
            }),
            entry({ id: 'bonus:equipment:mace', kind: 'piece', gearKey: 'equipment:mace' }),
          ],
          [
            piece('equipment:rapier', 'Rapieira'),
            piece('equipment:mace', 'Maça'),
            piece('equipment:dagger', 'Adaga'),
          ],
        ),
      ),
    );
    const rows = Array.from(el.querySelectorAll('.gear-row'));
    expect(flat(rows[0].querySelector('.gear-row__att'))).toBe('linkSintonizado');
    expect(rows[0].querySelector('.gear-row__att--on')).not.toBeNull();
    expect(flat(rows[1].querySelector('.gear-row__att'))).toBe('linkPede sintonização');
    expect(rows[1].querySelector('.gear-row__att--on')).toBeNull();
    expect(rows[2].querySelector('.gear-row__att')).toBeNull();
  });

  it('says what the lock leaves to the owner, once nobody here can add or mark', () => {
    const { el } = render((h) => h.sheet.set(sheetWith([entry({})])));
    expect(flat(el.querySelector('.gear__lock'))).toContain(
      'Travada: equipar e guardar continuam valendo; marcas, bônus e itens mágicos, só o mestre muda.',
    );
  });

  it('has no lock line for whoever can manage the items', () => {
    const { el } = render((h) => h.manage.set(true));
    expect(el.querySelector('.gear__lock')).toBeNull();
  });

  it('puts the armor class on the "Equipado" head, not on a line of its own', () => {
    const { el } = render((h) =>
      h.sheet.set(sheetWith([entry({})], [piece('equipment:mace', 'Maça')])),
    );
    expect(el.querySelector('.gear__ac')).toBeNull();
    expect(flat(el.querySelector('.gear-group__ac'))).toBe('CA 14');
    const said = Array.from(el.querySelectorAll('.mr-visually-hidden')).find((n) =>
      flat(n)?.includes('Classe de armadura'),
    )!;
    expect(flat(said)).toBe('Classe de armadura 14');
    expect(said.classList).toContain('mr-visually-hidden');
  });

  it('marks the shield "Parado" on its row when a two-handed weapon is equipped', () => {
    const shield = { ...piece('equipment:shield', 'Escudo'), kind: 'shield' as const };
    const { el } = render((h) =>
      h.sheet.set({ ...sheetWith([], [shield]), shieldIdle: true } as FullSheetVm),
    );
    expect(flat(el.querySelector('.gear-row--idle .gear-row__idle'))).toBe('Parado');
  });

  it('draws the legend of the chips on the rows: the marks, the attunement and the bonus', () => {
    const { el } = render((h) =>
      h.sheet.set(
        sheetWith(
          [],
          [
            {
              ...piece('equipment:rapier', 'Rapieira'),
              marks: { silvered: true, adamantine: false, magical: true },
              bonus: 1,
              bonusCounts: true,
            },
          ],
        ),
      ),
    );
    const legend = el.querySelector('ul.legend')!;
    expect(legend.getAttribute('aria-label')).toBe('Legenda dos selos');
    expect(Array.from(legend.querySelectorAll('li')).map((li) => flat(li))).toEqual([
      expect.stringContaining('Prateada'),
      expect.stringContaining('Mágica'),
      expect.stringContaining('Bônus mágico, já somado'),
    ]);
  });

  it('reads the legend’s bonus once: the chip is hidden and the words stay', () => {
    const { el } = render((h) =>
      h.sheet.set(
        sheetWith([], [{ ...piece('equipment:rapier', 'Rapieira'), bonus: 1, bonusCounts: true }]),
      ),
    );
    const li = el.querySelector('ul.legend li')!;
    expect(li.querySelector('[aria-label]')).toBeNull();
    expect(li.querySelector('.bonus-chip')!.getAttribute('aria-hidden')).toBe('true');
    expect(flat(li)).toContain('Bônus mágico, já somado');
  });

  it('says the legend’s bonus does not count, with a dashed chip, when every bonus is dashed', () => {
    const { el } = render((h) =>
      h.sheet.set(
        sheetWith([], [{ ...piece('equipment:rapier', 'Rapieira'), bonus: 1, bonusCounts: false }]),
      ),
    );
    const li = el.querySelector('ul.legend li')!;
    expect(flat(li)).toContain('Bônus mágico, não conta agora');
    expect(li.querySelector('.bonus-chip--idle')).not.toBeNull();
  });

  it('draws no legend when no row has a chip', () => {
    const { el } = render((h) => h.sheet.set(sheetWith([], [piece('equipment:mace', 'Maça')])));
    expect(el.querySelector('ul.legend')).toBeNull();
  });
});
