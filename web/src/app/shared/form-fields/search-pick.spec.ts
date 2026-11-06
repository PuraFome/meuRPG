import { ChangeDetectionStrategy, Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import {
  type SearchChip,
  type SearchGroup,
  type SearchOption,
  SearchPick,
  searchSections,
} from './search-pick';

const groups: SearchGroup[] = [
  { id: 'feat', title: 'Talentos', empty: 'Nenhum talento com esse nome.' },
  { id: 'trait', title: 'Traços', empty: 'Nenhum traço com esse nome.' },
];
const options: SearchOption[] = [
  { value: 'a', label: 'Agarrador', hint: 'talento do SRD', group: 'feat' },
  { value: 'b', label: 'Atleta', hint: '', group: 'feat' },
  { value: 'c', label: 'Visão no Escuro', hint: 'traço do SRD', group: 'trait' },
];

describe('searchSections', () => {
  it('filters by name without case or accents and keeps a group with no match, to say so', () => {
    const s = searchSections(options, groups, 'agar');
    expect(s.map((x) => [x.group?.id, x.options.map((o) => o.value)])).toEqual([
      ['feat', ['a']],
      ['trait', []],
    ]);
    expect(searchSections(options, groups, 'VISAO')[1].options).toHaveLength(1);
  });

  it('lists everything on an empty search, and has one section with no groups', () => {
    expect(searchSections(options, groups, '  ').flatMap((x) => x.options)).toHaveLength(3);
    const flat = searchSections(options, [], 'a');
    expect(flat).toHaveLength(1);
    expect(flat[0].group).toBeNull();
  });
});

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [SearchPick],
  template: `
    <app-search-pick label="Não pode ter" placeholder="Procurar" path="p.lacks" [options]="options" [groups]="groups" [chips]="chips()" [issues]="issues()" hint="Dica." (added)="added.push($event)" (removed)="removed.push($event)" />
  `,
})
class Host {
  readonly options = options;
  readonly groups = groups;
  readonly chips = signal<SearchChip[]>([
    { value: 'x', label: 'Corujeiro', hint: 'talento do SRD', tag: 'Desligada' },
  ]);
  readonly issues = signal<string[]>([]);
  readonly added: string[] = [];
  readonly removed: string[] = [];
}

describe('app-search-pick', () => {
  let fixture: ReturnType<typeof TestBed.createComponent<Host>>;
  let el: HTMLElement;
  const input = () => el.querySelector<HTMLInputElement>('input[data-field="p.lacks"]')!;
  const text = (e: Element) => (e.textContent ?? '').replace(/\s+/g, ' ');
  const type = (value: string) => {
    input().value = value;
    input().dispatchEvent(new Event('input'));
    fixture.detectChanges();
  };

  beforeEach(() => {
    TestBed.resetTestingModule();
    fixture = TestBed.createComponent(Host);
    fixture.detectChanges();
    el = fixture.nativeElement;
  });

  it('opens a grouped dropdown on focus and says which group has no match', () => {
    expect(el.querySelector('[role="listbox"]')).toBeNull();
    input().dispatchEvent(new Event('focus'));
    type('agar');
    expect(input().getAttribute('aria-expanded')).toBe('true');
    expect(text(el.querySelector('[role="listbox"]')!)).toContain('Talentos');
    expect(text(el.querySelector('[role="listbox"]')!)).toContain('Agarrador · talento do SRD');
    expect(text(el.querySelector('[role="listbox"]')!)).toContain('Nenhum traço com esse nome.');
  });

  it('adds the clicked option, clears the search and closes', () => {
    type('atl');
    el.querySelector<HTMLElement>('[role="option"]')!.click();
    fixture.detectChanges();
    expect(fixture.componentInstance.added).toEqual(['b']);
    expect(input().value).toBe('');
    expect(el.querySelector('[role="listbox"]')).toBeNull();
  });

  it('picks with the arrow keys and Enter, and closes on Escape', () => {
    type('');
    input().dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown' }));
    input().dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown' }));
    fixture.detectChanges();
    expect(input().getAttribute('aria-activedescendant')).toBe(
      el.querySelectorAll('[role="option"]')[1].id,
    );
    input().dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' }));
    expect(fixture.componentInstance.added).toEqual(['b']);
    type('a');
    input().dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    fixture.detectChanges();
    expect(el.querySelector('[role="listbox"]')).toBeNull();
  });

  it('shows the chip with its origin and tag, and takes it off', () => {
    const chip = el.querySelector('.chip')!;
    expect(text(chip)).toContain('Corujeiro · talento do SRD');
    expect(text(chip)).toContain('Desligada');
    chip.querySelector<HTMLButtonElement>('button[aria-label="Tirar Corujeiro"]')!.click();
    expect(fixture.componentInstance.removed).toEqual(['x']);
  });

  it('shows the refusal under the field and marks it invalid', () => {
    fixture.componentInstance.issues.set(['Esta raça ou sub-raça não existe mais. Escolha outra.']);
    fixture.detectChanges();
    expect(text(el)).toContain('Esta raça ou sub-raça não existe mais.');
    expect(input().getAttribute('aria-invalid')).toBe('true');
  });
});
