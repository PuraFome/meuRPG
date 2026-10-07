import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap } from '@angular/router';
import { of } from 'rxjs';

import { InviteError } from './invite-error';

function renderWithReason(reason: string | null): HTMLElement {
  TestBed.configureTestingModule({
    imports: [InviteError],
    providers: [
      {
        provide: ActivatedRoute,
        useValue: { queryParamMap: of(convertToParamMap(reason ? { reason } : {})) },
      },
    ],
  });
  const fixture = TestBed.createComponent(InviteError);
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('InviteError', () => {
  it('has exactly one h1', () => {
    const el = renderWithReason('expired');
    expect(el.querySelectorAll('h1').length).toBe(1);
  });

  it.each([
    ['expired', 'expirou'],
    ['revoked', 'revogado'],
    ['used_up', 'já foi usado'],
    ['not_found', 'não encontrado'],
    ['invalid', 'inválido'],
  ])('shows a message for reason=%s containing %j', (reason, fragment) => {
    expect(renderWithReason(reason).textContent).toContain(fragment);
  });

  it('shows a link back to the home page', () => {
    const el = renderWithReason('expired');
    expect(el.querySelector('a[href="/"]')).toBeTruthy();
  });

  it('falls back to a generic message when reason is missing or unrecognized', () => {
    expect(renderWithReason(null).textContent).toContain('Não foi possível aceitar o convite');
  });
});
