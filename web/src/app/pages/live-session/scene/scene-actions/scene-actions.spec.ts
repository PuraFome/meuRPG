import { TestBed } from '@angular/core/testing';

import { SceneClient } from '../../../../core/play/scene-client';
import { SceneState } from '../../../../core/play/scene-state';
import { FakeSceneClient, masterScene, sceneRoll } from '../../../../core/play/scene-testing';
import { SceneActions } from './scene-actions';

const flat = (e: Element | null | undefined) =>
  e?.textContent?.replace(/ /g, ' ').replace(/\s+/g, ' ').trim();

describe('SceneActions ("Ações e rolagens", C3-01)', () => {
  const toren = sceneRoll('r1', 'a2', 'Toren', 7, { passed: false });
  const brisa = sceneRoll('r2', 'a4', 'Brisa', 17, {
    roll: { diceCount: 1, diceSides: 20, faces: [14], modifier: 3, total: 17, physical: true },
  });
  const pens = sceneRoll('r3', 'a1', 'Pensantus', 17, {
    passed: true,
    roll: { diceCount: 1, diceSides: 20, faces: [11], modifier: 6, total: 17 },
  });

  function setup(scene = masterScene([toren, brisa, pens])) {
    const api = new FakeSceneClient();
    const state = new SceneState(
      () => api.get(),
      () => true,
    );
    state.apply(scene);
    TestBed.configureTestingModule({ providers: [{ provide: SceneClient, useValue: api }] });
    const fixture = TestBed.createComponent(SceneActions);
    fixture.componentRef.setInput('campaignId', 'c1');
    fixture.componentRef.setInput('state', state);
    fixture.componentRef.setInput('scene', scene);
    fixture.detectChanges();
    const apply = (next: ReturnType<typeof masterScene>) => {
      state.apply(next);
      fixture.componentRef.setInput('scene', next);
      fixture.detectChanges();
    };
    return { fixture, state, apply, el: fixture.nativeElement as HTMLElement };
  }

  const rows = (el: HTMLElement) => Array.from(el.querySelectorAll('.sa__item'));
  const chips = (el: HTMLElement) => Array.from(el.querySelectorAll<HTMLButtonElement>('.chip'));

  it('draws one line per action: the name, then the check, the DC and the attempts', () => {
    const { el } = setup();
    expect(rows(el)).toHaveLength(5);
    expect(rows(el).map((r) => flat(r.querySelector('.sa__name')))).toEqual([
      'Procurar pistas na carroça',
      'Seguir os rastros dos goblins',
      'Acalmar os cavalos',
      'Percepção',
      'Resistir ao cheiro de fumaça',
    ]);
    expect(flat(rows(el)[0].querySelector('.sa__line'))).toBe('Investigação·CD 12·1 tentativa');
    expect(flat(rows(el)[3].querySelector('.sa__line'))).toBe('Perícia·3 tentativas');
    expect(flat(rows(el)[2].querySelector('.sa__line'))).toBe('Adestrar Animais·sem limite');
  });

  it('puts each roll beside its action as a chip: who, the total and the verdict in an icon and in words', () => {
    const { el } = setup();
    const first = Array.from(rows(el)[0].querySelectorAll<HTMLElement>('.chip'));
    expect(first.map((c) => flat(c))).toEqual(['PPensantus17checkPassou']);
    const second = rows(el)[1].querySelector<HTMLElement>('.chip')!;
    expect(second.classList.contains('chip--failed')).toBe(true);
    expect(flat(second)).toContain('Não passou');
    // An action with no DC has no verdict.
    const noDc = rows(el)[3].querySelector<HTMLElement>('.chip')!;
    expect(noDc.querySelector('mat-icon')).toBeNull();
  });

  it('opens the roll under its row from the chip: the formula, the verdict, and nothing before', () => {
    const { fixture, el } = setup();
    expect(el.querySelector('app-scene-roll-line')).toBeNull();
    const chip = chips(el)[0];
    expect(chip.getAttribute('aria-expanded')).toBe('false');
    chip.click();
    fixture.detectChanges();
    expect(chip.getAttribute('aria-expanded')).toBe('true');
    const line = el.querySelector('app-scene-roll-line')!;
    expect(flat(line.querySelector('.rl__formula'))).toBe('1d20 (11) + 6 = 17');
    expect(flat(line.querySelector('.mr-tag'))).toContain('Passou · CD 12');
    chip.click();
    fixture.detectChanges();
    expect(el.querySelector('app-scene-roll-line')).toBeNull();
  });

  it('says at the top whether the players see the DC, with an eye or a struck eye', () => {
    const off = setup();
    expect(flat(off.el.querySelector('.sa__dc'))).toContain('Só você vê a CD');
    expect(off.el.querySelector('.sa__dc mat-icon')?.textContent).toBe('visibility_off');
    TestBed.resetTestingModule();
    const on = setup(masterScene([], [], { showDc: true }));
    expect(flat(on.el.querySelector('.sa__dc'))).toContain('Os jogadores veem a CD');
    expect(on.el.querySelector('.sa__dc mat-icon')?.textContent).toBe('visibility');
  });

  it('counts the rolls that arrive while the page is open as new, until their chip is opened', () => {
    const { fixture, el, apply } = setup(masterScene([toren]));
    expect(el.querySelector('.sa__fresh')).toBeNull();
    apply(masterScene([pens, toren]));
    expect(flat(el.querySelector('.sa__fresh'))).toBe('1 rolagem nova');
    expect(flat(rows(el)[0].querySelector('.sa__new'))).toBe('1 rolagem nova');
    chips(el)
      .find((c) => flat(c)?.includes('Pensantus'))!
      .click();
    fixture.detectChanges();
    expect(el.querySelector('.sa__fresh')).toBeNull();
  });

  it('offers "Dar mais uma tentativa" in the opened roll only for a player with no attempts left', () => {
    const out = sceneRoll('r1', 'a2', 'Toren', 7, { passed: false, attemptsLeft: 0 });
    const some = sceneRoll('r2', 'a4', 'Pensantus', 9, { attemptsLeft: 2 });
    const { fixture, el } = setup(masterScene([out, some]));
    chips(el).forEach((c) => c.click());
    fixture.detectChanges();
    const grants = Array.from(el.querySelectorAll('.rl__grant'));
    expect(grants.map((g) => g.getAttribute('aria-label'))).toEqual([
      'Dar mais uma tentativa a Toren em Seguir os rastros dos goblins',
    ]);
  });

  it('has "Teste em grupo" in the header, named "Pedir teste em grupo", and it opens the form', () => {
    const { fixture, el } = setup();
    const button = el.querySelector<HTMLButtonElement>('.sa__group')!;
    expect(button.getAttribute('aria-label')).toBe('Pedir teste em grupo');
    expect(flat(button)).toContain('Teste em grupo');
    expect(button.getAttribute('aria-expanded')).toBe('false');
    button.click();
    fixture.detectChanges();
    expect(fixture.componentInstance.groupAsk()).toBe(true);
    expect(button.getAttribute('aria-expanded')).toBe('true');
  });

  it('lists the chips of one action newest first, as the server sends the rolls', () => {
    const newer = sceneRoll('r9', 'a4', 'Pensantus', 12, { passed: true });
    const { el } = setup(masterScene([newer, brisa]));
    const names = Array.from(rows(el)[3].querySelectorAll('.chip__who'), (c) => flat(c));
    expect(names).toEqual(['Pensantus', 'Brisa']);
  });
});
