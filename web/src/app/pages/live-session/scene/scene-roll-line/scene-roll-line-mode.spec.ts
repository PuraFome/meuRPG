import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import { AdvantageSourceSchema, RollMode } from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import { SceneRollSchema } from '../../../../../gen/meurpg/play/v1/scene_pb';
import { SceneClient } from '../../../../core/play/scene-client';
import { SceneState } from '../../../../core/play/scene-state';
import { FakeSceneClient, playerScene } from '../../../../core/play/scene-testing';
import { SceneRollLine } from './scene-roll-line';

describe('SceneRollLine: the mode of a roll', () => {
  function render(roll: ReturnType<typeof create<typeof SceneRollSchema>>) {
    const api = new FakeSceneClient();
    TestBed.configureTestingModule({ providers: [{ provide: SceneClient, useValue: api }] });
    const fixture = TestBed.createComponent(SceneRollLine);
    fixture.componentRef.setInput('campaignId', 'c1');
    fixture.componentRef.setInput(
      'state',
      new SceneState(
        () => api.get(),
        () => false,
      ),
    );
    fixture.componentRef.setInput('scene', playerScene());
    fixture.componentRef.setInput('roll', roll);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('shows the disadvantage, the pair with the counted die and the sources of a character roll', () => {
    const el = render(
      create(SceneRollSchema, {
        id: 'r',
        actionId: 'a1',
        characterName: 'Pensantus',
        roll: {
          diceCount: 2,
          diceSides: 20,
          faces: [14, 3],
          modifier: 6,
          total: 9,
          countedIndex: 1,
        },
        mode: RollMode.DISADVANTAGE,
        sources: [
          create(AdvantageSourceSchema, {
            textPt: 'Envenenado: desvantagem em testes de habilidade',
          }),
        ],
      }),
    );
    expect(el.querySelector('.mode__word')?.textContent).toBe('Desvantagem');
    expect(
      Array.from(el.querySelectorAll('.die'), (d) => d.textContent?.replace(/\s+/g, ' ').trim()),
    ).toEqual(['14 não conta', '3 conta']);
    expect(el.querySelector('.mode__sources')?.textContent).toContain('Envenenado');
  });

  it('draws nothing more for a normal roll', () => {
    const el = render(
      create(SceneRollSchema, {
        id: 'r',
        actionId: 'a1',
        characterName: 'Pensantus',
        roll: { diceCount: 1, diceSides: 20, faces: [9], modifier: 6, total: 15 },
        mode: RollMode.NORMAL,
      }),
    );
    expect(el.querySelector('app-check-mode')).toBeNull();
  });
});
