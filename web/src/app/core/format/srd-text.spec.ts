import { parseSrdText } from './srd-text';

describe('parseSrdText', () => {
  it('parses paragraphs with bold, bold italic and italics (* and _)', () => {
    const [p] = parseSrdText(['***Título.*** Um _Bola de Fogo_ e *runas*.']);
    expect(p.type).toBe('paragraph');
    const kinds = JSON.stringify(p);
    expect(kinds).toContain('bold');
    expect(kinds).toContain('italic');
    expect(kinds).toContain('Bola de Fogo');
    expect(kinds).not.toContain('_');
  });

  it('groups "- " lines into one list and "|" lines into one table without its separator', () => {
    const blocks = parseSrdText([
      'Antes',
      '- a',
      '- b',
      '| d100 | Tipo |',
      '|---|---|',
      '| 01-40 | Prata |',
      '| 41-75 | **Latão** |',
      'Depois',
    ]);
    expect(blocks.map((b) => b.type)).toEqual(['paragraph', 'list', 'table', 'paragraph']);
    const table = blocks[2];
    if (table.type !== 'table') {
      throw new Error('not a table');
    }
    expect(table.head).toHaveLength(2);
    expect(table.rows).toHaveLength(2);
    expect(table.rows[1][1][0].type).toBe('bold');
  });

  it('turns "#" lines into headings and never produces markup from hostile text', () => {
    const blocks = parseSrdText([
      '##### Estatísticas',
      '<img src=x onerror=alert(1)> [x](javascript:alert(1))',
    ]);
    expect(blocks[0].type).toBe('heading');
    expect(JSON.stringify(blocks[1])).not.toContain('"href"');
  });
});
