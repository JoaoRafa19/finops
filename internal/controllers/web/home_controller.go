package web

import (
	"fmt"
	"net/http"

	"finops/internal/web/middleware"
)

type HomeController struct{}

func NewHomeController() *HomeController {
	return &HomeController{}
}

func (c *HomeController) Home(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html lang="pt-BR">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>Finops</title>
  </head>
  <body>
    <main style="max-width:640px;margin:48px auto;font-family:sans-serif;">
      <h1>Finops online</h1>
      <p>Usuário autenticado: %s</p>

      <form method="post" action="/logout">
        <input type="hidden" name="_csrf" value="%s" />
        <button type="submit">Sair</button>
      </form>
    </main>
  </body>
</html>`, session.Email, session.CSRFToken)
}
