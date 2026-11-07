import { TestBed } from '@angular/core/testing';

import type { GearVm } from '../../../core/characters/gear';
import type { WeaponMarkVm } from '../character-sheet.types';
import { WeaponMarks } from './weapon-marks';

const weapon = (key: string, namePt: string): GearVm => ({
  key,
  namePt,
  kind: 'weapon',
  equipped: true,
  twoHanded: false,
  armorClassIfToggled: 0,
  count: 1,
});
const none = { silvered: false, adamantine: false, magical: false };

function setup(marks: WeaponMarkVm[] = [], extra: Record<string, unknown> = {}) {
  const fixture = TestBed.createComponent(WeaponMarks);
  fixture.componentRef.setInput('weapons', [
    weapon('equipment:dagger', 'Adaga'),
    weapon('equipment:longsword', 'Espada longa'),
  ]);
  fixture.componentRef.setInput('marks', marks);
  for (const [k, v] of Object.entries(extra)) {
    fixture.componentRef.setInput(k, v);
  }
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const saved: WeaponMarkVm[][] = [];
  fixture.componentInstance.saved.subscribe((m) => saved.push(m));
  const box = (weaponName: string, word: string): HTMLInputElement => {
    const fieldset = Array.from(el.querySelectorAll('fieldset')).find(
      (f) => f.querySelector('legend')!.textContent!.trim() === weaponName,
    )!;
    const label = Array.from(fieldset.querySelectorAll('label')).find(
      (l) => l.querySelector('.marks__word')!.textContent === word,
    )!;
    return label.querySelector('input')!;
  };
  const submit = () =>
    el.querySelector('form')!.dispatchEvent(new Event('submit', { cancelable: true }));
  return { fixture, el, saved, box, submit };
}

describe('WeaponMarks ("Marcar as armas")', () => {
  it('has a row for each weapon and the three marks, with when each one applies', () => {
    const { el } = setup();
    expect(Array.from(el.querySelectorAll('legend')).map((l) => l.textContent!.trim())).toEqual([
      'Adaga',
      'Espada longa',
    ]);
    expect(
      Array.from(el.querySelectorAll('fieldset:first-of-type .marks__word')).map(
        (w) => w.textContent,
      ),
    ).toEqual(['Prateada', 'Adamantina', 'Mágica']);
    expect(el.querySelector('.marks__against')!.textContent).toContain('Vale contra');
  });

  it('starts from the marks the sheet has', () => {
    const { box } = setup([{ weaponKey: 'equipment:dagger', ...none, silvered: true }]);
    expect(box('Adaga', 'Prateada').checked).toBe(true);
    expect(box('Adaga', 'Mágica').checked).toBe(false);
    expect(box('Espada longa', 'Prateada').checked).toBe(false);
  });

  it('hands over only the weapons that have a mark, and a plain one has none', () => {
    const { box, saved, submit } = setup([
      { weaponKey: 'equipment:dagger', ...none, silvered: true },
    ]);
    box('Espada longa', 'Mágica').checked = true;
    box('Espada longa', 'Mágica').dispatchEvent(new Event('change'));
    submit();
    expect(saved).toEqual([
      [
        { weaponKey: 'equipment:dagger', silvered: true, adamantine: false, magical: false },
        { weaponKey: 'equipment:longsword', silvered: false, adamantine: false, magical: true },
      ],
    ]);
  });

  it('drops a weapon whose last mark was turned off', () => {
    const { box, saved, submit } = setup([
      { weaponKey: 'equipment:dagger', ...none, silvered: true },
    ]);
    box('Adaga', 'Prateada').checked = false;
    box('Adaga', 'Prateada').dispatchEvent(new Event('change'));
    submit();
    expect(saved).toEqual([[]]);
  });

  it('keeps the stored mark of a weapon the grid has no row for', () => {
    const { box, saved, submit } = setup([
      { weaponKey: 'equipment:gone', ...none, adamantine: true },
      { weaponKey: 'equipment:dagger', ...none, silvered: true },
    ]);
    box('Espada longa', 'Mágica').checked = true;
    box('Espada longa', 'Mágica').dispatchEvent(new Event('change'));
    submit();
    expect(saved[0].map((m) => m.weaponKey)).toEqual([
      'equipment:gone',
      'equipment:dagger',
      'equipment:longsword',
    ]);
  });

  it('shows through the marks of a sheet read again for the weapons not touched here', () => {
    const { fixture, box } = setup([]);
    box('Espada longa', 'Mágica').checked = true;
    box('Espada longa', 'Mágica').dispatchEvent(new Event('change'));
    fixture.componentRef.setInput('marks', [
      { weaponKey: 'equipment:dagger', ...none, silvered: true },
    ]);
    fixture.detectChanges();
    expect(box('Adaga', 'Prateada').checked).toBe(true);
    expect(box('Espada longa', 'Mágica').checked).toBe(true);
  });

  it('says the error in an alert and holds the toggles while it saves', () => {
    const { el, box } = setup([], { error: 'A ficha mudou.', saving: true });
    expect(el.querySelector('[role="alert"]')!.textContent).toContain('A ficha mudou.');
    expect(box('Adaga', 'Prateada').disabled).toBe(true);
  });

  it('puts the focus on the first toggle when it opens', async () => {
    const fixture = TestBed.createComponent(WeaponMarks);
    fixture.componentRef.setInput('weapons', [weapon('equipment:dagger', 'Adaga')]);
    fixture.componentRef.setInput('marks', []);
    document.body.appendChild(fixture.nativeElement);
    fixture.detectChanges();
    await fixture.whenStable();
    expect(document.activeElement).toBe(
      (fixture.nativeElement as HTMLElement).querySelector('input'),
    );
    (fixture.nativeElement as HTMLElement).remove();
  });
});
