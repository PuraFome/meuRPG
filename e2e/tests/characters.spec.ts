import { expect, test } from '@playwright/test';

import {
  acceptInvite,
  callRPC,
  characterRpcBody,
  createCampaign,
  createCharacterRPC,
  createCharacterViaUI,
  pensantus,
  pensantusDerived,
  signIn,
} from './support';

// Etapa 4 (characters, rules): the "Criar meu personagem" CTA, the sheet
// screen and the NPC screens do not exist yet — WP-B (rules), WP-C
// (characters backend) and WP-D (web) are building them in parallel
// branches (see /scratchpad/etapa4-plan.md). Every test here is
// `test.fixme`, so the suite stays green; each one-line comment says what
// unblocks it. Labels and RPC names come from the plan's §4/§5 contract;
// where this file guesses (the "name_pt" option text in `support.ts`'s
// `pensantus` fixture, protojson field names), it says so.

test.fixme(
  'o jogador entra pelo convite, cria o personagem e o mestre o vê na campanha',
  { tag: '@MR-003' },
  async ({ page, browser }) => {
    // enabled when CharacterService.CreateCharacter (kind PLAYER), the
    // "Criar meu personagem" CTA and the character editor land.
    await signIn(page, 'Mestre Teste', '/');
    const campaignId = await createCampaign(page, `Personagens ${Date.now()}`);
    await page.getByRole('button', { name: 'Gerar convite' }).click();
    await expect(page.getByText('não será mostrado de novo')).toBeVisible();
    const link = (await page.locator('.invite-reveal__link').textContent())?.trim();

    const playerContext = await browser.newContext();
    try {
      const playerPage = await playerContext.newPage();
      await signIn(playerPage, 'Jogador Teste', '/');
      const { page: joinedPage } = await acceptInvite(playerContext, link!);

      // No living character yet: the CTA offers creating one (plan §5,
      // campaign-characters.ts).
      await joinedPage.getByRole('button', { name: 'Criar meu personagem' }).click();
      await expect(joinedPage).toHaveURL(new RegExp(`/campanhas/${campaignId}/personagens/novo$`));

      const characterId = await createCharacterViaUI(joinedPage, campaignId, pensantus);
      expect(characterId).toBeTruthy();
      await expect(joinedPage.getByRole('heading', { level: 1 })).toContainText(pensantus.name);
    } finally {
      await playerContext.close();
    }

    // Back on the master's page: reload to see the new character.
    await page.reload();
    await expect(page.getByRole('heading', { name: 'Personagens dos jogadores' })).toBeVisible();
    await expect(page.getByText(pensantus.name)).toBeVisible();
  },
);

test.fixme(
  'o jogador abre a ficha no celular e vê as seções da ficha oficial com os valores calculados pelo servidor',
  { tag: '@MR-004' },
  async ({ browser }) => {
    // enabled when CharacterService.GetCharacter, rules.Derive (WP-B) and
    // the character-sheet screen (WP-D) land. Runs in a Chromium mobile
    // viewport, not a real device: Safari rejects __Host- cookies on
    // http://localhost (plan §10, e2e risks).
    const masterContext = await browser.newContext();
    const playerContext = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true });
    try {
      const masterPage = await masterContext.newPage();
      await signIn(masterPage, 'Mestre Teste', '/');
      const campaignId = await createCampaign(masterPage, `Ficha mobile ${Date.now()}`);
      await masterPage.getByRole('button', { name: 'Gerar convite' }).click();
      const link = (await masterPage.locator('.invite-reveal__link').textContent())?.trim();

      const playerPage = await playerContext.newPage();
      await signIn(playerPage, 'Jogador Teste', '/');
      const { page: joinedPage } = await acceptInvite(playerContext, link!);

      // Created through the API, not the editor: this test's criterion is
      // the sheet screen, already proven separately by the @MR-003 test.
      const created = await createCharacterRPC(joinedPage, campaignId, characterRpcBody('PLAYER', pensantus));
      expect(created.status()).toBe(200);
      const characterId = (await created.json()).character.id as string;

      await joinedPage.goto(`/campanhas/${campaignId}/personagens/${characterId}`);

      // The official sheet's sections (plan §5, character-sheet.ts, mobile
      // order), each an <h2>.
      for (const section of ['Atributos', 'Combate', 'Perícias', 'Magias', 'Equipamento', 'Características e traços', 'História']) {
        await expect(joinedPage.getByRole('heading', { level: 2, name: section })).toBeVisible();
      }

      // The numbers come from the server's DerivedSheet, never recalculated
      // in the browser (plan §5: "o navegador nunca calcula uma regra").
      await expect(joinedPage.getByText(pensantusDerived.intModifier, { exact: false })).toBeVisible();
      await expect(joinedPage.getByText('CD de magia', { exact: false })).toBeVisible();
      await expect(joinedPage.getByText(String(pensantusDerived.spellSaveDc), { exact: false })).toBeVisible();
      await expect(joinedPage.getByText(pensantusDerived.spellAttackBonus, { exact: false })).toBeVisible();
      await expect(joinedPage.getByText(String(pensantusDerived.armorClass), { exact: false })).toBeVisible();
      await expect(joinedPage.getByText(String(pensantusDerived.hitPointsMax), { exact: false })).toBeVisible();
    } finally {
      await masterContext.close();
      await playerContext.close();
    }
  },
);

test.fixme(
  'o mestre cria um inimigo com ficha completa e um minion com ficha básica',
  { tag: '@MR-005' },
  async ({ page }) => {
    // enabled when CharacterService.CreateCharacter (NPC kinds) and the
    // "Novo NPC" menu land. The NPC creation form's own field labels are not
    // yet in the plan's §5 contract (only the player character editor's
    // are), so this creates through the API and asserts the screen's NPC
    // list; the form itself needs its own spec once its labels are agreed.
    await signIn(page, 'Mestre Teste', '/');
    const campaignId = await createCampaign(page, `NPCs ${Date.now()}`);

    const enemy = await createCharacterRPC(
      page,
      campaignId,
      characterRpcBody('ENEMY', { ...pensantus, name: 'Goblin Chefe' }),
    );
    expect(enemy.status()).toBe(200);

    const minion = await createCharacterRPC(page, campaignId, {
      kind: 'CHARACTER_KIND_MINION',
      name: 'Bandido',
      // BasicSheet fields (plan §4): hitPointsMax, armorClass, speedFt,
      // attackBonus, damage, description — protojson names unconfirmed.
      sheet: { basic: { hitPointsMax: 4, armorClass: 12, speedFt: 30 } },
    });
    expect(minion.status()).toBe(200);

    await page.reload();
    await expect(page.getByRole('heading', { name: 'NPCs', level: 2 })).toBeVisible();
    await expect(page.getByText('Goblin Chefe')).toBeVisible();
    await expect(page.getByText('Bandido')).toBeVisible();
  },
);

test.fixme('o jogador não vê os NPCs da campanha', { tag: '@MR-005' }, async ({ page, browser }) => {
  // enabled with the same pieces as the test above, plus ListCharacters and
  // GetCharacter hiding NPCs from a player (RN-04, authz), same "not_found"
  // pattern as a hidden campaign (ADR-0011, docs/arquitetura.md).
  await signIn(page, 'Mestre Teste', '/');
  const campaignId = await createCampaign(page, `NPCs escondidos ${Date.now()}`);
  const npc = await createCharacterRPC(page, campaignId, characterRpcBody('ENEMY', { ...pensantus, name: 'Vilão Secreto' }));
  const npcId = (await npc.json()).character.id as string;

  await page.getByRole('button', { name: 'Gerar convite' }).click();
  const link = (await page.locator('.invite-reveal__link').textContent())?.trim();

  const playerContext = await browser.newContext();
  try {
    const playerPage = await playerContext.newPage();
    await signIn(playerPage, 'Jogador Teste', '/');
    const { page: joinedPage } = await acceptInvite(playerContext, link!);

    await expect(joinedPage.getByRole('heading', { name: 'NPCs', level: 2 })).toHaveCount(0);
    await expect(joinedPage.getByText('Vilão Secreto')).toHaveCount(0);

    // The RPC itself hides it too, exactly like a campaign the caller is
    // not a member of: `not_found`, never `permission_denied`, so no NPC id
    // leaks.
    const getRes = await callRPC(joinedPage, 'meurpg.characters.v1.CharacterService/GetCharacter', {
      campaignId,
      characterId: npcId,
    });
    expect(getRes.status()).toBe(404);
    expect(await getRes.json()).toMatchObject({ code: 'not_found' });
  } finally {
    await playerContext.close();
  }
});
