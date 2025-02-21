package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-redis/redis"
	logrus "github.com/sirupsen/logrus"
	// "phyer.click/sardine/utils"
)

type Series struct {
	InstID         string  `json:"instID"`
	Period         string  `json:"Period"`
	Count          int     `json:"count,number"`
	Scale          float64 `json:"scale,number"`
	LastUpdateTime int64   `json:"lastUpdateTime,number"`
	UpdateNickName string
	LastCandle1m   Candle     `json:"lastCandle1m"`
	CandleSeries   *PixelList `json:"candleSerie"`
	Ma7Series      *PixelList `json:"ma7Serie"`
	Ma30Series     *PixelList `json:"ma30Serie"`
}

type SeriesInfo struct {
	InstID      string  `json:"instID"`
	Period      string  `json:"period"`
	InsertedNew bool    `json:"insertedNew,bool"`
	Score       float64 `json:"score,number"`
}
type SeriesInfoScore struct {
	InstID string  `json:"instID"`
	Score  float64 `json:"score,number"`
}

// TODO
// redis key: verticalReportItem|BTC-USDT|4H-15m|ts:1643002300000
// sortedSet: verticalLimit|2D-4H|rank|sortedSet
type VerticalReportItem struct {
	InstID            string
	Period            string
	ReportTime        int64
	LastUpdate        int64
	LastUpdateTime    string
	Interval          int
	TrigerValue       float64
	AdvUpSellPrice    float64
	AdvDownSellPrice  float64
	Rank              float64
	ShearForce        float64
	VerticalElevation float64
	SecondPeriod      string
}

// type Segment struct {
// IndextStart int
// IndexEnd    int
//
// }

// 根据instId 和period 从 PlateMap里拿到coaster，创建对应的	series,
func (sr *Series) Refresh(cr *Core) error {
	curCo, err := cr.GetCoasterFromPlate(sr.InstID, sr.Period)
	if err != nil {
		return err
	}

	ma30List := curCo.Ma30List.List
	ma30len := len(ma30List)
	if ma30len == 0 {
		err = errors.New("ma30List is empty:" + sr.InstID + "," + sr.Period)
		return err
	}
	baseMaX := ma30List[ma30len-1]

	ma30Pxl, err := curCo.Ma30List.MakePixelList(cr, baseMaX, sr.Scale)
	if err != nil {
		return err
	}
	sr.Ma30Series = ma30Pxl
	ma7Pxl, err := curCo.Ma7List.MakePixelList(cr, baseMaX, sr.Scale)
	if err != nil {
		return err
	}
	sr.Ma7Series = ma7Pxl
	curCo.CandleList.RecursiveBubbleS(len(curCo.CandleList.List), "asc")
	candlePxl, err := curCo.CandleList.MakePixelList(cr, baseMaX, sr.Scale)
	if err != nil {
		return err
	}
	sr.CandleSeries = candlePxl
	// bj, _ := json.Marshal(sr.Ma30Series)
	// fmt.Println("sr.Ma30Series:", sr.Period, sr.InstID, string(bj))
	sr.LastUpdateTime = sr.Ma30Series.LastUpdateTime
	// fmt.Println("candlePxl: ", candlePxl)
	return nil
}

func (sr *Series) SetToKey(cr *Core) (string, error) {
	if sr == nil || sr.CandleSeries == nil {
		return "", errors.New("sr.CandlesSeries == nil")
	}
	sr.CandleSeries.RecursiveBubbleS(sr.CandleSeries.Count, "asc")
	sr.CandleSeries.ReIndex()
	sr.CandleSeries.RecursiveBubbleS(sr.CandleSeries.Count, "asc")
	// sr.CandleSeries.RecursiveBubbleX(sr.CandleSeries.Count, "asc")
	sr.Ma7Series.RecursiveBubbleS(sr.CandleSeries.Count, "asc")
	// sr.Ma7Series.RecursiveBubbleX(sr.CandleSeries.Count, "asc")
	sr.Ma30Series.RecursiveBubbleS(sr.CandleSeries.Count, "asc")
	// sr.Ma30Series.RecursiveBubbleX(sr.CandleSeries.Count, "asc")
	now := time.Now().UnixMilli()
	sr.LastUpdateTime = now
	sr.CandleSeries.LastUpdateTime = now
	sr.CandleSeries.UpdateNickName = utils.GetRandomString(12)
	sr.UpdateNickName = utils.GetRandomString(12)
	js, _ := json.Marshal(*sr)
	seriesName := sr.InstID + "|" + sr.Period + "|series"
	res, err := cr.RedisLocalCli.Set(seriesName, string(js), 0).Result()
	if err != nil {
		logrus.Panic(utils.GetFuncName(), err, " seriesSetToKey1: instId:", sr.InstID, " period: ", sr.Period, " lastUpdate:", sr.LastUpdateTime, " md5:", utils.Md5V(string(js)))
	}
	res, err = cr.RedisLocal2Cli.Set(seriesName, string(js), 0).Result()
	return res, err
}
func PrintSerieY(cr *Core, list []redis.Z, period string, count int) {
	// fmt.Println("PrintSerieY start")
	env := os.Getenv("GO_ENV")
	isProduction := env == "production"
	//TODO 只有非产线环境，才会显示此列表
	if !isProduction {
		fmt.Println("seriesYTop count:", count, "period:", period, "sort start")
	}
	seiScrList := []*SeriesInfoScore{}
	for _, v := range list {
		sei := SeriesInfo{}
		seiScr := SeriesInfoScore{}
		json.Unmarshal([]byte(v.Member.(string)), &sei)
		seiScr.InstID = sei.InstID
		seiScr.Score = v.Score
		seiScrList = append(seiScrList, &seiScr)

		// if k < count {
		// if !isProduction {
		// fmt.Println("seriesYTop", count, "No.", k+1, "period"+period, "InstID:", sei.InstID, "score:", v.Score)
		// }
		// 拉扯极限报告
		// }
		// if k == count+1 {
		// if !isProduction {
		// fmt.Println("seriesYTop end -------" + "period" + period + "-------------------------------------")
		// fmt.Println("seriesYLast start -------" + "period" + period + "-------------------------------------")
		// }
		// }
		// if k > len(list)-count-1 {
		// if !isProduction {
		// fmt.Println("seriesYLast", count, "No.", k+1, "period"+period, "InstID:", sei.InstID, "score:", v.Score)
		// }
		// }
	}

	bj, _ := json.Marshal(seiScrList)
	reqBody := bytes.NewBuffer(bj)
	cr.Env = os.Getenv("GO_ENV")
	cr.FluentBitUrl = os.Getenv("SARDINE_FluentBitUrl")
	fullUrl := "http://" + cr.FluentBitUrl + "/seriesY." + period

	res, err := http.Post(fullUrl, "application/json", reqBody)
	fmt.Println("requested, response:", fullUrl, reqBody, res)
	if err != nil {
		logrus.Error(err)
	}

	if !isProduction {
		fmt.Println("seriesYLast count:", count, "period:", period, "sort end")
	}
}

func (sei *SeriesInfo) Process(cr *Core) {
	curSe, err := cr.GetPixelSeries(sei.InstID, sei.Period)
	if err != nil {
		logrus.Warn("GetPixelSeries: ", err)
		return
	}

	// TODO 金拱门
	// list := cr.GetMyCcyBalanceName()
	go func(se Series) {

		threeSeg := [][]int{[]int{0, 19}, []int{19, 22}, []int{22, 23}}

		ma7Seg := se.MakeSegment(cr, 0, 23, threeSeg, "ma7")
		go func() {
			cr.SegmentItemChan <- ma7Seg
		}()

		ma30Seg := se.MakeSegment(cr, 0, 23, threeSeg, "ma30")

		go func() {
			cr.SegmentItemChan <- ma30Seg
		}()

	}(curSe)

	cli := cr.RedisLocalCli
	go func(se Series) {
		// 拉扯极限报告
		willReport := os.Getenv("SARDINE_SERIESTOREPORT") == "true"
		logrus.Info("willReport:", willReport)
		// fmt.Println("willReport:", willReport)
		if !willReport {
			return
		}
		err = curSe.AddToYSorted(cr)
		if err != nil {
			logrus.Warn("sei addToYSorted err: ", err)
			return
		}
		// 所有维度拉扯极限
		go func(se Series) {
			if se.InstID != "BTC-USDT" {
				return
			}
			list, err := cli.ZRevRangeWithScores("series|YValue|sortedSet|period"+se.Period, 0, -1).Result()
			if err != nil {
				fmt.Println("series sorted err", err)
			}
			PrintSerieY(cr, list, se.Period, 20)
		}(se)

	}(curSe)
	// TODO 刘海儿检测, 监测金拱门中的刘海儿，预警下跌趋势, 其实有没有金拱门并不重要，刘海儿比金拱门更有说服力
	go func(se Series) {
		// 如何定义刘海：目前定义如下，3m以上的周期时，当7个或小于7个周期内的时间内发生了一次下坠和一次上升，下坠幅度达到2%以上，并随后的上升中收复了下坠的幅度，那么疑似刘海儿发生。用的周期越少，越强烈，探底和抬升的幅度越大越强烈，所处的维度越高越强烈，比如15m的没有1H的强烈
		// 如果发生在BTC身上，那么将影响所有
		// se.CheckLiuhai()  {
		//
		// }

	}(curSe)
	go func(se Series) {
		allow := os.Getenv("SARDINE_SERIESINFOTOCHNL") == "true"
		if !allow {
			return
		}
		time.Sleep(0 * time.Second)
		sei := SeriesInfo{
			InstID: curSe.InstID,
			Period: curSe.Period,
		}
		cr.AddToGeneralSeriesInfoChnl(&sei)
	}(curSe)

}

//-------------------------------------------------------------------------------

// 拉扯极限相关： 加入seriesY值排行榜, 用于生成拉扯极限
func (srs *Series) AddToYSorted(cr *Core) error {
	setName := "series|YValue|sortedSet|period" + srs.Period
	srs.CandleSeries.RecursiveBubbleS(srs.CandleSeries.Count, "asc")
	length := len(srs.CandleSeries.List)
	if length != srs.Count {
		err := errors.New("AddToYSorted err: 数据量不够")
		return err
	}
	lastCandlePixel1 := srs.CandleSeries.List[srs.Count-1]
	sei := SeriesInfo{
		InstID: srs.InstID,
		Period: srs.Period,
	}
	bj, _ := json.Marshal(sei)
	// TODO -200 是个无效的值，如果遇到-200就赋予0值，这个办法不好，后面考虑不用sortedSet，而用自定义对象更好些。
	if lastCandlePixel1.Y == -200 {
		lastCandlePixel1.Y = 0
	}
	z := redis.Z{
		Score:  float64(lastCandlePixel1.Y),
		Member: string(bj),
	}
	// TODO ZAdd 有可能由于bug或者key不一样的原因，让列表变长，需要想办法怎么定期请空
	if lastCandlePixel1.Score != 0 {
		cr.RedisLocalCli.ZAdd(setName, z).Result()
	}
	return nil
}

// 垂直极限排名有一定片面性。暂时先不开放。垂直极限推荐最高的，可能是个不太容易📈上来的股票，甚至垃圾股，而且过一会儿可能跌的更多，所以就算使用这个功能，也仅供参考，
func (vir *VerticalReportItem) AddToVeriticalLimitSorted(cr *Core, srs *Series, period2 string) error {
	// redis key: verticalReportItem|BTC-USDT|4H-15m|ts:1643002300000
	// sortedSet: verticalLimit|2D-4H|rank|sortedSet

	setName := "verticalLimit|" + srs.Period + "-" + period2 + "|rank|sortedSet"
	tms := strconv.FormatInt(srs.LastUpdateTime, 10)
	keyName := "verticalLimit|" + srs.InstID + "|" + srs.Period + "-" + period2 + "|ts:" + tms
	z := redis.Z{
		Score:  float64(srs.LastUpdateTime),
		Member: keyName,
	}
	if vir.Rank != -1 && vir.Rank != 0 {
		extt := 48 * time.Hour
		ot := time.Now().Add(extt * -1)
		oti := ot.UnixMilli()
		count, _ := cr.RedisLocalCli.ZRemRangeByScore(setName, "0", strconv.FormatInt(oti, 10)).Result()
		if count > 0 {
			logrus.Warning("移出过期的引用数量：", setName, count, "ZRemRangeByScore ", setName, 0, strconv.FormatInt(oti, 10))
		}
		cr.RedisLocalCli.ZAdd(setName, z).Result()
		bj, _ := json.Marshal(vir)
		cr.RedisLocalCli.Set(keyName, bj, 48*time.Hour).Result()
	}
	return nil
}

func (vri *VerticalReportItem) Report(cr *Core) error {
	dd := DingdingMsg{
		Topic:     "垂直极限触发",
		RobotName: "pengpeng",
		AtAll:     true,
		Ctype:     "markdown",
		Content:   "",
	}
	ary1 := []string{}
	str := "``币名: ``" + vri.InstID + "\n"
	str1 := fmt.Sprintln("``基础维度：``", vri.Period)
	str2 := fmt.Sprintln("``剪切维度：``", vri.SecondPeriod)
	str21 := fmt.Sprintln("``观察周期：``", vri.Interval)
	str3 := fmt.Sprintln("``平均仰角：``", vri.VerticalElevation)
	str4 := fmt.Sprintln("``剪切力：``", vri.ShearForce)
	str5 := fmt.Sprintln("``Rank：``", vri.Rank)
	score := vri.TrigerValue
	str6 := fmt.Sprintln("``触发买入价位：``", score)
	str7 := fmt.Sprintln("``建议止盈价位：``", vri.AdvUpSellPrice)
	str8 := fmt.Sprintln("``建议止损价位：``", vri.AdvDownSellPrice)
	str9 := "----------------------\n"
	ary1 = append(ary1, str, str1, str2, str21, str3, str4, str5, str6, str7, str8, str9)
	dd.AddItemListGrp("垂直极限", 2, ary1)
	ary2 := []string{}

	tm := time.Now().Format("01-02:15:04")
	rtime := fmt.Sprintln("``报告时间：``", tm)
	ctype := fmt.Sprintln("``类型：``", "极限触发，已弃用")
	from := "来自: " + os.Getenv("HOSTNAME")
	ary2 = append(ary2, rtime, ctype, from)
	dd.AddItemListGrp("", 2, ary2)
	dd.PostToRobot("pengpeng", cr)

	return nil
}

func (vri *VerticalReportItem) Show(cr *Core) error {
	ary1 := []string{}
	str := "``币名: ``" + vri.InstID + "\n"
	str1 := fmt.Sprintln("``基础维度：``", vri.Period)
	str2 := fmt.Sprintln("``剪切维度：``", vri.SecondPeriod)
	str21 := fmt.Sprintln("``观察周期：``", vri.Interval)
	str3 := fmt.Sprintln("``平均仰角：``", vri.VerticalElevation)
	str4 := fmt.Sprintln("``剪切力：``", vri.ShearForce)
	str5 := fmt.Sprintln("``Rank：``", vri.Rank)
	score := vri.TrigerValue
	str6 := fmt.Sprintln("``触发买入价位：``", score)
	str7 := fmt.Sprintln("``建议止盈价位：``", vri.AdvUpSellPrice)
	str8 := fmt.Sprintln("``建议止损价位：``", vri.AdvDownSellPrice)
	str9 := "----------------------\n"
	ary1 = append(ary1, str, str1, str2, str21, str3, str4, str5, str6, str7, str8, str9)
	for _, v := range ary1 {
		fmt.Println("verticalReportItem: ", v)
	}
	return nil
}

// TODO 求某个PixelList里两个点之间的仰角，从ridx开始，往lidx的元素画一条直线，的仰角

func (srs *Series) GetElevation(cr *Core, ctype string, lIdx int, rIdx int) (float64, error) {
	yj := float64(0)
	switch ctype {
	case "candle":
		{
			yj = (srs.CandleSeries.List[rIdx].Y - srs.CandleSeries.List[lIdx].Y) / float64(rIdx-lIdx)
		}
	case "ma7":
		{
			yj = (srs.Ma7Series.List[rIdx].Y - srs.Ma7Series.List[lIdx].Y) / float64(rIdx-lIdx)
		}
	case "ma30":
		{
			yj = (srs.Ma30Series.List[rIdx].Y - srs.Ma30Series.List[lIdx].Y) / float64(rIdx-lIdx)

		}
	}
	return yj, nil
}

// TODO 求极值，在某个线段上。一个最大值，一个最小值
func (srs *Series) GetExtremum(cr *Core, lIdx int, rIdx int, ctype string) (*Extremum, error) {
	ext := Extremum{
		Max: &Pixel{},
		Min: &Pixel{},
	}

	switch ctype {
	case "candle":
		{
			done := false
			for k, v := range srs.CandleSeries.List {
				if k < lIdx {
					continue
				}
				if v.X == 0 && v.Y == 0 {
					continue
				}
				if k > rIdx {
					continue
				}
				if !done {
					ext.Max = srs.CandleSeries.List[k]
					ext.Min = srs.CandleSeries.List[k]
					done = true
				}

				if v.Y > ext.Max.Y {
					ext.Max = v
				}
				if v.Y < ext.Min.Y {
					ext.Min = v
				}
			}
			// ext = nil
		}
	case "ma7":
		{
			done := false
			for k, v := range srs.Ma7Series.List {
				if k < lIdx {
					continue
				}
				if v.X == 0 && v.Y == 0 {
					continue
				}
				if k > rIdx {
					continue
				}
				if !done {
					ext.Max = srs.Ma7Series.List[k]
					ext.Min = srs.Ma7Series.List[k]
					done = true
				}
				if v.Y > ext.Max.Y {
					ext.Max = v
				}
				if v.Y < ext.Min.Y {
					ext.Min = v
				}
			}
			// ext = nil
		}
	case "ma30":
		{

			done := false
			for k, v := range srs.Ma30Series.List {
				if k < lIdx {
					continue
				}
				if v.X == 0 && v.Y == 0 {
					continue
				}
				if k > rIdx {
					continue
				}
				if !done {
					ext.Max = srs.Ma30Series.List[k]
					ext.Min = srs.Ma30Series.List[k]
					done = true
				}
				if v.Y > ext.Max.Y {
					ext.Max = v
				}
				if v.Y < ext.Min.Y {
					ext.Min = v
				}
			}
			// ext = nil
		}
	}
	return &ext, nil
}

// TODO 获取垂直极限列表
// 筛选条件：
//
// 1. 极大值未发生在最后周期的，排除
// 2. n周期内，有仰角小于0的，排除
// 注意： 仰角极值未必发生在最后一个周期
//
// 对剩下的币种结果，计算：
//
// 1. n周期平均仰角: s
// 2. 最后周期仰角: p
//
// 筛选出最后仰角高于n周期平均仰角的币列表，
// 以最后仰角为结果，得到一个值 p
// 对此列表集合，得到每个的15分钟维度拉扯极限，每个计算后得到一个结果 f,
//
// f值权重更高，p值权重降一个量级，求出分值用于排名,
//
// rank = 2 * (lcjx * -1) * (1 + avgElevation) * (1 + lastElevation) * (1 + lastElevation)
//
// 存储在sortedSet里，命名：
// verticalLimit|15m~4H|rank|sortedSet
// return rank, err
func (vir *VerticalReportItem) MakeVerticalLimit(cr *Core, srs *Series, startIdx int, endIdx int, period2 string) (err error) {
	count := len(srs.CandleSeries.List) - 1
	lastMa30Pixel := srs.Ma30Series.List[count]
	// func (srs *Series) GetExtremum(cr *Core, lIdx int, rIdx int, ctype string) (*Extremum, error) {
	ext, err := srs.GetExtremum(cr, startIdx, endIdx, "ma30")
	if err != nil {
		logrus.Warn(utils.GetFuncName(), ":", err)
	}

	if ext.Max.Score < 1.05*lastMa30Pixel.Score {
		lbj, _ := json.Marshal(lastMa30Pixel)
		lext, _ := json.Marshal(ext)
		err = errors.New(fmt.Sprintln("当前pixel不是极值", " lastMa30Pixel: ", string(lbj), " ext: ", string(lext)))
		return err
	} else {
		err = errors.New(fmt.Sprintln("当前pixel满足极值", lastMa30Pixel))
	}

	yj, err := srs.GetElevation(cr, "ma30", startIdx, endIdx)
	if err != nil {
		logrus.Warn(utils.GetFuncName(), ":", err)
	}

	vir.VerticalElevation = yj
	lcjx, _ := LacheJixian(cr, srs, period2)
	vir.ShearForce = lcjx
	vir.TrigerValue = srs.CandleSeries.List[len(srs.CandleSeries.List)-1].Score
	vir.AdvUpSellPrice = vir.TrigerValue * 1.04
	vir.AdvDownSellPrice = vir.TrigerValue * 0.98
	// 计算rank的公式如下
	// rank := 2 * (lcjx * -1) * (1 + avgElevation) * (1 + lastElevation) * (1 + lastElevation)
	// vir.Rank = rank
	return nil
}

// 计算剪切力
func LacheJixian(cr *Core, srs *Series, period string) (float64, error) {
	curSe, _ := cr.GetPixelSeries(srs.InstID, period)
	return curSe.CandleSeries.List[len(srs.CandleSeries.List)-1].Y, nil
}

// type SegmentItem struct {
// InstID            string
// Period            string
// ReportTime        int64
// lastUpdate        int64
// Interval          int
// Direct            string // up, down
// VerticalElevation float64
// }
