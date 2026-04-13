package app

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"time"

	"github.com/mikumifa/BiliShareMall/internal/dao"
	"github.com/mikumifa/BiliShareMall/internal/events"
	authsvc "github.com/mikumifa/BiliShareMall/internal/service/auth"
	catalogsvc "github.com/mikumifa/BiliShareMall/internal/service/catalog"
	scrapysvc "github.com/mikumifa/BiliShareMall/internal/service/scrapy"
	"github.com/mikumifa/BiliShareMall/internal/util"
	cache "github.com/patrickmn/go-cache"
	"github.com/rs/zerolog/log"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const DatabaseVersion = 5

// App struct
type App struct {
	ctx    context.Context
	d      *dao.Database
	c      *cache.Cache
	bus    *events.Bus
	initMu sync.Mutex

	authService       *authsvc.Service
	catalogService    *catalogsvc.Service
	scrapyService     *scrapysvc.Service
	wailsEventsCancel func()
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// Startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	if err := a.Initialize(); err != nil {
		log.Panic().Err(err).Msg("initialize app error")
	}

	a.attachWailsEvents()
}

func (a *App) Initialize() error {
	a.initMu.Lock()
	defer a.initMu.Unlock()

	if a.d != nil && a.bus != nil && a.authService != nil && a.catalogService != nil && a.scrapyService != nil {
		return nil
	}

	database, err := a.checkAndCreateDatabase(DatabaseVersion)
	if err != nil {
		return err
	}

	content, err := os.ReadFile(util.GetPath("dict/init.sql"))
	if err != nil {
		return err
	}
	if err = database.Init(string(content)); err != nil {
		return err
	}
	if err = database.EnsureMonitorRuleRemarkColumn(); err != nil {
		return err
	}
	if err = database.UpdateVersion(DatabaseVersion); err != nil {
		return err
	}

	a.d = database
	a.bus = events.NewBus()
	a.c = cache.New(5*time.Minute, 10*time.Minute)
	a.authService = authsvc.NewService()
	a.catalogService = catalogsvc.NewService(a.d, a.c)
	a.scrapyService = scrapysvc.NewService(a.d, a.bus.Emit)
	return nil
}

// checkAndCreateDatabase 测试当前数据库的版本号，如果版本号低就重新建库
func (a *App) checkAndCreateDatabase(nowVersion int) (ret *dao.Database, err error) {
	ret, err = dao.NewDatabase(util.GetPath("data/bsm.db"))
	if err != nil {
		return nil, err
	}
	currentVersion, _ := ret.GetVersion()
	// 如果版本号小于minVersion，则删除现有数据库并重新创建
	if currentVersion < nowVersion {
		log.Warn().Int("currentVersion", currentVersion).Int("nowVersion", nowVersion).Msg("recreate database because the database version is old")
		err := ret.Close()
		if err != nil {
			return nil, err
		}
		err = os.Remove(util.GetPath("data/bsm.db"))
		if err != nil {
			return nil, err
		}
		//重新打开
		db, err := sql.Open("sqlite3", util.GetPath("data/bsm.db"))
		if err != nil {
			return nil, err
		}
		ret = &dao.Database{Db: db}
	}
	return ret, nil
}

func (a *App) SubscribeEvents(buffer int) (<-chan events.Event, func(), error) {
	if err := a.Initialize(); err != nil {
		return nil, nil, err
	}
	ch, cancel := a.bus.Subscribe(buffer)
	return ch, cancel, nil
}

func (a *App) attachWailsEvents() {
	if a.ctx == nil || a.bus == nil {
		return
	}

	if a.wailsEventsCancel != nil {
		a.wailsEventsCancel()
		a.wailsEventsCancel = nil
	}

	ch, cancel := a.bus.Subscribe(64)
	a.wailsEventsCancel = cancel

	go func(ctx context.Context, eventsCh <-chan events.Event) {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-eventsCh:
				if !ok {
					return
				}
				runtime.EventsEmit(ctx, event.Name, event.Payload)
			}
		}
	}(a.ctx, ch)
}

func (a *App) getAuthService() *authsvc.Service {
	if a.authService == nil {
		a.authService = authsvc.NewService()
	}
	return a.authService
}

func (a *App) getCatalogService() *catalogsvc.Service {
	if a.catalogService == nil {
		if a.d == nil {
			if err := a.Initialize(); err != nil {
				log.Error().Err(err).Msg("initialize catalog service error")
				return catalogsvc.NewService(nil, cache.New(5*time.Minute, 10*time.Minute))
			}
		}
		if a.c == nil {
			a.c = cache.New(5*time.Minute, 10*time.Minute)
		}
		a.catalogService = catalogsvc.NewService(a.d, a.c)
	}
	return a.catalogService
}

func (a *App) getScrapyService() *scrapysvc.Service {
	if a.scrapyService == nil {
		if a.d == nil {
			if err := a.Initialize(); err != nil {
				log.Error().Err(err).Msg("initialize scrapy service error")
			}
		}
		if a.bus == nil {
			a.bus = events.NewBus()
		}
		a.scrapyService = scrapysvc.NewService(a.d, a.bus.Emit)
	}
	return a.scrapyService
}
