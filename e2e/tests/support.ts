import { expect, type APIResponse, type BrowserContext, type Page } from '@playwright/test';

// devidp's issuer (deploy/local/compose.yaml). The browser resolves any
// *.localhost name to 127.0.0.1 by itself.
export const idpOrigin = process.env.E2E_IDP_ORIGIN ?? 'http://idp.localhost:9090';

export const sessionCookie = '__Host-meurpg_session';
export const loginCookie = '__Host-meurpg_login';

/** The test users devidp lists on its login page (oidctest.TestUsers). */
export type TestUser = 'Mestre Teste' | 'Jogador Teste';

/**
 * Signs in through devidp, the way a person would: open the sign-in URL,
 * pick a user on the provider's page, and land back on returnTo.
 *
 * It opens /auth/login directly, which is faster and keeps these tests about
 * the server. ui.spec.ts covers the same flow through the app's buttons.
 */
export async function signIn(page: Page, user: TestUser = 'Mestre Teste', returnTo = '/'): Promise<void> {
  await page.goto(`/auth/login?return_to=${encodeURIComponent(returnTo)}`);
  await expect(page).toHaveURL((url) => url.origin === idpOrigin && url.pathname === '/authorize');
  await page.getByRole('button', { name: user, exact: true }).click();
  // A relative URL is resolved against baseURL: back on the app, at returnTo.
  await expect(page).toHaveURL(returnTo);
}

/**
 * Calls a unary Connect RPC with the JSON codec, through the page's own
 * cookies. Every Connect call must carry Connect-Protocol-Version: 1 (the
 * server's CSRF protection, docs/arquitetura.md#csrf).
 *
 * Use it for what the page does not show; ui.spec.ts asserts on the page.
 */
export function callRPC(page: Page, method: string, body: object = {}): Promise<APIResponse> {
  return page.request.post(`/${method}`, {
    data: body,
    headers: { 'Connect-Protocol-Version': '1' },
  });
}

export const getMe = (page: Page) => callRPC(page, 'meurpg.identity.v1.IdentityService/GetMe');
export const signOut = (page: Page) => callRPC(page, 'meurpg.identity.v1.IdentityService/SignOut');

export const day = 24 * 60 * 60 * 1000;

/**
 * Creates a campaign through the "Nova campanha" form on `/campanhas`, the
 * way MR-001 asks for, and returns its id from the `/campanhas/<id>` URL
 * the app navigates to afterwards.
 *
 * Tests whose own story is not campaign creation (MR-002, MR-003) use this
 * as setup, so their assertions stay about invites and membership, not
 * about the form they don't need to prove again.
 */
export async function createCampaign(page: Page, name: string): Promise<string> {
  await page.goto('/campanhas');
  await page.getByLabel('Nome da campanha').fill(name);
  await page.getByLabel('Modo de XP').click();
  await page.getByRole('option', { name: 'Por inimigos derrotados' }).click();
  await page.getByRole('button', { name: 'Criar campanha' }).click();

  await expect(page).toHaveURL(/\/campanhas\/[^/]+$/);
  return page.url().split('/').pop()!;
}

// --- Etapa 4: characters, rules and the play stub --------------------------
//
// The character screens and RPCs do not exist yet: WP-B (rules), WP-C
// (characters/play backend) and WP-D (web) are building them in parallel
// branches. Every Etapa 4 spec that uses these helpers is `test.fixme` for
// now (see characters.spec.ts, sheet-lock.spec.ts, character-rules.spec.ts,
// credits.spec.ts). These helpers are written against the contract fixed in
// /scratchpad/etapa4-plan.md (§4 RPC names, §5 field/button labels, §6
// helper signatures and the pensantus fixture), so the specs compile now and
// need no rewrite, only their `test.fixme` removed, once the feature lands.
//
// Two kinds of names below carry different confidence:
//   - RPC service/method names and route paths come straight from the plan
//     (an agreed contract with WP-A/WP-C/WP-D) and should not need changing.
//   - Portuguese option text for content keys (race, class, subclass,
//     skills) is this file's own best guess at the `name_pt` values WP-B's
//     SRD translation will produce, and the stepper's "next step" button
//     label ("Próximo") is not in the plan's label contract at all. Both are
//     called out again next to their use, and belong in the integrator's
//     open questions until WP-B/WP-D confirm or correct them.

/** One class/race/background build, used by both `createCharacterViaUI` (the
 * screen) and `createCharacterRPC` (direct API setup) to create the same
 * character two ways. */
export interface CharacterBuild {
  name: string;
  /** Content key's `name_pt`, e.g. "Gnomo" for `race:gnome`. */
  race: string;
  subrace?: string;
  class: string;
  subclass?: string;
  level: number;
  /** Custom background name (ADR-0008: the SRD only has "Acólito"/Acolyte).
   * Omitted picks the SRD's "Acólito" instead of "Outro (personalizado)". */
  background?: string;
  backgroundSkills: string[];
  /** Skill proficiencies chosen beyond the background (e.g. the class's
   * skill picks). */
  extraSkills: string[];
  scores: { for: number; des: number; con: number; int: number; sab: number; car: number };
}

/**
 * The Pensantus reference build (Rock Gnome Wizard 3, School of Evocation,
 * custom background "Sábio"): Gnomo/Gnomo da Rocha, Mago 3/Evocação,
 * background "Sábio" (Arcana + História), class skills Investigação and
 * Intuição, scores FOR 12 / DES 16 / CON 15 / INT 16 / SAB 13 / CAR 12.
 *
 * The build itself (scores, level, background name, chosen skills) is fixed
 * by the Etapa 4 plan (§6) and the private ADR-0008 (not reproduced here:
 * see docs/adr/ in the main clone, gitignored in worktrees). The race,
 * subrace, class, subclass and skill *strings* below are this file's own
 * guess at the Portuguese `name_pt` values the rules content will use —
 * confirm against WP-B's SRD snapshot before enabling the specs that use
 * this fixture.
 *
 * `pensantusDerived` (below) records the numbers MR-004's spec checks
 * against the server's `GetCharacter` response, from the same source: INT 18
 * after the +2 racial bonus gives modifier +4; proficiency +2 gives spell
 * save DC 14 and spell attack +6; HP 23 (the hit die's max at level 1, plus
 * two levels of the fixed average, plus the +3 CON modifier each level); AC
 * 13 unarmored (10 + the +3 DEX modifier, no shield or armor worn).
 */
export const pensantus: CharacterBuild = {
  name: 'Pensantus',
  race: 'Gnomo', // race:gnome — name_pt unconfirmed
  subrace: 'Gnomo da Rocha', // subrace:rock-gnome — name_pt unconfirmed
  class: 'Mago', // class:wizard — name_pt unconfirmed
  subclass: 'Evocação', // subclass:evocation — name_pt unconfirmed
  level: 3,
  background: 'Sábio', // custom background, not an SRD content key (ADR-0008)
  backgroundSkills: ['Arcana', 'História'], // skill:arcana, skill:history — name_pt unconfirmed
  extraSkills: ['Investigação', 'Intuição'], // skill:investigation, skill:insight — name_pt unconfirmed
  scores: { for: 12, des: 16, con: 15, int: 16, sab: 13, car: 12 },
};

/** The server-computed numbers MR-004's spec checks for the `pensantus`
 * build, from the same source as the fixture above. */
export const pensantusDerived = {
  intModifier: '+4',
  spellSaveDc: 14,
  spellAttackBonus: '+6',
  armorClass: 13,
  hitPointsMax: 23,
};

/**
 * Opens an invite link in a new page of the given browser context, the way a
 * signed-in person accepting an invite would, and waits for the app to land
 * on the campaign it belongs to (MR-003's "o mestre já vê" half). The
 * context must already be signed in (see `signIn`); the signed-out path is
 * its own flow, already covered directly in invite.spec.ts.
 */
export async function acceptInvite(context: BrowserContext, link: string): Promise<{ page: Page; campaignId: string }> {
  const page = await context.newPage();
  await page.goto(link);
  await expect(page).toHaveURL(/\/campanhas\/[^/]+$/);
  return { page, campaignId: page.url().split('/').pop()! };
}

/**
 * Creates a player character through the "Criar personagem" screen
 * (`/campanhas/:id/personagens/novo`), filling the stepper with `build`
 * (default: `pensantus`) using the field and button labels the Etapa 4 plan
 * fixes in §5. The stepper's step layout (Básico, Atributos, Perícias,
 * Magias, Equipamento, História — plan §5) and its "next step" button label
 * are NOT in that contract: this helper assumes a button named "Próximo"
 * advances a step, and clicks it until "Criar personagem" (the last step's
 * submit button) is visible, skipping Magias/Equipamento/História since no
 * Etapa 4 acceptance criterion needs them filled. Confirm the "Próximo"
 * label with WP-D once the editor exists.
 *
 * Returns the created character's id, read from the sheet page's URL after
 * submit (`/campanhas/:id/personagens/:characterId`).
 */
export async function createCharacterViaUI(page: Page, campaignId: string, build: CharacterBuild = pensantus): Promise<string> {
  await page.goto(`/campanhas/${campaignId}/personagens/novo`);

  // Passo "Básico".
  await page.getByLabel('Nome do personagem').fill(build.name);
  await page.getByLabel('Raça').click();
  await page.getByRole('option', { name: build.race }).click();
  if (build.subrace) {
    await page.getByLabel('Sub-raça').click();
    await page.getByRole('option', { name: build.subrace }).click();
  }
  await page.getByLabel('Classe').click();
  await page.getByRole('option', { name: build.class }).click();
  if (build.subclass) {
    await page.getByLabel('Subclasse').click();
    await page.getByRole('option', { name: build.subclass }).click();
  }
  await page.getByLabel('Nível').fill(String(build.level));
  await page.getByLabel('Antecedente').click();
  await page.getByRole('option', { name: build.background ? 'Outro (personalizado)' : 'Acólito' }).click();
  if (build.background) {
    await page.getByLabel('Nome do antecedente').fill(build.background);
  }
  await page.getByRole('button', { name: 'Próximo' }).click();

  // Passo "Atributos".
  await page.getByLabel('Força').fill(String(build.scores.for));
  await page.getByLabel('Destreza').fill(String(build.scores.des));
  await page.getByLabel('Constituição').fill(String(build.scores.con));
  await page.getByLabel('Inteligência').fill(String(build.scores.int));
  await page.getByLabel('Sabedoria').fill(String(build.scores.sab));
  await page.getByLabel('Carisma').fill(String(build.scores.car));
  await page.getByRole('button', { name: 'Próximo' }).click();

  // Passo "Perícias".
  for (const skill of [...build.backgroundSkills, ...build.extraSkills]) {
    await page.getByRole('checkbox', { name: skill }).check();
  }

  // Skip Magias/Equipamento/História: click through to the submit button.
  for (let i = 0; i < 5 && !(await page.getByRole('button', { name: 'Criar personagem' }).isVisible()); i++) {
    await page.getByRole('button', { name: 'Próximo' }).click();
  }
  await page.getByRole('button', { name: 'Criar personagem' }).click();

  await expect(page).toHaveURL(/\/campanhas\/[^/]+\/personagens\/[^/]+$/);
  return page.url().split('/').pop()!;
}

/**
 * Creates a character directly through the API
 * (`meurpg.characters.v1.CharacterService/CreateCharacter`), bypassing the
 * editor screen. For specs whose acceptance criterion isn't character
 * creation itself (MR-005, MR-006/RN-01, RN-03, RN-11): they need a
 * character to exist, not a second proof that the form works.
 *
 * `body` is sent as-is next to `campaignId` (protojson accepts both
 * camelCase and proto_name fields); pass at least `kind` and `name`, and a
 * `sheet` for a full-sheet kind. See `characterRpcBody` for a body built
 * from a `CharacterBuild`.
 */
export function createCharacterRPC(page: Page, campaignId: string, body: Record<string, unknown>): Promise<APIResponse> {
  return callRPC(page, 'meurpg.characters.v1.CharacterService/CreateCharacter', { campaignId, ...body });
}

/**
 * Builds a `CreateCharacter` request body from a `CharacterBuild`, using
 * content keys (`race:gnome`, `class:wizard`, ...) rather than the
 * `name_pt` strings `createCharacterViaUI` selects by — content keys are
 * fixed by the plan's `Build` Go type (§3) and far more stable than the
 * translated labels. Field names (`baseScores`, `customBackground`, ...)
 * follow §4's FullSheet shape but are this file's best guess at the actual
 * protojson field names pending WP-A's generated client; expect to adjust
 * them once `web/src/gen` exists.
 */
export function characterRpcBody(kind: 'PLAYER' | 'ENEMY' | 'BOSS' | 'MINION' | 'STORY', build: CharacterBuild): Record<string, unknown> {
  return {
    kind: `CHARACTER_KIND_${kind}`,
    name: build.name,
    sheet: {
      full: {
        baseScores: {
          str: build.scores.for,
          dex: build.scores.des,
          con: build.scores.con,
          int: build.scores.int,
          wis: build.scores.sab,
          cha: build.scores.car,
        },
        race: 'race:gnome',
        subrace: build.subrace ? 'subrace:rock-gnome' : undefined,
        classes: [{ class: 'class:wizard', subclass: build.subclass ? 'subclass:evocation' : undefined, level: build.level }],
        customBackground: build.background ? { name: build.background, skills: ['skill:arcana', 'skill:history'] } : undefined,
        skillProficiencies: ['skill:investigation', 'skill:insight'],
        hitPoints: { fixed: {} },
      },
    },
  };
}

/**
 * Starts a game session for a campaign
 * (`meurpg.play.v1.PlayService/StartGameSession`), the RN-01 lock trigger
 * (MR-006): starting a session locks, in the same transaction, every
 * unlocked player character in the campaign (Etapa 4 plan, amendment A5/V1,
 * the `play` stub). Master-only.
 *
 * Defaults to the campaign in the page's current URL
 * (`/campanhas/<id>/...`), so a spec already on the campaign page can just
 * call `startGameSession(page)`, as the plan's helper list (§6) has it; pass
 * `campaignId` explicitly from any other page.
 */
export async function startGameSession(page: Page, campaignId?: string): Promise<APIResponse> {
  const id = campaignId ?? page.url().match(/\/campanhas\/([^/]+)/)?.[1];
  if (!id) {
    throw new Error('startGameSession: no campaignId given, and none found in the current page URL');
  }
  return callRPC(page, 'meurpg.play.v1.PlayService/StartGameSession', { campaignId: id });
}
