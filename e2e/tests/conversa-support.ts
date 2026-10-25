import { expect, type Browser, type BrowserContext, type Locator, type Page } from '@playwright/test';

import { endOpenSessionRPC, openSessionPage, startSessionRPC, tableWithPensantus } from './live-session-support';
import { pickRadio } from './move-support';
import { secondPlayer, type SecondPlayer } from './puzzles-support';
import { createCapitaoRPC } from './stage-support';
import { callRPC, newSignedInContext } from './support';

// Setup and helpers for the scene's conversation specs (MR-051, RN-36). The table has the master,
// Pensantus (Jogador Teste) and Toren ("E-mail Não Verificado"): the only two saved player accounts, so
// "the reader who was not chosen" is always the other of the two. Every read is of the feed
// ("Mensagens da cena"), never of the screen-reader log that repeats each new line.

/** Toren's account name: set as his display name at the table (the devidp account has none), so an out-of-character line can say it. */
export const TOREN_ACCOUNT = 'E-mail Não Verificado';

export interface ConversaTable {
  campaignId: string;
  master: Page;
  pensantus: Page;
  toren: Page;
  close: () => Promise<void>;
}

export interface ConversaOptions {
  width?: number;
  colorScheme?: 'light' | 'dark';
}

/**
 * A live session (at `width`, in `colorScheme`) with the master, Pensantus and Toren all on the session page, and the Capitão
 * Goblin (an NPC the master may speak as) in the campaign. Closing ends the session and the contexts.
 */
export async function openConversaTable(browser: Browser, name: string, options: ConversaOptions = {}): Promise<ConversaTable> {
  const { width = 1280, colorScheme } = options;
  const viewport = { width, height: width >= 768 ? 1000 : 844 };
  const masterContext: BrowserContext = await newSignedInContext(browser, 'Mestre Teste', { viewport, colorScheme });
  const playerContext: BrowserContext = await newSignedInContext(browser, 'Jogador Teste', { viewport, colorScheme });
  const master = await masterContext.newPage();
  const pensantus = await playerContext.newPage();
  let toren: SecondPlayer | undefined;
  let campaignId = '';
  const close = async () => {
    if (campaignId) {
      await endOpenSessionRPC(master, campaignId);
    }
    await toren?.close();
    await playerContext.close();
    await masterContext.close();
  };
  try {
    await Promise.all([master.goto('/'), pensantus.goto('/')]);
    ({ campaignId } = await tableWithPensantus(master, pensantus, name));
    toren = await secondPlayer(browser, master, campaignId, viewport);
    // A display name comes from the Perfil, not from the sign-in: without one an out-of-character line has no name to show.
    const named = await callRPC(toren.page, 'meurpg.identity.v1.IdentityService/UpdateProfile', { displayName: TOREN_ACCOUNT });
    expect(named.ok(), await named.text()).toBeTruthy();
    await createCapitaoRPC(master, campaignId);
    await startSessionRPC(master, campaignId);
    // The page reads the lines on the stream's first `ready`; a line sent before that is history, not news. On a phone
    // nothing else on the page tells the stream is up, so wait for that read before anybody speaks.
    const streamUp = (p: Page) =>
      p.waitForResponse((res) => res.url().includes('PlayService/ListSessionMessages'), { timeout: 30_000 });
    const pages = [master, pensantus, toren.page];
    const reads = width < 768 ? pages.map(streamUp) : [];
    reads.forEach((read) => void read.catch(() => undefined)); // an early failure below must not leave these unhandled
    await Promise.all(pages.map((p) => openSessionPage(p, campaignId)));
    await Promise.all(reads);
    if (width >= 768) {
      // On a phone the panel is not drawn until the "Conversa" tab is opened.
      // The empty feed is not drawn (the empty sentence stands in its place), so the panel is awaited by its message box.
      await Promise.all([master, pensantus, toren.page].map((p) => expect(messageBoxOf(p)).toBeVisible({ timeout: 30_000 })));
    }
  } catch (error) {
    await close();
    throw error;
  }
  return { campaignId, master, pensantus, toren: toren.page, close };
}

/** The message box of the panel: there as soon as the panel is, whether or not a line has arrived. */
export function messageBoxOf(page: Page): Locator {
  return page.getByRole('textbox', { name: 'Mensagem' });
}

/** The list of lines. It is in the page once a first line has arrived, not before. */
export function feedOf(page: Page): Locator {
  return page.getByRole('list', { name: 'Mensagens da cena' });
}

/** The line whose text is `text`, as a list item of the feed. */
export function lineOf(page: Page, text: string): Locator {
  return feedOf(page).getByRole('listitem').filter({ hasText: text });
}

/** The player's "Só o mestre lê" toggle: pressed, the next line is for the master alone. */
export async function onlyMaster(page: Page): Promise<void> {
  const toggle = page.getByRole('button', { name: 'Só o mestre lê' });
  await toggle.click();
  await expect(toggle).toHaveAttribute('aria-pressed', 'true');
}

/** Writes in the message box and presses "Enviar". */
export async function say(page: Page, text: string): Promise<void> {
  await messageBoxOf(page).fill(text);
  await page.getByRole('button', { name: 'Enviar', exact: true }).click();
}

/** The master's "Para" chooser: the whole table, or the characters named (matched by their start). */
export async function chooseReaders(master: Page, names: readonly string[]): Promise<void> {
  await master.getByLabel('Para', { exact: true }).click();
  const chooser = master.getByRole('dialog', { name: 'Quem lê esta mensagem?' });
  if (names.length === 0) {
    await pickRadio(chooser, 'Toda a mesa');
  } else {
    await pickRadio(chooser, 'Só algumas pessoas');
    for (const name of names) {
      await chooser.getByRole('checkbox', { name: new RegExp(`^${name}`) }).check();
    }
  }
  await chooser.getByRole('button', { name: 'Pronto' }).click();
  await expect(chooser).toBeHidden();
}
