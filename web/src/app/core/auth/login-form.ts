/**
 * Signs in through the server with a hidden same-origin form: `POST /auth/login` with the given
 * fields (`return_to`, and optionally `intent`, `intent_payload` and `prompt`). A form and not
 * `fetch`: signing in is a full-page OIDC redirect, and the fields (a link's secret) travel in
 * the body, never in a URL (docs/architecture.md, "The invite flow" and "The claim flow"). The CSP's
 * `form-action` allows the same origin and the provider's.
 */
export function submitLoginForm(fields: Readonly<Record<string, string>>): void {
  const form = document.createElement('form');
  form.method = 'post';
  form.action = '/auth/login';
  form.hidden = true;
  for (const [name, value] of Object.entries(fields)) {
    const field = document.createElement('input');
    field.type = 'hidden';
    field.name = name;
    field.value = value;
    form.appendChild(field);
  }
  document.body.appendChild(form);
  form.submit();
}
