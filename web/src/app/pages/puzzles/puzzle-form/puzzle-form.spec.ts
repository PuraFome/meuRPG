import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter, Router } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import { GetMapLayersResponseSchema, GetMapResponseSchema, MapSchema } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { PuzzleInvalidReason, PuzzleInvalidSchema, PuzzleSolveAction } from '../../../../gen/meurpg/play/v1/puzzles_pb';
import { MapsClient } from '../../../core/maps/maps-client';
import { type PuzzleAccess, PuzzleAccessCheck } from '../../../core/puzzles/puzzle-access';
import { PuzzlesClient } from '../../../core/puzzles/puzzles-client';
import { FakePuzzlesClient, lightsPuzzle, lockPuzzle, pillarsPuzzle, preview } from '../../../core/puzzles/puzzles-testing';
import { PuzzleForm } from './puzzle-form';

@Component({ template: 'campanha' })
class Stub {}

const lit17 = Array.from({ length: 25 }, (_, i) => i < 17);

describe('PuzzleForm (MR-038, E10-06 state 2)', () => {
  let api: FakePuzzlesClient;
  let access: PuzzleAccess;

  async function open(url = '/campanhas/camp-1/quebra-cabecas/novo', prep: (a: FakePuzzlesClient) => void = () => undefined) {
    api = new FakePuzzlesClient();
    api.previewResult = preview(lit17, 4, 7n);
    prep(api);
    const maps = {
      list: async () => [create(MapSchema, { id: 'm1', name: 'A capela' })],
      get: async () => create(GetMapResponseSchema, {}),
      layers: async () => create(GetMapLayersResponseSchema, {}),
    };
    TestBed.configureTestingModule({
      providers: [
        provideRouter([
          { path: 'campanhas/:id/quebra-cabecas/novo', component: PuzzleForm },
          { path: 'campanhas/:id/quebra-cabecas/:puzzleId/editar', component: PuzzleForm },
          { path: 'campanhas/:id', component: Stub },
        ]),
        { provide: PuzzlesClient, useValue: api },
        { provide: MapsClient, useValue: maps },
        { provide: PuzzleAccessCheck, useValue: { check: async () => access } },
      ],
    });
    const harness = await RouterTestingHarness.create();
    const component = await harness.navigateByUrl(url, PuzzleForm);
    const settle = async () => {
      for (let i = 0; i < 4; i++) {
        harness.detectChanges();
        await harness.fixture.whenStable();
        await vi.advanceTimersByTimeAsync(0);
      }
      harness.detectChanges();
    };
    const pause = async () => {
      await vi.advanceTimersByTimeAsync(300);
      await settle();
    };
    await settle();
    const el = harness.routeNativeElement as HTMLElement;
    return { el, settle, pause, component, harness };
  }

  const button = (el: HTMLElement, text: string) => Array.from(el.querySelectorAll('button, a')).find((b) => b.textContent?.trim().includes(text)) as HTMLElement;
  const type = (input: HTMLInputElement | HTMLTextAreaElement, value: string) => {
    input.value = value;
    input.dispatchEvent(new Event('input'));
  };

  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
    access = { status: 'master', campaignName: 'Mirathel' };
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('opens on "O tipo" with "Apagar as luzes" marked, and asks the server for a start', async () => {
    const { el, pause } = await open();
    expect(el.querySelector('h1')?.textContent).toBe('Novo quebra-cabeça');
    const kinds = Array.from(el.querySelectorAll('.opt__title')).slice(0, 3).map((t) => t.textContent?.trim());
    expect(kinds).toEqual(['Apagar as luzes', 'Fechadura de combinação', 'Símbolos giratórios']);
    expect(el.querySelector<HTMLInputElement>('input[value="lights"]')?.checked).toBe(true);
    await pause();
    expect(api.calls.find((c) => c[0] === 'previewStart')).toBeTruthy();
    expect(el.textContent).toContain('17 acesas, 8 apagadas.');
    expect(el.textContent).toContain('Dá para resolver em 4 toques.');
    expect(el.textContent).toContain('É só uma prévia: os jogadores é que jogam.');
    // The preview is a picture: no light in it is a button.
    expect(el.querySelectorAll('app-lights-board button')).toHaveLength(0);
    expect(el.querySelectorAll('app-lights-board [role="img"]')).toHaveLength(25);
  });

  it('draws another start with "Gerar outro começo" and saves the one on screen, seed and all', async () => {
    const { el, pause, settle } = await open(undefined, (a) => (a.createResult = lightsPuzzle('new', 'O selo da Capela')));
    await pause();
    api.previewResult = preview(Array.from({ length: 25 }, (_, i) => i % 2 === 0), 3, 99n);
    button(el, 'Gerar outro começo').click();
    await settle();
    expect(el.textContent).toContain('13 acesas, 12 apagadas.');
    expect(el.textContent).toContain('Dá para resolver em 3 toques.');
    type(el.querySelector<HTMLInputElement>('input[name="name"]')!, 'O selo da Capela');
    await settle();
    (el.querySelector('form') as HTMLFormElement).requestSubmit();
    await settle();
    const created = api.calls.find((c) => c[0] === 'create')!;
    expect(created[1]).toBe('camp-1');
    const init = created[2] as { name: string; seed: bigint; config: unknown };
    expect(init.name).toBe('O selo da Capela');
    expect(init.seed).toBe(99n);
    expect(init.config).toEqual({ kind: { case: 'lights', value: { size: 5 } } });
    expect(TestBed.inject(Router).url).toBe('/campanhas/camp-1');
  });

  it('asks for another start when the size changes, after a short pause', async () => {
    const { el, pause, settle } = await open();
    await pause();
    const before = api.calls.filter((c) => c[0] === 'previewStart').length;
    el.querySelector<HTMLInputElement>('input[value="7"]')!.click();
    await settle();
    expect(api.calls.filter((c) => c[0] === 'previewStart').length).toBe(before);
    expect(el.textContent).toContain('Sorteando um começo...');
    await pause();
    const calls = api.calls.filter((c) => c[0] === 'previewStart');
    expect(calls.length).toBe(before + 1);
    expect((calls.at(-1)![2] as { kind: { value: { size: number } } }).kind.value.size).toBe(7);
  });

  it('refuses to save without a name, says so under the field and puts the focus there', async () => {
    const { el, pause, settle } = await open();
    document.body.append(el);
    await pause();
    (el.querySelector('form') as HTMLFormElement).requestSubmit();
    await settle();
    expect(api.calls.some((c) => c[0] === 'create')).toBe(false);
    expect(el.textContent).toContain('Dê um nome ao quebra-cabeça.');
    expect(document.activeElement).toBe(el.querySelector('input[name="name"]'));
    el.remove();
  });

  it('makes a lock with the solution and the start the master chose, and no start is asked of the server', async () => {
    const { el, settle } = await open(undefined, (a) => (a.createResult = lockPuzzle('new', 'O cofre')));
    el.querySelector<HTMLInputElement>('input[value="lock"]')!.click();
    await settle();
    expect(el.querySelector('h2#form-title')?.textContent).toBe('Fechadura de combinação');
    expect(api.calls.filter((c) => c[0] === 'previewStart')).toHaveLength(1 - 1 + api.calls.filter((c) => c[0] === 'previewStart').length); // none after switching
    type(el.querySelector<HTMLInputElement>('input[name="name"]')!, 'O cofre do Refeitório');
    // Turn the solution's first wheel up once: [1, 0, 0, 0] against the start [0, 0, 0, 1].
    el.querySelector<HTMLElement>('[aria-label="Próximo símbolo: Roda 1"]')!.click();
    await settle();
    (el.querySelector('form') as HTMLFormElement).requestSubmit();
    await settle();
    const init = api.calls.find((c) => c[0] === 'create')![2] as { solution: unknown; start: unknown; seed: bigint };
    expect(init.solution).toEqual({ kind: { case: 'lock', value: { wheels: [1, 0, 0, 0] } } });
    expect(init.start).toEqual({ kind: { case: 'lock', value: { wheels: [0, 0, 0, 1] } } });
    expect(init.seed).toBe(0n);
  });

  it('says a lock starts solved before the server has to', async () => {
    const { el, settle } = await open();
    el.querySelector<HTMLInputElement>('input[value="lock"]')!.click();
    await settle();
    type(el.querySelector<HTMLInputElement>('input[name="name"]')!, 'x');
    // The start's last wheel from 1 back to 0: the same as the solution.
    el.querySelectorAll<HTMLElement>('[aria-label="Símbolo anterior: Roda 4"]')[1].click();
    await settle();
    (el.querySelector('form') as HTMLFormElement).requestSubmit();
    await settle();
    expect(api.calls.some((c) => c[0] === 'create')).toBe(false);
    expect(el.textContent).toContain('O começo é igual à solução.');
  });

  it('sends the pillars\' mural and links, and says when the links make the mural impossible', async () => {
    const { el, settle, pause } = await open(undefined, (a) => (a.createResult = pillarsPuzzle('new', 'Os pilares')));
    el.querySelector<HTMLInputElement>('input[value="pillars"]')!.click();
    await settle();
    await pause();
    const request = api.calls.filter((c) => c[0] === 'previewStart').at(-1)!;
    expect((request[2] as { kind: { case: string } }).kind.case).toBe('pillars');
    expect((request[3] as { kind: { case: string } }).kind.case).toBe('pillars');
    expect(el.textContent).toContain('Girar junto com os vizinhos · Ligado');
    expect(el.textContent).toContain('Os 4 primeiros estão em uso.');
    api.previewResult = { ...preview([], 0), start: { kind: { case: 'pillars', value: { pillars: [1, 2, 3, 0], $typeName: 'meurpg.play.v1.PillarsState' } }, $typeName: 'meurpg.play.v1.PuzzleState' } as never, minimum: { solvable: false, moves: 0, path: [], $typeName: 'meurpg.play.v1.PuzzleMinimum' } };
    button(el, 'Gerar outro começo').click();
    await settle();
    expect(el.textContent).toContain('os pilares não chegam ao mural');
  });

  it('shows the server\'s refusal of the name under the name, and any other as a notice', async () => {
    const { el, pause, settle } = await open();
    await pause();
    type(el.querySelector<HTMLInputElement>('input[name="name"]')!, 'x');
    api.failWith = new ConnectError('x', Code.InvalidArgument, undefined, [{ desc: PuzzleInvalidSchema, value: create(PuzzleInvalidSchema, { reason: PuzzleInvalidReason.NAME, field: 'name' }) }]);
    // `failWith` would fail the preview too: it is set after the preview has come.
    (el.querySelector('form') as HTMLFormElement).requestSubmit();
    await settle();
    expect(el.textContent).toContain('O nome precisa ter de 1 a 80 letras.');
    api.failWith = new ConnectError('x', Code.Unavailable);
    (el.querySelector('form') as HTMLFormElement).requestSubmit();
    await settle();
    expect(el.querySelector('.mr-notice--danger')?.textContent).toContain('o servidor não respondeu');
  });

  it('carries "Ao resolver" with its door and its message to the server', async () => {
    const { el, pause, settle } = await open(undefined, (a) => (a.createResult = lightsPuzzle('new', 'x')));
    await pause();
    type(el.querySelector<HTMLInputElement>('input[name="name"]')!, 'O selo da Capela');
    el.querySelector<HTMLInputElement>('input[value="point"]')!.click();
    await settle();
    // No map has points: the form says so and refuses to save.
    (el.querySelector('form') as HTMLFormElement).requestSubmit();
    await settle();
    expect(api.calls.some((c) => c[0] === 'create')).toBe(false);
    expect(el.textContent).toContain('Escolha o ponto que aparece no mapa.');
    el.querySelector<HTMLInputElement>('input[value="notify"]')!.click();
    await settle();
    (el.querySelector('form') as HTMLFormElement).requestSubmit();
    await settle();
    const init = api.calls.find((c) => c[0] === 'create')![2] as { onSolve: { action: PuzzleSolveAction } };
    expect(init.onSolve.action).toBe(PuzzleSolveAction.NOTIFY);
  });

  describe('editing', () => {
    it('opens a puzzle in its own form, shows its own start and asks the server for nothing', async () => {
      const puzzle = lightsPuzzle('p1', 'O selo da Capela', { hints: ['A luz responde ao toque.'], clue: 'Só o selo apagado abre o caminho.' });
      const { el, pause } = await open('/campanhas/camp-1/quebra-cabecas/p1/editar', (a) => (a.getResult = puzzle));
      await pause();
      expect(el.querySelector('h1')?.textContent).toBe('Editar quebra-cabeça');
      expect(el.querySelector('input[name="name"]') as HTMLInputElement).toHaveProperty('value', 'O selo da Capela');
      expect(el.querySelector<HTMLInputElement>('input[value="5"]')?.checked).toBe(true);
      expect(el.querySelectorAll('app-hints-field input[type="text"]')).toHaveLength(1);
      expect(el.querySelector('#kind-title')).toBeNull();
      expect(api.calls.filter((c) => c[0] === 'previewStart')).toHaveLength(0);
      expect(el.textContent).toContain('Dá para resolver em 4 toques.');
    });

    it('saves with seed 0, which keeps the puzzle\'s start', async () => {
      const puzzle = lightsPuzzle('p1', 'O selo da Capela');
      const { el, pause, settle } = await open('/campanhas/camp-1/quebra-cabecas/p1/editar', (a) => (a.getResult = puzzle));
      await pause();
      (el.querySelector('form') as HTMLFormElement).requestSubmit();
      await settle();
      const call = api.calls.find((c) => c[0] === 'update')!;
      expect(call.slice(1, 3)).toEqual(['camp-1', 'p1']);
      expect((call[3] as { seed: bigint }).seed).toBe(0n);
    });

    it('says a puzzle that was shown cannot be edited, and has no form', async () => {
      const { el } = await open('/campanhas/camp-1/quebra-cabecas/p1/editar', (a) => (a.getResult = lightsPuzzle('p1', 'x', { shown: true })));
      expect(el.textContent).toContain('já foi mostrado numa sessão e não pode mais ser editado');
      expect(el.querySelector('form')).toBeNull();
    });
  });

  it('tells a player the page is the master\'s, and a stranger there is no such campaign', async () => {
    access = { status: 'forbidden' };
    const player = await open();
    expect(player.el.textContent).toContain('Só o mestre faz os quebra-cabeças da campanha.');
    expect(player.el.querySelector('form')).toBeNull();
  });

  it('says there is no such campaign to a stranger', async () => {
    access = { status: 'not-found' };
    const { el } = await open();
    expect(el.textContent).toContain('Campanha não encontrada');
  });
});
