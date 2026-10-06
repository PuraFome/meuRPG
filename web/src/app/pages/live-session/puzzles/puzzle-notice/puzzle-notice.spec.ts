import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import type { PuzzleSummary } from '../../../../../gen/meurpg/play/v1/puzzles_pb';
import { textOf } from '../../../../core/format/text-testing';
import { lightsPuzzle, lockPuzzle, summary } from '../../../../core/puzzles/puzzles-testing';
import { PuzzleNotice } from './puzzle-notice';

@Component({ imports: [PuzzleNotice], template: `<app-puzzle-notice campaignId="camp-1" [puzzles]="puzzles()" />` })
class Host {
  puzzles = signal<PuzzleSummary[]>([]);
}

describe('PuzzleNotice (MR-038, E10-06 state 6)', () => {
  async function render(list: PuzzleSummary[] = []) {
    TestBed.configureTestingModule({ providers: [provideRouter([])] });
    const fixture = TestBed.createComponent(Host);
    fixture.componentInstance.puzzles.set(list);
    const settle = async () => {
      fixture.detectChanges();
      await fixture.whenStable();
      fixture.detectChanges();
    };
    await settle();
    return { el: fixture.nativeElement as HTMLElement, host: fixture.componentInstance, settle };
  }

  it('is nothing at all while the master shows nothing', async () => {
    const { el } = await render();
    expect(el.querySelector('section')).toBeNull();
  });

  it('shows a card for the shown puzzle with "Abrir o quebra-cabeça", a link into the session with the puzzle in the address', async () => {
    const { el } = await render([summary(lightsPuzzle('a', 'O selo da Capela'))]);
    expect(textOf(el.querySelector('section'))).toContain('O mestre mostrou um quebra-cabeça');
    expect(textOf(el.querySelector('section'))).toContain('O selo da Capela Apagar as luzes · todos jogam juntos');
    const open = el.querySelector('a')!;
    expect(open.textContent?.trim()).toBe('Abrir o quebra-cabeça');
    expect(open.getAttribute('href')).toBe('/campanhas/camp-1/sessao?quebra-cabeca=a');
  });

  it('fills only the first unsolved card, so the page never has two filled buttons', async () => {
    const { el } = await render([summary(lightsPuzzle('a', 'A')), summary(lockPuzzle('b', 'B'))]);
    const [first, second] = Array.from(el.querySelectorAll('a'));
    expect(first.className).toContain('mat-mdc-unelevated-button');
    expect(second.className).not.toContain('mat-mdc-unelevated-button');
  });

  it('says a solved or stopped puzzle in words, and keeps the way back to it', async () => {
    const { el } = await render([summary(lightsPuzzle('a', 'A'), { solved: true }), summary(lockPuzzle('b', 'B'), { stopped: true })]);
    const cards = Array.from(el.querySelectorAll('section')).map((s) => textOf(s));
    expect(cards[0]).toContain('Resolvido pelo grupo');
    expect(cards[0]).toContain('Ver o quebra-cabeça');
    expect(cards[1]).toContain('O quebra-cabeça parou');
  });

  it('announces a puzzle that arrives later, once, and not the ones that were there when the page opened', async () => {
    const { el, host, settle } = await render([summary(lightsPuzzle('a', 'A'))]);
    const live = () => el.querySelector('[role="status"]')?.textContent?.trim();
    expect(live()).toBe('');
    host.puzzles.set([summary(lightsPuzzle('a', 'A')), summary(lockPuzzle('b', 'O cofre do Refeitório'))]);
    await settle();
    expect(live()).toBe('O mestre mostrou o quebra-cabeça O cofre do Refeitório.');
  });
});
