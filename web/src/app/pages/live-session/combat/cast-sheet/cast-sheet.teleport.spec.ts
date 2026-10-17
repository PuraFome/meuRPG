import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { By } from '@angular/platform-browser';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  CombatantKind,
  EncounterBlockedReason,
  EncounterBlockedSchema,
  SpellTargetsSchema,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { ActionEconomy } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import { fakeCombatClient } from '../../../../core/combat/combat-client-testing';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { SpellCatalog } from '../../../../core/combat/spell-catalog';
import { TeleportMap } from '../../../../shared/area-picker/teleport-map';
import { settle } from '../../../../../test-settle';
import { CastSheet, type CastSheetData } from './cast-sheet';

const REACH_FT = 30;

const plain = (t: string | null | undefined) =>
  (t ?? '')
    .replace(/\u00a0/g, ' ')
    .replace(/\s+/g, ' ')
    .trim();

const pensantus = combatant({
  id: 'p',
  label: 'Pensantus',
  kind: CombatantKind.PLAYER,
  mine: true,
  col: 2,
  row: 4,
});

const mistyStep = { spell: { level: 2 }, damage: [], healBySlotLevel: {}, higherLevel: [] };

/** A refusal as the server sends it: `failed_precondition` with the typed reason. */
function refused(reason: EncounterBlockedReason): ConnectError {
  return new ConnectError('refused', Code.FailedPrecondition, undefined, [
    { desc: EncounterBlockedSchema, value: create(EncounterBlockedSchema, { reason }) },
  ]);
}

async function open(
  over: { master?: boolean; map?: boolean; castSpell?: ReturnType<typeof vi.fn> } = {},
) {
  TestBed.resetTestingModule();
  const castSpell =
    over.castSpell ??
    vi.fn().mockResolvedValue({
      encounter: encounter({ combatants: [pensantus] }),
      cast: { targets: [], pendingDamages: [], metamagicKeys: [], slot: { level: 2, pact: false } },
      summoned: [],
    });
  const data = {
    campaignId: 'c',
    encounterId: 'enc',
    casterId: 'p',
    round: 2,
    spellKey: 'spell:misty-step',
    name: 'Passo Nebuloso',
    level: 2,
    concentration: false,
    cantripDice: '',
    economy: ActionEconomy.BONUS_ACTION,
    slots: [{ level: 2, free: 2, pact: false }],
    usage: [{ level: 2, total: 2, used: 0 }],
    pact: null,
    targets: create(SpellTargetsSchema, {
      spellKey: 'spell:misty-step',
      teleportRangeFt: REACH_FT,
    }),
    shieldFree: null,
    shieldName: '',
    attackBonus: 0,
    diceMode: DiceMode.PLAYERS_CHOOSE,
    preference: DicePreference.APP,
    master: over.master ?? false,
    state: { encounter: signal(encounter({ combatants: [pensantus] })), apply: vi.fn() },
    map:
      over.map === false
        ? null
        : {
            image: { url: 'cave.png', width: 1200, height: 800 },
            mapName: 'A caverna',
            columns: 12,
            rows: 8,
            layers: null,
            fog: null,
          },
  } as unknown as CastSheetData;
  TestBed.configureTestingModule({
    providers: [
      { provide: CombatClient, useValue: fakeCombatClient({ castSpell }) },
      { provide: SpellCatalog, useValue: { details: () => Promise.resolve(mistyStep) } },
      { provide: MAT_DIALOG_DATA, useValue: data },
      { provide: MatDialogRef, useValue: { close: vi.fn() } },
    ],
  });
  const fixture = TestBed.createComponent(CastSheet);
  await settle(fixture);
  const el = fixture.nativeElement as HTMLElement;
  const cast = () =>
    Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
      plain(b.textContent).includes('Conjurar Passo Nebuloso'),
    )!;
  const pickSquare = async (col: number, row: number) => {
    fixture.debugElement.query(By.directive(TeleportMap)).componentInstance.pick.emit({ col, row });
    await settle(fixture);
  };
  return { fixture, el, cast, pickSquare, castSpell };
}

describe('CastSheet: a teleport spell on the map', () => {
  it('asks a player for the square and keeps "Conjurar" off until one is picked', async () => {
    const { fixture, el, cast, pickSquare, castSpell } = await open();
    expect(el.querySelector('app-teleport-map')).not.toBeNull();
    expect(cast().getAttribute('aria-disabled')).toBe('true');
    expect(plain(el.querySelector('.missing')?.textContent)).toContain(
      'Escolha no mapa o espaço para onde você vai.',
    );
    cast().click();
    await settle(fixture);
    expect(castSpell).not.toHaveBeenCalled();
    await pickSquare(6, 4);
    expect(cast().getAttribute('aria-disabled')).not.toBe('true');
  });

  it('casts with the square picked, as the destination', async () => {
    const { fixture, cast, pickSquare, castSpell } = await open();
    await pickSquare(6, 4);
    cast().click();
    await settle(fixture);
    expect(castSpell).toHaveBeenCalledTimes(1);
    const call = castSpell.mock.calls[0] as unknown[];
    expect(call[12]).toEqual({ destination: { col: 6, row: 4 } });
  });

  it('turns the server refusals into the words of the table', async () => {
    const castSpell = vi
      .fn()
      .mockRejectedValueOnce(refused(EncounterBlockedReason.TELEPORT_TOO_FAR))
      .mockRejectedValueOnce(refused(EncounterBlockedReason.DESTINATION_UNAVAILABLE))
      .mockRejectedValueOnce(refused(EncounterBlockedReason.DESTINATION_REQUIRED));
    const { fixture, el, cast, pickSquare } = await open({ castSpell });
    await pickSquare(6, 4);
    const said: string[] = [];
    for (let i = 0; i < 3; i++) {
      cast().click();
      await settle(fixture);
      said.push(plain(el.querySelector('[role="alert"] p')?.textContent));
    }
    expect(said).toEqual([
      'Longe demais',
      'Esse espaço não está disponível',
      'Escolha no mapa o espaço para onde você vai.',
    ]);
  });

  it('lets the master cast without a square, and with one', async () => {
    const { fixture, cast, pickSquare, castSpell } = await open({ master: true });
    expect(cast().getAttribute('aria-disabled')).not.toBe('true');
    await pickSquare(9, 4);
    cast().click();
    await settle(fixture);
    expect((castSpell.mock.calls[0] as unknown[])[12]).toEqual({ destination: { col: 9, row: 4 } });
  });

  it('has no picker without a map, and casts as before', async () => {
    const { fixture, el, cast, castSpell } = await open({ map: false });
    expect(el.querySelector('app-teleport-map')).toBeNull();
    cast().click();
    await settle(fixture);
    expect((castSpell.mock.calls[0] as unknown[])[12]).toEqual({});
  });
});
