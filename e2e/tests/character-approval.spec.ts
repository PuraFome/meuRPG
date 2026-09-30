import { expect, test, type Page } from '@playwright/test';

import {
  authStatePath,
  callRPC,
  characterRpcBody,
  createCampaign,
  createCharacterRPC,
  createCharacterViaUI,
  newSignedInContext,
  pensantus,
} from './support';

// MR-024 / RN-15 (convite com aprovação), against the real screens:
//   - the invite form's "Exigir aprovação do mestre" checkbox
//     (web/src/app/pages/campaign-detail/invites);
//   - the invite page taking a new pending member straight to "Criar
//     personagem" (invite-accept.ts);
//   - the wait banner on the campaign page and on the sheet;
//   - the master's "Esperando aprovação" list (campaign-characters) and the
//     "Aprovar personagem" / "Recusar personagem" buttons on the sheet.
//
// No test here signs in for real: both users reuse the states auth.setup.ts
// saved. The default page is the master's.
test.use({ storageState: authStatePath('Mestre Teste') });

/** Creates a campaign and an invite that requires approval, through the
 * screens, and returns the campaign's id and the invite link. */
async function campaignWithApprovalInvite(page: Page, name: string): Promise<{ campaignId: string; link: string }> {
  const campaignId = await createCampaign(page, name);
  await page.getByLabel('Exigir aprovação do mestre').check();
  await page.getByRole('button', { name: 'Gerar convite' }).click();
  await expect(page.getByText('só entra na campanha depois que você aprovar')).toBeVisible();
  const link = (await page.locator('.invite-reveal__link').textContent())?.trim();
  expect(link).toBeTruthy();
  // The invite list says so too (one row per invite, not a table).
  await expect(page.getByRole('list', { name: 'Convites gerados' }).getByText('Exige aprovação do mestre')).toBeVisible();
  return { campaignId, link: link! };
}

/** Opens the master's "Esperando aprovação" list and the sheet of the
 * character named `name` from it. */
async function openPendingCharacter(page: Page, campaignId: string, name: string): Promise<void> {
  await page.goto(`/campanhas/${campaignId}`);
  await expect(page.getByRole('heading', { name: 'Esperando aprovação' })).toBeVisible();
  await page.getByRole('list', { name: 'Esperando aprovação' }).getByRole('link', { name }).click();
  await expect(page).toHaveURL(new RegExp(`/campanhas/${campaignId}/personagens/[^/]+$`));
  await expect(page.getByRole('heading', { level: 1 })).toContainText(name);
}

test(
  'personagem criado por convite com aprovação fica pendente até o mestre aprovar',
  { tag: ['@MR-024', '@RN-15'] },
  async ({ page, browser }) => {
    const campaignName = `Convite com aprovação ${Date.now()}`;
    const { campaignId, link } = await campaignWithApprovalInvite(page, campaignName);

    const playerContext = await newSignedInContext(browser, 'Jogador Teste');
    try {
      const playerPage = await playerContext.newPage();
      // Accepting takes the new pending member straight to creating the
      // character (RN-15: "pelo convite, o jogador já cria o personagem").
      await playerPage.goto(link);
      await expect(playerPage).toHaveURL(new RegExp(`/campanhas/${campaignId}/personagens/novo$`));

      const characterId = await createCharacterViaUI(playerPage, campaignId, pensantus);
      await expect(playerPage.getByRole('list', { name: 'Estado do personagem' })).toContainText('Pendente de aprovação');
      await expect(playerPage.getByText('Esperando a aprovação do mestre')).toBeVisible();
      // They keep editing while they wait.
      await expect(playerPage.getByRole('link', { name: 'Editar ficha' })).toBeVisible();

      // The campaign page shows only the name and the wait, never the members.
      await playerPage.goto(`/campanhas/${campaignId}`);
      await expect(playerPage.getByRole('heading', { level: 1 })).toHaveText(campaignName);
      await expect(playerPage.getByText('Esperando a aprovação do mestre.')).toBeVisible();
      await expect(playerPage.getByRole('heading', { name: 'Membros' })).toHaveCount(0);
      const members = await callRPC(playerPage, 'meurpg.campaigns.v1.CampaignService/ListMembers', { campaignId });
      expect(members.status()).toBe(404);

      // Not a member yet: the master's member list has only the master.
      const listRes = await callRPC(page, 'meurpg.campaigns.v1.CampaignService/ListMembers', { campaignId });
      const listed = (await listRes.json()).members as { role: string }[];
      expect(listed.filter((m) => m.role !== 'ROLE_MASTER')).toHaveLength(0);

      // The master finds it waiting, opens the sheet and approves it.
      await openPendingCharacter(page, campaignId, pensantus.name);
      await page.getByRole('button', { name: 'Aprovar personagem' }).click();
      await expect(page.getByRole('list', { name: 'Estado do personagem' })).toContainText('Rascunho');
      await expect(page.getByRole('button', { name: 'Aprovar personagem' })).toHaveCount(0);

      // Now the player is a player: the whole campaign page, and a draft.
      await playerPage.reload();
      await expect(playerPage.getByRole('heading', { name: 'Membros' })).toBeVisible();
      await expect(playerPage.getByText('Esperando a aprovação do mestre.')).toHaveCount(0);
      const afterApproval = await callRPC(playerPage, 'meurpg.characters.v1.CharacterService/GetCharacter', {
        campaignId,
        characterId,
      });
      expect((await afterApproval.json()).character.state).toBe('CHARACTER_STATE_DRAFT');
    } finally {
      await playerContext.close();
    }
  },
);

test(
  'o mestre recusa o personagem pendente e o jogador não entra',
  { tag: ['@MR-024', '@RN-15'] },
  async ({ page, browser }) => {
    const { campaignId, link } = await campaignWithApprovalInvite(page, `Convite recusado ${Date.now()}`);

    const playerContext = await newSignedInContext(browser, 'Jogador Teste');
    try {
      const playerPage = await playerContext.newPage();
      await playerPage.goto(link);
      await expect(playerPage).toHaveURL(new RegExp(`/campanhas/${campaignId}/personagens/novo$`));

      // Created through the API: the editor is the first test's story.
      const created = await createCharacterRPC(playerPage, campaignId, characterRpcBody('PLAYER', pensantus));
      expect(created.status()).toBe(200);
      const character = (await created.json()).character;
      expect(character.state).toBe('CHARACTER_STATE_PENDING');

      // The master rejects it, confirming first: the character is deleted.
      await openPendingCharacter(page, campaignId, pensantus.name);
      await page.getByRole('button', { name: 'Recusar personagem' }).click();
      await page.getByRole('button', { name: 'Confirmar recusa' }).click();
      await expect(page).toHaveURL(new RegExp(`/campanhas/${campaignId}$`));
      await expect(page.getByRole('heading', { name: 'Esperando aprovação' })).toHaveCount(0);

      // The player never became a member: the same not_found as anyone who
      // isn't (ADR-0011), and the same "campanha não encontrada" page.
      const getCampaignRes = await callRPC(playerPage, 'meurpg.campaigns.v1.CampaignService/GetCampaign', { campaignId });
      expect(getCampaignRes.status()).toBe(404);
      expect(await getCampaignRes.json()).toMatchObject({ code: 'not_found' });
      const getCharacterRes = await callRPC(playerPage, 'meurpg.characters.v1.CharacterService/GetCharacter', {
        campaignId,
        characterId: character.id,
      });
      expect(getCharacterRes.status()).toBe(404);
      await playerPage.goto(`/campanhas/${campaignId}`);
      await expect(playerPage.getByRole('heading', { level: 1 })).toHaveText('Campanha não encontrada');

      // The link was single use: trying again needs a new invite.
      await playerPage.goto(link);
      await expect(playerPage.getByText('já foi usado', { exact: false })).toBeVisible();
    } finally {
      await playerContext.close();
    }
  },
);
