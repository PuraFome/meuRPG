import { ListenController, type VoiceAudio } from './listen-controller';

class FakeAudio implements VoiceAudio {
  src = '';
  paused = false;
  playCalls = 0;
  playResult: Promise<void> = Promise.resolve();
  private readonly listeners = new Map<string, (() => void)[]>();

  play(): Promise<void> {
    this.playCalls++;
    return this.playResult;
  }

  pause(): void {
    this.paused = true;
  }

  addEventListener(type: string, listener: () => void): void {
    this.listeners.set(type, [...(this.listeners.get(type) ?? []), listener]);
  }

  fire(type: string): void {
    (this.listeners.get(type) ?? []).forEach((l) => l());
  }
}

describe('ListenController', () => {
  let made: FakeAudio[];
  let listen: ListenController;
  const flush = () => Promise.resolve().then(() => Promise.resolve());

  beforeEach(() => {
    made = [];
    listen = new ListenController(() => {
      const audio = new FakeAudio();
      made.push(audio);
      return audio;
    });
  });

  it('creates no audio element until a line is heard', () => {
    expect(made).toEqual([]);
    expect(listen.stateOf('a')).toBe('idle');
    expect(listen.labelOf('a')).toBe('Ouvir');
  });

  it('asks for the voice and says so until it plays', () => {
    listen.toggle('a', '/session-messages/a/audio');
    expect(made).toHaveLength(1);
    expect(made[0].src).toBe('/session-messages/a/audio');
    expect(made[0].playCalls).toBe(1);
    expect(listen.stateOf('a')).toBe('loading');
    expect(listen.labelOf('a')).toBe('Gerando a voz…');
    made[0].fire('playing');
    expect(listen.stateOf('a')).toBe('playing');
    expect(listen.labelOf('a')).toBe('Parar');
  });

  it('goes back to idle by itself when the voice ends', () => {
    listen.toggle('a', '/a');
    made[0].fire('playing');
    made[0].fire('ended');
    expect(listen.stateOf('a')).toBe('idle');
  });

  it('a second tap on the same line stops it', () => {
    listen.toggle('a', '/a');
    made[0].fire('playing');
    listen.toggle('a', '/a');
    expect(made[0].paused).toBe(true);
    expect(listen.stateOf('a')).toBe('idle');
  });

  it('stop() stops whatever plays, also while it is loading', () => {
    listen.toggle('a', '/a');
    listen.stop();
    expect(made[0].paused).toBe(true);
    expect(listen.stateOf('a')).toBe('idle');
  });

  it('plays one line at a time: starting another stops the first', () => {
    listen.toggle('a', '/a');
    made[0].fire('playing');
    listen.toggle('b', '/b');
    expect(made[0].paused).toBe(true);
    expect(listen.stateOf('a')).toBe('idle');
    expect(listen.stateOf('b')).toBe('loading');
  });

  it('ignores what a stopped voice still says', () => {
    listen.toggle('a', '/a');
    listen.toggle('b', '/b');
    made[0].fire('playing');
    made[0].fire('error');
    expect(listen.stateOf('a')).toBe('idle');
    expect(listen.stateOf('b')).toBe('loading');
  });

  it('a voice that does not load is a quiet error, and a new tap tries again', () => {
    listen.toggle('a', '/a');
    made[0].fire('error');
    expect(listen.stateOf('a')).toBe('error');
    expect(listen.errorOf('a')).toBe('A voz não carregou. Tente de novo.');
    expect(listen.errorOf('b')).toBe('');
    listen.toggle('a', '/a');
    expect(listen.stateOf('a')).toBe('loading');
    expect(made).toHaveLength(2);
  });

  it('a play() that the browser refuses is the same error', async () => {
    const rejecting = new FakeAudio();
    rejecting.playResult = Promise.reject(new Error('NotAllowedError'));
    const refusing = new ListenController(() => rejecting);
    refusing.toggle('a', '/a');
    await flush();
    expect(refusing.stateOf('a')).toBe('error');
  });

  it('never plays by itself: nothing is created without a tap', () => {
    listen.stop();
    expect(made).toEqual([]);
  });
});
