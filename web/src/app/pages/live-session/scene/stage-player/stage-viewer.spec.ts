import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';

import { SceneState } from '../../../../core/play/scene-state';
import { playerScene, stageNpc } from '../../../../core/play/scene-testing';
import { StageViewer, type StageViewerData } from './stage-viewer';

const flat = (e: Element | null | undefined) => {
  const copy = e?.cloneNode(true) as Element | undefined;
  copy?.querySelectorAll('mat-icon').forEach((i) => i.remove());
  return copy?.textContent?.replace(/\s+/g, ' ').trim();
};

describe('StageViewer', () => {
  const mira = stageNpc('s1', 'Mira', { portraitUrl: '/images/i1' });
  const aldo = stageNpc('s2', 'Aldo');

  function setup(entryId = 's1') {
    const state = new SceneState(
      () => Promise.resolve(null),
      () => false,
    );
    state.apply(playerScene([], [mira, aldo]));
    const close = vi.fn();
    const data: StageViewerData = { state, entryId };
    TestBed.configureTestingModule({
      providers: [
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close } },
      ],
    });
    const fixture = TestBed.createComponent(StageViewer);
    fixture.detectChanges();
    return { fixture, state, close, el: fixture.nativeElement as HTMLElement };
  }

  it('shows the name as the title and the portrait, and nothing else: no kind, no numbers, no action', () => {
    const { el } = setup();
    expect(flat(el.querySelector('#stage-viewer-title'))).toBe('Mira');
    expect(el.querySelector('.fig__img')?.getAttribute('src')).toBe('/images/i1');
    expect(flat(el)).toBe('MiraFechar');
  });

  it('has only "Fechar" and the ✕, both closing it', () => {
    const { el, close } = setup();
    const buttons = Array.from(el.querySelectorAll<HTMLButtonElement>('button'));
    expect(buttons.map((b) => b.getAttribute('aria-label') ?? flat(b))).toEqual([
      'Fechar',
      'Fechar',
    ]);
    buttons[0].click();
    buttons[1].click();
    expect(close).toHaveBeenCalledTimes(2);
  });

  it('draws the initials, in the same tile, for an NPC with no portrait', () => {
    const { el } = setup('s2');
    expect(el.querySelector('.fig__img')).toBeNull();
    expect(flat(el.querySelector('.fig__tile'))).toBe('AL');
  });

  it('falls back to the initials when the image fails, with no broken icon', () => {
    const { el, fixture } = setup();
    el.querySelector('.fig__img')!.dispatchEvent(new Event('error'));
    fixture.detectChanges();
    expect(el.querySelector('.fig__img')).toBeNull();
    expect(flat(el.querySelector('.fig__tile'))).toBe('MI');
  });

  it('closes by itself when the NPC leaves the stage, keeping its name meanwhile', () => {
    const { fixture, state, close, el } = setup();
    expect(close).not.toHaveBeenCalled();
    state.apply(playerScene([], [aldo]));
    fixture.detectChanges();
    expect(close).toHaveBeenCalled();
    expect(flat(el.querySelector('#stage-viewer-title'))).toBe('Mira');
  });

  it('stays open while another NPC leaves', () => {
    const { fixture, state, close } = setup();
    state.apply(playerScene([], [mira]));
    fixture.detectChanges();
    expect(close).not.toHaveBeenCalled();
  });
});
