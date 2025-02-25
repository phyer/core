package core

import (
	"crypto/sha256"
	"encoding/hex"
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
	"github.com/phyer/texus/utils"
	logrus "github.com/sirupsen/logrus"
)

type Candle struct {
	Id         string `json:"_id"`
	core       *Core
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
	SetToKey(cr *Core) ([]interface{}, error)
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

func (cd *Candle) Filter(cr *Core) bool {
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

func (core *Core) GetCandlesWithRest(instId string, kidx int, dura time.Duration, maxCandles int) error {
	ary := []string{}

	wsary := core.Cfg.CandleDimentions
	for k, v := range wsary {
		matched := false
		// 这个算法的目的是：越靠后的candles维度，被命中的概率越低，第一个百分之百命中，后面开始越来越低, 每分钟都会发生这样的计算，
		// 因为维度多了的话，照顾不过来
		rand.New(rand.NewSource(time.Now().UnixNano()))
		rand.Seed(time.Now().UnixNano())
		n := (k*2 + 2) * 3
		if n < 1 {
			n = 1
		}
		b := rand.Intn(n)
		if b < 8 {
			matched = true
		}
		if matched {
			ary = append(ary, v)
		}
	}

	mdura := dura/(time.Duration(len(ary)+1)) - 50*time.Millisecond
	// fmt.Println("loop4 Ticker Start instId, dura: ", instId, dura, dura/10, mdura, len(ary), " idx: ", kidx)
	// time.Duration(len(ary)+1)
	ticker := time.NewTicker(mdura)
	done := make(chan bool)
	idx := 0
	go func(i int) {
		for {
			select {
			case <-ticker.C:
				if i >= (len(ary)) {
					done <- true
					break
				}
				rand.Seed(time.Now().UnixNano())
				b := rand.Intn(2)
				maxCandles = maxCandles * (i + b) * 2

				if maxCandles < 3 {
					maxCandles = 3
				}
				if maxCandles > 30 {
					maxCandles = 30
				}
				mx := strconv.Itoa(maxCandles)
				// fmt.Println("loop4 getCandlesWithRest, instId, period,limit,dura, t: ", instId, ary[i], mx, mdura)
				go func(ii int) {
					restQ := RestQueue{
						InstId:   instId,
						Bar:      ary[ii],
						Limit:    mx,
						Duration: mdura,
						WithWs:   true,
					}
					js, _ := json.Marshal(restQ)
					core.RedisLocalCli.LPush("restQueue", js)
				}(i)
				i++
			}
		}
	}(idx)
	time.Sleep(dura - 10*time.Millisecond)
	ticker.Stop()
	// fmt.Println("loop4 Ticker stopped instId, dura: ", instId, dura, mdura)
	done <- true
	return nil
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

func (core *Core) SaveCandle(instId string, period string, rsp *CandleData, dura time.Duration, withWs bool) {
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

func (candle *Candle) PushToWriteLogChan(cr *Core) error {
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
func HashString(input string) string {
	// 计算SHA-256哈希值
	hash := sha256.Sum256([]byte(input))
	// 转换为十六进制字符串
	hashHex := hex.EncodeToString(hash[:])
	// 返回前20位
	return hashHex[:23]
}

func (cl *Candle) ToStruct(core *Core) (*Candle, error) {
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

// 保证同一个 period, keyName ，在一个周期里，SaveToSortSet只会被执行一次
func (core *Core) SaveUniKey(period string, keyName string, extt time.Duration, tsi int64) {

	refName := keyName + "|refer"
	// refRes, _ := core.RedisLocalCli.GetSet(refName, 1).Result()
	core.RedisLocalCli.Expire(refName, extt)
	// 为保证唯一性机制，防止SaveToSortSet 被重复执行, ps: 不需要唯一，此操作幂等在redis里
	// founded, _ := core.findInSortSet(period, keyName, extt, tsi)
	// if len(refRes) != 0 {
	// 	logrus.Error("refName exist: ", refName)
	// 	return
	// }
	core.SaveToSortSet(period, keyName, extt, tsi)
}

func (core *Core) findInSortSet(period string, keyName string, extt time.Duration, tsi int64) (bool, error) {
	founded := false
	ary := strings.Split(keyName, "ts:")
	setName := ary[0] + "sortedSet"
	opt := redis.ZRangeBy{
		Min: ToString(tsi),
		Max: ToString(tsi),
	}
	rs, err := core.RedisLocalCli.ZRangeByScore(setName, opt).Result()
	if len(rs) > 0 {
		founded = true
	}
	if err != nil {
		logrus.Error("err of ma7|ma30 add to redis:", err)
	} else {
		logrus.Info("sortedSet added to redis:", rs, keyName)
	}
	return founded, nil
}

// tsi: 上报时间timeStamp millinSecond
func (core *Core) SaveToSortSet(period string, keyName string, extt time.Duration, tsi int64) {
	ary := strings.Split(keyName, "ts:")
	setName := ary[0] + "sortedSet"
	z := redis.Z{
		Score:  float64(tsi),
		Member: keyName,
	}
	rs, err := core.RedisLocalCli.ZAdd(setName, z).Result()
	if err != nil {
		logrus.Warn("err of ma7|ma30 add to redis:", err)
	} else {
		logrus.Warn("sortedSet added to redis:", rs, keyName)
	}
}

// 根据周期的文本内容，返回这代表多少个分钟
func (cr *Core) PeriodToMinutes(period string) (int64, error) {
	ary := strings.Split(period, "")
	beiStr := "1"
	danwei := ""
	if len(ary) == 0 {
		err := errors.New(utils.GetFuncName() + " period is block")
		return 0, err
	}
	if len(ary) == 3 {
		beiStr = ary[0] + ary[1]
		danwei = ary[2]
	} else {
		beiStr = ary[0]
		danwei = ary[1]
	}
	cheng := 1
	bei, _ := strconv.Atoi(beiStr)
	switch danwei {
	case "m":
		{
			cheng = bei
			break
		}
	case "H":
		{
			cheng = bei * 60
			break
		}
	case "D":
		{
			cheng = bei * 60 * 24
			break
		}
	case "W":
		{
			cheng = bei * 60 * 24 * 7
			break
		}
	case "M":
		{
			cheng = bei * 60 * 24 * 30
			break
		}
	case "Y":
		{
			cheng = bei * 60 * 24 * 365
			break
		}
	default:
		{
			logrus.Warning("notmatch:", danwei, period)
			panic("notmatch:" + period)
		}
	}
	return int64(cheng), nil
}

// type ScanCmd struct {
// baseCmd
//
// page   []string
// cursor uint64
//
// process func(cmd Cmder) error
// }
func (core *Core) GetRangeKeyList(pattern string, from time.Time) ([]*simple.Json, error) {
	// 比如，用来计算ma30或ma7，倒推多少时间范围，
	redisCli := core.RedisLocalCli
	cursor := uint64(0)
	n := 0
	allTs := []int64{}
	var keys []string
	for {
		var err error
		keys, cursor, _ = redisCli.Scan(cursor, pattern+"*", 2000).Result()
		if err != nil {
			panic(err)
		}
		n += len(keys)
		if n == 0 {
			break
		}
	}

	// keys, _ := redisCli.Keys(pattern + "*").Result()
	for _, key := range keys {
		keyAry := strings.Split(key, ":")
		key = keyAry[1]
		keyi64, _ := strconv.ParseInt(key, 10, 64)
		allTs = append(allTs, keyi64)
	}
	nary := utils.RecursiveBubble(allTs, len(allTs))
	tt := from.UnixMilli()
	ff := tt - tt%60000
	fi := int64(ff)
	mary := []int64{}
	for _, v := range nary {
		if v < fi {
			break
		}
		mary = append(mary, v)
	}
	res := []*simple.Json{}
	for _, v := range mary {
		// if k > 1 {
		// break
		// }
		nv := pattern + strconv.FormatInt(v, 10)
		str, err := redisCli.Get(nv).Result()
		if err != nil {
			logrus.Error("err of redis get key:", nv, err)
		}
		cur, err := simple.NewJson([]byte(str))
		if err != nil {
			logrus.Error("err of create newJson:", str, err)
		}
		res = append(res, cur)
	}

	return res, nil
}

func (cl *Candle) SetToKey(core *Core) ([]interface{}, error) {
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
