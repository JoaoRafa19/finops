package middleware

import (
	"finops/internal/observability"
	"net/http"
	"runtime/debug"
	"strings"
)

// NotFoundInterceptor swaps the default 404 plain-text response for a styled
// HTML page. Wraps the given handler; if the downstream writes a 404 with a
// plain-text body, we intercept before the client sees it.
func NotFoundInterceptor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bw := &bufferedWriter{ResponseWriter: w}
		next.ServeHTTP(bw, r)

		// Só reescreve 404 plain-text (o Go default é "404 page not found\n"),
		// preservando 404 já-estilizados de handlers específicos.
		if bw.status == http.StatusNotFound && !bw.wroteHTML() && !bw.headerFlushed {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(notFoundPageHTML))
			return
		}
		bw.flush(w)
	})
}

// PanicRecover captures panics from handlers, logs them, and serves a styled
// 500 page instead of letting the process crash the request.
func PanicRecover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				observability.Logger(r.Context()).Error("panic_recovered", "panic", rec, "stack", string(debug.Stack()))
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(serverErrorPageHTML))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ponytail: buffer completo em memória. Substituir por streaming interceptor
// (write direto se status != 404 no primeiro chunk) se surgir download >1MB.

// bufferedWriter buffers status + body until we decide whether to intercept.
type bufferedWriter struct {
	http.ResponseWriter
	status        int
	body          []byte
	headerFlushed bool
}

func (b *bufferedWriter) WriteHeader(code int) {
	b.status = code
}

func (b *bufferedWriter) Write(p []byte) (int, error) {
	b.body = append(b.body, p...)
	return len(p), nil
}

func (b *bufferedWriter) wroteHTML() bool {
	ct := b.ResponseWriter.Header().Get("Content-Type")
	if strings.HasPrefix(ct, "text/html") {
		return true
	}
	body := string(b.body)
	return strings.Contains(body, "<!DOCTYPE") || strings.Contains(body, "<html")
}

func (b *bufferedWriter) flush(w http.ResponseWriter) {
	if b.status != 0 {
		w.WriteHeader(b.status)
	}
	if len(b.body) > 0 {
		_, _ = w.Write(b.body)
	}
}

const notFoundPageHTML = `<!DOCTYPE html>
<html lang="pt-BR"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Página não encontrada · Finops</title>
<script src="https://cdn.tailwindcss.com"></script>
<link href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.5.2/css/all.min.css" rel="stylesheet">
</head><body class="min-h-screen bg-slate-100 flex items-center justify-center px-4">
<div class="max-w-md w-full rounded-2xl border border-slate-200 bg-white p-8 shadow-lg text-center">
<div class="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-slate-100 text-2xl text-slate-500"><i class="fa-solid fa-compass"></i></div>
<p class="text-xs font-bold uppercase tracking-widest text-slate-400">Erro 404</p>
<h1 class="mt-1 text-2xl font-extrabold text-slate-900">Página não encontrada</h1>
<p class="mt-3 text-sm text-slate-600">O endereço que você tentou acessar não existe ou foi movido.</p>
<a href="/" class="mt-6 inline-flex items-center justify-center rounded-xl bg-slate-900 px-6 py-3 text-sm font-bold text-white transition hover:bg-slate-800">Voltar para a home</a>
</div></body></html>`

const serverErrorPageHTML = `<!DOCTYPE html>
<html lang="pt-BR"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Erro interno · Finops</title>
<script src="https://cdn.tailwindcss.com"></script>
<link href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.5.2/css/all.min.css" rel="stylesheet">
</head><body class="min-h-screen bg-slate-100 flex items-center justify-center px-4">
<div class="max-w-md w-full rounded-2xl border border-slate-200 bg-white p-8 shadow-lg text-center">
<div class="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-rose-100 text-2xl text-rose-600"><i class="fa-solid fa-triangle-exclamation"></i></div>
<p class="text-xs font-bold uppercase tracking-widest text-slate-400">Erro 500</p>
<h1 class="mt-1 text-2xl font-extrabold text-slate-900">Algo deu errado</h1>
<p class="mt-3 text-sm text-slate-600">Encontramos um erro inesperado. A equipe já foi notificada.</p>
<a href="/" class="mt-6 inline-flex items-center justify-center rounded-xl bg-slate-900 px-6 py-3 text-sm font-bold text-white transition hover:bg-slate-800">Voltar para a home</a>
</div></body></html>`
