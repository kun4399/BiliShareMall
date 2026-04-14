package auth

import (
	"strings"

	"github.com/kun4399/BiliShareMall/internal/dao"
	bilihttp "github.com/kun4399/BiliShareMall/internal/http"
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
	return strings.TrimSpace(session.Cookies)
}
