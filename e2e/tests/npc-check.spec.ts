import { test } from './diagnostics';

import {
  appDiceFlow,
  combatOpenFlow,
  cutFailureFlow,
  hiddenFlow,
  hideAfterRollFlow,
  lateAnswerFlow,
  loadingFlow,
  noBonusFlow,
  noSceneFlow,
  nobodySeesFlow,
  passFlow,
  physicalDiceFlow,
  refusalsFlow,
  sheetFlow,
  stageMenuFlow,
  twoBardsFlow,
} from './npc-check-flows';
import { npcTable, type NpcOptions } from './npc-check-support';

// #591: Cutting Words (Palavras de Interrupção) outside a combat. The master rolls the check of an NPC ("Teste de um NPC"), the
// College of Lore bards of the table are asked one at a time whether to cut it, and the master's card follows. The table comes through
// the API; what is under test is what the master reads and what each bard is shown (RN-10: the bard never reads the creature's d20,
// bonus, total or DC, and the other players read nothing).

async function withTable(
  browser: Parameters<typeof npcTable>[0],
  options: NpcOptions,
  flow: (t: Awaited<ReturnType<typeof npcTable>>) => Promise<unknown>,
): Promise<void> {
  const t = await npcTable(browser, {}, options);
  try {
    await flow(t);
  } finally {
    await t.done().catch(() => undefined);
  }
}

test('a folha "Teste de um NPC" lista quem rola, começa a caixa certa e diz quando a criatura rola com +0', { tag: ['@MR-015', '@RN-10'] }, async ({ browser }) => {
  test.setTimeout(180_000);
  await withTable(browser, {}, (t) => sheetFlow(t, {}));
});

test('o mestre rola com os dados do app, a bardo corta com os dela, e o cartão fecha, volta ao recarregar e some com o X só neste aparelho', { tag: ['@MR-015', '@RN-10', '@RN-18'] }, async ({ browser }) => {
  test.setTimeout(180_000);
  await withTable(browser, {}, (t) => appDiceFlow(t, {}));
});

test('o mestre digita o d20 e a bardo digita o dado físico dela: "Cortar com 4"', { tag: ['@MR-015', '@RN-18'] }, async ({ browser }) => {
  test.setTimeout(180_000);
  await withTable(browser, { physical: true }, (t) => physicalDiceFlow(t, {}));
});

test('a bardo deixa passar: nada é gasto, a cena diz uma linha e o cartão do mestre diz "Deixou passar"', { tag: ['@MR-015'] }, async ({ browser }) => {
  test.setTimeout(180_000);
  await withTable(browser, {}, (t) => passFlow(t, {}));
});

test('dois bardos em ordem: o segundo só lê a pergunta na vez dele, e "Continuar sem esperar" fecha a dele sem gastar nada', { tag: ['@MR-015', '@RN-10'] }, async ({ browser }) => {
  test.setTimeout(240_000);
  await withTable(browser, { other: 'bard' }, (t) => twoBardsFlow(t, {}));
});

test('a resposta que chega depois de o mestre seguir é recusada: "A pergunta já foi encerrada"', { tag: ['@MR-015'] }, async ({ browser }) => {
  test.setTimeout(240_000);
  await withTable(browser, { other: 'bard' }, (t) => lateAnswerFlow(t, {}));
});

test('uma criatura escondida começa sem caixa marcada e, marcada, a bardo lê "uma criatura"', { tag: ['@MR-015', '@RN-10'] }, async ({ browser }) => {
  test.setTimeout(180_000);
  await withTable(browser, {}, (t) => hiddenFlow(t, {}));
});

test('o mestre esconde o token da criatura depois da rolagem: a pergunta da bardo some e o cartão fecha a pergunta como "Sem resposta"', { tag: ['@MR-015', '@RN-10'] }, async ({ browser }) => {
  test.setTimeout(180_000);
  await withTable(browser, {}, (t) => hideAfterRollFlow(t, {}));
});

test('uma criatura sem bônus conhecido rola com +0 e o cartão diz isso ao lado do resultado', { tag: ['@MR-015'] }, async ({ browser }) => {
  test.setTimeout(180_000);
  await withTable(browser, {}, (t) => noBonusFlow(t, {}));
});

test('um combate aberto recusa o teste: nada é rolado e a entrada fica no lugar, tracejada, com o motivo', { tag: ['@MR-015'] }, async ({ browser }) => {
  test.setTimeout(180_000);
  await withTable(browser, {}, (t) => refusalsFlow(t, {}));
});

test('um combate que começa com a folha aberta: o servidor recusa o teste e nada é rolado', { tag: ['@MR-015'] }, async ({ browser }) => {
  test.setTimeout(180_000);
  await withTable(browser, {}, (t) => combatOpenFlow(t));
});

test('a folha lê as criaturas, mostra o erro com "Tentar de novo" e diz "Rolando…"', { tag: ['@MR-015'] }, async ({ browser }) => {
  test.setTimeout(180_000);
  await withTable(browser, {}, (t) => loadingFlow(t, {}));
});

test('o corte que falha não gasta nada: "Cortando…" e "Não deu para cortar agora"', { tag: ['@MR-015'] }, async ({ browser }) => {
  test.setTimeout(180_000);
  await withTable(browser, {}, (t) => cutFailureFlow(t, {}));
});

test('outro jogador não vê nada: nem pergunta, nem cartão, nem linha, e o servidor não lhe dá nada', { tag: ['@MR-015', '@RN-10'] }, async ({ browser }) => {
  test.setTimeout(240_000);
  await withTable(browser, { other: 'fighter' }, (t) => nobodySeesFlow(t, {}));
});

test('sem cena aberta a entrada fica ao lado de "Pedir teste em grupo" e o cartão aparece ali', { tag: ['@MR-015'] }, async ({ browser }) => {
  test.setTimeout(180_000);
  await withTable(browser, { noScene: true }, (t) => noSceneFlow(t, {}));
});

test('o menu de um NPC em cena abre a folha com ele escolhido', { tag: ['@MR-015'] }, async ({ browser }) => {
  test.setTimeout(180_000);
  await withTable(browser, {}, (t) => stageMenuFlow(t));
});
