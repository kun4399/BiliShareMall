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
type SharedLoginSession = authsvc.SharedLoginSession

func (a *App) VerifyLogin(loginKey string) VerifyLoginResponse {
	result, err := a.getAuthService().VerifyLogin(loginKey)
	if err != nil {
		log.Error().Err(err).Msg("VerifyLogin error")
		return result
	}
	return result
}

func (a *App) GetSharedLoginSession() SharedLoginSession {
	result, err := a.getAuthService().GetSharedLoginSession()
	if err != nil {
		log.Error().Err(err).Msg("GetSharedLoginSession error")
		return SharedLoginSession{}
	}
	return result
}

func (a *App) ClearSharedLoginSession() error {
	return a.getAuthService().ClearSharedLoginSession()
}

func (a *App) ResolveLoginCookie(cookieHeader string) string {
	return a.getAuthService().ResolveLoginCookie(cookieHeader)
}
