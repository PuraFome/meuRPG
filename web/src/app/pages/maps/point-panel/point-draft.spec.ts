import { MapPointKind } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { POINT_DESCRIPTION_MAX, changesOf, draftErrors, draftOf, isDirty } from './point-draft';

const taverna = {
  kind: MapPointKind.SCENE,
  name: 'Taverna do Javali',
  description: 'Onde a Velha Odra conta o que sabe.',
  revealed: false,
};
const torre = {
  kind: MapPointKind.SUBMAP,
  name: 'Torre',
  description: '',
  revealed: true,
  targetMap: { id: 'map-torre' },
};

describe('point draft', () => {
  it('starts equal to the saved point', () => {
    expect(isDirty(draftOf(taverna), taverna)).toBe(false);
    expect(isDirty(draftOf(torre), torre)).toBe(false);
    expect(changesOf(draftOf(taverna), taverna)).toBeNull();
  });

  it('sends only what changed', () => {
    const draft = { ...draftOf(taverna), name: '  Taverna nova  ', revealed: true };
    expect(isDirty(draft, taverna)).toBe(true);
    expect(changesOf(draft, taverna)).toEqual({ name: 'Taverna nova', revealed: true });
  });

  it('removes a Submapa target with an empty ID', () => {
    const draft = { ...draftOf(torre), targetMapId: '' };
    expect(changesOf(draft, torre)).toEqual({ targetMapId: '' });
  });

  it('ignores a leftover target when the kind is not Submapa', () => {
    const draft = { ...draftOf(torre), kind: MapPointKind.BATTLE };
    // The kind change alone: the server drops the target by itself.
    expect(changesOf(draft, torre)).toEqual({ kind: MapPointKind.BATTLE });
    const same = { ...draftOf(taverna), targetMapId: 'x' };
    expect(isDirty(same, taverna)).toBe(false);
  });

  it('checks the name and the description like the server', () => {
    expect(draftErrors({ ...draftOf(taverna), name: '   ' }).name).toBe('Dê um nome ao ponto.');
    expect(draftErrors({ ...draftOf(taverna), name: 'x'.repeat(81) }).name).toContain('80');
    expect(draftErrors({ ...draftOf(taverna), name: 'a\nb' }).name).toContain('uma linha');
    expect(
      draftErrors({ ...draftOf(taverna), description: 'x'.repeat(POINT_DESCRIPTION_MAX + 1) })
        .description,
    ).toContain('2.000');
    expect(draftErrors(draftOf(taverna))).toEqual({});
  });

  describe('the hooks of a Cena (MR-029)', () => {
    const cena = { ...taverna, hooks: 'O mercador foi levado.' };

    it('start equal to the saved point, and are sent alone when they change', () => {
      expect(isDirty(draftOf(cena), cena)).toBe(false);
      const draft = { ...draftOf(cena), hooks: 'Mira está escondida.' };
      expect(isDirty(draft, cena)).toBe(true);
      expect(changesOf(draft, cena)).toEqual({ hooks: 'Mira está escondida.' });
      // Emptying them clears them.
      expect(changesOf({ ...draftOf(cena), hooks: '' }, cena)).toEqual({ hooks: '' });
    });

    it('are refused past 4.000 characters, counting characters and not UTF-16 units', () => {
      expect(draftErrors({ ...draftOf(cena), hooks: '😀'.repeat(4000) })).toEqual({});
      expect(draftErrors({ ...draftOf(cena), hooks: 'x'.repeat(4001) }).hooks).toBe(
        'Use até 4.000 caracteres.',
      );
    });

    it('are not sent when the point stops being a Cena', () => {
      const draft = { ...draftOf(cena), kind: MapPointKind.BATTLE };
      expect(changesOf(draft, cena)).toEqual({ kind: MapPointKind.BATTLE });
      // Typed hooks on a point that is not a Cena change nothing.
      const battle = { ...taverna, kind: MapPointKind.BATTLE };
      expect(isDirty({ ...draftOf(battle), hooks: 'x' }, battle)).toBe(false);
    });
  });
});
