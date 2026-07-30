package main

import (
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/phonyg/phonyg/internal/admin"
	"github.com/phonyg/phonyg/internal/capture"
	"github.com/phonyg/phonyg/internal/config"
	"github.com/phonyg/phonyg/internal/healthcheck"
	"github.com/phonyg/phonyg/internal/proxy"
	"github.com/phonyg/phonyg/internal/seed"
	"github.com/phonyg/phonyg/internal/snapshot"
	"github.com/phonyg/phonyg/internal/store"
	"github.com/phonyg/phonyg/internal/webembed"
)

func main() {
	cfg := config.Load()
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatal(err)
	}
	st, err := store.Open(cfg.DBPath())
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	if err := seed.EnsureBuiltinPresets(st); err != nil {
		log.Fatal(err)
	}

	secret, err := admin.EnsureJWTSecret(cfg.JWTSecretPath(), cfg.JWTSecret)
	if err != nil {
		log.Fatal(err)
	}

	snap := snapshot.NewManager(st)
	if err := snap.Reload(); err != nil {
		log.Fatal(err)
	}

	capMgr := capture.New(st)
	hc := healthcheck.New(st, snap)
	hc.Start()

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	r.MaxMultipartMemory = cfg.MaxBodyBytes

	auth := &admin.Auth{Store: st, Secret: secret, TTLHours: cfg.JWTTTLHours}
	api := &admin.API{Store: st, Auth: auth, Snap: snap, Health: hc, Capture: capMgr}
	api.Register(r)

	ph := proxy.NewHandler(snap, st, capMgr, cfg.MaxBodyBytes)
	r.Any("/v1/*path", ph.Handle)

	sub, err := fs.Sub(webembed.DistFS, "dist")
	if err == nil {
		fileServer := http.FileServer(http.FS(sub))
		r.NoRoute(func(c *gin.Context) {
			p := c.Request.URL.Path
			if strings.HasPrefix(p, "/api") || strings.HasPrefix(p, "/v1") {
				c.JSON(404, gin.H{"error": "not found"})
				return
			}
			fp := strings.TrimPrefix(path.Clean(p), "/")
			if fp == "" || fp == "." {
				fp = "index.html"
			}
			if f, err := sub.Open(fp); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
			c.Request.URL.Path = "/"
			fileServer.ServeHTTP(c.Writer, c.Request)
		})
	} else {
		r.GET("/", func(c *gin.Context) {
			c.String(200, "PhonyG API is running. Frontend not embedded yet.")
		})
	}

	log.Printf("PhonyG listening on %s (data=%s)", cfg.Addr, cfg.DataDir)
	if err := r.Run(cfg.Addr); err != nil {
		log.Fatal(err)
	}
}
