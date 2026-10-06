import { expect, test, type Browser, type Page } from '@playwright/test';

import { openSessionPage } from './live-session-support';
import {
  BELL_NAMES,
  CIPHER,
  RIDDLE,
  SEQUENCE,
  createCipherRPC,
  createRiddleRPC,
  createSequenceRPC,
  endTable,
  masterRunJson,
  playSequenceRPC,
  playerRunText,
  puzzleRoute,
  revealClueRPC,
  sceneClueRPC,
  secondPlayer,
  setDiceModeRPC,
  showPuzzleRPC,
  tableForPuzzles,
  trapPointRPC,
} from './puzzles-support';
import { setCurrentMapRPC } from './maps-support';
import { newSignedInContext } from './support';

// More puzzles on screen (Etapa 10, slice 10.15b: MR-038, RN-27, RN-10, RN-18; E10-12): the riddle, the sequence and the cipher, the
// skill check that wins a hint, the split information and "Ao errar". The puzzles come through the API where the screen is not what is
// under test; the tests read what a player's response carries, as the app reads it, to prove no answer, sequence, key or other player's
// part ever leaves the server.

test.describe.configure({ timeout: 300_000 });

/** What a player's response must never carry (RN-10, RN-27), for every kind. */
const NEVER = ['"solution"', '"minimum"', '"path"', '"onSolve"', '"on_solve"', '"hintCheck"', '"dc"', '"onWrong"', '"attempts"'];

async function twoPeople(browser: Browser, viewport = { width: 1280, height: 1000 }, colorScheme: 'light' | 'dark' = 'light') {
  const masterContext = await newSignedInContext(browser, 'Mestre Teste', { viewport: { width: 1280, height: 1000 }, colorScheme });
  const playerContext = await newSignedInContext(browser, 'Jogador Teste', { viewport, colorScheme });
  const master = await masterContext.newPage();
  const player = await playerContext.newPage();
  await Promise.all([master.goto('/'), player.goto('/')]);
  return { master, player, close: () => Promise.all([masterContext.close(), playerContext.close()]) };
}

/** The player's page on a puzzle, once its title is there. */
async function playerOpens(player: Page, campaignId: string, puzzleId: string, name: string): Promise<void> {
  await player.goto(`/campanhas/${campaignId}/sessao?quebra-cabeca=${puzzleId}`);
  await expect(player.getByRole('heading', { level: 1, name })).toBeVisible({ timeout: 30_000 });
}

test('o enigma: o mestre o faz no formulário, o jogador erra e depois acerta, e nenhuma resposta chega a ele @MR-038 @RN-27 @RN-10', async ({ browser }) => {
  const { master, player, close } = await twoPeople(browser);
  const table = await tableForPuzzles(master, player, `Enigma ${Date.now()}`);
  try {
    // The master's form: the riddle, the accepted answers one by one, and "Ao errar": two attempts for each player.
    await master.goto(puzzleRoute(table.campaignId, 'quebra-cabecas', 'novo'));
    await master.getByRole('radio', { name: /^Enigma/ }).check();
    await master.getByLabel('Nome').fill('A porta da Cripta pergunta');
    await master.getByLabel('O enigma').fill(RIDDLE.text);
    await master.getByLabel('Pista para os jogadores').fill('Procure no chão da Cripta.');
    for (const answer of RIDDLE.answers) {
      await master.getByLabel(/^(Uma|Outra) resposta$/).fill(answer);
      await master.getByRole('button', { name: 'Adicionar', exact: true }).click();
    }
    await expect(master.getByRole('button', { name: 'Tirar a resposta a sombra' })).toBeVisible();
    await master.getByRole('radio', { name: 'Gastar uma tentativa do jogador' }).check();
    await master.getByRole('button', { name: 'Menos tentativa' }).click();
    await master.getByRole('button', { name: 'Criar quebra-cabeça' }).click();
    await expect(master).toHaveURL(puzzleRoute(table.campaignId));
    await expect(master.getByRole('region', { name: 'Quebra-cabeças' }).getByText('Enigma · 2 respostas aceitas')).toBeVisible();

    await openSessionPage(master, table.campaignId);
    await master.getByRole('button', { name: 'Mostrar aos jogadores A porta da Cripta pergunta' }).click();
    const card = master.getByRole('article', { name: 'A porta da Cripta pergunta' });
    await expect(card.getByText('Os jogadores veem')).toBeVisible();
    // Only the master reads the answers, each time with "Só você vê".
    await expect(card.getByText('Respostas aceitas')).toBeVisible();
    await expect(card.getByRole('list', { name: 'Respostas aceitas' }).getByRole('listitem')).toHaveText(['sombra', 'a sombra']);

    await openSessionPage(player, table.campaignId);
    await expect(player.getByText('O mestre mostrou um quebra-cabeça')).toBeVisible({ timeout: 30_000 });
    await player.getByRole('link', { name: 'Abrir o quebra-cabeça' }).click();
    await expect(player.getByRole('heading', { level: 1, name: 'A porta da Cripta pergunta' })).toBeVisible();
    await expect(player.getByText(RIDDLE.text)).toBeVisible();
    await expect(player.getByText('Suas tentativas')).toBeVisible();
    const puzzleId = new URL(player.url()).searchParams.get('quebra-cabeca')!;
    const before = await playerRunText(player, table.campaignId, puzzleId);
    for (const answer of RIDDLE.answers) {
      expect(before).not.toContain(answer);
    }
    for (const secret of NEVER) {
      expect(before, secret).not.toContain(secret);
    }

    // A wrong answer: the words and the icon, how many attempts are left, and never how close it was.
    await player.getByLabel('Sua resposta').fill('escuridão');
    await player.getByRole('button', { name: 'Responder' }).click();
    const wrong = player.getByRole('alert').filter({ hasText: 'Não é isso.' });
    await expect(wrong).toContainText('Tente outra resposta.');
    await expect(player.locator('app-limit-counters')).toContainText('Suas tentativas 1 de 2');
    // What the server keeps of it is the master's: the player's own response has neither the typed answer nor a list of answers.
    const after = await playerRunText(player, table.campaignId, puzzleId);
    expect(after).not.toContain('escuridão');
    for (const answer of RIDDLE.answers) {
      expect(after).not.toContain(answer);
    }
    expect(after).toContain('"wrong":true');
    // The master reads the answer and the attempts left, by player.
    await expect(card.getByText(/Última jogada: Pensantus tentou “escuridão”: errou/)).toBeVisible();
    await expect(card.getByText('Tentativas dele: 1 de 2.')).toBeVisible();
    await expect(card.getByText('Pensantus 1 de 2')).toBeVisible();

    // The right answer, with other capitals and punctuation: solved, for the table.
    await player.getByLabel('Sua resposta').fill('  A Sombra! ');
    await player.getByRole('button', { name: 'Responder' }).click();
    await expect(player.getByText('Resolvido.', { exact: false }).first()).toBeVisible();
    await expect(player.getByText('O enigma foi respondido.')).toBeVisible();
    await expect(player.getByLabel('Sua resposta')).toHaveCount(0);
    await expect(card.getByText('Pensantus resolveu “A porta da Cripta pergunta”', { exact: false })).toBeVisible();
    const solved = await playerRunText(player, table.campaignId, puzzleId);
    for (const answer of RIDDLE.answers) {
      expect(solved).not.toContain(answer);
    }
  } finally {
    await endTable(master, table.campaignId);
    await close();
  }
});

test('a sequência: o jogador vê tocar passo a passo e repete; um passo errado dispara a armadilha @MR-038 @RN-27 @RN-10 @MR-035', async ({ browser }) => {
  const { master, player, close } = await twoPeople(browser);
  const table = await tableForPuzzles(master, player, `Sequência ${Date.now()}`);
  try {
    await setCurrentMapRPC(master, table.campaignId, table.map.mapId);
    const trap = await trapPointRPC(master, table.campaignId, table.map.mapId, 'Dardos envenenados');
    const puzzleId = await createSequenceRPC(master, table.campaignId, 'Os sinos do Salão do trono', {
      clue: 'Quem toca os sinos escuta o trono.',
      onWrong: { trap: { mapId: table.map.mapId, pointId: trap } },
    });
    await showPuzzleRPC(master, table.campaignId, puzzleId);
    await openSessionPage(master, table.campaignId);
    const card = master.getByRole('article', { name: 'Os sinos do Salão do trono' });
    // The master reads the whole sequence ("Só você vê"); the players have not seen it play.
    await expect(card.getByRole('list', { name: 'A sequência, passo a passo' }).getByRole('listitem')).toHaveCount(6);
    await expect(card.getByText('Os jogadores ainda não viram a sequência tocar')).toBeVisible();

    await playerOpens(player, table.campaignId, puzzleId, 'Os sinos do Salão do trono');
    await expect(player.getByText('O mestre ainda não tocou os sinos.')).toBeVisible();
    // Before the first play the bells do nothing (the server refuses): the app shows them still.
    await expect(player.getByRole('button', { name: 'Sino alto' })).toHaveAttribute('aria-disabled', 'true');

    // "Tocar a sequência": each phone shows it step by step, and the server sends only the steps already played.
    await card.getByRole('button', { name: 'Tocar a sequência' }).click();
    await expect(player.getByText('O mestre está tocando os sinos.')).toBeVisible({ timeout: 15_000 });
    const mid = JSON.parse(await playerRunText(player, table.campaignId, puzzleId)).run.sequence as { shown?: number[]; playing?: boolean; totalSteps: number };
    expect(mid.totalSteps).toBe(6);
    expect((mid.shown ?? []).length).toBeLessThan(6);
    await expect(player.getByText(/passo [2-6] de 6/)).toBeVisible({ timeout: 15_000 });
    await expect(player.locator('.big__name')).toBeVisible();
    // No bell to tap while it plays; when it ends it is "com vocês".
    await expect(player.getByRole('button', { name: 'Sino alto' })).toHaveCount(0);
    await expect(player.getByText('Agora é com vocês.')).toBeVisible({ timeout: 30_000 });
    await expect(card.getByText('Os jogadores já viram a sequência tocar 1 vez.')).toBeVisible();
    const after = await playerRunText(player, table.campaignId, puzzleId);
    // The config says how many steps there are ("steps":6); never which bell each one is.
    expect(after).not.toMatch(/"steps":\[/);
    for (const secret of NEVER) {
      expect(after, secret).not.toContain(secret);
    }
    // The players never receive the sequence again once the play ends: nothing of it is in what they read now.
    expect(JSON.parse(after).run.sequence.shown ?? []).toEqual([]);

    // A wrong first bell: "Errou o passo 1", the attempt starts over, the trap fires, and the master says so too.
    await player.getByRole('button', { name: 'Sino largo' }).click();
    await expect(player.locator('.board-card').getByRole('alert').filter({ hasText: 'Errou o passo 1.' })).toContainText('A tentativa recomeçou; você errou.');
    await expect(card.getByText(/A armadilha disparou: Dardos envenenados\. Foi o erro de Pensantus\./)).toBeVisible();
    await expect(card.getByText(/Última jogada: Pensantus errou no passo 1\. A tentativa recomeçou/)).toBeVisible();
    // The trap that fired is public to who sees its map (the master put it on the player's screen), with the name the master gave it.
    await expect(player.getByText('A armadilha disparou:')).toBeVisible();
    await expect(player.getByText('Dardos envenenados.', { exact: false })).toBeVisible();

    // Repeat it right, bell by bell: the count of right steps is the server's, and the last one solves it.
    for (const [i, bell] of SEQUENCE.entries()) {
      await player.getByRole('button', { name: BELL_NAMES[bell] }).click();
      if (i < SEQUENCE.length - 1) {
        await expect(player.locator('.board-card')).toContainText(`${i + 1} de 6`);
      }
    }
    await expect(player.getByText('Os sinos tocaram na ordem certa.')).toBeVisible();
    await expect(card.getByText('Pensantus resolveu “Os sinos do Salão do trono”', { exact: false })).toBeVisible();
    expect((await masterRunJson(master, table.campaignId, puzzleId)).run).toBeTruthy();
  } finally {
    await endTable(master, table.campaignId);
    await close();
  }
});

test('a cifra: a chave é uma pista da cena que o grupo acha, e o jogador decifra à mão @MR-038 @RN-27 @RN-10 @MR-029', async ({ browser }) => {
  const { master, player, close } = await twoPeople(browser);
  const table = await tableForPuzzles(master, player, `Cifra ${Date.now()}`);
  try {
    const clueText = 'Cada letra anda três para trás.';
    const clueId = await sceneClueRPC(master, table.campaignId, table.map.mapId, 'A biblioteca', clueText);

    // The master's form: the server ciphers the message for him ("Como os jogadores a veem"), and the key is a scene clue.
    await master.goto(puzzleRoute(table.campaignId, 'quebra-cabecas', 'novo'));
    await master.getByRole('radio', { name: /^Cifra/ }).check();
    await master.getByLabel('Nome').fill('A carta do Capitão');
    await master.getByLabel('Mensagem', { exact: true }).fill(CIPHER.message);
    await expect(master.locator('app-cipher-form .cipher')).toHaveText(CIPHER.ciphertext, { timeout: 30_000 });
    await master.getByLabel('Qual pista guarda a chave').selectOption(clueId);
    await master.getByRole('button', { name: 'Criar quebra-cabeça' }).click();
    await expect(master).toHaveURL(puzzleRoute(table.campaignId));

    await openSessionPage(master, table.campaignId);
    await master.getByRole('button', { name: 'Mostrar aos jogadores A carta do Capitão' }).click();
    const card = master.getByRole('article', { name: 'A carta do Capitão' });
    await expect(card.locator('.cipher')).toHaveText(CIPHER.ciphertext);
    await expect(card.getByText(`A mensagem: ${CIPHER.message}`)).toBeVisible();

    await openSessionPage(player, table.campaignId);
    await player.getByRole('link', { name: 'Abrir o quebra-cabeça' }).click();
    await expect(player.getByRole('heading', { level: 1, name: 'A carta do Capitão' })).toBeVisible();
    const puzzleId = new URL(player.url()).searchParams.get('quebra-cabeca')!;
    await expect(player.locator('.cipher')).toHaveText(CIPHER.ciphertext);
    // The letter, a column for each of its letters, and the field: the table is the player's own helper.
    await expect(player.locator('.col__bet')).toHaveCount(9);
    await expect(player.getByText('Ainda não acharam a chave.')).toBeVisible();
    const lost = await playerRunText(player, table.campaignId, puzzleId);
    expect(lost).toContain('"hasKeyClue":true');
    expect(lost).not.toContain(clueId);
    expect(lost).not.toContain('Cada letra anda');
    expect(lost).not.toContain('"shift"');
    expect(lost).not.toContain('"keyword"');
    expect(lost).not.toContain(CIPHER.message);
    for (const secret of NEVER) {
      expect(lost, secret).not.toContain(secret);
    }

    // The group finds the key in the adventure (the master gives the clue to Pensantus): it is in the player's notes.
    await revealClueRPC(master, table.campaignId, clueId, [table.characterId]);
    await expect(player.getByText('Pista achada na aventura')).toBeVisible({ timeout: 30_000 });
    const found = await playerRunText(player, table.campaignId, puzzleId);
    expect(found).toContain('"keyClueId"');
    expect(found).not.toContain(CIPHER.message);
    await player.locator('.key').getByRole('button', { name: 'Abrir as notas' }).click();
    await expect(player.getByText(clueText).first()).toBeVisible();
    await player.keyboard.press('Escape');

    // Decoding by hand: the table is never read; a wrong message says so, a right one (without accents or capitals) solves it.
    await player.locator('.col__bet').first().fill('a');
    await player.getByLabel('A mensagem decifrada').fill('o tesouro esta sobre o altar');
    await player.getByRole('button', { name: 'Conferir' }).click();
    await expect(player.getByRole('alert').filter({ hasText: 'Não é isso.' })).toContainText('Confira as letras da tabela.');
    await expect(card.getByText(/Última jogada: Pensantus digitou “o tesouro esta sobre o altar”: errou/)).toBeVisible();
    await player.getByLabel('A mensagem decifrada').fill('O TESOURO ESTA SOB O ALTAR');
    await player.getByRole('button', { name: 'Conferir' }).click();
    await expect(player.getByText('A mensagem foi decifrada.')).toBeVisible();
    await expect(card.getByText('Pensantus resolveu “A carta do Capitão”', { exact: false })).toBeVisible();
  } finally {
    await endTable(master, table.campaignId);
    await close();
  }
});

test('a dica por teste de perícia com dado físico: o jogador digita o d20, falha, e depois ganha uma dica só dele @MR-038 @RN-18 @RN-27 @RN-10', async ({ browser }) => {
  const { master, player, close } = await twoPeople(browser);
  const table = await tableForPuzzles(master, player, `Dica ${Date.now()}`);
  const toren = await secondPlayer(browser, master, table.campaignId);
  try {
    await setDiceModeRPC(master, table.campaignId, 'DICE_MODE_PHYSICAL');
    const puzzleId = await createRiddleRPC(master, table.campaignId, 'A porta da Cripta pergunta', {
      hints: ['Pense no que acompanha você ao meio-dia.', 'Ela some quando a tocha apaga.'],
      hintCheck: { skillKey: 'skill:investigation', dc: 13 },
    });
    await showPuzzleRPC(master, table.campaignId, puzzleId);
    await openSessionPage(master, table.campaignId);
    const card = master.getByRole('article', { name: 'A porta da Cripta pergunta' });

    await playerOpens(player, table.campaignId, puzzleId, 'A porta da Cripta pergunta');
    // The button names the skill and never the DC; with physical dice the player types the d20.
    const go = player.getByRole('button', { name: 'Tentar uma dica · Investigação' });
    await expect(go).toBeVisible();
    await expect(player.getByText('A mesa usa dados físicos: você digita o resultado.')).toBeVisible();
    const text = await playerRunText(player, table.campaignId, puzzleId);
    expect(text).toContain('"hintByCheck":true');
    expect(text).toContain('skill:investigation');
    for (const secret of NEVER) {
      expect(text, secret).not.toContain(secret);
    }
    expect(await player.locator('body').innerText()).not.toMatch(/\bCD\b/);

    // A 1 fails: "Não deu desta vez.", with no number to beat, and no second try for the same hint.
    await go.click();
    await expect(player.getByText('Teste de Investigação. Role o seu d20 e digite o dado, sem o bônus.')).toBeVisible();
    await player.getByLabel('O d20 que você rolou').fill('1');
    await player.getByRole('button', { name: 'Confirmar 1' }).click();
    await expect(player.getByText('Não deu desta vez.')).toBeVisible();
    await expect(player.getByText('Outro jogador pode tentar, ou o mestre solta uma dica.')).toBeVisible();
    await expect(player.getByRole('button', { name: /^Tentar uma dica/ })).toHaveCount(0);
    expect(await player.locator('body').innerText()).not.toMatch(/\bCD\b/);
    // The master reads the roll, and who.
    await expect(card.getByText(/Pensantus rolou \d+( \(d20: 1\))? para a dica 1: não passou\./)).toBeVisible();

    // The master releases hint 1 to everyone; now the next hint can be tried: a 20 passes and the hint is the player's alone.
    await card.getByRole('button', { name: 'Mostrar a próxima dica' }).click();
    await expect(player.getByText('Pense no que acompanha você ao meio-dia.')).toBeVisible();
    await player.getByRole('button', { name: 'Tentar uma dica · Investigação' }).click();
    await player.getByLabel('O d20 que você rolou').fill('20');
    await player.getByRole('button', { name: 'Confirmar 20' }).click();
    await expect(player.getByText('Você conseguiu.')).toBeVisible();
    await expect(player.getByText('Esta dica é só sua; se quiser, conte aos outros.')).toBeVisible();
    await expect(player.getByText('Ela some quando a tocha apaga.')).toBeVisible();
    await expect(card.getByText(/Pensantus rolou \d+( \(d20: 20\))? para a dica 2: passou\./)).toBeVisible();

    // The other player reads the hint the master released, never the one Pensantus won (RN-27).
    const torensText = await playerRunText(toren.page, table.campaignId, puzzleId);
    expect(torensText).toContain('Pense no que acompanha você ao meio-dia.');
    expect(torensText).not.toContain('Ela some quando a tocha apaga.');
    const own = JSON.parse(await playerRunText(player, table.campaignId, puzzleId)).run as { hints: string[]; sharedHints: number };
    expect(own.hints).toEqual(['Pense no que acompanha você ao meio-dia.', 'Ela some quando a tocha apaga.']);
    expect(own.sharedHints).toBe(1);
  } finally {
    await toren.close();
    await endTable(master, table.campaignId);
    await close();
  }
});

test('a informação dividida: o mestre dá uma parte a cada jogador e cada um só recebe a sua @MR-038 @RN-27 @RN-10', async ({ browser }) => {
  const { master, player, close } = await twoPeople(browser);
  const table = await tableForPuzzles(master, player, `Partes ${Date.now()}`);
  const toren = await secondPlayer(browser, master, table.campaignId);
  try {
    const mine = '“…os tambores ecoam três vezes antes de a porta ceder.”';
    const hers = '“A porta ouve o que o chão esconde…”';
    // The master's form: two parts, each for one player's character.
    await master.goto(puzzleRoute(table.campaignId, 'quebra-cabecas', 'novo'));
    await master.getByRole('radio', { name: /^Enigma/ }).check();
    await master.getByLabel('Nome').fill('A porta da Cripta pergunta');
    await master.getByLabel('O enigma').fill(RIDDLE.text);
    await master.getByLabel(/^(Uma|Outra) resposta$/).fill('sombra');
    await master.getByRole('button', { name: 'Adicionar', exact: true }).click();
    await master.getByRole('button', { name: 'Adicionar parte' }).click();
    await master.getByLabel('Para quem (parte 1)').selectOption(table.characterId);
    await master.getByLabel('O que ele lê (parte 1)').fill(mine.replace(/[“”]/g, ''));
    await master.getByRole('button', { name: 'Adicionar parte' }).click();
    await master.getByLabel('Para quem (parte 2)').selectOption(toren.characterId);
    await master.getByLabel('O que ele lê (parte 2)').fill(hers.replace(/[“”]/g, ''));
    // A character has one part: Pensantus is no longer offered on the second.
    await expect(master.getByLabel('Para quem (parte 2)').locator('option', { hasText: 'Pensantus' })).toBeDisabled();
    await master.getByRole('button', { name: 'Criar quebra-cabeça' }).click();
    await expect(master).toHaveURL(puzzleRoute(table.campaignId));

    await openSessionPage(master, table.campaignId);
    await master.getByRole('button', { name: 'Mostrar aos jogadores A porta da Cripta pergunta' }).click();
    await expect(master.getByRole('article', { name: 'A porta da Cripta pergunta' }).getByText('Os jogadores veem')).toBeVisible();

    await openSessionPage(player, table.campaignId);
    await player.getByRole('link', { name: 'Abrir o quebra-cabeça' }).click();
    await expect(player.getByRole('heading', { level: 1, name: 'A porta da Cripta pergunta' })).toBeVisible();
    const puzzleId = new URL(player.url()).searchParams.get('quebra-cabeca')!;
    // Pensantus's phone: his own part, labelled as only his, and who else has one (a name, never the text).
    await expect(player.getByRole('region', { name: 'A sua parte da pista' })).toContainText(mine.replace(/[“”]/g, ''));
    await expect(player.getByText('Só você vê esta parte.')).toBeVisible();
    await expect(player.getByRole('region', { name: 'Quem mais tem uma parte' })).toContainText('Toren');
    await expect(player.getByText('A porta ouve o que o chão esconde')).toHaveCount(0);

    // The JSON each player reads holds only that player's part (RN-27).
    const pensantus = await playerRunText(player, table.campaignId, puzzleId);
    expect(pensantus).toContain('os tambores ecoam três vezes');
    expect(pensantus).not.toContain('A porta ouve o que o chão esconde');
    expect(pensantus).toContain('"partHolders":["Toren"]');
    const torens = await playerRunText(toren.page, table.campaignId, puzzleId);
    expect(torens).toContain('A porta ouve o que o chão esconde');
    expect(torens).not.toContain('os tambores ecoam três vezes');
    expect(torens).toContain('"partHolders":["Pensantus"]');
    for (const text of [pensantus, torens]) {
      for (const secret of NEVER) {
        expect(text, secret).not.toContain(secret);
      }
      expect(text).not.toContain('"parts"');
      expect(text).not.toContain(table.characterId);
      expect(text).not.toContain(toren.characterId);
    }

    // Toren's phone shows his own.
    await playerOpens(toren.page, table.campaignId, puzzleId, 'A porta da Cripta pergunta');
    await expect(toren.page.getByRole('region', { name: 'A sua parte da pista' })).toContainText('A porta ouve o que o chão esconde');
    await expect(toren.page.getByText('os tambores ecoam três vezes')).toHaveCount(0);
  } finally {
    await toren.close();
    await endTable(master, table.campaignId);
    await close();
  }
});

test('o limite de jogadas para o quebra-cabeça: o jogador lê "parou", os contadores, e o mestre recomeça @MR-038 @RN-27', async ({ browser }) => {
  const { master, player, close } = await twoPeople(browser);
  const table = await tableForPuzzles(master, player, `Limite ${Date.now()}`);
  try {
    const puzzleId = await createRiddleRPC(master, table.campaignId, 'A porta da Cripta pergunta', { onWrong: { maxMoves: 2 } });
    await showPuzzleRPC(master, table.campaignId, puzzleId);
    await openSessionPage(master, table.campaignId);
    const card = master.getByRole('article', { name: 'A porta da Cripta pergunta' });
    await playerOpens(player, table.campaignId, puzzleId, 'A porta da Cripta pergunta');
    await expect(player.locator('app-limit-counters')).toContainText('Jogadas 0 de 2');

    await player.getByLabel('Sua resposta').fill('escuridão');
    await player.getByRole('button', { name: 'Responder' }).click();
    await expect(player.locator('app-limit-counters')).toContainText('Jogadas 1 de 2');
    await player.getByLabel('Sua resposta').fill('luz');
    await player.getByRole('button', { name: 'Responder' }).click();

    // The second move reached the limit: nothing moves, the players read it neutrally, and the counter says so in words.
    await expect(player.getByText('O quebra-cabeça parou.')).toBeVisible();
    await expect(player.getByText('O mestre decide o que acontece agora.')).toBeVisible();
    await expect(player.locator('app-limit-counters')).toContainText('Jogadas 2 de 2 · acabou');
    await expect(player.getByLabel('Sua resposta')).toHaveCount(0);
    const stopped = JSON.parse(await playerRunText(player, table.campaignId, puzzleId)).run as { stopped?: boolean };
    expect(stopped.stopped).toBe(true);
    // The server refuses a move on a stopped puzzle all the same.
    const refused = await player.request.post('/meurpg.play.v1.PuzzleService/MakePuzzleMove', {
      data: { campaignId: table.campaignId, puzzleId, move: { riddle: { answer: 'sombra' } }, idempotencyKey: crypto.randomUUID() },
      headers: { 'Connect-Protocol-Version': '1' },
    });
    expect(refused.status()).toBe(400);
    // The master reads why, and "Recomeçar" gives the table its moves back.
    await expect(card.getByText('O limite de jogadas foi atingido: ninguém joga mais até você recomeçar ou fechar.')).toBeVisible();
    await expect(card.getByText('Parou')).toBeVisible();
    await card.getByRole('button', { name: 'Recomeçar' }).click();
    await card.locator('app-map-ask').getByRole('button', { name: 'Recomeçar', exact: true }).click();
    await expect(player.getByLabel('Sua resposta')).toBeVisible({ timeout: 30_000 });
    await expect(player.locator('app-limit-counters')).toContainText('Jogadas 0 de 2');
    await expect(player.getByText('O quebra-cabeça parou.')).toHaveCount(0);
  } finally {
    await endTable(master, table.campaignId);
    await close();
  }
});

test('os três quebra-cabeças novos cabem em 320 × 568 e o campo e "Responder" ficam à vista @MR-038', async ({ browser }) => {
  const { master, player, close } = await twoPeople(browser, { width: 320, height: 568 }, 'dark');
  const table = await tableForPuzzles(master, player, `Cabe ${Date.now()}`);
  try {
    const riddle = await createRiddleRPC(master, table.campaignId, 'A porta da Cripta pergunta', { clue: 'Procure no chão da Cripta.', onWrong: { attemptsPerPlayer: 3 } });
    const sequence = await createSequenceRPC(master, table.campaignId, 'Os sinos do Salão do trono');
    const cipher = await createCipherRPC(master, table.campaignId, 'A carta do Capitão', { onWrong: { maxMoves: 10, timeLimitSeconds: 300 } });
    for (const id of [riddle, sequence, cipher]) {
      await showPuzzleRPC(master, table.campaignId, id);
    }
    const fits = () =>
      player.evaluate(() => ({
        scrolls: document.documentElement.scrollWidth > document.documentElement.clientWidth,
        small: Array.from(document.querySelectorAll<HTMLElement>('main button, main input, main textarea'))
          .filter((e) => e.getBoundingClientRect().width > 0)
          .filter((e) => Math.min(e.getBoundingClientRect().width, e.getBoundingClientRect().height) < 44 && !(e instanceof HTMLTextAreaElement))
          .map((e) => `${e.tagName}:${e.getAttribute('aria-label') ?? e.textContent?.trim()}`),
      }));

    await playerOpens(player, table.campaignId, riddle, 'A porta da Cripta pergunta');
    // The riddle rolls and the field and "Responder" stay stuck at the bottom of the screen.
    await expect(player.getByRole('button', { name: 'Responder' })).toBeInViewport();
    await expect(player.getByLabel('Sua resposta')).toBeInViewport();
    expect((await fits()).scrolls).toBe(false);

    await playerOpens(player, table.campaignId, sequence, 'Os sinos do Salão do trono');
    await playSequenceRPC(master, table.campaignId, sequence);
    await expect(player.getByText('Agora é com vocês.')).toBeVisible({ timeout: 30_000 });
    expect(await fits()).toEqual({ scrolls: false, small: [] });

    await playerOpens(player, table.campaignId, cipher, 'A carta do Capitão');
    expect(await fits()).toEqual({ scrolls: false, small: [] });
    await expect(player.getByRole('button', { name: 'Conferir' })).toBeVisible();
  } finally {
    await endTable(master, table.campaignId);
    await close();
  }
});
