import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { SessionTabs } from './session-tabs';

describe('SessionTabs', () => {
  beforeEach(() => TestBed.configureTestingModule({ providers: [provideRouter([])] }));

  function setup(tab: 'scene' | 'sheet' | 'chat' = 'scene', unread = 0, sheetPanel = false) {
    const fixture = TestBed.createComponent(SessionTabs);
    fixture.componentRef.setInput('tab', tab);
    fixture.componentRef.setInput('unread', unread);
    fixture.componentRef.setInput('sheetPanel', sheetPanel);
    fixture.componentRef.setInput('sheetLink', ['/campaigns', 'c1', 'characters', 'pc1']);
    const chosen: string[] = [];
    fixture.componentInstance.choose.subscribe((t) => chosen.push(t));
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    return { fixture, el, chosen };
  }

  it('is a navigation named "Seções da sessão" with Cena, Ficha and Conversa', () => {
    const { el } = setup();
    expect(el.querySelector('nav')?.getAttribute('aria-label')).toBe('Seções da sessão');
    const items = Array.from(el.querySelectorAll('.tab'), (t) => t.textContent?.trim());
    expect(items).toEqual(['Cena', 'Ficha', 'Conversa']);
  });

  it('marks the open section as current', () => {
    const { el } = setup('chat');
    const current = Array.from(el.querySelectorAll('.tab')).filter(
      (t) => t.getAttribute('aria-current') === 'true',
    );
    expect(current.map((t) => t.textContent?.trim())).toEqual(['Conversa']);
  });

  it('links "Ficha" to the given route', () => {
    const { el } = setup();
    const link = el.querySelector('a.tab') as HTMLAnchorElement;
    expect(link.getAttribute('href')).toBe('/campaigns/c1/characters/pc1');
  });

  it('says which section was chosen', () => {
    const { el, chosen } = setup('scene');
    (el.querySelectorAll('button.tab')[1] as HTMLButtonElement).click();
    (el.querySelectorAll('button.tab')[0] as HTMLButtonElement).click();
    expect(chosen).toEqual(['chat', 'scene']);
  });

  it('shows no badge without news', () => {
    expect(setup().el.querySelector('.tab__badge')).toBeNull();
  });

  it('shows the unread count on "Conversa", in words for a screen reader', () => {
    const { el } = setup('scene', 3);
    const chat = el.querySelectorAll('button.tab')[1];
    expect(chat.getAttribute('aria-label')).toBe('Conversa, 3 mensagens novas');
    const badge = chat.querySelector('.tab__badge') as HTMLElement;
    expect(badge.textContent?.trim()).toBe('3');
    expect(badge.getAttribute('aria-hidden')).toBe('true');
  });

  it('says "1 mensagem nova" in the singular and caps the badge at 99+', () => {
    const one = setup('scene', 1);
    expect(one.el.querySelectorAll('button.tab')[1].getAttribute('aria-label')).toBe(
      'Conversa, 1 mensagem nova',
    );
    one.fixture.componentRef.setInput('unread', 120);
    one.fixture.detectChanges();
    expect(one.el.querySelector('.tab__badge')?.textContent?.trim()).toBe('99+');
  });

  describe('with the sheet as a panel of the session (a player with a character)', () => {
    it('draws "Ficha" as a button, not a link', () => {
      const { el } = setup('scene', 0, true);
      expect(el.querySelector('a.tab')).toBeNull();
      expect(Array.from(el.querySelectorAll('button.tab'), (t) => t.textContent?.trim())).toEqual([
        'Cena',
        'Ficha',
        'Conversa',
      ]);
    });

    it('says "sheet" when it is chosen and marks it as current when open', () => {
      const { el, chosen } = setup('sheet', 0, true);
      const sheet = el.querySelectorAll('button.tab')[1] as HTMLButtonElement;
      expect(sheet.getAttribute('aria-current')).toBe('true');
      sheet.click();
      expect(chosen).toEqual(['sheet']);
    });
  });

  describe('for the master outside a combat', () => {
    function master(tab: 'scene' | 'group' | 'chat' = 'scene', groupCount = 0) {
      const made = setup();
      made.fixture.componentRef.setInput('tab', tab);
      made.fixture.componentRef.setInput('master', true);
      made.fixture.componentRef.setInput('groupCount', groupCount);
      made.fixture.detectChanges();
      return made;
    }

    it('is Cena, Grupo and Conversa, all buttons', () => {
      const { el } = master();
      expect(Array.from(el.querySelectorAll('.tab'), (t) => t.textContent?.trim())).toEqual([
        'Cena',
        'Grupo',
        'Conversa',
      ]);
      expect(el.querySelector('a.tab')).toBeNull();
    });

    it('says "group" when Grupo is chosen and marks it current', () => {
      const { el, chosen } = master('group');
      const group = el.querySelectorAll('button.tab')[1] as HTMLButtonElement;
      expect(group.getAttribute('aria-current')).toBe('true');
      group.click();
      expect(chosen).toEqual(['group']);
    });

    it('counts the casts waiting on Grupo, in words', () => {
      const { el } = master('scene', 2);
      const group = el.querySelectorAll('button.tab')[1];
      expect(group.getAttribute('aria-label')).toContain('2');
      expect(group.querySelector('.tab__badge--wait')?.textContent?.trim()).toBe('2');
    });
  });

  it('says "Grupo" for the master and "Combate" while the master\'s combat is on', () => {
    const { fixture, el } = setup();
    fixture.componentRef.setInput('sceneLabel', 'Combate');
    fixture.componentRef.setInput('sheetLabel', 'Grupo');
    fixture.detectChanges();
    expect(Array.from(el.querySelectorAll('.tab'), (t) => t.textContent?.trim())).toEqual([
      'Combate',
      'Grupo',
      'Conversa',
    ]);
  });
});
