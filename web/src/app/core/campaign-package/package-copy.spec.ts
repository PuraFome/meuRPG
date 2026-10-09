import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';

import { PackageCountsSchema } from '../../../gen/meurpg/campaignpackage/v1/campaignpackage_pb';
import { plain } from '../images/gallery-testing';
import { counts, exportOf, importUpload, previewOf } from './campaign-package-testing';
import {
  approxSize,
  beginName,
  countRows,
  counted,
  createdItems,
  createdSentenceTail,
  dayWord,
  exportPhase,
  exportProgressText,
  expiryText,
  fileRefusalText,
  fingerprintOf,
  importUploadFailureText,
  keptUploadText,
  lastPackageText,
  moreProblemsText,
  precheckPackageFile,
  previewHeadline,
  suggestedFileName,
  uploadProgressText,
} from './package-copy';

const MIB = 1024 * 1024;

describe('precheckPackageFile', () => {
  it('lets a .meurpg.zip and a .zip through, in any case', () => {
    expect(precheckPackageFile({ name: 'Mirathel.meurpg.zip', size: 10 })).toBeNull();
    expect(precheckPackageFile({ name: 'mirathel.ZIP', size: 10 })).toBeNull();
  });

  it('refuses a file that is not a zip, before any byte is sent', () => {
    expect(precheckPackageFile({ name: 'Mirathel.pdf', size: 10 })).toBe('NOT_ZIP');
    expect(precheckPackageFile({ name: 'zip', size: 10 })).toBe('NOT_ZIP');
  });

  it('refuses an empty file and one over 200 MiB (exactly 200 MiB is fine)', () => {
    expect(precheckPackageFile({ name: 'a.zip', size: 0 })).toBe('EMPTY');
    expect(precheckPackageFile({ name: 'a.zip', size: 200 * MIB })).toBeNull();
    expect(precheckPackageFile({ name: 'a.zip', size: 200 * MIB + 1 })).toBe('TOO_BIG');
  });

  it('says each refusal in plain words', () => {
    expect(fileRefusalText('TOO_BIG')).toBe(
      'Esse arquivo passa de 200 MB, o limite de um pacote.'.replace(' MB', ' MB'),
    );
    expect(fileRefusalText('NOT_ZIP')).toContain('.meurpg.zip');
    expect(fileRefusalText('EMPTY')).toContain('vazio');
  });
});

describe('fingerprintOf and beginName', () => {
  it('is the same for the same file and differs by name, size or time', () => {
    const file = { name: 'Mirathel.meurpg.zip', size: 40, lastModified: 1_700_000_000_000 };
    expect(fingerprintOf(file)).toBe(fingerprintOf({ ...file }));
    expect(fingerprintOf(file)).not.toBe(fingerprintOf({ ...file, size: 41 }));
    expect(fingerprintOf(file)).not.toBe(fingerprintOf({ ...file, lastModified: 1 }));
    expect(fingerprintOf(file)).not.toBe(fingerprintOf({ ...file, name: 'Outra.zip' }));
  });

  it('fits the server limits (200 characters, a name of 120) with a very long name', () => {
    const file = { name: `${'n'.repeat(300)}.zip`, size: 7, lastModified: 9 };
    expect(fingerprintOf(file).length).toBeLessThanOrEqual(200);
    expect(fingerprintOf(file).startsWith('7:9:')).toBe(true);
    expect(beginName(file).length).toBe(120);
  });
});

describe('upload texts', () => {
  it('says the board sentence: part 7 of 17 (41%)', () => {
    expect(uploadProgressText(7, 17)).toBe(
      'Enviando em partes: parte 7 de 17 (41%). Se a conexão cair, continua de onde parou. As partes ficam guardadas por 1 hora.',
    );
    expect(uploadProgressText(1, 0)).toContain('(100%)');
  });

  it('says every upload failure in Portuguese, without a server message', () => {
    for (const kind of [
      'NETWORK',
      'UNAVAILABLE',
      'RATE_LIMITED',
      'UNAUTHENTICATED',
      'PERMISSION_DENIED',
      'NOT_FOUND',
      'TOO_LARGE',
      'QUOTA',
      'CANCELED',
      'UNKNOWN',
      'CORRUPT',
    ] as const) {
      const text = importUploadFailureText(kind);
      expect(text, kind).toMatch(/\.$/);
      expect(text, kind).not.toMatch(/[A-Z]{4,}/);
    }
  });

  it('offers the kept upload with the parts it holds and how to resume', () => {
    const text = keptUploadText(
      importUpload({ receivedParts: [1, 2, 3], partCount: 17, totalBytes: BigInt(84 * MIB) }),
    );
    expect(plain(text)).toContain('Mirathel.meurpg.zip (84 MB)');
    expect(text).toContain('3 de 17 partes');
    expect(text).toContain('Escolha o mesmo arquivo de novo');
    expect(keptUploadText(importUpload({ receivedParts: [1], partCount: 1 }))).toContain(
      '1 de 1 parte já',
    );
  });
});

describe('preview texts', () => {
  it('lists the counts in the board order', () => {
    const rows = countRows(counts());
    expect(rows.map((r) => r.label)).toEqual([
      'Mapas',
      'NPCs e criaturas',
      'Cenas',
      'Quebra-cabeças',
      'Pontos de batalha e encontros',
      'Pontos de tesouro',
      'Imagens',
      'Conteúdo da mesa',
      'Personagens (reservados)',
    ]);
    expect(rows.map((r) => plain(r.value))).toEqual([
      '6',
      '24',
      '9',
      '3',
      '7',
      '5',
      '58 (82 MB)',
      '6 entradas',
      '4',
    ]);
  });

  it('says "1 entrada" and reads a missing counts as zeros', () => {
    expect(countRows(counts({ contentEntries: 1 }))[7].value).toBe('1 entrada');
    expect(countRows(undefined).map((r) => r.value)).toContain('0 entradas');
    expect(countRows(create(PackageCountsSchema))[0].value).toBe('0');
  });

  it('says the headline of the board, leaving out what the package did not say', () => {
    expect(previewHeadline(previewOf())).toBe(
      'Mirathel · exportado em 08/10 · formato do pacote 1',
    );
    expect(
      previewHeadline(previewOf({ exportedAt: undefined, formatVersion: 0, campaignName: '' })),
    ).toBe('');
    expect(previewHeadline(previewOf({ exportedAt: undefined }))).toBe(
      'Mirathel · formato do pacote 1',
    );
  });

  it('says "N mais problemas" in the singular and the plural', () => {
    expect(moreProblemsText(1)).toBe('E mais 1 problema.');
    expect(moreProblemsText(7)).toBe('E mais 7 problemas.');
  });
});

describe('the campaign made', () => {
  it('says maps, NPCs and creatures, and reserved characters, as the board does', () => {
    expect(createdSentenceTail(counts())).toBe(
      ' foi criada com 6 mapas, 24 NPCs e criaturas e 4 personagens reservados.',
    );
  });

  it('pluralises each item', () => {
    expect(createdItems(counts({ maps: 1, npcs: 1, characters: 1 }))).toEqual([
      '1 mapa',
      '1 NPC ou criatura',
      '1 personagem reservado',
    ]);
  });

  it('leaves a zero out and joins two items with "e"', () => {
    expect(createdSentenceTail(counts({ npcs: 0 }))).toBe(
      ' foi criada com 6 mapas e 4 personagens reservados.',
    );
    expect(createdSentenceTail(counts({ maps: 0, npcs: 0 }))).toBe(
      ' foi criada com 4 personagens reservados.',
    );
    expect(createdSentenceTail(counts({ maps: 2, npcs: 0, characters: 0 }))).toBe(
      ' foi criada com 2 mapas.',
    );
  });

  it('says just "foi criada." when none of the three is there', () => {
    expect(createdSentenceTail(counts({ maps: 0, npcs: 0, characters: 0 }))).toBe(' foi criada.');
    expect(createdSentenceTail(undefined)).toBe(' foi criada.');
  });

  it('counts with the right noun', () => {
    expect(counted(1, 'mapa', 'mapas')).toBe('1 mapa');
    expect(counted(0, 'mapa', 'mapas')).toBe('0 mapas');
  });
});

describe('export texts', () => {
  // The browser's time zone: dates are built in it, so the clock reads the same on every machine.
  const now = new Date(2026, 9, 9, 19, 5);

  it('names the file <campanha>.meurpg.zip, without characters a file name cannot have', () => {
    expect(suggestedFileName('Mirathel')).toBe('Mirathel.meurpg.zip');
    expect(suggestedFileName('A/B: "C"?')).toBe('A B C.meurpg.zip');
    expect(suggestedFileName('  ')).toBe('campanha.meurpg.zip');
  });

  it('says "cerca de N MB" as a whole number, never zero', () => {
    expect(plain(approxSize(84.4 * MIB))).toBe('cerca de 84 MB');
    expect(plain(approxSize(1000))).toBe('cerca de 1 MB');
  });

  it('says hoje, amanhã, ontem or the date', () => {
    expect(dayWord(new Date(2026, 9, 9, 8, 0), now)).toBe('hoje');
    expect(dayWord(new Date(2026, 9, 10, 0, 1), now)).toBe('amanhã');
    expect(dayWord(new Date(2026, 9, 8, 23, 59), now)).toBe('ontem');
    expect(dayWord(new Date(2026, 9, 12, 19, 5), now)).toEqual({ date: '12/10' });
    // Across a month.
    expect(dayWord(new Date(2026, 10, 1, 9, 0), new Date(2026, 9, 31, 22, 0))).toBe('amanhã');
  });

  it('says how long the file is kept, until tomorrow at 19:05', () => {
    expect(expiryText(timestampFromDate(new Date(2026, 9, 10, 19, 5)), now)).toBe(
      'O arquivo fica guardado por 24 horas (até amanhã às 19:05) e depois é apagado. Só o mestre baixa.',
    );
    expect(expiryText(timestampFromDate(new Date(2026, 9, 12, 7, 30)), now)).toContain(
      '(até 12/10 às 07:30)',
    );
    expect(expiryText(undefined, now)).toBe(
      'O arquivo fica guardado por 24 horas e depois é apagado. Só o mestre baixa.',
    );
  });

  it('describes the last package: size, items and when it was made', () => {
    const exp = exportOf({
      byteSize: BigInt(Math.round(84.2 * MIB)),
      entryCount: 312,
      finishedAt: timestampFromDate(new Date(2026, 9, 9, 19, 5)),
    });
    expect(plain(lastPackageText(exp, now))).toBe('84,2 MB · 312 itens · gerado hoje às 19:05.');
    expect(plain(lastPackageText({ ...exp, entryCount: 1, finishedAt: undefined }, now))).toBe(
      '84,2 MB · 1 item.',
    );
    expect(
      lastPackageText({ ...exp, finishedAt: timestampFromDate(new Date(2026, 9, 8, 7, 0)) }, now),
    ).toContain('gerado ontem às 07:00');
    expect(
      lastPackageText({ ...exp, finishedAt: timestampFromDate(new Date(2026, 9, 1, 7, 0)) }, now),
    ).toContain('gerado em 01/10 às 07:00');
  });

  it('says where a running export is, with the board sentence at 62%', () => {
    expect(exportProgressText(62)).toBe(
      'Juntando os mapas e as imagens (62%). Pode fechar esta página: o arquivo fica pronto por 24 horas.',
    );
    expect(exportPhase(0)).toBe('Lendo a campanha');
    expect(exportPhase(85)).toBe('Fechando o arquivo');
    expect(exportPhase(100)).toBe('Guardando o arquivo');
    expect(exportProgressText(180)).toContain('(100%)');
    expect(exportProgressText(-5)).toContain('(0%)');
  });
});
