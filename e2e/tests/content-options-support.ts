import { expect, type Page } from '@playwright/test';

import { callRPC } from './support';

// Setup for the "Opções para os jogadores" specs (Etapa 10, slice 10.11c, MR-025, RN-23, RN-10): the switches are turned
// through the API where a test proves what the players read, and on the screen where it proves the screen.

const service = 'meurpg.rules.v1.TableContentService';

export interface OptionRow {
  key: string;
  kind: string;
  namePt: string;
  table?: boolean;
  off?: boolean;
  archived?: boolean;
  hidden?: boolean;
  parentKey?: string;
  charactersUsing?: number;
}

/** `ListOptionSwitches` as the app's JSON (the master's own read). */
export async function optionSwitchesJSON(master: Page, campaignId: string): Promise<OptionRow[]> {
  const res = await callRPC(master, `${service}/ListOptionSwitches`, { campaignId });
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()).options ?? [];
}

/** `SetOptionSwitches`, as another tab of the master would. */
export async function setSwitchesRPC(master: Page, campaignId: string, switches: { key: string; off: boolean }[]): Promise<void> {
  const res = await callRPC(master, `${service}/SetOptionSwitches`, { campaignId, switches });
  expect(res.ok(), await res.text()).toBeTruthy();
}

/** `ContentService.ListContent` as the JSON of whoever asks: the catalog the pickers read. */
export async function catalogJSON(page: Page, campaignId: string): Promise<{ races?: { key: string; namePt: string }[]; classes?: { key: string }[]; subclasses?: { key: string; classKey: string }[] }> {
  const res = await callRPC(page, 'meurpg.rules.v1.ContentService/ListContent', { campaignId });
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()).content ?? {};
}

/** The names the Raça field of the character editor offers, read from the open list (it is closed again). */
export async function raceOptions(page: Page): Promise<string[]> {
  const field = page.getByLabel('Raça', { exact: true });
  await field.click();
  await expect(page.getByRole('option').first()).toBeVisible();
  const names = (await page.getByRole('option').allTextContents()).map((t) => t.trim());
  await page.keyboard.press('Escape');
  await expect(page.getByRole('option')).toHaveCount(0);
  return names;
}

/** `ListContent` as the raw text the player's app receives: the JSON a test greps for what must never be there. */
export async function catalogText(page: Page, campaignId: string): Promise<string> {
  const res = await callRPC(page, 'meurpg.rules.v1.ContentService/ListContent', { campaignId });
  expect(res.ok(), await res.text()).toBeTruthy();
  return JSON.stringify(await res.json());
}
