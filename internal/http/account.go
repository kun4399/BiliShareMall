package http

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type AccountProfile struct {
	UID         string
	AccountName string
}

type navProfileResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Mid   any    `json:"mid"`
		Uname string `json:"uname"`
	} `json:"data"`
}

func GetAccountProfile(cookieHeader string) (AccountProfile, error) {
	session := ParseBiliSession(cookieHeader)
	uid := strings.TrimSpace(session.Cookies["DedeUserID"])
	if uid == "" {
		return AccountProfile{}, fmt.Errorf("missing DedeUserID")
	}

	client, err := NewBiliClient()
	if err != nil {
		return AccountProfile{UID: uid, AccountName: uid}, err
	}

	var resp navProfileResponse
	err = client.DoJSON(
		context.Background(),
		GET,
		"https://api.bilibili.com/x/web-interface/nav",
		nil,
		nil,
		map[string]string{
			"Cookie":  session.CookieHeader(),
			"Referer": "https://www.bilibili.com/",
			"Origin":  "https://www.bilibili.com",
		},
		&resp,
	)
	if err != nil {
		return AccountProfile{UID: uid, AccountName: uid}, err
	}

	if resp.Code != 0 {
		return AccountProfile{UID: uid, AccountName: uid}, fmt.Errorf("%s", resp.Message)
	}

	profile := AccountProfile{
		UID:         uid,
		AccountName: strings.TrimSpace(resp.Data.Uname),
	}
	if profile.AccountName == "" {
		profile.AccountName = uid
	}

	if profile.UID == "" {
		profile.UID = parseMid(resp.Data.Mid)
	}
	if profile.UID == "" {
		profile.UID = uid
	}

	return profile, nil
}

func parseMid(mid any) string {
	switch value := mid.(type) {
	case string:
		return strings.TrimSpace(value)
	case float64:
		return strconv.FormatInt(int64(value), 10)
	default:
		return ""
	}
}
