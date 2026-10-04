import { TestBed } from '@angular/core/testing';
import { MatDialog } from '@angular/material/dialog';
import { of } from 'rxjs';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { SceneState } from '../../../../core/play/scene-state';
import { playerScene, sceneAction, sceneRoll } from '../../../../core/play/scene-testing';
import { ScenePlayer } from './scene-player';

describe('ScenePlayer', () => {
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
    const { el } = setup(playerScene([roll]));
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
});
