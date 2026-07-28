import { render, screen } from '@testing-library/angular';
import { App } from './app';
import { HomeComponent } from './home.component';

describe('App', () => {
  it('should render the home component', async () => {
    await render(App, {
      routes: [{ path: '', component: HomeComponent }],
      initialRoute: '/',
    });
    expect(screen.getByText('MeuRPG')).toBeTruthy();
  });
});
