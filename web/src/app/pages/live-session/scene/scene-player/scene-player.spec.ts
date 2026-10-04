import { TestBed } from '@angular/core/testing';
import { MatDialog } from '@angular/material/dialog';
import { of } from 'rxjs';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { SceneState } from '../../../../core/play/scene-state';
import { playerScene, sceneAction, sceneRoll } from '../../../../core/play/scene-testing';
import { ScenePlayer } from './scene-player';

describe('ScenePlayer', () => {
  /** The scene's actions with one at no attempts left, as the server sends it after the roll. */
  const exhausted = (id: string) => playerScene().actions.map((a) => (a.id === id ? { ...a, attemptsLeft: 0 } : a));
  function setup(scene = playerScene()) {
    const state = new SceneState(() => Promise.resolve(scene), () => false);
    state.apply(scene);
    const fixture = TestBed.createComponent(ScenePlayer);
    fixture.componentRef.setInput('campaignId', 'c1');
    fixture.componentRef.setInput('state', state);
    fixture.componentRef.setInput('diceMode', DiceMode.PLAYERS_CHOOSE);
    fixture.componentRef.setInput('dicePreference', DicePreference.APP);
    fixture.detectChanges();
    return { fixture, state, el: fixture.nativeElement as HTMLElement };
  }

  const rows = (el: HTMLElement) => Array.from(el.querySelectorAll<HTMLElement>('.sc__row'));
  const flat = (e: Element | null | undefined) => e?.textContent?.replace(/ /g, ' ').replace(/\s+/g, ' ').trim();
  /** The words of an element, without its icons' ligature names. */
  const words = (e: Element | null | undefined) => {
    if (!e) {
      return undefined;
    }
    const copy = e.cloneNode(true) as Element;
    copy.querySelectorAll('mat-icon').forEach((i) => i.remove());
    return flat(copy);
  };

  it('shows nothing while no scene is open', () => {
    const state = new SceneState(() => Promise.resolve(null), () => false);
    const fixture = TestBed.createComponent(ScenePlayer);
    fixture.componentRef.setInput('campaignId', 'c1');
    fixture.componentRef.setInput('state', state);
    fixture.componentRef.setInput('diceMode', DiceMode.PLAYERS_CHOOSE);
    fixture.componentRef.setInput('dicePreference', DicePreference.APP);
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).textContent?.trim()).toBe('');
  });

  it('shows the title, the description and one row per action with the own bonus and "Rolar"', () => {
    const { el } = setup();
    expect(el.querySelector('h2')?.textContent).toBe('Cena: A\u00a0carroça tombada');
    expect(el.textContent).toContain('Uma carroça de mercador tombada na estrada.');
    expect(rows(el)).toHaveLength(5);
    expect(flat(rows(el)[0])).toContain('Procurar pistas na carroça');
    expect(flat(rows(el)[0])).toContain('+6');
    expect(rows(el).map((r) => r.querySelector('.sc__bonus')?.textContent)).toEqual(['+6', '+1', '+1', '+1', '+3']);
    expect(rows(el).every((r) => r.querySelector('button')?.getAttribute('aria-label')?.startsWith('Rolar '))).toBe(true);
    expect(rows(el)[0].querySelector('button')?.getAttribute('aria-label')).toBe('Rolar Procurar pistas na carroça');
    expect(el.textContent).toContain('O mestre vê cada resultado. Você vê só os seus.');
  });

  it('names an action with no name by its check, with the kind under it', () => {
    const { el } = setup();
    expect(rows(el)[3].querySelector('.sc__name')?.textContent).toBe('Percepção');
    expect(rows(el)[3].querySelector('.sc__check')?.textContent).toBe('Perícia');
    expect(rows(el)[4].querySelector('.sc__check')?.textContent).toBe('Salvaguarda de Constituição');
  });

  it('shows the passive value only where the server sent one, in small print', () => {
    const { el } = setup();
    expect(rows(el).map((r) => flat(r.querySelector('.sc__passive')) ?? null)).toEqual([
      'Investigação passiva 16', null, null, 'Percepção passiva 11', null,
    ]);
  });

  it('never shows a DC, "passou" or the word "CD"', () => {
    const { el } = setup(playerScene([sceneRoll('r1', 'a1', 'Pensantus', 17)]));
    expect(el.textContent).not.toMatch(/\bCD\b|passou|Passou/);
  });

  it('turns a rolled action into the result and "Rolada", and keeps "Rolar" on the others', () => {
    const roll = sceneRoll('r1', 'a1', 'Pensantus', 17, {
      roll: { diceCount: 1, diceSides: 20, faces: [11], modifier: 6, total: 17 },
    });
    const { el } = setup(playerScene([roll], [], { actions: exhausted('a1') }));
    const first = rows(el)[0];
    expect(first.querySelector('.sc__total')?.textContent).toBe('17');
    expect(flat(first.querySelector('.sc__formula'))).toBe('1d20 (11) + 6 = 17');
    // The result sits where the bonus sits, with the width of "Rolar" kept empty after it.
    expect(first.querySelector('.sc__slot')).not.toBeNull();
    expect(flat(first.querySelector('.sc__done'))).toContain('Rolada');
    expect(first.querySelector('button')).toBeNull();
    expect(first.querySelector('.sc__bonus')).toBeNull();
    expect(rows(el).slice(1).every((r) => r.querySelector('button') !== null)).toBe(true);
  });

  it('has no "Rolar" for a player with no living character (no bonus)', () => {
    const scene = playerScene();
    const noBonus = sceneAction('a1', 'Investigação', { name: 'Procurar' });
    const { el } = setup({ ...scene, actions: [noBonus] });
    expect(rows(el)).toHaveLength(1);
    expect(el.querySelector('button')).toBeNull();
  });

  it('opens the roll sheet for the action, with the campaign\'s dice choice', () => {
    const open = vi.fn(() => ({ afterClosed: () => of(true) }));
    TestBed.overrideProvider(MatDialog, { useValue: { open } });
    const { el } = setup();
    rows(el)[1].querySelector('button')!.click();
    expect(open).toHaveBeenCalledTimes(1);
    const config = (open.mock.calls[0] as unknown[])[1] as { data: { action: { id: string }; diceMode: number } };
    expect(config.data.action.id).toBe('a2');
    expect(config.data.diceMode).toBe(DiceMode.PLAYERS_CHOOSE);
  });

  describe('the DC and the attempts (E8-13)', () => {
    const roll = (passed?: boolean) =>
      sceneRoll('r1', 'a1', 'Pensantus', 17, {
        passed,
        roll: { diceCount: 1, diceSides: 20, faces: [11], modifier: 6, total: 17 },
      });
    const withActions = (change: (a: ReturnType<typeof playerScene>['actions'][number]) => object) =>
      playerScene().actions.map((a) => ({ ...a, ...change(a) }));
    const tags = (el: HTMLElement) => rows(el).map((r) => words(r.querySelector('.mr-tag')) ?? null);
    const attempts = (el: HTMLElement) => rows(el).map((r) => words(r.querySelector('.sc__attempts')) ?? null);

    it('shows the "CD 12" pill only when the master shows the DC, and only on an action that has one', () => {
      const shown = playerScene([], [], {
        showDc: true,
        actions: withActions((a) => (a.id === 'a1' ? { dc: 12 } : a.id === 'a2' ? { dc: 13 } : {})),
      });
      const { el } = setup(shown);
      expect(tags(el)).toEqual(['CD 12', 'CD 13', null, null, null]);
      // The DC hidden: the server sends 0 and the row shows nothing.
      expect(tags(setup().el)).toEqual([null, null, null, null, null]);
    });

    it('turns the pill into "Passou · CD 12" with a check, or "Não passou · CD 10" with a cross', () => {
      const scene = playerScene([roll(true), sceneRoll('r2', 'a5', 'Pensantus', 7, { passed: false })], [], {
        showDc: true,
        actions: withActions((a) => (a.id === 'a1' ? { dc: 12, attemptsLeft: 0 } : a.id === 'a5' ? { dc: 10, attemptsLeft: 0 } : {})),
      });
      const { el } = setup(scene);
      const [first, , , , last] = rows(el);
      expect(words(first.querySelector('.mr-tag'))).toBe('Passou · CD 12');
      expect(first.querySelector('.mr-tag--success mat-icon')?.textContent).toBe('check');
      expect(words(last.querySelector('.mr-tag'))).toBe('Não passou · CD 10');
      expect(last.querySelector('.mr-tag--danger mat-icon')?.textContent).toBe('close');
      // The pill says how it went, so there is no "Rolada" line beside it.
      expect(first.querySelector('.sc__done')).toBeNull();
    });

    it('writes the attempts: "1 tentativa", "Restam 2 de 3 tentativas", "Restam N" over the limit, "Sem mais tentativas", and nothing when unlimited', () => {
      const scene = playerScene([roll()], [], {
        actions: withActions((a) =>
          a.id === 'a1' ? { attemptsLeft: 0 } : a.id === 'a4' ? { attemptsLeft: 2 } : a.id === 'a5' ? { attemptsLeft: 2 } : {},
        ),
      });
      const { el } = setup(scene);
      expect(attempts(el)).toEqual([
        'Sem mais tentativas',
        '1 tentativa',
        null,
        'Restam 2 de 3 tentativas',
        // maxAttempts 1 and a grant: above the limit, so no "de M".
        'Restam 2 tentativas',
      ]);
      expect(rows(el)[1].querySelector('.sc__attempts--left')).toBeNull();
      expect(rows(el)[3].querySelector('.sc__attempts--left')).not.toBeNull();
      expect(rows(el)[0].querySelector('.sc__attempts mat-icon')?.textContent).toBe('block');
    });

    it('keeps "Rolar" on an action with attempts left (and the last result), and drops it at none', () => {
      const scene = playerScene([sceneRoll('r1', 'a4', 'Pensantus', 9)], [], {
        actions: withActions((a) => (a.id === 'a4' ? { attemptsLeft: 2 } : a.id === 'a1' ? { attemptsLeft: 0 } : {})),
      });
      const { el } = setup(scene);
      const perception = rows(el)[3];
      expect(perception.querySelector('.sc__total')?.textContent).toBe('9');
      expect(perception.querySelector('button')?.getAttribute('aria-label')).toBe('Rolar Percepção');
      expect(perception.querySelector('.sc__done')).toBeNull();
      expect(rows(el)[0].querySelector('button')).toBeNull();
      // Every row keeps the slot or the button at its end: "Rolar" never moves.
      expect(rows(el).every((r) => r.querySelector('.sc__last')?.lastElementChild?.matches('button, .sc__slot'))).toBe(true);
    });

    it('says "Rolada às 21:14" when it was rolled out of attempts and no pill says how it went', () => {
      const { el } = setup(playerScene([roll()], [], { actions: exhausted('a1') }));
      expect(words(rows(el)[0].querySelector('.sc__done'))).toBe('Rolada às 21:14');
    });

    it('shows the last roll of an unlimited action and still offers "Rolar", with no counter', () => {
      const { el } = setup(playerScene([sceneRoll('r1', 'a3', 'Pensantus', 8)]));
      const row = rows(el)[2];
      expect(row.querySelector('.sc__total')?.textContent).toBe('8');
      expect(row.querySelector('.sc__attempts')).toBeNull();
      expect(row.querySelector('button')).not.toBeNull();
    });
  });
});
