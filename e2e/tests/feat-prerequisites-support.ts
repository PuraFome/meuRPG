import type { Page } from '@playwright/test';

import { createEntryRPC } from './content-support';
import { setSwitchesRPC } from './content-options-support';
import { setTableRulesRPC } from './table-rules-support';

export interface PrerequisiteFeats {
  runes: string;
  flame: string;
  archivist: string;
  blade: string;
  shadow: string;
}

/**
 * The feats of the artboard 580-04 at a guided level-up (#580): the table plays with feats, one the character qualifies for,
 * one that asks a spell, one that asks a class level, a pact, a spell and two options joined into one line, and one that asks
 * an option the master turned off (so the player reads "uma condição que a mesa não mostra").
 */
export async function createPrerequisiteFeatsRPC(master: Page, campaignId: string): Promise<PrerequisiteFeats> {
  await setTableRulesRPC(master, campaignId, { featsAllowed: true });
  const ember = await createEntryRPC(master, campaignId, 'tableOption', { namePt: 'Olhar de Brasa', descPt: ['Você enxerga no escuro.'], set: 'invocation' });
  const sedent = await createEntryRPC(master, campaignId, 'tableOption', { namePt: 'Lâmina Sedenta', descPt: ['A lâmina pede mais.'], set: 'invocation' });
  const hiddenOption = await createEntryRPC(master, campaignId, 'tableOption', { namePt: 'Pacto Oculto', descPt: ['Escondido.'], set: 'invocation' });
  const runes = await createEntryRPC(master, campaignId, 'tableFeat', {
    namePt: 'Estudioso das Runas',
    descPt: ['Você aprende as runas do grimório.'],
    prerequisite: { classLevel: { classKey: 'class:wizard', level: 4 } },
  });
  const flame = await createEntryRPC(master, campaignId, 'tableFeat', {
    namePt: 'Chama Insistente',
    descPt: ['Seu fogo ignora resistência.'],
    prerequisite: { spellcasting: true },
  });
  const archivist = await createEntryRPC(master, campaignId, 'tableFeat', {
    namePt: 'Arquivista do Vale',
    descPt: ['Você guarda os saberes do Vale.'],
    prerequisite: { requiresSpellKey: 'spell:identify' },
  });
  const blade = await createEntryRPC(master, campaignId, 'tableFeat', {
    namePt: 'Lâmina Rubra',
    descPt: ['A arma do pacto guarda a força da sua rajada.'],
    prerequisite: {
      classLevel: { classKey: 'class:warlock', level: 5 },
      requiresPact: 'feature:pact-of-the-blade',
      requiresSpellKey: 'spell:eldritch-blast',
      requiresOptionKeys: [ember, sedent],
    },
  });
  const shadow = await createEntryRPC(master, campaignId, 'tableFeat', {
    namePt: 'Sombra do Curador',
    descPt: ['Uma sombra que cura.'],
    prerequisite: { requiresOptionKeys: [hiddenOption] },
  });
  await setSwitchesRPC(master, campaignId, [{ key: hiddenOption, off: true }]);
  return { runes, flame, archivist, blade, shadow };
}
