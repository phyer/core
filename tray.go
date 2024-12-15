package core

import (
	"encoding/json"
)

type Tray struct {
	InstID         string  `json:"instId,string"`
	Period         string  `json:"period,string"`
	Count          int     `json:"count,number"`
	Scale          float64 `json:"scale,number"`
	LastUpdateTime int64   `json:"lastUpdateTime,number"`
	// SeriesMap      map[string]*Series `json:"seriesMap"`
}

type PixelSeries struct {
	Count   int64    `json:"count"`
	Section int64    `json:"section"`
	List    []*Pixel `json:"list"`
}

func (tr *Tray) Init(instId string) {
	tr.InstID = instId
	tr.Count = 24
	tr.Scale = float64(0.005)
	// tr.SeriesMap = make(map[string]*Series)
}
func (tr *Tray) SetToKey(cr *Core) error {
	js, _ := json.Marshal(tr)
	keyName := tr.InstID + "|" + tr.Period + "|tray"
	_, err := cr.RedisLocalCli.Set(keyName, string(js), 0).Result()
	// fmt.Println(utils.GetFuncName(), "tray SetToKey:", string(js))
	return err
}

// TODO 执行单维度分析，相对应的是跨维度的分析,那个还没想好
// 单维度下的分析结果中包含以下信息：
// 1.
func (tr *Tray) Analytics(cr *Core) {
	go func() {

	}()
}

// TODO 实例化一个series
// func (tr *Tray) NewSeries(cr *Core, period string) (*Series, error) {
// 	sr := Series{
// 		InstID:       tr.InstID,
// 		Period:       period,
// 		Count:        tr.Count,
// 		Scale:        tr.Scale,
// 		CandleSeries: &PixelList{},
// 		Ma7Series:    &PixelList{},
// 		Ma30Series:   &PixelList{},
// 	}
// 	// 自我更新
// 	err := sr.Refresh(cr)
// 	tr.SeriesMap["period"+period] = &sr
// 	return &sr, err
// }
