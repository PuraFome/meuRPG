import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { AuthState, AuthService } from './core/auth/auth.service';
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
});
