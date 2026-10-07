import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { Code, ConnectError } from '@connectrpc/connect';
import type { Transport } from '@connectrpc/connect';

import {
  GetMeResponseSchema,
  UpdateProfileResponseSchema,
} from '../../../gen/meurpg/identity/v1/identity_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';
import { AuthService } from './auth.service';

// These fakes stand in for the real fetch-backed Transport: AuthService
// only ever calls unary RPCs, so `stream` is never exercised. `unary`'s
// return type is generic per the real method's I/O descriptors, which a
// fixed fake response can't match exactly — hence the `as never` casts
// below, confined to this file.

/** A Transport whose every unary call resolves to `message`. */
function transportThatResolves(message: unknown): Transport {
  return {
    unary: (method) =>
      Promise.resolve({
        stream: false,
        service: method.parent,
        method,
        header: new Headers(),
        trailer: new Headers(),
        message,
      }) as never,
    stream(): never {
      throw new Error('IdentityService has no streaming RPCs');
    },
  };
}

/** A Transport whose every unary call rejects with `err`. */
function transportThatRejects(err: unknown): Transport {
  return {
    unary: () => Promise.reject(err) as never,
    stream(): never {
      throw new Error('IdentityService has no streaming RPCs');
    },
  };
}

/** A Transport whose unary call never settles, so `state()` can be read
 * before GetMe has had a chance to resolve. */
function transportThatNeverResolves(): Transport {
  return {
    unary: () => new Promise(() => undefined) as never,
    stream(): never {
      throw new Error('IdentityService has no streaming RPCs');
    },
  };
}

function createService(transport: Transport): AuthService {
  TestBed.configureTestingModule({
    providers: [{ provide: CONNECT_TRANSPORT, useValue: transport }],
  });
  return TestBed.inject(AuthService);
}

/** jsdom's `window.location.assign` is a non-configurable own property, so
 * `vi.spyOn` can't replace it directly. Swapping the whole `location`
 * object (restored afterwards, see `restoreLocation`) is the standard
 * workaround. */
const realLocation = window.location;

function stubLocationAssign(): ReturnType<typeof vi.fn> {
  const assign = vi.fn();
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: { ...realLocation, assign },
  });
  return assign;
}

function restoreLocation(): void {
  Object.defineProperty(window, 'location', {
    configurable: true,
    writable: true,
    value: realLocation,
  });
}

describe('AuthService', () => {
  it('starts as unknown, before the first GetMe resolves', () => {
    const service = createService(transportThatNeverResolves());
    expect(service.state()).toEqual({ status: 'unknown' });
    expect(service.isSignedIn()).toBe(false);
  });

  it('becomes signed-in when GetMe resolves, with displayName null (no name/e-mail on User)', async () => {
    const expiresAt = new Date('2026-10-29T00:00:00.000Z');
    const message = create(GetMeResponseSchema, {
      user: { id: 'user-1' },
      sessionExpiresAt: timestampFromDate(expiresAt),
    });
    const service = createService(transportThatResolves(message));
    await service.refresh();

    expect(service.state()).toEqual({
      status: 'signed-in',
      user: { id: 'user-1', displayName: null },
      sessionExpiresAt: expiresAt,
    });
    expect(service.isSignedIn()).toBe(true);
  });

  it('maps a non-empty display name from GetMe (never treats it as absent)', async () => {
    const message = create(GetMeResponseSchema, {
      user: { id: 'user-1', displayName: 'Vinicius' },
    });
    const service = createService(transportThatResolves(message));
    await service.refresh();

    expect(service.state()).toMatchObject({
      status: 'signed-in',
      user: { id: 'user-1', displayName: 'Vinicius' },
    });
  });

  it('becomes signed-out on an unauthenticated error', async () => {
    const service = createService(
      transportThatRejects(new ConnectError('sign in to continue', Code.Unauthenticated)),
    );
    await service.refresh();

    expect(service.state()).toEqual({ status: 'signed-out' });
    expect(service.isSignedIn()).toBe(false);
  });

  it('becomes unavailable on an unavailable error, never signed-out', async () => {
    const service = createService(
      transportThatRejects(
        new ConnectError('cannot check the session right now', Code.Unavailable),
      ),
    );
    await service.refresh();

    expect(service.state()).toEqual({ status: 'unavailable' });
  });

  it('becomes unavailable on any other Connect error code (not a sign-out look-alike)', async () => {
    const service = createService(transportThatRejects(new ConnectError('nope', Code.Internal)));
    await service.refresh();

    expect(service.state()).toEqual({ status: 'unavailable' });
  });

  it('becomes unavailable on a plain network failure (fetch rejecting before it reaches the server)', async () => {
    const service = createService(transportThatRejects(new TypeError('Failed to fetch')));
    await service.refresh();

    expect(service.state()).toEqual({ status: 'unavailable' });
  });

  describe('signIn', () => {
    let assign: ReturnType<typeof vi.fn>;

    beforeEach(() => {
      assign = stubLocationAssign();
    });

    afterEach(() => {
      restoreLocation();
    });

    it('navigates to /auth/login with the given same-site path as return_to', () => {
      const service = createService(transportThatNeverResolves());
      service.signIn('/campaigns');
      expect(assign).toHaveBeenCalledWith('/auth/login?return_to=%2Fcampaigns');
    });

    it('drops the fragment, so a token in it never reaches the URL', () => {
      const service = createService(transportThatNeverResolves());
      service.signIn('/invite#t=segredo-do-convite');
      expect(assign).toHaveBeenCalledWith('/auth/login?return_to=%2Finvite');
      expect(assign.mock.calls[0][0]).not.toContain('segredo');
    });

    it('falls back to "/" for anything that is not a safe same-site path', () => {
      const service = createService(transportThatNeverResolves());
      service.signIn('https://evil.example/campaigns');
      expect(assign).toHaveBeenCalledWith('/auth/login?return_to=%2F');

      service.signIn('//evil.example');
      expect(assign).toHaveBeenCalledWith('/auth/login?return_to=%2F');
    });
  });

  describe('signOut', () => {
    let assign: ReturnType<typeof vi.fn>;

    beforeEach(() => {
      assign = stubLocationAssign();
    });

    afterEach(() => {
      restoreLocation();
    });

    it('calls SignOut and always returns to the home page', async () => {
      const service = createService(transportThatResolves({}));
      await service.signOut();
      expect(assign).toHaveBeenCalledWith('/');
    });

    it('still returns to the home page when SignOut fails (e.g. the server is unavailable)', async () => {
      const service = createService(
        transportThatRejects(new ConnectError('cannot sign out right now', Code.Unavailable)),
      );
      await service.signOut();
      expect(assign).toHaveBeenCalledWith('/');
    });
  });

  describe('updateProfile', () => {
    /** A Transport that resolves each RPC by the generated method's name, so
     * a single service instance can go through `refresh()` (GetMe) and then
     * `updateProfile()` (UpdateProfile) with its own fake responses. */
    function transportDispatching(byMethodName: Record<string, unknown>): Transport {
      return {
        unary: (method) => {
          const message = byMethodName[method.name];
          if (message === undefined) {
            return Promise.reject(new Error(`unexpected call to ${method.name}`)) as never;
          }
          return Promise.resolve({
            stream: false,
            service: method.parent,
            method,
            header: new Headers(),
            trailer: new Headers(),
            message,
          }) as never;
        },
        stream(): never {
          throw new Error('IdentityService has no streaming RPCs');
        },
      };
    }

    it('updates state.user.displayName from the response on success', async () => {
      const service = createService(
        transportDispatching({
          GetMe: create(GetMeResponseSchema, { user: { id: 'user-1' } }),
          UpdateProfile: create(UpdateProfileResponseSchema, {
            user: { id: 'user-1', displayName: 'Novo Nome' },
          }),
        }),
      );
      await service.refresh();
      await service.updateProfile('Novo Nome');

      expect(service.state()).toMatchObject({
        status: 'signed-in',
        user: { id: 'user-1', displayName: 'Novo Nome' },
      });
    });

    it('clears the display name when the response carries an empty one', async () => {
      const service = createService(
        transportDispatching({
          GetMe: create(GetMeResponseSchema, { user: { id: 'user-1', displayName: 'Antigo' } }),
          UpdateProfile: create(UpdateProfileResponseSchema, {
            user: { id: 'user-1', displayName: '' },
          }),
        }),
      );
      await service.refresh();
      await service.updateProfile('');

      expect(service.state()).toMatchObject({
        status: 'signed-in',
        user: { id: 'user-1', displayName: null },
      });
    });

    it('leaves state untouched and rethrows when UpdateProfile fails (e.g. invalid_argument)', async () => {
      const service = createService({
        unary: (method) => {
          if (method.name === 'GetMe') {
            return Promise.resolve({
              stream: false,
              service: method.parent,
              method,
              header: new Headers(),
              trailer: new Headers(),
              message: create(GetMeResponseSchema, { user: { id: 'user-1' } }),
            }) as never;
          }
          return Promise.reject(
            new ConnectError('name has control characters', Code.InvalidArgument),
          ) as never;
        },
        stream(): never {
          throw new Error('IdentityService has no streaming RPCs');
        },
      });
      await service.refresh();

      await expect(service.updateProfile('bad\nname')).rejects.toBeInstanceOf(ConnectError);
      expect(service.state()).toMatchObject({
        status: 'signed-in',
        user: { id: 'user-1', displayName: null },
      });
    });
  });
});
