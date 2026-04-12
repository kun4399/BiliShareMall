package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/mattn/go-sqlite3"
	"github.com/mikumifa/BiliShareMall/internal/dao"
	"github.com/mikumifa/BiliShareMall/internal/domain"
	"github.com/mikumifa/BiliShareMall/internal/util"
	"github.com/rs/zerolog/log"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// MailListResponse https://mall.bilibili.com/mall-magic-c/internet/c2c/v2/list

func TestMailListResponse(t *testing.T) {
	t.Skip("manual database fixture test")

	rawJson := `
{
    "code": 0,
    "message": "success",
    "data": {
        "data": [
            {
                "c2cItemsId": 111897655071,
                "type": 1,
                "c2cItemsName": "罗马仕 PSL20 标准款 20000mAh 数码配件",
                "detailDtoList": [
                    {
                        "blindBoxId": 193779426,
                        "itemsId": 10592603,
                        "skuId": 1001637264,
                        "name": "罗马仕 PSL20 标准款 20000mAh 数码配件",
                        "img": "//i0.hdslb.com/bfs/mall/vendor/96/01/9601caf576d57d123f7efabb0f1dfef4.png",
                        "marketPrice": 12900,
                        "type": 0,
                        "isHidden": false
                    }
                ],
                "totalItemsCount": 1,
                "price": 6000,
                "showPrice": "60",
                "showMarketPrice": "129",
                "uid": "41***9",
                "paymentTime": 0,
                "isMyPublish": false,
                "uspaceJumpUrl": null,
                "uface": "https://i0.hdslb.com/bfs/face/948935a8554058e025216fbe8a9d2526ea7af20d.webp",
                "uname": "转***"
            }
        ],
        "nextId": "EwE8WteY9O6zbVe+2IlN40zpVKI+ixW9lgiy6ynSOxk="
    },
    "errtag": 0
}`
	util.PrettyLogger()
	var response domain.MailListResponse
	_ = json.Unmarshal([]byte(rawJson), &response)
	log.Info().Any("MailListResponse", response)

	dbPath := filepath.Join(t.TempDir(), "bsm.db")
	d, _ := dao.NewDatabase(dbPath)
	content, err := os.ReadFile("dict/init.sql")
	if err != nil {
		t.Fatalf("read init.sql: %v", err)
	}
	err = d.Init(string(content))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	num := d.SaveMailListToDB(&response)
	fmt.Printf("%d", num)

}

func Test_list(t *testing.T) {
	t.Skip("manual database listing test")
	util.PrettyLogger()
	dbPath := filepath.Join(t.TempDir(), "bsm.db")
	d, _ := dao.NewDatabase(dbPath)
	content, err := os.ReadFile("dict/init.sql")
	if err != nil {
		panic(err)
	}
	err = d.Init(string(content))
	if err != nil {
		log.Error().Err(err).Msg("Init")
		return
	}
	items, total, err := d.ReadCSCItems(1, 10, "", 1, nil, nil, 800, 900)
	if err != nil {
		log.Error().Err(err).Msg("ReadCSCItems")
	}
	println(total)
	for idx, item := range items {
		log.Info().Any(strconv.Itoa(idx), item).Msg("list")
	}

}

// 只适用于window -tags fts5
func Test_simple_list(t *testing.T) {
	t.Skip("manual sqlite extension check")
	util.PrettyLogger()
	sql.Register("sqlite3_simple",
		&sqlite3.SQLiteDriver{
			Extensions: []string{
				"dict/libsimple-windows-x64/simple",
			},
		})

	db, err := sql.Open("sqlite3_simple", ":memory:")
	if err != nil {
		log.Error().Err(err).Msg("Open")
	}
	defer db.Close()

	// db.Exec("create virtual table repo using github(id, full_name, description, html_url)")
	rows, err := db.Query(`select simple_query('pinyin')`)
	if err != nil {
		log.Error().Err(err).Msg("Query")
	}
	defer rows.Close()
	for rows.Next() {
		var query string
		rows.Scan(&query)
		fmt.Printf("%s\n", query)
	}
}
