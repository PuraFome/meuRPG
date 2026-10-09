import { ComponentFixture, TestBed } from '@angular/core/testing';
import { MatDialog } from '@angular/material/dialog';
import { provideRouter } from '@angular/router';
import { Code, ConnectError } from '@connectrpc/connect';
import { of } from 'rxjs';
import type { Mock } from 'vitest';

import {
  CharacterBlockedReason,
  CharacterBlockedSchema,
} from '../../../../../gen/meurpg/characters/v1/characters_pb';
import { ClaimsService } from '../../../../core/characters/claims.service';
import type { CampaignCharacterListItemVm, ClaimVm } from '../campaign-characters.types';
import { ReservedCharacters } from './reserved-characters';

function row(
  name: string,
  claim: ClaimVm,
  over: Partial<CampaignCharacterListItemVm> = {},
): CampaignCharacterListItemVm {
  return {
    id: `id-${name}`,
    name,
    kind: 'player',
    state: 'draft',
    classSummary: 'Monge 5',
    playerDisplayName: null,
    raceName: 'Humano',
    reserved: claim.state !== 'used',
    claim,
    ...over,
  };
}

const NONE: ClaimVm = { state: 'none', expiresAt: null, claimedBy: null };
const SENT: ClaimVm = { state: 'sent', expiresAt: new Date(2026, 9, 15), claimedBy: null };
const REVOKED: ClaimVm = { state: 'revoked', expiresAt: null, claimedBy: null };
const EXPIRED: ClaimVm = { state: 'expired', expiresAt: new Date(2026, 9, 1), claimedBy: null };
const USED: ClaimVm = { state: 'used', expiresAt: null, claimedBy: 'Lia' };

describe('ReservedCharacters: "Personagens reservados" (MR-049, PM-09 state 3)', () => {
  let claims: Record<
    'revokeLink' | 'returnToReserve' | 'deleteReserved' | 'createLink',
    ReturnType<typeof vi.fn>
  >;
  let changed: Mock<() => void>;
  let dialogOpen: Mock<(...args: unknown[]) => unknown>;

  async function render(rows: CampaignCharacterListItemVm[]) {
    claims = {
      revokeLink: vi.fn().mockResolvedValue({}),
      returnToReserve: vi.fn().mockResolvedValue({}),
      deleteReserved: vi.fn().mockResolvedValue({}),
      createLink: vi.fn(),
    };
    changed = vi.fn<() => void>();
    dialogOpen = vi.fn<(...args: unknown[]) => unknown>(() => ({ afterClosed: () => of(true) }));
    TestBed.configureTestingModule({
      imports: [ReservedCharacters],
      providers: [
        provideRouter([]),
        { provide: ClaimsService, useValue: claims },
        { provide: MatDialog, useValue: { open: dialogOpen } },
      ],
    });
    const fixture: ComponentFixture<ReservedCharacters> =
      TestBed.createComponent(ReservedCharacters);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput('rows', rows);
    fixture.componentInstance.changed.subscribe(() => changed());
    const el = fixture.nativeElement as HTMLElement;
    const settle = async () => {
      for (let i = 0; i < 4; i++) {
        fixture.detectChanges();
        await fixture.whenStable();
      }
    };
    await settle();
    const text = () => (el.textContent ?? '').replace(/\s+/g, ' ');
    const li = (name: string) => el.querySelector<HTMLElement>(`#row-id-${name}`)!;
    const btn = (scope: ParentNode, label: string) =>
      Array.from(scope.querySelectorAll<HTMLElement>('button, a')).find((b) =>
        b.textContent?.trim().startsWith(label),
      )!;
    return { fixture, el, settle, text, li, btn };
  }

  it('says what the reserve is, lists each character with the state of its link, and offers the creation', async () => {
    const { el, text, li } = await render([
      row('Brisa', USED),
      row('Sálvia', SENT),
      row('Kai', NONE),
      row('Ragna', REVOKED),
      row('Orin', EXPIRED),
    ]);

    expect(text()).toContain('Personagens reservados');
    expect(text()).toContain(
      'Reservados: ninguém é dono ainda. Os jogadores não os veem até um deles assumir.',
    );
    expect(li('Brisa').textContent).toContain('Assumido por Lia');
    expect(li('Sálvia').textContent).toContain('Link enviado · vale até 15/10');
    expect(li('Kai').textContent).toContain('Sem link');
    expect(li('Ragna').textContent).toContain('Link revogado');
    expect(li('Orin').textContent).toContain('Link expirado em 01/10');
    expect(li('Kai').textContent).toContain('Monge 5 · Humano');
    const create = el.querySelector<HTMLAnchorElement>('a.reserved__create')!;
    expect(create.textContent).toContain('Criar personagem para um jogador');
    expect(create.getAttribute('href')).toBe('/campaigns/camp-1/reserved/new');
  });

  it('offers each state its own actions', async () => {
    const { li, btn } = await render([
      row('Brisa', USED),
      row('Sálvia', SENT),
      row('Kai', NONE),
      row('Ragna', REVOKED),
    ]);
    const labels = (name: string) =>
      Array.from(li(name).querySelectorAll('.row__actions > *')).map((b) => b.textContent?.trim());

    expect(labels('Brisa')).toEqual(['Ver ficha', 'Devolver à reserva']);
    expect(labels('Sálvia')).toEqual(['Editar', 'Revogar o link', 'Excluir']);
    expect(labels('Kai')).toEqual(['Gerar link para o jogador', 'Editar', 'Excluir']);
    expect(labels('Ragna')).toEqual(['Gerar novo link', 'Editar', 'Excluir']);
    expect(btn(li('Kai'), 'Editar').getAttribute('href')).toBe(
      '/campaigns/camp-1/characters/id-Kai/edit',
    );
    expect(btn(li('Brisa'), 'Ver ficha').getAttribute('href')).toBe(
      '/campaigns/camp-1/characters/id-Brisa',
    );
  });

  it('"Gerar link" opens the dialog on that character and the list is read again once a link was made', async () => {
    const { li, btn, settle } = await render([row('Kai', NONE)]);

    btn(li('Kai'), 'Gerar link para o jogador').click();
    await settle();

    expect(dialogOpen).toHaveBeenCalledTimes(1);
    const config = dialogOpen.mock.calls[0][1] as {
      data: { characterId: string; characterName: string; description: string };
    };
    expect(config.data).toMatchObject({
      characterId: 'id-Kai',
      characterName: 'Kai',
      description: 'Monge 5, Humano',
    });
    expect(changed).toHaveBeenCalledTimes(1);
  });

  it('"Revogar o link" asks in place, on the same character, with the focus on "Cancelar"', async () => {
    const { el, li, btn, settle, text } = await render([row('Sálvia', SENT), row('Kai', SENT)]);

    btn(li('Sálvia'), 'Revogar o link').click();
    await settle();

    const ask = li('Sálvia').querySelector<HTMLElement>('[role="group"]')!;
    expect(ask.textContent).toContain('Revogar o link de Sálvia?');
    expect(ask.textContent).toContain(
      'Quem abrir o link verá apenas “Este link não pode ser usado”. Você pode gerar outro depois.',
    );
    expect(li('Kai').querySelector('[role="group"]')).toBeNull();
    expect(document.activeElement).toBe(ask.querySelector('[data-cancel]'));
    expect(claims.revokeLink).not.toHaveBeenCalled();

    btn(ask, 'Revogar o link').click();
    await settle();

    expect(claims.revokeLink).toHaveBeenCalledWith('camp-1', 'id-Sálvia');
    expect(changed).toHaveBeenCalled();
    expect(text()).toContain('Link de Sálvia revogado.');
    expect(el.querySelector('[role="group"]')).toBeNull();
  });

  it('"Cancelar" closes the question and puts the focus back on the button that asked it', async () => {
    const { li, btn, settle } = await render([row('Sálvia', SENT)]);
    const opener = btn(li('Sálvia'), 'Revogar o link');
    opener.click();
    await settle();

    btn(li('Sálvia'), 'Cancelar').click();
    await settle();

    expect(li('Sálvia').querySelector('[role="group"]')).toBeNull();
    expect(document.activeElement).toBe(li('Sálvia').querySelector('[data-opener="revoke"]'));
    expect(claims.revokeLink).not.toHaveBeenCalled();
  });

  it('"Excluir" is irreversible and says it deletes the link too', async () => {
    const { li, btn, settle } = await render([row('Sálvia', SENT)]);

    btn(li('Sálvia'), 'Excluir').click();
    await settle();
    const ask = li('Sálvia').querySelector<HTMLElement>('[role="group"]')!;
    expect(ask.textContent).toContain('Excluir Sálvia?');
    expect(ask.textContent).toContain(
      'O personagem é apagado e o link enviado deixa de valer. Isto não se desfaz.',
    );

    btn(ask, 'Excluir Sálvia').click();
    await settle();

    expect(claims.deleteReserved).toHaveBeenCalledWith('camp-1', 'id-Sálvia');
    expect(changed).toHaveBeenCalled();
  });

  it('"Devolver à reserva" names who stays in the campaign', async () => {
    const { li, btn, settle } = await render([row('Brisa', USED)]);

    btn(li('Brisa'), 'Devolver à reserva').click();
    await settle();
    const ask = li('Brisa').querySelector<HTMLElement>('[role="group"]')!;
    expect(ask.textContent).toContain('Devolver Brisa à reserva?');
    expect(ask.textContent).toContain('Brisa fica sem dono e o link deixa de existir.');
    expect(ask.textContent).toContain('Lia continua na campanha como membro.');

    btn(ask, 'Devolver à reserva').click();
    await settle();

    expect(claims.returnToReserve).toHaveBeenCalledWith('camp-1', 'id-Brisa');
    expect(changed).toHaveBeenCalled();
  });

  it('a revoke that loses to the player who used the link says who, and the list is read again', async () => {
    const { li, btn, settle, text } = await render([row('Sálvia', SENT)]);
    claims.revokeLink.mockRejectedValue(
      new ConnectError('used', Code.FailedPrecondition, undefined, [
        {
          desc: CharacterBlockedSchema,
          value: {
            reason: CharacterBlockedReason.CLAIM_LINK_USED,
            characterId: 'id-Sálvia',
            playerDisplayName: 'Lia',
          },
        },
      ]),
    );

    btn(li('Sálvia'), 'Revogar o link').click();
    await settle();
    btn(li('Sálvia').querySelector<HTMLElement>('[role="group"]')!, 'Revogar o link').click();
    await settle();

    expect(text()).toContain('Este link já foi usado por Lia.');
    expect(text()).toContain('Você não pôde revogá-lo: Lia acabou de assumir Sálvia.');
    expect(text()).toContain('use “Devolver à reserva” se foi engano.');
    expect(changed).toHaveBeenCalled();
  });

  it('any other failure is said in words, and the list is left as it is', async () => {
    const { li, btn, settle, text } = await render([row('Sálvia', SENT)]);
    claims.deleteReserved.mockRejectedValue(new ConnectError('no', Code.PermissionDenied));

    btn(li('Sálvia'), 'Excluir').click();
    await settle();
    btn(li('Sálvia').querySelector<HTMLElement>('[role="group"]')!, 'Excluir Sálvia').click();
    await settle();

    expect(text()).toContain('Você não tem permissão para fazer isso.');
    expect(changed).not.toHaveBeenCalled();
  });
});
