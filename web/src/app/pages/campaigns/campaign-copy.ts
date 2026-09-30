import { Campaign, Role, XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { roleLabel } from '../../core/campaigns/campaign-labels';

/**
 * The campaign's XP mode as a phrase that stands on its own ("XP por
 * inimigos derrotados"), for the second line of a campaign on "Minhas
 * campanhas" and the lead of the campaign page. `null` when the mode is not
 * set: a pending member's campaign comes without it (campaigns.proto,
 * `Campaign.awaiting_approval`).
 */
export function xpModeSentence(xpMode: XpMode): string | null {
  switch (xpMode) {
    case XpMode.ENEMIES:
      return 'XP por inimigos derrotados';
    case XpMode.GOLD:
      return 'XP por ouro';
    case XpMode.MILESTONES:
      return 'XP por marcos';
    default:
      return null;
  }
}

/** The caller's role as a tag on "Minhas campanhas": one word, capitalised. */
export function roleTag(role: Role): string {
  return role === Role.MASTER ? 'Mestre' : 'Jogador';
}

/**
 * The line under the campaign's name: "Você é mestre nesta campanha. XP por
 * inimigos derrotados." The first sentence is what the e2e tests look for
 * (`campaigns.spec.ts`), so it keeps the role word lowercase.
 */
export function campaignLead(campaign: Pick<Campaign, 'myRole' | 'xpMode'>): string {
  const role = `Você é ${roleLabel(campaign.myRole)} nesta campanha.`;
  const xp = xpModeSentence(campaign.xpMode);
  return xp ? `${role} ${xp}.` : role;
}
