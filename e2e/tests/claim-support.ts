import { expect, type Page } from '@playwright/test';

import { callRPC, characterRpcBody, pensantus, type CharacterBuild } from './support';

// What the specs of the reserved characters and their claim links share (MR-049).

const CHARACTERS = 'meurpg.characters.v1.CharacterService';

/** A reserved Rock Gnome Wizard, made as the master through the API (the screens of its creation have a spec of their own). */
export async function reservedRPC(master: Page, campaignId: string, name: string, build: CharacterBuild = pensantus): Promise<string> {
  const res = await callRPC(master, `${CHARACTERS}/CreateCharacter`, {
    campaignId,
    ...characterRpcBody('PLAYER', { ...build, name }),
    forPlayer: true,
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()).character.id as string;
}

/** Makes the character's claim link as the master and returns the token (shown once). */
export async function linkRPC(master: Page, campaignId: string, characterId: string, validityDays = 7): Promise<string> {
  const res = await callRPC(master, `${CHARACTERS}/CreateClaimLink`, { campaignId, characterId, validityDays });
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()).token as string;
}

export async function revokeLinkRPC(master: Page, campaignId: string, characterId: string): Promise<void> {
  const res = await callRPC(master, `${CHARACTERS}/RevokeClaimLink`, { campaignId, characterId });
  expect(res.ok(), await res.text()).toBeTruthy();
}

/** The path a claim link opens: the secret sits after the `#`, which no server ever sees. */
export function claimRoute(token: string): string {
  return `/claim#t=${token}`;
}

/** The row of a reserved character on the master's campaign page. */
export function rowOf(master: Page, name: string) {
  return master.locator('app-reserved-characters li.row').filter({ has: master.getByText(name, { exact: true }) });
}
