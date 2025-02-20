package analysis

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/go-redis/redis"
	logrus "github.com/sirupsen/logrus"
)

type ShearItem struct {
	ShearForce        float64 // ma30-candle剪切力
	VerticalElevation float64 // 仰角, Interval范围内线段的仰角
	Ratio             float64 // 剪切力除以仰角的比值
	Score             float64 // 当前LastCandleY点本值
	PolarQuadrant     string  // shangxian，manyue，xiaxian,xinyue， 分别对应圆周的四个阶段。
	LastUpdate        int64
	LastUpdateTime    string
}
type ShearForceGrp struct {
	InstID          string
	LastUpdate      int64
	LastUpdateTime  string
	Ma30PeriodGroup map[string]ShearItem
	Ma7PeriodGroup  map[string]ShearItem
	From            string
}

// TODO 弃用
// func (seg *SegmentItem) MakeShearForceGrp(cr *Core) (*ShearForceGrp, error) {
// shg := ShearForceGrp{
// InstID:          seg.InstID,
// Ma30PeriodGroup: map[string]ShearItem{},
// Ma7PeriodGroup:  map[string]ShearItem{},
// }
// err := shg.ForceUpdate(cr)
// sf1 := float64(0)
// sf1 = seg.LastCandle.Y - seg.LastMa7.Y
// she := ShearItem{
// LastUpdate:        time.Now().UnixMilli(),
// VerticalElevation: seg.SubItemList[2].VerticalElevation,
// Ratio:             seg.LastCandle.Y / seg.SubItemList[2].VerticalElevation,
// Score:             seg.LastCandle.Score,
// PolarQuadrant:     seg.PolarQuadrant,
// }
// if seg.Ctype == "ma7" {
// she.ShearForce = seg.LastCandle.Y
// shg.Ma7PeriodGroup[seg.Period] = she
// }
// if seg.Ctype == "ma30" {
// she.ShearForce = sf1
// shg.Ma30PeriodGroup[seg.Period] = she
// }
// return &shg, err
// }

// TODO 弃用
// func (shg *ShearForceGrp) ForceUpdate(cr *Core) error {
// ctype := "ma7"
// hmName := shg.InstID + "|" + ctype + "|shearForceGrp"
// res, err := cr.RedisLocalCli.HGetAll(hmName).Result()
//
// for k, v := range res {
// si := ShearItem{}
// json.Unmarshal([]byte(v), &si)
// shg.Ma7PeriodGroup[k] = si
// }
//
// ctype = "ma30"
// hmName = shg.InstID + "|" + ctype + "|shearForceGrp"
// res, err = cr.RedisLocalCli.HGetAll(hmName).Result()
//
// for k, v := range res {
// si := ShearItem{}
// json.Unmarshal([]byte(v), &si)
// shg.Ma30PeriodGroup[k] = si
// }
// shg.SetToKey(cr)
// return err
// }
func (she *ShearForceGrp) Show(cr *Core) error {
	js, err := json.Marshal(she)
	logrus.Info(GetFuncName(), ": ", string(js))

	return err
}

// TODO 需要重构: 已经重构
// 对象数据库落盘
func (she *ShearForceGrp) SetToKey(cr *Core) error {
	keyName := she.InstID + "|shearForce"
	she.From = os.Getenv("HOSTNAME")
	she.LastUpdateTime = time.Now().Format("2006-01-02 15:04:05.000")
	js, err := json.Marshal(she)
	if err != nil {
		logrus.Panic(GetFuncName(), " err: ", err)
	} else {
		cr.RedisLocalCli.Set(keyName, string(js), 0).Result()
		cr.RedisLocal2Cli.Set(keyName, string(js), 0).Result()
	}
	return err
}

func (she *ShearForceGrp) maXPrd(cr *Core, ctype string) {
	// 先把对象克隆，防止在处理的过程中对象发生变更
	she2 := *she
	she3 := &she2
	// 查了一下，json marshal 有线程安全问题，需要用户自己加锁，先不用了
	// bj, _ := json.Marshal(she3)
	// bytes := []byte(bj)
	// var she4 ShearForceGrp
	// json.Unmarshal(bytes, she4)
	// 先声明map
	var grp map[string]ShearItem
	// 再使用make函数创建一个非nil的map，nil map不能赋值
	grp = make(map[string]ShearItem)
	if ctype == "ma7" {
		//fmt.Println("len of ma7 she.Ma7PeriodGroup: ", len(she3.Ma7PeriodGroup))
		bj, err := json.Marshal(she3.Ma7PeriodGroup)
		if err != nil {
			logrus.Panic(GetFuncName(), " err:", err)
		}
		json.Unmarshal(bj, &grp)
		//fmt.Println("len of ma30 she.Ma7PeriodGroup: ", len(she3.Ma7PeriodGroup))
	} else if ctype == "ma30" {
		bj, err := json.Marshal(she3.Ma30PeriodGroup)
		if err != nil {
			logrus.Panic(GetFuncName(), " err: ", err)
		}
		json.Unmarshal(bj, &grp)
	}
	for period, shearItem := range grp {
		setName := "shearForce|ratio|" + ctype + "|" + period + "|sortedSet"
		// TODO：这个key用于判定当前instID|maX|period|的ratio排名是否已经过期
		timelinessKey := "shearForce|ratio|" + she.InstID + "|" + ctype + "|" + period + "|lastUpdate"
		sei := SeriesInfo{
			InstID: she3.InstID,
			Period: period,
		}
		// 阈值先暂且设置为 -100
		// SHEARFORCE_VERTICAL_RATE
		threahold := float64(SHEARFORCE_VERTICAL_RATE)
		bj, _ := json.Marshal(sei)
		z := redis.Z{
			Score:  float64(shearItem.Ratio),
			Member: string(bj),
		}
		//无论超过阈值，还是低于阈值的负数，都是达标
		if shearItem.Ratio < -1*threahold {
			cr.RedisLocalCli.ZAdd(setName, z).Result()
			cr.RedisLocalCli.Set(timelinessKey, shearItem.LastUpdate, 3*time.Minute)
		} else if shearItem.Ratio > threahold {
			cr.RedisLocalCli.ZAdd(setName, z).Result()
			cr.RedisLocalCli.Set(timelinessKey, shearItem.LastUpdate, 3*time.Minute)
		} else {
			cr.RedisLocalCli.ZRem(setName, string(bj)).Result()
		}
	}
}

// 把所有引用调用都改成传值调用，试试，看能不能解决那个陈年bug
func (she *ShearForceGrp) AddToRatioSorted(cr *Core) error {
	she.maXPrd(cr, "ma7")
	she.maXPrd(cr, "ma30")
	return nil
}

// TODO 需要重构: 看了一下，不用重构
func (she *ShearForceGrp) MakeSnapShot(cr *Core) error {
	nw := time.Now()
	tm := nw.UnixMilli()
	tm = tm - tm%60000
	tms := strconv.FormatInt(tm, 10)
	js, err := json.Marshal(she)

	keyName1 := fmt.Sprint(she.InstID + "|shearForce|snapShot|ts:" + tms)
	keyName2 := fmt.Sprint(she.InstID + "|shearForce|snapShot|last")
	_, err = cr.RedisLocalCli.Set(keyName1, string(js), time.Duration(24)*time.Hour).Result()
	_, err = cr.RedisLocalCli.Set(keyName2, string(js), time.Duration(24)*time.Hour).Result()
	_, err = cr.RedisLocal2Cli.Set(keyName1, string(js), time.Duration(24)*time.Hour).Result()
	_, err = cr.RedisLocal2Cli.Set(keyName2, string(js), time.Duration(24)*time.Hour).Result()
	writeLog := os.Getenv("SARDINE_WRITELOG") == "true"
	if !writeLog {
		return err
	}
	wg := WriteLog{
		Content: js,
		Tag:     she.InstID + ".shearForce",
	}
	go func() {
		cr.WriteLogChan <- &wg
	}()
	return nil
}

func (sheGrp *ShearForceGrp) Refresh(cr *Core) error {
	segments := cr.Cfg.Config.Get("softCandleSegmentList").MustArray()
	ma7Grp := map[string]ShearItem{}
	ma30Grp := map[string]ShearItem{}
	//搜集各个维度未过期的shearItem数据,组合成shearForceGrp对象
	for _, v := range segments {
		cs := CandleSegment{}
		sv, _ := json.Marshal(v)
		json.Unmarshal(sv, &cs)
		shi30, err := MakeShearItem(cr, sheGrp.InstID, cs.Seg, "ma30")
		if err != nil {
			logrus.Warn(GetFuncName(), err)
		} else {
			ma30Grp[cs.Seg] = *shi30
		}
		shi7, err := MakeShearItem(cr, sheGrp.InstID, cs.Seg, "ma7")
		if err != nil {
			logrus.Warn(GetFuncName(), err)
		} else {
			ma7Grp[cs.Seg] = *shi7
		}
		sheGrp.Ma7PeriodGroup = ma7Grp
		sheGrp.Ma30PeriodGroup = ma30Grp
	}
	return nil
}

func MakeShearItem(cr *Core, instId string, period string, ctype string) (*ShearItem, error) {
	shi := ShearItem{}
	keyn := instId + "|" + period + "|" + ctype + "|shearItem"
	res, err := cr.RedisLocalCli.Get(keyn).Result()
	if err != nil && len(res) == 0 {
		return &shi, err
	}
	json.Unmarshal([]byte(res), &shi)
	return &shi, err
}

func (sheGrp *ShearForceGrp) Process(cr *Core) error {
	go func() {
		sheGrp.Show(cr)
		// 传递过来的shg对象是空的，需要从segmentItem对象创建的shearItem对象组合中来重建
		sheGrp.Refresh(cr)
		err := sheGrp.SetToKey(cr)
		if err != nil {
			logrus.Panic("srs SetToKey err: ", err)
		}
		// sheGrp.MakeSnapShot(cr)
		// 下一个阶段计算
		allow := os.Getenv("SARDINE_MAKEANALYTICS") == "true"
		if !allow {
			return
		}

		periodList := []string{}
		for k := range sheGrp.Ma30PeriodGroup {
			periodList = append(periodList, k)
		}
	}()
	go func() {
		sheGrp.AddToRatioSorted(cr)
	}()
	go func() {
		// 另一个携程中，Analytics对象要读这里snapShot，我希望它读到的是老的而不是新的，所以等待2秒钟
		time.Sleep(2 * time.Second)
		sheGrp.MakeSnapShot(cr)
	}()
	return nil
}
