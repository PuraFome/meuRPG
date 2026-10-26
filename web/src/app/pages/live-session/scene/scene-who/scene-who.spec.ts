import { TestBed } from '@angular/core/testing';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { of } from 'rxjs';

import { SceneClient } from '../../../../core/play/scene-client';
import { SceneState } from '../../../../core/play/scene-state';
import {
  FakeSceneClient,
  masterScene,
  sceneAudienceView,
} from '../../../../core/play/scene-testing';
import { nextTurn } from '../../../../../test-settle';
import { SceneWho } from './scene-who';

const flat = (e: Element | null | undefined) => {
  if (!e) {
    return undefined;
  }
  const copy = e.cloneNode(true) as Element;
  copy.querySelectorAll('mat-icon').forEach((i) => i.remove());
  return copy.textContent
    ?.replace(/\u00a0/g, ' ')
    .replace(/\s+/g, ' ')
    .trim();
};

describe('SceneWho ("Aberta desde … para", "Quem está na cena", "Os jogadores podem entrar")', () => {
  const dialogOpen = vi.fn();

  function setup(audience = sceneAudienceView(['Pensantus', 'Brisa'], ['Toren']), mayJoin = false) {
    const api = new FakeSceneClient();
    const scene = masterScene([], [], { audience, playersMayJoin: mayJoin });
    api.scene = scene;
    const state = new SceneState(
      () => api.get(),
      () => true,
    );
    state.apply(scene);
    dialogOpen.mockReset().mockReturnValue({ afterClosed: () => of(undefined) });
    TestBed.configureTestingModule({
      providers: [
        { provide: SceneClient, useValue: api },
        { provide: MatDialog, useValue: { open: dialogOpen } },
        { provide: MatBottomSheet, useValue: { open: dialogOpen } },
      ],
    });
    const fixture = TestBed.createComponent(SceneWho);
    fixture.componentRef.setInput('campaignId', 'c1');
    fixture.componentRef.setInput('state', state);
    fixture.detectChanges();
    return { fixture, api, state, el: fixture.nativeElement as HTMLElement };
  }

  async function settle(fixture: { detectChanges(): void; whenStable(): Promise<unknown> }) {
    for (let i = 0; i < 3; i++) {
      await fixture.whenStable();
      await nextTurn();
      fixture.detectChanges();
    }
  }

  it('names the players of a scene open to some, and says who is out', () => {
    const { el } = setup();
    expect(flat(el.querySelector('.who__lead'))).toMatch(/^Aberta desde \d\d:\d\d para$/);
    expect(
      Array.from(el.querySelectorAll('.who__chip')).map((c) =>
        c.textContent?.replace(/\s+/g, ' ').trim(),
      ),
    ).toEqual(['P Pensantus', 'B Brisa']);
    expect(flat(el.querySelector('.who__out'))).toBe('Toren não está nesta cena.');
    expect(flat(el.querySelector('.who__edit'))).toBe('Quem está na cena');
  });

  it('counts the players past three and lists who is out', () => {
    const { el } = setup(sceneAudienceView(['A', 'B', 'C', 'D', 'E', 'F'], ['Sálvia', 'Jabuti']));
    expect(flat(el.querySelector('.who__count'))).toBe('6 de 8 jogadores');
    expect(el.querySelectorAll('.who__chip')).toHaveLength(6);
    expect(flat(el.querySelector('.who__out'))).toBe('Fora da cena: Sálvia e Jabuti.');
  });

  it('says "Aberta para os jogadores" for a scene open to the whole table, with no switch', () => {
    const { el } = setup(sceneAudienceView(['Pensantus', 'Brisa', 'Toren'], [], true));
    expect(flat(el.querySelector('.who__since'))).toMatch(
      /^Aberta para os jogadores desde \d\d:\d\d$/,
    );
    expect(el.querySelector('app-keep-switch')).toBeNull();
    expect(flat(el.querySelector('.who__edit'))).toBe('Quem está na cena');
  });

  it('turns "Os jogadores podem entrar" on and off, and says it', async () => {
    const { el, api, fixture } = setup();
    const toggle = el.querySelector<HTMLButtonElement>('button[role="switch"]')!;
    expect(toggle.getAttribute('aria-checked')).toBe('false');
    expect(flat(el.querySelector('app-keep-switch'))).toContain('Os jogadores podem entrar');
    toggle.click();
    await settle(fixture);
    expect(api.calls).toContain('may-join true');
    expect(flat(el.querySelector('[role="status"]'))).toBe('Os jogadores podem entrar na cena.');
  });

  it('shows the switch on when the scene lets players in', () => {
    const { el } = setup(sceneAudienceView(['Pensantus'], ['Toren']), true);
    expect(el.querySelector('button[role="switch"]')?.getAttribute('aria-checked')).toBe('true');
  });

  it('says who entered, and when, after a player joined by themselves', async () => {
    const { el, state, fixture } = setup();
    state.apply(
      masterScene([], [], {
        audience: sceneAudienceView(['Pensantus', 'Brisa', 'Toren'], [], false, ['Toren']),
        playersMayJoin: true,
      }),
    );
    await settle(fixture);
    expect(flat(el.querySelector('.who__out--joined'))).toMatch(
      /^Toren entrou na cena às \d\d:\d\d\.$/,
    );
  });

  it('opens the chooser with the players in, and changes the scene with the answer', async () => {
    const { el, api, fixture } = setup();
    dialogOpen.mockReturnValue({
      afterClosed: () => of({ everyone: false, userIds: ['u-brisa', 'u-toren'] }),
    });
    el.querySelector<HTMLButtonElement>('.who__edit')!.click();
    await settle(fixture);
    const data = dialogOpen.mock.calls[0][1].data;
    expect(data.title).toBe('Quem está na cena?');
    expect(data.selected).toEqual(['u-pensantus', 'u-brisa']);
    expect(data.players.map((p: { name: string; sub: string }) => `${p.name}: ${p.sub}`)).toEqual([
      'Pensantus: na cena',
      'Brisa: na cena',
      'Toren: fora da cena',
    ]);
    expect(api.calls).toContain('audience u-brisa,u-toren');
  });

  it('asks the whole table when every player is chosen, and nothing when "Cancelar" is pressed', async () => {
    const { el, api, fixture } = setup();
    dialogOpen.mockReturnValue({ afterClosed: () => of(undefined) });
    el.querySelector<HTMLButtonElement>('.who__edit')!.click();
    await settle(fixture);
    expect(api.calls).toEqual([]);
    dialogOpen.mockReturnValue({
      afterClosed: () => of({ everyone: true, userIds: ['u-pensantus', 'u-brisa', 'u-toren'] }),
    });
    el.querySelector<HTMLButtonElement>('.who__edit')!.click();
    await settle(fixture);
    expect(api.calls).toEqual(['audience everyone']);
  });

  it('says what went wrong and reads the scene again', async () => {
    const { el, api, fixture } = setup();
    api.failWith = new (await import('@connectrpc/connect')).ConnectError('x', 3);
    el.querySelector<HTMLButtonElement>('button[role="switch"]')!.click();
    await settle(fixture);
    expect(flat(el.querySelector('[role="alert"]'))).toContain(
      'Alguém dessa lista não joga mais nesta campanha',
    );
  });
});
