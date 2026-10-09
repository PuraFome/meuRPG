import { create } from '@bufbuild/protobuf';

import {
  TableContentKind,
  TableContentPackSchema,
  TableEntrySchema,
  TableImportStatus,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import {
  PACK_MAX_BYTES,
  changedText,
  countsByKind,
  packFileName,
  packToText,
  packViolationText,
  parsePack,
  statusWord,
} from './content-pack';
import { entry, mirathel } from './content-testing';

const pack = () =>
  create(TableContentPackSchema, {
    format: 'meurpg.table-content',
    version: 1,
    name: 'Mesa de Mirathel',
    entries: mirathel().map((e) =>
      create(TableEntrySchema, { key: e.key, kind: e.kind, namePt: e.namePt, body: e.body }),
    ),
  });

describe('the content pack file (MR-025)', () => {
  it('names the file after the campaign: no accents, dashes, "-conteudo.json"', () => {
    expect(packFileName('Mesa de Mirathel')).toBe('mesa-de-mirathel-conteudo.json');
    expect(packFileName('  Crônicas do Vale — 2ª Era! ')).toBe(
      'cronicas-do-vale-2-era-conteudo.json',
    );
    expect(packFileName('???')).toBe('campanha-conteudo.json');
  });

  it("writes the file in proto JSON with the proto's field names, and reads it back", () => {
    const text = packToText(pack());
    expect(text).toContain('"name_pt"');
    expect(text).toContain('"table_class"');
    expect(text).not.toContain('namePt');
    expect(text).toContain('\n  "format": "meurpg.table-content"');
    const back = parsePack(text, text.length);
    expect(back.ok).toBe(true);
    if (back.ok) {
      expect(back.pack.entries.map((e) => e.namePt)).toEqual(pack().entries.map((e) => e.namePt));
      expect(back.pack.entries[0].kind).toBe(TableContentKind.CLASS);
    }
  });

  it('refuses, before the server, a file that is too big, not JSON, not an object or of another format', () => {
    const message = (text: string, size = text.length) => {
      const r = parsePack(text, size);
      return r.ok ? '' : r.message;
    };
    expect(message('{}', PACK_MAX_BYTES + 1)).toContain('2 MiB');
    expect(message('{ nope')).toContain('não é um JSON válido');
    expect(message('[1, 2]')).toContain('não parece um pacote');
    expect(message('{"format": "outro"}')).toContain('meurpg.table-content');
  });

  it('refuses a file with a field we do not know instead of ignoring it', () => {
    const text = JSON.stringify({
      format: 'meurpg.table-content',
      version: 1,
      entries: [],
      extra: 1,
    });
    const r = parsePack(text, text.length);
    expect(r.ok).toBe(false);
    expect(r.ok ? '' : r.message).toContain('Não reconheci algum campo');
  });

  it('says what an update changes, in words, once each', () => {
    expect(changedText(['name_pt'])).toBe('o nome');
    expect(changedText(['name_pt', 'table_class.levels', 'table_spell.desc_pt'])).toBe(
      'o nome, a tabela dos níveis e o texto',
    );
    expect(changedText(['table_class.levels', 'table_class.levels'])).toBe('a tabela dos níveis');
    expect(changedText(['table_class.something_new'])).toBe('outros dados');
    expect(changedText([])).toBe('outros dados');
  });

  it('counts the entries by the kinds of the menu, the sub-races with the races', () => {
    const rows = [
      entry(TableContentKind.RACE, 'A'),
      entry(TableContentKind.SUBRACE, 'B'),
      entry(TableContentKind.FEAT, 'C'),
    ];
    expect(countsByKind(rows).map((c) => `${c.nav.plural} ${c.count}`)).toEqual([
      'Raças 2',
      'Talentos 1',
    ]);
  });

  it('writes the refusal of a pack and of an entry in Portuguese, with how to fix it', () => {
    const text = (field: string, reason = 'bad_value') => packViolationText({ field, reason });
    expect(text('pack', 'size_limit')).toContain('2 MiB');
    expect(text('pack.format')).toContain('meurpg.table-content');
    expect(text('pack.version')).toContain('versão');
    expect(text('pack.entries[3].key', 'duplicate_key')).toContain('mesma chave');
    expect(text('pack.entries[3].key', 'bad_key')).toContain('Corrija o campo "key"');
    expect(text('pack.entries[3].body')).toContain('sem conteúdo');
    expect(text('table_feat.prerequisite.level')).toBe(
      'O nível vai de 1 a 20 (vazio não pede nada).',
    );
    expect(
      packViolationText({ field: 'table_feat.prerequisite.proficiency', reason: 'unknown_field' }),
    ).toBe('Campo desconhecido: proficiency. Confira a grafia no arquivo.');
    expect(statusWord(TableImportStatus.REFUSED)).toBe('Recusada');
  });
});
