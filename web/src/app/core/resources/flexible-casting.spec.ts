import {
  SLOT_CREATION_COSTS,
  convertPreview,
  convertRows,
  createPreview,
  createRows,
  fullText,
} from './flexible-casting';
import { plainText } from './plain-text';

describe('the SRD table of Creating Spell Slots', () => {
  it('costs 2, 3, 5, 6 and 7 points for the levels 1 to 5', () => {
    expect(SLOT_CREATION_COSTS).toEqual({ 1: 2, 2: 3, 3: 5, 4: 6, 5: 7 });
  });
});

describe('createRows ("Pontos → espaço")', () => {
  it('lists the five levels with their cost, and greys the ones the points do not pay for with the reason written', () => {
    expect(plainText(createRows({ left: 5, total: 5 }))).toEqual([
      { level: 1, title: 'Criar um espaço de 1º nível', sub: 'custa 2 pontos', blocked: '' },
      { level: 2, title: 'Criar um espaço de 2º nível', sub: 'custa 3 pontos', blocked: '' },
      { level: 3, title: 'Criar um espaço de 3º nível', sub: 'custa 5 pontos', blocked: '' },
      {
        level: 4,
        title: 'Criar um espaço de 4º nível',
        sub: '',
        blocked: 'custa 6 pontos (você tem 5)',
      },
      {
        level: 5,
        title: 'Criar um espaço de 5º nível',
        sub: '',
        blocked: 'custa 7 pontos (você tem 5)',
      },
    ]);
  });

  it('blocks every level with no points', () => {
    expect(plainText(createRows({ left: 1, total: 5 })).every((r) => r.blocked !== '')).toBe(true);
  });
});

describe('convertRows ("Espaço → pontos")', () => {
  it('lists the free slots, each giving its level in points', () => {
    expect(
      plainText(
        convertRows([
          { level: 1, total: 4, used: 1 },
          { level: 2, total: 3, used: 3 },
          { level: 3, total: 2, used: 0 },
        ]),
      ),
    ).toEqual([
      { level: 1, title: '1º nível', sub: '3 livres de 4 · +1 ponto', blocked: '' },
      { level: 3, title: '3º nível', sub: '2 livres de 2 · +3 pontos', blocked: '' },
    ]);
  });
});

describe('the sentences', () => {
  it('says what creating a slot costs and that it vanishes on a long rest', () => {
    expect(plainText(createPreview(2, { left: 5, total: 5 }))).toBe(
      'Gasta 3 pontos (restam 2) e cria um espaço de 2º nível. O espaço criado some no descanso longo. Ação bônus.',
    );
  });

  it('says what converting a slot gives, never past the maximum', () => {
    expect(plainText(convertPreview(2, { left: 2, total: 5 }))).toBe(
      'Gasta um espaço de 2º nível e ganha 2 pontos de feitiçaria: ficam em 4 de 5. Ação bônus.',
    );
    expect(plainText(convertPreview(3, { left: 4, total: 5 }))).toContain('ficam em 5 de 5');
  });

  it('refuses a conversion with the points at the maximum, naming the maximum', () => {
    expect(fullText({ left: 5, total: 5 })).toBe(
      'Você já tem o máximo de pontos de feitiçaria. O máximo é o seu nível (5): converter um espaço agora perderia os pontos.',
    );
  });
});
