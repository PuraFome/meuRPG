import { ChangeDetectionStrategy, Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import type { PickItem } from '../../../core/levelup/levelup-flow';
import { PickList } from './pick-list';

const items: PickItem[] = [
  'Alarme',
  'Armadura Arcana',
  'Leque Cromático',
  'Luz',
  'Passo Nebuloso',
  'Sono',
].map((name) => ({
  key: `k:${name}`,
  name,
  sub: '1º nível',
}));

@Component({
  imports: [PickList],
  template: `<app-pick-list pickId="spells" title="Livro de magias" noun="magia" nounMany="magias" searchLabel="Buscar magia" [items]="items" [picked]="picked" [count]="count" [base]="base" [describable]="true" (pick)="events.push($event)" (describe)="described.push($event.name)" />`,
})
class Host {
  items = items;
  picked = new Set<string>();
  count = 2;
  base = 0;
  events: string[] = [];
  described: string[] = [];
}

describe('PickList', () => {
  function setup(over: Partial<Host> = {}) {
    const fixture = TestBed.createComponent(Host);
    Object.assign(fixture.componentInstance, over);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    return {
      fixture,
      el,
      host: fixture.componentInstance,
      text: () => el.textContent?.replace(/\s+/g, ' ') ?? '',
    };
  }

  it('says how many of how many, with the warning words while one is missing', () => {
    const { el, text } = setup({ picked: new Set(['k:Luz']) });
    expect(text()).toContain('1 de 2');
    expect(text()).toContain(': falta escolher');
    expect(el.querySelector('app-pick-list')?.hasAttribute('data-missing')).toBe(true);
    expect(el.querySelector('mat-icon')?.textContent).toBe('warning');
  });

  it('shows a tick and no missing mark when complete', () => {
    const { el, text } = setup({ picked: new Set(['k:Luz', 'k:Sono']) });
    expect(text()).toContain('2 de 2');
    expect(text()).toContain(': completo');
    expect(el.querySelector('app-pick-list')?.hasAttribute('data-missing')).toBe(false);
  });

  it('counts what was prepared before, as "9 de 9"', () => {
    const { text } = setup({ base: 7, count: 2, picked: new Set(['k:Luz']) });
    expect(text()).toContain('8 de 9');
  });

  it('has checkboxes for several, and radios for one', () => {
    expect(setup().el.querySelector('.row__input')?.getAttribute('type')).toBe('checkbox');
    TestBed.resetTestingModule();
    expect(setup({ count: 1 }).el.querySelector('.row__input')?.getAttribute('type')).toBe('radio');
  });

  it('says "Ver os outros N" for a masculine noun', () => {
    const fixture = TestBed.createComponent(PickList);
    fixture.componentRef.setInput('pickId', 'cantrips');
    fixture.componentRef.setInput('title', 'Truque novo');
    fixture.componentRef.setInput('noun', 'truque');
    fixture.componentRef.setInput('nounMany', 'truques');
    fixture.componentRef.setInput('masculine', true);
    fixture.componentRef.setInput(
      'items',
      ['A', 'B', 'C', 'D', 'E', 'F'].map((name) => ({ key: `spell:${name}`, name })),
    );
    fixture.componentRef.setInput('picked', new Set<string>());
    fixture.componentRef.setInput('count', 1);
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).textContent).toContain('Ver os outros 2 truques');
  });

  it('shows the first rows and "Ver as outras N" for a feminine noun, and the rest after it', () => {
    const { fixture, el, text } = setup();
    expect(el.querySelectorAll('.row')).toHaveLength(4);
    expect(text()).toContain('Ver as outras 2 magias');
    (el.querySelector('.list__more') as HTMLButtonElement).click();
    fixture.detectChanges();
    expect(el.querySelectorAll('.row')).toHaveLength(6);
    expect(el.querySelector('.list__more')).toBeNull();
  });

  it('keeps a picked row visible even when it is past the first ones', () => {
    const { el } = setup({ picked: new Set(['k:Sono']) });
    expect(Array.from(el.querySelectorAll('.row__name')).map((n) => n.textContent)).toContain(
      'Sono',
    );
  });

  it('searches by name, and says so when nothing matches', () => {
    const { fixture, el, text } = setup();
    const input = el.querySelector('input[type=search]') as HTMLInputElement;
    input.value = 'nebu';
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    expect(Array.from(el.querySelectorAll('.row__name')).map((n) => n.textContent)).toEqual([
      'Passo Nebuloso',
    ]);
    input.value = 'zzz';
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    expect(text()).toContain('Nenhum resultado');
  });

  it('reports a tap, and disables the unpicked rows once the list is full', () => {
    const { fixture, el, host } = setup({ picked: new Set(['k:Luz', 'k:Sono']) });
    const boxes = Array.from(el.querySelectorAll<HTMLInputElement>('input[type=checkbox]'));
    expect(boxes.filter((b) => b.disabled)).toHaveLength(boxes.length - 2);
    host.picked = new Set(['k:Luz']);
    fixture.changeDetectorRef.detectChanges();
    const open = Array.from(el.querySelectorAll<HTMLInputElement>('input[type=checkbox]')).find(
      (b) => !b.checked,
    )!;
    open.dispatchEvent(new Event('change'));
    expect(host.events).toHaveLength(1);
  });

  it('opens the description from the "?" button, named for the spell', () => {
    const { el, host } = setup();
    const help = el.querySelector('.row__help') as HTMLButtonElement;
    expect(help.getAttribute('aria-label')).toBe('Descrição de Alarme');
    help.click();
    expect(host.described).toEqual(['Alarme']);
  });
});

describe('PickList: a spell the master wrote (E10-11 state 4)', () => {
  it('tags a table spell "Da mesa" beside its name, with the word and the icon', () => {
    const fixture = TestBed.createComponent(PickList);
    fixture.componentRef.setInput('pickId', 'spells');
    fixture.componentRef.setInput('title', 'Magias conhecidas');
    fixture.componentRef.setInput('noun', 'magia');
    fixture.componentRef.setInput('nounMany', 'magias');
    fixture.componentRef.setInput('items', [
      { key: 'spell:a', name: 'Alarme', sub: '1º nível' },
      { key: 'spell:ink@mesa', name: 'Lâmina de Nanquim', sub: '1º nível', table: true },
    ]);
    fixture.componentRef.setInput('picked', new Set<string>());
    fixture.componentRef.setInput('count', 1);
    fixture.detectChanges();
    const names = Array.from((fixture.nativeElement as HTMLElement).querySelectorAll('.row__name'));
    expect(names[0].querySelector('.mr-tag')).toBeNull();
    expect(names[1].querySelector('.mr-tag')?.textContent).toContain('Da mesa');
  });
});

describe('PickList: showAll', () => {
  @Component({
    changeDetection: ChangeDetectionStrategy.OnPush,
    imports: [PickList],
    template: `<app-pick-list pickId="prepared" title="Magias preparadas" noun="magia" nounMany="magias" [showAll]="true" [items]="items" [picked]="picked" [count]="2" />`,
  })
  class AllHost {
    items = items;
    picked = new Set<string>();
  }

  it('lists every row of the spells to prepare, with no "Ver as outras" (rehearsal 4: the 5th and later were hidden)', () => {
    const fixture = TestBed.createComponent(AllHost);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(
      el.querySelectorAll('input[type="checkbox"], mat-checkbox').length,
    ).toBeGreaterThanOrEqual(items.length);
    expect(el.textContent).toContain('Passo Nebuloso');
    expect(el.textContent).not.toContain('Ver as outras');
  });
});
