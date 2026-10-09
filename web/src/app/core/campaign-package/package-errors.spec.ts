import { Code, ConnectError } from '@connectrpc/connect';

import {
  CampaignPackageBlockedReason,
  PackageProblemReason,
} from '../../../gen/meurpg/campaignpackage/v1/campaignpackage_pb';
import { CampaignCreationRefusedReason } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { OUTCOME_UNKNOWN, SESSION_ENDED } from '../connect/connect-errors';
import { creationRefused, packageBlocked, previewOf } from './campaign-package-testing';
import { exportErrorText, importIssue } from './package-errors';

describe('importIssue', () => {
  it('reads the campaign cap with the number of the detail', () => {
    expect(importIssue(creationRefused(CampaignCreationRefusedReason.LIMIT_REACHED, 10))).toEqual({
      kind: 'cap',
      max: 10,
    });
    expect(importIssue(creationRefused(CampaignCreationRefusedReason.LIMIT_REACHED, 3))).toEqual({
      kind: 'cap',
      max: 3,
    });
  });

  it('reads an account that may not create campaigns', () => {
    expect(importIssue(creationRefused(CampaignCreationRefusedReason.NOT_ALLOWED))).toEqual({
      kind: 'not-allowed',
    });
  });

  it('hands over the preview of a package with problems', () => {
    const preview = previewOf({
      problems: [
        {
          $typeName: 'meurpg.campaignpackage.v1.PackageProblem',
          kind: 1,
          name: '',
          reason: PackageProblemReason.INVALID,
          limit: 0n,
        },
      ],
    });
    const issue = importIssue(packageBlocked(CampaignPackageBlockedReason.HAS_PROBLEMS, preview));
    expect(issue.kind).toBe('problems');
    expect(issue.kind === 'problems' && issue.preview.problems).toHaveLength(1);
  });

  it('says a lost upload (not_found) as such', () => {
    expect(importIssue(new ConnectError('gone', Code.NotFound))).toEqual({ kind: 'lost' });
  });

  it('words the other refusals in Portuguese, never the message', () => {
    const off = importIssue(packageBlocked(CampaignPackageBlockedReason.IMAGES_OFF));
    expect(off).toMatchObject({ kind: 'text', retryable: false, unknown: false });
    expect(off.kind === 'text' && off.text).toContain('armazenamento de arquivos');

    const incomplete = importIssue(packageBlocked(CampaignPackageBlockedReason.INCOMPLETE));
    expect(incomplete.kind === 'text' && incomplete.text).toContain('faltam partes');

    const invalid = importIssue(new ConnectError('size over', Code.InvalidArgument));
    expect(invalid).toMatchObject({ kind: 'text', retryable: false });
    expect(invalid.kind === 'text' && invalid.text).toContain('200 MB');
  });

  it('marks what a retry may fix, and the ambiguous outcome', () => {
    const down = importIssue(new ConnectError('boom', Code.Unavailable));
    expect(down).toMatchObject({ kind: 'text', retryable: true, unknown: false });
    expect(down.kind === 'text' && down.text).not.toContain('boom');

    expect(importIssue(new ConnectError('?', Code.Unknown))).toMatchObject({
      text: OUTCOME_UNKNOWN,
      unknown: true,
      retryable: true,
    });
    expect(importIssue(new ConnectError('', Code.Unauthenticated))).toMatchObject({
      text: SESSION_ENDED,
      retryable: false,
    });
  });

  it('treats a plain network failure as the server being down', () => {
    expect(importIssue(new TypeError('Failed to fetch'))).toMatchObject({
      kind: 'text',
      retryable: true,
    });
  });
});

describe('exportErrorText', () => {
  it('words a server without a blob store and any other failure', () => {
    expect(exportErrorText(new ConnectError('off', Code.FailedPrecondition))).toContain(
      'armazenamento de arquivos',
    );
    expect(exportErrorText(new ConnectError('x', Code.Internal))).not.toContain('x');
    expect(exportErrorText(new ConnectError('', Code.Unauthenticated))).toBe(SESSION_ENDED);
  });
});
