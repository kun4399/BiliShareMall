package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kun4399/BiliShareMall/internal/dao"
	"github.com/kun4399/BiliShareMall/internal/events"
	authsvc "github.com/kun4399/BiliShareMall/internal/service/auth"
	catalogsvc "github.com/kun4399/BiliShareMall/internal/service/catalog"
	scrapysvc "github.com/kun4399/BiliShareMall/internal/service/scrapy"
	"github.com/kun4399/BiliShareMall/internal/util"
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
		a.handleStartupError(err)
		return
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
	initialized := false
	defer func() {
		if !initialized {
			_ = database.Close()
		}
	}()

	currentVersion, err := readDatabaseVersion(database)
	if err != nil {
		return err
	}
	if currentVersion < DatabaseVersion {
		if err = a.setupDatabase(database, DatabaseVersion); err != nil {
			return err
		}
	}

	a.d = database
	a.bus = events.NewBus()
	a.c = cache.New(5*time.Minute, 10*time.Minute)
	a.authService = authsvc.NewService(a.d)
	a.catalogService = catalogsvc.NewService(a.d, a.c)
	a.scrapyService = scrapysvc.NewService(a.d, a.bus.Emit)
	initialized = true
	return nil
}

// checkAndCreateDatabase 测试当前数据库的版本号，如果版本号低就重新建库
func (a *App) checkAndCreateDatabase(nowVersion int) (ret *dao.Database, err error) {
	dbPath := util.GetPath("data/bsm.db")
	ret, err = dao.NewDatabase(dbPath)
	if err != nil {
		return nil, err
	}
	currentVersion, err := readDatabaseVersion(ret)
	if err != nil {
		_ = ret.Close()
		return nil, err
	}
	// 如果版本号小于minVersion，则删除现有数据库并重新创建
	if currentVersion > 0 && currentVersion < nowVersion {
		log.Warn().Int("currentVersion", currentVersion).Int("nowVersion", nowVersion).Msg("recreate database because the database version is old")
		err := ret.Close()
		if err != nil {
			return nil, err
		}
		err = os.Remove(dbPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		// 重新打开
		db, err := dao.NewDatabase(dbPath)
		if err != nil {
			return nil, err
		}
		ret = db
	}
	return ret, nil
}

func (a *App) setupDatabase(database *dao.Database, version int) error {
	content, err := os.ReadFile(util.GetPath("dict/init.sql"))
	if err != nil {
		return err
	}
	if err = database.Init(string(content)); err != nil {
		return err
	}
	if err = database.EnsureC2CItemReferencePriceColumn(); err != nil {
		return err
	}
	if err = database.EnsureMonitorRuleRemarkColumn(); err != nil {
		return err
	}
	if err = database.EnsureAuthSessionTable(); err != nil {
		return err
	}
	return database.UpdateVersion(version)
}

func readDatabaseVersion(database *dao.Database) (int, error) {
	version, err := database.GetVersion()
	if err == nil {
		return version, nil
	}
	if errors.Is(err, sql.ErrNoRows) || isMissingVersionTableError(err) {
		return 0, nil
	}
	return 0, err
}

func isMissingVersionTableError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "no such table: version")
}

func isSQLiteLockedError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database is locked") || strings.Contains(message, "database table is locked")
}

func (a *App) handleStartupError(err error) {
	log.Error().Err(err).Msg("initialize app error")

	if a.ctx == nil {
		return
	}

	message := fmt.Sprintf("BiliShareMall 启动失败。\n\n数据文件：%s\n\n错误：%v", filepath.Clean(util.GetPath("data/bsm.db")), err)
	if isSQLiteLockedError(err) {
		message = fmt.Sprintf("BiliShareMall 无法访问共享数据库，可能已有另一个桌面端或 Web 进程正在使用同一份数据。\n\n数据文件：%s\n\n请关闭其他正在占用该数据目录的实例后重试。\n\n错误：%v", filepath.Clean(util.GetPath("data/bsm.db")), err)
	}

	if _, dialogErr := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:    runtime.ErrorDialog,
		Title:   "BiliShareMall 启动失败",
		Message: message,
		Buttons: []string{"确定"},
	}); dialogErr != nil {
		log.Error().Err(dialogErr).Msg("show startup error dialog failed")
	}

	runtime.Quit(a.ctx)
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
		a.authService = authsvc.NewService(a.d)
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
