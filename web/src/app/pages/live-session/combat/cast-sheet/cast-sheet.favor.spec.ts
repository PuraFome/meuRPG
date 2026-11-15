import { signal } from '@angular/core';
import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  CombatantKind,
  CombatantSide,
  CoverDegree,
  CoverSource,
  SpellCastSchema,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { RollMode } from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import { ActionEconomy, SpellAttackType } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import { fakeCombatClient } from '../../../../core/combat/combat-client-testing';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { SpellCatalog } from '../../../../core/combat/spell-catalog';
import { FavorClient } from '../../../../core/play/favor-client';
import { FavorState } from '../../../../core/play/favor-state';
import { FakeFavorClient, openFavor } from '../../../../core/play/favor-testing';
import { CastSheet, type CastSheetData } from './cast-sheet';

const plain = (t: string | null | undefined) => (t ?? '').replace(/\s+/g, ' ').trim();

async function open(holders: Record<string, string> = { 'char-c': 'Mago' }) {
  TestBed.resetTestingModule();
  const castSpell = vi.fn().mockRejectedValue(new Error('stop'));
  const bolt = {
    spell: { level: 0 },
    attackType: SpellAttackType.RANGED,
    healBySlotLevel: {},
    damage: [],
    higherLevel: [],
  };
  const state = {
    encounter: signal(
      encounter({
        combatants: [
          combatant({
            id: 'c',
            label: 'Mago',
            kind: CombatantKind.PLAYER,
            side: CombatantSide.PARTY,
          }),
        ],
      } as never),
    ),
    apply: vi.fn(),
  };
  const data = {
    campaignId: 'c',
    encounterId: 'enc',
    casterId: 'c',
    round: 1,
    spellKey: 'spell:fire-bolt',
    name: 'Raio de Fogo',
    level: 0,
    concentration: false,
    cantripDice: '1d10',
    economy: ActionEconomy.ACTION,
    slots: [],
    usage: [],
    pact: null,
    targets: {
      spellKey: 'spell:fire-bolt',
      targets: [
        {
          combatantId: 'g',
          label: 'Goblin',
          state: 1,
          cover: CoverDegree.NONE,
          coverSource: CoverSource.UNSPECIFIED,
          tooFar: false,
          untargetable: false,
          distanceFt: 30,
          rollMode: RollMode.NORMAL,
          sources: [],
          criticalOnHit: false,
        },
      ],
      maxTargets: 1,
      extraTargetPerLevel: false,
      darts: [],
    },
    shieldFree: null,
    shieldName: 'Escudo Arcano',
    attackBonus: 6,
    diceMode: DiceMode.PLAYERS_CHOOSE,
    preference: DicePreference.APP,
    state,
  } as unknown as CastSheetData;
  TestBed.configureTestingModule({
    providers: [
      { provide: MAT_DIALOG_DATA, useValue: data },
      { provide: MatDialogRef, useValue: { close: () => undefined } },
      { provide: CombatClient, useValue: fakeCombatClient({ castSpell }) },
      { provide: SpellCatalog, useValue: { details: () => Promise.resolve(bolt) } },
      { provide: FavorClient, useValue: new FakeFavorClient().asClient() },
    ],
  });
  openFavor(TestBed.inject(FavorState), holders);
  const fixture = TestBed.createComponent(CastSheet);
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  el.querySelector<HTMLInputElement>('app-cast-targets input')?.click();
  fixture.detectChanges();
  const box = () => el.querySelector<HTMLInputElement>('app-favor-spend input[type=checkbox]');
  const cast = async () => {
    [...el.querySelectorAll('button')]
      .find((b) => plain(b.textContent).includes('Conjurar e rolar'))!
      .click();
    await fixture.whenStable();
    fixture.detectChanges();
  };
  return { fixture, el, castSpell, box, cast };
}

/** The player picked a worse mode before ticking: the request would carry it, if the Inspiração were not spending. */
function pickDisadvantage(s: Awaited<ReturnType<typeof open>>) {
  [...s.el.querySelectorAll<HTMLElement>('.radio')]
    .find((r) => r.textContent!.includes('Desvantagem'))!
    .querySelector('input')!
    .click();
  s.fixture.detectChanges();
  const reason = s.el.querySelector<HTMLInputElement>('.reason__field')!;
  reason.value = 'atrapalhado pela fumaça';
  reason.dispatchEvent(new Event('input'));
  s.fixture.detectChanges();
}

describe('CastSheet: the Inspiração on a spell attack', () => {
  beforeEach(() => {
    Element.prototype.scrollTo = vi.fn();
  });

  it('shows no box when the caster has none', async () => {
    const { box } = await open({});
    expect(box()).toBeNull();
  });

  it('sends spend_favor and no mode when ticked, and neither when not', async () => {
    const off = await open();
    pickDisadvantage(off);
    await off.cast();
    // castSpell(campaign, encounter, caster, spell, slot, targets, roll, key, _, damageType, mode, metamagic, extras)
    expect(off.castSpell.mock.calls[0][10]).toMatchObject({ mode: RollMode.DISADVANTAGE });
    expect(off.castSpell.mock.calls[0][12].spendFavor).toBeUndefined();

    const on = await open();
    pickDisadvantage(on);
    on.box()!.click();
    on.fixture.detectChanges();
    expect(plain(on.el.textContent)).toContain('pela Inspiração');
    await on.cast();
    const args = on.castSpell.mock.calls[0];
    expect(args[10]).toBeUndefined();
    expect(args[12].spendFavor).toBe(true);
  });

  it('unticks the box after the spend and reads who has the Inspiração again', async () => {
    const s = await open();
    s.castSpell.mockResolvedValue({
      encounter: encounter({} as never),
      cast: create(SpellCastSchema, {}),
    });
    const refresh = vi.spyOn(TestBed.inject(FavorState), 'refresh').mockResolvedValue();
    s.box()!.click();
    s.fixture.detectChanges();
    await s.cast();
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(s.box()?.checked ?? false).toBe(false);
  });
});
