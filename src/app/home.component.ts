import { Component, inject } from '@angular/core';
import { Router } from '@angular/router';

@Component({
  selector: 'app-home',
  standalone: true,
  template: `
    <div class="home-container">
      <header class="hero">
        <div class="hero-content">
          <h1 class="title">MeuRPG</h1>
          <p class="subtitle">Crie suas aventuras, construa mundos, viva histórias épicas.</p>
          <div class="cta-buttons">
            <button class="btn btn-primary" (click)="router.navigate(['/personagens', 'novo'])">Começar Jornada</button>
            <button class="btn btn-secondary" (click)="router.navigate(['/personagens'])">Explorar</button>
          </div>
        </div>
      </header>

      <section class="features">
        <div class="feature-card">
          <div class="feature-icon">&#9876;</div>
          <h3>Crie Personagens</h3>
          <p>Monte fichas detalhadas com atributos, habilidades e históricos.</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">&#128214;</div>
          <h3>Gerencie Campanhas</h3>
          <p>Organize sessões, controle NPCs e acompanhe o progresso do grupo.</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">&#127922;</div>
          <h3>Sistema de Dados</h3>
          <p>Role dados virtuais com animações e registre os resultados.</p>
        </div>
      </section>

      <footer class="footer">
        <p>&copy; 2026 MeuRPG — Feito para aventureiros.</p>
      </footer>
    </div>
  `,
  styles: [
    `
      .home-container {
        min-height: 100vh;
        display: flex;
        flex-direction: column;
        font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
      }

      .hero {
        flex: 1;
        display: flex;
        align-items: center;
        justify-content: center;
        text-align: center;
        padding: 2rem;
        background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
        color: #fff;
      }

      .hero-content {
        max-width: 640px;
      }

      .title {
        font-size: 4rem;
        font-weight: 800;
        margin: 0 0 0.5rem;
        letter-spacing: 2px;
        text-shadow: 0 0 20px rgba(255, 215, 0, 0.3);
      }

      .subtitle {
        font-size: 1.25rem;
        color: #a0aec0;
        margin: 0 0 2rem;
        line-height: 1.6;
      }

      .cta-buttons {
        display: flex;
        gap: 1rem;
        justify-content: center;
        flex-wrap: wrap;
      }

      .btn {
        padding: 0.75rem 2rem;
        border: none;
        border-radius: 8px;
        font-size: 1rem;
        font-weight: 600;
        cursor: pointer;
        transition: transform 0.2s, box-shadow 0.2s;
      }

      .btn:hover {
        transform: translateY(-2px);
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
      }

      .btn-primary {
        background: #e2b714;
        color: #1a1a2e;
      }

      .btn-secondary {
        background: transparent;
        color: #e2b714;
        border: 2px solid #e2b714;
      }

      .features {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
        gap: 2rem;
        padding: 4rem 2rem;
        background: #f7fafc;
      }

      .feature-card {
        background: #fff;
        border-radius: 12px;
        padding: 2rem;
        text-align: center;
        box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
        transition: transform 0.2s, box-shadow 0.2s;
      }

      .feature-card:hover {
        transform: translateY(-4px);
        box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1);
      }

      .feature-icon {
        font-size: 2.5rem;
        margin-bottom: 0.75rem;
      }

      .feature-card h3 {
        font-size: 1.2rem;
        margin: 0 0 0.5rem;
        color: #1a202c;
      }

      .feature-card p {
        font-size: 0.9rem;
        color: #718096;
        line-height: 1.5;
        margin: 0;
      }

      .footer {
        text-align: center;
        padding: 1.5rem;
        background: #1a1a2e;
        color: #a0aec0;
        font-size: 0.875rem;
      }
    `,
  ],
})
export class HomeComponent {
  readonly router = inject(Router);
}