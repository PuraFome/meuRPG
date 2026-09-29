package oidctest

import (
	"html/template"
	"net/http"
	"net/url"
)

// loginPage lists the test users, one button each. The form carries the
// authorization request in hidden fields and posts it back to /authorize,
// which checks it again before issuing a code. html/template escapes every
// value, so nothing from the query can inject markup.
var loginPage = template.Must(template.New("login").Parse(`<!doctype html>
<html lang="pt-BR">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>devidp: entrar</title>
<style>
  body { font-family: system-ui, sans-serif; margin: 0; padding: 16px; background: #fafafa; color: #1b1b1b; }
  main { max-width: 32rem; margin: 0 auto; }
  .warning { background: #fff3cd; border: 2px solid #b58100; padding: 12px; border-radius: 8px; }
  ul { list-style: none; padding: 0; }
  li { margin: 12px 0; }
  button { font-size: 1rem; padding: 8px 16px; min-width: 12rem; cursor: pointer; }
  small { display: block; color: #555; margin-top: 4px; }
</style>
</head>
<body>
<main>
  <p class="warning" role="alert"><strong>Provedor OIDC de desenvolvimento.</strong>
    Só existe na sua máquina e no CI: qualquer pessoa entra como qualquer usuário de teste, sem senha.</p>
  <h1>Entrar no MeuRPG</h1>
  <p>Escolha um usuário de teste:</p>
  <form method="post" action="{{.Action}}">
    {{range .Params}}<input type="hidden" name="{{.Name}}" value="{{.Value}}">
    {{end}}<ul>
      {{range .Users}}<li>
        <button type="submit" name="user" value="{{.Subject}}">{{.Name}}</button>
        <small>{{.Email}}{{if ne (printf "%v" .EmailVerified) "true"}} (e-mail não verificado){{end}}</small>
      </li>
      {{end}}</ul>
  </form>
</main>
</body>
</html>
`))

// forwardedParams are the authorization request parameters the login form
// posts back. max_age and prompt are left out: the user is signing in right
// now, which satisfies both.
var forwardedParams = []string{
	"client_id", "redirect_uri", "response_type", "scope",
	"state", "nonce", "code_challenge", "code_challenge_method",
}

type hiddenParam struct{ Name, Value string }

// renderLoginPage shows the login page for a validated authorization
// request.
func (p *Provider) renderLoginPage(w http.ResponseWriter, q url.Values) {
	var params []hiddenParam
	for _, name := range forwardedParams {
		if q.Has(name) {
			params = append(params, hiddenParam{name, q.Get(name)})
		}
	}
	p.mu.Lock()
	users := append([]User(nil), p.users...)
	p.mu.Unlock()

	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("Referrer-Policy", "no-referrer")
	// No scripts at all; the inline <style> is the only exception. The
	// form's redirect back to the client is a navigation, which CSP's
	// form-action would also check, so form-action is left unset.
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; frame-ancestors 'none'")
	_ = loginPage.Execute(w, map[string]any{
		"Action": p.basePath + "/authorize",
		"Params": params,
		"Users":  users,
	})
}
