import { Injectable, inject } from '@angular/core';
import { Code, ConnectError, createClient } from '@connectrpc/connect';

import {
  CampaignService,
  CriticalRule,
  DeathSaveVisibility,
  DiceMode,
  HitPointsRule,
  TableStyle,
  XpMode,
  XpModeChangeBlockedSchema,
  type GetTableRulesResponse,
  type SetCampaignXpModeResponse,
  type TableStylePreset,
} from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { describeConnectError } from '../connect/connect-errors';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** The rules the page edits (`TableRules`), as plain values so a draft is a copy. */
export interface RulesDraft {
  readonly diceMode: DiceMode;
  readonly combatStartsWithMap: boolean;
  readonly fogOnNewMaps: boolean;
  readonly hitPoints: HitPointsRule;
  readonly standardArray: boolean;
  readonly pointBuy: boolean;
  readonly rolled4d6: boolean;
  readonly typed: boolean;
  readonly critical: CriticalRule;
  readonly deathSaves: DeathSaveVisibility;
  readonly houseRules: readonly string[];
}

/** What the server says about the rules and the numbers the methods use. */
export interface TableRulesVm {
  readonly saved: RulesDraft;
  readonly style: TableStyle;
  readonly presets: readonly TableStylePreset[];
  readonly standardArray: readonly number[];
  readonly pointBuyCosts: readonly number[];
  readonly pointBuyMinScore: number;
  readonly pointBuyBudget: number;
  readonly typedMin: number;
  readonly typedMax: number;
}

/** The longest house rule, and how many (`TableRules.house_rules`). */
export const HOUSE_RULE_MAX_LENGTH = 200;
export const HOUSE_RULE_MAX_COUNT = 20;

export function draftFromRules(rules: GetTableRulesResponse['rules']): RulesDraft {
  return {
    diceMode: rules?.diceMode ?? DiceMode.PLAYERS_CHOOSE,
    combatStartsWithMap: rules?.combatStartsWithMap ?? true,
    fogOnNewMaps: rules?.fogOnNewMaps ?? false,
    hitPoints: rules?.hitPoints ?? HitPointsRule.PLAYER_CHOOSES,
    standardArray: rules?.abilityMethods?.standardArray ?? true,
    pointBuy: rules?.abilityMethods?.pointBuy ?? true,
    rolled4d6: rules?.abilityMethods?.rolled4d6 ?? true,
    typed: rules?.abilityMethods?.typed ?? true,
    critical: rules?.critical ?? CriticalRule.DOUBLED_DICE,
    deathSaves: rules?.deathSaves ?? DeathSaveVisibility.VISIBLE_TO_ALL,
    houseRules: rules?.houseRules ?? [],
  };
}

export function vmFromResponse(res: GetTableRulesResponse): TableRulesVm {
  return {
    saved: draftFromRules(res.rules),
    style: res.style,
    presets: res.presets,
    standardArray: res.standardArray,
    pointBuyCosts: res.pointBuyCosts,
    pointBuyMinScore: res.pointBuyMinScore,
    pointBuyBudget: res.pointBuyBudget,
    typedMin: res.typedMinScore,
    typedMax: res.typedMaxScore,
  };
}

/** The style a draft makes: the preset whose three settings it matches, or "Personalizado" (the server's rule, applied to what is on screen before it is saved). */
export function styleOf(draft: RulesDraft, presets: readonly TableStylePreset[]): TableStyle {
  const hit = presets.find(
    (p) =>
      p.diceMode === draft.diceMode &&
      p.combatStartsWithMap === draft.combatStartsWithMap &&
      p.fogOnNewMaps === draft.fogOnNewMaps,
  );
  return hit?.style ?? TableStyle.PERSONALIZADO;
}

/** A preset copied into a draft: only the three values it fills. */
export function applyPreset(draft: RulesDraft, preset: TableStylePreset): RulesDraft {
  return {
    ...draft,
    diceMode: preset.diceMode,
    combatStartsWithMap: preset.combatStartsWithMap,
    fogOnNewMaps: preset.fogOnNewMaps,
  };
}

/** How many choices differ from what is saved: "2 mudanças não salvas". A house rule counts as one each. */
export function changeCount(draft: RulesDraft, saved: RulesDraft): number {
  let n = 0;
  const scalar: (keyof RulesDraft)[] = [
    'diceMode',
    'combatStartsWithMap',
    'fogOnNewMaps',
    'hitPoints',
    'standardArray',
    'pointBuy',
    'rolled4d6',
    'typed',
    'critical',
    'deathSaves',
  ];
  for (const key of scalar) {
    if (draft[key] !== saved[key]) {
      n++;
    }
  }
  const a = draft.houseRules;
  const b = saved.houseRules;
  for (let i = 0; i < Math.max(a.length, b.length); i++) {
    if (a[i] !== b[i]) {
      n++;
    }
  }
  return n;
}

/** Why the draft cannot be saved yet, or `''`. The server checks the same limits. */
export function draftProblem(draft: RulesDraft): string {
  if (!draft.standardArray && !draft.pointBuy && !draft.rolled4d6 && !draft.typed) {
    return 'Marque pelo menos um jeito de fazer os atributos.';
  }
  if (draft.houseRules.some((r) => r.trim() === '')) {
    return 'Escreva o lembrete ou remova a linha vazia.';
  }
  return '';
}

/** The words of each choice, as the page and the "Mudou" tags say them. */
export const DICE_WORDS: Readonly<Record<number, string>> = {
  [DiceMode.APP]: 'todos rolam no app',
  [DiceMode.PHYSICAL]: 'todos rolam os próprios dados',
  [DiceMode.PLAYERS_CHOOSE]: 'cada jogador escolhe',
};

export const STYLE_LABELS: Readonly<Record<number, string>> = {
  [TableStyle.TUDO_NO_APP]: 'Tudo no app',
  [TableStyle.MESA_FISICA]: 'Mesa física',
  [TableStyle.TEATRO_DA_MENTE]: 'Teatro da mente',
  [TableStyle.PERSONALIZADO]: 'Personalizado',
};

export const XP_MODE_WORDS: Readonly<Record<number, string>> = {
  [XpMode.ENEMIES]: 'por inimigos',
  [XpMode.MILESTONES]: 'por marcos',
  [XpMode.GOLD]: 'por ouro',
};

/** What a blocked XP mode change says: how many awards stand and how much XP they gave. */
export interface XpModeBlocked {
  readonly awards: number;
  readonly totalXp: number;
}

/** The `XpModeChangeBlocked` detail of a refused `SetCampaignXpMode`, or `null`. By the typed detail, never the message. */
export function xpModeBlocked(err: unknown): XpModeBlocked | null {
  const e = ConnectError.from(err, Code.Unavailable);
  if (e.code !== Code.FailedPrecondition) {
    return null;
  }
  const d = e.findDetails(XpModeChangeBlockedSchema)[0];
  return d ? { awards: d.awards, totalXp: Number(d.totalXp) } : null;
}

/** The Portuguese message of a failed rules call. */
export function tableRulesError(err: unknown, what: string): string {
  return describeConnectError(err, {
    [Code.InvalidArgument]: `Não deu para ${what}: confira as escolhas e os lembretes (até 20, de 1 a 200 caracteres, numa linha).`,
    [Code.NotFound]: 'Essa campanha não existe mais, ou você não é membro dela.',
    [Code.PermissionDenied]: 'Só o mestre da campanha muda as regras da mesa.',
    [Code.Unavailable]: 'Não foi possível falar com o servidor agora. Tente de novo em instantes.',
  });
}

/**
 * The master's "Regras da mesa" calls (MR-025, RN-24, RN-09) on the generated `CampaignService`. `providedIn: 'root'`,
 * imported only by lazy code. Nothing here knows a rule: the numbers of the methods and the presets come from the server.
 */
@Injectable({ providedIn: 'root' })
export class TableRulesClient {
  private readonly client = createClient(CampaignService, inject(CONNECT_TRANSPORT));

  async get(campaignId: string): Promise<TableRulesVm> {
    return vmFromResponse(await this.client.getTableRules({ campaignId }));
  }

  /** The whole draft at once, with the dice mode; answers with what is now in force. */
  async set(campaignId: string, d: RulesDraft): Promise<{ saved: RulesDraft; style: TableStyle }> {
    const res = await this.client.setTableRules({
      campaignId,
      rules: {
        diceMode: d.diceMode,
        combatStartsWithMap: d.combatStartsWithMap,
        fogOnNewMaps: d.fogOnNewMaps,
        hitPoints: d.hitPoints,
        abilityMethods: {
          standardArray: d.standardArray,
          pointBuy: d.pointBuy,
          rolled4d6: d.rolled4d6,
          typed: d.typed,
        },
        critical: d.critical,
        deathSaves: d.deathSaves,
        houseRules: [...d.houseRules],
      },
    });
    return { saved: draftFromRules(res.rules), style: res.style };
  }

  setXpMode(campaignId: string, xpMode: XpMode, confirm: boolean): Promise<SetCampaignXpModeResponse> {
    return this.client.setCampaignXpMode({ campaignId, xpMode, confirm });
  }
}
