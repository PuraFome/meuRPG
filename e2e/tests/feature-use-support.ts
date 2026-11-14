import { expect, type Page } from '@playwright/test';
import { combatRPC, startEncounterRPC, type CombatTable, type Encounter } from './combat-support';
import { endOpenSessionRPC } from './live-session-support';

/** Support of the "Usar característica" screens (#743): table feats whose `grant_action` declares a `use`, so the character's turn lists them
 * as "Usar ..." rows and the sheet asks who and which. Invented names. */

/** Once per combat, one enemy up to 30 feet: the target list and the "Em quem" limit. */
export function markFeatBody(namePt = 'Marca de Teste'): Record<string, unknown> {
  return {
    namePt,
    descPt: ['Você marca um inimigo ao alcance, uma vez por combate.'],
    effects: [
      {
        type: 'grant_action',
        economy: 'bonus_action',
        use: {
          limitScope: 'USE_SCOPE_COMBAT',
          limitUses: '1',
          targets: { min: 1, max: 1, who: 'FEATURE_TARGET_WHO_ENEMY', rangeFt: 30 },
          applies: { tempHp: '2' },
        },
      },
    ],
  };
}

/** Twice per round, one to three enemies up to 60 feet: the check boxes and "2 de 3 escolhidos". */
export function volleyFeatBody(namePt = 'Rajada de Teste'): Record<string, unknown> {
  return {
    namePt,
    descPt: ['Você aponta até três inimigos ao alcance, duas vezes por rodada.'],
    effects: [
      {
        type: 'grant_action',
        economy: 'action',
        use: {
          limitScope: 'USE_SCOPE_ROUND',
          limitUses: '2',
          targets: { min: 1, max: 3, who: 'FEATURE_TARGET_WHO_ENEMY', rangeFt: 60 },
          applies: { tempHp: '1' },
        },
      },
    ],
  };
}

/** Once per turn, no creature but a choice of two words: the "Qual opção" buttons. */
export function elementFeatBody(namePt = 'Elemento de Teste'): Record<string, unknown> {
  return {
    namePt,
    descPt: ['Você escolhe um elemento, uma vez por turno.'],
    effects: [
      {
        type: 'grant_action',
        economy: 'free',
        use: {
          limitScope: 'USE_SCOPE_TURN',
          limitUses: '1',
          choices: [
            { key: 'fogo', textPt: 'Fogo' },
            { key: 'gelo', textPt: 'Gelo' },
          ],
          applies: { tempHp: '3' },
        },
      },
    ],
  };
}

/** Ends the open session of a table when the test got as far as making one (a test that failed before it has no id). */
export async function endSessionOf(master: Page, campaignId: string): Promise<void> {
  if (campaignId) {
    await endOpenSessionRPC(master, campaignId);
  }
}

/** No limit, one enemy at any distance: with many enemies in the combat, the list of the sheet is long and scrolls inside it. */
export function callFeatBody(namePt = 'Chamado de Teste'): Record<string, unknown> {
  return {
    namePt,
    descPt: ['Você chama um inimigo, onde quer que ele esteja.'],
    effects: [
      {
        type: 'grant_action',
        economy: 'bonus_action',
        use: { targets: { min: 1, max: 1, who: 'FEATURE_TARGET_WHO_ENEMY' }, applies: { tempHp: '1' } },
      },
    ],
  };
}

/**
 * Begins a combat with the Capitão and `goblins` Goblins in plain sight, Pensantus first: the list of a use that takes one enemy
 * has `goblins + 2` cards (the Pensantus card is there too, off, "Lado errado"). The initiative faces differ, so no turn is joint.
 */
export async function beginCrowdedCombatRPC(master: Page, table: CombatTable, goblins: number): Promise<Encounter> {
  let enc = await startEncounterRPC(master, table, [
    { characterId: table.captainId, count: 1, hidden: false },
    { characterId: table.goblinId, count: goblins, hidden: false },
  ]);
  const faces = (label: string): number => (label === 'Pensantus' ? 20 : label === 'Capitão Goblin' ? 15 : Number(label.replace(/\D/g, '')) || 1);
  for (const c of enc.combatants) {
    enc = await combatRPC(master, 'SubmitInitiative', { campaignId: table.campaignId, encounterId: enc.id, combatantId: c.id, d20Face: faces(c.label) });
  }
  const squares: Record<string, [number, number]> = { 'Capitão Goblin': [2, 2] };
  for (let i = 1; i <= goblins; i++) {
    squares[`Goblin ${i}`] = [10 + (i % 6), 3 + Math.floor(i / 6) * 2];
  }
  for (const [label, [col, row]] of Object.entries(squares)) {
    const id = enc.combatants.find((c) => c.label === label)!.id;
    enc = await combatRPC(master, 'MoveCombatant', { campaignId: table.campaignId, encounterId: enc.id, combatantId: id, col, row });
  }
  return combatRPC(master, 'BeginCombat', { campaignId: table.campaignId, encounterId: enc.id });
}

/** The box of a locator, or a failure that says it has none (a locator that is not drawn). */
export async function boxIn(page: Page, target: ReturnType<Page['locator']>): Promise<{ x: number; y: number; width: number; height: number }> {
  const box = await target.boundingBox();
  expect(box, 'the element is drawn').not.toBeNull();
  return box!;
}
