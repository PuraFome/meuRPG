import { create } from '@bufbuild/protobuf';
import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  CombatantState,
  SpellCastSchema,
  SpellTargetsSchema,
  TargetInReachSchema,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  ActionEconomy,
  SpellTableOptionSchema,
  SpellDetailsSchema,
} from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { SpellCatalog } from '../../../../core/combat/spell-catalog';
import { CastSheet, type CastSheetData } from './cast-sheet';
import { fakeCombatClient } from '../../../../core/combat/combat-client-testing';

const flat = (n: Element | null | undefined) =>
  n?.textContent?.replace(/\s+/g, ' ').trim().replace(/ /g, ' ') ?? '';

const SWAP = 'feature:tinted-flame@mesa';

/** Invented options: a swap with a resource, one that is always on, and one with no uses left. */
const options = [
  create(SpellTableOptionSchema, {
    key: 'feature:warm-hands@mesa',
    namePt: 'Mãos mornas',
    summaryPt: '+1 de dano de fogo.',
    change: 'bonus_damage',
    alwaysOn: true,
    allowed: true,
  }),
  create(SpellTableOptionSchema, {
    key: SWAP,
    namePt: 'Chama tingida',
    summaryPt: 'O dano vira de outro tipo.',
    change: 'damage_type_swap',
    resource: 'tintas',
    usesLeft: 2,
    allowed: true,
    damageTypeKeys: ['damage-type:cold', 'damage-type:fire'],
  }),
  create(SpellTableOptionSchema, {
    key: 'feature:spare@mesa',
    namePt: 'Reserva',
    summaryPt: '+2 de dano.',
    change: 'bonus_damage',
    resource: 'reserva',
    allowed: false,
    disabledReasonPt: 'Sem usos: volta num descanso longo.',
  }),
];

const reach = (id: string, label: string, feet: number) =>
  create(TargetInReachSchema, {
    combatantId: id,
    label,
    state: CombatantState.UNHURT,
    distanceFt: feet,
  });

describe('CastSheet: the cast options of table features (MR-025)', () => {
  const castSpell = vi.fn();
  const close = vi.fn();

  function open(tableOptions: CastSheetData['tableOptions'] = options) {
    TestBed.resetTestingModule();
    const nael = combatant({ id: 'n', label: 'Nael', mine: true, characterId: 'nc' });
    const data = {
      campaignId: 'c',
      encounterId: 'enc',
      casterId: 'n',
      round: 2,
      spellKey: 'spell:ray-of-frost',
      name: 'Raio de Gelo',
      level: 0,
      concentration: false,
      cantripDice: '1d8',
      economy: ActionEconomy.ACTION,
      slots: [],
      usage: [],
      pact: null,
      targets: create(SpellTargetsSchema, {
        spellKey: 'spell:ray-of-frost',
        maxTargets: 1,
        targets: [reach('g1', 'Goblin 1', 20)],
      }),
      shieldFree: null,
      shieldName: '',
      attackBonus: 0,
      diceMode: DiceMode.PLAYERS_CHOOSE,
      preference: DicePreference.APP,
      state: { encounter: signal(encounter({ combatants: [nael] })), apply: vi.fn() },
      tableOptions,
    } as unknown as CastSheetData;
    TestBed.configureTestingModule({
      providers: [
        { provide: CombatClient, useValue: fakeCombatClient({ castSpell }) },
        {
          provide: SpellCatalog,
          useValue: { details: () => Promise.resolve(create(SpellDetailsSchema, {})) },
        },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close, disableClose: false } },
      ],
    });
    const fixture = TestBed.createComponent(CastSheet);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const settle = async () => {
      await fixture.whenStable();
      fixture.detectChanges();
      await fixture.whenStable();
      fixture.detectChanges();
    };
    const picker = () => el.querySelector('app-table-option-picker');
    const change = (input: HTMLInputElement, checked: boolean) => {
      input.checked = checked;
      input.dispatchEvent(new Event('change'));
      fixture.detectChanges();
    };
    const tick = (label: string) =>
      change(
        Array.from(picker()!.querySelectorAll<HTMLInputElement>('input[type=checkbox]')).find((c) =>
          flat(c.closest('label')).includes(label),
        )!,
        true,
      );
    const pickType = (label: string) =>
      change(
        Array.from(picker()!.querySelectorAll<HTMLInputElement>('input[type=radio]')).find((r) =>
          flat(r.closest('label')).includes(label),
        )!,
        true,
      );
    return { fixture, el, settle, picker, tick, pickType };
  }

  beforeEach(() => {
    castSpell.mockReset();
    close.mockReset();
  });

  const cast = async (el: HTMLElement, settle: () => Promise<void>) => {
    castSpell.mockResolvedValue({
      encounter: encounter(),
      cast: create(SpellCastSchema, { spellKey: 'spell:ray-of-frost', targets: [] }),
      summoned: [],
    });
    el.querySelector<HTMLInputElement>('app-cast-targets input[type=radio]')!.click();
    await settle();
    Array.from(el.querySelectorAll('button'))
      .find((b) => flat(b).includes('Conjurar Raio de Gelo'))!
      .click();
    await settle();
  };

  it('tells an always-on option, offers the others as checkboxes with their cost and greys the refused one with its reason', async () => {
    const { settle, picker } = open();
    await settle();
    const p = picker()!;
    expect(flat(p.querySelector('h3'))).toBe('Opções das suas características');
    const boxes = p.querySelectorAll<HTMLInputElement>('input[type=checkbox]');
    // The always-on option has no checkbox: it is told, not asked.
    expect(boxes.length).toBe(2);
    expect(flat(p)).toContain('Mãos mornas');
    expect(flat(p)).toContain('Sempre ativa');
    expect(flat(p)).toContain('Gasta 1 uso · restam 2');
    expect(boxes[0].disabled).toBe(false);
    expect(boxes[1].disabled).toBe(true);
    expect(flat(p)).toContain('Sem usos: volta num descanso longo.');
    expect(p.querySelector('input[type=radio]')).toBeNull();
  });

  it('shows nothing for a caster whose features offer none', async () => {
    const { settle, picker } = open([]);
    await settle();
    expect(picker()).toBeNull();
  });

  it('asks the new damage type of a ticked swap and sends it with the cast', async () => {
    const { el, settle, tick, pickType, picker } = open();
    await settle();
    tick('Chama tingida');
    expect(flat(picker()!.querySelector('legend'))).toBe('Novo tipo de dano · Chama tingida');
    expect(
      Array.from(picker()!.querySelectorAll('input[type=radio]'), (r) => flat(r.closest('label'))),
    ).toEqual(['Frio', 'Fogo']);
    pickType('Fogo');
    await cast(el, settle);
    expect(castSpell.mock.calls[0][12]).toMatchObject({
      spellOptions: [{ key: SWAP, damageTypeKey: 'damage-type:fire' }],
    });
  });

  it('sends the first type of the swap when none was picked, and names no option that is not ticked', async () => {
    const { el, settle, tick } = open();
    await settle();
    tick('Chama tingida');
    await cast(el, settle);
    expect(castSpell.mock.calls[0][12].spellOptions).toEqual([
      { key: SWAP, damageTypeKey: 'damage-type:cold' },
    ]);
  });

  it('sends no spell options when nothing was ticked', async () => {
    const { el, settle } = open();
    await settle();
    await cast(el, settle);
    expect(castSpell.mock.calls[0][12].spellOptions).toBeUndefined();
  });
});
