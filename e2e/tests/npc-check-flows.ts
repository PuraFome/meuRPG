import { expect, type Locator } from '@playwright/test';

import { openSessionPage } from './live-session-support';
import { expectDashed, expectFits } from './take-out-support';
import { callRPC } from './support';
import {
  beginCombatRPC,
  bardReady,
  cutSheet,
  masterRolls,
  npcCard,
  npcSheet,
  openNpcSheet,
  pickNpc,
  rollChosen,
  see,
  typeD20,
  type NpcTable,
  type NpcView,
} from './npc-check-support';

// The screens and states of the NPC check outside a combat (#591), as flows: each takes a table and a view, asserts what the
// boards say and, when the view has them, scans and shoots each screen. The flow spec runs them with an empty view, the a11y spec
// with the axe scan and the shots at every width and theme.

const CUT = /Você cortou: −\d/;
const FORMULA = /d20 \(\d+\) [+−] \d+ = \d+/;

/** The sheet "Teste de um NPC": who rolls, the box, the d20 typed, the creature with no number and the empty form (boards 591-01, 591-07, 591-08 states 2 and 6). */
export async function sheetFlow(t: NpcTable, v: NpcView): Promise<void> {
  const { m } = t;
  const sheet = await openNpcSheet(m, t.campaignId);
  await expect(sheet.getByText('Fora de combate · a rolagem é só sua')).toBeVisible();
  // Nothing chosen: the roll is dashed and says what is missing, above it.
  const roll = sheet.getByRole('button', { name: 'Rolar o teste' });
  await expectDashed(roll);
  await expect(sheet.getByText('Escolha quem rola e o teste.')).toBeVisible();
  await see(v, m, 'sheet-empty');
  for (const group of ['Em cena', 'No mapa', 'Do bestiário']) {
    await expect(sheet.getByRole('heading', { name: group })).toBeVisible();
  }
  await expect(sheet.getByText('Sem bloco de estatísticas: rola com +0')).toHaveCount(1);
  await expect(sheet.getByText('Escondido dos jogadores')).toHaveCount(1);

  // A creature the players see: the box starts ticked, and the roll names the check.
  await pickNpc(sheet, 'Capitão bandido');
  const box = sheet.getByRole('checkbox', { name: /Os bardos veem a criatura e ela os ouve/ });
  await expect(box).toBeChecked();
  await expect(sheet.getByRole('button', { name: 'Rolar Enganação do Capitão bandido' })).toBeVisible();
  await see(v, m, 'sheet', sheet);
  await expectFits(m, sheet, 'Teste de um NPC, a folha');

  // The hidden one starts unticked; ticked, the bards read "uma criatura".
  await pickNpc(sheet, 'Espião');
  await expect(box).not.toBeChecked();
  await expect(sheet.getByText(/está escondido dos jogadores, então a caixa começa desmarcada/)).toBeVisible();
  await expect(sheet.getByText('os bardos leem “uma criatura”, sem o nome', { exact: false })).toBeVisible();
  await see(v, m, 'sheet-hidden', sheet);

  // The map measures a creature with a token beside a bard's: no box, the quiet line, and the d20 typed.
  await pickNpc(sheet, 'Capitão Goblin');
  await expect(box).toHaveCount(0);
  await expect(sheet.getByText(/o app mede os 18 m e a visão/)).toBeVisible();
  await typeD20(sheet, '11');
  await see(v, m, 'sheet-typed', sheet);
  await sheet.getByLabel('Número que saiu no d20 (1 a 20)').fill('21');
  await expect(sheet.getByText('Digite um número de 1 a 20.')).toBeVisible();
  await expectDashed(sheet.getByRole('button', { name: /^Rolar .* do Capitão Goblin/ }));
  await expect(sheet.getByText('O d20 vai de 1 a 20.')).toBeVisible();
  await see(v, m, 'sheet-typed-invalid', sheet);

  // A creature with no number for the check: said beside the check, and in the select.
  await pickNpc(sheet, 'Bartolo');
  await expect(sheet.getByTestId('no-bonus')).toContainText(/Sem bônus para .*: o d20 conta com \+0 e você soma o bônus que achar justo\./);
  await expect(sheet.locator('option[data-side="sem bônus"]').first()).toBeAttached();
  await see(v, m, 'sheet-no-bonus', sheet);
}

/** The master rolls with the app's dice and the bard cuts with hers; the card follows, survives a reload and hides on this device (boards 591-02, 03, 05, 06). */
export async function appDiceFlow(t: NpcTable, v: NpcView): Promise<void> {
  const { m, p, campaignId } = t;
  await masterRolls(t, 'Capitão bandido', 'app');
  const card = npcCard(m);
  await expect(card.getByRole('heading', { name: 'Capitão bandido · Enganação' })).toBeVisible();
  await expect(card).toContainText(FORMULA);
  await expect(card).toContainText('Esperando Brisa');
  await expect(card).toContainText('Antes das Palavras de Interrupção: o total fecha quando os bardos responderem.');
  await expect(card.getByRole('button', { name: 'Continuar sem esperar' })).toBeVisible();
  await expect(card.getByRole('button', { name: 'Esconder o teste' })).toHaveCount(0);
  await see(v, m, 'card-waiting', card);
  // The check is not a question to answer for the roller.
  await expect(m.getByRole('heading', { name: 'Perguntas pendentes' })).toHaveCount(0);

  await bardReady(p, campaignId);
  const q = cutSheet(p);
  await expect(q.getByRole('heading', { name: 'O Capitão bandido faz um teste' })).toBeVisible();
  await expect(q).toContainText('Usar Palavras de Interrupção?');
  await expect(q).toContainText('Você tira o seu dado de Inspiração de Bardo (d8) do teste do Capitão bandido. O resultado ainda não foi dito.');
  for (const chip of ['Gasta 1 Inspiração de Bardo', 'Você tem 3', 'Não gasta reação']) {
    await expect(q.getByText(chip, { exact: true })).toBeVisible();
  }
  await expect(q).toContainText('Qual é o teste, o total e a CD ficam com o mestre.');
  await expect(q).toContainText('A cena espera a sua resposta. O mestre pode seguir sem ela.');
  // RN-10: nothing of the creature's roll, no clock, never "Sem efeito".
  await expect(q).not.toContainText(/Enganação|Sem efeito|d20|segundos/);
  await expect(q.getByRole('timer')).toHaveCount(0);
  await expect(p.getByRole('region', { name: /^Teste de um NPC: / })).toHaveCount(0);
  await see(v, p, 'bard-question-app');
  await expectFits(p, q, 'Palavras de Interrupção, a pergunta');
  await q.getByRole('button', { name: 'Cortar', exact: true }).click();
  await expect(q.getByRole('heading', { name: 'Palavras de Interrupção usadas' })).toBeVisible();
  await expect(q).toContainText(CUT);
  await expect(q).toContainText('Gastou 1 Inspiração de Bardo');
  await expect(q).toContainText('Restam 2');
  await expect(q).toContainText('O total do teste e o resultado ficam com o mestre, que conta na cena.');
  await expect(q).not.toContainText(/Enganação|\d+ = \d+/);
  await see(v, p, 'bard-result');
  await q.getByRole('button', { name: 'Voltar à cena' }).click();
  await expect(cutSheet(p)).toHaveCount(0);

  // The master's card is settled: who cut and what the number came to.
  await expect(card).toContainText(/Cortou \d/);
  await expect(card).toContainText(/\d+ − \d+ = [-−]?\d+/);
  await expect(card.getByRole('button', { name: 'Continuar sem esperar' })).toHaveCount(0);
  await expect(card).toContainText('Fica aqui até o próximo teste ou a sessão acabar.');
  await see(v, m, 'card-settled', card);

  // A reload brings it back; the "X" hides it on this device only, and a reload shows it again.
  await openSessionPage(m, campaignId);
  await expect(npcCard(m)).toContainText(/Cortou \d/);
  await npcCard(m).getByRole('button', { name: 'Esconder o teste' }).click();
  await expect(npcCard(m)).toHaveCount(0);
  await openSessionPage(m, campaignId);
  await expect(npcCard(m)).toBeVisible();
}

/** The master types a d20 and the bard types the face of her real die: "Cortar com 4" (boards 591-04, 591-07). */
export async function physicalDiceFlow(t: NpcTable, v: NpcView): Promise<void> {
  const { m, p, campaignId } = t;
  await masterRolls(t, 'Capitão bandido', '13');
  const card = npcCard(m);
  await expect(card).toContainText(/d20 \(13\) \+ \d+ = \d+/);
  await bardReady(p, campaignId);
  const q = cutSheet(p);
  await expect(q.getByLabel('Número que saiu no seu d8 (1 a 8)')).toBeVisible();
  await expect(q).toContainText('Role o seu d8 e digite o que saiu. Para deixar passar, não precisa rolar.');
  await expectDashed(q.getByRole('button', { name: 'Cortar', exact: true }));
  await see(v, p, 'bard-question-empty');
  await q.getByLabel('Número que saiu no seu d8 (1 a 8)').fill('9');
  await expect(q.getByText('Digite um número de 1 a 8.')).toBeVisible();
  await q.getByLabel('Número que saiu no seu d8 (1 a 8)').fill('4');
  await expect(q.getByRole('button', { name: 'Cortar com 4' })).toBeVisible();
  await see(v, p, 'bard-question-physical');
  await q.getByRole('button', { name: 'Cortar com 4' }).click();
  await expect(q).toContainText('Você cortou: −4');
  await expect(q).toContainText('O seu d8 saiu 4 e foi tirado do teste do Capitão bandido.');
  await q.getByRole('button', { name: 'Voltar à cena' }).click();
  await expect(card).toContainText('Cortou 4');
  await expect(card).toContainText(/d20 \(13\) \+ \d+, menos o corte de Brisa/);
}

/** The bard lets it pass: the sheet closes with one line on the scene, nothing is spent, and the card says "Deixou passar" (board 591-08 state 7). */
export async function passFlow(t: NpcTable, v: NpcView): Promise<void> {
  const { m, p, campaignId } = t;
  await masterRolls(t, 'Capitão bandido', '12');
  await bardReady(p, campaignId);
  const q = cutSheet(p);
  await expect(q.getByRole('button', { name: 'Deixar passar' })).toBeFocused();
  await q.getByRole('button', { name: 'Deixar passar' }).click();
  await expect(cutSheet(p)).toHaveCount(0);
  const note = p.getByRole('status').filter({ hasText: 'Você deixou passar. Nada foi gasto.' });
  await expect(note).toBeVisible();
  await see(v, p, 'bard-pass-note', note);
  const card = npcCard(m);
  await expect(card).toContainText('Deixou passar');
  await expect(card).toContainText('d8 · nada gasto');
  await expect(card).toContainText('Ninguém cortou o teste.');
  await see(v, m, 'card-passed', card);
  // Nothing was spent: the next check asks with the same three uses.
  await masterRolls(t, 'Capitão bandido', '15');
  await expect(cutSheet(p)).toContainText('Você tem 3');
}

/** Two bards, in order: Brisa cut, bilbo waits; the master goes on without him (boards 591-08 states 4 and 7). */
export async function twoBardsFlow(t: NpcTable, v: NpcView): Promise<void> {
  const { m, p, campaignId, other } = t;
  if (!other) {
    throw new Error('twoBardsFlow wants a second bard');
  }
  await masterRolls(t, 'Capitão bandido', '13');
  const card = npcCard(m);
  await expect(card).toContainText('Esperando Brisa');
  await expect(card).toContainText('Na fila');
  await expect(card).toContainText('d6 · depois de Brisa');
  await see(v, m, 'card-queue', card);
  // The order is the party's: bilbo reads nothing while Brisa answers.
  await bardReady(other.page, campaignId);
  await bardReady(p, campaignId);
  await expect(cutSheet(p)).toBeVisible();
  await expect(cutSheet(other.page)).toHaveCount(0);
  await expect(other.page.getByText(/Usar Palavras de Interrupção|faz um teste/)).toHaveCount(0);

  await cutSheet(p).getByRole('button', { name: 'Cortar', exact: true }).click();
  await cutSheet(p).getByRole('button', { name: 'Voltar à cena' }).click();
  const q = cutSheet(other.page);
  await expect(q.getByRole('heading', { name: 'O Capitão bandido faz um teste' })).toBeVisible();
  await expect(q).toContainText('(d6)');
  await expect(card).toContainText(/Cortou \d/);
  await expect(card).toContainText('Esperando bilbo');
  await expect(card).toContainText(/, por enquanto/);
  await see(v, m, 'card-two-bards', card);

  // "Continuar sem esperar": bilbo's open question says the master went on, and what he cut stays cut.
  await card.getByRole('button', { name: 'Continuar sem esperar' }).click();
  await expect(card).toContainText('Sem resposta');
  await expect(card).toContainText('d6 · você seguiu sem esperar · nada gasto');
  await expect(card).toContainText(/Cortou \d/);
  await expect(card.getByRole('button', { name: 'Esconder o teste' })).toBeVisible();
  await see(v, m, 'card-skipped', card);
  await expect(q).toContainText('O mestre seguiu sem a sua resposta. Nada foi gasto.');
  await expect(q.getByRole('button', { name: 'Cortar', exact: true })).toHaveCount(0);
  await see(v, other.page, 'bard-skipped');
  await q.getByRole('button', { name: 'Fechar' }).click();
  await expect(cutSheet(other.page)).toHaveCount(0);
}

/** The bard's answer reaches the server after the master went on: "Essa pergunta já foi respondida." (board 591-08 state 7). */
export async function lateAnswerFlow(t: NpcTable, v: NpcView): Promise<void> {
  const { m, p, campaignId, other } = t;
  if (!other) {
    throw new Error('lateAnswerFlow wants a second bard');
  }
  await masterRolls(t, 'Capitão bandido', '13');
  await bardReady(p, campaignId);
  await cutSheet(p).getByRole('button', { name: 'Deixar passar' }).click();
  await bardReady(other.page, campaignId);
  const q = cutSheet(other.page);
  await expect(q).toBeVisible();
  const read = await callRPC(m, 'meurpg.play.v1.PlayService/GetNpcCheck', { campaignId });
  const hold = (await read.json()).check.holdId as string;
  await other.page.route('**/*ResourceService/AnswerHeldRoll', async (route) => {
    // The master goes on between the bard's click and the server's answer.
    await callRPC(m, 'meurpg.play.v1.PlayService/SkipNpcCheckAsks', { campaignId, holdId: hold, idempotencyKey: crypto.randomUUID() });
    await route.continue();
  });
  await q.getByRole('button', { name: 'Cortar', exact: true }).click();
  await expect(q).toContainText('Essa pergunta já foi respondida. Nada foi gasto.');
  await see(v, other.page, 'bard-late');
  await other.page.unroute('**/*ResourceService/AnswerHeldRoll');
  await q.getByRole('button', { name: 'Fechar' }).click();
  await expect(npcCard(m)).toContainText('Sem resposta');
}

/** A creature the master hid: unticked, nobody is asked; ticked, the bard reads "uma criatura" (boards 591-08 states 3 and 6). */
export async function hiddenFlow(t: NpcTable, v: NpcView): Promise<void> {
  const { m, p, campaignId } = t;
  await bardReady(p, campaignId);
  await masterRolls(t, 'Espião', '14');
  const card = npcCard(m);
  await expect(card.getByRole('heading', { name: /^Espião · / })).toBeVisible();
  await expect(card.getByTestId('nobody')).toContainText('Ninguém foi perguntado');
  await expect(card.getByRole('button', { name: 'Continuar sem esperar' })).toHaveCount(0);
  await expect(card.getByRole('button', { name: 'Esconder o teste' })).toBeVisible();
  await see(v, m, 'card-nobody', card);
  await expect(cutSheet(p)).toHaveCount(0);

  await masterRolls(t, 'Espião', '14', true);
  const q = cutSheet(p);
  await expect(q.getByRole('heading', { name: 'Uma criatura faz um teste' })).toBeVisible();
  await expect(q).toContainText('do teste da criatura');
  await expect(q).not.toContainText('Espião');
  await see(v, p, 'bard-unnamed');
  await q.getByRole('button', { name: 'Deixar passar' }).click();
  await expect(npcCard(m)).toContainText('Deixou passar');
}

/** A creature with no known modifier rolls +0, and the screens say so (the card, the sheet beside the check). */
export async function noBonusFlow(t: NpcTable, v: NpcView): Promise<void> {
  const { m, p, campaignId } = t;
  await masterRolls(t, 'Bartolo', '9');
  const card = npcCard(m);
  await expect(card.getByRole('heading', { name: /^Bartolo · / })).toBeVisible();
  await expect(card).toContainText('d20 (9) + 0 = 9');
  await expect(card.getByTestId('no-bonus')).toContainText(/Sem bônus: Bartolo não tem número para .*\. O d20 contou com \+0: some o bônus que achar justo\./);
  await see(v, m, 'card-no-bonus', card);
  await bardReady(p, campaignId);
  await expect(cutSheet(p)).not.toContainText(/Sem bônus|\+0/);
  await cutSheet(p).getByRole('button', { name: 'Deixar passar' }).click();
  await expect(card.getByTestId('no-bonus')).toBeVisible();
  await expect(card).toContainText('Deixou passar');
  await see(v, m, 'card-no-bonus-settled', card);
}

/** Refusals: a combat began (the check is not rolled and the entry stays, dashed), no open session (boards 591-08 state 1). */
export async function refusalsFlow(t: NpcTable, v: NpcView): Promise<void> {
  const { m } = t;
  const detail = (type: string) => ({ code: 'failed_precondition', message: 'x', details: [{ type, value: 'CAE' }] });
  const sheet = await openNpcSheet(m, t.campaignId);
  await pickNpc(sheet, 'Capitão bandido');
  await m.route('**/*PlayService/RollNpcCheck', (route) => route.fulfill({ status: 400, contentType: 'application/json', body: JSON.stringify(detail('meurpg.play.v1.GameSessionBlocked')) }));
  await sheet.getByRole('button', { name: 'Rolar Enganação do Capitão bandido' }).click();
  await expect(sheet.getByRole('alert')).toContainText('Só numa sessão aberta. Nada foi rolado.');
  await see(v, m, 'refusal-session', sheet);
  // The combat is the last refusal: it is the one that leaves the entry dashed.
  await m.route('**/*PlayService/RollNpcCheck', (route) => route.fulfill({ status: 400, contentType: 'application/json', body: JSON.stringify(detail('meurpg.play.v1.ResourceBlocked')) }));
  await sheet.getByRole('button', { name: 'Rolar Enganação do Capitão bandido' }).click();
  const alert = sheet.getByRole('alert');
  await expect(alert).toContainText('Um combate começou.');
  await expect(alert).toContainText('O teste do Capitão bandido não foi rolado: os testes agora são do combate.');
  await see(v, m, 'refusal-combat', sheet);
  await m.unroute('**/*PlayService/RollNpcCheck');
  await sheet.getByRole('button', { name: 'Cancelar' }).click();
  await expect(npcSheet(m)).toHaveCount(0);
  // The entry stays in its place, dashed, with the reason beside it, and the card of nothing rolled is not drawn.
  const entry = m.getByRole('button', { name: 'Teste de um NPC' }).first();
  await expectDashed(entry);
  await expect(m.getByText('Há um combate aberto: os testes das criaturas são do combate.')).toBeVisible();
  await expect(npcCard(m)).toHaveCount(0);
  await see(v, m, 'entry-combat', entry);
}

/** A combat begins for real while the sheet is open: the server refuses the roll and nothing is rolled. */
export async function combatOpenFlow(t: NpcTable): Promise<void> {
  const { m, campaignId } = t;
  const sheet = await openNpcSheet(m, campaignId);
  await pickNpc(sheet, 'Capitão bandido');
  await beginCombatRPC(t);
  await sheet.getByRole('button', { name: 'Rolar Enganação do Capitão bandido' }).click();
  await expect(sheet.getByRole('alert')).toContainText('Um combate começou.');
  const res = await callRPC(m, 'meurpg.play.v1.PlayService/GetNpcCheck', { campaignId });
  expect((await res.json()).check).toBeUndefined();
}

/** The sheet's loading, its read error with "Tentar de novo", the roll that fails and "Rolando…" (board 591-08 state 8). */
export async function loadingFlow(t: NpcTable, v: NpcView): Promise<void> {
  const { m, campaignId } = t;
  await openSessionPage(m, campaignId);
  let release = (): void => undefined;
  const gate = new Promise<void>((resolve) => (release = resolve));
  let fail = true;
  await m.route('**/*PlayService/ListNpcCheckPicker', async (route) => {
    await gate;
    return fail ? route.fulfill({ status: 503, contentType: 'application/json', body: '{"code":"unavailable","message":"x"}' }) : route.continue();
  });
  await m.getByRole('button', { name: 'Teste de um NPC' }).first().click();
  const sheet = npcSheet(m);
  await expect(sheet.getByText('Lendo quem está em cena e no mapa…')).toBeVisible();
  await see(v, m, 'sheet-loading', sheet);
  release();
  await expect(sheet.getByRole('alert')).toContainText('Não deu para ler as criaturas.');
  await see(v, m, 'sheet-read-error', sheet);
  fail = false;
  await sheet.getByRole('button', { name: 'Tentar de novo' }).click();
  await expect(sheet.getByText('Capitão bandido').first()).toBeVisible();
  await m.unroute('**/*PlayService/ListNpcCheckPicker');

  await pickNpc(sheet, 'Capitão bandido');
  let go = (): void => undefined;
  const hold = new Promise<void>((resolve) => (go = resolve));
  await m.route('**/*PlayService/RollNpcCheck', async (route) => {
    await hold;
    await route.fulfill({ status: 503, contentType: 'application/json', body: '{"code":"unavailable","message":"x"}' });
  });
  await sheet.getByRole('button', { name: 'Rolar Enganação do Capitão bandido' }).click();
  const rolling = sheet.getByRole('button', { name: 'Rolando…' });
  await expect(rolling).toBeVisible();
  await expectDashed(rolling);
  await see(v, m, 'sheet-rolling', sheet);
  go();
  await expect(sheet.getByRole('alert')).toContainText('Não deu para rolar agora. Nada foi rolado. Tente de novo.');
  await see(v, m, 'sheet-roll-error', sheet);
  await m.unroute('**/*PlayService/RollNpcCheck');
  await expect(npcCard(m)).toHaveCount(0);
}

/** The bard's own failures: "Cortando…" while it is on the way, and "Não deu para cortar agora." (board 591-08 state 7). */
export async function cutFailureFlow(t: NpcTable, v: NpcView): Promise<void> {
  const { m, p, campaignId } = t;
  await masterRolls(t, 'Capitão bandido', '13');
  await bardReady(p, campaignId);
  const q = cutSheet(p);
  let release = (): void => undefined;
  const gate = new Promise<void>((resolve) => (release = resolve));
  await p.route('**/*ResourceService/AnswerHeldRoll', async (route) => {
    await gate;
    await route.fulfill({ status: 503, contentType: 'application/json', body: '{"code":"unavailable","message":"x"}' });
  });
  await q.getByRole('button', { name: 'Cortar', exact: true }).click();
  const sending = q.getByRole('button', { name: 'Cortando…' });
  await expect(sending).toBeVisible();
  await expectDashed(sending);
  await expectDashed(q.getByRole('button', { name: 'Deixar passar' }));
  await see(v, p, 'bard-cutting');
  release();
  await expect(q.getByRole('alert')).toContainText('Não deu para cortar agora. Nada foi gasto. Tente de novo.');
  await see(v, p, 'bard-cut-error');
  await p.unroute('**/*ResourceService/AnswerHeldRoll');
  // Nothing was spent, and the question is still there to answer.
  await expect(q.getByRole('button', { name: 'Deixar passar' })).toBeVisible();
  await q.getByRole('button', { name: 'Cortar', exact: true }).click();
  await expect(q).toContainText('Você cortou');
  await expect(q).toContainText('Restam 2');
  await q.getByRole('button', { name: 'Voltar à cena' }).click();
  await expect(npcCard(m)).toContainText(/Cortou \d/);
}

/** Who sees nothing (RN-10): another player reads no question, no card and no line, and the server gives him none (board 591-08 state 9). */
export async function nobodySeesFlow(t: NpcTable, v: NpcView): Promise<void> {
  const { m, p, campaignId, other } = t;
  if (!other) {
    throw new Error('nobodySeesFlow wants a second player');
  }
  await bardReady(other.page, campaignId);
  await masterRolls(t, 'Capitão bandido', '13');
  await bardReady(p, campaignId);
  await expect(cutSheet(p)).toBeVisible();
  const seen = other.page;
  await expect(npcCard(m)).toContainText('Esperando Brisa');
  await expect(seen.getByRole('alertdialog')).toHaveCount(0);
  await expect(seen.getByRole('region', { name: /^Teste de um NPC: / })).toHaveCount(0);
  await expect(seen.getByText(/Usar Palavras de Interrupção|faz um teste|Esperando|Deixou passar/)).toHaveCount(0);
  await see(v, seen, 'other-sees-nothing');
  const list = await callRPC(seen, 'meurpg.play.v1.ResourceService/ListHeldRolls', { campaignId });
  expect((await list.json()).offers ?? []).toEqual([]);
  const latest = await callRPC(seen, 'meurpg.play.v1.PlayService/GetNpcCheck', { campaignId });
  expect(latest.status()).toBe(403);
  // Brisa's own list holds her question and none of the creature's numbers.
  const mine = await callRPC(p, 'meurpg.play.v1.ResourceService/ListHeldRolls', { campaignId });
  const body = JSON.stringify(await mine.json());
  expect(body).not.toMatch(/"d20"|"total"|"modifier"|"checkKey"|Enganação/);
  await cutSheet(p).getByRole('button', { name: 'Deixar passar' }).click();
}

/** With no scene open the entry is beside "Pedir teste em grupo", and the card appears there (board 591-08 state 5). */
export async function noSceneFlow(t: NpcTable, v: NpcView): Promise<void> {
  const { m, p, campaignId } = t;
  await openSessionPage(m, campaignId);
  await expect(m.getByRole('button', { name: 'Pedir teste em grupo' })).toBeVisible();
  const entry = m.getByRole('button', { name: 'Teste de um NPC' });
  await expect(entry).toBeVisible();
  await see(v, m, 'entry-no-scene', entry);
  await entry.click();
  const sheet = npcSheet(m);
  await pickNpc(sheet, 'Capitão Goblin');
  await typeD20(sheet, '10');
  await rollChosen(m, sheet);
  await expect(npcCard(m)).toContainText(/d20 \(10\) [+−] \d+ = \d+/);
  await see(v, m, 'card-no-scene', npcCard(m));
  await bardReady(p, campaignId);
  await expect(cutSheet(p)).toBeVisible();
  await cutSheet(p).getByRole('button', { name: 'Deixar passar' }).click();
}

/** The "⋮" of an NPC on the stage opens the same sheet with that NPC chosen (board 591-08 state 5). */
export async function stageMenuFlow(t: NpcTable): Promise<Locator> {
  const { m, campaignId } = t;
  await openSessionPage(m, campaignId);
  await m.getByRole('button', { name: 'Mais ações de Bartolo' }).click();
  await m.getByRole('menuitem', { name: 'Teste do Bartolo' }).click();
  const sheet = npcSheet(m);
  await expect(sheet.getByRole('button', { name: /^Rolar .* do Bartolo/ })).toBeVisible();
  return sheet;
}
