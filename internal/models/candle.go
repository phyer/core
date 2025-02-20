package models

import (
	"encoding/json"
	"errors"
	// "reflect"
	// "fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	simple "github.com/bitly/go-simplejson"
	"github.com/go-redis/redis"
	logrus "github.com/sirupsen/logrus"

	"github.com/phyer/core/internal/core" // 新增
)

type Candle struct {
	Id         string `json:"_id"`
	core       *core.Core
	InstID     string `json:"instID"`
	Period     string `json:"period"`
	Data       []interface{}
	From       string    `json:"from"`
	Timestamp  time.Time `json:"timeStamp"`
	LastUpdate time.Time `json:"lastUpdate"`
	Open       float64   `json:"open"`
	High       float64   `json:"high"`
	Low        float64   `json:"low"`
	Close      float64   `json:"close"`
	VolCcy     float64   `json:"volCcy"`
	Confirm    bool      `json:"confirm"`
}
type Sample interface {
	SetToKey(cr *core.Core) ([]interface{}, error)
}

type SampleList interface {
	// 从左边插入一个元素，把超过长度的元素顶出去
	RPush(sp Sample) (Sample, error)
	// 得到一个切片，end一般是0，代表末尾元素，start是负值，-3代表倒数第三个
	// start：-10, end: -3 代表从倒数第10个到倒数第三个之间的元素组成的切片。
	GetSectionOf(start int, end int) ([]*Sample, error)
}
type CandleList struct {
	Count          int       `json:"count,number"`
	LastUpdateTime int64     `json:"lastUpdateTime"`
	UpdateNickName string    `json:"updateNickName"`
	List           []*Candle `json:"list"`
}
type CandleSegment struct {
	StartTime string `json:"startTime"`
	Enabled   bool   `json:"enabled,bool"`
	Seg       string `json:"seg"`
}
type MatchCheck struct {
	Minutes int64
	Matched bool
}

func (cd *Candle) Filter(cr *core.Core) bool {
	myFocusList := cr.Cfg.Config.Get("focusList").MustArray()
	founded := false
	for _, v := range myFocusList {
		if v.(string) == cd.InstID {
			founded = true
			break
		}
	}
	return founded
}
func (mc *MatchCheck) SetMatched(value bool) {
	mc.Matched = value
}

// 当前的时间毫秒数 对于某个时间段，比如3分钟，10分钟，是否可以被整除，
func IsModOf(curInt int64, duration time.Duration) bool {
	vol := int64(0)
	if duration < 24*time.Hour {
		// 小于1天
		vol = (curInt + 28800000)
	} else if duration >= 24*time.Hour && duration < 48*time.Hour {
		// 1天
		vol = curInt - 1633881600000
	} else if duration >= 48*time.Hour && duration < 72*time.Hour {
		// 2天
		vol = curInt - 1633795200000
	} else if duration >= 72*time.Hour && duration < 120*time.Hour {
		// 3天
		vol = curInt - 1633708800000
	} else if duration >= 120*time.Hour {
		// 5天
		vol = curInt - 1633795200000
	} else {
		// fmt.Println("noMatched:", curInt)
	}

	mody := vol % duration.Milliseconds()
	if mody == 0 {
		return true
	}
	return false
}

func (core *core.Core) SaveCandle(instId string, period string, rsp *CandleData, dura time.Duration, withWs bool) {
	leng := len(rsp.Data)
	// fmt.Println("saveCandle leng: ", leng, " instId: ", instId, " period: ", period)
	logrus.Info("saveCandles leng: ", leng, " instId: ", instId, " period: ", period, " length of rsp.Data: ", len(rsp.Data))
	// softCandleSegmentList
	segments := core.Cfg.Config.Get("softCandleSegmentList").MustArray()
	logrus.Warn("lensof segments:", len(segments))
	curSegStartTime := ""
	for k, v := range segments {
		logrus.Warn("fetch segments:", k, v)
		cs := CandleSegment{}
		sv, _ := json.Marshal(v)
		json.Unmarshal(sv, &cs)
		if !cs.Enabled {
			continue
		}
		logrus.Warn("fetch segments2: cs.Seg: ", cs.Seg, ", period:", period, ", cs.Seg == period: ", (cs.Seg == period))

		if cs.Seg == period {
			curSegStartTime = cs.StartTime
			break
		}
	}
	logrus.Warn("curSegStartTime:", curSegStartTime)
	curTm, _ := time.ParseInLocation("2006-01-02 15:04.000", curSegStartTime, time.Local)
	curTmi := curTm.UnixMilli()

	for k, v := range rsp.Data {
		tmi := ToInt64(v[0])
		last := ToFloat64(v[4])
		// ty := reflect.TypeOf(v[4]).Name()
		// v4It, err := strconv.ParseInt(v[4].(string), 10, 64)
		// if err != nil {
		// 	logrus.Info("saveCandles last is 0 is err: ", err, "v4It: ", v4It, "v[4]: ", v[4], "v[4] type: ", ty, "leng: ", leng, " instId: ", instId, " period: ", period, " length of rsp.Data: ", len(rsp.Data), " data:", rsp.Data)
		// }
		if last == 0 {
			// logrus.Info("saveCandles last is 0: ", "v[4]: ", v[4], "v[4] type: ", ty, " v4It: ", v4It, "leng: ", leng, " instId: ", instId, " period: ", period, " length of rsp.Data: ", len(rsp.Data), " data:", rsp.Data)
			continue
		}
		minutes, _ := core.PeriodToMinutes(period)
		logrus.Warn("tmi: ", tmi, " curTim:", curTmi, " minutes: ", minutes)
		if (tmi-curTmi)%(minutes*60000) != 0 {
			logrus.Warn("saveCandles error: 当前记录中的时间戳：", curSegStartTime, "，并非周期节点：", period, " 忽略")
			continue
		}

		ts, _ := Int64ToTime(tmi)
		candle := Candle{
			InstID:     instId,
			Period:     period,
			Data:       v,
			From:       "rest",
			Timestamp:  ts,
			LastUpdate: time.Now(),
		}

		//存到elasticSearch
		candle.PushToWriteLogChan(core)
		//保存rest得到的candle
		// 发布到allCandles|publish, 给外部订阅者用于setToKey
		arys := []string{ALLCANDLES_PUBLISH}
		if withWs {
			arys = append(arys, ALLCANDLES_INNER_PUBLISH)
			time.Sleep(time.Duration(k*40) * time.Millisecond)
		}
		// 如果candle都不需要存到redis，那么AddToGeneralCandleChnl也没有意义
		saveCandle := os.Getenv("TEXUS_SAVECANDLE")
		logrus.Info("saveCandles datas: k,v: ", k, v)
		if saveCandle == "true" {
			go func(k int) {
				time.Sleep(time.Duration(k*100) * time.Millisecond)
				candle.SetToKey(core)
				core.AddToGeneralCandleChnl(&candle, arys)
			}(k)
		}
	}
}

func (candle *Candle) PushToWriteLogChan(cr *core.Core) error {
	did := candle.InstID + candle.Period + candle.Data[0].(string)
	candle.Id = HashString(did)
	cl, _ := candle.ToStruct(cr)
	logrus.Debug("cl: ", cl)
	cd, err := json.Marshal(cl)
	if err != nil {
		logrus.Error("PushToWriteLog json marshal candle err: ", err)
	}
	candle = cl
	wg := WriteLog{
		Content: cd,
		Tag:     "sardine.log.candle." + candle.Period,
		Id:      candle.Id,
	}
	cr.WriteLogChan <- &wg
	return nil
}

func Daoxu(arr []interface{}) {
	var temp interface{}
	length := len(arr)
	for i := 0; i < length/2; i++ {
		temp = arr[i]
		arr[i] = arr[length-1-i]
		arr[length-1-i] = temp
	}
}
func (cl *Candle) ToStruct(core *core.Core) (*Candle, error) {
	// cl.Timestamp
	// 将字符串转换为 int64 类型的时间戳
	ts, err := strconv.ParseInt(cl.Data[0].(string), 10, 64)
	if err != nil {
		logrus.Error("Error parsing timestamp:", err)
		return nil, err
	}
	cl.Timestamp = time.Unix(ts/1000, (ts%1000)*1000000) // 纳秒级别
	op, err := strconv.ParseFloat(cl.Data[1].(string), 64)
	if err != nil {
		logrus.Error("Error parsing string to float64:", err)
		return nil, err
	}
	cl.Open = op
	hi, err := strconv.ParseFloat(cl.Data[2].(string), 64)
	if err != nil {
		logrus.Error("Error parsing string to float64:", err)
		return nil, err
	}
	cl.High = hi
	lo, err := strconv.ParseFloat(cl.Data[3].(string), 64)
	if err != nil {
		logrus.Error("Error parsing string to float64:", err)
		return nil, err
	}
	cl.Low = lo
	clse, err := strconv.ParseFloat(cl.Data[4].(string), 64)

	if err != nil {
		logrus.Error("Error parsing string to float64:", err)
		return nil, err
	}
	cl.Close = clse
	cl.VolCcy, err = strconv.ParseFloat(cl.Data[6].(string), 64)
	if err != nil {
		logrus.Error("Error parsing string to float64:", err)
		return nil, err
	}
	if cl.Data[6].(string) == "1" {
		cl.Confirm = true
	} else {
		cl.Confirm = false
	}
	return cl, nil
}

func (cl *Candle) SetToKey(core *core.Core) ([]interface{}, error) {
	data := cl.Data
	tsi, err := strconv.ParseInt(data[0].(string), 10, 64)

	tss := strconv.FormatInt(tsi, 10)
	tm, _ := Int64ToTime(tsi)

	keyName := "candle" + cl.Period + "|" + cl.InstID + "|ts:" + tss
	//过期时间：根号(当前candle的周期/1分钟)*10000
	cl.LastUpdate = time.Now()
	cl.Timestamp = tm
	dt, err := json.Marshal(cl)
	if err != nil {
		logrus.Error("candle Save to String err:", err)
	}
	logrus.Info("candle Save to String: ", string(dt))
	exp, err := core.PeriodToMinutes(cl.Period)
	if err != nil {
		logrus.Error("err of PeriodToMinutes:", err)
	}
	// expf := float64(exp) * 60
	length, _ := core.Cfg.Config.Get("sortedSet").Get("length").String()
	ln := ToFloat64(length)
	expf := float64(exp) * ln
	extt := time.Duration(expf) * time.Minute
	curVolstr, _ := data[5].(string)
	curVol, err := strconv.ParseFloat(curVolstr, 64)
	if err != nil {
		logrus.Error("err of convert ts:", err)
	}
	curVolCcystr, _ := data[6].(string)
	curVolCcy, err := strconv.ParseFloat(curVolCcystr, 64)
	curPrice := curVolCcy / curVol
	if curPrice <= 0 {
		logrus.Error("price有问题", curPrice, "dt: ", string(dt), "from:", cl.From)
		err = errors.New("price有问题")
		return cl.Data, err
	}
	redisCli := core.RedisLocalCli
	// tm := time.UnixMilli(tsi).Format("2006-01-02 15:04")
	logrus.Info("setToKey:", keyName, "ts: ", "price: ", curPrice, "from:", cl.From)
	redisCli.Set(keyName, dt, extt).Result()
	core.SaveUniKey(cl.Period, keyName, extt, tsi)
	return cl.Data, err
}

// 冒泡排序
func (cdl *CandleList) RecursiveBubbleS(length int, ctype string) error {
	if length == 0 {
		return nil
	}

	for idx, _ := range cdl.List {
		if idx >= length-1 {
			break
		}
		temp := Candle{}
		pre := ToInt64(cdl.List[idx].Data[0])
		nex := ToInt64(cdl.List[idx+1].Data[0])
		daoxu := pre < nex
		if ctype == "asc" {
			daoxu = !daoxu
		}
		if daoxu { //改变成>,换成从小到大排序
			temp = *cdl.List[idx]
			cdl.List[idx] = cdl.List[idx+1]
			cdl.List[idx+1] = &temp
		}
	}
	length--
	cdl.RecursiveBubbleS(length, ctype)
	return nil
}

// TODO 返回的Sample是被弹出队列的元素，如果没有就是nil
func (cdl *CandleList) RPush(sp *Candle) (Sample, error) {
	last := Candle{}
	tsi := ToInt64(sp.Data[0])
	matched := false
	// bj, _ := json.Marshal(*sp)
	cdl.RecursiveBubbleS(len(cdl.List), "asc")
	for k, v := range cdl.List {
		if ToInt64(v.Data[0]) == tsi {
			matched = true
			cdl.List[k] = sp
			bj, err := json.Marshal(sp)
			if err != nil {
				logrus.Warning("err of convert cdl item:", err)
			}
			logrus.Debug("candleList RPush replace: ", string(bj), "v.Data[0]: ", v.Data[0], "tsi:", tsi)
		}
	}
	if matched {
		return nil, nil
	}
	if len(cdl.List) >= cdl.Count {
		last = *cdl.List[0]
		cdl.List = cdl.List[1:]
		cdl.List = append(cdl.List, sp)
		bj, err := json.Marshal(sp)
		logrus.Debug("candleList RPush popup: ", string(bj), "len(cdl.List): ", len(cdl.List), "cdl.Count:", cdl.Count)
		return &last, err
	} else {
		cdl.List = append(cdl.List, sp)
		bj, err := json.Marshal(sp)
		logrus.Debug("candleList RPush insert: ", string(bj), "len(cdl.List): ", len(cdl.List), "cdl.Count:", cdl.Count)
		return nil, err
	}
}

// TODO pixel
func (cdl *CandleList) MakePixelList(cr *core.Core, mx *MaX, score float64) (*PixelList, error) {
	if mx.Data[2] != float64(30) {
		err := errors.New("ma30 原始数据不足30条")
		return nil, err
	}
	pxl := PixelList{
		Count:          cdl.Count,
		UpdateNickName: cdl.UpdateNickName,
		LastUpdateTime: cdl.LastUpdateTime,
		List:           []*Pixel{},
	}
	realLens := len(cdl.List)
	cha := cdl.Count - realLens
	for i := 0; i < 24; i++ {
		pix := Pixel{}
		pxl.List = append(pxl.List, &pix)
	}
	ma30Val := ToFloat64(mx.Data[1])
	for h := cdl.Count - 1; h-cha >= 0; h-- {
		// Count 是希望值,比如24，realLens是实际值, 如果希望值和实际值相等，cha就是0
		cdLast := cdl.List[h-cha].Data[4]
		cdLastf := ToFloat64(cdLast)
		cdOpen := cdl.List[h-cha].Data[1]
		cdOpenf := ToFloat64(cdOpen)
		cdHigh := cdl.List[h-cha].Data[2]
		cdHighf := ToFloat64(cdHigh)
		cdLow := cdl.List[h-cha].Data[3]
		cdLowf := ToFloat64(cdLow)

		yCandle := YCandle{
			Open:  (cdOpenf - ma30Val) / ma30Val / score,
			High:  (cdHighf - ma30Val) / ma30Val / score,
			Low:   (cdLowf - ma30Val) / ma30Val / score,
			Close: (cdLastf - ma30Val) / ma30Val / score,
		}
		tmi := ToInt64(cdl.List[h-cha].Data[0])
		pxl.List[h].Y = (cdLastf - ma30Val) / ma30Val / score
		pxl.List[h].X = float64(h)
		pxl.List[h].YCandle = yCandle
		pxl.List[h].Score = cdLastf
		pxl.List[h].TimeStamp = tmi
	}

	return &pxl, nil
}
func (cr *core.Core) AddToGeneralSeriesInfoChnl(sr *SeriesInfo) {
	redisCli := cr.RedisLocalCli
	ab, _ := json.Marshal(sr)
	if len(string(ab)) == 0 {
		return
	}
	_, err := redisCli.Publish(ALLSERIESINFO_PUBLISH, string(ab)).Result()
	if err != nil {
		logrus.Debug("err of seriesinfo add to redis2:", err, sr.InstID, sr.Period)
	}
}
