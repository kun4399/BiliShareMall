package app

import (
	"context"
	"database/sql"
	"github.com/mikumifa/BiliShareMall/internal/dao"
	authsvc "github.com/mikumifa/BiliShareMall/internal/service/auth"
	catalogsvc "github.com/mikumifa/BiliShareMall/internal/service/catalog"
	scrapysvc "github.com/mikumifa/BiliShareMall/internal/service/scrapy"
	"github.com/mikumifa/BiliShareMall/internal/util"
	cache "github.com/patrickmn/go-cache"
	"github.com/rs/zerolog/log"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"os"
	"time"
)

const DatabaseVersion = 4

// App struct
type App struct {
	ctx context.Context
	d   *dao.Database
	c   *cache.Cache

	authService    *authsvc.Service
	catalogService *catalogsvc.Service
	scrapyService  *scrapysvc.Service
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// Startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	var err error
	a.d, err = a.checkAndCreateDatabase(DatabaseVersion)
	if err != nil {
		log.Panic().Err(err).Msg("data/bsm.db NewApp Error")
	}
	content, err := os.ReadFile(util.GetPath("dict/init.sql"))
	if err != nil {
		log.Panic().Err(err).Msg("dict/init.sql NewApp Error")
	}
	err = a.d.Init(string(content))
	if err != nil {
		log.Panic().Err(err).Msg("database init NewApp Error")
	}
	//更新version
	err = a.d.UpdateVersion(DatabaseVersion)
	if err != nil {
		log.Panic().Err(err).Msg("UpdateVersion  Error")
	}
	// 设置超时时间和清理时间
	a.c = cache.New(5*time.Minute, 10*time.Minute)
	a.catalogService = catalogsvc.NewService(a.d, a.c)
	a.authService = authsvc.NewService()
	a.scrapyService = scrapysvc.NewService(a.d, func(eventName string, payload any) {
		runtime.EventsEmit(a.ctx, eventName, payload)
	})
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
		db, err := sql.Open("sqlite3_simple", util.GetPath("data/bsm.db"))
		if err != nil {
			return nil, err
		}
		ret = &dao.Database{Db: db}
	}
	return ret, nil
}

func (a *App) getAuthService() *authsvc.Service {
	if a.authService == nil {
		a.authService = authsvc.NewService()
	}
	return a.authService
}

func (a *App) getCatalogService() *catalogsvc.Service {
	if a.catalogService == nil {
		a.catalogService = catalogsvc.NewService(a.d, a.c)
	}
	return a.catalogService
}

func (a *App) getScrapyService() *scrapysvc.Service {
	if a.scrapyService == nil {
		a.scrapyService = scrapysvc.NewService(a.d, func(eventName string, payload any) {
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, eventName, payload)
			}
		})
	}
	return a.scrapyService
}
