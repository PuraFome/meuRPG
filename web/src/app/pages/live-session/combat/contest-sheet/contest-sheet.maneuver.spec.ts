import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CombatantKind, CombatantSide } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  CheckOptionSchema,
  ContestAttackOptionKind,
  ContestAttackOptionSchema,
  ContestPurpose,
  ContestTargetReason,
  ContestTargetSchema,
  ManeuverGrappleOptionSchema,
  RollModeKind,
} from '../../../../../gen/meurpg/play/v1/contest_types_pb';
import {
  CreatureSize,
  DisabledReasonCode,
  DisabledReasonSchema,
} from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { CombatState } from '../../../../core/combat/combat-state';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { ContestClient } from '../../../../core/combat/contest-client';
import { ContestState } from '../../../../core/combat/contest-state';
import {
  FakeContestClient,
  checkRoll,
  contestView,
  textOf,
} from '../../../../core/combat/contest-testing';
import { ContestSheet, type ContestSheetData } from './contest-sheet';

/** The table maneuver that starts a grapple, and the words of a target row the other spec does not reach. */
describe('ContestSheet: the maneuver, and the rows of the targets', () => {
  const maneuver = (over: Record<string, unknown>) =>
    create(ManeuverGrappleOptionSchema, {
      key: 'maneuver:grab',
      namePt: 'Golpe Agarrador',
      sides: 8,
      enabled: true,
      usesLeft: 2,
      ...over,
    });
  const option = (
    maneuvers: ReturnType<typeof maneuver>[],
    targets = [
      create(ContestTargetSchema, { combatantId: 'h', size: CreatureSize.MEDIUM, eligible: true }),
    ],
  ) =>
    create(ContestAttackOptionSchema, {
      kind: ContestAttackOptionKind.GRAPPLE,
      replacesAttack: true,
      enabled: true,
      targets,
      maneuvers,
      rollOption: create(CheckOptionSchema, {
        modifier: 5,
        known: true,
        mode: RollModeKind.NORMAL,
      }),
    });

  let api: FakeContestClient;

  function setup(
    attack: ReturnType<typeof option>,
    people = [combatant({ id: 'h', label: 'Hobgoblin' })],
    over: Partial<ContestSheetData> = {},
  ) {
    Element.prototype.scrollTo = vi.fn() as never;
    TestBed.resetTestingModule();
    api = new FakeContestClient();
    const enc = encounter({
      combatants: [
        combatant({
          id: 't',
          label: 'Toren',
          kind: CombatantKind.PLAYER,
          side: CombatantSide.PARTY,
        } as never),
        ...people,
      ],
    });
    api.encounterAnswer = enc;
    const state = new CombatState();
    state.encounter.set(enc);
    const data: ContestSheetData = {
      campaignId: 'c',
      encounterId: 'enc',
      initiatorId: 't',
      purpose: ContestPurpose.GRAPPLE,
      attack,
      diceMode: DiceMode.PLAYERS_CHOOSE,
      preference: DicePreference.APP,
      state,
      contests: new ContestState(),
      ...over,
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: ContestClient, useValue: api.as() },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close: vi.fn(), disableClose: false } },
      ],
    });
    const fixture = TestBed.createComponent(ContestSheet);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const settle = async () => {
      for (let i = 0; i < 3; i++) {
        await fixture.whenStable();
        fixture.detectChanges();
      }
    };
    const button = (name: string) =>
      Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
        b.textContent?.includes(name),
      );
    return { fixture, el, settle, button, text: () => textOf(el) };
  }

  const cards = (el: HTMLElement) =>
    Array.from(el.querySelectorAll('[aria-label="Golpe"] .card')).map((c) =>
      textOf(c).replace(/\s+/g, ' ').trim(),
    );

  it('lists "Só agarrar" and each maneuver with its die, uses and the bonus action', () => {
    const { el } = setup(
      option([maneuver({}), maneuver({ key: 'm2', namePt: 'Outro', usesLeft: 1 })]),
    );
    expect(cards(el)).toEqual([
      'Só agarrar Substitui um ataque.',
      'Golpe Agarrador d8 · 2 usos · ação bônus',
      'Outro d8 · 1 uso · ação bônus',
    ]);
  });

  it('says why a maneuver is off: no melee hit this turn, or the server reason', () => {
    const { el } = setup(
      option([
        maneuver({ enabled: false, noMeleeHit: true }),
        maneuver({
          key: 'm2',
          namePt: 'Outro',
          enabled: false,
          reason: create(DisabledReasonSchema, { code: DisabledReasonCode.BONUS_ACTION_USED }),
        }),
      ]),
    );
    const [, first, second] = cards(el);
    expect(first).toContain('Precisa de um acerto corpo a corpo neste turno.');
    expect(second).toContain('Ação bônus já usada');
    const radios = el.querySelectorAll<HTMLInputElement>('[aria-label="Golpe"] input');
    expect(radios[1].disabled).toBe(true);
  });

  it('asks for the die of the maneuver once picked, and sends its key and face with the roll', async () => {
    const { el, settle, button } = setup(option([maneuver({})]));
    el.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click();
    await settle();
    el.querySelectorAll<HTMLInputElement>('input[name="contest-maneuver"]')[1].click();
    await settle();
    button('Rolar a disputa')!.click();
    await settle();
    const field = el.querySelector<HTMLInputElement>('input[type="number"]')!;
    expect(textOf(el)).toContain('Dado do golpe (d8), só se você rolar dados físicos:');
    expect(field.value).toBe('');
    field.value = '6';
    field.dispatchEvent(new Event('input'));
    await settle();
    api.contest = contestView({ initiatorRoll: checkRoll() });
    button('Rolar no app')!.click();
    await settle();
    expect(api.started[0].input).toMatchObject({ maneuverKey: 'maneuver:grab', maneuverFace: 6 });
  });

  it('reads a face that is not a number as none', async () => {
    const { el, settle, button } = setup(option([maneuver({})]));
    el.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click();
    await settle();
    el.querySelectorAll<HTMLInputElement>('input[name="contest-maneuver"]')[1].click();
    await settle();
    button('Rolar a disputa')!.click();
    await settle();
    const field = el.querySelector<HTMLInputElement>('input[type="number"]')!;
    field.value = 'x';
    field.dispatchEvent(new Event('input'));
    await settle();
    api.contest = contestView({ initiatorRoll: checkRoll() });
    button('Rolar no app')!.click();
    await settle();
    expect(api.started[0].input).toMatchObject({ maneuverKey: 'maneuver:grab', maneuverFace: 0 });
  });

  it('sends no maneuver when the plain grapple stays picked', async () => {
    const { el, settle, button } = setup(option([maneuver({})]));
    el.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click();
    await settle();
    button('Rolar a disputa')!.click();
    await settle();
    api.contest = contestView({ initiatorRoll: checkRoll() });
    button('Rolar no app')!.click();
    await settle();
    expect(api.started[0].input).not.toHaveProperty('maneuverKey');
  });

  it('offers the maneuvers for a grapple only, not for a shove', () => {
    const { el } = setup(option([maneuver({})]), undefined, { purpose: ContestPurpose.SHOVE });
    expect(el.querySelector('[aria-label="Golpe"]')).toBeNull();
  });

  it('names a target it cannot find "Alguém", with no distance and no size when the server sends none', () => {
    const { el } = setup(
      option(
        [],
        [
          create(ContestTargetSchema, {
            combatantId: 'ghost',
            size: CreatureSize.UNSPECIFIED,
            distanceFt: 0,
            eligible: true,
          }),
        ],
      ),
      [],
    );
    const row = textOf(el.querySelector('label.card, label.target') ?? el)
      .replace(/\s+/g, ' ')
      .trim();
    expect(textOf(el)).toContain('Alguém');
    expect(row).toBeTruthy();
  });

  it('does not list a target that is not placed', () => {
    const { el } = setup(
      option(
        [],
        [
          create(ContestTargetSchema, {
            combatantId: 'h',
            size: CreatureSize.MEDIUM,
            eligible: true,
            reason: ContestTargetReason.NOT_PLACED,
          }),
        ],
      ),
    );
    expect(el.querySelectorAll('input[type="radio"]')).toHaveLength(0);
    expect(textOf(el)).toContain('Não há ninguém ao alcance.');
  });
});
