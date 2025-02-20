package market

import (
	"encoding/json"
	"time"

	"github.com/phyer/core/internal/core"  // 新增
	"github.com/phyer/core/internal/utils" // 新增
)

type TickerInfo struct {
	Id         string    `json:"_id"`
	InstID     string    `json:"instID"`
	Last       float64   `json:"last"`
	LastUpdate time.Time `json:"lastUpdate"`
	InstType   string    `json:"instType"`
	VolCcy24h  float64   `json:"volCcy24h"`
	Ts         int64     `json:"ts"`
}

type TickerInfoResp struct {
	InstID    string `json:"instID"`
	Last      string `json:"last"`
	InstType  string `json:"instType"`
	VolCcy24h string `json:"volCcy24h"`
	Ts        string `json:"ts"`
}

func (tir *TickerInfoResp) Convert() TickerInfo {
	ti := TickerInfo{
		Id:         utils.HashString(tir.InstID + tir.Ts),
		InstID:     tir.InstID,
		InstType:   tir.InstType,
		Last:       utils.ToFloat64(tir.Last),
		VolCcy24h:  utils.ToFloat64(tir.VolCcy24h),
		Ts:         utils.ToInt64(tir.Ts),
		LastUpdate: time.Now(),
	}
	return ti
}

// TODO 有待实现
func (ti *TickerInfo) SetToKey(cr *core.Core) error {
	js, _ := json.Marshal(*ti)
	plateName := ti.InstID + "|tickerInfo"
	_, err := cr.RedisLocalCli.Set(plateName, string(js), 0).Result()
	return err
}
