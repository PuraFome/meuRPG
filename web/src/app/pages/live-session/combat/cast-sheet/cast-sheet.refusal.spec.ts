import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  EncounterBlockedReason,
  EncounterBlockedSchema,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { ActionEconomy } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import { fakeCombatClient } from '../../../../core/combat/combat-client-testing';
import { encounter } from '../../../../core/combat/combat-testing';
import { SpellCatalog } from '../../../../core/combat/spell-catalog';
import { CastSheet, type CastSheetData } from './cast-sheet';

const blocked = (reason: EncounterBlockedReason) =>
  new ConnectError('x', Code.FailedPrecondition, undefined, [
    { desc: EncounterBlockedSchema, value: create(EncounterBlockedSchema, { reason }) },
  ]);

/** An ordinary cast, outside a release: whatever the server refuses, the caster reads why. */
describe('CastSheet: a refused cast outside a release', () => {
  async function open(refusal: unknown) {
    TestBed.resetTestingModule();
    const castSpell = vi.fn().mockRejectedValue(refusal);
    const details = { spell: { level: 1 }, damage: [], healBySlotLevel: {}, higherLevel: [] };
    const data = {
      campaignId: 'c',
      encounterId: 'enc',
      casterId: 's',
      round: 1,
      spellKey: 'spell:bless',
      name: 'Bênção',
      level: 1,
      concentration: false,
      cantripDice: '',
      economy: ActionEconomy.ACTION,
      slots: [{ level: 1, free: 1, pact: false }],
      usage: [{ level: 1, total: 1, used: 0 }],
      pact: null,
      targets: undefined,
      shieldFree: null,
      shieldName: '',
      attackBonus: 0,
      diceMode: DiceMode.PLAYERS_CHOOSE,
      preference: DicePreference.APP,
      state: { encounter: signal(encounter({ combatants: [] })), apply: vi.fn() },
    } as unknown as CastSheetData;
    TestBed.configureTestingModule({
      providers: [
        { provide: CombatClient, useValue: fakeCombatClient({ castSpell }) },
        { provide: SpellCatalog, useValue: { details: () => Promise.resolve(details) } },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close: vi.fn() } },
      ],
    });
    const fixture = TestBed.createComponent(CastSheet);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    el.querySelector<HTMLInputElement>('app-slot-picker input')?.click();
    fixture.detectChanges();
    Array.from(el.querySelectorAll('button'))
      .find((b) => b.textContent?.includes('Conjurar Bênção'))!
      .click();
    await fixture.whenStable();
    fixture.detectChanges();
    return { castSpell, el };
  }

  it('says "Não é a sua vez." when the server refuses it as NOT_YOUR_TURN', async () => {
    const { castSpell, el } = await open(blocked(EncounterBlockedReason.NOT_YOUR_TURN));
    expect(castSpell).toHaveBeenCalledTimes(1);
    expect(el.querySelector('[role=alert]')?.textContent ?? '').toContain('Não é a sua vez.');
  });
});
