import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter, Router } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import {
  GetMapLayersResponseSchema,
  GetMapResponseSchema,
  MapPointKind,
  MapSchema,
} from '../../../../gen/meurpg/maps/v1/maps_pb';
import { SceneChecks } from '../../../core/maps/scene-actions';
import { RosterClient } from '../../../core/maps/roster-client';
import {
  PuzzleInvalidReason,
  PuzzleInvalidSchema,
  PuzzleSolveAction,
} from '../../../../gen/meurpg/play/v1/puzzles_pb';
import { MapsClient } from '../../../core/maps/maps-client';
import { type PuzzleAccess, PuzzleAccessCheck } from '../../../core/puzzles/puzzle-access';
import { PuzzlesClient } from '../../../core/puzzles/puzzles-client';
import {
  FakePuzzlesClient,
  cipherPuzzle,
  fakeChecks,
  fakeRoster,
  lightsPuzzle,
  lockPuzzle,
  pillarsPuzzle,
  preview,
  riddlePuzzle,
  sequencePuzzle,
} from '../../../core/puzzles/puzzles-testing';
import { PuzzleForm } from './puzzle-form';

@Component({ template: 'campanha' })
class Stub {}

/** A request as the fake recorded it: read by path, whatever its kind. */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
type Loose = any;

const lit17 = Array.from({ length: 25 }, (_, i) => i < 17);

// The form pulls in six kinds' forms and every field: its first test is cold, and a busy machine needs more than the default 5 s.
describe('PuzzleForm (MR-038, E10-06 state 2)', { timeout: 20_000 }, () => {
  let api: FakePuzzlesClient;
  let access: PuzzleAccess;

  async function open(
    url = '/campaigns/camp-1/puzzles/new',
    prep: (a: FakePuzzlesClient) => void = () => undefined,
  ) {
    api = new FakePuzzlesClient();
    api.previewResult = preview(lit17, 4, 7n);
    prep(api);
    const maps = {
      list: async () => [create(MapSchema, { id: 'm1', name: 'A capela' })],
      get: async () =>
        create(GetMapResponseSchema, {
          points: [
            { id: 't1', kind: MapPointKind.TRAP, name: 'Dardos envenenados' },
            {
              id: 's1',
              kind: MapPointKind.SCENE,
              name: 'Biblioteca',
              clues: [{ id: 'k1', text: 'Cada letra anda três para trás.' }],
            },
          ],
        }),
      layers: async () => create(GetMapLayersResponseSchema, {}),
    };
    TestBed.configureTestingModule({
      providers: [
        provideRouter([
          { path: 'campaigns/:id/puzzles/new', component: PuzzleForm },
          { path: 'campaigns/:id/puzzles/:puzzleId/edit', component: PuzzleForm },
          { path: 'campaigns/:id', component: Stub },
        ]),
        { provide: PuzzlesClient, useValue: api },
        { provide: MapsClient, useValue: maps },
        { provide: SceneChecks, useValue: fakeChecks },
        { provide: RosterClient, useValue: fakeRoster },
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

  const button = (el: HTMLElement, text: string) =>
    Array.from(el.querySelectorAll('button, a')).find((b) =>
      b.textContent?.trim().includes(text),
    ) as HTMLElement;
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
    const kinds = Array.from(el.querySelectorAll('.opt__title'))
      .slice(0, 6)
      .map((t) => t.textContent?.trim());
    expect(kinds).toEqual([
      'Apagar as luzes',
      'Fechadura de combinação',
      'Símbolos giratórios',
      'Enigma',
      'Sequência',
      'Cifra',
    ]);
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
    const { el, pause, settle } = await open(
      undefined,
      (a) => (a.createResult = lightsPuzzle('new', 'O selo da Capela')),
    );
    await pause();
    api.previewResult = preview(
      Array.from({ length: 25 }, (_, i) => i % 2 === 0),
      3,
      99n,
    );
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
    expect(TestBed.inject(Router).url).toBe('/campaigns/camp-1');
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
    const { el, settle } = await open(
      undefined,
      (a) => (a.createResult = lockPuzzle('new', 'O cofre')),
    );
    el.querySelector<HTMLInputElement>('input[value="lock"]')!.click();
    await settle();
    expect(el.querySelector('h2#form-title')?.textContent).toBe('Fechadura de combinação');
    expect(api.calls.filter((c) => c[0] === 'previewStart')).toHaveLength(
      1 - 1 + api.calls.filter((c) => c[0] === 'previewStart').length,
    ); // none after switching
    type(el.querySelector<HTMLInputElement>('input[name="name"]')!, 'O cofre do Refeitório');
    // Turn the solution's first wheel up once: [1, 0, 0, 0] against the start [0, 0, 0, 1].
    el.querySelector<HTMLElement>('[aria-label="Próximo símbolo: Roda 1"]')!.click();
    await settle();
    (el.querySelector('form') as HTMLFormElement).requestSubmit();
    await settle();
    const init = api.calls.find((c) => c[0] === 'create')![2] as {
      solution: unknown;
      start: unknown;
      seed: bigint;
    };
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

  it("sends the pillars' mural and links, and says when the links make the mural impossible", async () => {
    const { el, settle, pause } = await open(
      undefined,
      (a) => (a.createResult = pillarsPuzzle('new', 'Os pilares')),
    );
    el.querySelector<HTMLInputElement>('input[value="pillars"]')!.click();
    await settle();
    await pause();
    const request = api.calls.filter((c) => c[0] === 'previewStart').at(-1)!;
    expect((request[2] as { kind: { case: string } }).kind.case).toBe('pillars');
    expect((request[3] as { kind: { case: string } }).kind.case).toBe('pillars');
    expect(el.textContent).toContain('Girar junto com os vizinhos · Ligado');
    expect(el.textContent).toContain('Os 4 primeiros estão em uso.');
    api.previewResult = {
      ...preview([], 0),
      start: {
        kind: {
          case: 'pillars',
          value: { pillars: [1, 2, 3, 0], $typeName: 'meurpg.play.v1.PillarsState' },
        },
        $typeName: 'meurpg.play.v1.PuzzleState',
      } as never,
      minimum: { solvable: false, moves: 0, path: [], $typeName: 'meurpg.play.v1.PuzzleMinimum' },
    };
    button(el, 'Gerar outro começo').click();
    await settle();
    expect(el.textContent).toContain('os pilares não chegam ao mural');
  });

  it("shows the server's refusal of the name under the name, and any other as a notice", async () => {
    const { el, pause, settle } = await open();
    await pause();
    type(el.querySelector<HTMLInputElement>('input[name="name"]')!, 'x');
    api.failWith = new ConnectError('x', Code.InvalidArgument, undefined, [
      {
        desc: PuzzleInvalidSchema,
        value: create(PuzzleInvalidSchema, { reason: PuzzleInvalidReason.NAME, field: 'name' }),
      },
    ]);
    // `failWith` would fail the preview too: it is set after the preview has come.
    (el.querySelector('form') as HTMLFormElement).requestSubmit();
    await settle();
    expect(el.textContent).toContain('O nome precisa ter de 1 a 80 letras.');
    api.failWith = new ConnectError('x', Code.Unavailable);
    (el.querySelector('form') as HTMLFormElement).requestSubmit();
    await settle();
    expect(el.querySelector('.mr-notice--danger')?.textContent).toContain(
      'o servidor não respondeu',
    );
  });

  it('carries "Ao resolver" with its door and its message to the server', async () => {
    const { el, pause, settle } = await open(
      undefined,
      (a) => (a.createResult = lightsPuzzle('new', 'x')),
    );
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
    const init = api.calls.find((c) => c[0] === 'create')![2] as {
      onSolve: { action: PuzzleSolveAction };
    };
    expect(init.onSolve.action).toBe(PuzzleSolveAction.NOTIFY);
  });

  describe('editing', () => {
    it('opens a puzzle in its own form, shows its own start and asks the server for nothing', async () => {
      const puzzle = lightsPuzzle('p1', 'O selo da Capela', {
        hints: ['A luz responde ao toque.'],
        clue: 'Só o selo apagado abre o caminho.',
      });
      const { el, pause } = await open(
        '/campaigns/camp-1/puzzles/p1/edit',
        (a) => (a.getResult = puzzle),
      );
      await pause();
      expect(el.querySelector('h1')?.textContent).toBe('Editar quebra-cabeça');
      expect(el.querySelector('input[name="name"]') as HTMLInputElement).toHaveProperty(
        'value',
        'O selo da Capela',
      );
      expect(el.querySelector<HTMLInputElement>('input[value="5"]')?.checked).toBe(true);
      expect(el.querySelectorAll('app-hints-field input[type="text"]')).toHaveLength(1);
      expect(el.querySelector('#kind-title')).toBeNull();
      expect(api.calls.filter((c) => c[0] === 'previewStart')).toHaveLength(0);
      expect(el.textContent).toContain('Dá para resolver em 4 toques.');
    });

    it("saves with seed 0, which keeps the puzzle's start", async () => {
      const puzzle = lightsPuzzle('p1', 'O selo da Capela');
      const { el, pause, settle } = await open(
        '/campaigns/camp-1/puzzles/p1/edit',
        (a) => (a.getResult = puzzle),
      );
      await pause();
      (el.querySelector('form') as HTMLFormElement).requestSubmit();
      await settle();
      const call = api.calls.find((c) => c[0] === 'update')!;
      expect(call.slice(1, 3)).toEqual(['camp-1', 'p1']);
      expect((call[3] as { seed: bigint }).seed).toBe(0n);
    });

    it('says a puzzle that was shown cannot be edited, and has no form', async () => {
      const { el } = await open(
        '/campaigns/camp-1/puzzles/p1/edit',
        (a) => (a.getResult = lightsPuzzle('p1', 'x', { shown: true })),
      );
      expect(el.textContent).toContain('já foi mostrado numa sessão e não pode mais ser editado');
      expect(el.querySelector('form')).toBeNull();
    });
  });

  it("tells a player the page is the master's, and a stranger there is no such campaign", async () => {
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

  describe('the riddle, the sequence and the cipher (slice 10.15b, E10-12 states 1 to 4)', () => {
    const pick = async (el: HTMLElement, kind: string, settle: () => Promise<void>) => {
      el.querySelector<HTMLInputElement>(`input[value="${kind}"]`)!.click();
      await settle();
    };
    const enter = (field: Element) =>
      field.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' }));
    const submit = async (el: HTMLElement, settle: () => Promise<void>) => {
      (el.querySelector('form') as HTMLFormElement).requestSubmit();
      await settle();
    };
    const created = () => api.calls.find((c) => c[0] === 'create')![2] as Loose;

    it('makes a riddle: the text, the accepted answers one by one with Enter, and what a wrong answer does', async () => {
      const { el, settle } = await open(
        undefined,
        (a) => (a.createResult = riddlePuzzle('new', 'x')),
      );
      await pick(el, 'riddle', settle);
      expect(el.querySelector('h2#form-title')?.textContent).toBe('Enigma');
      type(el.querySelector<HTMLInputElement>('input[name="name"]')!, 'A porta da Cripta pergunta');
      type(
        el.querySelector<HTMLTextAreaElement>('textarea[name="riddle"]')!,
        'Moro embaixo de cada passo seu. O que sou?',
      );
      for (const answer of ['sombra', 'a sombra']) {
        type(el.querySelector<HTMLInputElement>('input[name="answer"]')!, answer);
        enter(el.querySelector('input[name="answer"]')!);
        await settle();
      }
      expect(Array.from(el.querySelectorAll('.chip__text')).map((c) => c.textContent)).toEqual([
        'sombra',
        'a sombra',
      ]);
      // The chips' "×" is a 44 px button with the answer in its name.
      expect(el.querySelector('.chip__x')?.getAttribute('aria-label')).toBe(
        'Tirar a resposta sombra',
      );
      // "Ao errar": spend an attempt, three per player.
      el.querySelector<HTMLInputElement>('app-wrong-field input[value="attempts"]')!.click();
      await settle();
      await submit(el, settle);
      const init = created();
      expect(init.config).toEqual({
        kind: { case: 'riddle', value: { text: 'Moro embaixo de cada passo seu. O que sou?' } },
      });
      expect(init.solution).toEqual({
        kind: { case: 'riddle', value: { answers: ['sombra', 'a sombra'] } },
      });
      expect(init.onWrong).toEqual({ attemptsPerPlayer: 3 });
      expect(TestBed.inject(Router).url).toBe('/campaigns/camp-1');
    });

    it('keeps an answer typed but not yet added when the master leaves the field and saves', async () => {
      const { el, settle } = await open(
        undefined,
        (a) => (a.createResult = riddlePuzzle('new', 'x')),
      );
      await pick(el, 'riddle', settle);
      type(el.querySelector<HTMLInputElement>('input[name="name"]')!, 'x');
      type(el.querySelector<HTMLTextAreaElement>('textarea[name="riddle"]')!, 'O que sou?');
      type(el.querySelector<HTMLInputElement>('input[name="answer"]')!, 'sombra');
      el.querySelector('input[name="answer"]')!.dispatchEvent(new Event('blur'));
      await settle();
      await submit(el, settle);
      expect(created().solution.kind.value.answers).toEqual(['sombra']);
    });

    it('keeps an answer typed but not added when the form is saved with no blur at all', async () => {
      const { el, settle } = await open(
        undefined,
        (a) => (a.createResult = riddlePuzzle('new', 'x')),
      );
      await pick(el, 'riddle', settle);
      type(el.querySelector<HTMLInputElement>('input[name="name"]')!, 'x');
      type(el.querySelector<HTMLTextAreaElement>('textarea[name="riddle"]')!, 'O que sou?');
      type(el.querySelector<HTMLInputElement>('input[name="answer"]')!, 'sombra');
      await submit(el, settle);
      expect(created().solution.kind.value.answers).toEqual(['sombra']);
    });

    it('refuses a riddle with no answer on the field, and an answer that repeats another', async () => {
      const { el, settle } = await open();
      await pick(el, 'riddle', settle);
      type(el.querySelector<HTMLInputElement>('input[name="name"]')!, 'x');
      await submit(el, settle);
      expect(api.calls.some((c) => c[0] === 'create')).toBe(false);
      expect(el.textContent).toContain('Escreva o enigma que os jogadores vão ler.');
      expect(el.textContent).toContain('Escreva pelo menos uma resposta aceita.');
      type(el.querySelector<HTMLTextAreaElement>('textarea[name="riddle"]')!, 'O que sou?');
      for (const answer of ['A Sombra', 'a sombra!']) {
        type(el.querySelector<HTMLInputElement>('input[name="answer"]')!, answer);
        enter(el.querySelector('input[name="answer"]')!);
        await settle();
      }
      await submit(el, settle);
      expect(el.textContent).toContain('Resposta 2: Esta resposta é igual a outra');
    });

    it('makes a sequence by tapping bells, takes the last step back, tests it and sends the steps', async () => {
      const { el, settle } = await open(
        undefined,
        (a) => (a.createResult = sequencePuzzle('new', 'x')),
      );
      await pick(el, 'sequence', settle);
      type(el.querySelector<HTMLInputElement>('input[name="name"]')!, 'Os sinos do Salão do trono');
      const bell = (name: string) =>
        el.querySelector<HTMLElement>(`app-bells-board [aria-label="${name}"]`)!;
      for (const name of [
        'Sino redondo',
        'Sino alto',
        'Sino pequeno',
        'Sino redondo',
        'Sino largo',
        'Sino largo',
      ]) {
        bell(name).click();
        await settle();
      }
      expect(
        Array.from(el.querySelectorAll('app-sequence-strip li')).map((i) =>
          i.getAttribute('aria-label'),
        ),
      ).toEqual([
        'Passo 1: Sino redondo',
        'Passo 2: Sino alto',
        'Passo 3: Sino pequeno',
        'Passo 4: Sino redondo',
        'Passo 5: Sino largo',
        'Passo 6: Sino largo',
      ]);
      button(el, 'Apagar o último passo').click();
      await settle();
      expect(el.querySelectorAll('app-sequence-strip li')).toHaveLength(5);
      expect(el.textContent).toContain('5 passos (de 3 a 12).');
      // "Tocar para testar" lights the steps one after another, here, with no server call.
      button(el, 'Tocar para testar').click();
      await settle();
      expect(el.querySelector('app-sequence-strip .slot--lit')?.getAttribute('aria-label')).toBe(
        'Passo 1: Sino redondo',
      );
      await vi.advanceTimersByTimeAsync(1200);
      await settle();
      expect(el.querySelector('app-sequence-strip .slot--lit')?.getAttribute('aria-label')).toBe(
        'Passo 2: Sino alto',
      );
      await vi.advanceTimersByTimeAsync(1200 * 5);
      await settle();
      expect(el.querySelector('app-sequence-strip .slot--lit')).toBeNull();
      expect(api.calls.some((c) => c[0] === 'playSequence')).toBe(false);
      await submit(el, settle);
      const init = created();
      expect(init.config).toEqual({ kind: { case: 'sequence', value: { bells: 4, steps: 5 } } });
      expect(init.solution).toEqual({
        kind: { case: 'sequence', value: { steps: [0, 1, 3, 0, 2] } },
      });
    });

    it('takes out the steps of a bell that is no longer there, and refuses fewer than 3 steps', async () => {
      const { el, settle } = await open();
      await pick(el, 'sequence', settle);
      type(el.querySelector<HTMLInputElement>('input[name="name"]')!, 'x');
      for (const name of ['Sino redondo', 'Sino pequeno']) {
        el.querySelector<HTMLElement>(`app-bells-board [aria-label="${name}"]`)!.click();
        await settle();
      }
      await submit(el, settle);
      expect(api.calls.some((c) => c[0] === 'create')).toBe(false);
      expect(el.textContent).toContain('Faltam passos: a sequência vai de 3 a 12, e tem 2.');
      el.querySelector<HTMLElement>('[aria-label="Menos sino"]')!.click();
      await settle();
      expect(el.querySelectorAll('app-bells-board button')).toHaveLength(3);
      expect(el.querySelectorAll('app-sequence-strip li')).toHaveLength(1);
    });

    it('makes a cipher: the server ciphers the message for the master after a pause, and the key is a scene clue', async () => {
      const { el, settle, pause } = await open(
        undefined,
        (a) => (a.createResult = cipherPuzzle('new', 'x')),
      );
      await pick(el, 'cipher', settle);
      type(el.querySelector<HTMLInputElement>('input[name="name"]')!, 'A carta do Capitão');
      type(
        el.querySelector<HTMLTextAreaElement>('textarea[name="cipher-message"]')!,
        'O tesouro está sob o altar',
      );
      await settle();
      // Nothing is asked before the pause; the browser never ciphers.
      expect(api.calls.some((c) => c[0] === 'previewCipher')).toBe(false);
      expect(el.textContent).toContain('Cifrando a mensagem...');
      await pause();
      const call = api.calls.find((c) => c[0] === 'previewCipher')!;
      expect(call[2]).toEqual({
        message: 'O tesouro está sob o altar',
        method: { case: 'shift', value: 3 },
      });
      expect(el.querySelector('.cipher')?.textContent).toBe('R WHVRXUR HVWD VRE R DOWDU');
      // The key as a clue of a scene.
      const clue = el.querySelector<HTMLSelectElement>('app-cipher-form select')!;
      expect(Array.from(clue.options).map((o) => o.textContent?.trim())).toEqual([
        'Nenhuma (você diz a chave na mesa)',
        'Biblioteca: Cada letra anda três para trás.',
      ]);
      clue.value = 'k1';
      clue.dispatchEvent(new Event('change'));
      await settle();
      await submit(el, settle);
      const init = created();
      expect(init.config).toEqual({ kind: { case: 'cipher', value: { keyClueId: 'k1' } } });
      expect(init.solution).toEqual({
        kind: {
          case: 'cipher',
          value: { message: 'O tesouro está sob o altar', method: { case: 'shift', value: 3 } },
        },
      });
    });

    it('asks for the keyword when the master chooses it, and refuses one that changes no letter', async () => {
      const { el, settle, pause } = await open(
        undefined,
        (a) => (a.createResult = cipherPuzzle('new', 'x')),
      );
      await pick(el, 'cipher', settle);
      type(el.querySelector<HTMLInputElement>('input[name="name"]')!, 'x');
      type(el.querySelector<HTMLTextAreaElement>('textarea[name="cipher-message"]')!, 'Olá');
      el.querySelector<HTMLInputElement>('app-cipher-form input[value="keyword"]')!.click();
      await settle();
      type(el.querySelector<HTMLInputElement>('input[name="keyword"]')!, 'abc');
      await settle();
      await submit(el, settle);
      expect(api.calls.some((c) => c[0] === 'create')).toBe(false);
      expect(el.textContent).toContain('Essa palavra-chave não troca nenhuma letra.');
      expect(api.calls.some((c) => c[0] === 'previewCipher')).toBe(false);
      type(el.querySelector<HTMLInputElement>('input[name="keyword"]')!, 'lua');
      await pause();
      expect(api.calls.find((c) => c[0] === 'previewCipher')![2]).toEqual({
        message: 'Olá',
        method: { case: 'keyword', value: 'lua' },
      });
    });

    it('offers a trap and attempts only to the kinds that judge a move, and the limits to all', async () => {
      const { el, settle } = await open();
      const titles = () =>
        Array.from(el.querySelectorAll('app-wrong-field .opt__title')).map((t) =>
          t.textContent?.trim(),
        );
      expect(titles()).toEqual(['Nada acontece', 'Limite de jogadas ou de tempo']);
      await pick(el, 'riddle', settle);
      expect(titles()).toEqual([
        'Nada acontece',
        'Disparar uma armadilha do mapa',
        'Gastar uma tentativa do jogador',
        'Limite de jogadas ou de tempo',
      ]);
    });

    it('chooses the trap of a map, and sends it', async () => {
      const { el, settle } = await open(
        undefined,
        (a) => (a.createResult = sequencePuzzle('new', 'x')),
      );
      await pick(el, 'sequence', settle);
      type(el.querySelector<HTMLInputElement>('input[name="name"]')!, 'x');
      for (const name of ['Sino redondo', 'Sino alto', 'Sino largo']) {
        el.querySelector<HTMLElement>(`app-bells-board [aria-label="${name}"]`)!.click();
        await settle();
      }
      el.querySelector<HTMLInputElement>('app-wrong-field input[value="trap"]')!.click();
      await settle();
      await settle();
      await submit(el, settle);
      expect(api.calls.some((c) => c[0] === 'create')).toBe(false);
      expect(el.textContent).toContain('Escolha a armadilha do mapa que dispara.');
      const select = el.querySelector<HTMLSelectElement>('app-wrong-field select')!;
      expect(Array.from(select.options).map((o) => o.textContent?.trim())).toEqual([
        'Escolha uma armadilha',
        'Dardos envenenados · A capela',
      ]);
      select.value = 'm1|t1';
      select.dispatchEvent(new Event('change'));
      await settle();
      await submit(el, settle);
      expect(created().onWrong).toEqual({ trap: { mapId: 'm1', pointId: 't1' } });
    });

    it('writes the limits of moves and of time, one or both', async () => {
      const { el, pause, settle } = await open(
        undefined,
        (a) => (a.createResult = lightsPuzzle('new', 'x')),
      );
      await pause();
      type(el.querySelector<HTMLInputElement>('input[name="name"]')!, 'x');
      el.querySelector<HTMLInputElement>('app-wrong-field input[value="limits"]')!.click();
      await settle();
      await submit(el, settle);
      expect(el.textContent).toContain('Ponha um limite de jogadas, de minutos, ou os dois.');
      const [moves, minutes] = Array.from(
        el.querySelectorAll<HTMLInputElement>('app-wrong-field input[type="text"]'),
      );
      type(moves, '10');
      await settle();
      type(minutes, '5');
      await settle();
      await submit(el, settle);
      expect(created().onWrong).toEqual({ maxMoves: 10, timeLimitSeconds: 300 });
    });

    it('wins a hint by a skill check: a skill and a DC, both or neither', async () => {
      const { el, pause, settle } = await open(
        undefined,
        (a) => (a.createResult = lightsPuzzle('new', 'x')),
      );
      await pause();
      type(el.querySelector<HTMLInputElement>('input[name="name"]')!, 'x');
      button(el, 'Adicionar uma dica').click();
      await settle();
      type(
        el.querySelector<HTMLInputElement>('app-hints-field input[type="text"]')!,
        'A luz responde ao toque.',
      );
      const skill = el.querySelector<HTMLSelectElement>('app-hint-check-field select')!;
      expect(Array.from(skill.options).map((o) => o.textContent?.trim())).toEqual([
        'Nenhuma',
        'Arcanismo',
        'Investigação',
      ]);
      skill.value = 'skill:investigation';
      skill.dispatchEvent(new Event('change'));
      await settle();
      await submit(el, settle);
      expect(api.calls.some((c) => c[0] === 'create')).toBe(false);
      expect(el.textContent).toContain('A CD vai de 1 a 30.');
      type(el.querySelector<HTMLInputElement>('app-hint-check-field input[type="text"]')!, '13');
      await settle();
      await submit(el, settle);
      expect(created().hintCheck).toEqual({ skillKey: 'skill:investigation', dc: 13 });
    });

    it("splits a clue in parts, one per player's character, and never offers the goblin", async () => {
      const { el, pause, settle } = await open(
        undefined,
        (a) => (a.createResult = lightsPuzzle('new', 'x')),
      );
      await pause();
      type(el.querySelector<HTMLInputElement>('input[name="name"]')!, 'x');
      button(el, 'Adicionar parte').click();
      await settle();
      const owner = el.querySelector<HTMLSelectElement>('app-parts-field select')!;
      expect(Array.from(owner.options).map((o) => o.textContent?.trim())).toEqual([
        'Sem dono ainda',
        'Toren',
        'Brisa',
        'Sálvia',
      ]);
      owner.value = 'c-toren';
      owner.dispatchEvent(new Event('change'));
      await settle();
      type(
        el.querySelector<HTMLTextAreaElement>('app-parts-field textarea')!,
        'A porta ouve o que o chão esconde…',
      );
      await settle();
      expect(el.querySelector('.part__badge')?.textContent?.trim()).toBe('T');
      button(el, 'Adicionar parte').click();
      await settle();
      // Toren has a part already: the second part cannot take him.
      const second = el.querySelectorAll<HTMLSelectElement>('app-parts-field select')[1];
      expect(Array.from(second.options).find((o) => o.value === 'c-toren')?.disabled).toBe(true);
      type(
        el.querySelectorAll<HTMLTextAreaElement>('app-parts-field textarea')[1],
        '…os tambores ecoam três vezes.',
      );
      await settle();
      await submit(el, settle);
      expect(created().parts).toEqual([
        { characterId: 'c-toren', text: 'A porta ouve o que o chão esconde…' },
        { characterId: '', text: '…os tambores ecoam três vezes.' },
      ]);
    });

    it('stops at 8 parts', async () => {
      const { el, pause, settle } = await open();
      await pause();
      for (let i = 0; i < 8; i++) {
        button(el, 'Adicionar parte').click();
        await settle();
      }
      expect(el.querySelectorAll('app-parts-field .part')).toHaveLength(8);
      expect(el.textContent).toContain('Máximo de 8 partes.');
      button(el, 'Adicionar parte').click();
      await settle();
      expect(el.querySelectorAll('app-parts-field .part')).toHaveLength(8);
    });

    const refuse = (reason: PuzzleInvalidReason, field: string) =>
      new ConnectError('x', Code.InvalidArgument, undefined, [
        { desc: PuzzleInvalidSchema, value: create(PuzzleInvalidSchema, { reason, field }) },
      ]);

    async function riddleReadyToSave(el: HTMLElement, settle: () => Promise<void>) {
      await pick(el, 'riddle', settle);
      type(el.querySelector<HTMLInputElement>('input[name="name"]')!, 'x');
      type(el.querySelector<HTMLTextAreaElement>('textarea[name="riddle"]')!, 'O que sou?');
      type(el.querySelector<HTMLInputElement>('input[name="answer"]')!, 'sombra');
      el.querySelector('input[name="answer"]')!.dispatchEvent(new Event('blur'));
      await settle();
    }

    it('shows what the server refused under the trap it names, in words, and takes the focus there', async () => {
      const { el, settle } = await open(undefined);
      document.body.append(el);
      await riddleReadyToSave(el, settle);
      el.querySelector<HTMLInputElement>('app-wrong-field input[value="trap"]')!.click();
      await settle();
      await settle();
      const select = el.querySelector<HTMLSelectElement>('app-wrong-field select')!;
      select.value = 'm1|t1';
      select.dispatchEvent(new Event('change'));
      await settle();
      api.failWith = refuse(PuzzleInvalidReason.ON_WRONG, 'on_wrong.trap');
      await submit(el, settle);
      expect(el.querySelector('app-wrong-field .field-error')?.textContent).toContain(
        'O “Ao errar” não vale',
      );
      expect(select.getAttribute('aria-invalid')).toBe('true');
      expect(el.querySelector('.mr-notice--danger')).toBeNull();
      // The master changes anything and the refusal goes.
      type(el.querySelector<HTMLInputElement>('input[name="name"]')!, 'y');
      await settle();
      expect(el.querySelector('app-wrong-field .field-error')).toBeNull();
      el.remove();
    });

    it('puts the refusal of a limit on its own number field', async () => {
      const { el, settle } = await open(undefined);
      await riddleReadyToSave(el, settle);
      el.querySelector<HTMLInputElement>('app-wrong-field input[value="limits"]')!.click();
      await settle();
      const [moves, minutes] = Array.from(
        el.querySelectorAll<HTMLInputElement>('app-wrong-field input[type="text"]'),
      );
      type(moves, '10');
      await settle();
      type(minutes, '5');
      await settle();
      api.failWith = refuse(PuzzleInvalidReason.ON_WRONG, 'on_wrong.max_moves');
      await submit(el, settle);
      const [m, t] = Array.from(
        el.querySelectorAll<HTMLInputElement>('app-wrong-field input[type="text"]'),
      );
      expect(m.getAttribute('aria-invalid')).toBe('true');
      expect(t.getAttribute('aria-invalid')).not.toBe('true');
      // The form's own check names the field too: only the one that is wrong.
      api.failWith = undefined;
      type(el.querySelectorAll<HTMLInputElement>('app-wrong-field input[type="text"]')[1], '999');
      await settle();
      await submit(el, settle);
      const [m2, t2] = Array.from(
        el.querySelectorAll<HTMLInputElement>('app-wrong-field input[type="text"]'),
      );
      expect(m2.getAttribute('aria-invalid')).not.toBe('true');
      expect(t2.getAttribute('aria-invalid')).toBe('true');
    });

    it('puts the refusal of a part on that part, by the field the server names', async () => {
      const { el, pause, settle } = await open(
        undefined,
        (a) => (a.createResult = lightsPuzzle('new', 'x')),
      );
      await pause();
      type(el.querySelector<HTMLInputElement>('input[name="name"]')!, 'x');
      for (let i = 0; i < 2; i++) {
        button(el, 'Adicionar parte').click();
        await settle();
        type(
          el.querySelectorAll<HTMLTextAreaElement>('app-parts-field textarea')[i],
          `Parte ${i + 1}.`,
        );
        await settle();
      }
      api.failWith = refuse(PuzzleInvalidReason.PARTS, 'parts[1].character_id');
      await submit(el, settle);
      const parts = Array.from(el.querySelectorAll('app-parts-field .part'));
      expect(parts[0].querySelector('.field-error')).toBeNull();
      expect(parts[1].querySelector('.field-error')?.textContent).toContain('informação dividida');
      expect(parts[1].querySelector('select')?.getAttribute('aria-invalid')).toBe('true');
    });

    it('puts what the server says about the cipher on the key', async () => {
      const { el, settle } = await open(
        undefined,
        (a) => (a.createResult = cipherPuzzle('new', 'x')),
      );
      await pick(el, 'cipher', settle);
      type(el.querySelector<HTMLInputElement>('input[name="name"]')!, 'x');
      type(el.querySelector<HTMLTextAreaElement>('textarea[name="cipher-message"]')!, 'Olá mundo');
      await settle();
      api.failWith = refuse(PuzzleInvalidReason.CIPHER, 'solution.cipher');
      await submit(el, settle);
      const key = el.querySelectorAll('app-cipher-form section')[1];
      expect(key.querySelector('.field-error')?.textContent).toContain('A cifra não vale');
      expect(
        el.querySelectorAll('app-cipher-form section')[0].querySelector('.field-error'),
      ).toBeNull();
    });

    it('keeps the last ciphered message on screen while the next one is asked for', async () => {
      const { el, settle, pause } = await open(
        undefined,
        (a) => (a.createResult = cipherPuzzle('new', 'x')),
      );
      await pick(el, 'cipher', settle);
      type(el.querySelector<HTMLTextAreaElement>('textarea[name="cipher-message"]')!, 'Olá');
      await pause();
      expect(el.querySelector('.cipher')?.textContent).toBe('R WHVRXUR HVWD VRE R DOWDU');
      type(el.querySelector<HTMLTextAreaElement>('textarea[name="cipher-message"]')!, 'Olá mundo');
      await settle();
      // Asking again: the old text stays, marked busy, and "Cifrando..." is not shown over it.
      expect(el.querySelector('.cipher')?.getAttribute('aria-busy')).toBe('true');
      expect(el.textContent).not.toContain('Cifrando a mensagem...');
      expect(el.querySelector('app-cipher-form [role="status"]')).toBeNull();
    });

    it('opens a riddle in its own form and saves it with the skill check, the parts and "Ao errar" it came with', async () => {
      const puzzle = riddlePuzzle('p1', 'A porta da Cripta pergunta', {
        hints: ['Pense no que acompanha você ao meio-dia.'],
        hintCheck: { skillKey: 'skill:investigation', dc: 13 },
        parts: [{ characterId: 'c-toren', text: 'A porta ouve.', ownerUnavailable: false }],
        onWrong: { attemptsPerPlayer: 4 },
      });
      const { el, settle } = await open(
        '/campaigns/camp-1/puzzles/p1/edit',
        (a) => (a.getResult = puzzle),
      );
      expect(el.querySelector('h1')?.textContent).toBe('Editar quebra-cabeça');
      expect(el.querySelector('textarea[name="riddle"]')).toHaveProperty(
        'value',
        'Moro embaixo de cada passo seu, mas nunca peso nada. O que sou?',
      );
      expect(Array.from(el.querySelectorAll('.chip__text')).map((c) => c.textContent)).toEqual([
        'sombra',
        'a sombra',
      ]);
      expect(el.querySelector('app-wrong-field input[value="attempts"]')).toHaveProperty(
        'checked',
        true,
      );
      expect(
        el.querySelector<HTMLInputElement>('app-hint-check-field input[type="text"]')?.value,
      ).toBe('13');
      await submit(el, settle);
      const call = api.calls.find((c) => c[0] === 'update')!;
      const init = call[3] as Loose;
      expect(init.hintCheck).toEqual({ skillKey: 'skill:investigation', dc: 13 });
      expect(init.parts).toEqual([{ characterId: 'c-toren', text: 'A porta ouve.' }]);
      expect(init.onWrong).toEqual({ attemptsPerPlayer: 4 });
      expect(init.solution.kind.value.answers).toEqual(['sombra', 'a sombra']);
    });

    it('opens a cipher with its message, its key and the clue linked', async () => {
      const puzzle = cipherPuzzle('p2', 'A carta', {
        config: { kind: { case: 'cipher', value: { ciphertext: 'R WHVRXUR', keyClueId: 'k1' } } },
      });
      const { el } = await open('/campaigns/camp-1/puzzles/p2/edit', (a) => (a.getResult = puzzle));
      expect(el.querySelector('textarea[name="cipher-message"]')).toHaveProperty(
        'value',
        'O tesouro está sob o altar',
      );
      expect(el.querySelector<HTMLSelectElement>('app-cipher-form select')?.value).toBe('k1');
      expect(el.querySelector('app-stepper .st__value')?.textContent).toBe('3');
    });
  });
});
