import { ComponentFixture, TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import { type SceneAction, SceneActionSchema } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { MapsClient } from '../../../core/maps/maps-client';
import { FakeMapsClient } from '../../../core/maps/maps-testing';
import { SceneChecks } from '../../../core/maps/scene-actions';
import { SceneActions } from './scene-actions';

function action(id: string, checkName: string, extra: Partial<Omit<SceneAction, '$typeName'>> = {}): SceneAction {
  return create(SceneActionSchema, { id, key: 'skill:x', checkName, ...extra });
}

const FIVE = [
  action('a1', 'Investigação', { name: 'Procurar pistas na carroça', dc: 12 }),
  action('a2', 'Sobrevivência', { name: 'Seguir os rastros dos goblins', dc: 13 }),
  action('a3', 'Adestrar Animais', { name: 'Acalmar os cavalos' }),
  action('a4', 'Percepção', { key: 'skill:perception' }),
  action('a5', 'Salvaguarda de Constituição', { name: 'Resistir ao cheiro de fumaça', dc: 10, key: 'save:con' }),
];

describe('SceneActions', () => {
  let api: FakeMapsClient;
  let fixture: ComponentFixture<SceneActions>;
  let el: HTMLElement;
  let emitted: (readonly SceneAction[])[];

  function setup(actions: SceneAction[]) {
    api = new FakeMapsClient();
    api.sceneActions = actions;
    emitted = [];
    TestBed.configureTestingModule({
      providers: [
        { provide: MapsClient, useValue: api },
        {
          provide: SceneChecks,
          useValue: {
            skills: () =>
              Promise.resolve([
                { key: 'skill:arcana', label: 'Arcanismo' },
                { key: 'skill:investigation', label: 'Investigação' },
              ]),
          },
        },
      ],
    });
    fixture = TestBed.createComponent(SceneActions);
    fixture.componentRef.setInput('campaignId', 'c1');
    fixture.componentRef.setInput('mapId', 'm1');
    fixture.componentRef.setInput('pointId', 'p1');
    fixture.componentRef.setInput('actions', actions);
    fixture.componentInstance.actionsChange.subscribe((list) => {
      emitted.push(list);
      // The panel puts the answer on the point, and the point comes back as the input.
      fixture.componentRef.setInput('actions', list);
    });
    fixture.detectChanges();
    el = fixture.nativeElement;
    // jsdom has no layout: the form scrolls itself into view.
    Element.prototype.scrollIntoView = vi.fn();
  }

  async function settle(): Promise<void> {
    for (let i = 0; i < 4; i++) {
      await fixture.whenStable();
      await new Promise((r) => setTimeout(r));
      fixture.detectChanges();
    }
  }

  const rows = () => Array.from(el.querySelectorAll('.sa__row'));
  const control = (id: string, which: 'up' | 'down' | 'remove') =>
    el.querySelector<HTMLButtonElement>(`[data-action="${id}"][data-control="${which}"]`)!;
  const button = (name: string) =>
    Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) => b.textContent?.includes(name))!;
  const flat = (e: Element | null | undefined) => e?.textContent?.replace(/ /g, ' ').replace(/\s+/g, ' ').trim();
  function type(input: HTMLInputElement, value: string): void {
    input.value = value;
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
  }

  it('lists the actions in order: the name or the check, the check, and the DC only when there is one', () => {
    setup([...FIVE]);
    expect(flat(el.querySelector('.sa__count'))).toBe('5 de 20');
    expect(rows().map((r) => r.querySelector('.sa__name')?.textContent)).toEqual([
      'Procurar pistas na carroça', 'Seguir os rastros dos goblins', 'Acalmar os cavalos', 'Percepção', 'Resistir ao cheiro de fumaça',
    ]);
    expect(rows().map((r) => r.querySelector('.sa__check')?.textContent)).toEqual([
      'Investigação', 'Sobrevivência', 'Adestrar Animais', 'Perícia', 'Salvaguarda de Constituição',
    ]);
    expect(rows().map((r) => flat(r.querySelector('.sa__dc')) ?? null)).toEqual(['CD 12', 'CD 13', null, null, 'CD 10']);
  });

  it('names every control by its action, and quiets the first ↑ and the last ↓', () => {
    setup([...FIVE]);
    expect(control('a2', 'up').getAttribute('aria-label')).toBe('Subir Seguir os rastros dos goblins');
    expect(control('a2', 'down').getAttribute('aria-label')).toBe('Descer Seguir os rastros dos goblins');
    expect(control('a4', 'remove').getAttribute('aria-label')).toBe('Remover Percepção');
    expect(control('a1', 'up').getAttribute('aria-disabled')).toBe('true');
    expect(control('a5', 'down').getAttribute('aria-disabled')).toBe('true');
    expect(control('a2', 'up').getAttribute('aria-disabled')).toBe('false');
  });

  it('invites the first action when there is none', () => {
    setup([]);
    expect(el.textContent).toContain('Nenhuma ação ainda');
    expect(flat(el.querySelector('.sa__count'))).toBe('0 de 20');
    expect(el.querySelector('.sa__list')).toBeNull();
    expect(button('Adicionar ação')).toBeTruthy();
  });

  it('moves an action up, saves at once, and keeps focus on the same button of the moved row', async () => {
    setup([...FIVE]);
    control('a3', 'up').focus();
    control('a3', 'up').click();
    await settle();
    expect(api.calls).toContain('moveSceneAction p1 a3 up');
    expect(rows().map((r) => r.querySelector('.sa__name')?.textContent).slice(0, 3)).toEqual([
      'Procurar pistas na carroça', 'Acalmar os cavalos', 'Seguir os rastros dos goblins',
    ]);
    expect(document.activeElement).toBe(control('a3', 'up'));
    expect(flat(el.querySelector('[role="status"]'))).toBe('Acalmar os cavalos: posição 2 de 5.');
    control('a3', 'down').click();
    await settle();
    expect(document.activeElement).toBe(control('a3', 'down'));
  });

  it('does nothing for the first ↑', async () => {
    setup([...FIVE]);
    control('a1', 'up').click();
    await settle();
    expect(api.calls).toEqual([]);
  });

  it('removes with no question and focuses the next row; the last one sends focus to "Adicionar ação"', async () => {
    setup([...FIVE]);
    control('a2', 'remove').click();
    await settle();
    expect(el.querySelector('[role="alertdialog"], [role="dialog"]')).toBeNull();
    expect(api.calls).toEqual(['removeSceneAction p1 a2']);
    expect(rows()).toHaveLength(4);
    expect(document.activeElement).toBe(control('a3', 'remove'));
    control('a5', 'remove').click();
    await settle();
    expect(document.activeElement).toBe(button('Adicionar ação'));
  });

  it('focuses "Adicionar ação" after removing the only action', async () => {
    setup([FIVE[0]]);
    control('a1', 'remove').click();
    await settle();
    expect(el.textContent).toContain('Nenhuma ação ainda');
    expect(document.activeElement).toBe(button('Adicionar ação'));
  });

  it('opens the form in place with focus on the first choice, and "Cancelar" gives focus back', async () => {
    setup([...FIVE]);
    button('Adicionar ação').click();
    await settle();
    expect(el.querySelector('form')).not.toBeNull();
    // The button is gone while the form is open (the form has its own "Adicionar ação").
    expect(el.querySelectorAll('.sa__add')).toHaveLength(0);
    expect(document.activeElement).toBe(el.querySelector('input[type="radio"]'));
    expect(Array.from(el.querySelectorAll('.sf__kind'), (l) => flat(l))).toEqual(['Perícia', 'Teste de atributo', 'Salvaguarda']);
    // The buttons are under the fields, "Adicionar ação" outlined and "Cancelar" a text button.
    const buttons = Array.from(el.querySelectorAll<HTMLButtonElement>('.sf__actions button'));
    expect(buttons.map((b) => flat(b))).toEqual(['Adicionar ação', 'Cancelar']);
    expect(buttons[0].classList).toContain('mat-mdc-outlined-button');
    expect(buttons[1].classList).toContain('mat-mdc-button');
    const lastField = el.querySelectorAll('mat-form-field')[2];
    expect(lastField.compareDocumentPosition(buttons[0]) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    button('Cancelar').click();
    await settle();
    expect(el.querySelector('form')).toBeNull();
    expect(document.activeElement).toBe(button('Adicionar ação'));
    expect(api.calls).toEqual([]);
  });

  it('changes the list under "O que rolar": skills, the six abilities, the six saves', async () => {
    setup([...FIVE]);
    button('Adicionar ação').click();
    await settle();
    const select = () => el.querySelector<HTMLSelectElement>('select')!;
    expect(Array.from(select().options, (o) => o.textContent?.trim())).toEqual(['Arcanismo', 'Investigação']);
    el.querySelectorAll<HTMLInputElement>('input[type="radio"]')[1].click();
    fixture.detectChanges();
    expect(Array.from(select().options, (o) => o.textContent?.trim())).toEqual([
      'Força', 'Destreza', 'Constituição', 'Inteligência', 'Sabedoria', 'Carisma',
    ]);
    el.querySelectorAll<HTMLInputElement>('input[type="radio"]')[2].click();
    fixture.detectChanges();
    expect(Array.from(select().options, (o) => o.value)[4]).toBe('save:wis');
  });

  it('adds an action with a name and a DC, saves at once and focuses "Adicionar ação"', async () => {
    setup([...FIVE]);
    button('Adicionar ação').click();
    await settle();
    const [name, dc] = Array.from(el.querySelectorAll<HTMLInputElement>('input[matInput], input.mat-mdc-input-element'));
    type(name, 'Procurar mais pistas');
    expect(flat(el.querySelector('mat-form-field:nth-of-type(2) .mat-mdc-form-field-hint-wrapper'))).toContain('20 de 60');
    type(dc, '14');
    el.querySelector<HTMLFormElement>('form')!.dispatchEvent(new Event('submit', { cancelable: true }));
    await settle();
    expect(api.calls).toEqual(['addSceneAction p1 {"key":"skill:arcana","name":"Procurar mais pistas","dc":14}']);
    expect(emitted.at(-1)).toHaveLength(6);
    expect(el.querySelector('form')).toBeNull();
    expect(rows()).toHaveLength(6);
    expect(flat(el.querySelector('.sa__count'))).toBe('6 de 20');
    expect(document.activeElement).toBe(button('Adicionar ação'));
  });

  it('says a DC out of 1 to 30 under its field, with icon and words, and focuses the field', async () => {
    setup([...FIVE]);
    button('Adicionar ação').click();
    await settle();
    const dc = el.querySelectorAll<HTMLInputElement>('input.mat-mdc-input-element')[1];
    type(dc, '31');
    el.querySelector<HTMLFormElement>('form')!.dispatchEvent(new Event('submit', { cancelable: true }));
    await settle();
    const error = el.querySelector('mat-error');
    expect(flat(error)).toContain('A CD vai de 1 a 30. Digite outro número ou deixe em branco.');
    expect(error?.querySelector('mat-icon')?.textContent).toBe('error');
    expect(document.activeElement).toBe(dc);
    expect(api.calls).toEqual([]);
    // Typing again clears it.
    type(dc, '3');
    await settle();
    expect(el.querySelector('mat-error')).toBeNull();
  });

  it('leaves the DC empty as "no DC" (0)', async () => {
    setup([...FIVE]);
    button('Adicionar ação').click();
    await settle();
    el.querySelector<HTMLFormElement>('form')!.dispatchEvent(new Event('submit', { cancelable: true }));
    await settle();
    expect(api.calls[0]).toContain('"dc":0');
  });

  it('shows the free-text notice next to the name field', async () => {
    setup([...FIVE]);
    button('Adicionar ação').click();
    await settle();
    expect(el.querySelector('[role="note"]')?.textContent).toContain('É ficção: não escreva dados reais de pessoas.');
  });

  it('at 20 of 20 says the limit in words above a disabled button, tied to it', () => {
    setup(Array.from({ length: 20 }, (_, i) => action(`x${i}`, 'Arcanismo')));
    expect(flat(el.querySelector('.sa__count'))).toBe('20 de 20');
    expect(flat(el.querySelector('.sa__limit'))).toBe('Limite de 20 ações. Remova uma para adicionar outra.');
    const add = button('Adicionar ação');
    expect(add.getAttribute('aria-disabled')).toBe('true');
    expect(add.getAttribute('aria-describedby')).toBe('sa-limit');
    add.click();
    fixture.detectChanges();
    expect(el.querySelector('form')).toBeNull();
    // The sentence comes before the button.
    expect(el.querySelector('.sa__limit')!.compareDocumentPosition(add) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('takes the limit from the server too (`resource_exhausted`) and stays in the form', async () => {
    setup([...FIVE]);
    button('Adicionar ação').click();
    await settle();
    api.sceneActions = Array.from({ length: 20 }, (_, i) => action(`x${i}`, 'Arcanismo'));
    el.querySelector<HTMLFormElement>('form')!.dispatchEvent(new Event('submit', { cancelable: true }));
    await settle();
    expect(el.querySelector('form [role="alert"]')?.textContent).toContain('Limite de 20 ações');
    expect(el.querySelector('form')).not.toBeNull();
  });

  it('answers nothing to a second tap while one write is in flight', async () => {
    setup([...FIVE]);
    control('a3', 'up').click();
    control('a4', 'up').click();
    await settle();
    expect(api.calls).toEqual(['moveSceneAction p1 a3 up']);
  });
});
