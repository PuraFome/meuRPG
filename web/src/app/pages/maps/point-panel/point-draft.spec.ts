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
});
