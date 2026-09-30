import { expect, test } from '@playwright/test';

import { acceptInvite, callRPC, characterRpcBody, createCampaign, createCharacterRPC, pensantus, signIn } from './support';

// MR-024 / RN-15 (convite com aprovação): PR 4 / WP-F, deferred until after
// PR 3 because it touches campaigns, authz, characters and web together
// (plan §8). Nothing here exists yet — not the invite's `requires_approval`
// field, not `CHARACTER_STATE_PENDING` going live, not
// ApproveCharacter/RejectCharacter. The contracts these specs assume
// (migrations 00020-00022, the pending member status) are only sketched in
// the plan, not agreed field-by-field, so more here than usual is a guess;
// each is flagged. Both specs are `test.fixme`.

test.fixme(
  'personagem criado por convite com aprovação fica pendente até o mestre aprovar',
  { tag: ['@MR-024', '@RN-15'] },
  async ({ page, browser }) => {
    // enabled when CreateInvite gains `requires_approval`, CreateCharacter
    // (kind PLAYER) can be born CHARACTER_STATE_PENDING, and
    // CharacterService.ApproveCharacter lands (plan §8, WP-F).
    await signIn(page, 'Mestre Teste', '/');
    const campaignId = await createCampaign(page, `Convite com aprovação ${Date.now()}`);

    // "Exigir aprovação do mestre" is this file's guess at the checkbox
    // label the plan's §8 "checkbox on the invite form" needs — not fixed
    // anywhere yet; confirm with WP-D.
    await page.getByLabel('Exigir aprovação do mestre').check();
    await page.getByRole('button', { name: 'Gerar convite' }).click();
    const link = (await page.locator('.invite-reveal__link').textContent())?.trim();

    const playerContext = await browser.newContext();
    try {
      const playerPage = await playerContext.newPage();
      await signIn(playerPage, 'Jogador Teste', '/');
      const { page: joinedPage } = await acceptInvite(playerContext, link!);

      const created = await createCharacterRPC(joinedPage, campaignId, characterRpcBody('PLAYER', pensantus));
      expect(created.status()).toBe(200);
      const character = (await created.json()).character;
      // CHARACTER_STATE_PENDING (amendment A1: already reserved in 00014's
      // characters_status_valid, so no later migration adds it).
      expect(character.state).toBe('CHARACTER_STATE_PENDING');

      // Pending: not yet part of the campaign the way an approved player is
      // (plan §8: "makes the character and the membership active in one
      // transaction" — implying neither is active before that).
      const listRes = await callRPC(page, 'meurpg.campaigns.v1.CampaignService/ListMembers', { campaignId });
      const members = (await listRes.json()).members as { role: string }[];
      expect(members.filter((m) => m.role !== 'ROLE_MASTER')).toHaveLength(0);

      const approveRes = await callRPC(page, 'meurpg.characters.v1.CharacterService/ApproveCharacter', {
        campaignId,
        characterId: character.id,
      });
      expect(approveRes.status()).toBe(200);

      const afterApproval = await callRPC(joinedPage, 'meurpg.characters.v1.CharacterService/GetCharacter', {
        campaignId,
        characterId: character.id,
      });
      expect((await afterApproval.json()).character.state).not.toBe('CHARACTER_STATE_PENDING');
    } finally {
      await playerContext.close();
    }
  },
);

test.fixme(
  'o mestre recusa o personagem pendente e o jogador não entra',
  { tag: ['@MR-024', '@RN-15'] },
  async ({ page, browser }) => {
    // enabled with the same pieces as the test above, plus
    // CharacterService.RejectCharacter (plan §8: deletes the character and
    // the pending membership — "Pendente → [*]" in the lifecycle diagram).
    await signIn(page, 'Mestre Teste', '/');
    const campaignId = await createCampaign(page, `Convite recusado ${Date.now()}`);
    await page.getByLabel('Exigir aprovação do mestre').check();
    await page.getByRole('button', { name: 'Gerar convite' }).click();
    const link = (await page.locator('.invite-reveal__link').textContent())?.trim();

    const playerContext = await browser.newContext();
    try {
      const playerPage = await playerContext.newPage();
      await signIn(playerPage, 'Jogador Teste', '/');
      const { page: joinedPage } = await acceptInvite(playerContext, link!);

      const created = await createCharacterRPC(joinedPage, campaignId, characterRpcBody('PLAYER', pensantus));
      const character = (await created.json()).character;

      const rejectRes = await callRPC(page, 'meurpg.characters.v1.CharacterService/RejectCharacter', {
        campaignId,
        characterId: character.id,
      });
      expect(rejectRes.status()).toBe(200);

      // The player never became a member: same `not_found` as anyone who
      // isn't (ADR-0011), not a special "seu pedido foi recusado" screen.
      const getCampaignRes = await callRPC(joinedPage, 'meurpg.campaigns.v1.CampaignService/GetCampaign', { campaignId });
      expect(getCampaignRes.status()).toBe(404);
      expect(await getCampaignRes.json()).toMatchObject({ code: 'not_found' });

      const getCharacterRes = await callRPC(joinedPage, 'meurpg.characters.v1.CharacterService/GetCharacter', {
        campaignId,
        characterId: character.id,
      });
      expect(getCharacterRes.status()).toBe(404);
    } finally {
      await playerContext.close();
    }
  },
);
