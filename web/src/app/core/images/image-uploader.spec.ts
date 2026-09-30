import { TestBed } from '@angular/core/testing';

import { ImageUploader } from './image-uploader';
import { UploadFailed } from './upload-errors';

/** Just enough of XMLHttpRequest to drive `ImageUploader` by hand. */
class FakeXhr {
  static last: FakeXhr;
  method = '';
  url = '';
  status = 0;
  responseText = '';
  responseType = '';
  body: FormData | null = null;
  requestHeaders: Record<string, string> = {};
  aborted = false;
  private readonly listeners: Record<string, (() => void)[]> = {};
  readonly upload = {
    progressListeners: [] as ((e: {
      lengthComputable: boolean;
      loaded: number;
      total: number;
    }) => void)[],
    addEventListener(
      _type: string,
      fn: (e: { lengthComputable: boolean; loaded: number; total: number }) => void,
    ) {
      this.progressListeners.push(fn);
    },
  };

  constructor() {
    FakeXhr.last = this;
  }

  open(method: string, url: string): void {
    this.method = method;
    this.url = url;
  }

  setRequestHeader(name: string, value: string): void {
    this.requestHeaders[name] = value;
  }

  addEventListener(type: string, fn: () => void): void {
    (this.listeners[type] ??= []).push(fn);
  }

  send(body: FormData): void {
    this.body = body;
  }

  abort(): void {
    this.aborted = true;
    this.fire('abort');
  }

  respond(status: number, body: string): void {
    this.status = status;
    this.responseText = body;
    this.fire('load');
  }

  fire(type: string): void {
    for (const fn of this.listeners[type] ?? []) {
      fn();
    }
  }
}

const imageJson = {
  id: 'img-1',
  campaignId: 'camp-1',
  name: 'ruinas',
  contentType: 'image/jpeg',
  width: 2000,
  height: 1400,
  byteSize: 1500000,
  createdAt: '2026-09-30T13:58:06.968040Z',
  url: '/images/img-1',
  thumbnailUrl: '/images/img-1/thumb',
};

describe('ImageUploader', () => {
  let uploader: ImageUploader;
  const file = new File(['jpeg bytes'], 'ruinas.jpg', { type: 'image/jpeg' });

  beforeEach(() => {
    vi.stubGlobal('XMLHttpRequest', FakeXhr);
    uploader = TestBed.inject(ImageUploader);
  });

  afterEach(() => vi.unstubAllGlobals());

  it('posts campaign_id and then file, and lets the browser set the Content-Type', async () => {
    const done = uploader.upload('camp-1', file);
    const xhr = FakeXhr.last;
    expect([xhr.method, xhr.url]).toEqual(['POST', '/uploads/images']);
    expect([...xhr.body!.keys()]).toEqual(['campaign_id', 'file']);
    expect(xhr.body!.get('campaign_id')).toBe('camp-1');
    expect((xhr.body!.get('file') as File).name).toBe('ruinas.jpg');
    expect(xhr.requestHeaders).toEqual({});

    xhr.respond(201, JSON.stringify(imageJson));
    const image = await done;
    expect(image.id).toBe('img-1');
    expect(image.thumbnailUrl).toBe('/images/img-1/thumb');
    expect(image.width).toBe(2000);
  });

  it('reports progress from 0 to 1', () => {
    const progress: number[] = [];
    void uploader.upload('camp-1', file, { onProgress: (f) => progress.push(f) });
    for (const fn of FakeXhr.last.upload.progressListeners) {
      fn({ lengthComputable: true, loaded: 60, total: 100 });
    }
    expect(progress).toEqual([0.6]);
  });

  it('rejects with the reason the server gave', async () => {
    const done = uploader.upload('camp-1', file);
    FakeXhr.last.respond(400, '{"code":"invalid_argument","reason":"CORRUPT","message":"bad"}');
    await expect(done).rejects.toEqual(new UploadFailed('CORRUPT'));
  });

  it('rejects with NETWORK when the connection drops, and CANCELED on abort', async () => {
    const dropped = uploader.upload('camp-1', file);
    FakeXhr.last.fire('error');
    await expect(dropped).rejects.toMatchObject({ kind: 'NETWORK' });

    const controller = new AbortController();
    const canceled = uploader.upload('camp-1', file, { signal: controller.signal });
    controller.abort();
    expect(FakeXhr.last.aborted).toBe(true);
    await expect(canceled).rejects.toMatchObject({ kind: 'CANCELED' });
  });
});
