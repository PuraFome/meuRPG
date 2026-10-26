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

const LABEL = 'Trocar uma escolha no aumento de atributo';

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
  pickSwapAllowed: false,
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

describe('TableRulesPage, the pick swap rule', () => {
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
  const swapSwitch = (el: HTMLElement) =>
    Array.from(el.querySelectorAll<HTMLButtonElement>('button[role="switch"]')).find((b) =>
      byId(el, b.getAttribute('aria-labelledby')).includes(LABEL),
    ) ?? null;
  const save = (el: HTMLElement) =>
    Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
      b.textContent?.includes('Salvar regras'),
    )!;

  it('draws the switch off, named and described', async () => {
    const { el } = await setup();
    const sw = swapSwitch(el)!;
    expect(sw).not.toBeNull();
    expect(sw.getAttribute('aria-checked')).toBe('false');
    expect(byId(el, sw.getAttribute('aria-describedby'))).toContain(
      'também pode trocar uma escolha anterior',
    );
  });

  it('toggles, counts one unsaved change and sends the whole rules with pickSwapAllowed', async () => {
    const { fixture, el } = await setup();
    swapSwitch(el)!.click();
    await settle(fixture, 3);
    expect(swapSwitch(el)!.getAttribute('aria-checked')).toBe('true');
    expect(text(el)).toContain('1 mudança não salva');
    save(el).click();
    await settle(fixture, 3);
    expect(set.mock.calls[0][1]).toEqual({ ...saved, pickSwapAllowed: true });
  });

  it.each([
    [false, 'Não usada'],
    [true, 'Permitida'],
  ])(
    'tells a player, read-only, whether a pick may be swapped (%s)',
    async (pickSwapAllowed, words) => {
      const { el } = await setup(Role.PLAYER, { pickSwapAllowed });
      const rows = Array.from(el.querySelectorAll('.read__row')).map(
        (r) => `${r.querySelector('dt')?.textContent} ${r.querySelector('dd')?.textContent}`,
      );
      expect(rows).toContain(`Troca de escolha no aumento de atributo ${words}`);
    },
  );
});
