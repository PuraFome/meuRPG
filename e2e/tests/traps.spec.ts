import { expect, test, type Page } from '@playwright/test';

import { openSessionPage } from './live-session-support';
import { callRPC } from './support';
import { movePensantus, pensantusFirst, pointNames, sq20, trapRPC, trapTable, treasureRPC } from './trap-support';

// Traps and treasure in the session, on screen (Etapa 9, slice 9.14, MR-035, MR-041, RN-10, RN-02,
// question 71). The table, the traps and the combat come through the API; what is under test is what
// the master and the player read and press, and what the player is never told.

const trapPanel = (m: Page) => m.getByRole('region', { name: 'Armadilhas do mapa' });
const treasurePanel = (m: Page) => m.getByRole('region', { name: 'Tesouros do mapa' });

test(
  'o mestre revela a armadilha para o jogador: antes ele não a recebe, depois sim',
  { tag: ['@MR-035', '@RN-10'] },
  async ({ browser }) => {
    test.setTimeout(180_000);
    const { m, p, table, campaignId, done } = await trapTable(browser, 'Revelar');
    try {
      await trapRPC(m, table, 'Fosso escondido', 12, 7, { noticeDc: 30 });
      await openSessionPage(m, campaignId);
      expect(await pointNames(p, table)).not.toContain('Fosso escondido');

      const card = trapPanel(m).getByRole('article', { name: 'Fosso escondido' });
      await expect(card).toContainText('Só você vê');
      await expect(card).toContainText('Quem notaria');
      await card.getByRole('button', { name: 'Revelar para…' }).click();
      const dialog = m.getByRole('dialog', { name: 'Revelar armadilha' });
      await expect(dialog.getByRole('button', { name: 'Revelar a armadilha' })).toHaveAttribute('aria-disabled', 'true');
      await dialog.locator('label', { hasText: 'Pensantus' }).click();
      await dialog.getByRole('button', { name: 'Revelar para Pensantus' }).click();
      await expect(dialog).toBeHidden();
      await expect(card).toContainText('Visível para todos');
      expect(await pointNames(p, table)).toContain('Fosso escondido');
    } finally {
      await done();
    }
  },
);

test(
  'o jogador procura com Investigação e acha; longe, a resposta é a mesma de quando falha',
  { tag: ['@MR-035', '@RN-10'] },
  async ({ browser }) => {
    test.setTimeout(180_000);
    const { m, p, table, campaignId, done } = await trapTable(browser, 'Procurar');
    try {
      await trapRPC(m, table, 'Fosso escondido', 6, 7);
      await openSessionPage(p, campaignId);
      expect(await pointNames(p, table)).not.toContain('Fosso escondido');

      await p.getByRole('button', { name: 'Procurar armadilhas' }).click();
      const sheet = p.getByRole('dialog', { name: 'Procurar armadilhas' });
      await expect(sheet).toContainText('Algumas armadilhas só se acham com Investigação.');
      await sheet.locator('label', { hasText: 'Investigação' }).click();
      await sheet.getByRole('button', { name: 'Digitar o resultado' }).click();
      await sheet.getByLabel(/Role 1d20 para Investigação/).fill('12');
      await sheet.getByRole('button', { name: /Confirmar 12/ }).click();
      await expect(sheet).toContainText('Você achou uma armadilha: Fosso escondido.');
      await sheet.getByRole('button', { name: 'Fechar', exact: true }).last().click();
      expect(await pointNames(p, table)).toContain('Fosso escondido');
      await expect(p.getByRole('region', { name: 'Registro' })).toContainText('procurou armadilhas (Investigação)');

      // A trap far away: the same words as a roll that falls short.
      await trapRPC(m, table, 'Fosso distante', 18, 2);
      await p.getByRole('button', { name: 'Procurar armadilhas' }).click();
      await sheet.getByRole('button', { name: 'Digitar o resultado' }).click();
      await sheet.getByLabel(/Role 1d20 para/).fill('20');
      await sheet.getByRole('button', { name: /Confirmar 20/ }).click();
      await expect(sheet).toContainText('Você não encontrou nada.');
      expect(await pointNames(p, table)).not.toContain('Fosso distante');
    } finally {
      await done();
    }
  },
);

test(
  'o personagem anda para dentro da armadilha no combate: o movimento para, o dano espera e o mestre aplica',
  { tag: ['@MR-035', '@RN-02'] },
  async ({ browser }) => {
    test.setTimeout(240_000);
    const { m, p, table, campaignId, done } = await trapTable(browser, 'Queda');
    try {
      await trapRPC(m, table, 'Fosso escondido', 7, 7, { damage: '3' });
      await pensantusFirst(m, table);
      await openSessionPage(m, campaignId);
      await openSessionPage(p, campaignId);

      const moved = await movePensantus(p, table, 9, 7);
      expect(moved.combatants.find((c) => c.label === 'Pensantus')!.col).toBe(7);

      await expect(p.getByText('Você caiu na armadilha Fosso escondido.')).toBeVisible();
      await expect(p.getByText('Esperando o mestre aplicar o dano')).toBeVisible();
      const card = m.getByRole('region', { name: /Fosso escondido pegou Pensantus/ });
      await expect(card).toContainText('3 de concussão');
      await card.getByRole('button', { name: 'Aplicar 3 de dano' }).click();
      await expect(card).toBeHidden();
      await expect(m.getByText('A armadilha Fosso escondido foi disparada').first()).toBeVisible();
    } finally {
      await done();
    }
  },
);

test(
  'o mestre dispara na mão fora do combate e decide o dano: aplicar um, descartar o outro',
  { tag: ['@MR-035', '@RN-02'] },
  async ({ browser }) => {
    test.setTimeout(180_000);
    const { m, table, campaignId, done } = await trapTable(browser, 'Disparar');
    try {
      await trapRPC(m, table, 'Fosso escondido', 12, 7, { manual: true, damage: '4' });
      await openSessionPage(m, campaignId);
      const card = trapPanel(m).getByRole('article', { name: 'Fosso escondido' });
      await card.getByRole('button', { name: 'Disparar…' }).click();
      const dialog = m.getByRole('dialog', { name: /Disparar/ });
      await expect(dialog).toContainText('Ninguém marcado: dispara para quem estiver na área');
      await dialog.locator('label', { hasText: 'Pensantus' }).click();
      await dialog.getByRole('button', { name: 'Disparar para Pensantus' }).click();
      await expect(card).toContainText('Disparada');

      const damage = trapPanel(m).getByRole('region', { name: /Fosso escondido pegou Pensantus/ });
      await expect(damage).toContainText('Mude o número se houver resistência: o app não calcula.');
      await damage.getByRole('textbox').fill('2');
      await damage.getByRole('button', { name: 'Aplicar 2 de dano' }).click();
      await expect(damage).toBeHidden();
      await expect(m.getByRole('region', { name: 'Registro' })).toBeVisible();

      // The same trap is disarmed after the table resolves the thieves' tools check.
      await card.getByRole('button', { name: 'Marcar como desarmada' }).click();
      await expect(card).toContainText('Desarmada');
    } finally {
      await done();
    }
  },
);

test(
  'o aviso passivo chega só ao jogador que notou: a mensagem, a armadilha no mapa dele e o fechar',
  { tag: ['@MR-035', '@RN-10'] },
  async ({ browser }) => {
    test.setTimeout(180_000);
    const { m, p, table, campaignId, done } = await trapTable(browser, 'Aviso');
    try {
      await trapRPC(m, table, 'Fosso escondido', 9, 7, { noticeDc: 5 });
      await openSessionPage(p, campaignId);
      expect(await pointNames(p, table)).not.toContain('Fosso escondido');
      // The master walks Pensantus up to within 3 m of the pit.
      const res = await callRPC(m, 'meurpg.maps.v1.MapService/PlaceMapToken', {
        campaignId,
        mapId: table.mapId,
        characterId: table.characterId,
        ...sq20(8, 7),
      });
      expect(res.ok(), await res.text()).toBeTruthy();
      const toast = p.getByRole('status').filter({ hasText: 'Você notou uma armadilha.' });
      await expect(toast).toBeVisible();
      await expect(toast).toContainText('Fosso escondido');
      expect(await pointNames(p, table)).toContain('Fosso escondido');
      await p.getByRole('button', { name: 'Dispensar o aviso' }).click();
      await expect(p.getByText('Você notou uma armadilha.')).toBeHidden();
    } finally {
      await done();
    }
  },
);

test(
  'o mestre marca o tesouro como encontrado, o jogador o vê e o mestre o desmarca',
  { tag: ['@MR-041', '@RN-10'] },
  async ({ browser }) => {
    test.setTimeout(180_000);
    const { m, p, table, campaignId, done } = await trapTable(browser, 'Tesouro');
    try {
      await treasureRPC(m, table, 'Baú de moedas', 12, 7);
      await openSessionPage(m, campaignId);
      await openSessionPage(p, campaignId);
      expect(await pointNames(p, table)).not.toContain('Baú de moedas');

      const card = treasurePanel(m).getByRole('article', { name: 'Baú de moedas' });
      await expect(card).toContainText('O que tem dentro · só você vê até achar');
      await card.getByRole('button', { name: 'Marcar o Baú de moedas como encontrado' }).click();
      await expect(card.getByRole('button', { name: 'Marcar como encontrado' })).toHaveAttribute('aria-disabled', 'true');
      await card.locator('label', { hasText: 'Pensantus' }).click();
      await expect(card).toContainText('No resumo da sessão: Pensantus');
      await card.getByRole('button', { name: 'Marcar como encontrado' }).click();
      await expect(card).toContainText('Encontrado por Pensantus');

      // The player: the toast, the row and the sheet with the value and what is inside.
      await expect(p.getByRole('status').filter({ hasText: 'Pensantus encontrou o Baú de moedas.' })).toBeVisible();
      await p.getByRole('button', { name: /Baú de moedas, Tesouro/ }).click();
      const sheet = p.getByRole('dialog', { name: 'Baú de moedas' });
      await expect(sheet).toContainText('250');
      await expect(sheet).toContainText('250 PO e uma adaga de prata.');
      await sheet.getByRole('button', { name: 'Fechar', exact: true }).last().click();

      await card.getByRole('button', { name: 'Desmarcar' }).click();
      await expect(card.getByRole('alertdialog')).toContainText('Desmarcar o Baú de moedas?');
      await card.getByRole('alertdialog').getByRole('button', { name: 'Desmarcar' }).click();
      await expect(card).toContainText('Escondido');
      await expect.poll(() => pointNames(p, table)).not.toContain('Baú de moedas');
    } finally {
      await done();
    }
  },
);
