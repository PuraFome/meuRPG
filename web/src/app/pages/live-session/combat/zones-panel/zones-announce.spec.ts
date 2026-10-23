import { zoneNotice } from './zones-announce';

const raio = { id: 'a', label: 'Raio Lunar · Bartolo' };
const teia = { id: 'b', label: 'Teia · Pensantus' };

describe('what the zones list says when it changes', () => {
  it('says nothing on the first read, nor when nothing changed', () => {
    expect(zoneNotice(null, [raio, teia])).toBe('');
    expect(zoneNotice([raio], [raio])).toBe('');
  });

  it('names the zone that appeared', () => {
    expect(zoneNotice([raio], [raio, teia])).toBe('Zona criada: Teia · Pensantus.');
  });

  it('names the zone that ended', () => {
    expect(zoneNotice([raio, teia], [teia])).toBe('Zona encerrada: Raio Lunar · Bartolo.');
  });

  it('says both when one ended as another began, and joins several', () => {
    expect(zoneNotice([raio], [teia])).toBe(
      'Zona criada: Teia · Pensantus. Zona encerrada: Raio Lunar · Bartolo.',
    );
    expect(zoneNotice([], [raio, teia])).toBe(
      'Zonas criadas: Raio Lunar · Bartolo, Teia · Pensantus.',
    );
  });
});
