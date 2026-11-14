import { expect } from '@playwright/test';
import { test } from './diagnostics';

import { beginAttackCombatRPC, combatRPC, passTurnsToAndWait, pensantusCasting, tableForCombat } from './combat-support';
import { openCombatPanel } from './combat-layout';
import { createEntryRPC, hitModifier, hitSpellBody } from './content-support';
import { openCombatLog, newSignedInContext, openActionTab, pensantus, viaTargetList } from './support';
import { endOpenSessionRPC, openSessionPage } from './live-session-support';

// #577: the effect a spell keeps for the caster's next weapon hit, from the cast to the log. "Geada no Punho" (next melee hit, 2d8 of cold,
// a Strength save that restrains) is the table's own spell, put in Pensantus's list before the sheet is made. The player casts it on the
// screen, then: a miss keeps it waiting; a ranged weapon cannot use it, so its part is listed off; a melee hit brings the part into the damage,
// the roll counts it and the target's save is in the combat log, where the master reads it too.

test.describe.configure({ timeout: 240_000 });

test(
  'o jogador conjura o efeito do próximo golpe, erra e ele espera, acerta de perto e o efeito entra no dano com o teste do alvo no registro',
  { tag: ['@MR-025', '@MR-048', '@RN-22', '@RN-23'] },
  async ({ browser }) => {
    const master = await newSignedInContext(browser, 'Mestre Teste', { viewport: { width: 1280, height: 900 } });
    const player = await newSignedInContext(browser, 'Jogador Teste', { viewport: { width: 1280, height: 900 } });
    const m = await master.newPage();
    const p = await player.newPage();
    await Promise.all([m.goto('/'), p.goto('/')]);
    const sheet: Record<string, unknown> = { ...pensantusCasting, weaponKeys: ['equipment:dagger', 'equipment:shortbow'] };
    const table = await tableForCombat(m, p, `Próximo golpe ${Date.now()}`, true, true, {
      sheet,
      before: async (id, owner) => {
        // Public: the save of a spell whose effect only its owner sees is a line of the master's alone (the server's effectHiddenFrom).
        const frost = await createEntryRPC(owner, id, 'tableSpell', hitSpellBody('Geada no Punho', {}, { effect: { visibility: 'TABLE_EFFECT_AUDIENCE_PUBLIC', modifiers: [hitModifier()] } }));
        sheet.knownSpellKeys = [...pensantusCasting.knownSpellKeys, frost];
        sheet.preparedSpellKeys = [...pensantusCasting.preparedSpellKeys, frost];
        return pensantus;
      },
    });
    const campaignId = table.campaignId;
    const enc = await beginAttackCombatRPC(m, table, { Pensantus: 20, 'Capitão Goblin': 15, 'Goblin 1': 5, 'Goblin 2': 4 });
    try {
      const id = (label: string) => enc.combatants.find((c) => c.label === label)!.id;
      const moveTo = async (col: number, row: number) => {
        await combatRPC(m, 'MoveCombatant', { campaignId, encounterId: enc.id, combatantId: id('Pensantus'), col, row });
        // The sheet reads the squares the screen has: wait until the map's list shows the move (its numbers count from 1).
        await expect(p.getByRole('listitem').filter({ hasText: `Pensantus, coluna ${col + 1}, linha ${row + 1}` })).toBeVisible();
        // The attack sheet's distances come from the options the screen read before the move, and the screen sometimes keeps them (the
        // sheet says "a 7,5 m" while the server says 1,5 m, for as long as the page stays): the screen reads them again on a reload.
        await p.reload();
        await expect(p.getByRole('heading', { name: 'Sua vez, Pensantus' })).toBeVisible();
      };
      const endTurn = async () => {
        await combatRPC(m, 'EndTurn', { campaignId, encounterId: enc.id, expectedCombatantId: id('Pensantus'), discardPendingDamage: true });
        await passTurnsToAndWait(m, campaignId, 'Pensantus', p);
      };
      // Attack with a weapon on a target and type the d20: the sheet stops at the damage (or says the miss).
      // `distance` is what the sheet says between the two once the screen has the move: the roll mode follows it.
      const attack = async (weapon: RegExp, sheetName: RegExp, target: string, d20: string, distance = '') => {
        await openActionTab(p, 'Ataques');
        await p.getByRole('button', { name: weapon }).click();
        const dialog = p.getByRole('dialog', { name: sheetName });
        await viaTargetList(p, dialog);
        await dialog.locator('label', { hasText: target }).click();
        await expect(dialog).toContainText(distance);
        await p.getByRole('button', { name: 'Digitar o resultado' }).click();
        await p.getByLabel(/Role 1d20/).fill(d20);
        await p.getByRole('button', { name: `Confirmar ${d20}` }).click();
        return dialog;
      };
      const card = p.getByTestId('hit-effect-card').filter({ has: p.getByRole('heading', { name: 'Geada no Punho' }) });
      const parts = p.getByRole('region', { name: 'O que entra no dano' });

      // The cast, from "Magias": a bonus action on himself. The card of the waiting effect is the first thing in "Ataques".
      await openSessionPage(p, campaignId);
      await openActionTab(p, 'Magias');
      await p.getByRole('button', { name: 'Conjurar Geada no Punho' }).click();
      const cast = p.getByRole('dialog', { name: 'Conjurar Geada no Punho' });
      await cast.getByRole('button', { name: 'Conjurar Geada no Punho' }).click();
      // The dialog takes the name of its step: "Você conjurou Geada no Punho".
      const cast2 = p.getByRole('dialog', { name: 'Você conjurou Geada no Punho' });
      await expect(cast2).toContainText('Você está concentrado em Geada no Punho.');
      await cast2.getByRole('button', { name: 'Voltar à sua vez' }).click();
      await openActionTab(p, 'Ataques');
      await expect(card).toBeVisible();
      await expect(card).toContainText('No próximo golpe corpo a corpo: +2d8 frio; teste de resistência de Força ou fica Impedido até o fim do próximo turno dele.');
      await expect(card.getByText('Concentração')).toBeVisible();

      // First, on the cast turn, with the short bow from afar (the goblins are never left behind, so no opportunity attack): a ranged hit cannot use a melee effect, so its part is listed off, never in the roll, and it keeps waiting.
      await moveTo(4, 9);
      await attack(/^Atacar com Arco curto/, /Atacar com Arco curto/, 'Goblin 2', '18');
      await expect(parts).toContainText('Geada no Punho');
      await expect(parts).toContainText('Indisponível: só num golpe corpo a corpo. Continua esperando.');
      await expect(parts.getByRole('checkbox')).toHaveCount(0);
      const bowRoll = p.getByRole('button', { name: /^Rolar 1d6( \+ \d+)? no app$/ });
      await expect(bowRoll).toBeVisible();
      await bowRoll.click();
      await p.getByRole('button', { name: 'Voltar à sua vez' }).click();
      await openActionTab(p, 'Ataques');
      await expect(card).toBeVisible();

      // The next turn, a miss (a natural 1) in melee: the effect stays, and the sheet and the card say it is still waiting.
      await endTurn();
      await moveTo(8, 9);
      const missed = await attack(/^Atacar com Adaga/, /Atacar com Adaga/, 'Goblin 1', '1', 'a 1,5 m');
      await expect(missed.getByTestId('hit-waiting')).toContainText('Geada no Punho continua esperando o próximo golpe corpo a corpo que acertar.');
      await expect(parts).toHaveCount(0);
      await missed.getByRole('button', { name: 'Voltar à sua vez' }).click();
      await openActionTab(p, 'Ataques');
      await expect(card).toBeVisible();

      // Another turn, a melee hit (already beside the goblin): the effect's part is in the damage and in the roll, with its own sentence; the roll counts it.
      await endTurn();
      await attack(/^Atacar com Adaga/, /Atacar com Adaga/, 'Goblin 1', '15', 'a 1,5 m');
      await expect(parts).toContainText('Geada no Punho');
      await expect(parts).toContainText('2d8 de frio');
      await expect(parts).toContainText('O efeito acaba com este golpe.');
      await expect(parts).not.toContainText('Indisponível');
      const roll = p.getByRole('button', { name: /^Rolar .* no app$/ });
      await expect(roll).toContainText('1d4');
      await expect(roll).toContainText('2d8');
      await roll.click();
      await expect(p.getByRole('button', { name: 'Voltar à sua vez' })).toBeFocused();
      await p.getByRole('button', { name: 'Voltar à sua vez' }).click();
      await openActionTab(p, 'Ataques');
      await expect(card).toHaveCount(0);

      // The log: the hit names the effect in its damage, and the target's save against it is a line of its own, for the player and for the master.
      const log = await openCombatLog(p);
      await expect(log).toContainText('Geada no Punho incluído');
      await expect(log).toContainText('no teste de resistência contra a Geada no Punho');
      await openSessionPage(m, campaignId);
      await openCombatPanel(m, 'Registro');
      await expect(m.getByRole('log', { name: 'Registro do combate' })).toContainText('no teste de resistência contra a Geada no Punho');
    } finally {
      await endOpenSessionRPC(m, campaignId);
      await master.close();
      await player.close();
    }
  },
);
