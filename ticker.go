package core

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"time"
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
		Id:         HashString(tir.InstID + tir.Ts),
		InstID:     tir.InstID,
		InstType:   tir.InstType,
		Last:       ToFloat64(tir.Last),
		VolCcy24h:  ToFloat64(tir.VolCcy24h),
		Ts:         ToInt64(tir.Ts),
		LastUpdate: time.Now(),
	}
	return ti
}

func ToString(val interface{}) string {
	valstr := ""
	if reflect.TypeOf(val).Name() == "string" {
		valstr = val.(string)
	} else if reflect.TypeOf(val).Name() == "float64" {
		valstr = fmt.Sprintf("%f", val)
	} else if reflect.TypeOf(val).Name() == "int64" {
		valstr = strconv.FormatInt(val.(int64), 16)
	} else if reflect.TypeOf(val).Name() == "int" {
		valstr = fmt.Sprintf("%d", val)
	}
	return valstr
}

func ToInt64(val interface{}) int64 {
	vali := int64(0)
	if reflect.TypeOf(val).Name() == "string" {
		vali, _ = strconv.ParseInt(val.(string), 10, 64)
	} else if reflect.TypeOf(val).Name() == "float64" {
		vali = int64(val.(float64))
	}
	return vali
}

func ToFloat64(val interface{}) float64 {
	valf := float64(0)
	if reflect.TypeOf(val).Name() == "string" {
		valf, _ = strconv.ParseFloat(val.(string), 64)
	} else if reflect.TypeOf(val).Name() == "float64" {
		valf = val.(float64)
	} else if reflect.TypeOf(val).Name() == "int64" {
		valf = float64(val.(int64))
	}
	return valf
}

// TODO 有待实现
func (ti *TickerInfo) SetToKey(cr *Core) error {
	js, _ := json.Marshal(*ti)
	plateName := ti.InstID + "|tickerInfo"
	_, err := cr.RedisLocalCli.Set(plateName, string(js), 0).Result()
	return err
}
