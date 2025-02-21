package core

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/go-redis/redis"
	logrus "github.com/sirupsen/logrus"
)

// 1. 成交量计算和保存
// func (tk *TickerInfo) MakeVolSorted(cr *Core, period string) {
// SORTED_INTERVAL 分钟的整数倍
// minutes := cr.PeriodToMinutes(period)
// ts := tk.Ts
// ts = ts - ts%60
// z := redis.Z{
// Score:  float64(tk.VolCcy24h),
// Member: tk.InstID,
// }
// 计算当次成交量环比增长比值, 存入当次成交价
// if ts%(minutes*60) == 0 {
// 计算成交量环比增幅
// tk.makeVolSpeed(cr, period)
// cr.RedisLocalCli.ZAdd(SORTED_VOL+"|"+period, z).Result()
// }
// }
//
// 2. 成交量环比增幅计算和保存
// func (tk *TickerInfo) makeVolSpeed(cr *Core, period string) {
// spd := float64(1)
// preVolCcy24h, err := cr.RedisLocalCli.ZScore(SORTED_VOL+"|"+period, tk.InstID).Result()
// if err != nil {
// TODO 新股
// } else {
// if tk.VolCcy24h < preVolCcy24h {
// 新的一天开始了
// } else {
// tk.VolDiff = tk.VolCcy24h - preVolCcy24h
// }
// }
// tk.VolDiff = spd
// 计算环比增幅振幅
// tk.makeVolAcclr(cr, period)
// z := redis.Z{
// Score:  tk.VolDiff,
// Member: tk.InstID,
// }
// 保存当次成交量环比增幅
// cr.RedisLocalCli.ZAdd(SORTED_VOLDIFF+"|"+period, z).Result()
// }
//
// 3. 成交量环比增长振幅计算和保存
// func (tk *TickerInfo) makeVolAcclr(cr *Core, period string) {
// acclr := float64(0)
// score, err := cr.RedisLocalCli.ZScore(SORTED_VOLDIFF+"|"+period, tk.InstID).Result()
// if err != nil {
// TODO 新股
// fmt.Println("zScore err:", score, err)
// } else {
// 当次比值和上次比值之间的比值作为当次振幅保存
// acclr = (tk.VolDiff + score) / (tk.VolCcy24h - score)
// }
// tk.VolAcclr = acclr
// z := redis.Z{
// Score:  acclr,
// Member: tk.InstID,
// }
// cr.RedisLocalCli.ZAdd(SORTED_VOLACCLER+"|"+period, z).Result()
// }

// ------------------------------------------------------------------------
// 1. 时间节点: t0; 价格保存
func (tk *TickerInfo) MakePriceSorted(cr *Core, period string) {
	ts := tk.Ts
	tm := time.Now().UnixMilli()
	bj, _ := json.Marshal(tk)
	if tm > (ts + 100000) {
		logrus.Info("tickerInfo已经失效:", tm-(ts+1000), "毫秒", string(bj))
	} else {
		logrus.Info("tickerInfo有效:", string(bj))
	}
	ts = ts - ts%60*1000
	minutes, err := cr.PeriodToMinutes(period)
	if err != nil {
		return
	}
	z := redis.Z{
		Score:  float64(tk.Last),
		Member: tk.InstId,
	}
	// 满足约定时间间隔，执行制作价格差和存储当次价格的动作, 每分钟都计算，不管间隔是多少分钟，分到不同的seg里去，10分钟的话，就会有9个seg
	for i := int64(0); i < minutes; i++ {
		if (ts%(minutes*60*1000))/(60*1000) == int64(i) {
			//计算成交价环比增幅
			tk.makePriceDiff(cr, period, i)
			// 保存本次成交价
			cr.RedisLocalCli.ZAdd(SORTED_PRICE+"|"+period+"|seg"+strconv.FormatInt(i, 10), z).Result()
		}
	}

}

// 2. 时间节点: t1; 成交价环比增幅计算和保存
func (tk *TickerInfo) makePriceDiff(cr *Core, period string, seg int64) {
	spd := float64(1)
	score, err := cr.RedisLocalCli.ZScore(SORTED_PRICE+"|"+period+"|seg"+strconv.FormatInt(seg, 10), tk.InstId).Result()
	if err != nil {
		//TODO 新股
		logrus.Warn("makeVolSpeed zScore err:", score, err)
	} else {
		spd = tk.Last - score
	}
	tk.PriceDiff = spd
	tk.makePriceAcclr(cr, period, seg)
	z := redis.Z{
		Score:  spd,
		Member: tk.InstId,
	}
	cr.RedisLocalCli.ZAdd(SORTED_PRICEDIFF+"|"+period+"|seg"+strconv.FormatInt(seg, 10), z).Result()
	// fmt.Println("sorted PriceDiff: ", tk.InstID, "new Price:", tk.Last, "old Price:", score, "PriceDiff:", spd, "rs:", rs)
}

// 3. 时间节点: t2; 价格变化振幅排行榜, 计算振幅需要三个阶段，第一个阶段得到降/增幅，第二个阶段得到降/增幅的差值，第三个阶段得到将增幅差值的比值
func (tk *TickerInfo) makePriceAcclr(cr *Core, period string, seg int64) {
	acclr := float64(0)
	score, err := cr.RedisLocalCli.ZScore(SORTED_PRICEDIFF+"|"+period+"|seg"+strconv.FormatInt(seg, 10), tk.InstId).Result()
	if err != nil {
		//TODO 新股
		fmt.Println("zScore err:", score, err)
	} else {
		// 在时间节点t2, 用最近两个周期增降幅的叠加之后的结果，除以t0时刻的成交价, 得到振幅
		// 时间跨度为两个周期，t0开始，t3结束
		acclr = (tk.PriceDiff + score) / (tk.Last - score)
	}
	tk.PriceAcclr = acclr
	z := redis.Z{
		Score:  acclr,
		Member: tk.InstId,
	}
	zname := SORTED_PRICEACCLR + "|" + period + "|seg" + strconv.FormatInt(seg, 10)
	// fmt.Println("zname1:", zname)
	cr.RedisLocalCli.ZAdd(zname, z).Result()
	// fmt.Println("sorted Price acclr ", tk.InstID, "new diff:", tk.PriceDiff, "old PriceDiff:", score, "acclr:", acclr, "rs:", rs)
}

func commonSorted(cr *Core, ctype string, period string, seg int64) []redis.Z {
	env := os.Getenv("GO_ENV")
	isProdction := env == "production"
	zname := ctype + "|" + period + "|seg" + strconv.FormatInt(seg, 10)
	ary, err := cr.RedisLocalCli.ZRevRangeWithScores(zname, 0, -1).Result()
	if err != nil {
		if !isProdction {
			fmt.Println("sortedSet err:", err)
		}
	}
	return ary
}
