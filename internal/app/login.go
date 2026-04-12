package app

import (
	"github.com/mikumifa/BiliShareMall/internal/http"
	"github.com/rs/zerolog/log"
)

type LoginInfo struct {
	Key      string `json:"key"`
	LoginUrl string `json:"login_url"`
}

func (a *App) GetLoginKeyAndUrl() LoginInfo {
	key, loginUrl, err := http.GetLoginKeyAndUrl()
	if err != nil {
		log.Error().Err(err).Msg("GetLoginKeyAndUrl error")
		return LoginInfo{}
	}
	loginInfo := LoginInfo{
		Key:      key,
		LoginUrl: loginUrl,
	}
	return loginInfo
}

// VerifyLoginResponse 封装登录验证响应的结构体
type VerifyLoginResponse struct {
	Status    string `json:"status"`
	CookieStr string `json:"cookies"`
	Message   string `json:"message"`
}

func (a *App) VerifyLogin(loginKey string) VerifyLoginResponse {
	result, err := http.VerifyLogin(loginKey)
	if err != nil {
		log.Error().Err(err).Msg("VerifyLogin error")
		return VerifyLoginResponse{
			Status:  string(result.Status),
			Message: result.Message,
		}
	}
	ret := VerifyLoginResponse{
		Status:    string(result.Status),
		CookieStr: result.Cookies,
		Message:   result.Message,
	}
	return ret
}
