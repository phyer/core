package core

import (
	// "crypto/sha256"
	// "encoding/hex"
	// "encoding/json"
	// "errors"
	// "fmt"
	// "math/rand"
	// "os"
	// "strconv"
	// "strings"
	"encoding/json"
	"time"
	// simple "github.com/bitly/go-simplejson"
	// "github.com/go-redis/redis"
	// "github.com/phyer/texus/utils"
	logrus "github.com/sirupsen/logrus"
)

type Rsi struct {
	Id         string `json:"_id"`
	core       *Core
	InstID     string    `json:"instID"`
	Period     string    `json:"period"`
	Timestamp  time.Time `json:"timeStamp"`
	Ts         int64     `json:"ts"`
	Count      int       `json:"count"`
	LastUpdate time.Time `json:"lastUpdate"`
	RsiVol     float64   `json:"rsiVol"`
	Confirm    bool      `json:"confirm"`
}
type RsiList struct {
	Count          int    `json:"count,number"`
	LastUpdateTime int64  `json:"lastUpdateTime"`
	UpdateNickName string `json:"updateNickName"`
	List           []*Rsi `json:"list"`
}
type StockRsi struct {
	Id         string `json:"_id"`
	core       *Core
	InstID     string    `json:"instID"`
	Period     string    `json:"period"`
	Timestamp  time.Time `json:"timeStamp"`
	Ts         int64     `json:"ts"`
	Count      int       `json:"count"`
	LastUpdate time.Time `json:"lastUpdate"`
	KVol       float64   `json:"kVol"`
	DVol       float64   `json:"dVol"`
	Confirm    bool      `json:"confirm"`
}
type StockRsiList struct {
	Count          int         `json:"count,number"`
	LastUpdateTime int64       `json:"lastUpdateTime"`
	UpdateNickName string      `json:"updateNickName"`
	List           []*StockRsi `json:"list"`
}

func (rsi *Rsi) PushToWriteLogChan(cr *Core) error {
	did := rsi.InstID + rsi.Period + ToString(rsi.Ts)
	rsi.Id = HashString(did)
	cd, err := json.Marshal(rsi)
	if err != nil {
		logrus.Error("PushToWriteLog json marshal rsi err: ", err)
	}
	wg := WriteLog{
		Content: cd,
		Tag:     "sardine.log.rsi." + rsi.Period,
		Id:      rsi.Id,
	}
	cr.WriteLogChan <- &wg
	return nil
}
func (srsi *StockRsi) PushToWriteLogChan(cr *Core) error {
	did := srsi.InstID + srsi.Period + ToString(srsi.Ts)
	srsi.Id = HashString(did)
	cd, err := json.Marshal(srsi)
	if err != nil {
		logrus.Error("PushToWriteLog json marshal rsi err: ", err)
	}
	wg := WriteLog{
		Content: cd,
		Tag:     "sardine.log.stockRsi." + srsi.Period,
		Id:      srsi.Id,
	}
	cr.WriteLogChan <- &wg
	return nil
}
