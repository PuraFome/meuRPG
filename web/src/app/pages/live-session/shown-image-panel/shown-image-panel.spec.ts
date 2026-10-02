import { Code, ConnectError } from '@connectrpc/connect';

import { mapMessage } from '../../../core/maps/maps-testing';
import { hiddenMapImages, showErrorMessage, shownImageNote } from './shown-image-panel';

describe('shownImageNote', () => {
  it('says what the players see the first time, and what replaces what on a swap', () => {
    expect(shownImageNote({ name: 'Carta' }, null)).toBe(
      'Os jogadores veem a imagem na hora, com o nome dela como legenda. O mapa atual continua na tela deles.',
    );
    expect(shownImageNote({ name: 'Taverna do Javali' }, { name: 'Capitão Goblin' })).toBe(
      'Os jogadores passam a ver Taverna do Javali no lugar de Capitão Goblin.',
    );
  });
});

describe('hiddenMapImages', () => {
  it('lists the images that are the background of a hidden map, with the map', () => {
    const maps = [
      mapMessage('a', 'Mirathel', { revealed: true }),
      mapMessage('b', 'Covil dos goblins', { revealed: false }),
    ];
    expect([...hiddenMapImages(maps)]).toEqual([['img-b', 'Covil dos goblins']]);
  });
});

describe('showErrorMessage', () => {
  it('maps the codes of SetShownImage', () => {
    expect(showErrorMessage(new ConnectError('x', Code.FailedPrecondition))).toContain('A sessão acabou');
    expect(showErrorMessage(new ConnectError('x', Code.NotFound))).toContain('não está mais na galeria');
    expect(showErrorMessage(new ConnectError('x', Code.PermissionDenied))).toContain('Só o mestre');
    expect(showErrorMessage(new TypeError('Failed to fetch'))).toContain('Tente de novo');
  });
});
