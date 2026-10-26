import { ComponentFixture, TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { of } from 'rxjs';

import { type SceneImage, SceneImageSchema } from '../../../../../gen/meurpg/maps/v1/maps_pb';
import { sceneAudienceView } from '../../../../core/play/scene-testing';
import { LiveSessionSource, type ShownImageVm } from '../../../../core/play/live-session.types';
import { SceneImagesShow } from './scene-images';
import { nextTurn } from '../../../../../test-settle';

function image(id: string, name: string, wholeMap = false): SceneImage {
  return create(SceneImageSchema, { id, name, showsWholeMap: wholeMap });
}

function vm(id: string, name: string): ShownImageVm {
  return { id, name, width: 10, height: 10, url: `/images/${id}` };
}

describe("SceneImagesShow (the master's open scene)", () => {
  let fixture: ComponentFixture<SceneImagesShow>;
  let el: HTMLElement;
  let source: {
    setShownImage: ReturnType<typeof vi.fn>;
    setShownImageAudience: ReturnType<typeof vi.fn>;
  };
  const dialogOpen = vi.fn();
  let changed: (ShownImageVm | null)[];
  let left: number;

  function setup(
    images: SceneImage[],
    shown: ShownImageVm | null = null,
    keep = false,
    audience: ReturnType<typeof sceneAudienceView> | null = null,
  ) {
    source = {
      setShownImage: vi.fn(async (_c: string, id: string | null) => (id ? vm(id, id) : null)),
      setShownImageAudience: vi.fn(async () => vm('i1', 'A carroça')),
    };
    dialogOpen.mockReset().mockReturnValue({ afterClosed: () => of(undefined) });
    changed = [];
    left = 0;
    TestBed.configureTestingModule({
      providers: [
        { provide: LiveSessionSource, useValue: source },
        { provide: MatDialog, useValue: { open: dialogOpen } },
        { provide: MatBottomSheet, useValue: { open: dialogOpen } },
      ],
    });
    fixture = TestBed.createComponent(SceneImagesShow);
    fixture.componentRef.setInput('campaignId', 'c1');
    fixture.componentRef.setInput('images', images);
    fixture.componentRef.setInput('shown', shown);
    fixture.componentRef.setInput('keep', keep);
    fixture.componentRef.setInput('audience', audience);
    fixture.componentInstance.changed.subscribe((s) => changed.push(s));
    fixture.componentInstance.leftChanged.subscribe(() => left++);
    fixture.detectChanges();
    el = fixture.nativeElement;
  }

  async function settle(): Promise<void> {
    for (let i = 0; i < 3; i++) {
      await fixture.whenStable();
      await nextTurn();
      fixture.detectChanges();
    }
  }

  const tile = (id: string) => el.querySelector(`[data-image="${id}"]`)!;
  /** The thumbnail is the button that shows the image; the questions of a whole-map image sit under the row. */
  const button = (id: string, text: string) =>
    text === 'Mostrar aos jogadores'
      ? tile(id).querySelector<HTMLButtonElement>('button.sm__pic')!
      : Array.from(el.querySelectorAll<HTMLButtonElement>('.sm__ask button')).find((b) =>
          b.textContent?.includes(text),
        )!;

  it('with no images draws the title, "0 de 8" and the place of the gallery tile', () => {
    setup([]);
    expect(el.querySelector('h3')?.textContent).toContain('Imagens da cena');
    expect(el.querySelector('.sm__count')?.textContent).toContain('0');
    expect(el.querySelectorAll('.sm__tile--add')).toHaveLength(1);
  });

  it('lists the images, each with "Mostrar aos jogadores"', () => {
    setup([image('i1', 'A carroça'), image('i2', 'A estrada')]);
    expect(el.querySelector('h3')?.textContent).toContain('Imagens da cena');
    expect(el.querySelectorAll('.sm__tile:not(.sm__tile--add)').length).toBe(2);
    expect(button('i1', 'Mostrar aos jogadores').getAttribute('aria-label')).toBe(
      'Mostrar A carroça aos jogadores',
    );
  });

  it('shows through the gallery action and tells the page', async () => {
    setup([image('i1', 'A carroça')]);
    button('i1', 'Mostrar aos jogadores').click();
    await settle();
    expect(source.setShownImage).toHaveBeenCalledWith('c1', 'i1');
    expect(changed.map((s) => s?.id)).toEqual(['i1']);
    expect(el.querySelector('[role="status"]')?.textContent).toContain(
      'A carroça está na tela dos jogadores',
    );
  });

  it("marks the image on display; stopping it is the scene row's button, not the tile's", () => {
    setup([image('i1', 'A carroça'), image('i2', 'A estrada')], vm('i1', 'A carroça'));
    expect(tile('i1').textContent).toContain('À mostra');
    expect(tile('i1').classList).toContain('sm__tile--shown');
    expect(tile('i2').textContent).not.toContain('À mostra');
    expect(tile('i1').querySelector('button')).toBeNull();
  });

  it('with "Deixar com os jogadores" on, replacing the image tells the page to read the left list', async () => {
    setup([image('i2', 'A estrada')], vm('i1', 'A carroça'), true);
    button('i2', 'Mostrar aos jogadores').click();
    await settle();
    expect(left).toBe(1);
  });

  it('an image that shows the whole map asks first, in place', async () => {
    setup([image('m1', 'Mapa texturizado', true)]);
    button('m1', 'Mostrar aos jogadores').click();
    await settle();
    expect(source.setShownImage).not.toHaveBeenCalled();
    expect(el.querySelector('.sm__ask')?.textContent).toContain('mostra o mapa inteiro');
    button('m1', 'Voltar').click();
    await settle();
    expect(button('m1', 'Mostrar aos jogadores')).toBeTruthy();
    button('m1', 'Mostrar aos jogadores').click();
    await settle();
    button('m1', 'Mostrar mesmo assim').click();
    await settle();
    expect(source.setShownImage).toHaveBeenCalledWith('c1', 'm1');
  });

  it('a refusal is written out', async () => {
    setup([image('i1', 'A carroça')]);
    source.setShownImage.mockRejectedValueOnce(new Error('x'));
    button('i1', 'Mostrar aos jogadores').click();
    await settle();
    expect(el.querySelector('[role="alert"]')).not.toBeNull();
    expect(changed).toEqual([]);
  });

  describe('a scene open to only some players (MR-028, C6-11)', () => {
    const partial = () => sceneAudienceView(['Pensantus', 'Brisa'], ['Toren']);

    async function openMenu(id: string) {
      tile(id).querySelector<HTMLButtonElement>('button.sm__pic')!.click();
      await settle();
    }
    const item = (text: string) =>
      Array.from(document.querySelectorAll<HTMLButtonElement>('[mat-menu-item]')).find((b) =>
        b.textContent?.includes(text),
      )!;

    it('opens a menu on a thumbnail instead of showing at once, with who each choice reaches', async () => {
      setup(
        [image('i1', 'Vista do salão'), image('i2', 'O porão')],
        vm('i1', 'x'),
        false,
        partial(),
      );
      expect(button('i2', 'Mostrar aos jogadores').getAttribute('aria-label')).toBe(
        'Mostrar O porão para…',
      );
      await openMenu('i2');
      expect(source.setShownImage).not.toHaveBeenCalled();
      expect(document.querySelector('.sm-menu__head')?.textContent).toContain(
        'Mostrar “O porão” para',
      );
      expect(item('Só quem está na cena').textContent).toContain('Pensantus e Brisa');
      expect(item('Toda a mesa').textContent).toContain('Pensantus, Brisa e Toren');
      expect(item('Escolher quem vê').textContent).toBeTruthy();
    });

    it('"Só quem está na cena" shows it to the scene\'s players, "Toda a mesa" to the table', async () => {
      setup([image('i2', 'O porão')], null, false, partial());
      await openMenu('i2');
      item('Só quem está na cena').click();
      await settle();
      expect(source.setShownImage).toHaveBeenLastCalledWith('c1', 'i2', undefined, {
        kind: 'scene',
        userIds: [],
      });
      await openMenu('i2');
      item('Toda a mesa').click();
      await settle();
      expect(source.setShownImage).toHaveBeenLastCalledWith('c1', 'i2', undefined, {
        kind: 'whole',
        userIds: [],
      });
    });

    it('"Escolher quem vê…" opens the chooser and shows it to those chosen', async () => {
      setup([image('i2', 'O porão')], null, false, partial());
      dialogOpen.mockReturnValue({
        afterClosed: () => of({ everyone: false, userIds: ['u-brisa', 'u-toren'] }),
      });
      await openMenu('i2');
      item('Escolher quem vê').click();
      await settle();
      expect(dialogOpen.mock.calls[0][1].data.title).toBe('Quem vê “O porão”?');
      expect(source.setShownImage).toHaveBeenCalledWith('c1', 'i2', undefined, {
        kind: 'chosen',
        userIds: ['u-brisa', 'u-toren'],
      });
    });

    it('says who sees the image on display, with "Mostrar a toda a mesa" and "Mudar quem vê"', () => {
      setup(
        [image('i1', 'Vista do salão')],
        { ...vm('i1', 'Vista do salão'), audience: { kind: 'scene', userIds: [] } },
        false,
        partial(),
      );
      expect(el.querySelector('.sm__seeing')?.textContent?.trim()).toContain(
        'Pensantus e Brisa veem',
      );
      const names = Array.from(el.querySelectorAll('.sm__who button')).map((b) =>
        b.textContent?.trim(),
      );
      expect(names).toEqual(['groupsMostrar a toda a mesa', 'editMudar quem vê']);
    });

    it('"Mostrar a toda a mesa" widens the audience and tells the page', async () => {
      setup(
        [image('i1', 'Vista do salão')],
        { ...vm('i1', 'Vista do salão'), audience: { kind: 'scene', userIds: [] } },
        false,
        partial(),
      );
      Array.from(el.querySelectorAll<HTMLButtonElement>('.sm__who button'))[0].click();
      await settle();
      expect(source.setShownImageAudience).toHaveBeenCalledWith('c1', {
        kind: 'whole',
        userIds: [],
      });
      expect(changed).toHaveLength(1);
    });

    it('"Mudar quem vê" opens the chooser with the viewers now, and sends the new ones', async () => {
      setup(
        [image('i1', 'Vista do salão')],
        { ...vm('i1', 'Vista do salão'), audience: { kind: 'scene', userIds: [] } },
        false,
        partial(),
      );
      dialogOpen.mockReturnValue({
        afterClosed: () => of({ everyone: false, userIds: ['u-brisa'] }),
      });
      Array.from(el.querySelectorAll<HTMLButtonElement>('.sm__who button'))[1].click();
      await settle();
      const data = dialogOpen.mock.calls[0][1].data;
      expect(data.title).toBe('Quem vê “Vista do salão”?');
      expect(data.selected).toEqual(['u-pensantus', 'u-brisa']);
      expect(source.setShownImageAudience).toHaveBeenCalledWith('c1', {
        kind: 'chosen',
        userIds: ['u-brisa'],
      });
    });

    it('with the scene open to the whole table the thumbnail shows at once and the line says "Os jogadores veem"', async () => {
      setup(
        [image('i2', 'O porão')],
        vm('i1', 'x'),
        false,
        sceneAudienceView(['Pensantus', 'Brisa', 'Toren'], [], true),
      );
      expect(el.querySelector('.sm__seeing')?.textContent?.trim()).toContain('Os jogadores veem');
      expect(el.querySelectorAll('.sm__who button')).toHaveLength(1);
      button('i2', 'Mostrar aos jogadores').click();
      await settle();
      expect(source.setShownImage).toHaveBeenCalledWith('c1', 'i2');
    });

    it('writes out a refusal of the audience', async () => {
      setup(
        [image('i1', 'Vista do salão')],
        { ...vm('i1', 'Vista do salão'), audience: { kind: 'scene', userIds: [] } },
        false,
        partial(),
      );
      source.setShownImageAudience.mockRejectedValueOnce(new Error('x'));
      Array.from(el.querySelectorAll<HTMLButtonElement>('.sm__who button'))[0].click();
      await settle();
      expect(el.querySelector('[role="alert"]')).not.toBeNull();
    });
  });
});
