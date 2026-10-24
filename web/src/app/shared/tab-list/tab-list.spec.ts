import { TestBed } from '@angular/core/testing';

import { TabItem, TabList, tabPanelId } from './tab-list';

const TABS: readonly TabItem[] = [
  { id: 'chat', label: 'Conversa', icon: 'chat', badge: '3', words: '3 mensagens novas' },
  { id: 'sheet', label: 'Ficha' },
  { id: 'log', label: 'Registro' },
];

describe('TabList', () => {
  function setup(selected = '') {
    const fixture = TestBed.createComponent(TabList);
    fixture.componentRef.setInput('label', 'Seções da conversa');
    fixture.componentRef.setInput('tabs', TABS);
    fixture.componentRef.setInput('idPrefix', 'rail');
    if (selected) {
      fixture.componentRef.setInput('selected', selected);
    }
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const tabs = () => Array.from(el.querySelectorAll<HTMLButtonElement>('[role="tab"]'));
    const press = (key: string, on: HTMLElement) => {
      on.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }));
      fixture.detectChanges();
    };
    return { fixture, el, tabs, press };
  }

  it('is a tab list with a name, one selected tab and one tab stop', () => {
    const { el, tabs } = setup();
    expect(el.querySelector('[role="tablist"]')?.getAttribute('aria-label')).toBe(
      'Seções da conversa',
    );
    expect(tabs().map((t) => t.getAttribute('aria-selected'))).toEqual(['true', 'false', 'false']);
    expect(tabs().map((t) => t.tabIndex)).toEqual([0, -1, -1]);
  });

  it('wires each tab to its panel by the ids the panels use', () => {
    const { tabs } = setup();
    expect(tabs()[1].getAttribute('aria-controls')).toBe(tabPanelId('rail', 'sheet'));
    expect(tabs()[1].id).toBe('rail-tab-sheet');
  });

  it('puts the count in the name, in words, and hides the badge from a reader', () => {
    const { tabs } = setup();
    expect(tabs()[0].getAttribute('aria-label')).toBe('Conversa, 3 mensagens novas');
    expect(tabs()[1].getAttribute('aria-label')).toBeNull();
    const badge = tabs()[0].querySelector('.tl__badge');
    expect(badge?.textContent?.trim()).toBe('3');
    expect(badge?.getAttribute('aria-hidden')).toBe('true');
    expect(tabs()[1].querySelector('.tl__badge')).toBeNull();
  });

  it('selects on a click', () => {
    const { fixture, tabs } = setup();
    tabs()[2].click();
    fixture.detectChanges();
    expect(fixture.componentInstance.selected()).toBe('log');
    expect(tabs()[2].getAttribute('aria-selected')).toBe('true');
    expect(tabs()[2].tabIndex).toBe(0);
  });

  it('moves with the arrows, wrapping, and with Home and End, selecting and focusing', () => {
    const { fixture, tabs, press } = setup();
    press('ArrowRight', tabs()[0]);
    expect(fixture.componentInstance.selected()).toBe('sheet');
    expect(document.activeElement).toBe(tabs()[1]);
    press('End', tabs()[1]);
    expect(fixture.componentInstance.selected()).toBe('log');
    press('ArrowRight', tabs()[2]);
    expect(fixture.componentInstance.selected()).toBe('chat');
    press('ArrowLeft', tabs()[0]);
    expect(fixture.componentInstance.selected()).toBe('log');
    press('Home', tabs()[2]);
    expect(fixture.componentInstance.selected()).toBe('chat');
  });

  it('ignores other keys', () => {
    const { fixture, tabs, press } = setup();
    press('a', tabs()[0]);
    expect(fixture.componentInstance.selected()).toBe('');
  });

  it('falls back to the first tab when the selected id is none of them', () => {
    const { tabs } = setup('gone');
    expect(tabs()[0].getAttribute('aria-selected')).toBe('true');
  });
});
