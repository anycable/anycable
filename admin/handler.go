package admin

import (
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"io/fs"
	"net/http"
	"text/template"

	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/sse"
	"github.com/anycable/anycable-go/version"
	"github.com/joomcode/errorx"
)

const (
	defaultAdminUsername = "anyadmin"

	ssePath = "/events"
)

//go:generate tailwindcss -c ui/tailwind.config.js -i ui/app.css -o ui/build/app.css
func (app *App) MountHandlers(srv *server.HTTPServer) error {
	// It's okay to not check auth for static files
	srv.SetupHandler(app.conf.Path+"/static/*", http.StripPrefix(app.conf.Path+"/static", must(app.StaticHandler())))
	// SSE is protected via JWT tokens
	srv.SetupHandler(app.conf.Path+ssePath, must(app.SSEHandler()))

	// Pages
	homeHandler := http.StripPrefix(
		app.conf.Path,
		app.withAuthHandler(
			must(app.PageHandler("", "home")),
		),
	)
	srv.SetupHandler(app.conf.Path+"/", homeHandler)
	srv.SetupHandler(app.conf.Path, homeHandler)

	srv.SetupHandler(
		app.conf.Path+"/logs*",
		http.StripPrefix(app.conf.Path,
			app.withAuthHandler(
				must(app.PageHandler("/logs", "logs", func(data *pageData) {
					(*data)["LogsURL"] = app.NewEventsURL("logs")
				})),
			),
		),
	)

	return nil
}

//go:embed ui
var static embed.FS

func (app *App) StaticHandler() (http.Handler, error) {
	fsys, err := fs.Sub(static, "ui")
	if err != nil {
		return nil, errorx.Decorate(err, "failed to create static fs")
	}

	return http.FileServer(http.FS(fsys)), nil
}

type pageData = map[string]interface{}

func (app *App) PageHandler(path string, name string, populate ...func(*pageData)) (http.Handler, error) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var path = r.URL.Path
		app.log.Debug("serving HTTP request", "path", path)

		t, err := template.ParseFS(static, "ui/layout.html", "ui/nav.html", "ui/"+name+".html")
		if err != nil {
			app.log.Error("failed to parse HTML template", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Add("Content-Type", "text/html")

		pageData := map[string]interface{}{
			"Root":    app.conf.Path,
			"Version": version.Version(),
		}

		for _, p := range populate {
			p(&pageData)
		}

		if err := t.ExecuteTemplate(w, "layout", pageData); err != nil {
			app.log.Error("failed to render index page", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}), nil
}

func (app *App) SSEHandler() (http.Handler, error) {
	extractor := server.DefaultHeadersExtractor{}
	sseConf := sse.Config{AllowedOrigins: "*"}
	handler := sse.SSEHandler(app.node, app.server.ShutdownCtx(), &extractor, &sseConf, app.log)

	return handler, nil
}

func (app *App) withAuthHandler(next http.Handler) http.Handler {
	if app.conf.Secret == "" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if ok {
			usernameHash := sha256.Sum256([]byte(username))
			passwordHash := sha256.Sum256([]byte(password))
			expectedUsernameHash := sha256.Sum256([]byte(defaultAdminUsername))
			expectedPasswordHash := sha256.Sum256([]byte(app.conf.Secret))

			usernameMatch := (subtle.ConstantTimeCompare(usernameHash[:], expectedUsernameHash[:]) == 1)
			passwordMatch := (subtle.ConstantTimeCompare(passwordHash[:], expectedPasswordHash[:]) == 1)

			if usernameMatch && passwordMatch {
				next.ServeHTTP(w, r)
				return
			}
		}

		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}

func must[T any](h T, err error) T {
	if err != nil {
		panic(err)
	}
	return h
}
