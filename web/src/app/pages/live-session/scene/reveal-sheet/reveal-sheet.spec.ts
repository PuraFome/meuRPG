import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { Code, ConnectError } from '@connectrpc/connect';

import { type SceneClue, SceneClueSchema } from '../../../../../gen/meurpg/maps/v1/maps_pb';
import { MapsClient } from '../../../../core/maps/maps-client';
import { FakeMapsClient } from '../../../../core/maps/maps-testing';
import { RevealSheet, type RevealSheetData } from './reveal-sheet';

const PLAYERS = [
  { id: 'p', name: 'Pensantus', playerName: 'Vinicius' },
  { id: 't', name: 'Toren', playerName: 'Caio' },
  { id: 'b', name: 'Brisa', playerName: 'Lia' },
];

function clue(to: string[] = []): SceneClue {
  return create(SceneClueSchema, {
    id: 'k3',
    text: 'Uma carta rasgada.',
    revealedTo: to.map((characterId) => ({ characterId, revealedAt: timestampFromDate(new Date()) })),
  });
}

describe('RevealSheet', () => {
  let api: FakeMapsClient;
  let close: ReturnType<typeof vi.fn>;

  function setup(c: SceneClue = clue()) {
    api = new FakeMapsClient();
    close = vi.fn();
    const data: RevealSheetData = {
      campaignId: 'c1',
      clue: c,
      number: 3,
      total: 3,
      sceneName: 'A carroça tombada',
      players: PLAYERS,
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: MapsClient, useValue: api },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close } },
      ],
    });
    const fixture = TestBed.createComponent(RevealSheet);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const settle = async () => {
      for (let i = 0; i < 3; i++) {
        await fixture.whenStable();
        fixture.detectChanges();
      }
    };
    const button = (name: string) =>
      Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) => b.textContent?.trim().includes(name))!;
    const row = (name: string) =>
      Array.from(el.querySelectorAll<HTMLInputElement>('.rv__row input')).find((i) => i.closest('label')?.textContent?.includes(name))!;
    const flat = (e: Element | null) => e?.textContent?.replace(/\s+/g, ' ').trim();
    return { fixture, el, settle, button, row, flat };
  }

  it('opens with nobody checked: the dashed button says why, the sentence says who is missing, and it reveals nothing', async () => {
    const { el, button, settle, flat } = setup();
    expect(flat(el.querySelector('.rv__clue'))).toBe('Uma carta rasgada.');
    expect(el.querySelectorAll('.rv__row input:checked')).toHaveLength(0);
    expect(flat(el.querySelector('.rv__summary'))).toBe('Ninguém marcado. Escolha quem recebe a pista.');
    const dashed = button('Revelar a pista');
    expect(dashed.getAttribute('aria-disabled')).toBe('true');
    expect(dashed.classList).toContain('rv__off');
    expect(dashed.querySelector('mat-icon')?.textContent).toBe('block');
    dashed.click();
    await settle();
    expect(api.calls).toEqual([]);
    expect(close).not.toHaveBeenCalled();
  });

  it('names the characters with their players, and the clue it is about in the subtitle', () => {
    const { el, flat } = setup();
    expect(Array.from(el.querySelectorAll('.rv__row'), (r) => flat(r))).toEqual([
      'checkPensantusVinicius',
      'checkTorenCaio',
      'checkBrisaLia',
    ]);
    expect(el.textContent).toContain('Pista 3 de 3 · A carroça tombada');
  });

  it('turns the button into the filled one that says who gets it: one name, a count, "todos", with no article', () => {
    const { fixture, el, row, button, flat } = setup();
    row('Brisa').click();
    fixture.detectChanges();
    expect(button('Revelar para Brisa').classList).toContain('mat-mdc-unelevated-button');
    expect(flat(el.querySelector('.rv__summary'))).toBe('A pista vai para 1 de 3 jogadores: Brisa.');
    row('Toren').click();
    fixture.detectChanges();
    expect(button('Revelar para 2 jogadores')).toBeTruthy();
    row('Pensantus').click();
    fixture.detectChanges();
    expect(button('Revelar para todos')).toBeTruthy();
    row('Pensantus').click();
    row('Toren').click();
    row('Brisa').click();
    fixture.detectChanges();
    expect(button('Revelar a pista').getAttribute('aria-disabled')).toBe('true');
  });

  it('"Marcar todos" checks everyone and becomes "Desmarcar todos"', () => {
    const { fixture, el, button } = setup();
    button('Marcar todos').click();
    fixture.detectChanges();
    expect(el.querySelectorAll('.rv__row input:checked')).toHaveLength(3);
    expect(button('Revelar para todos')).toBeTruthy();
    button('Desmarcar todos').click();
    fixture.detectChanges();
    expect(el.querySelectorAll('.rv__row input:checked')).toHaveLength(0);
    expect(button('Marcar todos')).toBeTruthy();
  });

  it('reveals to the checked characters only and answers the clue', async () => {
    const { fixture, row, button, settle } = setup();
    api.revealed = clue(['b']);
    row('Brisa').click();
    fixture.detectChanges();
    button('Revelar para Brisa').click();
    await settle();
    expect(api.calls).toEqual(['revealSceneClue k3 b']);
    expect(close).toHaveBeenCalledWith(api.revealed);
  });

  it('shows someone who already has it as checked, disabled and "Já tem a pista", and never sends them again', async () => {
    const { fixture, el, row, button, settle, flat } = setup(clue(['b']));
    const has = Array.from(el.querySelectorAll<HTMLInputElement>('.rv__row--has input'));
    expect(has).toHaveLength(1);
    expect(has[0].disabled).toBe(true);
    expect(has[0].checked).toBe(true);
    expect(flat(el.querySelector('.rv__row--has'))).toContain('Já tem a pista');
    button('Marcar todos').click();
    fixture.detectChanges();
    // Everyone who can still receive it is checked, but one already had it: not "todos".
    expect(el.textContent).not.toContain('Revelar para todos');
    button('Revelar aos outros').click();
    await settle();
    expect(api.calls).toEqual(['revealSceneClue k3 p,t']);
    expect(row('Pensantus')).toBeTruthy();
  });

  it('says what the server refused by its code, at the top, and stays open', async () => {
    const { fixture, el, row, button, settle } = setup();
    row('Brisa').click();
    fixture.detectChanges();
    api.failWith = new ConnectError('x', Code.NotFound);
    button('Revelar para Brisa').click();
    await settle();
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('A pista, ou um desses personagens, não existe mais.');
    expect(close).not.toHaveBeenCalled();
  });

  it('cancels without revealing', () => {
    const { button } = setup();
    button('Cancelar').click();
    expect(close).toHaveBeenCalledWith(undefined);
    expect(api.calls).toEqual([]);
  });
});
