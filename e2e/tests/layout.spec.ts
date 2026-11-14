import * as path from 'node:path';
import { expect } from '@playwright/test';
import { compile } from 'sass';
import { test } from './diagnostics';
import { expectAligned, layoutIssues } from './layout';

// The stylesheets of the rows are compiled with the sass of this package, pinned to the web app's version, so the list of tests
// loads where only e2e/ is installed.
const webDir = path.join(__dirname, '..', '..', 'web');

// The one-line and chip checks of layout.ts (8, 6, 12), run on small pages that carry the markup and the layout rules of the
// action row, the gear row's marks group and the combat column's item modifiers, in a column as narrow as a phone's.
// A wrong layout must be reported and the right one must pass.

const css = `
  body { margin: 0; padding: 8px; font: 16px/22px sans-serif; }
  .col { box-sizing: border-box; }
  .row__head { display: flex; flex-wrap: wrap; align-items: center; gap: 0 6px; }
  .row__head > :not(.row__name) { flex: none; }
  .row__name { flex: 1000 1 96px; min-width: 0; max-width: max-content; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: 700; font-size: 21px; line-height: 26px; }
  .row__marks-group { display: flex; align-items: center; gap: 6px; }
  .chip { display: inline-block; box-sizing: border-box; width: 22px; height: 22px; border: 1px solid #555; border-radius: 11px; }
  .row__tags { display: flex; flex-wrap: nowrap; gap: 4px 8px; }
  .row__pill { display: inline-flex; align-items: center; height: 24px; padding: 0 10px; border: 1px solid #555; border-radius: 12px; font-size: 14px; line-height: 1; }
  .gear-row__marks { display: flex; align-items: center; gap: 4px; flex-wrap: wrap; }
  .modifiers__line { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: 0 12px; min-width: 0; }
  .modifiers__name { flex: 1000 1 96px; min-width: 0; max-width: max-content; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .modifiers__target { flex: 0 0 auto; min-width: 0; font-size: 14px; white-space: nowrap; }
`;

const longName = 'Espada longa dos ventos do norte';

/** An action row's head: the name, the marks group (a mark and the bonus chip) and the tags (the economy). */
const actionHead = (extra = '') => `
  <div class="col" style="width: 150px">
    <span class="row__head" data-chip-row data-one-line>
      <span class="row__name" data-chip-ref>${longName}</span>
      <span class="row__marks-group" data-wrap-group${extra}><span class="chip"></span><span class="chip" data-chip></span></span>
      <span class="row__tags" data-wrap-group><span class="row__pill">Ação</span></span>
    </span>
  </div>`;

test('a linha de ataque com o nome comprido, as marcas e a etiqueta de economia na segunda linha passa nas conferências de layout', async ({ page }) => {
  await page.setContent(`<style>${css}</style>${actionHead()}`);
  const name = (await page.locator('.row__name').boundingBox())!;
  const marks = (await page.locator('.row__marks-group').boundingBox())!;
  const tags = (await page.locator('.row__tags').boundingBox())!;
  expect(marks.y, 'the marks dropped under the name').toBeGreaterThan(name.y + name.height - 1);
  expect(tags.y, 'the tags follow the marks on their line').toBeLessThan(marks.y + marks.height);
  expect(marks.x).toBeCloseTo(name.x, 0);
  await expectAligned(page, 'Linha de ataque, as marcas e a economia na segunda linha');
});

test('uma etiqueta que não é um grupo, ou um grupo fora do x do nome, sob o nome é achado das conferências de layout', async ({ page }) => {
  await page.setContent(`<style>${css}</style>${actionHead(' style="margin-left: 10px"')}`);
  expect((await layoutIssues(page, 'Grupo deslocado')).join('\n')).toMatch(/not a wrap group that starts at the name's x/);

  await page.setContent(
    `<style>${css}</style><div class="col" style="width: 150px"><span class="row__head" data-one-line><span class="row__name" data-chip-ref>${longName}</span><span class="row__pill">Truque</span></span></div>`,
  );
  expect((await layoutIssues(page, 'Etiqueta solta')).join('\n')).toMatch(/not a wrap group that starts at the name's x/);
});

test('o chip de sintonização sozinho na segunda linha do seu grupo passa e separado do resto do grupo é achado', async ({ page }) => {
  const row = (width: number) => `<ul style="margin: 0; padding: 0"><li class="gear-row" data-chip-row><span class="gear-row__marks" data-wrap-group style="width: ${width}px"><span class="chip"></span><span class="chip gear-row__att" data-chip></span></span></li></ul>`;
  await page.setContent(`<style>${css}</style>${row(120)}`);
  await expectAligned(page, 'Chip junto das marcas');
  await page.setContent(`<style>${css}</style>${row(30)}`);
  expect((await layoutIssues(page, 'Chip sozinho')).join('\n')).toMatch(/chip-alone .*the chip sits on a line of its own/);
});

test('o nome de um modificador de itens fica com 96px ao lado do alvo, que desce sob ele quando não cabe', async ({ page }) => {
  const line = (width: number, name: string, target: string) => `
    <div style="width: ${width}px"><span class="modifiers__line" data-one-line>
      <span class="modifiers__name" data-chip-ref><strong>+1</strong> · ${name}</span>
      <span class="modifiers__target" data-wrap-group>${target}</span>
    </span></div>`;
  await page.setContent(`<style>${css}</style>${line(200, 'Espada longa dos ventos do norte', 'no ataque e no dano')}`);
  const name = (await page.locator('.modifiers__name').boundingBox())!;
  const target = (await page.locator('.modifiers__target').boundingBox())!;
  expect(target.y, 'the target dropped under the name').toBeGreaterThan(name.y + name.height - 1);
  expect(target.x).toBeCloseTo(name.x, 0);
  expect(name.width, 'the name takes the whole line').toBeCloseTo(200, 0);
  await expectAligned(page, 'Modificador, o alvo sob o nome');

  await page.setContent(`<style>${css}</style>${line(300, 'Escudo', 'na CA')}`);
  const short = (await page.locator('.modifiers__target').boundingBox())!;
  const shortName = (await page.locator('.modifiers__name').boundingBox())!;
  expect(short.y, 'the target stays beside the name').toBeLessThan(shortName.y + shortName.height);
  await expectAligned(page, 'Modificador, o alvo ao lado do nome');
});

test('a legenda dos chips quebra por itens inteiros: uma palavra solta do seu chip é achado das conferências de layout', async ({ page }) => {
  const legend = (itemWidth: string) => `
    <ul class="legend" data-chip-legend style="display: flex; flex-wrap: wrap; gap: 8px 16px; margin: 0; padding: 0; list-style: none; width: 300px; font-size: 14px">
      <li style="display: inline-flex; align-items: center; gap: 8px"><span class="chip legend__chip"></span>Prateada</li>
      <li style="display: inline-flex; align-items: center; gap: 8px; ${itemWidth}"><span class="chip legend__chip"></span>Bônus mágico, já somado</li>
    </ul>`;
  await page.setContent(`<style>${css}</style>${legend('')}`);
  await expectAligned(page, 'Legenda dos chips, itens inteiros');
  await page.setContent(`<style>${css}</style>${legend('width: 90px')}`);
  expect((await layoutIssues(page, 'Legenda quebrada')).join('\n')).toMatch(/legend-broken .*a legend wraps by whole items/);
});

/**
 * A combat gear row of the real `gear-list` stylesheet (compiled here, the host as a box), with its markup: the kind icon, the name, one mark,
 * the "⋮" and an outlined button that carries its cost. The list is as wide as the fold is on a phone and up.
 */
function gearRow(listWidth: number, name: string, cost: string): string {
  const css = compile(path.join(webDir, 'src', 'app', 'shared', 'gear-list', 'gear-list.scss')).css.replace(/:host/g, '.gear-host');
  return `<style>:root { --mr-space-1: 4px; --mr-space-2: 8px; --mr-space-3: 12px; --mr-font-14: 14px; --mr-font-18: 18px; --mr-rule: #ccc; } body { margin: 0; font: 16px/22px sans-serif; } .mat-icon { display: inline-block; } .gear-row__button { box-sizing: border-box; border: 1px solid #555; background: none; font: inherit; } ${css}</style>
    <div class="gear-host" style="width: ${listWidth}px">
      <ul class="gear-rows" data-equal-width=".gear-row__button">
        <li class="gear-row" data-chip-row data-one-line>
          <span class="gear-row__main gear-row__main--more">
            <span class="mat-icon gear-row__icon">x</span>
            <span class="gear-row__body">
              <span class="gear-row__name" data-chip-ref>${name}</span>
              <span class="gear-row__marks" data-wrap-group><span class="chip gear-row__att" data-chip></span></span>
            </span>
            <button type="button" class="gear-row__more">⋮</button>
          </span>
          <button type="button" class="gear-row__button">Guardar<span class="gear-row__cost"><span class="mat-icon">b</span>${cost}</span></button>
        </li>
      </ul>
    </div>`;
}

test('a linha de equipamento do combate guarda 96px para o nome, ao lado do "⋮" e do botão com o seu custo, do celular em diante', async ({ page }) => {
  for (const width of [236, 270, 290, 306, 400, 768]) {
    await page.setContent(gearRow(width, 'Espada longa dos ventos do norte', 'grátis'));
    const name = (await page.locator('.gear-row__name').boundingBox())!;
    expect(name.width, `the name at ${width}px`).toBeGreaterThanOrEqual(96);
    await expectAligned(page, `Linha de equipamento do combate, lista de ${width}px`);
  }
});

/** A spell row of the real `action-row` stylesheet (compiled here, the host as a box): the name and its economy tag, the "?" and the button. */
function actionRow(rowWidth: number): string {
  const css = compile(path.join(webDir, 'src', 'app', 'pages', 'live-session', 'combat', 'action-groups', 'action-row.scss')).css.replace(/:host\(([^)]*)\)/g, '.row-host$1').replace(/:host/g, '.row-host');
  return `<style>:root { --mr-font-14: 14px; --mr-font-15: 15px; --mr-font-21: 21px; } body { margin: 0; font: 16px/22px sans-serif; } ${css}</style>
    <div class="row-host row--help" style="width: ${rowWidth}px">
      <div class="row__text">
        <span class="row__head row__head--one" data-chip-row data-one-line>
          <span class="row__name" data-chip-ref>Raio de Fogo</span>
          <span class="row__tags" data-wrap-group><span class="row__econ">Ação</span></span>
        </span>
      </div>
      <button type="button" class="help" style="grid-column: 2; width: 44px">?</button>
      <button type="button" class="row__btn">Conjurar</button>
    </div>`;
}

test('a linha de ação com o "?" e o botão guarda 96px para o nome na largura de um celular de 320', async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 568 });
  await page.setContent(actionRow(262));
  const name = (await page.locator('.row__name').boundingBox())!;
  expect(name.width, 'the name beside the "?" and the button').toBeGreaterThanOrEqual(96);
  await expectAligned(page, 'Linha de ação com o "?", a 320px');
});
