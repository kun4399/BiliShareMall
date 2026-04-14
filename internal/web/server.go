package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	appcore "github.com/kun4399/BiliShareMall/internal/app"
	"github.com/kun4399/BiliShareMall/internal/dao"
	"github.com/kun4399/BiliShareMall/internal/events"
	"github.com/kun4399/BiliShareMall/internal/util"
	"github.com/rs/zerolog/log"
)

const biliCookieHeader = "X-Bili-Cookie"
const imageProxyReferer = "https://mall.bilibili.com/"
const imageProxyUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36"

type AppAPI interface {
	GetLoginKeyAndUrl() appcore.LoginInfo
	VerifyLogin(loginKey string) appcore.VerifyLoginResponse
	GetSharedLoginSession() appcore.SharedLoginSession
	ClearSharedLoginSession() error
	ResolveLoginCookie(cookieHeader string) string
	ListC2CItem(page, pageSize int, filterName string, sortOption int, startTime, endTime int64, fromPrice, toPrice int) (appcore.C2CItemGroupListVO, error)
	GetC2CItemNameBySku(skuID int64) (string, error)
	ListC2CItemDetailBySku(skuID int64, page, pageSize int, sortOption int, statusFilter, cookieStr string) (appcore.C2CItemDetailListVO, error)
	ReadAllScrapyItems() []dao.ScrapyItem
	DeleteScrapyItem(id int) error
	CreateScrapyItem(item dao.ScrapyItem) int64
	StartTask(taskID int, cookies string) error
	DoneTask(taskID int) error
	GetRunningTaskIds() []int
	GetMarketRuntimeConfig(cookieStr string) appcore.MarketRuntimeConfig
	GetMonitorConfig() appcore.MonitorConfig
	SaveMonitorConfig(config appcore.MonitorConfig) error
	ListMonitorRuleHits(limitPerRule int) []appcore.MonitorHitGroup
	SubscribeEvents(buffer int) (<-chan events.Event, func(), error)
}

type Server struct {
	api        AppAPI
	staticRoot string
	imageHTTP  *http.Client
}

func NewServer(api AppAPI, staticRoot string) *Server {
	return &Server{
		api:        api,
		staticRoot: staticRoot,
		imageHTTP:  &http.Client{Timeout: 20 * time.Second},
	}
}

func ResolveStaticRoot() (string, error) {
	if custom := strings.TrimSpace(os.Getenv("BSM_WEB_ROOT")); custom != "" {
		if stat, err := os.Stat(custom); err == nil && stat.IsDir() {
			return filepath.Clean(custom), nil
		}
	}

	candidates := []string{
		filepath.Join(util.GetPath("."), "frontend", "dist"),
		filepath.Join(util.GetPath("."), "dist"),
	}

	base := util.GetPath(".")
	parents := []string{base, filepath.Dir(base), filepath.Dir(filepath.Dir(base))}
	for _, parent := range parents {
		candidates = append(candidates,
			filepath.Join(parent, "frontend", "dist"),
			filepath.Join(parent, "dist"),
		)
	}

	for _, candidate := range candidates {
		if stat, err := os.Stat(candidate); err == nil && stat.IsDir() {
			return filepath.Clean(candidate), nil
		}
	}

	return "", fmt.Errorf("frontend dist not found, set BSM_WEB_ROOT or build frontend first")
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/auth/qr", s.handleLoginQR)
	mux.HandleFunc("GET /api/auth/poll", s.handleLoginPoll)
	mux.HandleFunc("GET /api/auth/session", s.handleLoginSession)
	mux.HandleFunc("DELETE /api/auth/session", s.handleClearLoginSession)
	mux.HandleFunc("GET /api/catalog/items", s.handleCatalogItems)
	mux.HandleFunc("GET /api/catalog/items/{skuId}", s.handleCatalogItemDetail)
	mux.HandleFunc("GET /api/catalog/sku/{skuId}/name", s.handleCatalogSkuName)
	mux.HandleFunc("GET /api/assets/image", s.handleImageProxy)
	mux.HandleFunc("GET /api/scrapy/tasks", s.handleListScrapyTasks)
	mux.HandleFunc("POST /api/scrapy/tasks", s.handleCreateScrapyTask)
	mux.HandleFunc("DELETE /api/scrapy/tasks/{id}", s.handleDeleteScrapyTask)
	mux.HandleFunc("POST /api/scrapy/tasks/{id}/start", s.handleStartScrapyTask)
	mux.HandleFunc("POST /api/scrapy/tasks/{id}/stop", s.handleStopScrapyTask)
	mux.HandleFunc("GET /api/scrapy/runtime-config", s.handleScrapyRuntimeConfig)
	mux.HandleFunc("GET /api/scrapy/running-task-ids", s.handleRunningTaskIDs)
	mux.HandleFunc("GET /api/monitor/config", s.handleMonitorConfig)
	mux.HandleFunc("PUT /api/monitor/config", s.handleSaveMonitorConfig)
	mux.HandleFunc("GET /api/monitor/rule-hits", s.handleMonitorRuleHits)
	mux.HandleFunc("GET /api/events", s.handleEvents)
	mux.Handle("/", s.handleSPA())

	return withLogging(mux)
}

func (s *Server) handleLoginQR(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.api.GetLoginKeyAndUrl())
}

func (s *Server) handleLoginPoll(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.URL.Query().Get("key"))
	if key == "" {
		writeError(w, http.StatusBadRequest, errors.New("key is required"))
		return
	}
	writeJSON(w, http.StatusOK, s.api.VerifyLogin(key))
}

func (s *Server) handleLoginSession(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.api.GetSharedLoginSession())
}

func (s *Server) handleClearLoginSession(w http.ResponseWriter, _ *http.Request) {
	if err := s.api.ClearSharedLoginSession(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCatalogItems(w http.ResponseWriter, r *http.Request) {
	page, err := intQuery(r, "page", 1)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	pageSize, err := intQuery(r, "pageSize", 12)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	sortOption, err := intQuery(r, "sortOption", 1)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	startTime, err := int64Query(r, "startTime", -1)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	endTime, err := int64Query(r, "endTime", -1)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	fromPrice, err := intQuery(r, "fromPrice", -1)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	toPrice, err := intQuery(r, "toPrice", -1)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	result, serviceErr := s.api.ListC2CItem(
		page,
		pageSize,
		strings.TrimSpace(r.URL.Query().Get("keyword")),
		sortOption,
		startTime,
		endTime,
		fromPrice,
		toPrice,
	)
	if serviceErr != nil {
		writeError(w, http.StatusInternalServerError, serviceErr)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleCatalogItemDetail(w http.ResponseWriter, r *http.Request) {
	skuID, err := pathInt64(r, "skuId")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	page, err := intQuery(r, "page", 1)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	pageSize, err := intQuery(r, "pageSize", 10)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	sortOption, err := intQuery(r, "sortOption", 1)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	result, serviceErr := s.api.ListC2CItemDetailBySku(
		skuID,
		page,
		pageSize,
		sortOption,
		strings.TrimSpace(r.URL.Query().Get("statusFilter")),
		s.resolveBiliCookie(r),
	)
	if serviceErr != nil {
		writeError(w, http.StatusInternalServerError, serviceErr)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleCatalogSkuName(w http.ResponseWriter, r *http.Request) {
	skuID, err := pathInt64(r, "skuId")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, serviceErr := s.api.GetC2CItemNameBySku(skuID)
	if serviceErr != nil {
		writeError(w, http.StatusInternalServerError, serviceErr)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"name": result})
}

func (s *Server) handleImageProxy(w http.ResponseWriter, r *http.Request) {
	rawURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if rawURL == "" {
		writeError(w, http.StatusBadRequest, errors.New("url is required"))
		return
	}

	targetURL, err := normalizeImageProxyURL(rawURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !isAllowedImageProxyHost(targetURL.Hostname()) {
		writeError(w, http.StatusBadRequest, errors.New("image host is not allowed"))
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, targetURL.String(), nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.Header.Set("User-Agent", imageProxyUserAgent)
	req.Header.Set("Referer", imageProxyReferer)
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")

	resp, err := s.imageHTTP.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Errorf("fetch image failed"))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		writeError(w, http.StatusBadGateway, fmt.Errorf("image source returned status %d", resp.StatusCode))
		return
	}

	if contentType := strings.TrimSpace(resp.Header.Get("Content-Type")); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, resp.Body)
}

func (s *Server) handleListScrapyTasks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.api.ReadAllScrapyItems())
}

func (s *Server) handleCreateScrapyTask(w http.ResponseWriter, r *http.Request) {
	var payload dao.ScrapyItem
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	id := s.api.CreateScrapyItem(payload)
	if id <= 0 {
		writeError(w, http.StatusInternalServerError, errors.New("create scrapy task failed"))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (s *Server) handleDeleteScrapyTask(w http.ResponseWriter, r *http.Request) {
	taskID, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if serviceErr := s.api.DeleteScrapyItem(taskID); serviceErr != nil {
		writeError(w, http.StatusBadRequest, serviceErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStartScrapyTask(w http.ResponseWriter, r *http.Request) {
	taskID, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if serviceErr := s.api.StartTask(taskID, s.resolveBiliCookie(r)); serviceErr != nil {
		writeError(w, http.StatusBadRequest, serviceErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStopScrapyTask(w http.ResponseWriter, r *http.Request) {
	taskID, err := pathInt(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if serviceErr := s.api.DoneTask(taskID); serviceErr != nil {
		writeError(w, http.StatusBadRequest, serviceErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleScrapyRuntimeConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.api.GetMarketRuntimeConfig(s.resolveBiliCookie(r)))
}

func (s *Server) handleRunningTaskIDs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.api.GetRunningTaskIds())
}

func (s *Server) handleMonitorConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.api.GetMonitorConfig())
}

func (s *Server) handleSaveMonitorConfig(w http.ResponseWriter, r *http.Request) {
	var payload appcore.MonitorConfig
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if serviceErr := s.api.SaveMonitorConfig(payload); serviceErr != nil {
		writeError(w, http.StatusBadRequest, serviceErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMonitorRuleHits(w http.ResponseWriter, r *http.Request) {
	limitPerRule, err := intQuery(r, "limitPerRule", 20)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, s.api.ListMonitorRuleHits(limitPerRule))
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, errors.New("streaming unsupported"))
		return
	}

	ch, cancel, err := s.api.SubscribeEvents(64)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer cancel()

	headers := w.Header()
	headers.Set("Content-Type", "text/event-stream")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Connection", "keep-alive")
	_, _ = io.WriteString(w, ": connected\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			_, _ = io.WriteString(w, ": heartbeat\n\n")
			flusher.Flush()
		case event, ok := <-ch:
			if !ok {
				return
			}
			payload, marshalErr := json.Marshal(event.Payload)
			if marshalErr != nil {
				log.Error().Err(marshalErr).Str("event", event.Name).Msg("marshal sse payload failed")
				continue
			}
			if _, writeErr := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Name, payload); writeErr != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) handleSPA() http.Handler {
	staticFS := os.DirFS(s.staticRoot)
	fileServer := http.FileServer(http.FS(staticFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		cleaned := strings.TrimPrefix(filepath.Clean(r.URL.Path), "/")
		if cleaned == "." || cleaned == "" {
			http.ServeFile(w, r, filepath.Join(s.staticRoot, "index.html"))
			return
		}

		if fileExists(staticFS, cleaned) {
			fileServer.ServeHTTP(w, r)
			return
		}

		http.ServeFile(w, r, filepath.Join(s.staticRoot, "index.html"))
	})
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		next.ServeHTTP(w, r)
		log.Debug().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Dur("duration", time.Since(startedAt)).
			Msg("web request")
	})
}

func fileExists(fsys fs.FS, path string) bool {
	stat, err := fs.Stat(fsys, path)
	return err == nil && !stat.IsDir()
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Error().Err(err).Msg("write json failed")
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{
		"message": err.Error(),
	})
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		return fmt.Errorf("invalid json body: %w", err)
	}
	return nil
}

func pathInt(r *http.Request, key string) (int, error) {
	value, err := strconv.Atoi(r.PathValue(key))
	if err != nil {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return value, nil
}

func pathInt64(r *http.Request, key string) (int64, error) {
	value, err := strconv.ParseInt(r.PathValue(key), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return value, nil
}

func intQuery(r *http.Request, key string, fallback int) (int, error) {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return parsed, nil
}

func int64Query(r *http.Request, key string, fallback int64) (int64, error) {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return parsed, nil
}

func (s *Server) resolveBiliCookie(r *http.Request) string {
	return s.api.ResolveLoginCookie(strings.TrimSpace(r.Header.Get(biliCookieHeader)))
}

func normalizeImageProxyURL(rawURL string) (*neturl.URL, error) {
	normalized := strings.TrimSpace(rawURL)
	if strings.HasPrefix(normalized, "//") {
		normalized = "https:" + normalized
	}
	parsed, err := neturl.Parse(normalized)
	if err != nil {
		return nil, fmt.Errorf("invalid image url")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("invalid image url")
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("invalid image url")
	}
	return parsed, nil
}

func isAllowedImageProxyHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	allowedSuffixes := []string{
		".hdslb.com",
		".biliimg.com",
		".bilibili.com",
	}
	for _, suffix := range allowedSuffixes {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

func ListenAndServe(ctx context.Context, addr string, handler http.Handler) error {
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		return ctx.Err()
	case err := <-errCh:
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
