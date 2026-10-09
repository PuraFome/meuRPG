import {
  PackageProblemKind,
  PackageProblemReason,
} from '../../../gen/meurpg/campaignpackage/v1/campaignpackage_pb';
import { plain } from '../images/gallery-testing';
import { cleanName, hasNewerVersion, problemSentence as sentenceOf } from './package-problems';

/** The sentence with its no-break spaces read as plain ones, so the expectations stay readable. */
const problemSentence = (p: Parameters<typeof sentenceOf>[0]) => plain(sentenceOf(p));

function problem(
  kind: PackageProblemKind,
  reason: PackageProblemReason,
  name = '',
  limit = 0,
): Parameters<typeof sentenceOf>[0] {
  return { kind, reason, name, limit: BigInt(limit) };
}

/** Every value of a generated enum (its reverse mapping holds the names too). */
function valuesOf(e: Record<string, string | number>): number[] {
  return Object.values(e).filter((v): v is number => typeof v === 'number');
}

const KINDS = valuesOf(PackageProblemKind);
const REASONS = valuesOf(PackageProblemReason);

describe('problemSentence', () => {
  it('says the board sentences word for word', () => {
    expect(
      problemSentence(
        problem(PackageProblemKind.IMAGE, PackageProblemReason.ENTRY_TOO_BIG, 'Mapa antigo.png'),
      ),
    ).toBe('A imagem “Mapa antigo.png” tem mais de 10 MB.');
    expect(
      problemSentence(
        problem(PackageProblemKind.SCENE, PackageProblemReason.UNKNOWN_CONTENT, 'A ponte quebrada'),
      ),
    ).toBe('A cena “A ponte quebrada” usa uma ação de teste que este servidor ainda não conhece.');
    expect(
      problemSentence(
        problem(PackageProblemKind.CHARACTER, PackageProblemReason.NOT_IN_PACKAGE, 'Orla'),
      ),
    ).toBe('O personagem “Orla” usa uma classe da mesa que não está no conteúdo do pacote.');
  });

  it('says the newer-version sentence of the board for the whole package', () => {
    expect(
      problemSentence(problem(PackageProblemKind.PACKAGE, PackageProblemReason.NEWER_VERSION)),
    ).toBe(
      'Este pacote é de uma versão mais nova do MeuRPG, e este servidor ainda não sabe abri-lo.',
    );
  });

  it('answers every kind with every reason: a sentence, never an enum name or a raw value', () => {
    for (const kind of KINDS) {
      for (const reason of REASONS) {
        for (const name of ['', 'Mirathel']) {
          const text = problemSentence(problem(kind, reason, name, 300));
          const where = `${PackageProblemKind[kind]} × ${PackageProblemReason[reason]} × "${name}"`;
          expect(text.length, where).toBeGreaterThan(10);
          expect(text.endsWith('.'), where).toBe(true);
          // No SCREAMING_CASE enum names, no "undefined", no "[object", no raw numbers of a bigint.
          expect(text, where).not.toMatch(/[A-Z]{3,}_[A-Z_]+/);
          expect(text, where).not.toMatch(/undefined|null|\[object|NaN/);
          expect(text, where).not.toContain('PROBLEM');
          // The banned wording of the review (decisions-pm09.md).
          expect(text, where).not.toMatch(/tipo que este servidor/);
        }
      }
    }
  });

  it('names the thing for every reason that is about one thing', () => {
    const aboutAThing = REASONS.filter(
      (r) =>
        ![
          PackageProblemReason.UNSPECIFIED,
          PackageProblemReason.NOT_A_PACKAGE,
          PackageProblemReason.NEWER_VERSION,
          PackageProblemReason.PACKAGE_TOO_BIG,
          PackageProblemReason.TOO_MANY_ENTRIES,
          PackageProblemReason.LIMIT,
        ].includes(r),
    );
    for (const kind of KINDS.filter((k) => k !== PackageProblemKind.UNSPECIFIED)) {
      for (const reason of aboutAThing) {
        const text = problemSentence(problem(kind, reason, 'Orla'));
        expect(text, `${PackageProblemKind[kind]} × ${PackageProblemReason[reason]}`).toContain(
          '“Orla”',
        );
      }
    }
  });

  it('says "um item do pacote" for an unnamed entry of the file as a whole', () => {
    expect(
      problemSentence(problem(PackageProblemKind.PACKAGE, PackageProblemReason.ENTRY_TOO_BIG)),
    ).toBe('Um item do pacote tem mais de 10 MB.');
    expect(
      problemSentence(problem(PackageProblemKind.IMAGE, PackageProblemReason.IMAGE_UNREADABLE)),
    ).toBe('Uma imagem não é um JPEG, PNG ou WebP que dê para ler.');
  });

  it('names what an unknown thing is, by the kind of the item', () => {
    const say = (kind: PackageProblemKind) =>
      problemSentence(problem(kind, PackageProblemReason.UNKNOWN_CONTENT, 'X'));
    expect(say(PackageProblemKind.NPC)).toBe(
      'O NPC “X” usa uma criatura que este servidor ainda não conhece.',
    );
    expect(say(PackageProblemKind.ENCOUNTER)).toContain('uma criatura');
    expect(say(PackageProblemKind.MAP)).toContain('armadilha');
    expect(say(PackageProblemKind.CAMPAIGN)).toContain('regra');
  });

  it('turns a LIMIT into the campaign passing a count, or a size', () => {
    expect(
      problemSentence(problem(PackageProblemKind.IMAGE, PackageProblemReason.LIMIT, '', 300)),
    ).toBe('A campanha passa do limite de 300 imagens.');
    expect(
      problemSentence(
        problem(PackageProblemKind.IMAGE, PackageProblemReason.LIMIT, '', 500 * 1024 * 1024),
      ),
    ).toBe('A campanha passa do limite de 500 MB.');
    expect(
      problemSentence(problem(PackageProblemKind.MAP, PackageProblemReason.LIMIT, '', 0)),
    ).toBe('A campanha passa de um dos limites que cada campanha tem.');
  });

  it('gives the package limits in plain numbers', () => {
    expect(
      problemSentence(problem(PackageProblemKind.PACKAGE, PackageProblemReason.PACKAGE_TOO_BIG)),
    ).toBe('O pacote passa do limite de 200 MB.');
    expect(
      problemSentence(problem(PackageProblemKind.PACKAGE, PackageProblemReason.TOO_MANY_ENTRIES)),
    ).toBe('O pacote tem itens demais: o limite é 2.000.');
  });

  it('falls back for a reason or kind this app does not know, still in plain words', () => {
    expect(
      problemSentence(problem(PackageProblemKind.SCENE, 99 as PackageProblemReason, 'A ponte')),
    ).toBe('Não dá para criar a cena “A ponte”.');
    expect(
      problemSentence(problem(PackageProblemKind.UNSPECIFIED, PackageProblemReason.UNSPECIFIED)),
    ).toBe('Não dá para criar um item.');
    expect(
      problemSentence(problem(99 as PackageProblemKind, PackageProblemReason.INVALID, 'Z')),
    ).toBe('O item “Z” tem dados fora do que o app aceita (um tamanho, um valor ou um nome).');
  });
});

describe('cleanName', () => {
  it('drops control characters, folds spaces and cuts a long name', () => {
    expect(cleanName('  Mapa\u0000 \n antigo\tI  ')).toBe('Mapa antigo I');
    const long = cleanName('x'.repeat(200));
    expect(long.length).toBe(60);
    expect(long.endsWith('…')).toBe(true);
  });

  it('is what a sentence prints, so a hostile name stays one line', () => {
    expect(
      problemSentence(
        problem(PackageProblemKind.MAP, PackageProblemReason.INVALID, 'a\r\nb\u0007c'),
      ),
    ).toContain('“a b c”');
  });
});

describe('hasNewerVersion', () => {
  it('finds the problem that has its own card', () => {
    expect(hasNewerVersion([{ reason: PackageProblemReason.NEWER_VERSION }])).toBe(true);
    expect(
      hasNewerVersion([
        { reason: PackageProblemReason.INVALID },
        { reason: PackageProblemReason.NEWER_VERSION },
      ]),
    ).toBe(true);
    expect(hasNewerVersion([{ reason: PackageProblemReason.INVALID }])).toBe(false);
    expect(hasNewerVersion([])).toBe(false);
  });
});
