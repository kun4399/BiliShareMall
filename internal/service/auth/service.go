package auth

import (
	"fmt"
	"strings"

	"github.com/kun4399/BiliShareMall/internal/dao"
	bilihttp "github.com/kun4399/BiliShareMall/internal/http"
	"github.com/rs/zerolog/log"
)

type LoginInfo struct {
	Key      string `json:"key"`
	LoginUrl string `json:"login_url"`
}

type VerifyLoginResponse struct {
	Status    string `json:"status"`
	CookieStr string `json:"cookies"`
	Message   string `json:"message"`
}

type SharedLoginSession struct {
	LoggedIn  bool  `json:"loggedIn"`
	UpdatedAt int64 `json:"updatedAt"`
}

type LoginAccount struct {
	ID          int64  `json:"id"`
	UID         string `json:"uid"`
	AccountName string `json:"accountName"`
	LoggedIn    bool   `json:"loggedIn"`
	UpdatedAt   int64  `json:"updatedAt"`
}

type Service struct {
	d *dao.Database
}

func NewService(database *dao.Database) *Service {
	return &Service{d: database}
}

func (s *Service) GetLoginKeyAndUrl() (LoginInfo, error) {
	key, loginURL, err := bilihttp.GetLoginKeyAndUrl()
	if err != nil {
		return LoginInfo{}, err
	}
	return LoginInfo{
		Key:      key,
		LoginUrl: loginURL,
	}, nil
}

func (s *Service) VerifyLogin(loginKey string) (VerifyLoginResponse, error) {
	result, err := bilihttp.VerifyLogin(loginKey)
	if err != nil {
		return VerifyLoginResponse{
			Status:  string(result.Status),
			Message: result.Message,
		}, err
	}
	if strings.TrimSpace(result.Cookies) != "" && s.d != nil {
		cookieStr := strings.TrimSpace(result.Cookies)
		session := bilihttp.ParseBiliSession(cookieStr)
		uid := strings.TrimSpace(session.Cookies["DedeUserID"])
		accountName := uid

		profile, profileErr := bilihttp.GetAccountProfile(cookieStr)
		if profileErr == nil {
			if strings.TrimSpace(profile.UID) != "" {
				uid = strings.TrimSpace(profile.UID)
			}
			if strings.TrimSpace(profile.AccountName) != "" {
				accountName = strings.TrimSpace(profile.AccountName)
			}
		} else {
			log.Warn().Err(profileErr).Msg("get account profile failed, fallback to uid")
		}

		if uid == "" {
			return VerifyLoginResponse{
				Status:  string(result.Status),
				Message: "登录成功但无法解析账号 UID",
			}, fmt.Errorf("login confirmed but uid missing")
		}

		if _, upsertErr := s.d.UpsertAuthAccount(uid, accountName, cookieStr); upsertErr != nil {
			return VerifyLoginResponse{
				Status:  string(result.Status),
				Message: upsertErr.Error(),
			}, upsertErr
		}

		if saveErr := s.d.SaveAuthSession(result.Cookies); saveErr != nil {
			return VerifyLoginResponse{
				Status:  string(result.Status),
				Message: saveErr.Error(),
			}, saveErr
		}
	}
	return VerifyLoginResponse{
		Status:    string(result.Status),
		CookieStr: result.Cookies,
		Message:   result.Message,
	}, nil
}

func (s *Service) GetSharedLoginSession() (SharedLoginSession, error) {
	if s.d == nil {
		return SharedLoginSession{}, nil
	}

	session, err := s.d.GetAuthSession()
	if err != nil {
		return SharedLoginSession{}, err
	}

	cookies := strings.TrimSpace(session.Cookies)
	if cookies == "" {
		return SharedLoginSession{}, nil
	}

	return SharedLoginSession{
		LoggedIn:  bilihttp.ParseBiliSession(cookies).IsLoggedIn(),
		UpdatedAt: session.UpdatedAt.UnixMilli(),
	}, nil
}

func (s *Service) ClearSharedLoginSession() error {
	if s.d == nil {
		return nil
	}
	if err := s.d.ClearAuthSession(); err != nil {
		return err
	}
	return s.d.ClearAuthAccounts()
}

func (s *Service) ListLoginAccounts() ([]LoginAccount, error) {
	if s.d == nil {
		return []LoginAccount{}, nil
	}

	accounts, err := s.d.ListAuthAccounts()
	if err != nil {
		return nil, err
	}
	ret := make([]LoginAccount, 0, len(accounts))
	for _, account := range accounts {
		cookies := strings.TrimSpace(account.Cookies)
		ret = append(ret, LoginAccount{
			ID:          account.ID,
			UID:         strings.TrimSpace(account.UID),
			AccountName: strings.TrimSpace(account.AccountName),
			LoggedIn:    bilihttp.ParseBiliSession(cookies).IsLoggedIn(),
			UpdatedAt:   account.UpdatedAt.UnixMilli(),
		})
	}
	return ret, nil
}

func (s *Service) DeleteLoginAccount(id int64) error {
	if s.d == nil {
		return nil
	}
	if err := s.d.DeleteAuthAccount(id); err != nil {
		return err
	}
	return s.syncDefaultSessionByLatestAccount()
}

func (s *Service) ClearAllLoginAccounts() error {
	if s.d == nil {
		return nil
	}
	if err := s.d.ClearAuthAccounts(); err != nil {
		return err
	}
	return s.d.ClearAuthSession()
}

func (s *Service) ResolveLoginCookie(cookieHeader string) string {
	cookieHeader = strings.TrimSpace(cookieHeader)
	if cookieHeader != "" {
		return cookieHeader
	}
	if s.d == nil {
		return ""
	}

	session, err := s.d.GetAuthSession()
	if err != nil {
		return ""
	}
	cookies := strings.TrimSpace(session.Cookies)
	if cookies != "" {
		return cookies
	}

	accounts, err := s.d.ListAuthAccounts()
	if err != nil || len(accounts) == 0 {
		return ""
	}
	return strings.TrimSpace(accounts[0].Cookies)
}

func (s *Service) syncDefaultSessionByLatestAccount() error {
	if s.d == nil {
		return nil
	}

	accounts, err := s.d.ListAuthAccounts()
	if err != nil {
		return err
	}
	if len(accounts) == 0 {
		return s.d.ClearAuthSession()
	}
	return s.d.SaveAuthSession(accounts[0].Cookies)
}
