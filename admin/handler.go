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
	"github.com/joomcode/errorx"
)

const (
	defaultAdminUsername = "anyadmin"

	ssePath = "/events"
)

//go:generate tailwindcss -c ui/tailwind.config.js -i ui/app.css -o ui/build/app.css
func (app *App) MountHandlers(srv *server.HTTPServer) error {
	staticHandler, err := app.StaticHandler()
	if err != nil {
		return errorx.Decorate(err, "failed to create static handler")
	}

	indexHandler, err := app.IndexHandler()
	if err != nil {
		return errorx.Decorate(err, "failed to create page handler")
	}

	sseHandler, err := app.SSEHandler()
	if err != nil {
		return errorx.Decorate(err, "failed to create sse handler")
	}

	// It's okay to not check auth for static files
	srv.SetupHandler(app.conf.Path+"/static/*", http.StripPrefix(app.conf.Path+"/static", staticHandler))
	// SSE is protected via JWT tokens
	srv.SetupHandler(app.conf.Path+ssePath, sseHandler)

	srv.SetupHandler(app.conf.Path+"/", http.StripPrefix(app.conf.Path, app.withAuthHandler(indexHandler)))
	srv.SetupHandler(app.conf.Path, http.StripPrefix(app.conf.Path, app.withAuthHandler(indexHandler)))

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

type indexPageData struct {
	Root    string
	LogsURL string
}

func (app *App) IndexHandler() (http.Handler, error) {
	t, err := template.ParseFS(static, "ui/index.html")
	if err != nil {
		return nil, errorx.Decorate(err, "failed to parse html templates")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var path = r.URL.Path
		app.log.Debug("serving HTTP request", "path", path, "handler", "index")
		w.Header().Add("Content-Type", "text/html")

		pageData := indexPageData{
			Root:    app.conf.Path,
			LogsURL: app.NewEventsURL("logs"),
		}

		if err := t.Execute(w, pageData); err != nil {
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
