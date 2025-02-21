package core

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	logrus "github.com/sirupsen/logrus"
	"phyer.click/sardine/utils"
)

// 段对象是对某个线段的表现进行评估的一个手段, 整个段会被分成3个小段, 整个段，计算整体的，字段，各自计算。包含仰角，段内极值等。
// SegmentItem 属于一阶分析结果
// {DMD-USDT 5D ma30 1643240793839 0 1642867200000 0xc00835fd80 0xc001687600 NaN 23 23 []}]
type SegmentItem struct {
	InstID            string
	Period            string //通过InstID,Periods可以定位到Series对象, 里面有一手数据
	Ctype             string //candle|ma7|ma30
	ReportTime        int64
	ReportTimeStr     string
	PolarQuadrant     string // shangxian，manyue，xiaxian,xinyue， 分别对应圆周的四个阶段。
	LastUpdate        int64
	ExtremumPixels    *Extremum     // 极值 是两个pixel对象
	FirstPixel        *Pixel        // 起始值，最后的pixel对象
	LastPixel         *Pixel        // 最后值，最后的maX pixel对象
	LastCandle        *Pixel        // 最后值，最后的Candle的pixel对象
	LastMa7           *Pixel        // 最后值，最后的Ma7的pixel对象
	LastMa30          *Pixel        // 最后值，最后的Ma30的pixel对象
	VerticalElevation float64       // 仰角, Interval范围内线段的仰角
	StartIdx          int           // 开始的坐标
	EndIdx            int           // 结束的坐标
	SubItemList       []SegmentItem //往下一级微分
}

const DAMANYUE = "damanyue"
const DAMANYUE_POST = "damanyue_post"
const DAMANYUE_PRE = "damanyue_pre"
const XIAOMANYUE = "xiaomanyue"
const XIAOMANYUE_POST = "xiaomanyue_post"
const XIAOMANYUE_PRE = "xiaomanyue_pre"
const DAXINYUE = "daxinyue"
const DAXINYUE_POST = "daxinyue_post"
const DAXINYUE_PRE = "daxinyue_pre"
const XIAOXINYUE = "xiaoxinyue"
const XIAOXINYUE_POST = "xiaoxinyue_post"
const XIAOXINYUE_PRE = "xiaoxinyue_pre"
const DASHANGXIANYUE = "dashangxianyue"
const XIAOSHANGXIANYUE = "xiaoshangxianyue"
const DAXIAXIANYUE = "daxiaxianyue"
const XIAOXIAXIANYUE = "xiaoxiaxianyue"

const tinySeg = 0.1

type Extremum struct {
	Max *Pixel
	Min *Pixel
}

func CalPolar(e0 float64, e1 float64, e2 float64) string {
	polarQuadrant := "default"
	//
	// ## 上弦月
	//
	// e0,e1,e2: -3.5315694477826525 -0.5773082714100172 1.0558744145515746
	if e2 >= e1 && e2 >= 0 {
		polarQuadrant = XIAOSHANGXIANYUE
	}
	// e2 > e1 > 0: 小上弦月
	// -> e1 > e0 > 0 : 大上弦月
	if e2 >= e1 && e1 >= 0 {
		polarQuadrant = XIAOSHANGXIANYUE
		if e1 >= e0 && e0 >= 0 {
			polarQuadrant = DASHANGXIANYUE
		}
	}
	//
	// ## 下弦月
	// 0 > e1 >  e2：小下弦月
	// -> 0 > e0 > e1: 大下弦月
	//
	if 0 >= e1 && e1 >= e2 {
		polarQuadrant = XIAOXIAXIANYUE
		if 0 >= e0 && e0 >= e1 {
			polarQuadrant = DAXIAXIANYUE
		}
	}

	// ## 同上
	if (0 >= e2 && 0 >= e1) && e2 >= e1 {
		polarQuadrant = XIAOXIAXIANYUE
	}
	// ##
	// ## 满月
	//
	// e1 >  e2 > 0  : 小满月 pre
	// -> e0 > e1 : 大满月pre
	//

	if e1 >= e2 && e2 >= 0 {
		polarQuadrant = XIAOMANYUE_PRE
		if e0 > e1 {
			polarQuadrant = DAMANYUE_PRE
		}
	}
	// e1 > 0.1 > e2 > 0 : 小满月
	// -> e0 > e1 : 大满月
	//

	if e1 >= tinySeg && tinySeg >= e2 && e2 >= 0 {
		polarQuadrant = XIAOMANYUE
		if e0 > e1 {
			polarQuadrant = DAMANYUE
		}
	}

	// e0,e1,e2: 0.9699903789854316 0.1802190672652184 -1.7888783234326784
	// e1 > 0 >  e2 > -0.1 : 小满月post
	// ->	e0 > e1 > 0 : 大满月post
	//
	if e1 >= 0 && 0 >= e2 && e2 >= -100000 {
		polarQuadrant = XIAOMANYUE_POST
		if e0 > e1 {
			polarQuadrant = DAMANYUE_POST
		}
	}
	// e0,e1,e2: -0.049579775302532776 0 -0.018291567587323976
	// ## 新月
	// e1 < e2 <0:  小新月pre
	// -> e1 > e0 : 大新月pre
	//
	if e1 <= e2 && e2 <= 0 && e2 >= -1*tinySeg {
		polarQuadrant = XIAOXINYUE_PRE
		if e1 > e0 {
			polarQuadrant = DAXINYUE_PRE
		}
	}
	// e1 < -0.1  < e2  <0  小新月
	// -> e1 > e0 : 大新月
	if e1 <= -1*tinySeg && -1*tinySeg <= e2 && e2 <= 0 {
		polarQuadrant = XIAOXINYUE
		if e1 > e0 {
			polarQuadrant = DAXINYUE
		}
	}
	//
	// e1 < 0 < e2 < 0.1 小新月post
	// -> e1 > e0 : 大新月post
	//e0,e1,e2: -0.03902244287114438 -0.13929829606729519 0.14828528291036536
	if e1 <= 0 && 0 <= e2 && e2 <= 1000000 {
		polarQuadrant = XIAOXINYUE_POST
		if e1 > e0 {
			polarQuadrant = DAXINYUE_POST
		}
	}

	return polarQuadrant
}

// 计算当前某段的曲线正弦所处极坐标象限
func CalPolarQuadrant(maXSeg *SegmentItem) string {
	if len(maXSeg.SubItemList) == 0 {
		return "subItem no polarQuadrant"
	}
	m0 := maXSeg.SubItemList[0]
	m1 := maXSeg.SubItemList[1]
	m2 := maXSeg.SubItemList[2]
	e0 := m0.VerticalElevation
	e1 := m1.VerticalElevation
	e2 := m2.VerticalElevation
	polarQuadrant := CalPolar(e0, e1, e2)
	if polarQuadrant == "default" {
		env := os.Getenv("GO_ENV")
		if env != "production" {
			fmt.Println(utils.GetFuncName(), " instId:", maXSeg.InstID, " period:", maXSeg.Period, " ctype", maXSeg.Ctype, " e0,e1,e2:", e0, e1, e2)
		}
	}

	return polarQuadrant
}

func (seg *SegmentItem) SetToKey(cr *Core) error {
	if seg.InstID == "USDC-USDT" {
		return nil
	}
	keyName := seg.InstID + "|" + seg.Period + "|" + seg.Ctype + "|segmentItem"
	bj, err := json.Marshal(seg)
	if err != nil {
		logrus.Warn("se.MakeSegment: ", err, seg)
	}
	cr.RedisLocalCli.Set(keyName, string(bj), 0)

	sf7 := float64(0)
	sf7 = seg.LastCandle.Y - seg.LastMa7.Y
	sf30 := float64(0)
	sf30 = seg.LastCandle.Y
	tms := time.Now().Format("2006-01-02 15:04:05.000")
	// fmt.Println("tms: ", seg.InstID, seg.Period, tms, seg.LastUpdate)
	she := ShearItem{
		LastUpdate:        time.Now().UnixMilli(),
		LastUpdateTime:    tms,
		VerticalElevation: seg.SubItemList[2].VerticalElevation,
		Ratio:             seg.LastCandle.Y / seg.SubItemList[2].VerticalElevation,
		Score:             seg.LastCandle.Score,
		PolarQuadrant:     seg.PolarQuadrant,
	}

	if seg.Ctype == "ma7" {
		she.ShearForce = sf7
	}

	if seg.Ctype == "ma30" {
		she.ShearForce = sf30
	}
	sbj, _ := json.Marshal(she)
	keyName = seg.InstID + "|" + seg.Period + "|" + seg.Ctype + "|shearItem"
	cr.RedisLocalCli.Set(keyName, string(sbj), 3*time.Minute)
	cr.RedisLocal2Cli.Set(keyName, string(sbj), 3*time.Minute)
	return nil
}

func (seg *SegmentItem) Show() error {
	if seg.InstID == "USDC-USDT" {
		return nil
	}
	bj, _ := json.Marshal(*seg)
	logrus.Warn("SegmentItem Show:", string(bj))
	return nil
}

func (jgm *SegmentItem) Report(cr *Core) error {
	return nil
}
func (seg *SegmentItem) Process(cr *Core) {
	go func() {
		if seg == nil {
			return
		}
		seg.Show()
		seg.SetToKey(cr)
		// sheGrp, err := seg.MakeShearForceGrp(cr)
		// if err != nil {
		// log.Panic(err)
		// }
		// 当最后一个维度数据更新后，触发显示和备份

		// 空的就可以
		shg := ShearForceGrp{
			InstID:          seg.InstID,
			Ma30PeriodGroup: map[string]ShearItem{},
			Ma7PeriodGroup:  map[string]ShearItem{},
		}
		if seg.Period == "4H" {
			time.Sleep(50 * time.Millisecond) //等可能存在的5D也ready
			go func() {
				cr.ShearForceGrpChan <- &shg
			}()
		}
	}()
}

func (srs *Series) MakeSegment(cr *Core, start int, end int, subArys [][]int, ctype string) *SegmentItem {
	list := []*Pixel{}
	if ctype == "ma7" {
		list = srs.Ma7Series.List
	}
	if ctype == "ma30" {
		list = srs.Ma30Series.List
	}
	st := start
	if len(list) == 0 {
		return nil
	}
	for i := start; i <= end; i++ {
		if list[i].X == 0 && list[i].Y == 0 {
			if i+1 < len(list) {
				st = i + 1
			} else {
				logrus.Panic(utils.GetFuncName(), "没有符合的记录")
			}
		}
	}
	extra, _ := srs.GetExtremum(cr, st, end, ctype)
	yj, err := srs.GetElevation(cr, ctype, st, end)
	if err != nil {
		fmt.Println("MakeSegment GetElevation err : ", err)
	}
	tm := time.Now()
	seg := SegmentItem{
		InstID:            srs.InstID,
		Period:            srs.Period,
		ReportTime:        tm.UnixMilli(),
		ReportTimeStr:     tm.Format("2006-01-02 15:04:05.000"),
		LastUpdate:        srs.LastUpdateTime,
		FirstPixel:        list[st],
		LastPixel:         list[end],
		ExtremumPixels:    extra,
		Ctype:             ctype,
		VerticalElevation: yj,
		StartIdx:          st,
		EndIdx:            end,
		LastCandle:        srs.CandleSeries.List[end],
		LastMa7:           srs.Ma7Series.List[end],
		SubItemList:       []SegmentItem{},
		PolarQuadrant:     "none",
	}

	if len(subArys) > 0 {
		for _, pair := range subArys {
			sub := [][]int{}
			curSeg := srs.MakeSegment(cr, pair[0], pair[1], sub, ctype)
			seg.SubItemList = append(seg.SubItemList, *curSeg)
		}
	}
	polar := CalPolarQuadrant(&seg)
	seg.PolarQuadrant = polar
	return &seg
}
