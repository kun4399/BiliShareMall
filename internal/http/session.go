package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

type BiliSession struct {
	RawCookie string
	Cookies   map[string]string
}

type fingerprintResponse struct {
	Code int `json:"code"`
	Data struct {
		B3 string `json:"b_3"`
		B4 string `json:"b_4"`
	} `json:"data"`
	Message string `json:"message"`
}

func ParseBiliSession(cookieStr string) *BiliSession {
	session := &BiliSession{
		RawCookie: strings.TrimSpace(cookieStr),
		Cookies:   map[string]string{},
	}

	for _, segment := range strings.Split(cookieStr, ";") {
		pair := strings.SplitN(strings.TrimSpace(segment), "=", 2)
		if len(pair) != 2 {
			continue
		}
		key := strings.TrimSpace(pair[0])
		if key == "" {
			continue
		}
		session.Cookies[key] = strings.TrimSpace(pair[1])
	}
	session.RawCookie = session.CookieHeader()
	return session
}

func (s *BiliSession) CookieHeader() string {
	if s == nil {
		return ""
	}
	if len(s.Cookies) == 0 {
		return strings.TrimSpace(s.RawCookie)
	}

	keys := make([]string, 0, len(s.Cookies))
	for key := range s.Cookies {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		value := s.Cookies[key]
		pairs = append(pairs, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(pairs, "; ")
}

func (s *BiliSession) SetCookie(key, value string) {
	if s == nil || key == "" || value == "" {
		return
	}
	if s.Cookies == nil {
		s.Cookies = map[string]string{}
	}
	s.Cookies[key] = value
	s.RawCookie = s.CookieHeader()
}

func (s *BiliSession) CSRF() string {
	if s == nil {
		return ""
	}
	return s.Cookies["bili_jct"]
}

func (s *BiliSession) IsLoggedIn() bool {
	if s == nil {
		return false
	}
	return s.Cookies["SESSDATA"] != "" && s.Cookies["DedeUserID"] != ""
}

func (s *BiliSession) EnsureFingerprint(ctx context.Context, client *BiliClient) error {
	if s == nil {
		return errors.New("session is nil")
	}
	if s.Cookies["buvid3"] != "" && s.Cookies["buvid4"] != "" {
		return nil
	}

	var response fingerprintResponse
	if err := client.DoJSON(ctx, GET, "https://api.bilibili.com/x/frontend/finger/spi", nil, nil, nil, &response); err != nil {
		return err
	}
	if response.Code != 0 {
		return errors.New(response.Message)
	}
	if response.Data.B3 != "" {
		s.SetCookie("buvid3", response.Data.B3)
	}
	if response.Data.B4 != "" {
		s.SetCookie("buvid4", response.Data.B4)
	}
	return nil
}

func DebugJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(data)
}
