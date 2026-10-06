import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import { PuzzleKind, PuzzleRunSchema } from '../../../gen/meurpg/play/v1/puzzles_pb';
import { type CounterRow } from '../../core/puzzles/puzzle-format';
import { BELLS, GLYPHS, RUNES, bellFaces, pillarFaces } from '../../core/puzzles/puzzle-symbols';
import { BellsBoard } from './bells-board';
import { CipherBoard } from './cipher-board';
import { LightsBoard, type LightPress } from './lights-board';
import { LimitCounters } from './limit-counters';
import { LockBoard } from './lock-board';
import { PillarsBoard } from './pillars-board';
import { PuzzleHost } from './puzzle-host';
import { RiddleBoard } from './riddle-board';
import { SequenceBoard } from './sequence-board';
import { SequenceStrip } from './sequence-strip';
import type { Turn } from './symbol-columns';

const flat = (n: number, on: readonly number[]) => Array.from({ length: n * n }, (_, i) => on.includes(i));

function lights(inputs: Record<string, unknown>) {
  const fixture = TestBed.createComponent(LightsBoard);
  for (const [k, v] of Object.entries(inputs)) {
    fixture.componentRef.setInput(k, v);
  }
  const presses: LightPress[] = [];
  fixture.componentInstance.press.subscribe((p) => presses.push(p));
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement, presses };
}

describe('LightsBoard (E10-06 states 6 to 9)', () => {
  it('names every light by its row, column and state, so no light is only a color', () => {
    const { el } = lights({ size: 3, lit: flat(3, [4]), mode: 'play' });
    const labels = Array.from(el.querySelectorAll('button')).map((b) => b.getAttribute('aria-label'));
    expect(labels).toHaveLength(9);
    expect(labels[0]).toBe('Luz na linha 1, coluna 1, apagada');
    expect(labels[4]).toBe('Luz na linha 2, coluna 2, acesa');
    expect(el.querySelector('[role="group"]')?.getAttribute('aria-label')).toBe('Painel de luzes, 3 por 3');
  });

  it('draws a lit light as a sun and an off one as a ring', () => {
    const { el } = lights({ size: 3, lit: flat(3, [0]), mode: 'view' });
    const cells = Array.from(el.querySelectorAll('.cell'));
    expect(cells[0].classList).toContain('cell--lit');
    expect(cells[0].querySelectorAll('path').length).toBe(1); // the rays
    expect(cells[1].classList).not.toContain('cell--lit');
    expect(cells[1].querySelector('circle')?.getAttribute('fill')).toBe('none');
  });

  it('emits the row and column of a pressed light (counted from 0)', () => {
    const { el, presses } = lights({ size: 5, lit: flat(5, []), mode: 'play' });
    (el.querySelector('[data-i="13"]') as HTMLElement).click();
    expect(presses).toEqual([{ row: 2, col: 3 }]);
  });

  it('is a static board for the master: nothing in it is a button', () => {
    const { el, presses } = lights({ size: 3, lit: flat(3, [1]), mode: 'view' });
    expect(el.querySelectorAll('button')).toHaveLength(0);
    expect(el.querySelectorAll('[role="img"]')).toHaveLength(9);
    (el.querySelector('.cell') as HTMLElement).click();
    expect(presses).toEqual([]);
  });

  it('does nothing once it is solved or stopped, and says so with aria-disabled', () => {
    const { el, presses } = lights({ size: 3, lit: flat(3, []), mode: 'play', disabled: true });
    const first = el.querySelector('button') as HTMLButtonElement;
    expect(first.getAttribute('aria-disabled')).toBe('true');
    first.click();
    expect(presses).toEqual([]);
  });

  it('has one tab stop, and the arrow keys walk the squares', () => {
    const { el, fixture } = lights({ size: 3, lit: flat(3, []), mode: 'play' });
    document.body.append(el);
    const stops = () => Array.from(el.querySelectorAll('button')).filter((b) => b.tabIndex === 0);
    expect(stops()).toHaveLength(1);
    const first = el.querySelector('[data-i="0"]') as HTMLElement;
    first.focus();
    first.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
    expect(document.activeElement).toBe(el.querySelector('[data-i="1"]'));
    (document.activeElement as HTMLElement).dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));
    expect(document.activeElement).toBe(el.querySelector('[data-i="4"]'));
    (document.activeElement as HTMLElement).dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowLeft', bubbles: true }));
    (document.activeElement as HTMLElement).dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowLeft', bubbles: true }));
    expect(document.activeElement).toBe(el.querySelector('[data-i="3"]')); // the edge holds
    fixture.detectChanges();
    expect(stops()).toHaveLength(1);
    el.remove();
  });

  it('marks what just changed (dashed) and the master\'s hints (solid garnet)', () => {
    const { el } = lights({ size: 3, lit: flat(3, []), mode: 'view', changed: [1, 2], hints: [8] });
    const cells = Array.from(el.querySelectorAll('.cell'));
    expect(cells[1].classList).toContain('cell--changed');
    expect(cells[2].classList).toContain('cell--changed');
    expect(cells[8].classList).toContain('cell--hint');
    expect(cells[0].classList).not.toContain('cell--changed');
  });
});

describe('LockBoard and PillarsBoard (E10-06 states 4 and 7)', () => {
  it('shows each wheel\'s face with its name written, and turns it a face at a time', () => {
    const fixture = TestBed.createComponent(LockBoard);
    fixture.componentRef.setInput('wheels', [0, 0, 2, 5]);
    fixture.componentRef.setInput('faces', RUNES);
    fixture.componentRef.setInput('mode', 'play');
    const turns: Turn[] = [];
    fixture.componentInstance.turn.subscribe((t) => turns.push(t));
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(Array.from(el.querySelectorAll('.name')).map((n) => n.textContent?.trim())).toEqual(['Lua', 'Lua', 'Onda', 'Estrela']);
    expect(Array.from(el.querySelectorAll('[role="group"][aria-label^="Roda "]')).map((g) => g.getAttribute('aria-label'))).toEqual(['Roda 1: Lua', 'Roda 2: Lua', 'Roda 3: Onda', 'Roda 4: Estrela']);
    (el.querySelector('[aria-label="Próximo símbolo: Roda 3"]') as HTMLElement).click();
    (el.querySelector('[aria-label="Símbolo anterior: Roda 1"]') as HTMLElement).click();
    expect(turns).toEqual([
      { index: 2, delta: 1 },
      { index: 0, delta: -1 },
    ]);
  });

  it('draws a digit as the character, with no second name beside it', () => {
    const fixture = TestBed.createComponent(LockBoard);
    fixture.componentRef.setInput('wheels', [3]);
    fixture.componentRef.setInput('faces', [{ key: '0', namePt: '0' }, { key: '1', namePt: '1' }, { key: '2', namePt: '2' }, { key: '3', namePt: '3' }]);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('.ch')?.textContent).toBe('3');
    expect(el.querySelector('.name')).toBeNull();
  });

  it('has no control in the master\'s static view', () => {
    const fixture = TestBed.createComponent(LockBoard);
    fixture.componentRef.setInput('wheels', [1, 2]);
    fixture.componentRef.setInput('faces', RUNES);
    fixture.componentRef.setInput('changed', [1]);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelectorAll('button')).toHaveLength(0);
    expect(el.querySelectorAll('.col--changed')).toHaveLength(1);
  });

  it('turns a pillar with "Girar", forward only, and names the pillar', () => {
    const fixture = TestBed.createComponent(PillarsBoard);
    fixture.componentRef.setInput('pillars', [3, 0, 2, 1]);
    fixture.componentRef.setInput('faces', pillarFaces(4));
    fixture.componentRef.setInput('mode', 'play');
    const turns: Turn[] = [];
    fixture.componentInstance.turn.subscribe((t) => turns.push(t));
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(Array.from(el.querySelectorAll('.head')).map((h) => h.textContent?.trim())).toEqual(['Pilar 1', 'Pilar 2', 'Pilar 3', 'Pilar 4']);
    expect(Array.from(el.querySelectorAll('.name')).map((n) => n.textContent?.trim())).toEqual(['Coruja', 'Corvo', 'Serpente', 'Lobo']);
    const girar = Array.from(el.querySelectorAll<HTMLButtonElement>('.turn'));
    expect(girar).toHaveLength(4);
    expect(girar[0].getAttribute('aria-label')).toBe('Girar o pilar 1');
    girar[1].click();
    expect(turns).toEqual([{ index: 1, delta: 1 }]);
    expect(el.querySelectorAll('.step')).toHaveLength(0);
  });

  it('gives the master arrows to choose the mural', () => {
    const fixture = TestBed.createComponent(PillarsBoard);
    fixture.componentRef.setInput('pillars', [0, 1, 2]);
    fixture.componentRef.setInput('faces', GLYPHS.slice(0, 3));
    fixture.componentRef.setInput('mode', 'edit');
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelectorAll('.step')).toHaveLength(6);
    expect(el.querySelectorAll('.turn')).toHaveLength(0);
  });
});

describe('PuzzleHost (the one place that knows the boards)', () => {
  function host(run: ReturnType<typeof create<typeof PuzzleRunSchema>>) {
    const fixture = TestBed.createComponent(PuzzleHost);
    fixture.componentRef.setInput('run', run);
    fixture.componentRef.setInput('mode', 'play');
    const moves: unknown[] = [];
    fixture.componentInstance.move.subscribe((m) => moves.push(m));
    fixture.detectChanges();
    return { el: fixture.nativeElement as HTMLElement, moves };
  }

  it('turns a press on the lights into the server\'s move', () => {
    const { el, moves } = host(create(PuzzleRunSchema, { kind: PuzzleKind.LIGHTS, config: { kind: { case: 'lights', value: { size: 3 } } }, state: { kind: { case: 'lights', value: { lit: flat(3, [0]) } } } }));
    (el.querySelector('[data-i="5"]') as HTMLElement).click();
    expect(moves).toEqual([{ kind: { case: 'lights', value: { row: 1, col: 2 } } }]);
  });

  it('turns a wheel and a pillar into the server\'s moves', () => {
    const lock = host(create(PuzzleRunSchema, { kind: PuzzleKind.LOCK, symbols: RUNES.map((s) => ({ key: s.key, namePt: s.namePt })), state: { kind: { case: 'lock', value: { wheels: [0, 1] } } } }));
    (lock.el.querySelector('[aria-label="Símbolo anterior: Roda 2"]') as HTMLElement).click();
    expect(lock.moves).toEqual([{ kind: { case: 'lock', value: { wheel: 1, delta: -1 } } }]);
    const pillars = host(create(PuzzleRunSchema, { kind: PuzzleKind.PILLARS, symbols: GLYPHS.slice(0, 3).map((s) => ({ key: s.key, namePt: s.namePt })), state: { kind: { case: 'pillars', value: { pillars: [0, 1, 2] } } } }));
    (pillars.el.querySelector('[aria-label="Girar o pilar 3"]') as HTMLElement).click();
    expect(pillars.moves).toEqual([{ kind: { case: 'pillars', value: { pillar: 2, delta: 1 } } }]);
  });

  it('says so, instead of drawing nothing, for a kind the app does not draw', () => {
    const { el } = host(create(PuzzleRunSchema, { kind: PuzzleKind.UNSPECIFIED }));
    expect(el.textContent).toContain('ainda não abre aqui');
  });

  it('turns a typed answer, a deciphered message and a bell into the server\'s moves', () => {
    const riddle = host(create(PuzzleRunSchema, { kind: PuzzleKind.RIDDLE, config: { kind: { case: 'riddle', value: { text: 'O que sou?' } } }, state: { kind: { case: 'riddle', value: {} } } }));
    typeInto(riddle.el.querySelector('input[name="answer"]')!, '  escuridão ');
    riddle.el.querySelector('form')!.dispatchEvent(new Event('submit'));
    expect(riddle.moves).toEqual([{ kind: { case: 'riddle', value: { answer: 'escuridão' } } }]);
    const cipher = host(create(PuzzleRunSchema, { kind: PuzzleKind.CIPHER, config: { kind: { case: 'cipher', value: { ciphertext: 'R WHVRXUR' } } }, state: { kind: { case: 'cipher', value: {} } } }));
    typeInto(cipher.el.querySelector('textarea')!, 'o tesouro');
    cipher.el.querySelector('form')!.dispatchEvent(new Event('submit'));
    expect(cipher.moves).toEqual([{ kind: { case: 'cipher', value: { text: 'o tesouro' } } }]);
    const sequence = host(
      create(PuzzleRunSchema, {
        kind: PuzzleKind.SEQUENCE,
        symbols: bellFaces(4).map((s) => ({ key: s.key, namePt: s.namePt })),
        state: { kind: { case: 'sequence', value: { progress: 0 } } },
        sequence: { totalSteps: 6, plays: 1, playing: false, shown: [], stepMs: 1200, nextInMs: 0 },
      }),
    );
    (sequence.el.querySelector('[aria-label="Sino largo"]') as HTMLElement).click();
    expect(sequence.moves).toEqual([{ kind: { case: 'sequence', value: { bell: 2 } } }]);
  });

  it('is the master\'s static view for the riddle and the cipher: the text alone, no field', () => {
    const fixture = TestBed.createComponent(PuzzleHost);
    fixture.componentRef.setInput('run', create(PuzzleRunSchema, { kind: PuzzleKind.RIDDLE, config: { kind: { case: 'riddle', value: { text: 'O que sou?' } } }, state: { kind: { case: 'riddle', value: {} } } }));
    fixture.componentRef.setInput('mode', 'view');
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.textContent).toContain('O que sou?');
    expect(el.querySelectorAll('input, button')).toHaveLength(0);
  });
});

function typeInto(field: Element, value: string): void {
  (field as HTMLInputElement).value = value;
  field.dispatchEvent(new Event('input'));
}

describe('RiddleBoard (E10-12 state 6)', () => {
  function riddle(inputs: Record<string, unknown>) {
    const fixture = TestBed.createComponent(RiddleBoard);
    fixture.componentRef.setInput('text', 'Moro embaixo de cada passo seu.');
    for (const [k, v] of Object.entries(inputs)) {
      fixture.componentRef.setInput(k, v);
    }
    const answers: string[] = [];
    fixture.componentInstance.answer.subscribe((a) => answers.push(a));
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement, answers };
  }

  it('keeps "Responder" off until something is typed, then sends the answer trimmed', () => {
    const { fixture, el, answers } = riddle({ mode: 'play' });
    const go = el.querySelector('button[type="submit"]') as HTMLButtonElement;
    expect(go.textContent?.trim()).toBe('Responder');
    expect(go.disabled || go.getAttribute('aria-disabled') === 'true').toBe(true);
    typeInto(el.querySelector('input')!, ' sombra ');
    fixture.detectChanges();
    el.querySelector('form')!.dispatchEvent(new Event('submit'));
    expect(answers).toEqual(['sombra']);
  });

  it('says "Não é isso." with an icon and the word, and never how close it was', () => {
    const { el } = riddle({ mode: 'play', verdict: 'wrong' });
    const notice = el.querySelector('.mr-notice--danger')!;
    expect(notice.getAttribute('role')).toBe('alert');
    expect(notice.textContent).toContain('Não é isso. Tente outra resposta.');
    expect(notice.querySelector('mat-icon')).not.toBeNull();
  });

  it('puts a dashed box with the reason where the field was when nothing can be typed', () => {
    const { el } = riddle({ mode: 'play', blocked: 'Você não tem mais tentativas.' });
    expect(el.querySelector('input')).toBeNull();
    expect(el.querySelector('.blocked')?.textContent).toContain('Você não tem mais tentativas.');
  });

  it('shows the counters between the field and the button', () => {
    const rows: CounterRow[] = [{ key: 'attempts', label: 'Suas tentativas', value: '2 de 3', spent: false }];
    const { el } = riddle({ mode: 'play', counters: rows });
    const form = el.querySelector('form')!;
    const order = Array.from(form.children).map((c) => c.tagName.toLowerCase());
    expect(order.indexOf('app-limit-counters')).toBeGreaterThan(order.indexOf('mat-form-field'));
    expect(order.indexOf('app-limit-counters')).toBeLessThan(order.indexOf('button'));
    expect(form.textContent).toContain('Suas tentativas');
    expect(form.textContent).toContain('2 de 3');
  });

  it('has no field in the master\'s view', () => {
    const { el } = riddle({ mode: 'view' });
    expect(el.querySelector('form')).toBeNull();
  });
});

describe('CipherBoard (E10-12 state 8)', () => {
  function cipher(inputs: Record<string, unknown>) {
    const fixture = TestBed.createComponent(CipherBoard);
    fixture.componentRef.setInput('ciphertext', 'R WHVRXUR HVWD VRE R DOWDU');
    for (const [k, v] of Object.entries(inputs)) {
      fixture.componentRef.setInput(k, v);
    }
    const answers: string[] = [];
    fixture.componentInstance.answer.subscribe((a) => answers.push(a));
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement, answers };
  }

  it('draws the letter and a decoding column for each distinct letter of it, in order', () => {
    const { el } = cipher({ mode: 'play' });
    expect(el.querySelector('.cipher')?.textContent).toBe('R WHVRXUR HVWD VRE R DOWDU');
    const letters = Array.from(el.querySelectorAll('.col__letter')).map((l) => l.textContent?.trim());
    expect(letters).toEqual(['D', 'E', 'H', 'O', 'R', 'U', 'V', 'W', 'X']);
    const bets = Array.from(el.querySelectorAll<HTMLInputElement>('.col__bet'));
    expect(bets[0].getAttribute('aria-label')).toBe('Sua aposta para a letra D');
    expect(bets[0].maxLength).toBe(1);
  });

  it('never reads the table: only "Conferir" sends the message', () => {
    const { fixture, el, answers } = cipher({ mode: 'play' });
    typeInto(el.querySelector('.col__bet')!, 'a');
    fixture.detectChanges();
    expect(answers).toEqual([]);
    expect((el.querySelector('.col__bet') as HTMLInputElement).value).toBe('A');
    typeInto(el.querySelector('textarea')!, 'o tesouro esta sobre o altar');
    fixture.detectChanges();
    expect(el.querySelector('button[type="submit"]')?.textContent?.trim()).toBe('Conferir');
    el.querySelector('form')!.dispatchEvent(new Event('submit'));
    expect(answers).toEqual(['o tesouro esta sobre o altar']);
  });

  it('says "Não é isso." and points at the table, and has no table in the master\'s view', () => {
    expect(cipher({ mode: 'play', verdict: 'wrong' }).el.textContent).toContain('Não é isso. Confira as letras da tabela.');
    const view = cipher({ mode: 'view' }).el;
    expect(view.querySelector('.col__bet')).toBeNull();
    expect(view.textContent).toContain('Os jogadores veem');
  });
});

describe('BellsBoard and SequenceStrip (E10-12 states 2 and 7)', () => {
  function bells(inputs: Record<string, unknown>) {
    const fixture = TestBed.createComponent(BellsBoard);
    fixture.componentRef.setInput('faces', bellFaces(4));
    for (const [k, v] of Object.entries(inputs)) {
      fixture.componentRef.setInput(k, v);
    }
    const struck: number[] = [];
    fixture.componentInstance.strike.subscribe((b) => struck.push(b));
    fixture.detectChanges();
    return { el: fixture.nativeElement as HTMLElement, struck };
  }

  it('is a button for each bell, named by its full name, that emits the bell\'s number', () => {
    const { el, struck } = bells({ mode: 'play' });
    const buttons = Array.from(el.querySelectorAll('button'));
    expect(buttons.map((b) => b.getAttribute('aria-label'))).toEqual(['Sino redondo', 'Sino alto', 'Sino largo', 'Sino pequeno']);
    expect(buttons.map((b) => b.textContent?.trim())).toEqual(['Redondo', 'Alto', 'Largo', 'Pequeno']);
    buttons[3].click();
    expect(struck).toEqual([3]);
  });

  it('frames the bell that is playing and does nothing while disabled', () => {
    const { el, struck } = bells({ mode: 'play', lit: 1, disabled: true });
    expect(el.querySelectorAll('.bell--lit')).toHaveLength(1);
    expect(el.querySelector('.bell--lit')?.getAttribute('aria-label')).toBe('Sino alto');
    (el.querySelector('button') as HTMLElement).click();
    expect(struck).toEqual([]);
    expect(el.querySelector('button')?.getAttribute('aria-disabled')).toBe('true');
  });

  it('draws a drawing for every bell of the table', () => {
    const { el } = bells({ mode: 'pick', faces: BELLS });
    expect(el.querySelectorAll('app-symbol-glyph svg')).toHaveLength(8);
  });

  it('lists the steps with the bell of each, and a dashed empty slot for one not shown yet', () => {
    const fixture = TestBed.createComponent(SequenceStrip);
    fixture.componentRef.setInput('faces', bellFaces(4));
    fixture.componentRef.setInput('steps', [0, 3]);
    fixture.componentRef.setInput('total', 4);
    fixture.componentRef.setInput('current', 1);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const items = Array.from(el.querySelectorAll('li'));
    expect(items.map((i) => i.getAttribute('aria-label'))).toEqual(['Passo 1: Sino redondo', 'Passo 2: Sino pequeno', 'Passo 3: ainda não mostrado', 'Passo 4: ainda não mostrado']);
    expect(items[1].classList).toContain('slot--lit');
    expect(items[2].classList).toContain('slot--empty');
  });
});

describe('SequenceBoard (E10-12 state 7)', () => {
  function board(playback: Record<string, unknown>, extra: Record<string, unknown> = {}) {
    const fixture = TestBed.createComponent(SequenceBoard);
    fixture.componentRef.setInput('playback', { totalSteps: 6, plays: 1, playing: false, shown: [], stepMs: 1200, nextInMs: 0, ...playback });
    fixture.componentRef.setInput('faces', bellFaces(4));
    for (const [k, v] of Object.entries(extra)) {
      fixture.componentRef.setInput(k, v);
    }
    const struck: number[] = [];
    fixture.componentInstance.strike.subscribe((b) => struck.push(b));
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement, struck };
  }

  it('says the master has not played it yet, with the bells still', () => {
    const { el, struck } = board({ plays: 0 });
    expect(el.textContent).toContain('O mestre ainda não tocou os sinos.');
    (el.querySelector('button') as HTMLElement).click();
    expect(struck).toEqual([]);
  });

  it('shows only the steps revealed so far, the step number and the bell playing, in words', () => {
    const { el } = board({ playing: true, shown: [0, 1, 3], nextInMs: 900 });
    expect(el.textContent).toContain('O mestre está tocando os sinos.');
    expect(el.textContent).toContain('passo 3 de 6');
    const items = Array.from(el.querySelectorAll('li'));
    expect(items).toHaveLength(6);
    expect(items.filter((i) => !i.classList.contains('slot--empty'))).toHaveLength(3);
    expect(el.querySelector('.big__name')?.textContent).toBe('Sino pequeno');
    expect(el.querySelector('[aria-live="polite"]')?.textContent).toContain('Passo 3 de 6: Sino pequeno.');
    // No bell to tap while it plays.
    expect(el.querySelectorAll('app-bells-board button')).toHaveLength(0);
  });

  it('lets any player repeat it once it ends, and counts the right steps the server says', () => {
    const { el, struck } = board({ plays: 2 }, { progress: 2 });
    expect(el.textContent).toContain('Agora é com vocês.');
    expect(el.textContent).toContain('Passos certos');
    expect(el.textContent).toContain('2 de 6');
    (el.querySelectorAll('button')[1] as HTMLElement).click();
    expect(struck).toEqual([1]);
  });

  it('says which step was wrong and who erred, above the bells', () => {
    const { el } = board({ plays: 2 }, { note: { lead: 'Errou o passo 4.', text: 'A tentativa recomeçou; Lia errou. Observem a sequência de novo.' } });
    const notice = el.querySelector('.mr-notice--danger')!;
    expect(notice.textContent).toContain('Errou o passo 4. A tentativa recomeçou; Lia errou.');
    expect(notice.getAttribute('role')).toBe('alert');
  });
});

describe('LimitCounters', () => {
  it('writes each counter as a label and a bold value, and says in words that one ran out', () => {
    const fixture = TestBed.createComponent(LimitCounters);
    fixture.componentRef.setInput('rows', [
      { key: 'moves', label: 'Jogadas', value: '10 de 10', spent: true },
      { key: 'time', label: 'Tempo', value: '4:48 de 5:00', spent: false },
    ] satisfies CounterRow[]);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const rows = Array.from(el.querySelectorAll('.row'));
    expect(rows[0].querySelector('dt')?.textContent).toBe('Jogadas');
    expect(rows[0].querySelector('dd')?.textContent?.replace(/\s+/g, ' ').trim()).toBe('10 de 10 · acabou');
    expect(rows[1].querySelector('dt')?.textContent).toBe('Tempo');
    expect(rows[1].querySelector('dd')?.textContent?.trim()).toBe('4:48 de 5:00');
  });
});
