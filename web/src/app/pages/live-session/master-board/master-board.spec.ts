import { TestBed } from '@angular/core/testing';
import { signal } from '@angular/core';
import { type ActivatedRoute, type Router, convertToParamMap } from '@angular/router';
import { BehaviorSubject } from 'rxjs';

import { MasterBoard } from './master-board';

describe('MasterBoard', () => {
  const query = new BehaviorSubject(convertToParamMap({}));
  const navigate = vi.fn(() => Promise.resolve(true));
  const sceneOpen = signal(false);
  const rolls = signal(0);
  const openedHere = signal(false);
  const phone = signal(false);
  const narrow = signal(false);
  const unread = signal(0);

  function make(): MasterBoard {
    return TestBed.runInInjectionContext(
      () =>
        new MasterBoard({
          route: { queryParamMap: query } as unknown as ActivatedRoute,
          router: { navigate } as unknown as Router,
          sceneOpen,
          rolls,
          openedHere,
          phone,
          narrow,
          unread,
        }),
    );
  }

  beforeEach(() => {
    query.next(convertToParamMap({}));
    navigate.mockClear();
    sceneOpen.set(false);
    rolls.set(0);
    openedHere.set(false);
    phone.set(false);
    narrow.set(false);
    unread.set(0);
  });

  it('opens on "Mapa" with no scene, on "Cena" with one, and on what the address names', () => {
    const board = make();
    expect(board.tab()).toBe('mapa');
    sceneOpen.set(true);
    expect(board.tab()).toBe('cena');
    query.next(convertToParamMap({ vista: 'grupo' }));
    expect(board.tab()).toBe('grupo');
  });

  it('puts the chosen tab in the address, merged with the rest of it, replacing the entry', () => {
    make().choose('grupo');
    expect(navigate).toHaveBeenCalledWith([], {
      queryParams: { vista: 'grupo' },
      queryParamsHandling: 'merge',
      replaceUrl: true,
    });
  });

  it('counts the scene rolls that came while another tab was shown, and forgets them on "Cena"', () => {
    sceneOpen.set(true);
    rolls.set(1);
    query.next(convertToParamMap({ vista: 'mapa' }));
    const board = make();
    TestBed.flushEffects();
    expect(board.tabs()[0].badge).toBeUndefined();
    rolls.set(3);
    TestBed.flushEffects();
    expect(board.tabs()[0]).toMatchObject({ badge: '2', words: '2 rolagens novas' });
    query.next(convertToParamMap({ vista: 'cena' }));
    TestBed.flushEffects();
    expect(board.tabs()[0].badge).toBeUndefined();
  });

  it('goes to "Cena" when the master opens a scene here', () => {
    query.next(convertToParamMap({ vista: 'mapa' }));
    make();
    openedHere.set(true);
    TestBed.flushEffects();
    expect(navigate).toHaveBeenCalledWith(
      [],
      expect.objectContaining({ queryParams: { vista: 'cena' } }),
    );
  });

  it('counts the long casts that wait in "Grupo"', () => {
    const board = make();
    board.setCastsWaiting(1);
    expect(board.tabs()[2]).toMatchObject({
      badge: '1',
      tone: 'wait',
      words: '1 conjuração esperando',
    });
  });

  it('has no tab for the map on a phone: it is a fold of the scene', () => {
    phone.set(true);
    query.next(convertToParamMap({ vista: 'mapa' }));
    expect(make().tab()).toBe('cena');
  });

  it('has the Conversa as a tab only on a tablet, with the lines that came while another tab showed', () => {
    sceneOpen.set(true);
    const board = make();
    query.next(convertToParamMap({ vista: 'conversa' }));
    // A computer has the rail: the name is no tab.
    expect(board.tab()).toBe('cena');
    expect(board.tabs().map((t) => t.label)).toEqual(['Cena', 'Mapa', 'Grupo']);
    narrow.set(true);
    expect(board.tab()).toBe('conversa');
    expect(board.tabs().map((t) => t.label)).toEqual(['Cena', 'Mapa', 'Grupo', 'Conversa']);
    expect(board.counts().unread).toBe(0);
    query.next(convertToParamMap({ vista: 'grupo' }));
    unread.set(2);
    expect(board.tabs()[3]).toMatchObject({ badge: '2', words: '2 mensagens novas' });
    // A phone has the pinned nav instead.
    phone.set(true);
    expect(board.chatTab()).toBe(false);
  });
});
