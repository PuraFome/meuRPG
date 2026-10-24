import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';
import { create } from '@bufbuild/protobuf';
import { of } from 'rxjs';

import {
  CriticalRule,
  DeathSaveVisibility,
  DiceMode,
  EnemyReactionsRule,
  HiddenAreaHitRule,
  HitPointsRule,
  Role,
  TableStyle,
  TableStylePresetSchema,
  XpMode,
} from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CampaignsService } from '../../core/campaigns/campaigns.service';
import {
  type RulesDraft,
  TableRulesClient,
  type TableRulesVm,
} from '../../core/campaigns/table-rules';
import { settle } from '../../../test-settle';
import { TableRulesPage } from './table-rules';

const LABEL = 'Os jogadores veem a CD dos testes de resistência';

const saved: RulesDraft = {
  diceMode: DiceMode.PLAYERS_CHOOSE,
  combatStartsWithMap: true,
  fogOnNewMaps: true,
  hitPoints: HitPointsRule.PLAYER_CHOOSES,
  standardArray: true,
  pointBuy: true,
  rolled4d6: true,
  typed: true,
  critical: CriticalRule.DOUBLED_DICE,
  deathSaves: DeathSaveVisibility.VISIBLE_TO_ALL,
  hiddenAreaHits: HiddenAreaHitRule.ASK,
  featsAllowed: true,
  enemyReactions: EnemyReactionsRule.ALWAYS,
  showSaveDc: false,
  houseRules: ['Beber uma poção é uma ação bônus'],
};

function vm(over: Partial<RulesDraft>): TableRulesVm {
  return {
    saved: { ...saved, ...over },
    style: TableStyle.PERSONALIZADO,
    presets: [
      create(TableStylePresetSchema, {
        style: TableStyle.TUDO_NO_APP,
        diceMode: DiceMode.APP,
        combatStartsWithMap: true,
        fogOnNewMaps: true,
      }),
    ],
    standardArray: [15, 14, 13, 12, 10, 8],
    pointBuyCosts: [0, 1, 2, 3, 4, 5, 7, 9],
    pointBuyMinScore: 8,
    pointBuyBudget: 27,
    typedMin: 3,
    typedMax: 18,
  };
}

describe('TableRulesPage, the save DC rule', () => {
  const set = vi.fn();

  async function setup(role: Role = Role.MASTER, over: Partial<RulesDraft> = {}) {
    set.mockReset().mockImplementation(async (_id: string, d: RulesDraft) => ({
      saved: d,
      style: TableStyle.PERSONALIZADO,
    }));
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        {
          provide: ActivatedRoute,
          useValue: { paramMap: of(convertToParamMap({ id: 'camp-1' })) },
        },
        {
          provide: CampaignsService,
          useValue: {
            getCampaign: vi.fn().mockResolvedValue({
              campaign: {
                id: 'camp-1',
                name: 'Mirathel',
                myRole: role,
                awaitingApproval: false,
                xpMode: XpMode.ENEMIES,
              },
            }),
          },
        },
        {
          provide: TableRulesClient,
          useValue: { get: vi.fn().mockResolvedValue(vm(over)), set, setXpMode: vi.fn() },
        },
      ],
    });
    const fixture = TestBed.createComponent(TableRulesPage);
    fixture.detectChanges();
    await settle(fixture, 3);
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  const text = (el: HTMLElement) => (el.textContent ?? '').replace(/\s+/g, ' ');
  const byId = (el: HTMLElement, id: string | null) =>
    text(el.querySelector<HTMLElement>(`#${id ?? 'none'}`) ?? document.createElement('i'));
  const dcSwitch = (el: HTMLElement) =>
    Array.from(el.querySelectorAll<HTMLButtonElement>('button[role="switch"]')).find((b) =>
      byId(el, b.getAttribute('aria-labelledby')).includes(LABEL),
    ) ?? null;
  const save = (el: HTMLElement) =>
    Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
      b.textContent?.includes('Salvar regras'),
    )!;

  it('draws the switch off, named and described, beside the fog switch that still works', async () => {
    const { el } = await setup();
    const sw = dcSwitch(el)!;
    expect(sw).not.toBeNull();
    expect(sw.getAttribute('aria-checked')).toBe('false');
    const hint = byId(el, sw.getAttribute('aria-describedby'));
    expect(hint).toContain('só o mestre vê a CD');
    expect(hint).toContain('Quem conjurou vê a CD da própria magia');
    // The fog switch and this one are two controls with their own names.
    const switches = el.querySelectorAll('button[role="switch"]');
    expect(switches).toHaveLength(2);
    expect(new Set(Array.from(switches).map((b) => b.getAttribute('aria-labelledby'))).size).toBe(
      2,
    );
  });

  it('draws it on when the table shows the DC', async () => {
    const { el } = await setup(Role.MASTER, { showSaveDc: true });
    expect(dcSwitch(el)!.getAttribute('aria-checked')).toBe('true');
  });

  it('toggles, counts one unsaved change and sends the whole rules with showSaveDc', async () => {
    const { fixture, el } = await setup();
    dcSwitch(el)!.click();
    await settle(fixture, 3);
    expect(dcSwitch(el)!.getAttribute('aria-checked')).toBe('true');
    expect(text(el)).toContain('1 mudança não salva');
    save(el).click();
    await settle(fixture, 3);
    expect(set).toHaveBeenCalledTimes(1);
    expect(set.mock.calls[0][1]).toEqual({ ...saved, showSaveDc: true });
    expect(text(el)).toContain('Regras salvas.');
  });

  it('turns it off again without a change left to save', async () => {
    const { fixture, el } = await setup(Role.MASTER, { showSaveDc: true });
    dcSwitch(el)!.click();
    await settle(fixture, 3);
    expect(dcSwitch(el)!.getAttribute('aria-checked')).toBe('false');
    expect(text(el)).toContain('1 mudança não salva');
    dcSwitch(el)!.click();
    await settle(fixture, 3);
    expect(text(el)).not.toContain('mudança não salva');
  });

  it.each([
    [false, 'Só o mestre vê'],
    [true, 'Os jogadores veem'],
  ])('tells a player, read-only, who sees the DC (%s)', async (showSaveDc, words) => {
    const { el } = await setup(Role.PLAYER, { showSaveDc });
    const rows = Array.from(el.querySelectorAll('.read__row')).map(
      (r) => `${r.querySelector('dt')?.textContent} ${r.querySelector('dd')?.textContent}`,
    );
    expect(rows).toContain(`CD dos testes de resistência ${words}`);
    expect(dcSwitch(el)).toBeNull();
  });
});
