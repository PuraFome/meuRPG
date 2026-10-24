import { Code, ConnectError } from '@connectrpc/connect';
import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { By } from '@angular/platform-browser';

import { CombatClient } from '../../../../core/combat/combat-client';
import { fakeCombatClient } from '../../../../core/combat/combat-client-testing';
import { combatant, encounter, spellZone } from '../../../../core/combat/combat-testing';
import { CombatMap } from '../../../../shared/combat-map/combat-map';
import { settle } from '../../../../../test-settle';
import { ZoneMoveSheet, type ZoneMoveSheetData } from './zone-move-sheet';

const plain = (t: string | null | undefined) =>
  (t ?? '').replace(/ /g, ' ').replace(/\s+/g, ' ').trim();

async function open(moveSpellZone: () => Promise<unknown>, master = true) {
  TestBed.resetTestingModule();
  const close = vi.fn();
  const apply = vi.fn();
  const next = encounter({ id: 'enc', revision: 7 });
  const move = vi.fn(async () => {
    await moveSpellZone();
    return next;
  });
  const data: ZoneMoveSheetData = {
    campaignId: 'c',
    encounterId: 'enc',
    zoneId: 'z1',
    master,
    map: {
      image: { url: 'cave.png', width: 1200, height: 800 },
      mapName: 'A caverna',
      columns: 20,
      rows: 8,
      layers: null,
      fog: null,
    },
    state: {
      encounter: signal(
        encounter({
          combatants: [combatant({ id: 'b', label: 'Bartolo', col: 1, row: 1 })],
          zones: [spellZone({ id: 'z1', origin: { col: 4, row: 4 } })],
        }),
      ),
      apply,
    } as never,
  };
  TestBed.configureTestingModule({
    providers: [
      { provide: CombatClient, useValue: fakeCombatClient({ moveSpellZone: move }) },
      { provide: MAT_DIALOG_DATA, useValue: data },
      { provide: MatDialogRef, useValue: { close } },
    ],
  });
  const fixture = TestBed.createComponent(ZoneMoveSheet);
  await settle(fixture);
  const el = fixture.nativeElement as HTMLElement;
  const buttons = () => Array.from(el.querySelectorAll<HTMLButtonElement>('.foot button'));
  const byText = (t: string) => buttons().find((b) => plain(b.textContent).includes(t))!;
  const pick = async (col: number, row: number) => {
    fixture.debugElement.query(By.directive(CombatMap)).triggerEventHandler('choose', { col, row });
    await settle(fixture);
  };
  return { fixture, el, close, apply, move, next, buttons, byText, pick };
}

describe('ZoneMoveSheet', () => {
  it('is named for the zone and has exactly one filled button, "Mover a zona", and "Cancelar"', async () => {
    const { el, buttons, byText } = await open(async () => undefined);
    expect(plain(el.querySelector('h2')?.textContent)).toBe('Mover o Raio Lunar');
    expect(buttons().filter((b) => b.className.includes('mat-mdc-unelevated-button'))).toHaveLength(
      1,
    );
    expect(byText('Mover a zona')).toBeTruthy();
    expect(byText('Cancelar')).toBeTruthy();
  });

  it('says the master spends no action and a player spends the caster action', async () => {
    const master = await open(async () => undefined, true);
    expect(plain(master.el.querySelector('.frame__sub')?.textContent)).toContain(
      'Sem limite de alcance',
    );
    expect(plain(master.el.querySelector('.frame__sub')?.textContent)).toContain('As setas movem');
    const player = await open(async () => undefined, false);
    expect(plain(player.el.querySelector('.frame__sub')?.textContent)).toContain(
      'Gasta a ação de Bartolo',
    );
    expect(plain(player.el.querySelector('.frame__sub')?.textContent)).toContain('Até 18 m');
    expect(player.el.querySelector('.hint')).toBeNull();
  });

  it('keeps the confirm off until a square other than the current one is picked', async () => {
    const { byText, pick, move } = await open(async () => undefined);
    expect(byText('Mover a zona').getAttribute('aria-disabled') ?? 'true').toBe('true');
    await pick(4, 4);
    expect(byText('Mover a zona').classList.contains('mr-button--off')).toBe(true);
    await pick(8, 4);
    expect(byText('Mover a zona').classList.contains('mr-button--off')).toBe(false);
    expect(move).not.toHaveBeenCalled();
  });

  it('shows the distance and the squares of the picked square', async () => {
    const { el, pick } = await open(async () => undefined);
    await pick(7, 4);
    expect(plain(el.querySelector('.summary')?.textContent)).toContain('4,5 m · 3 quadrados');
  });

  it('moves the zone with its id and the square, applies the combat and closes', async () => {
    const { byText, pick, move, apply, close, next, fixture } = await open(async () => undefined);
    await pick(8, 4);
    byText('Mover a zona').click();
    await settle(fixture);
    expect(move).toHaveBeenCalledWith('c', 'enc', 'z1', { col: 8, row: 4 });
    expect(apply).toHaveBeenCalledWith(next);
    expect(close).toHaveBeenCalledWith(true);
  });

  it('says the refusal in the words of the move and stays open', async () => {
    const { el, byText, pick, close, fixture } = await open(async () => {
      throw new ConnectError('gone', Code.NotFound);
    });
    await pick(8, 4);
    byText('Mover a zona').click();
    await settle(fixture);
    expect(plain(el.querySelector('[role="alert"]')?.textContent)).toContain('não existe mais');
    expect(close).not.toHaveBeenCalled();
  });

  it('keeps the confirm off for a player beyond the range, and says why', async () => {
    const { el, byText, pick, move } = await open(async () => undefined, false);
    await pick(19, 4);
    expect(plain(el.querySelector('[role="alert"]')?.textContent)).toContain('Longe demais');
    expect(byText('Mover a zona').classList.contains('mr-button--off')).toBe(true);
    byText('Mover a zona').click();
    expect(move).not.toHaveBeenCalled();
  });

  it('cancels without a call', async () => {
    const { byText, close, move } = await open(async () => undefined);
    byText('Cancelar').click();
    expect(close).toHaveBeenCalledWith(false);
    expect(move).not.toHaveBeenCalled();
  });
});
