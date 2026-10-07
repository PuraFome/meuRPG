import { Code, ConnectError } from '@connectrpc/connect';

import {
  EncounterBand,
  EncounterBuildBlockedReason,
  EncounterBuildBlockedSchema,
  type EncounterEvaluation,
  type EncounterPartyMember,
  EncounterWarning,
} from '../../../gen/meurpg/play/v1/encounters_pb';
import { describeConnectError } from '../connect/connect-errors';
import { formatInt, tight } from '../format/text';

/** The label every difficulty number carries (RN-29, question 77): the table is the 2024 guide's, not the SRD 5.1's. */
export const GUIDE_LABEL = 'Guia de dificuldade do SRD 5.2.1 (regras de 2024)';
/** The honest caveat under it: the 2024 guide measures 2024 monsters, and the app's are the 2014 ones. */
export const GUIDE_CAVEAT =
  'Com os monstros de 2014, o encontro tende a ficar um pouco mais fácil.';

/** The word of a band; never "mortal" (RN-29): above the high budget is "Acima de alta". */
export function bandWord(band: EncounterBand): string {
  switch (band) {
    case EncounterBand.LOW:
      return 'Baixa';
    case EncounterBand.MODERATE:
      return 'Moderada';
    case EncounterBand.HIGH:
      return 'Alta';
    case EncounterBand.ABOVE_HIGH:
      return 'Acima de alta';
    default:
      return '';
  }
}

/** The budget the band's headline is measured against: the band's own (the high one past it). */
function bandBudget(ev: EncounterEvaluation): number {
  const b = ev.budget;
  switch (ev.band) {
    case EncounterBand.LOW:
      return b?.low ?? 0;
    case EncounterBand.MODERATE:
      return b?.moderate ?? 0;
    default:
      return b?.high ?? 0;
  }
}

/** "Moderada · 1.550 de 1.875 XP", or "Acima de alta: 2.900 de 2.600 XP". Every number is the server's. */
export function headline(ev: EncounterEvaluation): string {
  const numbers = tight(`${formatInt(ev.totalXp)} de ${formatInt(bandBudget(ev))} XP`);
  return ev.band === EncounterBand.ABOVE_HIGH
    ? `Acima de alta: ${numbers}`
    : `${bandWord(ev.band)} · ${numbers}`;
}

/** "até 1.250 XP" / "mais de 2.600 XP": the phone's list of bands. */
export function bandLimit(ev: EncounterEvaluation, band: EncounterBand): string {
  const b = ev.budget;
  switch (band) {
    case EncounterBand.LOW:
      return tight(`até ${formatInt(b?.low ?? 0)} XP`);
    case EncounterBand.MODERATE:
      return tight(`até ${formatInt(b?.moderate ?? 0)} XP`);
    case EncounterBand.HIGH:
      return tight(`até ${formatInt(b?.high ?? 0)} XP`);
    default:
      return tight(`mais de ${formatInt(b?.high ?? 0)} XP`);
  }
}

export interface BarGeometry {
  /** Where each budget's tick sits and how much of the bar the total fills, in percent of the bar. */
  readonly low: number;
  readonly moderate: number;
  readonly high: number;
  readonly fill: number;
  /** The total passes the high budget: the marker stays at the tip, with "›". */
  readonly over: boolean;
}

/**
 * Where things sit on the bar. This is drawing, not rules: the three budgets and the total are the server's;
 * the scale is just wide enough to hold the high budget and a little past it, and the total when it is bigger.
 */
export function barGeometry(ev: EncounterEvaluation): BarGeometry {
  const { low = 0, moderate = 0, high = 0 } = ev.budget ?? {};
  const over = ev.band === EncounterBand.ABOVE_HIGH;
  const scale = Math.max(high * 1.13, 1);
  const at = (xp: number) => Math.min(100, (xp / scale) * 100);
  return {
    low: at(low),
    moderate: at(moderate),
    high: at(high),
    fill: over ? 100 : at(ev.totalXp),
    over,
  };
}

const COUNT_WORDS = [
  '',
  'um',
  'dois',
  'três',
  'quatro',
  'cinco',
  'seis',
  'sete',
  'oito',
  'nove',
  'dez',
];

/** "três de nível 4 e um de nível 5": the party by level, for the generate sheet's lead. */
export function partyByLevel(party: readonly EncounterPartyMember[]): string {
  const counts = new Map<number, number>();
  for (const m of party) {
    counts.set(m.level, (counts.get(m.level) ?? 0) + 1);
  }
  const parts = [...counts.entries()]
    .sort((a, b) => a[0] - b[0])
    .map(([level, n]) => `${COUNT_WORDS[n] ?? n} de nível ${level}`);
  return parts.length < 2
    ? (parts[0] ?? '')
    : `${parts.slice(0, -1).join(', ')} e ${parts[parts.length - 1]}`;
}

/** A warning as the builder says it, with the numbers it needs from the evaluation. */
export interface WarningLine {
  readonly kind: EncounterWarning;
  readonly title: string;
  readonly text: string;
}

/** The warnings of an evaluation, in words. None of them stops the master (encounters.proto). */
export function warningLines(ev: EncounterEvaluation): WarningLine[] {
  const out: WarningLine[] = [];
  for (const kind of ev.warnings) {
    switch (kind) {
      case EncounterWarning.ABOVE_HIGH:
        out.push({
          kind,
          title: 'Passa do orçamento de alta.',
          text: tight(
            `${formatInt(ev.totalXp)} XP contra ${formatInt(ev.budget?.high ?? 0)} XP: o encontro fica bem acima do que o grupo aguenta. Dá para guardar e jogar assim.`,
          ),
        });
        break;
      case EncounterWarning.ABOVE_CR_CAP: {
        const names = ev.lines.filter((l) => l.aboveCap).map((l) => l.creature?.namePt ?? '');
        out.push({
          kind,
          title: 'Criatura acima do ND máximo.',
          text: `${names.join(', ')} passa${names.length > 1 ? 'm' : ''} do ND ${ev.maxCr}, o limite do grupo. Dá para manter.`,
        });
        break;
      }
      case EncounterWarning.NO_PARTY:
        out.push({
          kind,
          title: 'O grupo está vazio.',
          text: 'Nenhum personagem de jogador vivo na campanha, então não há orçamento. Ponha um NPC no grupo para medir o encontro.',
        });
        break;
      case EncounterWarning.TOO_MANY:
        out.push({
          kind,
          title: 'Criaturas demais para um combate.',
          text: 'Com o grupo e as criaturas dele, passam de 40 combatentes: “Começar este combate” não aceitaria.',
        });
        break;
      case EncounterWarning.UNKNOWN_CREATURE:
        out.push({
          kind,
          title: 'Uma criatura deste encontro não está mais no SRD.',
          text: 'Ela ficou fora da conta. Tire-a do encontro antes de guardar de novo.',
        });
        break;
    }
  }
  return out;
}

/** "A criatura mais forte pode ter ND 7: o menor nível do grupo (4) mais 3."; empty for an empty party. */
export function capLine(ev: EncounterEvaluation): string {
  return ev.lowestLevel > 0
    ? tight(
        `A criatura mais forte pode ter ND ${ev.maxCr}: o menor nível do grupo (${ev.lowestLevel}) mais 3.`,
      )
    : '';
}

/** What the builder was doing when a call failed, to finish "Não deu para …". */
export type EncounterAction = 'evaluate' | 'generate' | 'swap' | 'save' | 'read' | 'clear';

const WHAT: Record<EncounterAction, string> = {
  evaluate: 'medir o encontro',
  generate: 'gerar o encontro',
  swap: 'listar as trocas',
  save: 'guardar o encontro',
  read: 'ler o encontro guardado',
  clear: 'tirar o encontro do ponto',
};

/** The reason of an `EncounterBuildBlocked` detail (a `failed_precondition`), or `null`. */
export function buildBlocked(err: unknown): EncounterBuildBlockedReason | null {
  const e = ConnectError.from(err, Code.Unavailable);
  return e.code === Code.FailedPrecondition
    ? (e.findDetails(EncounterBuildBlockedSchema)[0]?.reason ?? null)
    : null;
}

/** The Portuguese message for a refused call of the builder, by code and typed detail, never by the message. */
export function encounterErrorMessage(err: unknown, action: EncounterAction): string {
  switch (buildBlocked(err)) {
    case EncounterBuildBlockedReason.NO_PARTY:
      return 'O grupo está vazio: não há personagem de jogador vivo. Ponha um NPC no grupo e tente de novo.';
    case EncounterBuildBlockedReason.NOTHING_FITS:
      return 'Nenhuma criatura desse tipo cabe nessa dificuldade. Escolha outro tipo ou outra dificuldade.';
    case EncounterBuildBlockedReason.UNKNOWN_CREATURE:
      return 'Uma criatura deste encontro não está mais no SRD. Tire-a da lista e tente de novo.';
  }
  return describeConnectError(err, {
    [Code.InvalidArgument]: `Não deu para ${WHAT[action]}: confira as quantidades (de 1 a 40), o grupo (até 10 NPCs, do nível 1 ao 20) e tente de novo.`,
    [Code.NotFound]:
      action === 'save' || action === 'read' || action === 'clear'
        ? 'Esse ponto de batalha não existe mais, ou você não é o mestre da campanha. Escolha outro.'
        : 'Essa campanha não existe, ou você não é o mestre dela. Volte para Minhas campanhas.',
    [Code.PermissionDenied]: 'Só o mestre da campanha monta encontros.',
    [Code.Unavailable]: `Não deu para ${WHAT[action]}: o servidor não respondeu. Tente de novo.`,
  });
}
