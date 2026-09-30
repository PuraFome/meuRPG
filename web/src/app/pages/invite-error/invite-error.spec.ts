import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap } from '@angular/router';
import { of } from 'rxjs';

import { InviteError } from './invite-error';

function renderWithMotivo(motivo: string | null): HTMLElement {
  TestBed.configureTestingModule({
    imports: [InviteError],
    providers: [
      {
        provide: ActivatedRoute,
        useValue: { queryParamMap: of(convertToParamMap(motivo ? { motivo } : {})) },
      },
    ],
  });
  const fixture = TestBed.createComponent(InviteError);
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('InviteError', () => {
  it('has exactly one h1', () => {
    const el = renderWithMotivo('expired');
    expect(el.querySelectorAll('h1').length).toBe(1);
  });

  it.each([
    ['expired', 'expirou'],
    ['revoked', 'revogado'],
    ['used_up', 'já foi usado'],
    ['not_found', 'não encontrado'],
    ['invalid', 'inválido'],
  ])('shows a message for motivo=%s containing %j', (motivo, fragment) => {
    expect(renderWithMotivo(motivo).textContent).toContain(fragment);
  });

  it('shows a link back to the home page', () => {
    const el = renderWithMotivo('expired');
    expect(el.querySelector('a[href="/"]')).toBeTruthy();
  });

  it('falls back to a generic message when motivo is missing or unrecognized', () => {
    expect(renderWithMotivo(null).textContent).toContain('Não foi possível aceitar o convite');
  });
});
