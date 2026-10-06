import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { Code, ConnectError } from '@connectrpc/connect';

import type { PuzzleRun } from '../../../../../gen/meurpg/play/v1/puzzles_pb';
import { textOf } from '../../../../core/format/text-testing';
import { PuzzleSessionState } from '../../../../core/puzzles/puzzle-session';
import { PuzzlesClient } from '../../../../core/puzzles/puzzles-client';
import { FakePuzzlesClient, NOW, asClient, at, lightsPuzzle, lockPuzzle, pillarsPuzzle, playerRun } from '../../../../core/puzzles/puzzles-testing';
import { PuzzlePlayPage } from './puzzle-play';

@Component({
  imports: [PuzzlePlayPage],
  template: `<app-puzzle-play campaignId="camp-1" [puzzleId]="id" [session]="session" [state]="state" ownName="Toren" [reconnecting]="reconnecting()" />`,
})
class Host {
  id = 'a';
  session = { sessionId: 's7', sessionNumber: 7, startedAt: new Date() };
  state!: PuzzleSessionState;
  reconnecting = signal(false);
}

const lit = (on: number[]) => Array.from({ length: 25 }, (_, i) => on.includes(i));

describe('PuzzlePlayPage (MR-038, RN-27, RN-10; E10-06 states 6 to 9)', () => {
  let api: FakePuzzlesClient;
  let host: Host;

  // The page reads the real clock ("agora há pouco", the frame on what just changed) and the runs are dated from the
  // fixture's NOW: the clock is NOW, or the tests depend on the hour they run at (they failed after 21:12 on 06/10).
  // Only Date is faked: the page's own timers stay real.
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(NOW);
  });

  async function render(run: PuzzleRun, id = 'a') {
    api = new FakePuzzlesClient();
    api.playerRunResult = run;
    TestBed.configureTestingModule({ providers: [provideRouter([]), { provide: PuzzlesClient, useValue: api }] });
    const fixture = TestBed.createComponent(Host);
    host = fixture.componentInstance;
    host.id = id;
    host.state = new PuzzleSessionState(asClient(api), () => 'camp-1', () => false);
    document.body.append(fixture.nativeElement);
    const settle = async () => {
      for (let i = 0; i < 3; i++) {
        fixture.detectChanges();
        await fixture.whenStable();
        await new Promise((r) => setTimeout(r));
      }
      fixture.detectChanges();
    };
    await settle();
    return { el: fixture.nativeElement as HTMLElement, settle, fixture };
  }

  afterEach(() => document.body.replaceChildren());

  const lights = (partial: Parameters<typeof playerRun>[1] = {}) =>
    playerRun(lightsPuzzle('a', 'O selo da Capela'), {
      clue: 'Só o selo apagado abre o caminho.',
      hints: ['A luz do selo responde ao toque.'],
      state: { kind: { case: 'lights', value: { lit: lit([8, 11, 12]) } } },
      ...partial,
    });

  it('opens with the title, the session, the live status and the way back to the session', async () => {
    const { el } = await render(lights());
    expect(el.querySelector('h1')?.textContent).toBe('O selo da Capela');
    expect(textOf(el)).toContain('Quebra-cabeça da Sessão 7');
    expect(el.querySelector('app-live-pill')?.textContent).toContain('Ao vivo');
    const back = el.querySelector('a.back')!;
    expect(back.textContent).toContain('Voltar para a sessão');
    expect(back.getAttribute('href')).toBe('/campanhas/camp-1/sessao');
  });

  it('reads the master\'s clue as a quote, the hints released, and how many lights are lit', async () => {
    const { el } = await render(lights());
    expect(textOf(el.querySelector('.clue'))).toContain('A pista do mestre “Só o selo apagado abre o caminho.”');
    expect(textOf(el.querySelector('.hints'))).toContain('Dicas soltas pelo mestre 1 A luz do selo responde ao toque.');
    expect(textOf(el.querySelector('.info'))).toContain('Toque numa luz: ela e as quatro vizinhas trocam. Apague todas.');
    expect(textOf(el.querySelector('.info'))).toContain('Luzes acesas: 3');
  });

  it('plays: a tap on a light goes to the server with a key, and the board follows what the server answers', async () => {
    const { el, settle } = await render(lights());
    api.moveResult = () => ({ run: lights({ revision: 2, state: { kind: { case: 'lights', value: { lit: lit([0, 1, 5]) } } }, lastMove: { characterName: 'Toren', move: { kind: { case: 'lights', value: { row: 0, col: 0 } } }, changed: [0, 1, 5], at: at(0) } }), replayed: false, solvedByThisMove: false });
    (el.querySelector('[aria-label="Luz na linha 1, coluna 1, apagada"]') as HTMLElement).click();
    await settle();
    const call = api.calls.find((c) => c[0] === 'move')!;
    expect(call[2]).toBe('a');
    expect(call[3]).toEqual({ kind: { case: 'lights', value: { row: 0, col: 0 } } });
    expect(typeof call[4]).toBe('string');
    expect(el.querySelector('[aria-label="Luz na linha 1, coluna 1, acesa"]')).not.toBeNull();
    expect(textOf(el.querySelector('.info'))).toContain('Luzes acesas: 3');
    // Your own move is not outlined as "just changed by someone else", but a screen reader hears it.
    expect(el.querySelectorAll('.cell--changed')).toHaveLength(0);
    expect(el.querySelector('[role="status"][aria-live="polite"]')?.textContent).toContain('Você tocou numa luz. 3 acesas.');
  });

  it('outlines what another person just changed, and says who and when', async () => {
    const { el } = await render(lights({ lastMove: { characterName: 'Lia', move: { kind: { case: 'lights', value: { row: 1, col: 1 } } }, changed: [6, 1, 5, 7, 11], at: at(1) } }));
    expect(el.querySelectorAll('.cell--changed')).toHaveLength(5);
    expect(textOf(el.querySelector('.info__last'))).toContain('Lia tocou numa luz agora há pouco.');
  });

  it('reads the run again when the session page says this puzzle changed', async () => {
    const { el, settle } = await render(lights());
    api.playerRunResult = lights({ revision: 5, hints: ['A luz do selo responde ao toque.', 'Cada toque troca cinco luzes de uma vez.'] });
    await host.state.changed('a');
    await settle();
    expect(el.querySelectorAll('.hints__list li')).toHaveLength(2);
  });

  it('is solved: "Resolvido", the board stops, and the master\'s own words with the time', async () => {
    const { el } = await render(lights({ solved: true, solvedAt: at(0), solvedByName: 'Brisa', solvedMessage: 'A porta da Capela se abriu.', state: { kind: { case: 'lights', value: { lit: lit([]) } } } }));
    expect(el.querySelector('.mr-notice--success')?.textContent).toContain('Resolvido');
    expect(textOf(el.querySelector('.info'))).toContain('O quebra-cabeça terminou. Todas as luzes estão apagadas.');
    expect(textOf(el.querySelector('.info__solved'))).toContain('A porta da Capela se abriu.');
    const cell = el.querySelector('button.cell') as HTMLButtonElement;
    expect(cell.getAttribute('aria-disabled')).toBe('true');
    cell.click();
    expect(api.calls.some((c) => c[0] === 'move')).toBe(false);
    // The page never says what the master chose to do (RN-10).
    expect(textOf(el)).not.toMatch(/Ao resolver|solução|mínimo/);
  });

  it('is stopped by a limit: the neutral line, and nothing moves', async () => {
    const { el } = await render(lights({ stopped: true, stoppedMessage: 'O quebra-cabeça parou. Ninguém joga mais até o mestre recomeçar ou fechar.' }));
    expect(el.querySelector('.mr-notice--neutral')?.textContent).toContain('O quebra-cabeça parou.');
    (el.querySelector('button.cell') as HTMLElement).click();
    expect(api.calls.some((c) => c[0] === 'move')).toBe(false);
  });

  it('says a refusal in words and keeps the board', async () => {
    const { el, settle } = await render(lights());
    api.moveResult = () => {
      throw new ConnectError('x', Code.PermissionDenied);
    };
    (el.querySelector('button.cell') as HTMLElement).click();
    await settle();
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('Você não pode fazer isso agora.');
    expect(el.querySelectorAll('button.cell')).toHaveLength(25);
  });

  it('says the master closed it, and leaves the way back', async () => {
    const { el, settle } = await render(lights());
    api.failWith = new ConnectError('x', Code.NotFound);
    await host.state.changed('a');
    await settle();
    expect(el.querySelector('.mr-notice--neutral')?.textContent).toContain('O mestre fechou o quebra-cabeça.');
    expect(el.querySelector('a.back')).not.toBeNull();
  });

  it('says there is nothing to open when the puzzle was never shown', async () => {
    api = new FakePuzzlesClient();
    api.failWith = new ConnectError('x', Code.NotFound);
    TestBed.configureTestingModule({ providers: [provideRouter([]), { provide: PuzzlesClient, useValue: api }] });
    const fixture = TestBed.createComponent(Host);
    fixture.componentInstance.state = new PuzzleSessionState(asClient(api), () => 'camp-1', () => false);
    fixture.detectChanges();
    for (let i = 0; i < 3; i++) {
      await fixture.whenStable();
      fixture.detectChanges();
    }
    expect((fixture.nativeElement as HTMLElement).textContent).toContain('O mestre não está mostrando este quebra-cabeça.');
  });

  it('says "Reconectando…" instead of "Ao vivo" while the stream is down', async () => {
    const { el, settle } = await render(lights());
    host.reconnecting.set(true);
    await settle();
    expect(el.querySelector('.head__status')?.textContent).toContain('Reconectando…');
    expect(el.querySelector('app-live-pill')).toBeNull();
  });

  describe('the lock', () => {
    const lock = () =>
      playerRun(lockPuzzle('a', 'O cofre do Refeitório'), {
        clue: 'O fogo nasce antes da lua, e a raiz vê tudo.',
        state: { kind: { case: 'lock', value: { wheels: [0, 0, 2, 5] } } },
        lastMove: { characterName: 'Toren', move: { kind: { case: 'lock', value: { wheel: 2, delta: 1 } } }, changed: [2], at: at(40) },
      });

    it('reads the wheels in words, never the solution, and turns a wheel with an arrow', async () => {
      const { el, settle } = await render(lock());
      expect(textOf(el.querySelector('.info'))).toContain('As rodas agora: Lua · Lua · Onda · Estrela');
      expect(textOf(el.querySelector('.info'))).toContain('Você girou a 3ª roda');
      expect(textOf(el.querySelector('.info'))).toContain('Quando todas as rodas estiverem certas, o mestre é avisado.');
      expect(el.querySelector('[aria-label="Roda 3: Onda"]')).not.toBeNull();
      api.moveResult = () => ({ run: lock(), replayed: false, solvedByThisMove: false });
      (el.querySelector('[aria-label="Próximo símbolo: Roda 2"]') as HTMLElement).click();
      await settle();
      expect(api.calls.find((c) => c[0] === 'move')![3]).toEqual({ kind: { case: 'lock', value: { wheel: 1, delta: 1 } } });
      expect(el.querySelector('[aria-label="Solução da fechadura"]')).toBeNull();
    });
  });

  describe('the pillars', () => {
    const pillars = () =>
      playerRun(pillarsPuzzle('a', 'Os pilares da Galeria'), {
        clue: 'Os pilares obedecem ao mural.',
        mural: { pillars: [1, 0, 3, 2] },
        state: { kind: { case: 'pillars', value: { pillars: [3, 0, 2, 1] } } },
        lastMove: { characterName: 'Lia', move: { kind: { case: 'pillars', value: { pillar: 0, delta: 1 } } }, changed: [0, 1], at: at(12) },
      });

    it('shows the mural to copy, the rule of the links, and who turned what', async () => {
      const { el, settle } = await render(pillars());
      expect(textOf(el.querySelector('.mural'))).toContain('O mural');
      expect(textOf(el.querySelector('.mural'))).toContain('Lobo Corvo Coruja Serpente');
      expect(textOf(el.querySelector('.mural'))).toContain('Deixe os pilares como o mural.');
      expect(textOf(el.querySelector('.info'))).toContain('Girar um pilar gira também o da esquerda e o da direita.');
      expect(textOf(el.querySelector('.info__last'))).toContain('Lia girou o pilar 1');
      expect(textOf(el.querySelector('.info__last'))).toContain('Os pilares 1 e 2 mudaram.');
      api.moveResult = () => ({ run: pillars(), replayed: false, solvedByThisMove: false });
      (el.querySelector('[aria-label="Girar o pilar 4"]') as HTMLElement).click();
      await settle();
      expect(api.calls.find((c) => c[0] === 'move')![3]).toEqual({ kind: { case: 'pillars', value: { pillar: 3, delta: 1 } } });
    });
  });

  it('draws a 7 × 7 board with all 49 lights', async () => {
    const puzzle = lightsPuzzle('a', 'Os candelabros da cripta', { config: { kind: { case: 'lights', value: { size: 7 } } } });
    const { el } = await render(playerRun(puzzle, { state: { kind: { case: 'lights', value: { lit: Array.from({ length: 49 }, (_, i) => i % 3 === 0) } } } }));
    expect(el.querySelectorAll('button.cell')).toHaveLength(49);
    expect(el.querySelector('[role="group"]')?.getAttribute('aria-label')).toBe('Painel de luzes, 7 por 7');
  });
});
