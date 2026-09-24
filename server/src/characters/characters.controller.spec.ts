import { INestApplication, ValidationPipe } from '@nestjs/common';
import { Test } from '@nestjs/testing';
import {
  afterAll,
  beforeAll,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from 'vitest';
import request from 'supertest';
import { CharactersController } from './characters.controller';
import { CharactersRepository } from '../db/characters.repository';
import type {
  CharacterRecord,
  JoinToken,
} from '../db/characters.repository';
import { SessionGuard } from '../auth/session.guard';
import { AuthTokensRepository } from '../db/auth-tokens.repository';
import { UsersRepository } from '../db/users.repository';

const OWNER_ID = '11111111-1111-4111-8111-111111111111';
const OTHER_OWNER_ID = '22222222-2222-4222-8222-222222222222';

function makeCharacter(overrides: Partial<CharacterRecord> = {}): CharacterRecord {
  return {
    id: '11111111-2222-4333-8444-555555555555',
    userId: OWNER_ID,
    masterUserId: null,
    type: 'npc',
    name: 'Goblin',
    description: '',
    imageUrl: null,
    history: null,
    masterNotes: null,
    attributes: {},
    skills: [],
    inventory: [],
    quotes: [],
    sheet: null,
    minion: null,
    createdAt: new Date('2026-01-01T00:00:00.000Z'),
    updatedAt: new Date('2026-01-01T00:00:00.000Z'),
    ...overrides,
  };
}

function makeJoinToken(token = 'share-token'): JoinToken {
  return {
    token,
    type: 'player',
    createdBy: OTHER_OWNER_ID,
    expiresAt: new Date('2027-01-01T00:00:00.000Z'),
  };
}

const repoMock = {
  create: vi.fn(),
  findByIdForUser: vi.fn(),
  findAllForUser: vi.fn(),
  updateByIdForUser: vi.fn(),
  deleteByIdForUser: vi.fn(),
  createJoinToken: vi.fn(),
  findJoinToken: vi.fn(),
};

const usersMock = {
  setRole: vi.fn(),
};

describe('CharactersController', () => {
  let app: INestApplication;

  beforeAll(async () => {
    const moduleRef = await Test.createTestingModule({
      controllers: [CharactersController],
      providers: [
        { provide: CharactersRepository, useValue: repoMock },
        { provide: UsersRepository, useValue: usersMock },
        SessionGuard,
        {
          provide: AuthTokensRepository,
          useValue: {
            findUserByToken: vi.fn().mockResolvedValue({
              id: OWNER_ID,
              sub: 'google-sub-1',
              email: 'owner@example.com',
              name: 'Owner',
            }),
          },
        },
      ],
    })
      .overrideGuard(SessionGuard)
      .useValue({
        canActivate: (context: {
          switchToHttp: () => { getRequest: () => Record<string, unknown> };
        }) => {
          const req = context.switchToHttp().getRequest();
          req['authUser'] = {
            id: OWNER_ID,
            sub: 'google-sub-1',
            email: 'owner@example.com',
            name: 'Owner',
            role: 'master',
          };
          return true;
        },
      })
      .compile();

    app = moduleRef.createNestApplication();
    // Mirror the global pipe from main.ts so whitelist/forbidNonWhitelisted
    // behavior is exercised exactly as in production.
    app.useGlobalPipes(
      new ValidationPipe({
        whitelist: true,
        forbidNonWhitelisted: true,
        transform: true,
      }),
    );
    await app.init();
  });

  afterAll(async () => {
    await app.close();
  });

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('POST /characters -> 201 with the created id and the caller as owner', async () => {
    const created = makeCharacter({ type: 'player', name: 'Aragorn' });
    repoMock.create.mockResolvedValue(created);

    const res = await request(app.getHttpServer())
      .post('/characters')
      .send({ name: 'Aragorn', type: 'player' });

    expect(res.status).toBe(201);
    expect(res.body.id).toBe(created.id);
    expect(repoMock.create).toHaveBeenCalledWith(
      expect.objectContaining({ name: 'Aragorn', type: 'player' }),
      OWNER_ID,
    );
  });

  it('POST /characters with invalid type -> 400', async () => {
    const res = await request(app.getHttpServer())
      .post('/characters')
      .send({ name: 'Bad', type: 'invalid' });

    expect(res.status).toBe(400);
    expect(repoMock.create).not.toHaveBeenCalled();
  });

  it('POST /characters with a non-whitelisted field -> 400', async () => {
    const res = await request(app.getHttpServer())
      .post('/characters')
      .send({ name: 'Sneaky', type: 'npc', hacker: true });

    expect(res.status).toBe(400);
    expect(repoMock.create).not.toHaveBeenCalled();
  });

  it('GET /characters?type=npc -> 200, scoped to the caller', async () => {
    repoMock.findAllForUser.mockResolvedValue([makeCharacter()]);

    const res = await request(app.getHttpServer()).get('/characters?type=npc');

    expect(res.status).toBe(200);
    expect(Array.isArray(res.body)).toBe(true);
    expect(repoMock.findAllForUser).toHaveBeenCalledWith(OWNER_ID, 'npc');
  });

  it('POST /characters/join-tokens -> 201 and records the caller as creator', async () => {
    repoMock.createJoinToken.mockResolvedValue(makeJoinToken('minted'));

    const res = await request(app.getHttpServer()).post(
      '/characters/join-tokens',
    );

    expect(res.status).toBe(201);
    expect(res.body.token).toBe('minted');
    expect(res.body.type).toBe('player');
    expect(res.body.expiresAt).toBeDefined();
    expect(repoMock.createJoinToken).toHaveBeenCalledWith(OWNER_ID);
  });

  it('GET /characters/join/:token valid -> 200 { valid: true, expiresAt }', async () => {
    repoMock.findJoinToken.mockResolvedValue(makeJoinToken());

    const res = await request(app.getHttpServer()).get(
      '/characters/join/share-token',
    );

    expect(res.status).toBe(200);
    expect(res.body.valid).toBe(true);
    expect(res.body.expiresAt).toBeDefined();
  });

  it('GET /characters/join/:token unknown/expired -> 410', async () => {
    repoMock.findJoinToken.mockResolvedValue(null);

    const res = await request(app.getHttpServer()).get(
      '/characters/join/expired-token',
    );

    expect(res.status).toBe(410);
  });

  it("POST /characters/join/:token forces type='player', owner=session user and master=inviter", async () => {
    repoMock.findJoinToken.mockResolvedValue(makeJoinToken());
    repoMock.create.mockResolvedValue(
      makeCharacter({ type: 'player', name: 'Joiner', userId: OWNER_ID }),
    );

    const res = await request(app.getHttpServer())
      .post('/characters/join/share-token')
      .send({ name: 'Joiner', type: 'npc' });

    expect(res.status).toBe(201);
    expect(res.body.type).toBe('player');
    expect(repoMock.create).toHaveBeenCalledWith(
      expect.objectContaining({ name: 'Joiner', type: 'npc' }),
      OWNER_ID,
      'player',
      OTHER_OWNER_ID,
    );
    expect(usersMock.setRole).toHaveBeenCalledWith(OWNER_ID, 'visitor');
  });

  it('POST /characters/join/:token unknown -> 410 and never creates', async () => {
    repoMock.findJoinToken.mockResolvedValue(null);

    const res = await request(app.getHttpServer())
      .post('/characters/join/nope')
      .send({ name: 'Joiner', type: 'player' });

    expect(res.status).toBe(410);
    expect(repoMock.create).not.toHaveBeenCalled();
  });

  it('GET /characters/:id -> 200 when owned', async () => {
    repoMock.findByIdForUser.mockResolvedValue(makeCharacter());

    const res = await request(app.getHttpServer()).get('/characters/some-id');

    expect(res.status).toBe(200);
    expect(res.body.id).toBeDefined();
    expect(repoMock.findByIdForUser).toHaveBeenCalledWith('some-id', OWNER_ID);
  });

  it('GET /characters/:id not owned/unknown -> 404', async () => {
    repoMock.findByIdForUser.mockResolvedValue(null);

    const res = await request(app.getHttpServer()).get('/characters/nope');

    expect(res.status).toBe(404);
  });

  it('PATCH /characters/:id -> 200 with the updated record', async () => {
    repoMock.updateByIdForUser.mockResolvedValue(
      makeCharacter({ name: 'Renamed' }),
    );

    const res = await request(app.getHttpServer())
      .patch('/characters/some-id')
      .send({ name: 'Renamed' });

    expect(res.status).toBe(200);
    expect(res.body.name).toBe('Renamed');
    expect(repoMock.updateByIdForUser).toHaveBeenCalledWith(
      'some-id',
      OWNER_ID,
      { name: 'Renamed' },
    );
  });

  it('PATCH /characters/:id not owned/unknown -> 404', async () => {
    repoMock.updateByIdForUser.mockResolvedValue(null);

    const res = await request(app.getHttpServer())
      .patch('/characters/nope')
      .send({ name: 'Renamed' });

    expect(res.status).toBe(404);
  });

  it('DELETE /characters/:id -> 204', async () => {
    repoMock.deleteByIdForUser.mockResolvedValue(true);

    const res = await request(app.getHttpServer()).delete(
      '/characters/some-id',
    );

    expect(res.status).toBe(204);
    expect(repoMock.deleteByIdForUser).toHaveBeenCalledWith(
      'some-id',
      OWNER_ID,
    );
  });

  it('DELETE /characters/:id not owned/unknown -> 404', async () => {
    repoMock.deleteByIdForUser.mockResolvedValue(false);

    const res = await request(app.getHttpServer()).delete('/characters/nope');

    expect(res.status).toBe(404);
  });

  it('uses the resolved owner for a different account (no cross-account leakage)', async () => {
    repoMock.findAllForUser.mockResolvedValue([]);
    const moduleRef = await Test.createTestingModule({
      controllers: [CharactersController],
      providers: [
        { provide: CharactersRepository, useValue: repoMock },
        { provide: UsersRepository, useValue: usersMock },
        SessionGuard,
        {
          provide: AuthTokensRepository,
          useValue: {
            findUserByToken: vi.fn().mockResolvedValue({
              id: OTHER_OWNER_ID,
              sub: 'google-sub-2',
              email: 'other@example.com',
              name: 'Other',
            }),
          },
        },
      ],
    })
      .overrideGuard(SessionGuard)
      .useValue({
        canActivate: (context: {
          switchToHttp: () => { getRequest: () => Record<string, unknown> };
        }) => {
          const req = context.switchToHttp().getRequest();
          req['authUser'] = {
            id: OTHER_OWNER_ID,
            sub: 'google-sub-2',
            email: 'other@example.com',
            name: 'Other',
            role: 'master',
          };
          return true;
        },
      })
      .compile();
    const otherApp = moduleRef.createNestApplication();
    await otherApp.init();

    await request(otherApp.getHttpServer()).get('/characters');

    expect(repoMock.findAllForUser).toHaveBeenCalledWith(
      OTHER_OWNER_ID,
      undefined,
    );
    await otherApp.close();
  });
});

describe('CharactersController (unauthenticated)', () => {
  let app: INestApplication;

  beforeAll(async () => {
    const moduleRef = await Test.createTestingModule({
      controllers: [CharactersController],
      providers: [
        { provide: CharactersRepository, useValue: repoMock },
        { provide: UsersRepository, useValue: usersMock },
        SessionGuard,
        {
          provide: AuthTokensRepository,
          useValue: { findUserByToken: vi.fn().mockResolvedValue(null) },
        },
      ],
    }).compile();

    app = moduleRef.createNestApplication();
    await app.init();
  });

  afterAll(async () => {
    await app.close();
  });

  it('GET /characters without a bearer token -> 401', async () => {
    const res = await request(app.getHttpServer()).get('/characters');
    expect(res.status).toBe(401);
  });

  it('POST /characters with an unknown token -> 401 and never creates', async () => {
    const res = await request(app.getHttpServer())
      .post('/characters')
      .set('Authorization', 'Bearer not-a-real-token')
      .send({ name: 'Ghost', type: 'npc' });

    expect(res.status).toBe(401);
    expect(repoMock.create).not.toHaveBeenCalled();
  });

  it('the public join endpoint stays reachable without a token', async () => {
    repoMock.findJoinToken.mockResolvedValue(makeJoinToken());
    const res = await request(app.getHttpServer()).get(
      '/characters/join/share-token',
    );
    expect(res.status).toBe(200);
  });
});
