import { CharacterState } from '../../../core/characters/characters.types';
import { CampaignCharacterListItemVm, ClaimVm } from './campaign-characters.types';

/**
 * The second line of a character's row: "Mago 3, de Vinicius" on the
 * master's lists, just "Mago 3" on a player's own. A player with no display
 * name reads "de um jogador sem nome" (never "Sem nome"). Empty when there
 * is nothing to say (a basic-sheet NPC has no class).
 */
export function characterRowSub(c: CampaignCharacterListItemVm, isMaster: boolean): string {
  const parts: string[] = [];
  if (c.classSummary) {
    parts.push(c.classSummary);
  }
  if (isMaster && c.kind === 'player') {
    // No-break spaces: the line never ends on an orphan word ("Mago 4, de um jogador sem" / "nome").
    parts.push(
      c.playerDisplayName
        ? `de ${c.playerDisplayName}`.replace(/ /g, '\u00a0')
        : 'de\u00a0um\u00a0jogador\u00a0sem\u00a0nome',
    );
  }
  return parts.join(', ');
}

/** The tag's tone for a character's state (docs/design.md#cor): pending in
 * the warning colour, dead in the danger colour, draft and locked neutral. */
export function stateTagClass(state: CharacterState): string {
  switch (state) {
    case 'pending':
      return 'mr-tag--pending';
    case 'dead':
      return 'mr-tag--danger';
    default:
      return '';
  }
}

/** "15/10". */
function dayMonth(d: Date): string {
  return `${String(d.getDate()).padStart(2, '0')}/${String(d.getMonth() + 1).padStart(2, '0')}`;
}

/** "16/10 às 19:05". */
export function dayMonthTime(d: Date): string {
  return `${dayMonth(d)} às ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
}

/**
 * What a reserved character's row says about its link (MR-049): "Sem link", "Link enviado · vale até 15/10",
 * "Link expirado em 15/10", "Link revogado", "Assumido por Lia". The words are the board's; "expirado" is the one
 * state the board does not draw (a link that ran out reads like none, with the date).
 */
export function claimStateLabel(claim: ClaimVm): string {
  switch (claim.state) {
    case 'sent':
      return `Link enviado${claim.expiresAt ? ` · vale até ${dayMonth(claim.expiresAt)}` : ''}`;
    case 'expired':
      return `Link expirado${claim.expiresAt ? ` em ${dayMonth(claim.expiresAt)}` : ''}`;
    case 'revoked':
      return 'Link revogado';
    case 'used':
      return claim.claimedBy ? `Assumido por ${claim.claimedBy}` : 'Assumido';
    default:
      return 'Sem link';
  }
}

/** The tag's tone for a claim state: sent in the warning colour (it waits for a player), used in the success colour. */
export function claimTagClass(state: ClaimVm['state']): string {
  switch (state) {
    case 'sent':
      return 'mr-tag--pending';
    case 'used':
      return 'mr-tag--success';
    default:
      return '';
  }
}

/** "Ladino 5 · Halfling": a reserved row's second line (the class, then the race). */
export function reservedRowSub(c: CampaignCharacterListItemVm): string {
  return [c.classSummary, c.raceName ?? ''].filter((p) => p !== '').join(' · ');
}
