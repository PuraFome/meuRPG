import { expect, type Page } from '@playwright/test';

import { startSessionRPC } from './live-session-support';
import { callRPC, characterRpcBody, createCharacterRPC, pensantus, type CharacterBuild } from './support';

// Setup for the class and race choices specs (PM-05), through the API: these tests prove the page that completes
// the choices a locked sheet left open, not the campaign, invite and editor ones other specs cover. Every test
// makes its own campaign.

export interface HalfElfTable {
  campaignId: string;
  characterId: string;
}

/** Pensantus as a half-elf: the race asks for two +1 abilities, which are picks of the sheet. */
export const halfElfPensantus: CharacterBuild = {
  ...pensantus,
  name: 'Pensantus',
  raceKey: 'race:half-elf',
  race: 'Meio-elfo',
  subraceKey: undefined,
  subrace: undefined,
};

/** The stored key of a half-elf's +1 pick ("race:half-elf#abilities=ability:dex"). */
const abilityPick = (ability: string) => `race:half-elf#abilities=ability:${ability}`;

/**
 * A campaign of Mestre Teste's with Jogador Teste in it, playing a half-elf whose sheet locked with the two +1 picks
 * still open: the player makes the sheet complete (a player is refused an incomplete one), the master takes the
 * picks out (the master is not refused), then the session starts and locks it. `masterPage` and `playerPage` must
 * be signed in as each; neither navigates. End the session with `endOpenSessionRPC` in a `finally`.
 */
export async function tableWithOpenChoices(masterPage: Page, playerPage: Page, name: string): Promise<HalfElfTable> {
  const created = await callRPC(masterPage, 'meurpg.campaigns.v1.CampaignService/CreateCampaign', {
    name,
    xpMode: 'XP_MODE_ENEMIES',
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const campaignId = (await created.json()).campaign.id as string;
  const invite = await callRPC(masterPage, 'meurpg.campaigns.v1.CampaignService/CreateInvite', { campaignId, maxUses: 1, expiresIn: '3600s' });
  expect(invite.ok()).toBeTruthy();
  const accepted = await callRPC(playerPage, 'meurpg.campaigns.v1.CampaignService/AcceptInvite', { token: (await invite.json()).token });
  expect(accepted.ok()).toBeTruthy();

  const body = characterRpcBody('PLAYER', halfElfPensantus) as { sheet: { full: object } };
  body.sheet.full = { ...body.sheet.full, featureChoiceKeys: [abilityPick('dex'), abilityPick('con')] };
  const made = await createCharacterRPC(playerPage, campaignId, body);
  expect(made.ok(), await made.text()).toBeTruthy();
  const character = (await made.json()).character as { id: string; name: string; revision: number; sheet: { full: Record<string, unknown> } };

  const sheet = { full: { ...character.sheet.full, featureChoiceKeys: [] } };
  const opened = await callRPC(masterPage, 'meurpg.characters.v1.CharacterService/UpdateCharacter', {
    campaignId,
    characterId: character.id,
    revision: character.revision,
    name: character.name,
    sheet,
  });
  expect(opened.ok(), await opened.text()).toBeTruthy();

  await startSessionRPC(masterPage, campaignId);
  return { campaignId, characterId: character.id };
}
