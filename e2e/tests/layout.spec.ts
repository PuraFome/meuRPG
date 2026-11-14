import { expect } from '@playwright/test';
import { test } from './diagnostics';
import { expectAligned, layoutIssues } from './layout';

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
