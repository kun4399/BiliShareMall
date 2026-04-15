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
type LoginAccount = authsvc.LoginAccount

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
	if a.getScrapyService().HasRunningTasks() {
		return authsvc.ErrRunningTasksExist
	}
	return a.getAuthService().ClearSharedLoginSession()
}

func (a *App) ListLoginAccounts() []LoginAccount {
	accounts, err := a.getAuthService().ListLoginAccounts()
	if err != nil {
		log.Error().Err(err).Msg("ListLoginAccounts error")
		return []LoginAccount{}
	}
	return accounts
}

func (a *App) DeleteLoginAccount(id int64) error {
	if a.getScrapyService().IsAnyTaskRunningWithAccount(id) {
		return authsvc.ErrAccountInUse
	}
	return a.getAuthService().DeleteLoginAccount(id)
}

func (a *App) ClearAllLoginAccounts() error {
	if a.getScrapyService().HasRunningTasks() {
		return authsvc.ErrRunningTasksExist
	}
	return a.getAuthService().ClearAllLoginAccounts()
}

func (a *App) ResolveLoginCookie(cookieHeader string) string {
	return a.getAuthService().ResolveLoginCookie(cookieHeader)
}
