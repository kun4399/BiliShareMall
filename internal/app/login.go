package app

import (
	authsvc "github.com/kun4399/BiliShareMall/internal/service/auth"
	"github.com/rs/zerolog/log"
)

type LoginInfo = authsvc.LoginInfo

func (a *App) GetLoginKeyAndUrl() LoginInfo {
	loginInfo, err := a.getAuthService().GetLoginKeyAndUrl()
	if err != nil {
		log.Error().Err(err).Msg("GetLoginKeyAndUrl error")
		return LoginInfo{}
	}
	return loginInfo
}

type VerifyLoginResponse = authsvc.VerifyLoginResponse

func (a *App) VerifyLogin(loginKey string) VerifyLoginResponse {
	result, err := a.getAuthService().VerifyLogin(loginKey)
	if err != nil {
		log.Error().Err(err).Msg("VerifyLogin error")
		return result
	}
	return result
}
