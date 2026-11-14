import { create, type MessageInitShape } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  AttackOutcome,
  AttackRollSchema,
  CombatantKind,
  CombatantSide,
  DiceRollSchema,
  EncounterMode,
  TargetInReachSchema,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { RollMode } from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import {
  EffectModifierKind,
  EffectModifierSchema,
  type LastingEffect,
  LastingEffectSchema,
  WeaponHitDamageSchema,
} from '../../../../../gen/meurpg/play/v1/lasting_effects_pb';
import type { Attack } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import { fakeCombatClient } from '../../../../core/combat/combat-client-testing';
import { CombatState } from '../../../../core/combat/combat-state';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { AttackSheet, type AttackSheetData } from './attack-sheet';

const plain = (t: string | null | undefined) => (t ?? '').replace(/\s+/g, ' ').trim();
const longsword = {
  key: 'attack:longsword',
  name: 'Longsword',
  namePt: 'Espada longa',
  attackBonus: 5,
  saveDc: 0,
  rangeFt: 5,
  longRangeFt: 0,
  damage: '1d8 + 3',
  damageTypePt: 'cortante',
  kind: 0,
} as unknown as Attack;

/** A pending hit effect on the caster's own row. */
const pending = (
  id: string,
  name: string,
  trigger: string,
  extra: MessageInitShape<typeof LastingEffectSchema> = {},
) =>
  create(LastingEffectSchema, {
    id,
    sourceNamePt: name,
    casterId: 't',
    modifiers: [
      create(EffectModifierSchema, {
        kind: EffectModifierKind.WEAPON_HIT_DAMAGE,
        hit: create(WeaponHitDamageSchema, { trigger, die: 6, dice: 1 }),
      }),
    ],
    ...extra,
  });

describe('AttackSheet: the hit effect that keeps waiting after a miss (#577)', () => {
  const rollAttack = vi.fn();

  function open(effects: LastingEffect[]) {
    const toren = combatant({
      id: 't',
      label: 'Toren',
      kind: CombatantKind.PLAYER,
      side: CombatantSide.PARTY,
      mine: true,
      effects,
    } as never);
    const state = new CombatState();
    state.apply(
      encounter({
        mode: EncounterMode.THEATRE,
        currentCombatantId: 't',
        combatants: [toren, combatant({ id: 'cap', label: 'Capitão Goblin' })],
      }),
    );
    const data: AttackSheetData = {
      campaignId: 'c',
      encounterId: 'enc',
      attackerId: 't',
      round: 1,
      attack: longsword,
      diceMode: DiceMode.APP,
      preference: DicePreference.APP,
      state,
      target: 'cap',
      targets: [
        create(TargetInReachSchema, {
          combatantId: 'cap',
          label: 'Capitão Goblin',
          rollMode: RollMode.NORMAL,
          sources: [],
        }),
      ],
    };
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({
      providers: [
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close: () => undefined } },
        { provide: CombatClient, useValue: fakeCombatClient({ rollAttack }) },
      ],
    });
    const fixture = TestBed.createComponent(AttackSheet);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const attack = async (outcome: AttackOutcome) => {
      rollAttack.mockResolvedValue({
        encounter: state.encounter()!,
        roll: create(AttackRollSchema, {
          attackKey: longsword.key,
          outcome,
          d20: create(DiceRollSchema, {
            diceCount: 1,
            diceSides: 20,
            faces: [3],
            modifier: 5,
            total: 8,
          }),
        }),
        pending: undefined,
      });
      const roll = Array.from(el.querySelectorAll('button')).find((b) =>
        plain(b.textContent).includes('Rolar no app'),
      )!;
      roll.click();
      for (let i = 0; i < 3; i++) {
        await fixture.whenStable();
        fixture.detectChanges();
      }
    };
    return { el, attack };
  }

  beforeEach(() => {
    Element.prototype.scrollTo = vi.fn();
    rollAttack.mockReset();
  });

  it('says each pending effect keeps waiting for the weapon hit it rides on, with the way back', async () => {
    const { el, attack } = open([
      pending('a', 'Golpe Marcante', 'next_hit'),
      pending('b', 'Clarão na Lâmina', 'next_melee_hit'),
    ]);
    await attack(AttackOutcome.MISS);
    const text = plain(el.textContent);
    expect(text).toContain('Errou');
    expect(text).toContain(
      'Golpe Marcante continua esperando o próximo golpe com arma que acertar.',
    );
    expect(text).toContain(
      'Clarão na Lâmina continua esperando o próximo golpe corpo a corpo que acertar.',
    );
    expect(text).toContain('Voltar à sua vez');
  });

  it('says nothing for an effect that rides on every hit or on the marked creature', async () => {
    const { el, attack } = open([
      pending('a', 'Bênção da Lâmina', 'each_hit'),
      pending('b', 'Marca do Caçador', 'marked_target'),
    ]);
    await attack(AttackOutcome.MISS);
    expect(plain(el.textContent)).not.toContain('continua esperando');
  });

  it('says nothing for an effect already spent, nor after a hit', async () => {
    const spent = open([pending('a', 'Golpe Marcante', 'next_hit', { spent: true })]);
    await spent.attack(AttackOutcome.MISS);
    expect(plain(spent.el.textContent)).not.toContain('continua esperando');
    const hit = open([pending('a', 'Golpe Marcante', 'next_hit')]);
    await hit.attack(AttackOutcome.HIT);
    expect(plain(hit.el.textContent)).not.toContain('continua esperando');
  });

  it('says nothing for a row another caster put on this one', async () => {
    const { el, attack } = open([pending('a', 'Golpe Marcante', 'next_hit', { casterId: 'x' })]);
    await attack(AttackOutcome.MISS);
    expect(plain(el.textContent)).not.toContain('continua esperando');
  });
});
