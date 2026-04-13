package main

import (
	"context"
	"errors"
	"os/signal"
	"syscall"

	"github.com/kun4399/BiliShareMall/internal/app"
	"github.com/kun4399/BiliShareMall/internal/bootstrap"
	"github.com/kun4399/BiliShareMall/internal/util"
	websrv "github.com/kun4399/BiliShareMall/internal/web"
	"github.com/rs/zerolog/log"
)

func main() {
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	server := websrv.NewServer(application, staticRoot)
	if err = websrv.ListenAndServe(ctx, addr, server.Handler()); err != nil && !errors.Is(err, context.Canceled) {
		log.Panic().Err(err).Msg("web server stopped with error")
	}
}
