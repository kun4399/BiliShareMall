package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/kun4399/BiliShareMall/internal/app"
	"github.com/kun4399/BiliShareMall/internal/bootstrap"
	"github.com/kun4399/BiliShareMall/internal/util"
	websrv "github.com/kun4399/BiliShareMall/internal/web"
	"github.com/rs/zerolog/log"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-healthcheck" {
		if runHealthCheck() {
			return
		}
		os.Exit(1)
	}

	bootstrap.InitEnv(bootstrap.InitOptions{})

	if err := util.FileLogger(); err != nil {
		log.Panic().Err(err).Msg("init file logger failed")
	}

	application := app.NewApp()
	if err := application.Initialize(); err != nil {
		log.Panic().Err(err).Msg("initialize application failed")
	}

	staticRoot, err := websrv.ResolveStaticRoot()
	if err != nil {
		log.Panic().Err(err).Msg("resolve frontend dist failed")
	}

	addr := bootstrap.HTTPAddr()
	log.Info().Str("addr", addr).Str("staticRoot", staticRoot).Msg("starting web server")
	log.Info().Str("addr", addr).Msg("standalone web runtime active via cmd/web; use this listener for browser access instead of the desktop app port")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	server := websrv.NewServer(application, staticRoot)
	if err = websrv.ListenAndServe(ctx, addr, server.Handler()); err != nil && !errors.Is(err, context.Canceled) {
		log.Panic().Err(err).Msg("web server stopped with error")
	}
}

func runHealthCheck() bool {
	addr := strings.TrimSpace(bootstrap.HTTPAddr())
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + addr + "/api/healthz")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
