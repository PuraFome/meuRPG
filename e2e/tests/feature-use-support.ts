import type { Page } from '@playwright/test';
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
