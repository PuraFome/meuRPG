import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';

import { ArchiveQuestion } from './archive-question';
import { ArchiveSheet } from './archive-sheet';

describe('the archive question (E10-01 states 8 and 10)', () => {
  const text = (e: Element) => (e.textContent ?? '').replace(/ /g, ' ').replace(/\s+/g, ' ');

  describe('in place (a laptop)', () => {
    function setup(name = 'Corujeiro', using = '2 fichas usam Corujeiro agora.') {
      TestBed.resetTestingModule();
      const fixture = TestBed.createComponent(ArchiveQuestion);
      fixture.componentRef.setInput('name', name);
      fixture.componentRef.setInput('using', using);
      const cancel = vi.fn();
      const confirm = vi.fn();
      fixture.componentInstance.cancel.subscribe(cancel);
      fixture.componentInstance.confirm.subscribe(confirm);
      fixture.detectChanges();
      return { fixture, el: fixture.nativeElement as HTMLElement, cancel, confirm };
    }

    it('asks with the warning and how many sheets use it, in words that suit a race and a background alike', () => {
      const { el } = setup();
      expect(text(el)).toContain('Arquivar Corujeiro?');
      expect(text(el)).toContain('As fichas que usam Corujeiro continuam funcionando. A entrada só deixa de aparecer para fichas novas.');
      expect(text(el)).toContain('2 fichas usam Corujeiro agora.');
      // No word that takes a gender: it reads the same for a spell, a race and a background.
      expect(text(el)).not.toMatch(/continuam com (ele|ela)/);
    });

    it('has "Voltar" and the one filled button, which names what it archives; Esc goes back', () => {
      const { el, cancel, confirm } = setup();
      const buttons = Array.from(el.querySelectorAll('button'));
      expect(buttons).toHaveLength(2);
      const back = buttons.find((b) => text(b).includes('Voltar'))!;
      const go = buttons.find((b) => text(b).includes('Arquivar Corujeiro'))!;
      go.click();
      expect(confirm).toHaveBeenCalled();
      back.click();
      expect(cancel).toHaveBeenCalled();
      el.querySelector('section')!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
      expect(cancel).toHaveBeenCalledTimes(2);
    });

    it('has the title as its first focus', () => {
      const { el } = setup();
      const title = el.querySelector('#ask-t');
      expect(title).not.toBeNull();
      expect(title!.getAttribute('tabindex')).toBe('-1');
    });
  });

  describe('as a bottom sheet (a phone)', () => {
    function setup(name = 'Guardião do Vale', using = 0) {
      TestBed.resetTestingModule();
      const close = vi.fn();
      TestBed.configureTestingModule({
        providers: [
          { provide: MAT_DIALOG_DATA, useValue: { name, using } },
          { provide: MatDialogRef, useValue: { close } },
        ],
      });
      const fixture = TestBed.createComponent(ArchiveSheet);
      fixture.detectChanges();
      return { el: fixture.nativeElement as HTMLElement, close };
    }

    it('asks in a frame whose title is the question, with the same neutral warning and how many sheets use it', () => {
      const { el } = setup('Guardião do Vale', 2);
      expect(text(el)).toContain('Arquivar Guardião do Vale?');
      expect(text(el)).toContain('As fichas que usam Guardião do Vale continuam funcionando. A entrada só deixa de aparecer para fichas novas.');
      expect(text(el)).toContain('2 fichas usam Guardião do Vale agora.');
      expect(text(setup('Bardo', 0).el)).toContain('Nenhuma ficha usa Bardo agora.');
    });

    it('answers true on "Arquivar" and false on "Voltar" and on the X', () => {
      const { el, close } = setup();
      const byText = (label: string) => Array.from(el.querySelectorAll('button')).find((b) => text(b).includes(label) || b.getAttribute('aria-label') === label)!;
      byText('Arquivar').click();
      expect(close).toHaveBeenLastCalledWith(true);
      byText('Voltar').click();
      expect(close).toHaveBeenLastCalledWith(false);
      byText('Fechar').click();
      expect(close).toHaveBeenLastCalledWith(false);
    });
  });
});
