import { Component, TemplateRef, signal, viewChild } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { AuthState, AuthService } from './core/auth/auth.service';
import { OpenSessionVm, OpenSessions } from './shell/live-notice/open-sessions';
import { SessionNotes } from './shell/session-notes/session-notes';
import { App } from './app';

describe('App', () => {
  // App always renders <app-user-menu>, which injects AuthService. Stub it
  // so this test never makes a real GetMe call.
  const authStub: Pick<AuthService, 'state'> = {
    state: signal<AuthState>({ status: 'unknown' }).asReadonly(),
  };

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [App],
      providers: [provideRouter([]), { provide: AuthService, useValue: authStub }],
    }).compileComponents();
  });

  it('should create the app', () => {
    const fixture = TestBed.createComponent(App);
    const app = fixture.componentInstance;
    expect(app).toBeTruthy();
  });

  it('renders the app name and the main nav links', () => {
    const fixture = TestBed.createComponent(App);
    fixture.detectChanges();
    const compiled = fixture.nativeElement as HTMLElement;
    const text = compiled.textContent ?? '';
    expect(text).toContain('MeuRPG');
    expect(text).toContain('Início');
    expect(text).toContain('Minhas campanhas');
  });

  it('shows the "Ao vivo" link to the open session, named for screen readers (RN-06)', async () => {
    const sessions = signal<readonly OpenSessionVm[]>([
      {
        sessionId: 's4',
        campaignId: 'mirathel',
        campaignName: 'Mirathel',
        sessionNumber: 4,
        startedAt: new Date(),
        isMaster: false,
      },
    ]);
    TestBed.overrideProvider(OpenSessions, {
      useValue: {
        sessions: sessions.asReadonly(),
        dismissed: signal(new Set<string>()).asReadonly(),
        dismiss: () => undefined,
      },
    });
    const fixture = TestBed.createComponent(App);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const links = el.querySelectorAll('a[aria-label="Ao vivo: sessão 4 de Mirathel"]');
    // One in the bar (phone), one before the account (wider screens); CSS
    // shows one at a time.
    expect(links.length).toBe(2);
    expect(links[0].getAttribute('href')).toBe('/campaigns/mirathel/session');
    expect(links[0].textContent).toContain('Ao vivo');
    expect(el.textContent).toContain('A sessão 4 de Mirathel começou.');
  });

  it('draws what a player\'s session page hands the bar (the "Anotações" button), and nothing when it takes it back', () => {
    @Component({ template: '<ng-template #bar><button class="notes-bar">Anotações</button></ng-template>' })
    class Host {
      readonly bar = viewChild.required<TemplateRef<unknown>>('bar');
    }
    const host = TestBed.createComponent(Host);
    host.detectChanges();
    const fixture = TestBed.createComponent(App);
    const notes = TestBed.inject(SessionNotes);
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('.notes-bar')).toBeNull();
    notes.bar.set(host.componentInstance.bar());
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('.notes-bar')?.textContent).toBe('Anotações');
    notes.bar.set(null);
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('.notes-bar')).toBeNull();
  });
});
