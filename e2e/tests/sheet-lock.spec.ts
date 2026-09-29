import { expect, test } from '@playwright/test';

import { acceptInvite, callRPC, characterRpcBody, createCampaign, createCharacterRPC, pensantus, signIn, startGameSession } from './support';

// MR-006 / RN-01 (trava da ficha): the player edits their own sheet freely
// until the campaign's first session starts; after that, only the master
// and the system (the play stub's LockSheets, run inside StartGameSession)
// change it. The character's descriptive story (amendment A3, as corrected
// by the integrator on 29/09/2026) is separate: once the sheet locks, the
// player can no longer edit the story either, until the master grants it
// per character through the new master-only
// CharacterService.SetStoryEditing(campaignId, characterId, allowed) —
// buttons "Permitir editar a história" / "Travar a história" on the
// master's view of the sheet. The master can always edit the story. None of
// this exists yet — WP-B (rules), WP-C (characters/play backend) and WP-D
// (web) are building it in parallel branches (see
// /scratchpad/etapa4-plan.md). Every test here is `test.fixme`; each
// one-line comment says what unblocks it.

test.fixme(
  'antes de qualquer sessão, o jogador edita a própria ficha e a alteração é salva',
  { tag: ['@MR-006', '@RN-01'] },
  async ({ page, browser }) => {
    // enabled when CharacterService.CreateCharacter/UpdateCharacter land
    // (no game_sessions row for the campaign yet, so state is DRAFT).
    await signIn(page, 'Mestre Teste', '/');
    const campaignId = await createCampaign(page, `Ficha rascunho ${Date.now()}`);
    await page.getByRole('button', { name: 'Gerar convite' }).click();
    const link = (await page.locator('.invite-reveal__link').textContent())?.trim();

    const playerContext = await browser.newContext();
    try {
      const playerPage = await playerContext.newPage();
      await signIn(playerPage, 'Jogador Teste', '/');
      const { page: joinedPage } = await acceptInvite(playerContext, link!);

      const created = await createCharacterRPC(joinedPage, campaignId, characterRpcBody('PLAYER', pensantus));
      const character = (await created.json()).character;
      expect(character.canEdit).toBe(true);

      await joinedPage.goto(`/campanhas/${campaignId}/personagens/${character.id}`);
      await expect(joinedPage.getByText('Ficha travada desde', { exact: false })).toHaveCount(0);
      await joinedPage.getByRole('button', { name: 'Editar ficha' }).click();
      await joinedPage.getByLabel('Nível').fill('4');
      await joinedPage.getByRole('button', { name: 'Salvar ficha' }).click();

      // Saved: a fresh GetCharacter shows the new level and a higher
      // revision (optimistic concurrency, AIP-154 style, plan §4).
      const getRes = await callRPC(joinedPage, 'meurpg.characters.v1.CharacterService/GetCharacter', {
        campaignId,
        characterId: character.id,
      });
      const saved = (await getRes.json()).character;
      expect(saved.revision).toBeGreaterThan(character.revision);
    } finally {
      await playerContext.close();
    }
  },
);

test.fixme(
  'depois que a primeira sessão começa, o jogador vê a ficha só para leitura, o servidor recusa a edição e o mestre edita',
  { tag: ['@MR-006', '@RN-01'] },
  async ({ page, browser }) => {
    // enabled when PlayService.StartGameSession locks every unlocked player
    // character in the campaign (the play stub, plan §4 "How play and
    // characters meet") and UpdateCharacter checks sheet_locked_at.
    await signIn(page, 'Mestre Teste', '/');
    const campaignId = await createCampaign(page, `Ficha travada ${Date.now()}`);
    await page.getByRole('button', { name: 'Gerar convite' }).click();
    const link = (await page.locator('.invite-reveal__link').textContent())?.trim();

    const playerContext = await browser.newContext();
    try {
      const playerPage = await playerContext.newPage();
      await signIn(playerPage, 'Jogador Teste', '/');
      const { page: joinedPage } = await acceptInvite(playerContext, link!);

      const created = await createCharacterRPC(joinedPage, campaignId, characterRpcBody('PLAYER', pensantus));
      const character = (await created.json()).character;

      const startRes = await startGameSession(page, campaignId);
      expect(startRes.status()).toBe(200);
      // StartGameSession's response reports how many sheets it locked (WP-A
      // fact, 29/09/2026): just this one player character.
      expect((await startRes.json()).lockedSheetCount).toBe(1);

      // The screen: read-only, with the lock banner, no edit button.
      await joinedPage.goto(`/campanhas/${campaignId}/personagens/${character.id}`);
      await expect(joinedPage.getByText('Ficha travada desde', { exact: false })).toBeVisible();
      await expect(joinedPage.getByRole('button', { name: 'Editar ficha' })).toHaveCount(0);

      // The server: refuses too, not just the screen (defense in depth),
      // with a CharacterBlocked detail (reason SHEET_LOCKED — WP-A fact,
      // 29/09/2026; the detail's JSON `debug` field carries the reason, per
      // connect-go's error_writer.go, but the exact enum wire value isn't
      // confirmed, so only the detail's `type` is asserted here).
      const updateRes = await callRPC(joinedPage, 'meurpg.characters.v1.CharacterService/UpdateCharacter', {
        campaignId,
        characterId: character.id,
        revision: character.revision,
        name: character.name,
        sheet: character.sheet,
      });
      expect(updateRes.status()).toBe(400); // failed_precondition (Connect's HTTP mapping)
      const updateBody = await updateRes.json();
      expect(updateBody).toMatchObject({ code: 'failed_precondition' });
      expect(updateBody.details?.[0]?.type).toBe('meurpg.characters.v1.CharacterBlocked');

      // The master: still edits the same, now-locked sheet.
      const masterUpdateRes = await callRPC(page, 'meurpg.characters.v1.CharacterService/UpdateCharacter', {
        campaignId,
        characterId: character.id,
        revision: character.revision,
        name: character.name,
        sheet: character.sheet,
      });
      expect(masterUpdateRes.status()).toBe(200);
    } finally {
      await playerContext.close();
    }
  },
);

test.fixme(
  'depois da primeira sessão, o jogador só edita a história quando o mestre permite',
  { tag: '@RN-01' },
  async ({ page, browser }) => {
    // Amendment A3, corrected by the integrator (Vinicius) on 29/09/2026:
    // RN-01's lock is for game data; the story is personal data
    // (docs/privacidade.md: it keeps the right to rectification), but after
    // the lock the player no longer edits it freely — the master grants
    // editing per character, through the new master-only
    // CharacterService.SetStoryEditing(campaignId, characterId, allowed).
    // "Editar história" / "Salvar história" work for the player while the
    // sheet is a draft, or after the master has allowed it; the master can
    // always edit the story, through the same UpdateCharacterStory RPC used
    // for the sheet-open case in the test above. Enabled when
    // SetStoryEditing, UpdateCharacterStory and the story editor land.
    await signIn(page, 'Mestre Teste', '/');
    const campaignId = await createCampaign(page, `História sob permissão ${Date.now()}`);
    await page.getByRole('button', { name: 'Gerar convite' }).click();
    const link = (await page.locator('.invite-reveal__link').textContent())?.trim();

    const playerContext = await browser.newContext();
    try {
      const playerPage = await playerContext.newPage();
      await signIn(playerPage, 'Jogador Teste', '/');
      const { page: joinedPage } = await acceptInvite(playerContext, link!);

      const created = await createCharacterRPC(joinedPage, campaignId, characterRpcBody('PLAYER', pensantus));
      let current = (await created.json()).character;

      await startGameSession(page, campaignId);

      // Locked, no permission yet: the player's own attempt is refused.
      const refusedRes = await callRPC(joinedPage, 'meurpg.characters.v1.CharacterService/UpdateCharacterStory', {
        campaignId,
        characterId: current.id,
        revision: current.revision,
        story: { backstory: 'Tentativa sem permissão do mestre.' },
      });
      expect(refusedRes.status()).toBe(400); // failed_precondition
      expect(await refusedRes.json()).toMatchObject({ code: 'failed_precondition' });

      // The master allows it, from their own view of the sheet.
      await page.goto(`/campanhas/${campaignId}/personagens/${current.id}`);
      await page.getByRole('button', { name: 'Permitir editar a história' }).click();
      const allowRes = await callRPC(page, 'meurpg.characters.v1.CharacterService/SetStoryEditing', {
        campaignId,
        characterId: current.id,
        allowed: true,
      });
      expect(allowRes.status()).toBe(200);

      // Now the player edits and saves, through the screen and the RPC.
      await joinedPage.goto(`/campanhas/${campaignId}/personagens/${current.id}`);
      await joinedPage.getByRole('button', { name: 'Editar história' }).click();
      // "Antecedentes" (the backstory field's label) is this file's guess —
      // not in the plan's §5 contract, confirm with WP-D.
      await joinedPage.getByLabel('Antecedentes').fill('Pensantus cresceu em Mirathel, entre livros e engrenagens.');
      await joinedPage.getByRole('button', { name: 'Salvar história' }).click();

      const storyRes = await callRPC(joinedPage, 'meurpg.characters.v1.CharacterService/UpdateCharacterStory', {
        campaignId,
        characterId: current.id,
        revision: current.revision,
        story: { backstory: 'Pensantus cresceu em Mirathel, entre livros e engrenagens.' },
      });
      expect(storyRes.status()).toBe(200);
      // Tracks the bumped revision (plan §4: UpdateCharacterStory bumps the
      // same `revision` column as UpdateCharacter) so the next write below
      // isn't refused for a stale revision instead of the lock this test
      // means to prove; the response shape is a guess, matching every other
      // mutating RPC's `{ character: {...} }` convention in this codebase.
      current = (await storyRes.json()).character;

      // The master locks it again.
      await page.getByRole('button', { name: 'Travar a história' }).click();
      const lockRes = await callRPC(page, 'meurpg.characters.v1.CharacterService/SetStoryEditing', {
        campaignId,
        characterId: current.id,
        allowed: false,
      });
      expect(lockRes.status()).toBe(200);

      const finalAttemptRes = await callRPC(joinedPage, 'meurpg.characters.v1.CharacterService/UpdateCharacterStory', {
        campaignId,
        characterId: current.id,
        revision: current.revision,
        story: { backstory: 'Segunda tentativa, depois da trava de novo.' },
      });
      expect(finalAttemptRes.status()).toBe(400); // failed_precondition
      expect(await finalAttemptRes.json()).toMatchObject({ code: 'failed_precondition' });
    } finally {
      await playerContext.close();
    }
  },
);
