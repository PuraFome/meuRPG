import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';

import type { ExperienceRow } from '../../../core/progression/experience-store';
import { ProgressionClient } from '../../../core/progression/progression-client';
import { type MarkReachedData, MarkReachedSheet } from './mark-reached-sheet';

const row = (id: string, name: string, sub: string): ExperienceRow => ({
  id, name, playerUserId: '', sub, level: 3, xp: 0, nextLevelXp: 2700, canLevelUp: false, levelUpReason: 0,
});
const rows = [row('p', 'Pensantus', 'Mago 3 · de Vinicius'), row('t', 'Toren', 'Guerreiro 3 · de Caio'), row('b', 'Brisa', 'Ladina 3 · de Lia')];

describe('MarkReachedSheet (E8-14)', () => {
  const api = { markMilestoneReached: vi.fn(), giveMilestoneTo: vi.fn() };
  const close = vi.fn();

  function setup(data: Partial<MarkReachedData> = {}) {
    TestBed.configureTestingModule({
      providers: [
        { provide: ProgressionClient, useValue: api },
        { provide: MAT_DIALOG_DATA, useValue: { campaignId: 'c1', milestoneId: 'm1', text: 'Chegar ao Vale Seco', rows, give: false, ...data } },
        { provide: MatDialogRef, useValue: { close } },
      ],
    });
    const fixture = TestBed.createComponent(MarkReachedSheet);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }
  const text = (el: HTMLElement) => el.textContent!.replace(/\s+/g, ' ');
  const buttons = (el: HTMLElement) => Array.from(el.querySelectorAll<HTMLButtonElement>('app-xp-actions button'));
  const checkboxes = (el: HTMLElement) => Array.from(el.querySelectorAll<HTMLInputElement>('input[type=checkbox]'));

  beforeEach(() => {
    api.markMilestoneReached.mockReset();
    api.giveMilestoneTo.mockReset();
    close.mockReset();
  });

  it('names the milestone in the title, checks everyone, and says the effect live', () => {
    const { fixture, el } = setup();
    expect(el.querySelector('h2')?.textContent).toBe('Marcar “Chegar ao Vale Seco” como alcançado');
    expect(checkboxes(el).every((c) => c.checked)).toBe(true);
    expect(text(el)).toContain('3 personagens podem subir de nível');

    checkboxes(el)[2].click();
    fixture.detectChanges();
    expect(text(el)).toContain('2 personagens podem subir de nível');
    expect(el.querySelector('.effect')?.getAttribute('aria-live')).toBe('polite');
  });

  it('with nobody checked, the button waits and says why', () => {
    const { fixture, el } = setup();
    checkboxes(el).forEach((c) => c.click());
    fixture.detectChanges();
    const go = buttons(el).find((b) => b.textContent?.includes('Marcar como alcançado'))!;
    expect(go.getAttribute('aria-disabled')).toBe('true');
    expect(text(el)).toContain('Marque pelo menos um personagem');
    go.click();
    expect(api.markMilestoneReached).not.toHaveBeenCalled();
  });

  it('marks the checked ones and answers who was left out', async () => {
    const { fixture, el } = setup();
    api.markMilestoneReached.mockResolvedValue({ milestone: undefined });
    checkboxes(el)[2].click();
    fixture.detectChanges();
    buttons(el).find((b) => b.textContent?.includes('Marcar como alcançado'))!.click();
    await fixture.whenStable();
    expect(api.markMilestoneReached).toHaveBeenCalledWith('c1', 'm1', ['p', 't'], expect.stringMatching(/^[0-9a-f-]{36}$/));
    expect(close).toHaveBeenCalledWith({ milestone: undefined, marked: ['Pensantus', 'Toren'], left: ['Brisa'] });
  });

  it('retries with the same key for the same people, and a new key for other people', async () => {
    const { fixture, el } = setup();
    api.markMilestoneReached.mockRejectedValueOnce(new Error('down')).mockResolvedValue({});
    const go = () => buttons(el).find((b) => b.textContent?.includes('Marcar como alcançado'))!;
    go().click();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('servidor');
    go().click();
    await fixture.whenStable();
    const [first, second] = api.markMilestoneReached.mock.calls.map((c) => c[3]);
    expect(second).toBe(first);
  });

  it('"Dar a mais alguém" lists only who is left and calls its own method', async () => {
    const { fixture, el } = setup({ give: true, rows: [rows[2]] });
    expect(el.querySelector('h2')?.textContent).toBe('Dar “Chegar ao Vale Seco” a mais alguém');
    expect(checkboxes(el)).toHaveLength(1);
    api.giveMilestoneTo.mockResolvedValue({});
    buttons(el).find((b) => b.textContent?.includes('Dar o marco'))!.click();
    await fixture.whenStable();
    expect(api.giveMilestoneTo).toHaveBeenCalledWith('c1', 'm1', ['b'], expect.any(String));
    expect(close).toHaveBeenCalled();
  });

  it('"Cancelar" closes without marking', () => {
    const { el } = setup();
    buttons(el).find((b) => b.textContent?.includes('Cancelar'))!.click();
    expect(close).toHaveBeenCalledWith(undefined);
    expect(api.markMilestoneReached).not.toHaveBeenCalled();
  });
});
