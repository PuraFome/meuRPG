import { Code, ConnectError } from '@connectrpc/connect';

import {
  ImageGenerationBlockedReason,
  ImageGenerationFailure,
} from '../../../gen/meurpg/maps/v1/imagegen_pb';
import { blockedOf, failureText, generateIssue, useIssue } from './imagegen-errors';
import { blocked, imageStatus, invalid } from './imagegen-testing';

const plain = (t: string) => t.replace(/\u00a0/g, ' ');

describe('every refusal, by its typed detail (never by the message)', () => {
  const cases: [ImageGenerationBlockedReason, string][] = [
    [ImageGenerationBlockedReason.OFF, 'A geração de imagens não está ligada neste servidor.'],
    [
      ImageGenerationBlockedReason.GALLERY_FULL,
      'A galeria está cheia. Apague as imagens que você não usa e tente de novo.',
    ],
    [
      ImageGenerationBlockedReason.DAILY_LIMIT_REACHED,
      'O servidor já fez todas as imagens de hoje. Tente de novo amanhã.',
    ],
    [
      ImageGenerationBlockedReason.REQUEST_TOO_LARGE,
      'O pedido ficou grande demais para o serviço. Escolha menos imagens de referência.',
    ],
    [
      ImageGenerationBlockedReason.MAP_HAS_NO_GRID,
      'Este mapa não tem grade. Defina a grade do mapa para gerar a imagem a partir dele.',
    ],
    [
      ImageGenerationBlockedReason.MAP_IMAGE_TOO_LARGE,
      'A imagem deste mapa tem mais de 16 megapixels (4.000 × 4.000 px). Troque por uma menor para usar o mapa com textura.',
    ],
  ];
  for (const [reason, text] of cases) {
    it(`${ImageGenerationBlockedReason[reason]}`, () => {
      const issue = generateIssue(blocked(reason));
      expect(plain(issue.text)).toBe(plain(text));
      expect(issue.reason).toBe(reason);
    });
  }

  it("the month's limit says how many were used and when it comes back, and carries the status for the count", () => {
    const issue = generateIssue(
      blocked(
        ImageGenerationBlockedReason.LIMIT_REACHED,
        imageStatus({ remaining: 0, usedThisMonth: 20 }),
      ),
    );
    expect(issue.text).toBe('Você usou as 20 imagens de outubro. Volta em 1º de novembro.');
    expect(issue.status?.remaining).toBe(0);
  });

  it('"the players see nothing" says to put the characters on the map', () => {
    const issue = generateIssue(blocked(ImageGenerationBlockedReason.PLAYERS_SEE_NOTHING));
    expect(issue.text).toContain('Os jogadores não veem nada deste mapa agora');
    expect(issue.text).toContain('Ponha os personagens no mapa');
  });

  it('"the map changed" says to generate again, and names what may have changed (the image, the grid or the walls)', () => {
    const issue = generateIssue(blocked(ImageGenerationBlockedReason.MAP_CHANGED));
    expect(issue.text).toContain('a imagem, a grade ou as paredes');
    expect(issue.text).toContain('Gere de novo');
  });

  it('an invalid field names the field the form must point at', () => {
    expect(generateIssue(invalid('npc_character_ids'))).toMatchObject({
      field: 'npc_character_ids',
      text: expect.stringContaining('NPCs marcados'),
    });
    expect(generateIssue(invalid('character_image_ids'))).toMatchObject({
      field: 'character_image_ids',
      text: expect.stringContaining('retrato de um NPC'),
    });
    expect(generateIssue(invalid(''))).toMatchObject({
      field: null,
      text: expect.stringContaining('de 1 a 500 caracteres'),
    });
  });

  it('a campaign or image that is gone, and a server that is down, are said in words', () => {
    expect(generateIssue(new ConnectError('x', Code.NotFound)).text).toContain('não existe mais');
    expect(generateIssue(new ConnectError('x', Code.Unavailable)).text).toContain(
      'Não foi possível falar com o servidor',
    );
  });

  it('the reason of a refused call is read from the detail, not the code alone: a `failed_precondition` without one is just an error', () => {
    expect(blockedOf(new ConnectError('x', Code.FailedPrecondition))).toBeNull();
    expect(blockedOf(blocked(ImageGenerationBlockedReason.OFF))?.reason).toBe(
      ImageGenerationBlockedReason.OFF,
    );
  });
});

describe('a request that ended without a picture', () => {
  it('"o serviço não gerou": a try that gave its slot back says so', () => {
    expect(
      failureText({ failure: ImageGenerationFailure.NO_IMAGE, slotSpent: false, reasonPt: '' }),
    ).toBe(
      'O serviço não gerou esta imagem. Tente descrever a cena de outro jeito. Esta tentativa não gastou nenhuma imagem do mês.',
    );
  });

  it('a gallery too full to keep the picture says that the slot was spent', () => {
    const text = failureText({
      failure: ImageGenerationFailure.GALLERY_FULL,
      slotSpent: true,
      reasonPt: '',
    });
    expect(text).toContain('a galeria não tinha espaço');
    expect(text).toContain('Esta tentativa gastou uma imagem do mês.');
  });

  it("every failure has its own words, and an unknown one falls back to the server's Portuguese", () => {
    for (const failure of [
      ImageGenerationFailure.REFUSED,
      ImageGenerationFailure.UNAVAILABLE,
      ImageGenerationFailure.IMAGE_MISSING,
      ImageGenerationFailure.TIMEOUT,
      ImageGenerationFailure.SERVICE_OFF,
    ]) {
      expect(failureText({ failure, slotSpent: false, reasonPt: 'x' })).not.toContain('x Esta');
    }
    expect(
      failureText({
        failure: ImageGenerationFailure.UNSPECIFIED,
        slotSpent: false,
        reasonPt: 'Deu errado.',
      }),
    ).toContain('Deu errado.');
  });
});

describe('"Usar como imagem do mapa" refused', () => {
  it('the map changed: gere de novo', () => {
    expect(useIssue(blocked(ImageGenerationBlockedReason.MAP_CHANGED))).toContain('Gere de novo');
  });

  it('a gallery too full for the copy a fog map needs', () => {
    expect(useIssue(new ConnectError('x', Code.ResourceExhausted))).toContain(
      'precisaria de uma cópia',
    );
  });
});
