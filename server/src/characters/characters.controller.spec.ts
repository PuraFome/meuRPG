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

function makeCharacter(overrides: Partial<CharacterRecord> = {}): CharacterRecord {
  return {
    id: '11111111-2222-4333-8444-555555555555',
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
    expiresAt: new Date('2027-01-01T00:00:00.000Z'),
  };
}

const repoMock = {
  create: vi.fn(),
  findById: vi.fn(),
  findAll: vi.fn(),
  updateById: vi.fn(),
  deleteById: vi.fn(),
  createJoinToken: vi.fn(),
  findJoinToken: vi.fn(),
};

describe('CharactersController', () => {
  let app: INestApplication;

  beforeAll(async () => {
    const moduleRef = await Test.createTestingModule({
      controllers: [CharactersController],
      providers: [{ provide: CharactersRepository, useValue: repoMock }],
    }).compile();

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

  it('POST /characters -> 201 with the created id', async () => {
    const created = makeCharacter({ type: 'player', name: 'Aragorn' });
    repoMock.create.mockResolvedValue(created);

    const res = await request(app.getHttpServer())
      .post('/characters')
      .send({ name: 'Aragorn', type: 'player' });

    expect(res.status).toBe(201);
    expect(res.body.id).toBe(created.id);
    expect(repoMock.create).toHaveBeenCalledWith(
      expect.objectContaining({ name: 'Aragorn', type: 'player' }),
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

  it('GET /characters?type=npc -> 200 and forwards the filter', async () => {
    repoMock.findAll.mockResolvedValue([makeCharacter()]);

    const res = await request(app.getHttpServer()).get('/characters?type=npc');

    expect(res.status).toBe(200);
    expect(Array.isArray(res.body)).toBe(true);
    expect(repoMock.findAll).toHaveBeenCalledWith('npc');
  });

  it('POST /characters/join-tokens -> 201 { token, expiresAt, type }', async () => {
    repoMock.createJoinToken.mockResolvedValue(makeJoinToken('minted'));

    const res = await request(app.getHttpServer()).post(
      '/characters/join-tokens',
    );

    expect(res.status).toBe(201);
    expect(res.body.token).toBe('minted');
    expect(res.body.type).toBe('player');
    expect(res.body.expiresAt).toBeDefined();
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

  it("POST /characters/join/:token forces type='player' even when the body says 'npc'", async () => {
    repoMock.findJoinToken.mockResolvedValue(makeJoinToken());
    repoMock.create.mockResolvedValue(
      makeCharacter({ type: 'player', name: 'Joiner' }),
    );

    const res = await request(app.getHttpServer())
      .post('/characters/join/share-token')
      .send({ name: 'Joiner', type: 'npc' });

    expect(res.status).toBe(201);
    expect(res.body.type).toBe('player');
    expect(repoMock.create).toHaveBeenCalledWith(
      expect.objectContaining({ name: 'Joiner', type: 'npc' }),
      'player',
    );
  });

  it('POST /characters/join/:token unknown -> 410 and never creates', async () => {
    repoMock.findJoinToken.mockResolvedValue(null);

    const res = await request(app.getHttpServer())
      .post('/characters/join/nope')
      .send({ name: 'Joiner', type: 'player' });

    expect(res.status).toBe(410);
    expect(repoMock.create).not.toHaveBeenCalled();
  });

  it('GET /characters/:id -> 200 when found', async () => {
    repoMock.findById.mockResolvedValue(makeCharacter());

    const res = await request(app.getHttpServer()).get('/characters/some-id');

    expect(res.status).toBe(200);
    expect(res.body.id).toBeDefined();
  });

  it('GET /characters/:id unknown -> 404', async () => {
    repoMock.findById.mockResolvedValue(null);

    const res = await request(app.getHttpServer()).get('/characters/nope');

    expect(res.status).toBe(404);
  });

  it('PATCH /characters/:id -> 200 with the updated record', async () => {
    repoMock.updateById.mockResolvedValue(makeCharacter({ name: 'Renamed' }));

    const res = await request(app.getHttpServer())
      .patch('/characters/some-id')
      .send({ name: 'Renamed' });

    expect(res.status).toBe(200);
    expect(res.body.name).toBe('Renamed');
  });

  it('PATCH /characters/:id unknown -> 404', async () => {
    repoMock.updateById.mockResolvedValue(null);

    const res = await request(app.getHttpServer())
      .patch('/characters/nope')
      .send({ name: 'Renamed' });

    expect(res.status).toBe(404);
  });

  it('DELETE /characters/:id -> 204', async () => {
    repoMock.deleteById.mockResolvedValue(true);

    const res = await request(app.getHttpServer()).delete(
      '/characters/some-id',
    );

    expect(res.status).toBe(204);
  });

  it('DELETE /characters/:id unknown -> 404', async () => {
    repoMock.deleteById.mockResolvedValue(false);

    const res = await request(app.getHttpServer()).delete('/characters/nope');

    expect(res.status).toBe(404);
  });
});
