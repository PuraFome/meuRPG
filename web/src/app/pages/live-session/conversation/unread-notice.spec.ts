import { TestBed } from '@angular/core/testing';

import { UnreadNotice, unreadWords } from './unread-notice';

describe('unreadWords', () => {
  it('says one line in the singular and the rest in the plural', () => {
    expect(unreadWords(1)).toBe('1 mensagem nova');
    expect(unreadWords(2)).toBe('2 mensagens novas');
    expect(unreadWords(120)).toBe('120 mensagens novas');
  });
});

describe('UnreadNotice', () => {
  function setup(unread = 3) {
    const fixture = TestBed.createComponent(UnreadNotice);
    fixture.componentRef.setInput('unread', unread);
    const opened: true[] = [];
    fixture.componentInstance.open.subscribe(() => opened.push(true));
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    return { fixture, el, opened, button: el.querySelector('button') as HTMLButtonElement };
  }

  it('is one neutral notice with the message icon and one button', () => {
    const { el } = setup();
    expect(el.querySelector('.mr-notice.mr-notice--neutral')).not.toBeNull();
    expect(el.querySelector('mat-icon')?.getAttribute('aria-hidden')).toBe('true');
    expect(el.querySelectorAll('button')).toHaveLength(1);
  });

  it('names the news in words, in the plural', () => {
    const { button } = setup(3);
    expect(button.textContent).toContain('Conversa: 3 mensagens novas');
    expect(button.textContent).toContain('Abrir');
  });

  it('uses the singular for one line', () => {
    expect(setup(1).button.textContent).toContain('Conversa: 1 mensagem nova');
  });

  it('has the visible words as the name of the button, with no aria-label that could differ', () => {
    const { button } = setup(2);
    expect(button.hasAttribute('aria-label')).toBe(false);
    expect(button.textContent?.replace(/\s+/g, ' ').trim()).toBe('Conversa: 2 mensagens novas Abrir');
  });

  it('asks to open the chat when the row is pressed', () => {
    const { button, opened } = setup();
    button.click();
    expect(opened).toHaveLength(1);
  });

  it('is not a live region: the panel tells each line once', () => {
    const { el } = setup();
    expect(el.querySelector('[role], [aria-live]')).toBeNull();
  });
});
