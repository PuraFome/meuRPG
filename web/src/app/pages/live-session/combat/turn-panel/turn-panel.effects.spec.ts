import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import { CombatantKind } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { EffectNoteSchema } from '../../../../../gen/meurpg/play/v1/lasting_effects_pb';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { OrderStrip } from './order-strip';
import { OrderColumn } from './order-column';
import { TurnPanel } from './turn-panel';

const plain = (t: string | null | undefined) => (t ?? '').replace(/\s+/g, ' ').trim();

const brisa = combatant({
  id: 'brisa',
  label: 'Brisa',
  kind: CombatantKind.PLAYER,
  mine: true,
  conditionNamesPt: ['Paralisado'],
  conditions: ['condition:paralyzed'],
} as never);

describe('TurnPanel: what the effects take from the turn (RN-22)', () => {
  function panel(notes: readonly { textPt: string }[]) {
    const fixture = TestBed.createComponent(TurnPanel);
    fixture.componentRef.setInput(
      'encounter',
      encounter({ currentCombatantId: 'brisa', combatants: [brisa] }),
    );
    fixture.componentRef.setInput(
      'effectNotes',
      notes.map((n) => create(EffectNoteSchema, n)),
    );
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('says the server sentence with its first sentence in bold, in a live region', () => {
    const el = panel([{ textPt: 'Você está Paralisada. Não age nem se move neste turno.' }]);
    const note = el.querySelector<HTMLElement>('[data-testid="effect-note"]')!;
    expect(note.getAttribute('role')).toBe('status');
    expect(note.querySelector('strong')?.textContent).toBe('Você está Paralisada.');
    expect(plain(note.textContent)).toContain('Não age nem se move neste turno.');
    expect(note.classList).toContain('mr-notice');
  });

  it('draws nothing without notes, and one notice for each note', () => {
    expect(panel([]).querySelector('[data-testid="effect-note"]')).toBeNull();
    const el = panel([
      { textPt: 'Você está Impedido. Deslocamento 0.' },
      { textPt: 'Você está Enfeitiçado.' },
    ]);
    expect(el.querySelectorAll('[data-testid="effect-note"]')).toHaveLength(2);
  });
});

describe('the order and the labels of the effects (RN-22)', () => {
  const web = combatant({
    id: 'g1',
    label: 'Goblin 1',
    effectLabels: [{ effectId: 'e9', textPt: 'Preso numa teia' }],
  } as never);

  it.each([
    ['the strip of a phone (390 px)', OrderStrip],
    ['the column of a desktop (1280 px)', OrderColumn],
  ])('adds the label of an effect to the tags in %s, once beside the condition', (_name, cmp) => {
    const fixture = TestBed.createComponent(cmp as typeof OrderStrip);
    fixture.componentRef.setInput(
      'encounter',
      encounter({
        currentCombatantId: 'g1',
        combatants: [
          combatant({
            ...brisa,
            effectLabels: [{ effectId: 'e1', textPt: 'Paralisado' }],
          } as never),
          web,
        ],
      }),
    );
    fixture.detectChanges();
    const text = plain((fixture.nativeElement as HTMLElement).textContent);
    expect(text).toContain('Preso numa teia');
    expect(text.match(/Paralisado/g)).toHaveLength(1);
  });
});
