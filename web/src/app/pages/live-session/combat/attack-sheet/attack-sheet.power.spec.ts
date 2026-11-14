import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  AttackOutcome,
  CombatantKind,
  CombatantSide,
  CriticalDamageRule,
  EncounterMode,
  ManeuverDieOptionSchema,
  PendingDamageStatus,
  TargetInReachSchema,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { RollMode } from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import {
  ResourceBlockedReason,
  ResourceBlockedSchema,
} from '../../../../../gen/meurpg/play/v1/resources_pb';
import {
  AttackKind,
  AttackSchema,
  PowerAttackOptionSchema,
  RollDieSchema,
} from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import { fakeCombatClient } from '../../../../core/combat/combat-client-testing';
import { CombatState } from '../../../../core/combat/combat-state';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { AttackSheet, type AttackSheetData } from './attack-sheet';

const plain = (t: string | null | undefined) =>
  (t ?? '')
    .replace(/\u00a0/g, ' ')
    .replace(/\s+/g, ' ')
    .trim();

const montante = create(AttackSchema, {
  key: 'attack:greatsword',
  name: 'Greatsword',
  namePt: 'Montante',
  attackBonus: 6,
  rangeFt: 5,
  damage: '2d6 + 3',
  damageTypePt: 'cortante',
  kind: AttackKind.WEAPON,
});
const heavy = create(PowerAttackOptionSchema, {
  key: 'feat:golpe-pesado@mesa',
  name: 'Golpe Pesado',
  penalty: 5,
  damageBonus: 10,
});
const steady = create(PowerAttackOptionSchema, {
  key: 'feat:disparo-firme@mesa',
  name: 'Disparo Firme',
  penalty: 5,
  damageBonus: 10,
  disabled: true,
  disabledReasonPt: 'Sem usos de Fôlego do Atirador: voltam no descanso curto.',
});
const certeiro = create(ManeuverDieOptionSchema, {
  die: create(RollDieSchema, {
    key: 'golpe',
    namePt: 'Golpe Certeiro',
    applies: 'attack',
    sides: 8,
  }),
  resourceNamePt: 'Reserva de Golpes',
  usesLeft: 1,
});

const noUses = () =>
  new ConnectError('no use', Code.FailedPrecondition, undefined, [
    {
      desc: ResourceBlockedSchema,
      value: create(ResourceBlockedSchema, { reason: ResourceBlockedReason.NO_USES_LEFT }),
    },
  ]);

/** The d20 the server answers: the modifier is the bonus with the penalty already off (6 − 5 = 1). */
const answer = (outcome: AttackOutcome, faces: number[], pending?: object) => {
  const pair = faces.length > 1;
  const counted = pair ? Math.max(...faces) : faces[0];
  return {
    encounter: encounter({ mode: EncounterMode.THEATRE, currentCombatantId: 't' }),
    roll: {
      outcome,
      mode: pair ? RollMode.ADVANTAGE : RollMode.NORMAL,
      sources: [],
      bonusDice: [],
      d20: {
        diceCount: faces.length,
        diceSides: 20,
        faces,
        modifier: 1,
        total: counted + 1,
        countedIndex: faces.indexOf(counted),
        extraDice: [],
      },
    },
    pending,
  };
};

describe('AttackSheet: the power attack trade ("Troca do talento")', () => {
  const api = { rollAttack: vi.fn(), turnOptions: vi.fn(), rollDamage: vi.fn() };
  const lastCall = () => api.rollAttack.mock.calls.at(-1) ?? [];
  const extras = () => lastCall()[11] as { powerAttackKey?: string } | undefined;

  function setup(
    opts: {
      trades?: ReturnType<typeof create<typeof PowerAttackOptionSchema>>[];
      mode?: RollMode;
      diceMode?: DiceMode;
      dice?: (typeof certeiro)[];
      master?: boolean;
      opportunity?: boolean;
      resume?: object;
    } = {},
  ) {
    const toren = combatant({
      id: 't',
      label: 'Tor',
      kind: CombatantKind.PLAYER,
      side: CombatantSide.PARTY,
      mine: true,
    });
    const state = new CombatState();
    state.apply(
      encounter({
        mode: EncounterMode.THEATRE,
        currentCombatantId: 't',
        combatants: [toren, combatant({ id: 'z', label: 'Zumbi 1' })],
      }),
    );
    const data: AttackSheetData = {
      campaignId: 'c',
      encounterId: 'enc',
      attackerId: 't',
      round: 2,
      attack: montante,
      diceMode: opts.diceMode ?? DiceMode.PLAYERS_CHOOSE,
      preference: DicePreference.APP,
      state,
      master: opts.master ?? false,
      target: 'z',
      targets: [
        create(TargetInReachSchema, {
          combatantId: 'z',
          label: 'Zumbi 1',
          rollMode: opts.mode ?? RollMode.NORMAL,
          sources: [],
        }),
      ],
      ...(opts.opportunity
        ? {
            asReaction: true,
            targets: [],
            opportunity: { offerId: 'o1', targetId: 'z', targetLabel: 'Zumbi 1', byMaster: false },
          }
        : {}),
      ...(opts.trades ? { powerAttacks: opts.trades } : {}),
      ...(opts.dice ? { maneuverDice: opts.dice } : {}),
      ...(opts.resume ? { resume: { pending: opts.resume as never, targetLabel: 'Zumbi 1' } } : {}),
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close: () => undefined } },
        { provide: CombatClient, useValue: fakeCombatClient(api) },
      ],
    });
    const fixture = TestBed.createComponent(AttackSheet);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const settle = async () => {
      for (let i = 0; i < 3; i++) {
        await fixture.whenStable();
        fixture.detectChanges();
      }
    };
    const checks = () => [...el.querySelectorAll<HTMLInputElement>('app-power-trade-offer input')];
    const button = (label: string) =>
      [...el.querySelectorAll<HTMLButtonElement>('button')].find((b) =>
        plain(b.textContent).includes(label),
      );
    const press = (label: string) => {
      const b = button(label);
      expect(b, `the button "${label}"`).toBeDefined();
      b!.click();
    };
    const tick = (i = 0) => {
      expect(checks()[i], `the check ${i} of "Troca do talento"`).toBeDefined();
      checks()[i].click();
      fixture.detectChanges();
    };
    return { fixture, el, settle, checks, button, press, tick, text: () => plain(el.textContent) };
  }

  beforeEach(() => {
    api.rollAttack.mockReset().mockRejectedValue(new ConnectError('stop', Code.Aborted));
    api.turnOptions.mockReset();
    api.rollDamage.mockReset();
    Element.prototype.scrollTo = vi.fn();
  });

  describe('before the roll', () => {
    it('offers "Usar Golpe Pesado" off, with no box and the roll button as it always was (586-05 #1)', () => {
      const { text, checks, el, button } = setup({ trades: [heavy] });
      expect(text()).toContain('Troca do talento');
      expect(text()).toContain('Usar Golpe Pesado');
      expect(text()).toContain('−5 no ataque, +10 no dano se acertar');
      expect(checks().map((c) => c.checked)).toEqual([false]);
      expect(el.querySelector('.box')).toBeNull();
      expect(button('Rolar no app')).toBeDefined();
    });

    it('is off on every attack: a sheet opened after one that ticked it starts off again', () => {
      const first = setup({ trades: [heavy] });
      first.tick();
      expect(first.checks().map((c) => c.checked)).toEqual([true]);
      TestBed.resetTestingModule();
      expect(
        setup({ trades: [heavy] })
          .checks()
          .map((c) => c.checked),
      ).toEqual([false]);
    });

    it('offers nothing when the weapon lists no trade: a spell, an NPC or a weapon the talent does not accept (586-05 #2, #11)', () => {
      expect(setup({}).el.querySelector('app-power-trade-offer')).toBeNull();
      TestBed.resetTestingModule();
      expect(setup({ trades: [] }).el.querySelector('app-power-trade-offer')).toBeNull();
    });

    it('shows what the trade does to the roll and to the damage, and the button with the new modifier (586-03)', () => {
      const { tick, el, button, text } = setup({ trades: [heavy] });
      tick();
      const box = plain(el.querySelector('.box')?.textContent);
      expect(box).toContain('1d20 + 6→fica 1d20 + 1');
      expect(box).toContain('2d6 + 3→fica 2d6 + 13 cortante, se acertar');
      expect(button('Rolar 1d20 + 1 no app')).toBeDefined();
      expect(button('Rolar no app')).toBeUndefined();
      expect(text()).not.toContain('Rolar 1d20 + 6');
    });

    it('goes back to the sheet as it was when the check is taken off', () => {
      const { tick, el, button } = setup({ trades: [heavy] });
      tick();
      tick();
      expect(el.querySelector('.box')).toBeNull();
      expect(button('Rolar no app')).toBeDefined();
    });

    it('names the two d20 on the button with advantage or disadvantage, the mode shown as always (586-05 #13)', () => {
      for (const mode of [RollMode.ADVANTAGE, RollMode.DISADVANTAGE]) {
        TestBed.resetTestingModule();
        const { tick, button, text } = setup({ trades: [heavy], mode });
        expect(text()).toContain(mode === RollMode.ADVANTAGE ? 'Vantagem' : 'Desvantagem');
        tick();
        expect(button('Rolar 2d20 + 1 no app')).toBeDefined();
      }
    });

    it('is the same for an opportunity attack, which has no mode to pick', () => {
      const { tick, button } = setup({ trades: [heavy], opportunity: true });
      tick();
      expect(button('Rolar 1d20 + 1 no app')).toBeDefined();
    });

    it('is the same for the master acting as the character (586-05 #11)', () => {
      const { tick, button } = setup({ trades: [heavy], master: true });
      tick();
      expect(button('Rolar 1d20 + 1 no app')).toBeDefined();
    });

    it('takes the penalty off the typed total with physical dice', () => {
      const { tick, fixture, el, button } = setup({
        trades: [heavy],
        diceMode: DiceMode.PHYSICAL,
      });
      tick();
      expect(button('Rolar')).toBeUndefined();
      const field = el.querySelector<HTMLInputElement>('input[type="text"]')!;
      expect(plain(el.querySelector('label[for="' + field.id + '"]')?.textContent)).toContain(
        'Role 1d20 para Montante (+1)',
      );
      field.value = '16';
      field.dispatchEvent(new Event('input'));
      fixture.detectChanges();
      expect(plain(el.querySelector('.type__sum')?.textContent)).toContain('17');
    });
  });

  describe('a trade with no use left (586-05 #3)', () => {
    it('stays listed, off, with the reason the server sent, and cannot be ticked', () => {
      const { text, checks, tick, el } = setup({ trades: [steady] });
      expect(checks().map((c) => c.disabled)).toEqual([true]);
      expect(text()).toContain('Usar Disparo Firme');
      expect(text()).toContain('Sem usos de Fôlego do Atirador: voltam no descanso curto.');
      tick();
      expect(checks().map((c) => c.checked)).toEqual([false]);
      expect(el.querySelector('.box')).toBeNull();
    });

    it('never sends the key of a trade that is off', async () => {
      api.rollAttack.mockResolvedValue(answer(AttackOutcome.HIT, [16]));
      const { tick, press, settle } = setup({ trades: [steady] });
      tick();
      press('Rolar no app');
      await settle();
      expect(extras()?.powerAttackKey ?? '').toBe('');
    });

    it('lets one trade be ticked beside another that is off', () => {
      const { checks, tick } = setup({ trades: [steady, heavy] });
      tick(1);
      expect(checks().map((c) => c.checked)).toEqual([false, true]);
    });

    it('ticks one trade at a time: the second takes the check of the first', () => {
      const other = create(PowerAttackOptionSchema, {
        ...heavy,
        key: 'feat:outro@mesa',
        name: 'Outro',
      });
      const { checks, tick } = setup({ trades: [heavy, other] });
      tick(0);
      tick(1);
      expect(checks().map((c) => c.checked)).toEqual([false, true]);
    });
  });

  describe('the request', () => {
    it('sends the key when the check is on, and none when it is off', async () => {
      api.rollAttack.mockResolvedValue(answer(AttackOutcome.HIT, [16]));
      const on = setup({ trades: [heavy] });
      on.tick();
      on.press('Rolar 1d20 + 1 no app');
      await on.settle();
      expect(extras()?.powerAttackKey).toBe('feat:golpe-pesado@mesa');
      TestBed.resetTestingModule();
      const off = setup({ trades: [heavy] });
      off.press('Rolar no app');
      await off.settle();
      expect(extras()?.powerAttackKey ?? '').toBe('');
    });

    it('sends the key with the typed d20 too, and a new idempotency key when the check changes', async () => {
      api.rollAttack.mockRejectedValue(new ConnectError('lost', Code.Unavailable));
      const s = setup({ trades: [heavy] });
      s.press('Rolar no app');
      await s.settle();
      const without = lastCall()[6];
      s.tick();
      s.press('Rolar 1d20 + 1 no app');
      await s.settle();
      expect(lastCall()[6]).not.toBe(without);
      expect(extras()?.powerAttackKey).toBe('feat:golpe-pesado@mesa');
    });
  });

  describe('the roll and its line (586-04, 586-05 #7, #13)', () => {
    async function rolled(
      outcome: AttackOutcome,
      faces: number[],
      mode = RollMode.NORMAL,
      pending?: object,
    ) {
      api.rollAttack.mockResolvedValue(answer(outcome, faces, pending));
      const s = setup({ trades: [heavy], mode });
      s.tick();
      s.press(mode === RollMode.NORMAL ? 'Rolar 1d20 + 1 no app' : 'Rolar 2d20 + 1 no app');
      await s.settle();
      return s;
    }

    it('writes the roll with the bonus and the penalty apart, names the target and never its armor class', async () => {
      const { text, el } = await rolled(AttackOutcome.HIT, [16]);
      expect(text()).toContain('1d20 (16) + 6 − 5 = 17 contra o Zumbi 1');
      expect(plain(el.querySelector('.pill')?.textContent)).toContain('Acertou');
      expect(text()).toContain('Golpe Pesado: −5 no ataque');
      expect(text()).not.toMatch(/\bCA\b/);
      expect(text()).not.toContain('sem dano');
    });

    it('says a miss adds no damage', async () => {
      const { text, el } = await rolled(AttackOutcome.MISS, [4]);
      expect(plain(el.querySelector('.pill')?.textContent)).toContain('Errou');
      expect(text()).toContain('Golpe Pesado: −5 no ataque · sem dano, o +10 não entra');
    });

    it('is the same on a critical hit', async () => {
      const { text, el } = await rolled(AttackOutcome.CRITICAL_HIT, [20]);
      expect(plain(el.querySelector('.pill')?.textContent)).toContain('Crítico');
      expect(text()).toContain('1d20 (20) + 6 − 5 = 21 contra o Zumbi 1');
    });

    it('counts the penalty on the d20 that stands, with advantage and with disadvantage', async () => {
      const adv = await rolled(AttackOutcome.HIT, [9, 15], RollMode.ADVANTAGE);
      expect(adv.text()).toContain('15 + 6 − 5 = 16 contra o Zumbi 1');
      expect(adv.text()).toContain('Golpe Pesado: −5 no ataque');
      TestBed.resetTestingModule();
      api.rollAttack.mockResolvedValue({
        ...answer(AttackOutcome.MISS, [14, 4]),
      });
      const dis = setup({ trades: [heavy], mode: RollMode.DISADVANTAGE });
      dis.tick();
      dis.press('Rolar 2d20 + 1 no app');
      await dis.settle();
      expect(dis.text()).toContain('Golpe Pesado: −5 no ataque · sem dano, o +10 não entra');
    });

    it('does not mention the trade on an attack that made none', async () => {
      api.rollAttack.mockResolvedValue(answer(AttackOutcome.HIT, [16]));
      const s = setup({ trades: [heavy] });
      s.press('Rolar no app');
      await s.settle();
      expect(s.text()).not.toContain('Golpe Pesado: −5');
      expect(s.text()).not.toContain('contra o Zumbi 1');
    });
  });

  describe('a use that cannot be spent (586-05 #3, #4)', () => {
    it('asks to take one check off when the die and the trade share a pool with one use left', async () => {
      api.rollAttack.mockRejectedValue(noUses());
      api.turnOptions.mockResolvedValue({ maneuverDice: [certeiro] });
      const s = setup({ trades: [heavy], dice: [certeiro] });
      expect(s.el.querySelector('app-roll-die-offer input')).not.toBeNull();
      s.el.querySelector<HTMLInputElement>('app-roll-die-offer input')!.click();
      s.fixture.detectChanges();
      s.tick();
      s.press('Rolar 1d20 + 1 no app');
      await s.settle();
      expect(plain(s.el.querySelector('[role="alert"]')?.textContent)).toContain(
        'Sem usos suficientes: cada um gasta 1 de Reserva de Golpes, e resta 1. Desmarque um.',
      );
      expect(s.checks().map((c) => c.checked)).toEqual([true]);
      expect(s.el.querySelector<HTMLInputElement>('app-roll-die-offer input')!.checked).toBe(true);
    });

    it('reads the uses again when the trade alone found none, and lists it off', async () => {
      api.rollAttack.mockRejectedValue(noUses());
      api.turnOptions.mockResolvedValue({
        maneuverDice: [],
        options: {
          attacks: [
            {
              attack: montante,
              powerAttacks: [
                create(PowerAttackOptionSchema, {
                  ...heavy,
                  disabled: true,
                  disabledReasonPt: 'Sem usos de Reserva de Golpes: voltam no descanso curto.',
                }),
              ],
            },
          ],
        },
      });
      const s = setup({ trades: [heavy] });
      s.tick();
      s.press('Rolar 1d20 + 1 no app');
      await s.settle();
      expect(plain(s.el.querySelector('[role="alert"]')?.textContent)).toContain(
        'Golpe Pesado: não há usos. Nada foi gasto.',
      );
      expect(s.checks().map((c) => c.checked)).toEqual([false]);
      expect(s.checks().map((c) => c.disabled)).toEqual([true]);
      expect(s.text()).toContain('Sem usos de Reserva de Golpes: voltam no descanso curto.');
    });
  });

  describe('the damage of a hit', () => {
    const pending = (over: object = {}) => ({
      id: 'p1',
      attackerId: 't',
      targetId: 'z',
      attackKey: montante.key,
      status: PendingDamageStatus.AWAITING_ROLL,
      diceCount: 2,
      diceSides: 6,
      bonus: 3,
      critical: false,
      criticalRule: CriticalDamageRule.DOUBLED_DICE,
      criticalMax: 0,
      parts: [
        {
          key: 'weapon',
          labelPt: 'Montante',
          diceCount: 2,
          diceSides: 6,
          flat: 3,
          damageTypePt: 'cortante',
        },
        {
          key: 'power_attack',
          labelPt: 'Golpe Pesado',
          diceCount: 0,
          diceSides: 0,
          flat: 10,
          auto: true,
          damageTypePt: 'cortante',
        },
      ],
      ...over,
    });

    it('lists the +10 as an automatic part and rolls 2d6 + 13 in the app (586-04)', () => {
      const { text, button } = setup({ resume: pending() });
      expect(text()).toContain('O que entra no dano');
      expect(text()).toContain('Montante 2d6 + 3 de cortante');
      expect(text()).toContain('Golpe Pesado +10 de cortante automático');
      expect(button('Rolar 2d6 + 13 no app')).toBeDefined();
    });

    it('says the +10 is not doubled on a critical hit', () => {
      const { text } = setup({ resume: pending({ critical: true }) });
      expect(text()).toContain('num crítico, os dados dobram e o +10 não');
    });

    it('asks only for the dice with physical dice, and adds the +13 to the total (586-05 #8)', () => {
      const { fixture, el, text } = setup({ diceMode: DiceMode.PHYSICAL, resume: pending() });
      const fields = el.querySelectorAll('app-multi-roll input[type="text"]');
      expect(fields.length).toBe(1);
      const field = fields[0] as HTMLInputElement;
      field.value = '8';
      field.dispatchEvent(new Event('input'));
      fixture.detectChanges();
      expect(text()).toContain('21');
    });

    it('keeps the label the sheet always had for damage with no automatic trade part', () => {
      const p = pending();
      const { button } = setup({
        resume: { ...p, parts: p.parts.filter((x) => x.key === 'weapon') },
      });
      expect(button('Rolar o dano no app')).toBeDefined();
    });
  });
});
