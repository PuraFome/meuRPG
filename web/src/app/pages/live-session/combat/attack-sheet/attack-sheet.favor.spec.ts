import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  AttackOutcome,
  CombatantKind,
  CombatantSide,
  CoverDegree,
  CoverSource,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { RollMode } from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import { FavorBlockedReason } from '../../../../../gen/meurpg/play/v1/favor_pb';
import type { Attack } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import { fakeCombatClient } from '../../../../core/combat/combat-client-testing';
import { CombatState } from '../../../../core/combat/combat-state';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { FavorClient } from '../../../../core/play/favor-client';
import { FavorState } from '../../../../core/play/favor-state';
import { FakeFavorClient, favorRefused, openFavor } from '../../../../core/play/favor-testing';
import { AttackSheet, type AttackSheetData } from './attack-sheet';

const plain = (t: string | null | undefined) => (t ?? '').replace(/\s+/g, ' ').trim();
const sword = {
  key: 'attack:sword',
  name: 'Sword',
  namePt: 'Espada curta',
  attackBonus: 5,
  saveDc: 0,
  rangeFt: 5,
  longRangeFt: 0,
  damage: '1d6 + 3',
  damageTypePt: 'perfurante',
  kind: 0,
} as unknown as Attack;
const toren = combatant({
  id: 't',
  label: 'Toren',
  kind: CombatantKind.PLAYER,
  side: CombatantSide.PARTY,
  mine: true,
});
const cap = combatant({ id: 'cap', label: 'Capitão Goblin' });
const target = (extra: Record<string, unknown> = {}) => ({
  combatantId: 'cap',
  label: 'Capitão Goblin',
  state: 1,
  cover: CoverDegree.NONE,
  coverSource: CoverSource.UNSPECIFIED,
  tooFar: false,
  untargetable: false,
  rollMode: RollMode.NORMAL,
  sources: [],
  criticalOnHit: false,
  ...extra,
});
const rolled = (extra: Record<string, unknown> = {}) => ({
  encounter: encounter({ currentCombatantId: 't', combatants: [toren, cap] } as never),
  roll: {
    outcome: AttackOutcome.MISS,
    mode: RollMode.ADVANTAGE,
    suggestedMode: RollMode.NORMAL,
    modeReason: '',
    sources: [{ kind: 116, effect: RollMode.ADVANTAGE, textPt: 'Inspiração' }],
    criticalOnHit: false,
    d20: { diceCount: 2, diceSides: 20, faces: [4, 17], modifier: 5, total: 22, countedIndex: 1 },
    ...extra,
  },
  pending: undefined,
});

function setup(opts: {
  targets: unknown[];
  holders?: Record<string, string>;
  master?: boolean;
  api?: object;
}) {
  const state = new CombatState();
  state.apply(encounter({ currentCombatantId: 't', combatants: [toren, cap] } as never));
  const api = {
    rollAttack: vi.fn().mockResolvedValue(rolled()),
    requestRollMode: vi.fn(),
    cancelRollModeRequest: vi.fn(),
    rollDamage: vi.fn(),
    ...opts.api,
  };
  const data: AttackSheetData = {
    campaignId: 'c',
    encounterId: 'enc',
    attackerId: 't',
    round: 1,
    attack: sword,
    targets: opts.targets as never,
    diceMode: DiceMode.PLAYERS_CHOOSE,
    preference: DicePreference.APP,
    state,
    master: opts.master,
  };
  TestBed.resetTestingModule();
  TestBed.configureTestingModule({
    providers: [
      { provide: MAT_DIALOG_DATA, useValue: data },
      { provide: MatDialogRef, useValue: { close: () => undefined } },
      { provide: CombatClient, useValue: fakeCombatClient(api) },
      { provide: FavorClient, useValue: new FakeFavorClient().asClient() },
    ],
  });
  openFavor(TestBed.inject(FavorState), opts.holders ?? { 'char-t': 'Toren' });
  const fixture = TestBed.createComponent(AttackSheet);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const button = (name: string) =>
    Array.from(el.querySelectorAll('button')).find((b) => plain(b.textContent).includes(name));
  const chooseTarget = () => {
    el.querySelector<HTMLInputElement>('.target input')!.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Enter' }),
    );
    fixture.detectChanges();
  };
  const box = () => el.querySelector<HTMLInputElement>('app-favor-spend input[type=checkbox]');
  const tick = () => {
    box()!.click();
    fixture.detectChanges();
  };
  const settle = async () => {
    await fixture.whenStable();
    fixture.detectChanges();
  };
  return { fixture, el, api, button, chooseTarget, box, tick, settle };
}

describe('AttackSheet: the Inspiração (581-03)', () => {
  beforeEach(() => {
    TestBed.resetTestingModule();
    Element.prototype.scrollTo = vi.fn();
  });

  it('shows no box when the character has none', () => {
    const s = setup({ targets: [target()], holders: {} });
    s.chooseTarget();
    expect(s.el.querySelector('app-favor-spend .favor')).toBeNull();
  });

  it('puts the box above "Como rolar o d20"', () => {
    const s = setup({ targets: [target()] });
    s.chooseTarget();
    const html = s.el.innerHTML;
    expect(html.indexOf('Inspiração do mestre')).toBeGreaterThan(-1);
    expect(html.indexOf('Inspiração do mestre')).toBeLessThan(html.indexOf('Como rolar o d20'));
  });

  /** The player picked another mode before ticking: the request would carry it, if the Inspiração were not spending. */
  function pickDisadvantage(s: ReturnType<typeof setup>) {
    const radio = Array.from(s.el.querySelectorAll<HTMLLabelElement>('.radio')).find((l) =>
      l.textContent!.includes('Desvantagem'),
    )!;
    radio.querySelector('input')!.click();
    s.fixture.detectChanges();
    const reason = s.el.querySelector<HTMLInputElement>('.reason__field')!;
    reason.value = 'estou ferido';
    reason.dispatchEvent(new Event('input'));
    s.fixture.detectChanges();
  }

  it('sends the picked mode when unticked, and spend_favor with no roll mode when ticked', async () => {
    const off = setup({ targets: [target()] });
    off.chooseTarget();
    pickDisadvantage(off);
    off.button('Rolar 2d20 no app')!.click();
    await off.settle();
    expect(off.api.rollAttack.mock.calls[0][9]).toMatchObject({ mode: RollMode.DISADVANTAGE });
    expect(off.api.rollAttack.mock.calls[0][11].spendFavor).toBeUndefined();

    const s = setup({ targets: [target()] });
    s.chooseTarget();
    pickDisadvantage(s);
    s.tick();
    const text = plain(s.el.querySelector('app-roll-mode-picker')?.textContent);
    expect(text).toContain('pela Inspiração');
    expect(text).toContain('Desmarque a caixa');
    const radios = Array.from(
      s.el.querySelectorAll<HTMLInputElement>('app-roll-mode-picker input'),
    );
    expect(radios.every((r) => r.disabled)).toBe(true);
    s.button('Rolar 2d20 no app')!.click();
    await s.settle();
    const args = s.api.rollAttack.mock.calls[0];
    expect(args[9]).toBeUndefined();
    expect(args[11].spendFavor).toBe(true);
  });

  it('unticks the box after the spend and reads who has the Inspiração again', async () => {
    const s = setup({ targets: [target()] });
    const refresh = vi.spyOn(TestBed.inject(FavorState), 'refresh').mockResolvedValue();
    s.chooseTarget();
    s.tick();
    s.button('Rolar 2d20 no app')!.click();
    await s.settle();
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(s.box()?.checked ?? false).toBe(false);
  });

  it('reads Normal under a disadvantage and says they cancel', () => {
    const s = setup({
      targets: [
        target({
          rollMode: RollMode.DISADVANTAGE,
          sources: [{ kind: 1, effect: RollMode.DISADVANTAGE, textPt: 'Envenenada: desvantagem' }],
        }),
      ],
    });
    s.chooseTarget();
    s.tick();
    expect(plain(s.el.textContent)).toContain(
      'Desvantagem (Envenenada) e Inspiração se anulam: um d20. Desmarque a caixa para voltar a desvantagem.',
    );
    expect(s.button('Rolar no app')).toBeTruthy();
  });

  it('sends neither the spend nor a changed request when unticked', async () => {
    const s = setup({ targets: [target()] });
    s.chooseTarget();
    s.button('Rolar no app')!.click();
    await s.settle();
    const args = s.api.rollAttack.mock.calls[0];
    expect(args[11].spendFavor).toBeUndefined();
    expect(plain(s.el.textContent)).not.toContain('pela Inspiração');
  });

  it('is dashed with the reason when the target already gives advantage', () => {
    const s = setup({
      targets: [
        target({
          rollMode: RollMode.ADVANTAGE,
          sources: [{ kind: 1, effect: RollMode.ADVANTAGE, textPt: 'Ajuda de Tor: vantagem' }],
        }),
      ],
    });
    s.chooseTarget();
    expect(s.box()!.getAttribute('aria-disabled')).toBe('true');
    expect(plain(s.el.querySelector('.favor__why')?.textContent)).toContain(
      'ela já tem vantagem (Ajuda de Tor)',
    );
  });

  it('is dashed when the master attacks for the player', () => {
    const s = setup({ targets: [target()], master: true });
    s.chooseTarget();
    expect(plain(s.el.querySelector('app-favor-spend')?.textContent)).toContain(
      'O mestre agindo por Toren',
    );
    expect(plain(s.el.querySelector('.favor__why')?.textContent)).toContain(
      'Só quem joga com este personagem pode usar a Inspiração dele.',
    );
  });

  it('says the refusal in the error area and unticks the box', async () => {
    const s = setup({
      targets: [target()],
      api: {
        rollAttack: vi.fn().mockRejectedValue(favorRefused(FavorBlockedReason.ADDS_NOTHING)),
      },
    });
    s.chooseTarget();
    s.tick();
    s.button('Rolar 2d20 no app')!.click();
    await s.settle();
    expect(plain(s.el.querySelector('[role=alert]')?.textContent)).toContain(
      'A Inspiração não mudaria esta rolagem.',
    );
    expect(s.box()!.checked).toBe(false);
  });

  it('shows "Vantagem: Inspiração" under the result after the roll', async () => {
    const s = setup({ targets: [target()] });
    s.chooseTarget();
    s.tick();
    s.button('Rolar 2d20 no app')!.click();
    await s.settle();
    expect(plain(s.el.querySelector('app-attack-result')?.textContent)).toContain(
      'Vantagem: Inspiração',
    );
    expect(s.api.rollAttack).toHaveBeenCalledTimes(1);
  });
});
