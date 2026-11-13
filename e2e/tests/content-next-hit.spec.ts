import { expect } from '@playwright/test';
import { test } from './diagnostics';

import { createEntryRPC, entryRoute, hitSpellBody, listEntriesJSON } from './content-support';
import { pickRadio } from './move-support';
import { campaignWithEmptyPlayer } from './table-rules-support';
import { newSignedInContext } from './support';

test.describe.configure({ timeout: 150_000 });

// #577: "Efeito no próximo golpe" of a table spell, through the editor and the read view. The spell is made on the screen; what the
// server kept is read back through the API, and a player reads it on the spell's page.

test(
  'o mestre cria uma magia com efeito no próximo golpe, recarrega e a lê como o jogador @MR-025',
  { tag: ['@MR-025', '@RN-23'] },
  async ({ browser }) => {
    const master = await newSignedInContext(browser, 'Mestre Teste');
    const player = await newSignedInContext(browser, 'Jogador Teste');
    try {
      const m = await master.newPage();
      const p = await player.newPage();
      await Promise.all([m.goto('/'), p.goto('/')]);
      const campaignId = await campaignWithEmptyPlayer(m, p, `Próximo golpe ${Date.now()}`);

      await m.goto(`/campaigns/${campaignId}/content/new/spell`);
      await m.getByLabel('Nome', { exact: true }).fill('Geada no Punho');
      await m.getByLabel('Nível da magia').selectOption({ label: '2º círculo' });
      await m.getByLabel('Tempo de conjuração').selectOption({ label: 'Ação bônus' });
      await m.getByLabel('Alcance', { exact: true }).selectOption({ label: 'Pessoal' });
      await m.getByLabel('Duração', { exact: true }).selectOption({ label: 'Por um tempo' });
      await m.getByRole('switch', { name: 'Concentração' }).click();
      await pickRadio(m, 'Só quem conjura');
      await m.getByLabel('Descrição').fill('O frio sobe pelo braço e fica preso no punho até o próximo golpe de perto.');

      // A spell that lasts keeps the effect: the switch is on offer, and the four triggers come with it.
      const block = m.locator('app-hit-effect-block');
      const toggle = block.getByRole('switch', { name: 'Efeito no próximo golpe' });
      await expect(toggle).toBeEnabled();
      await toggle.click();
      await expect(toggle).toHaveAttribute('aria-checked', 'true');
      await pickRadio(block, 'Próximo golpe corpo a corpo');
      // "Só quem conjura" is the target: the marked target needs one creature, so its card says so.
      await expect(block.getByRole('radio', { name: /^Cada golpe no alvo marcado/ })).toBeDisabled();
      await block.getByLabel('Dados', { exact: true }).fill('2');
      await block.getByLabel('Dado', { exact: true }).selectOption({ label: 'd8' });
      await block.getByLabel('A mais por nível').fill('1');
      await block.getByLabel('No máximo').fill('5');
      await block.getByLabel('Tipo de dano').selectOption({ label: 'Frio' });
      await block.getByLabel('Habilidade').selectOption({ label: 'Força' });
      await block.getByLabel('Se falhar, fica').selectOption({ label: 'Impedido' });
      await block.getByLabel('Por quanto tempo').selectOption({ label: 'Até o fim do próximo turno do alvo' });

      const preview = m.getByRole('complementary', { name: 'Como os jogadores veem' });
      await expect(preview).toContainText('No próximo golpe corpo a corpo: +2d8 frio, +1d8 por nível acima do 2º, até 5d8.');
      await expect(preview).toContainText('O alvo faz um teste de resistência de Força; se falhar, fica Impedido até o fim do próximo turno dele.');

      await m.getByRole('button', { name: 'Salvar magia' }).click();
      await expect(m.getByText('A magia Geada no Punho foi salva.')).toBeVisible();
      await expect(m).toHaveURL(/\/content\/entries\//);

      // What the server kept is what the form said.
      const stored = (await listEntriesJSON(m, campaignId)).entries!.find((e) => e.namePt === 'Geada no Punho')!;
      expect(stored.tableSpell.concentration).toBe(true);
      expect(stored.tableSpell.effect.modifiers).toHaveLength(1);
      expect(stored.tableSpell.effect.modifiers[0]).toMatchObject({
        kind: 'TABLE_EFFECT_MODIFIER_KIND_WEAPON_HIT_DAMAGE',
        trigger: 'TABLE_HIT_TRIGGER_NEXT_MELEE_HIT',
        die: 8,
        dice: 2,
        perSlotDice: 1,
        maxDice: 5,
        damageType: 'damage-type:cold',
        save: 'str',
        onFailConditions: ['condition:restrained'],
        conditionDuration: 'TABLE_HIT_CONDITION_DURATION_UNTIL_END_OF_NEXT_TURN',
      });

      // After a reload the block is on with the same values.
      await m.reload();
      await expect(m.getByLabel('Nome', { exact: true })).toHaveValue('Geada no Punho');
      await expect(block.getByRole('switch', { name: 'Efeito no próximo golpe' })).toHaveAttribute('aria-checked', 'true');
      await expect(block.getByRole('radio', { name: /^Próximo golpe corpo a corpo/ })).toBeChecked();
      await expect(block.getByLabel('Dados', { exact: true })).toHaveValue('2');
      await expect(block.getByLabel('Dado', { exact: true })).toHaveValue('8');
      await expect(block.getByLabel('A mais por nível')).toHaveValue('1');
      await expect(block.getByLabel('No máximo')).toHaveValue('5');
      await expect(block.getByLabel('Tipo de dano')).toHaveValue('damage-type:cold');
      await expect(block.getByLabel('Habilidade')).toHaveValue('str');
      await expect(block.getByLabel('Se falhar, fica')).toHaveValue('condition:restrained');
      await expect(block.getByLabel('Por quanto tempo')).toHaveValue('next_turn');

      // A player reads the section, the chips, the sentence and who sees the effect.
      await p.goto(m.url());
      await expect(p.getByRole('heading', { name: 'Efeito no próximo golpe' })).toBeVisible();
      await expect(p.getByText('Em quem conjura: o próximo golpe corpo a corpo, por até 1 minuto.')).toBeVisible();
      await expect(p.getByText('+2d8 frio', { exact: true })).toBeVisible();
      await expect(p.getByText('+1d8 por nível acima do 2º, até 5d8', { exact: true })).toBeVisible();
      await expect(p.getByText('O alvo faz um teste de resistência de Força; se falhar, fica Impedido até o fim do próximo turno dele.')).toBeVisible();
      await expect(p.getByText('Quem vê o efeito')).toBeVisible();
      await expect(p.getByText('Só quem conjura e você')).toBeVisible();
    } finally {
      await Promise.all([master.close(), player.close()]);
    }
  },
);

test('a recusa do efeito no próximo golpe volta no campo, e a magia só salva depois de corrigir @MR-025', { tag: ['@MR-025', '@RN-23'] }, async ({ browser }) => {
  const master = await newSignedInContext(browser, 'Mestre Teste');
  const player = await newSignedInContext(browser, 'Jogador Teste');
  try {
    const m = await master.newPage();
    const p = await player.newPage();
    await Promise.all([m.goto('/'), p.goto('/')]);
    const campaignId = await campaignWithEmptyPlayer(m, p, `Próximo golpe recusa ${Date.now()}`);
    const key = await createEntryRPC(m, campaignId, 'tableSpell', hitSpellBody('Geada no Punho'));

    await m.goto(entryRoute(campaignId, key));
    const block = m.locator('app-hit-effect-block');
    await expect(m.getByLabel('Nome', { exact: true })).toHaveValue('Geada no Punho');

    // The cap is the form's own refusal: it never reaches the server.
    await block.getByLabel('No máximo').fill('');
    await m.getByRole('button', { name: 'Salvar magia' }).click();
    await expect(block.getByLabel('No máximo')).toHaveAttribute('aria-invalid', 'true');
    await expect(block.getByText('Diga o máximo de dados, de 1 a 20.')).toBeVisible();

    // Dice at 0 go to the server, which refuses them at their own field; what was typed stays.
    await block.getByLabel('No máximo').fill('5');
    await block.getByLabel('Dados', { exact: true }).fill('0');
    await m.getByRole('button', { name: 'Salvar magia' }).click();
    const dice = block.getByLabel('Dados', { exact: true });
    await expect(dice).toHaveAttribute('aria-invalid', 'true');
    await expect(block.getByText('De 1 a 10 dados.')).toBeVisible();
    await expect(m.getByRole('alert').filter({ hasText: 'Não foi possível salvar a magia.' })).toContainText('1 campo precisa de ajuste');
    await expect(dice).toHaveValue('0');

    // The saved spell still has the old dice.
    const stored = (await listEntriesJSON(m, campaignId)).entries!.find((e) => e.namePt === 'Geada no Punho')!;
    expect(stored.tableSpell.effect.modifiers[0].dice).toBe(2);

    // Fixed, it saves.
    await dice.fill('3');
    await m.getByRole('button', { name: 'Salvar magia' }).click();
    await expect(m.getByText('A magia Geada no Punho foi salva.')).toBeVisible();
    await expect(dice).not.toHaveAttribute('aria-invalid', 'true');
  } finally {
    await Promise.all([master.close(), player.close()]);
  }
});
