import { render, screen } from '@testing-library/angular';
import { App } from './app';

describe('App', () => {
  it('should render the home component', async () => {
    await render(App);
    expect(screen.getByText('MeuRPG')).toBeTruthy();
  });
});
