/**
 * The sentence under "Apagar Mirathel e arredores?" (E6-27): what goes with
 * the map, what stays, and the open session when this is its map.
 * "Os 5 pontos e os 4 tokens dele vão junto; a imagem continua na galeria.
 * É o mapa atual da Sessão 5: a sessão fica sem mapa até você escolher
 * outro. Não dá para desfazer."
 */
export function deleteMapConsequences(
  points: number,
  tokens: number,
  sessionNumber: number | null,
): string {
  const parts: string[] = [];
  if (points > 0) {
    parts.push(points === 1 ? 'o ponto' : `os ${points} pontos`);
  }
  if (tokens > 0) {
    parts.push(tokens === 1 ? 'o token' : `os ${tokens} tokens`);
  }
  let text: string;
  if (parts.length === 0) {
    text = 'A imagem continua na galeria.';
  } else {
    const what = parts.join(' e ');
    const plural = parts.length > 1 || points > 1 || tokens > 1;
    text = `${what[0].toUpperCase()}${what.slice(1)} dele ${plural ? 'vão' : 'vai'} junto; a imagem continua na galeria.`;
  }
  if (sessionNumber !== null) {
    text += ` É o mapa atual da Sessão ${sessionNumber}: a sessão fica sem mapa até você escolher outro.`;
  }
  return `${text} Não dá para desfazer.`;
}
