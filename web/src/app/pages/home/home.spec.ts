import { TestBed } from '@angular/core/testing';
import { ConnectError, Code } from '@connectrpc/connect';

import { ServerInfoService } from '../../core/system/server-info.service';
import { Home } from './home';

describe('Home', () => {
  function setup(serverInfo: Partial<ServerInfoService>) {
    TestBed.configureTestingModule({
      imports: [Home],
      providers: [{ provide: ServerInfoService, useValue: serverInfo }],
    });
    const fixture = TestBed.createComponent(Home);
    fixture.detectChanges();
    return fixture;
  }

  it('shows the version and commit once the call resolves', async () => {
    const fixture = setup({
      getServerInfo: () => Promise.resolve({ version: 'v0.1.0', commit: 'abc123' } as never),
    });
    await fixture.whenStable();
    fixture.detectChanges();

    const text = (fixture.nativeElement as HTMLElement).textContent ?? '';
    expect(text).toContain('v0.1.0');
    expect(text).toContain('abc123');
  });

  it('shows a clear error state when the call fails', async () => {
    const fixture = setup({
      getServerInfo: () =>
        Promise.reject(new ConnectError('backend indisponível', Code.Unavailable)),
    });
    await fixture.whenStable();
    fixture.detectChanges();

    const text = (fixture.nativeElement as HTMLElement).textContent ?? '';
    expect(text).toContain('backend indisponível');
  });
});
