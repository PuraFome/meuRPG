import { deleteMapConsequences } from './map-head-copy';

describe('deleteMapConsequences (E6-27)', () => {
  it('says what goes with the map and what stays', () => {
    expect(deleteMapConsequences(5, 4, null)).toBe(
      'Os 5 pontos e os 4 tokens dele vão junto; a imagem continua na galeria. Não dá para desfazer.',
    );
  });

  it('uses the singular for one point or one token', () => {
    expect(deleteMapConsequences(1, 0, null)).toBe(
      'O ponto dele vai junto; a imagem continua na galeria. Não dá para desfazer.',
    );
    expect(deleteMapConsequences(0, 1, null)).toBe(
      'O token dele vai junto; a imagem continua na galeria. Não dá para desfazer.',
    );
    expect(deleteMapConsequences(1, 1, null)).toBe(
      'O ponto e o token dele vão junto; a imagem continua na galeria. Não dá para desfazer.',
    );
    expect(deleteMapConsequences(3, 1, null)).toBe(
      'Os 3 pontos e o token dele vão junto; a imagem continua na galeria. Não dá para desfazer.',
    );
  });

  it('leaves the list out of an empty map', () => {
    expect(deleteMapConsequences(0, 0, null)).toBe('A imagem continua na galeria. Não dá para desfazer.');
  });

  it("warns when it is the open session's map", () => {
    expect(deleteMapConsequences(5, 4, 5)).toBe(
      'Os 5 pontos e os 4 tokens dele vão junto; a imagem continua na galeria. ' +
        'É o mapa atual da Sessão 5: a sessão fica sem mapa até você escolher outro. Não dá para desfazer.',
    );
  });
});
