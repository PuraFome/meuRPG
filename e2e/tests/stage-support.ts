import { expect, type Page } from '@playwright/test';

import { beginAttackCombatRPC, combatRPC, getEncounterRPC, type CombatTable, type Encounter } from './combat-support';
import { callRPC, characterRpcBody, createCharacterRPC, pensantus } from './support';
import { uploadImageRPC } from './maps-support';

// Setup for the stage and highlights specs (Etapa 8, MR-031, MR-032), through
// the API. The portraits are drawn here as PNGs with a transparent background
// (a canvas keeps the alpha), so no binary fixture lives in the repository.

/** A bust on a transparent background: a head and shoulders, the rest of the
 * canvas left clear (alpha 0), the way an NPC cut-out arrives. */
export async function portraitPng(page: Page, skin: string, label: string): Promise<Buffer> {
  const base64 = await page.evaluate(
    ({ skin, label }) => {
      const canvas = document.createElement('canvas');
      canvas.width = 300;
      canvas.height = 360;
      const ctx = canvas.getContext('2d')!;
      ctx.fillStyle = '#7a5534';
      ctx.beginPath();
      ctx.moveTo(20, 360);
      ctx.bezierCurveTo(20, 270, 90, 250, 150, 250);
      ctx.bezierCurveTo(210, 250, 280, 270, 280, 360);
      ctx.fill();
      ctx.fillStyle = skin;
      ctx.beginPath();
      ctx.ellipse(150, 150, 62, 80, 0, 0, Math.PI * 2);
      ctx.fill();
      void label; // the image carries no text: the name is the app's to draw
      return canvas.toDataURL('image/png').split(',')[1];
    },
    { skin, label },
  );
  return Buffer.from(base64, 'base64');
}

/** Uploads a transparent portrait to the gallery; returns the image's ID. */
export async function uploadPortrait(page: Page, campaignId: string, name: string, skin = '#e0b590'): Promise<string> {
  return uploadImageRPC(page, campaignId, name, await portraitPng(page, skin, name));
}

/** Mira (or another story NPC, by `name`), through the API; returns the ID. */
export async function createMiraRPC(page: Page, campaignId: string, portraitImageId = '', name = 'Mira'): Promise<string> {
  const res = await createCharacterRPC(page, campaignId, {
    kind: 'CHARACTER_KIND_STORY',
    name,
    sheet: { basic: { hitPointsMax: 9, armorClass: 11, speedFt: 30, attackBonus: 0, damage: '', description: '', portraitImageId } },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()).character.id as string;
}

/** The Capitão Goblin, an enemy with a portrait, through the API. */
export async function createCapitaoRPC(page: Page, campaignId: string, portraitImageId = ''): Promise<string> {
  const body = characterRpcBody('ENEMY', { ...pensantus, name: 'Capitão Goblin' }) as { sheet: { full: object } };
  body.sheet.full = { ...body.sheet.full, portraitImageId };
  const res = await createCharacterRPC(page, campaignId, body);
  expect(res.ok(), await res.text()).toBeTruthy();
  return (await res.json()).character.id as string;
}

/** The stage as the caller sees it (`GetOpenScene`). */
export async function getStageRPC(page: Page, campaignId: string): Promise<{ id: string; name: string; portraitUrl?: string; speaking?: boolean; characterId?: string }[]> {
  const res = await callRPC(page, 'meurpg.play.v1.PlayService/GetOpenScene', { campaignId });
  expect(res.ok(), await res.text()).toBeTruthy();
  return ((await res.json()).scene?.stage ?? []) as never;
}

/** `PutOnStage` through the API, as the master. */
export async function putOnStageRPC(page: Page, campaignId: string, characterId: string): Promise<void> {
  const res = await callRPC(page, 'meurpg.play.v1.PlayService/PutOnStage', { campaignId, characterId });
  expect(res.ok(), await res.text()).toBeTruthy();
}

/** A combat in which Pensantus (the player) hits the Capitão for real and the
 * damage is applied, so the highlights have a winner. Returns the encounter,
 * still running. */
export async function playedCombatRPC(m: Page, p: Page, table: CombatTable): Promise<Encounter> {
  const { campaignId } = table;
  // Pensantus goes first, hits the Capitão and the master applies the damage.
  let enc = await beginAttackCombatRPC(m, table, { Pensantus: 20, 'Capitão Goblin': 5, 'Goblin 1': 4, 'Goblin 2': 3 });
  const pens = enc.combatants.find((c) => c.label === 'Pensantus')!;
  const captain = enc.combatants.find((c) => c.label === 'Capitão Goblin')!;
  const opts = await callRPC(p, 'meurpg.play.v1.CombatService/GetTurnOptions', { campaignId, encounterId: enc.id, combatantId: pens.id });
  expect(opts.ok(), await opts.text()).toBeTruthy();
  const body = await opts.json();
  const attacks = body.options.attacks as { attack: { key: string } }[];
  const attackKey = attacks.find((a) => a.attack.key.startsWith('equipment:'))!.attack.key;
  // The server says how the d20 rolls (a bow with an enemy next to the archer has disadvantage): one die for a
  // normal roll, two for advantage or disadvantage, and the one that counts is a 19 whatever the mode.
  const reach = (body.attackTargets as { attackKey: string; targets: { combatantId: string; rollMode?: string }[] }[]).find((t) => t.attackKey === attackKey);
  const mode = reach?.targets.find((t) => t.combatantId === captain.id)?.rollMode;
  const dice = mode === 'ROLL_MODE_ADVANTAGE' ? { d20Faces: [19, 1] } : mode === 'ROLL_MODE_DISADVANTAGE' ? { d20Faces: [19, 20] } : { d20Face: 19 };
  const hit = await callRPC(p, 'meurpg.play.v1.CombatService/RollAttack', {
    campaignId, encounterId: enc.id, attackerId: pens.id, attackKey, targetId: captain.id, ...dice, idempotencyKey: crypto.randomUUID(),
  });
  expect(hit.ok(), await hit.text()).toBeTruthy();
  const pending = (await hit.json()).pendingDamage.id as string;
  const dmg = await callRPC(p, 'meurpg.play.v1.CombatService/RollDamage', {
    campaignId, encounterId: enc.id, pendingDamageId: pending, typedSum: 4, idempotencyKey: crypto.randomUUID(),
  });
  expect(dmg.ok(), await dmg.text()).toBeTruthy();
  const status = (await dmg.json()).pendingDamage?.status as string | undefined;
  if (status !== 'PENDING_DAMAGE_STATUS_APPLIED') {
    await combatRPC(m, 'ApplyPendingDamage', { campaignId, encounterId: enc.id, pendingDamageId: pending });
  }
  enc = await getEncounterRPC(m, campaignId);
  return enc;
}
