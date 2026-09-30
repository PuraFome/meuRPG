import { expect, type Locator, type Page } from '@playwright/test';

import { callRPC, characterRpcBody, createCharacterRPC, pensantus } from './support';

// Setup for the live-session specs (Etapa 5, MR-011, MR-012, RN-02), through
// the API: these tests prove the session screens, not the campaign, invite
// and character forms other specs already cover. Every test makes its own
// campaign, so sessions never collide with another test's.

export interface LiveTable {
  campaignId: string;
  campaignName: string;
  characterId: string;
}

/**
 * A campaign of Mestre Teste's with Jogador Teste in it, playing Pensantus
 * (23 HP, Mago 3). `masterPage` and `playerPage` must be signed in as each
 * (their saved states); neither navigates.
 */
export async function tableWithPensantus(masterPage: Page, playerPage: Page, name: string): Promise<LiveTable> {
  const created = await callRPC(masterPage, 'meurpg.campaigns.v1.CampaignService/CreateCampaign', {
    name,
    xpMode: 'XP_MODE_ENEMIES',
  });
  expect(created.ok()).toBeTruthy();
  const campaignId = (await created.json()).campaign.id as string;

  const invite = await callRPC(masterPage, 'meurpg.campaigns.v1.CampaignService/CreateInvite', {
    campaignId,
    maxUses: 1,
    expiresIn: '3600s',
  });
  expect(invite.ok()).toBeTruthy();
  const accepted = await callRPC(playerPage, 'meurpg.campaigns.v1.CampaignService/AcceptInvite', {
    token: (await invite.json()).token,
  });
  expect(accepted.ok()).toBeTruthy();

  const character = await createCharacterRPC(playerPage, campaignId, characterRpcBody('PLAYER', pensantus));
  expect(character.ok()).toBeTruthy();
  return { campaignId, campaignName: name, characterId: (await character.json()).character.id };
}

/** Starts the campaign's session through the API; returns its ID. */
export async function startSessionRPC(masterPage: Page, campaignId: string): Promise<string> {
  const res = await callRPC(masterPage, 'meurpg.play.v1.PlayService/StartGameSession', { campaignId });
  expect(res.ok()).toBeTruthy();
  return (await res.json()).gameSession.id as string;
}

/**
 * Ends whatever session the campaign has open, if any: tests clean up after
 * themselves, so Jogador Teste doesn't pile up open sessions (each one is a
 * notice and an "Ao vivo" link on every screen of the shared account).
 */
export async function endOpenSessionRPC(masterPage: Page, campaignId: string): Promise<void> {
  const res = await callRPC(masterPage, 'meurpg.play.v1.PlayService/ListGameSessions', { campaignId });
  if (!res.ok()) {
    return;
  }
  const sessions = ((await res.json()).gameSessions ?? []) as { id: string; endedAt?: string }[];
  for (const s of sessions.filter((s) => !s.endedAt)) {
    await endSessionRPC(masterPage, campaignId, s.id);
  }
}

/** Ends the campaign's session through the API. */
export async function endSessionRPC(masterPage: Page, campaignId: string, gameSessionId: string): Promise<void> {
  const res = await callRPC(masterPage, 'meurpg.play.v1.PlayService/EndGameSession', { campaignId, gameSessionId });
  expect(res.ok()).toBeTruthy();
}

/**
 * Opens a session page and waits for it to be live. Not `networkidle`: the
 * page keeps its live stream open, so the network is never idle. Up to 30 s:
 * going live takes a few calls in a row (the sign-in check, the stream's
 * `ready`, the snapshot), and on a busy shared stack each can take seconds.
 */
export async function openSessionPage(page: Page, campaignId: string): Promise<void> {
  await page.goto(`/campanhas/${campaignId}/sessao`);
  await expect(page.getByText('Ao vivo', { exact: true }).first()).toBeVisible({ timeout: 30_000 });
}

/**
 * Waits for the session notice of `campaignName` (RN-06) and returns it.
 *
 * Jogador Teste is in many campaigns other tests left with an open session,
 * and the notice shows the newest one this tab hasn't closed. So while
 * waiting, this closes any other campaign's notice ("Fechar aviso"), as a
 * person would, until ours shows. The app asks for open sessions every 30
 * seconds, so give it up to 45.
 */
export async function waitForNotice(page: Page, campaignName: string, timeoutMs = 45_000): Promise<Locator> {
  const ours = page.getByRole('status').filter({ hasText: `A sessão 1 de ${campaignName} começou.` });
  const close = page.getByRole('button', { name: 'Fechar aviso' });
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (await ours.isVisible()) {
      return ours;
    }
    if (await close.isVisible()) {
      await close.click();
      continue;
    }
    await page.waitForTimeout(250);
  }
  throw new Error(`the notice of "${campaignName}" did not show within ${timeoutMs / 1000} s`);
}
