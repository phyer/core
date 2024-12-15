package core

import (
	"encoding/json"
	"errors"
	"fmt"
	logrus "github.com/sirupsen/logrus"
	"os"
	"strconv"
	"time"
)

// TODO 目前没有实现tickerInfo，用一分钟维度的candle代替, 后续如果订阅ws的话，ticker就不用了，也还是直接用candle1m就够了
type Coaster struct {
	InstID         string     `json:"instID"`
	Period         string     `json:"period"`
	Count          int        `json:"count"`
	Scale          float64    `json:"scale"`
	LastUpdateTime int64      `json:"lastUpdateTime"`
	UpdateNickName string     `json:"updateNickName"`
	CandleList     CandleList `json:"candleList"`
	Ma7List        MaXList    `json:"ma7List"`
	Ma30List       MaXList    `json:"ma30List"`
}

type CoasterInfo struct {
	InstID      string
	Period      string
	InsertedNew bool
}

func (co Coaster) RPushSample(cr *Core, sp Sample, ctype string) (*Sample, error) {
	cd := Candle{}
	spjs, _ := json.Marshal(sp)
	logrus.Debug("RPushSample spjs: ", string(spjs))
	if ctype == "candle" {
		json.Unmarshal(spjs, &cd)
		cd.Data[0] = cd.Data[0]
		cd.Data[1], _ = strconv.ParseFloat(cd.Data[1].(string), 64)
		cd.Data[2], _ = strconv.ParseFloat(cd.Data[2].(string), 64)
		cd.Data[3], _ = strconv.ParseFloat(cd.Data[3].(string), 64)
		cd.Data[4], _ = strconv.ParseFloat(cd.Data[4].(string), 64)
		cd.Data[5], _ = strconv.ParseFloat(cd.Data[5].(string), 64)
		cd.Data[6], _ = strconv.ParseFloat(cd.Data[6].(string), 64)
		sm, err := co.CandleList.RPush(&cd)
		if err == nil {
			now := time.Now().UnixMilli()
			co.LastUpdateTime = now
			co.CandleList.LastUpdateTime = now
			co.UpdateNickName = GetRandomString(12)
			co.CandleList.UpdateNickName = GetRandomString(12)
		}
		return &sm, err
	}
	mx := MaX{}
	if ctype == "ma7" {
		json.Unmarshal(spjs, &mx)
		sm, err := co.Ma7List.RPush(&mx)
		if err == nil {
			now := time.Now().UnixMilli()
			co.LastUpdateTime = now
			co.Ma7List.UpdateNickName = GetRandomString(12)
			co.Ma7List.LastUpdateTime = now
		}
		return &sm, err
	}
	if ctype == "ma30" {
		json.Unmarshal(spjs, &mx)
		sm, err := co.Ma30List.RPush(&mx)
		// bj, _ := json.Marshal(co)
		if err == nil {
			now := time.Now().UnixMilli()
			co.LastUpdateTime = now
			co.Ma30List.UpdateNickName = GetRandomString(12)
			co.Ma30List.LastUpdateTime = now
		}
		return &sm, err
	}
	return nil, nil
}

func (co *Coaster) SetToKey(cr *Core) (string, error) {
	co.CandleList.RecursiveBubbleS(len(co.CandleList.List), "asc")
	co.Ma7List.RecursiveBubbleS(len(co.Ma7List.List), "asc")
	co.Ma30List.RecursiveBubbleS(len(co.Ma30List.List), "asc")
	js, _ := json.Marshal(*co)
	coasterName := co.InstID + "|" + co.Period + "|coaster"
	res, err := cr.RedisLocalCli.Set(coasterName, string(js), 0).Result()
	return res, err
}

// func (coi *CoasterInfo) Process(cr *Core) {
// 	curCo, _ := cr.GetCoasterFromPlate(coi.InstID, coi.Period)
// 	go func(co Coaster) {
// 		//这里执行：创建一个tray对象,用现有的co的数据计算和填充其listMap
// 		// TODO 发到一个channel里来执行下面的任务，
// 		allow := os.Getenv("SARDINE_MAKESERIES") == "true"
// 		if !allow {
// 			return
// 		}
// 		srs, err := co.UpdateTray(cr)
// 		if err != nil || srs == nil {
// 			logrus.Warn("tray err: ", err)
// 			return
// 		}
// 		_, err = srs.SetToKey(cr)
// 		if err != nil {
// 			logrus.Warn("srs SetToKey err: ", err)
// 			return
// 		}
// 		//实例化完一个tray之后，拿着这个tray去执行Analytics方法
// 		//
// 		// srsinfo := SeriesInfo{
// 		// 	InstID: curCo.InstID,
// 		// 	Period: curCo.Period,
// 		// }
// 		//
// 		// cr.SeriesChan <- &srsinfo
// 	}(curCo)
//
// 	go func(co Coaster) {
// 		// 每3次会有一次触发缓存落盘
// 		// run := utils.Shaizi(3)
// 		// if run {
// 		_, err := co.SetToKey(cr)
// 		if err != nil {
// 			logrus.Warn("coaster process err: ", err)
// 			fmt.Println("coaster SetToKey err: ", err)
// 		}
// 		// }
//
// 	}(curCo)
// }
//
// TODO 类似于InsertIntoPlate函数，照猫画虎就行了
//
//	func (co *Coaster) UpdateTray(cr *Core) (*Series, error) {
//		cr.Mu1.Lock()
//		defer cr.Mu1.Unlock()
//		//尝试从内存读取tray对象
//		tr, trayFounded := cr.TrayMap[co.InstID]
//		if !trayFounded {
//			tr1, err := co.LoadTray(cr)
//			if err != nil {
//				return nil, err
//			}
//			cr.TrayMap[co.InstID] = tr1
//			tr = tr1
//		}
//		srs, seriesFounded := tr.SeriesMap["period"+co.Period]
//		err := errors.New("")
//		if !seriesFounded {
//			srs1, err := tr.NewSeries(cr, co.Period)
//			if err != nil {
//				return nil, err
//			}
//			tr.SeriesMap["period"+co.Period] = srs1
//		} else {
//			err = srs.Refresh(cr)
//		}
//		// if err == nil {
//		// bj, _ := json.Marshal(srs)
//		// logrus.Debug("series:,string"(bj))
//		// }
//		return srs, err
//	}
//
// TODO
// func (co *Coaster) LoadTray(cr *Core) (*Tray, error) {
// 	tray := Tray{}
// 	tray.Init(co.InstID)
// 	prs := cr.Cfg.Config.Get("candleDimentions").MustArray()
// 	for _, v := range prs {
// 		tray.NewSeries(cr, v.(string))
// 	}
// 	return &tray, nil
// }
