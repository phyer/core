package core

import (
	"encoding/json"
	"errors"
	"fmt"
	logrus "github.com/sirupsen/logrus"
	// "os"
	"strconv"
	"time"
)

type MaXList struct {
	Count          int    `json:"count"`
	LastUpdateTime int64  `json:"lastUpdateTime"`
	UpdateNickName string `json:"updateNickName"`
	List           []*MaX `json:"list"`
}

type MaX struct {
	Id         string        `json:"_id"`
	InstID     string        `json:"instID"`
	Period     string        `json:"period"`
	Timestamp  time.Time     `json:"timeStamp"`
	LastUpdate time.Time     `json:"lastUpdate"`
	KeyName    string        `json:"keyName"`
	Data       []interface{} `json:"data"`
	Count      int           `json:"count,number"`
	Ts         int64         `json:"ts,number"`
	AvgVal     float64       `json:"avgVal,number"`
	From       string        `json:"from,string"`
}

type WillMX struct {
	KeyName string
	Count   int
}

func (mx MaX) SetToKey(cr *Core) ([]interface{}, error) {
	// fmt.Println(utils.GetFuncName(), " step1 ", mx.InstID, " ", mx.Period)
	// mx.Timestamp, _ = Int64ToTime(mx.Ts)
	cstr := strconv.Itoa(mx.Count)
	tss := strconv.FormatInt(mx.Ts, 10)
	//校验时间戳是否合法
	ntm, err := cr.PeriodToLastTime(mx.Period, time.UnixMilli(mx.Ts))
	if ntm.UnixMilli() != mx.Ts {
		logrus.Warn(fmt.Sprint(GetFuncName(), " candles时间戳有问题 ", " 应该: ", ntm, "实际:", mx.Ts))
		mx.Ts = ntm.UnixMilli()
	}
	keyName := "ma" + cstr + "|candle" + mx.Period + "|" + mx.InstID + "|ts:" + tss
	//过期时间：根号(当前candle的周期/1分钟)*10000

	dj, err := json.Marshal(mx)
	if err != nil {
		logrus.Error("maX SetToKey json marshal err: ", err)
	}
	extt, err := cr.GetExpiration(mx.Period)
	if err != nil {
		logrus.Error("max SetToKey err: ", err)
		return mx.Data, err
	}
	// fmt.Println(utils.GetFuncName(), " step2 ", mx.InstID, " ", mx.Period)
	// tm := time.UnixMilli(mx.Ts).Format("01-02 15:04")
	cli := cr.RedisLocalCli
	if len(string(dj)) == 0 {
		logrus.Error("mx data is block data: ", mx, string(dj))
		err := errors.New("data is block")
		return mx.Data, err
	}
	// fmt.Println(utils.GetFuncName(), " step3 ", mx.InstID, " ", mx.Period)
	_, err = cli.Set(keyName, dj, extt).Result()
	if err != nil {
		logrus.Error(GetFuncName(), " maXSetToKey err:", err)
		return mx.Data, err
	}
	// fmt.Println(utils.GetFuncName(), " step4 ", mx.InstID, " ", mx.Period)
	// fmt.Println("max setToKey: ", keyName, "res:", res, "data:", string(dj), "from: ", mx.From)
	cr.SaveUniKey(mx.Period, keyName, extt, mx.Ts)
	return mx.Data, err
}

func Int64ToTime(ts int64) (time.Time, error) {
	timestamp := int64(ts)
	// 将时间戳转换为 time.Time 类型，单位为秒
	t := time.Unix(timestamp/1000, (timestamp%1000)*int64(time.Millisecond))
	// 获取东八区（北京时间）的时区信息
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		logrus.Error("加载时区失败:", err)
		return t, err
	}
	// 将时间转换为东八区时间
	t = t.In(loc)
	return t, nil
}
func (mx *MaX) PushToWriteLogChan(cr *Core) error {
	s := strconv.FormatFloat(float64(mx.Ts), 'f', 0, 64)
	did := "maX" + ToString(mx.Count) + mx.InstID + "|" + mx.Period + "|" + s
	logrus.Debug("did of max:", did)
	hs := HashString(did)
	mx.Id = hs
	md, _ := json.Marshal(mx)
	wg := WriteLog{
		Content: md,
		Tag:     "sardine.log.maX." + mx.Period,
		Id:      hs,
	}
	cr.WriteLogChan <- &wg
	return nil
}

// TODO
// 返回：
// Sample：被顶出队列的元素
func (mxl *MaXList) RPush(sm *MaX) (Sample, error) {
	last := MaX{}
	bj, _ := json.Marshal(*sm)
	json.Unmarshal(bj, &sm)
	tsi := sm.Data[0]
	matched := false
	for k, v := range mxl.List {
		if v.Data[0] == tsi {
			matched = true
			mxl.List[k] = sm
		}
	}
	if matched {
		return nil, nil
	}
	if len(mxl.List) >= mxl.Count && len(mxl.List) > 1 {
		last = *mxl.List[0]
		mxl.List = mxl.List[1:]
		mxl.List = append(mxl.List, sm)
		return last, nil
	} else {
		mxl.List = append(mxl.List, sm)
		return nil, nil
	}
	return nil, nil
}

// 冒泡排序
func (mxl *MaXList) RecursiveBubbleS(length int, ctype string) error {
	if length == 0 {
		return nil
	}
	realLength := len(mxl.List)
	//FIXME：在对这个List进行排序时，List中途长度变了，就会报错：
	// Jan 17 02:40:39 miracle ubuntu[25239]: panic: runtime error: index out of range [23] with length 23
	for idx, _ := range mxl.List {
		if idx >= length-1 || idx > realLength-1 {
			break
		}
		temp := MaX{}

		pre, _ := mxl.List[idx].Data[0].(float64)
		nex, _ := mxl.List[idx+1].Data[0].(float64)
		daoxu := pre < nex
		if ctype == "asc" {
			daoxu = !daoxu
		}
		if daoxu { //改变成>,换成从小到大排序
			temp = *mxl.List[idx]
			mxl.List[idx] = mxl.List[idx+1]
			mxl.List[idx+1] = &temp
		}
	}
	length--
	mxl.RecursiveBubbleS(length, ctype)
	return nil
}
