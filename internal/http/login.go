package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	neturl "net/url"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

type LoginStatus string

const (
	LoginStatusPending   LoginStatus = "pending"
	LoginStatusScanned   LoginStatus = "scanned"
	LoginStatusConfirmed LoginStatus = "confirmed"
	LoginStatusExpired   LoginStatus = "expired"
	LoginStatusError     LoginStatus = "error"
)

type LoginPollResult struct {
	Status  LoginStatus `json:"status"`
	Cookies string      `json:"cookies"`
	Message string      `json:"message"`
}

type qrGenerateResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		QRCodeKey string `json:"qrcode_key"`
		URL       string `json:"url"`
	} `json:"data"`
}

type qrPollResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Code         int    `json:"code"`
		Message      string `json:"message"`
		URL          string `json:"url"`
		RefreshToken string `json:"refresh_token"`
	} `json:"data"`
}

func GetLoginKeyAndUrl() (loginKey string, loginURL string, err error) {
	client, err := NewBiliClient()
	if err != nil {
		return "", "", err
	}

	var resp qrGenerateResponse
	if err = client.DoJSON(context.Background(), GET, "https://passport.bilibili.com/x/passport-login/web/qrcode/generate", nil, nil, nil, &resp); err != nil {
		return "", "", err
	}
	if resp.Code != 0 {
		return "", "", fmt.Errorf("generate login qr failed: %s", resp.Message)
	}
	log.Info().Str("loginKey", resp.Data.QRCodeKey).Str("loginUrl", resp.Data.URL).Msg("generated login qrcode")
	return resp.Data.QRCodeKey, resp.Data.URL, nil
}

func VerifyLogin(loginKey string) (LoginPollResult, error) {
	client, err := NewBiliClient()
	if err != nil {
		return LoginPollResult{Status: LoginStatusError, Message: err.Error()}, err
	}

	query := neturl.Values{}
	query.Set("qrcode_key", loginKey)
	req, err := nethttp.NewRequestWithContext(context.Background(), GET, "https://passport.bilibili.com/x/passport-login/web/qrcode/poll?"+query.Encode(), nil)
	if err != nil {
		return LoginPollResult{Status: LoginStatusError, Message: err.Error()}, err
	}
	req.Header.Set("User-Agent", defaultUserAgent)

	resp, err := client.HTTPClient().Do(req)
	if err != nil {
		return LoginPollResult{Status: LoginStatusError, Message: err.Error()}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return LoginPollResult{Status: LoginStatusError, Message: err.Error()}, err
	}

	var payload qrPollResponse
	if err = json.Unmarshal(body, &payload); err != nil {
		return LoginPollResult{Status: LoginStatusError, Message: err.Error()}, err
	}

	log.Info().Int("pollCode", payload.Data.Code).Msg("checked login status")
	result, err := mapLoginPollResult(payload)
	if err != nil {
		return result, err
	}
	if result.Status != LoginStatusConfirmed {
		return result, nil
	}

	cookies, cookieErr := buildLoginCookies(client, resp)
	if cookieErr != nil {
		return LoginPollResult{Status: LoginStatusError, Message: cookieErr.Error()}, cookieErr
	}
	result.Cookies = cookies
	return result, nil
}

func mapLoginPollResult(payload qrPollResponse) (LoginPollResult, error) {
	switch payload.Data.Code {
	case 0:
		return LoginPollResult{
			Status:  LoginStatusConfirmed,
			Message: payload.Data.Message,
		}, nil
	case 86101:
		return LoginPollResult{Status: LoginStatusPending, Message: payload.Data.Message}, nil
	case 86090:
		return LoginPollResult{Status: LoginStatusScanned, Message: payload.Data.Message}, nil
	case 86038:
		return LoginPollResult{Status: LoginStatusExpired, Message: payload.Data.Message}, nil
	default:
		err := fmt.Errorf("unexpected login status %d: %s", payload.Data.Code, payload.Data.Message)
		return LoginPollResult{Status: LoginStatusError, Message: err.Error()}, err
	}
}

func buildLoginCookies(client *BiliClient, resp *nethttp.Response) (string, error) {
	session := ParseBiliSession("")
	if err := session.EnsureFingerprint(context.Background(), client); err != nil {
		return "", err
	}

	for _, value := range resp.Header["Set-Cookie"] {
		pair := strings.Split(value, ";")
		if len(pair) == 0 {
			continue
		}
		parts := strings.SplitN(pair[0], "=", 2)
		if len(parts) != 2 {
			continue
		}
		session.SetCookie(parts[0], parts[1])
	}
	if session.CookieHeader() == "" {
		return "", fmt.Errorf("cookies not found")
	}
	return session.CookieHeader(), nil
}

func init() {
	// Keep the package-level default client responsive for login polling.
	nethttp.DefaultClient.Timeout = 20 * time.Second
}
