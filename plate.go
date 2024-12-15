package core

import (
	"encoding/json"
	"time"
)

type Plate struct {
	InstID     string             `json:"instId,string"`
	Scale      float64            `json:"scale,number"`
	Count      int                `json:"count,number"`
	CoasterMap map[string]Coaster `json:"coasterMap"`
}

func (pl *Plate) Init(instId string) {
	pl.InstID = instId
	pl.Count = 24
	pl.Scale = float64(0.005)
	pl.CoasterMap = make(map[string]Coaster)
}

// TODO 从redis里读出来已经存储的plate，如果不存在就创建一个新的
func LoadPlate(cr *Core, instId string) (*Plate, error) {
	pl := Plate{}
	plateName := instId + "|plate"
	_, err := cr.RedisLocalCli.Exists().Result()
	if err == nil {
		str, _ := cr.RedisLocalCli.Get(plateName).Result()
		json.Unmarshal([]byte(str), &pl)
	} else {
		pl.Init(instId)
		prs := cr.Cfg.Config.Get("candleDimentions").MustArray()
		for _, v := range prs {
			pl.MakeCoaster(cr, v.(string))
		}
	}
	return &pl, nil
}

func (pl *Plate) SetToKey(cr *Core) error {
	js, _ := json.Marshal(*pl)
	plateName := pl.InstID + "|plate"
	_, err := cr.RedisLocalCli.Set(plateName, string(js), 0).Result()
	return err
}

func (pl *Plate) MakeCoaster(cr *Core, period string) (*Coaster, error) {
	lastTime := time.Now()
	setName := "candle" + period + "|" + pl.InstID + "|sortedSet"
	cdl, err := cr.GetRangeCandleSortedSet(setName, pl.Count, lastTime)
	if err != nil {
		return nil, err
	}
	cdl.RecursiveBubbleS(len(cdl.List), "asc")
	setName7 := "ma7|" + setName
	setName30 := "ma30|" + setName
	mxl7, err := cr.GetRangeMaXSortedSet(setName7, pl.Count, lastTime)
	if err != nil {
		return nil, err
	}
	mxl7.RecursiveBubbleS(len(mxl7.List), "asc")
	mxl30, err := cr.GetRangeMaXSortedSet(setName30, pl.Count, lastTime)
	if err != nil {
		return nil, err
	}
	mxl30.RecursiveBubbleS(len(mxl30.List), "asc")
	coaster := Coaster{
		InstID:     pl.InstID,
		Period:     period,
		Count:      pl.Count,
		Scale:      pl.Scale,
		CandleList: *cdl,
		Ma7List:    *mxl7,
		Ma30List:   *mxl30,
	}
	pl.CoasterMap["period"+period] = coaster
	return &coaster, err
}
