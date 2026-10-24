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
  await page.goto(`/campaigns/${campaignId}/session`);
  // The bar's "Ao vivo" pill leaves it under 360 px, so the title "Sessão 3" (there at every width) counts too.
  const live = page.getByText('Ao vivo', { exact: true }).or(page.getByRole('heading', { level: 1, name: /^Sessão \d+$/ }));
  await expect(live.first()).toBeVisible({ timeout: 30_000 });
}

/**
 * Waits for the notice of `campaignName`'s session 1 on the player's open
 * page and returns its "Entrar na sessão" link. Other tests start sessions
 * at the same time, so the page may show several notices (one per open
 * session, oldest first): the link's accessible name carries the campaign,
 * which tells ours apart. The poll runs every 30 s, so allow for one full
 * wait.
 */
export async function waitForNotice(page: Page, campaignName: string, timeoutMs = 45_000): Promise<Locator> {
  const link = page.getByRole('link', { name: `Entrar na sessão 1 de ${campaignName}`, exact: true });
  await expect(page.getByRole('status').getByText(`A sessão 1 de ${campaignName} começou.`)).toBeVisible({
    timeout: timeoutMs,
  });
  await expect(link).toBeVisible();
  return link;
}

/** A phone's session bar keeps "Iniciar combate" in the "Sessão" menu; a computer's has it as a button. */
const onPhone = (page: Page): boolean => (page.viewportSize()?.width ?? Number.POSITIVE_INFINITY) < 768;

/** Opens the master's "Sessão" menu of the session bar. */
export async function openSessionMenu(page: Page): Promise<void> {
  await page.getByRole('button', { name: 'Sessão', exact: true }).click();
}

/** "Iniciar combate" of the session bar, wherever the screen keeps it: opens the dialog. */
export async function clickStartCombat(page: Page): Promise<void> {
  if (onPhone(page)) {
    await openSessionMenu(page);
    await page.getByRole('menuitem', { name: 'Iniciar combate' }).click();
    return;
  }
  await page.getByRole('button', { name: 'Iniciar combate' }).click();
}

/** The master is offered "Iniciar combate" (no combat is on). */
export async function expectStartCombatOffered(page: Page): Promise<void> {
  if (onPhone(page)) {
    await openSessionMenu(page);
    await expect(page.getByRole('menuitem', { name: 'Iniciar combate' })).toBeVisible();
    await page.keyboard.press('Escape');
    return;
  }
  await expect(page.getByRole('button', { name: 'Iniciar combate' })).toBeVisible();
}

/** "Encerrar sessão" from the "Sessão" menu, then "Confirmar encerramento" in place under the bar. */
export async function endSessionFromBar(page: Page): Promise<void> {
  await openSessionMenu(page);
  await page.getByRole('menuitem', { name: 'Encerrar sessão' }).click();
  await page.getByRole('button', { name: 'Confirmar encerramento' }).click();
}
