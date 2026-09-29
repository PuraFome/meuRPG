import { expect, test } from '@playwright/test';

import { acceptInvite, callRPC, characterRpcBody, createCampaign, createCharacterRPC, pensantus, signIn } from './support';

// RN-03 (um personagem, uma campanha) and RN-11 (notas do mestre): neither
// CharacterService nor its authz exist yet — WP-C is building them in a
// parallel branch (see /scratchpad/etapa4-plan.md). Every test here is
// `test.fixme`; each one-line comment says what unblocks it.

test.fixme(
  'o jogador só cria outro personagem na campanha depois que o atual morre, e o morto continua na lista',
  { tag: '@RN-03' },
  async ({ page, browser }) => {
    // enabled when CreateCharacter enforces the partial unique index
    // (characters_one_living_player_character) and MarkCharacterDead lands.
    await signIn(page, 'Mestre Teste', '/');
    const campaignId = await createCampaign(page, `Um personagem por vez ${Date.now()}`);
    await page.getByRole('button', { name: 'Gerar convite' }).click();
    const link = (await page.locator('.invite-reveal__link').textContent())?.trim();

    const playerContext = await browser.newContext();
    try {
      const playerPage = await playerContext.newPage();
      await signIn(playerPage, 'Jogador Teste', '/');
      const { page: joinedPage } = await acceptInvite(playerContext, link!);

      const first = await createCharacterRPC(joinedPage, campaignId, characterRpcBody('PLAYER', pensantus));
      expect(first.status()).toBe(200);
      const firstCharacter = (await first.json()).character;

      // A second living character in the same campaign: refused, with the
      // CharacterBlocked detail (reason LIVING_CHARACTER_EXISTS, plan §4).
      const second = await createCharacterRPC(
        joinedPage,
        campaignId,
        characterRpcBody('PLAYER', { ...pensantus, name: 'Segundo Personagem' }),
      );
      expect(second.status()).toBe(400); // failed_precondition
      expect(await second.json()).toMatchObject({ code: 'failed_precondition' });

      // Only the master marks a player character dead (RPC table, plan §4).
      const deadRes = await callRPC(page, 'meurpg.characters.v1.CharacterService/MarkCharacterDead', {
        campaignId,
        characterId: firstCharacter.id,
      });
      expect(deadRes.status()).toBe(200);

      // Now a new one is allowed, and the dead one stays in the list.
      const replacement = await createCharacterRPC(
        joinedPage,
        campaignId,
        characterRpcBody('PLAYER', { ...pensantus, name: 'Personagem Novo' }),
      );
      expect(replacement.status()).toBe(200);

      const list = await callRPC(joinedPage, 'meurpg.characters.v1.CharacterService/ListCharacters', { campaignId });
      const names = ((await list.json()).characters as { name: string }[]).map((c) => c.name);
      expect(names).toEqual(expect.arrayContaining([pensantus.name, 'Personagem Novo']));
    } finally {
      await playerContext.close();
    }
  },
);

test.fixme('as notas do mestre nunca chegam ao jogador', { tag: '@RN-11' }, async ({ page, browser }) => {
  // enabled when GetMasterNotes/UpdateMasterNotes (character_master_notes,
  // master-only) land. The old app let the player read and edit these — a
  // known, never-fixed bug there; this test is the proof it doesn't exist
  // here.
  await signIn(page, 'Mestre Teste', '/');
  const campaignId = await createCampaign(page, `Notas do mestre ${Date.now()}`);
  const npc = await createCharacterRPC(page, campaignId, characterRpcBody('ENEMY', { ...pensantus, name: 'Vilão com Segredo' }));
  const npcId = (await npc.json()).character.id as string;

  const secretNote = `Segredo do mestre ${Date.now()}: o vilão é o irmão do Pensantus.`;
  const notesRes = await callRPC(page, 'meurpg.characters.v1.CharacterService/UpdateMasterNotes', {
    campaignId,
    characterId: npcId,
    notes: secretNote,
  });
  expect(notesRes.status()).toBe(200);

  await page.getByRole('button', { name: 'Gerar convite' }).click();
  const link = (await page.locator('.invite-reveal__link').textContent())?.trim();

  const playerContext = await browser.newContext();
  try {
    const playerPage = await playerContext.newPage();
    await signIn(playerPage, 'Jogador Teste', '/');
    const { page: joinedPage } = await acceptInvite(playerContext, link!);

    // Every response body the player's page receives, across the campaign
    // screen and a direct attempt at the notes RPC: the secret string must
    // never appear in any of them.
    const bodyPromises: Promise<string>[] = [];
    joinedPage.on('response', (response) => {
      bodyPromises.push(response.text().catch(() => ''));
    });

    await joinedPage.goto(`/campanhas/${campaignId}`);
    const getNotesRes = await callRPC(joinedPage, 'meurpg.characters.v1.CharacterService/GetMasterNotes', {
      campaignId,
      characterId: npcId,
    });
    expect(getNotesRes.status()).toBe(403); // permission_denied

    const bodies = await Promise.all(bodyPromises);
    expect(bodies.some((body) => body.includes(secretNote))).toBe(false);
  } finally {
    await playerContext.close();
  }
});
