package route

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strings"
	"unsafe"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/go-chi/render"
	"github.com/gorilla/websocket"

	"github.com/clash-dev/clash/internal/log"
)

var (
	serverSecret = ""
	serverAddr   = ""

	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

type Traffic struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

func Start(addr string, secret string) {
	if serverAddr != "" {
		return
	}

	serverAddr = addr
	serverSecret = secret

	r := chi.NewRouter()

	cors := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
		MaxAge:         300,
	})

	r.Use(cors.Handler)
	r.Group(func(r chi.Router) {
		r.Use(authentication)

		r.Mount("/configs", configRouter())
		r.Mount("/proxies", proxyRouter())
		r.Mount("/providers/proxies", proxyProviderRouter())
	})

	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Errorln("External controller listen error: %s", err)
		return
	}
	serverAddr = l.Addr().String()
	log.Infoln("RESTful API listening at: %s", serverAddr)
	if err = http.Serve(l, r); err != nil {
		log.Errorln("External controller serve error: %s", err)
	}
}

func safeEqual(a, b string) bool {
	aBuf := unsafe.Slice(unsafe.StringData(a), len(a))
	bBuf := unsafe.Slice(unsafe.StringData(b), len(b))
	return subtle.ConstantTimeCompare(aBuf, bBuf) == 1
}

func authentication(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if serverSecret == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Browser websocket not support custom header
		if websocket.IsWebSocketUpgrade(r) && r.URL.Query().Get("token") != "" {
			token := r.URL.Query().Get("token")
			if !safeEqual(token, serverSecret) {
				render.Status(r, http.StatusUnauthorized)
				render.JSON(w, r, ErrUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		header := r.Header.Get("Authorization")
		bearer, token, found := strings.Cut(header, " ")

		hasInvalidHeader := bearer != "Bearer"
		hasInvalidSecret := !found || !safeEqual(token, serverSecret)
		if hasInvalidHeader || hasInvalidSecret {
			render.Status(r, http.StatusUnauthorized)
			render.JSON(w, r, ErrUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}
