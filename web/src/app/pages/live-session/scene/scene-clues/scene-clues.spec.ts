import { TestBed } from '@angular/core/testing';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { of } from 'rxjs';

import { type SceneClue, SceneClueSchema } from '../../../../../gen/meurpg/maps/v1/maps_pb';
import type { CluePlayer } from '../../../../core/maps/scene-clues';
import { SceneState } from '../../../../core/play/scene-state';
import { masterScene } from '../../../../core/play/scene-testing';
import { SceneClues } from './scene-clues';

const PLAYERS: CluePlayer[] = [
  { id: 'p', name: 'Pensantus', playerName: 'Vinicius' },
  { id: 'b', name: 'Brisa', playerName: 'Lia' },
];

function clue(id: string, text: string, to: { id: string; at: Date }[] = []): SceneClue {
  return create(SceneClueSchema, {
    id,
    text,
    revealedTo: to.map((r) => ({
      characterId: r.id,
      characterName: r.id === 'p' ? 'Pensantus' : 'Brisa',
      revealedAt: timestampFromDate(r.at),
    })),
  });
}

const CLUES = [
  clue('k1', 'Um brasão de lobo queimado.', [
    { id: 'p', at: new Date(2026, 9, 3, 21, 20) },
    { id: 'b', at: new Date(2026, 9, 3, 21, 20) },
  ]),
  clue('k2', 'Rastros de três goblins.'),
  clue('k3', 'Uma carta rasgada.', [{ id: 'b', at: new Date(2026, 9, 3, 21, 26) }]),
];

describe('SceneClues (the open scene)', () => {
  let state: SceneState;
  let opened: unknown[];
  let answer: SceneClue | undefined;

  function setup(phone = false) {
    state = new SceneState(
      () => Promise.resolve(null),
      () => true,
    );
    state.apply(masterScene([], [], { clues: CLUES }));
    opened = [];
    answer = undefined;
    TestBed.configureTestingModule({
      providers: [
        {
          provide: MatDialog,
          useValue: {
            open: (_c: unknown, config: unknown) => {
              opened.push(config);
              return { afterClosed: () => of(answer) };
            },
          },
        },
        { provide: MatBottomSheet, useValue: {} },
      ],
    });
    Element.prototype.scrollIntoView = vi.fn();
    const fixture = TestBed.createComponent(SceneClues);
    fixture.componentRef.setInput('campaignId', 'c1');
    fixture.componentRef.setInput('state', state);
    fixture.componentRef.setInput('players', PLAYERS);
    fixture.componentRef.setInput('phone', phone);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const flat = (e: Element | null | undefined) => e?.textContent?.replace(/\s+/g, ' ').trim();
    const settle = async () => {
      for (let i = 0; i < 3; i++) {
        await fixture.whenStable();
        fixture.detectChanges();
      }
    };
    return { fixture, el, flat, settle };
  }

  it('lists the clues with who has each in words, and counts them', () => {
    const { el, flat } = setup();
    expect(flat(el.querySelector('.sc__count'))).toBe('3 pistas');
    const rows = Array.from(el.querySelectorAll('.sc__row'));
    expect(rows.map((r) => flat(r.querySelector('.sc__who')))).toEqual([
      'groupsRevelada para todos às 21:20',
      'visibility_offNinguém ainda',
      'personRevelada só para Brisa às 21:26',
    ]);
  });

  it('offers "Revelar" on a clue nobody has, "Revelar aos outros" on a partly revealed one and nothing on one given to everyone', () => {
    const { el } = setup();
    expect(el.querySelector('[data-reveal="k1"]')).toBeNull();
    expect(el.querySelector('[data-reveal="k2"]')?.textContent).toContain('Revelar');
    expect(el.querySelector('[data-reveal="k2"]')?.getAttribute('aria-label')).toBe(
      'Revelar a pista 2',
    );
    expect(el.querySelector('[data-reveal="k3"]')?.textContent).toContain('Revelar aos outros');
    expect(el.querySelector('[data-reveal="k3"]')?.getAttribute('aria-label')).toBe(
      'Revelar a pista 3 aos outros',
    );
    // There is no way to take a clue back.
    expect(el.textContent).not.toContain('Esconder');
  });

  it('keeps the hint short on a phone', () => {
    const { el, flat } = setup(true);
    expect(flat(el.querySelector('.sc__hint'))).toBe('Só quem você escolher recebe a pista.');
  });

  it("says on a computer that the clue goes to the person's notes", () => {
    const { el, flat } = setup(false);
    expect(flat(el.querySelector('.sc__hint'))).toContain('vai para as anotações da pessoa');
  });

  it('opens the dialog for the clue with its place in the list and the players', () => {
    const { el } = setup();
    el.querySelector<HTMLButtonElement>('[data-reveal="k2"]')!.click();
    expect(opened).toHaveLength(1);
    expect(
      (opened[0] as { data: { number: number; total: number; sceneName: string } }).data,
    ).toMatchObject({
      number: 2,
      total: 3,
      sceneName: 'A carroça tombada',
    });
  });

  it('after revealing: the clue changes, a strip in a live region says what happened, and focus goes to its button', async () => {
    const { el, flat, settle } = setup();
    answer = clue('k2', 'Rastros de três goblins.', [
      { id: 'b', at: new Date(2026, 9, 3, 21, 31) },
    ]);
    el.querySelector<HTMLButtonElement>('[data-reveal="k2"]')!.click();
    await settle();
    expect(flat(el.querySelector('[role="status"]'))).toContain(
      'Pista revelada só para Brisa às 21:31.',
    );
    expect(flat(el.querySelectorAll('.sc__who')[1])).toContain('Revelada só para Brisa às 21:31');
    expect(el.querySelector('[data-reveal="k2"]')?.textContent).toContain('Revelar aos outros');
    expect(document.activeElement).toBe(el.querySelector('[data-reveal="k2"]'));
  });

  it('after revealing to everyone the button goes away and focus passes to the next clue that can be revealed', async () => {
    const { el, settle } = setup();
    answer = clue('k2', 'Rastros de três goblins.', [
      { id: 'p', at: new Date(2026, 9, 3, 21, 31) },
      { id: 'b', at: new Date(2026, 9, 3, 21, 31) },
    ]);
    el.querySelector<HTMLButtonElement>('[data-reveal="k2"]')!.click();
    await settle();
    expect(el.querySelector('[data-reveal="k2"]')).toBeNull();
    expect(document.activeElement).toBe(el.querySelector('[data-reveal="k3"]'));
  });

  it('changes nothing when the dialog is closed without revealing', async () => {
    const { el, flat, settle } = setup();
    el.querySelector<HTMLButtonElement>('[data-reveal="k2"]')!.click();
    await settle();
    expect(el.querySelector('.sc__done')).toBeNull();
    expect(flat(el.querySelectorAll('.sc__who')[1])).toContain('Ninguém ainda');
  });

  it('invites the master to write clues when the scene has none', () => {
    const { fixture, el, flat } = setup();
    state.apply(masterScene([], [], { clues: [] }));
    fixture.detectChanges();
    expect(flat(el.querySelector('.sc__empty'))).toBe(
      'Esta cena não tem pistas. Adicione no editor do mapa.',
    );
  });
});
