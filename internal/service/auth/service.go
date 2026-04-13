package auth

import (
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

type Service struct{}

func NewService() *Service {
	return &Service{}
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
	return VerifyLoginResponse{
		Status:    string(result.Status),
		CookieStr: result.Cookies,
		Message:   result.Message,
	}, nil
}
