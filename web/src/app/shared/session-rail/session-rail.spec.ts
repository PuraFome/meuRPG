import { ChangeDetectionStrategy, Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { TabItem } from '../tab-list/tab-list';
import { RailPanel, SessionRail } from './session-rail';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [SessionRail, RailPanel],
  template: `
    <app-session-rail label="Conversa" [tabs]="tabs()" [(selected)]="selected">
      <div appRailPanel="chat"><input class="draft" aria-label="Mensagem" /></div>
      <div appRailPanel="sheet">Ficha</div>
    </app-session-rail>
  `,
})
class Host {
  readonly tabs = signal<readonly TabItem[]>([
    { id: 'chat', label: 'Conversa', badge: '3', words: '3 mensagens novas' },
    { id: 'sheet', label: 'Ficha' },
  ]);
  readonly selected = signal('');
}

describe('SessionRail', () => {
  function setup() {
    const fixture = TestBed.createComponent(Host);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const panel = (id: string) =>
      el.querySelector<HTMLElement>(`[role="tabpanel"]#rail-panel-${id}`)!;
    const tab = (i: number) => el.querySelectorAll<HTMLButtonElement>('[role="tab"]')[i];
    return { fixture, el, panel, tab };
  }

  it('is an aside named by its user', () => {
    const { el } = setup();
    const aside = el.querySelector('app-session-rail')!;
    expect(aside.getAttribute('role')).toBe('complementary');
    expect(aside.getAttribute('aria-label')).toBe('Conversa');
  });

  it('has a tab list with the counts in words', () => {
    const { tab } = setup();
    expect(tab(0).getAttribute('aria-label')).toBe('Conversa, 3 mensagens novas');
    expect(tab(0).getAttribute('aria-selected')).toBe('true');
  });

  it('keeps every panel mounted and hides the inactive one, named by its tab', () => {
    const { fixture, panel, tab } = setup();
    expect(panel('chat').hidden).toBe(false);
    expect(panel('sheet').hidden).toBe(true);
    expect(panel('sheet').getAttribute('aria-labelledby')).toBe('rail-tab-sheet');
    tab(1).click();
    fixture.detectChanges();
    expect(panel('chat').hidden).toBe(true);
    expect(panel('sheet').hidden).toBe(false);
    expect(fixture.componentInstance.selected()).toBe('sheet');
  });

  it('keeps the draft of a hidden panel', () => {
    const { fixture, el, tab } = setup();
    const draft = el.querySelector<HTMLInputElement>('.draft')!;
    draft.value = 'meio escrito';
    tab(1).click();
    fixture.detectChanges();
    tab(0).click();
    fixture.detectChanges();
    expect(el.querySelector<HTMLInputElement>('.draft')?.value).toBe('meio escrito');
  });

  it('draws no tab list for a single tab', () => {
    const { fixture, el } = setup();
    fixture.componentInstance.tabs.set([{ id: 'chat', label: 'Conversa' }]);
    fixture.detectChanges();
    expect(el.querySelector('[role="tablist"]')).toBeNull();
    expect(el.querySelector('#rail-panel-chat')?.getAttribute('aria-labelledby')).toBeNull();
  });
});
