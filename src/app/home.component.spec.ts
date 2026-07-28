import { render, screen } from '@testing-library/angular';
import { HomeComponent } from './home.component';

describe('HomeComponent', () => {
  it('should render title', async () => {
    await render(HomeComponent);
    expect(screen.getByText('MeuRPG')).toBeTruthy();
  });
});
