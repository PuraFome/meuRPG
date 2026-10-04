import { TestBed } from '@angular/core/testing';

import { note } from '../../../core/notes/notes-testing';
import { ClueNotice } from './clue-notice';

describe('ClueNotice', () => {
  const clue = (scene = 'A carroça tombada') => note('c1', 'Uma pista', new Date(), { clue: true, sceneName: scene, sceneId: scene ? 's1' : '' });

  function render(clues: ReturnType<typeof clue>[]) {
    const fixture = TestBed.createComponent(ClueNotice);
    fixture.componentRef.setInput('clues', clues);
    const opened = vi.fn();
    const dismissed = vi.fn();
    fixture.componentInstance.opened.subscribe(opened);
    fixture.componentInstance.dismissed.subscribe(dismissed);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    return { fixture, el, opened, dismissed, flat: () => el.textContent?.replace(/\s+/g, ' ').trim() };
  }

  it('has its live region in the page even with nothing to say, so a screen reader hears it fill', () => {
    const { el } = render([]);
    expect(el.querySelector('[role="status"]')).not.toBeNull();
    expect(el.querySelector('.mr-notice')).toBeNull();
  });

  it('says a clue arrived and where it is, with "Abrir anotações" in words and a ✕ of 44px that has a name', () => {
    const { el, flat } = render([clue()]);
    expect(flat()).toContain('O mestre revelou uma pista para você. Ela está em Anotações, com a cena marcada.');
    expect(Array.from(el.querySelectorAll('button')).map((b) => b.getAttribute('aria-label') ?? b.textContent?.trim())).toEqual([
      'Abrir anotações',
      'Dispensar o aviso',
    ]);
  });

  it('does not promise a scene tag a clue does not have, and counts more than one', () => {
    expect(render([clue('')]).flat()).toContain('Ela está em Anotações.');
    expect(render([clue(), clue()]).flat()).toContain('O mestre revelou 2 pistas para você. Elas estão em Anotações, com a cena marcada.');
  });

  it('opens the notes or is dismissed, and never goes away by itself', () => {
    const { el, opened, dismissed } = render([clue()]);
    const [open, close] = Array.from(el.querySelectorAll('button'));
    open.click();
    close.click();
    expect(opened).toHaveBeenCalledTimes(1);
    expect(dismissed).toHaveBeenCalledTimes(1);
  });
});
