package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	// "math/rand"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	// simple "github.com/bitly/go-simplejson"
	// "v5sdk_go/ws/wImpl"
	"github.com/go-redis/redis"
	"github.com/phyer/texus/private"
	"github.com/phyer/v5sdkgo/rest"
	logrus "github.com/sirupsen/logrus"
)

type Core struct {
	Env                  string
	Cfg                  *MyConfig
	RedisLocalCli        *redis.Client
	RedisRemoteCli       *redis.Client
	FluentBitUrl         string
	PlateMap             map[string]*Plate
	TrayMap              map[string]*Tray
	CoasterMd5SyncMap    sync.Map
	Mu                   *sync.Mutex
	Mu1                  *sync.Mutex
	Waity                *sync.WaitGroup
	CandlesProcessChan   chan *Candle
	MaXProcessChan       chan *MaX
	RsiProcessChan       chan *Rsi
	StockRsiProcessChan  chan *StockRsi
	TickerInforocessChan chan *TickerInfo
	CoasterChan          chan *CoasterInfo
	//	SeriesChan           chan *SeriesInfo
	//	SegmentItemChan      chan *SegmentItem
	MakeMaXsChan chan *Candle
	//	ShearForceGrpChan    chan *ShearForceGrp
	InvokeRestQueueChan chan *RestQueue
	RedisLocal2Cli      *redis.Client
	RestQueueChan       chan *RestQueue
	RestQueue
	WriteLogChan chan *WriteLog
}

type RestQueue struct {
	InstId   string
	Bar      string
	After    int64
	Before   int64
	Limit    string
	Duration time.Duration
	WithWs   bool
}

type CandleData struct {
	Code string          `json:"code"`
	Msg  string          `json:"msg"`
	Data [][]interface{} `json:"data"` // 用二维数组来接收 candles 数据
}
type SubAction struct {
	ActionName string
	ForAll     bool
	MetaInfo   map[string]interface{}
}

func (rst *RestQueue) Show(cr *Core) {
	logrus.Info("restQueue:", rst.InstId, rst.Bar, rst.Limit)
}

func (rst *RestQueue) Save(cr *Core) {
	afterSec := ""
	if rst.After > 0 {
		afterSec = fmt.Sprint("&after=", rst.After)
	}
	beforeSec := ""
	if rst.Before > 0 {
		beforeSec = fmt.Sprint("&before=", rst.Before)
	}
	limitSec := ""
	if len(rst.Limit) > 0 {
		limitSec = fmt.Sprint("&limit=", rst.Limit)
	}
	isHistory := false
	ct, _ := cr.PeriodToMinutes(rst.Bar)
	if rst.After/ct > 10 {
		isHistory = true
	}
	prfix := ""
	if isHistory {
		prfix = "history-"
	}
	link := "/api/v5/market/" + prfix + "candles?instId=" + rst.InstId + "&bar=" + rst.Bar + limitSec + afterSec + beforeSec

	logrus.Info("restLink: ", link)
	rsp, err := cr.v5PublicInvoke(link)
	if err != nil {
		logrus.Warn("cr.v5PublicInvoke err:", err, " count: ", len(rsp.Data), " instId: ", rst.InstId, " period: ", rst.Bar, " after:", afterSec)
	} else {
		logrus.Warn("cr.v5PublicInvoke result count:", len(rsp.Data), " instId: ", rst.InstId, " period: ", rst.Bar, " after:", afterSec, " rsp: ", rsp)
	}
	cr.SaveCandle(rst.InstId, rst.Bar, rsp, rst.Duration, rst.WithWs)
}

func WriteLogProcess(cr *Core) {
	for {
		wg := <-cr.WriteLogChan
		go func(wg *WriteLog) {
			logrus.Info("start writelog: " + wg.Tag + " " + wg.Id)
			wg.Process(cr)
		}(wg)
		time.Sleep(5 * time.Millisecond)
	}
}

// func (cr *Core) ShowSysTime() {
// 	rsp, _ := cr.RestInvoke("/api/v5/public/time", rest.GET)
// 	fmt.Println("serverSystem time:", rsp)
// }

func (core *Core) Init() {
	core.Env = os.Getenv("GO_ENV")
	gitBranch := os.Getenv("gitBranchName")
	commitID := os.Getenv("gitCommitID")

	logrus.Info("当前环境: ", core.Env)
	logrus.Info("gitBranch: ", gitBranch)
	logrus.Info("gitCommitID: ", commitID)
	cfg := MyConfig{}
	cfg, _ = cfg.Init()
	core.Cfg = &cfg
	cli, err := core.GetRedisLocalCli()
	core.RedisLocalCli = cli
	core.RestQueueChan = make(chan *RestQueue)
	core.WriteLogChan = make(chan *WriteLog)
	// 跟订单有关的都关掉
	// core.OrderChan = make(chan *private.Order)
	if err != nil {
		logrus.Error("init redis client err: ", err)
	}
}

func (core *Core) GetRedisCliFromConf(conf RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     conf.Url,
		Password: conf.Password, //默认空密码
		DB:       conf.Index,    //使用默认数据库
	})
	pong, err := client.Ping().Result()
	if pong == "PONG" && err == nil {
		return client, err
	} else {
		logrus.Error("redis状态不可用:", conf.Url, conf.Password, conf.Index, err)
	}

	return client, nil
}

func (core *Core) GetRemoteRedisLocalCli() (*redis.Client, error) {
	ru := core.Cfg.RedisConf.Url
	rp := core.Cfg.RedisConf.Password
	ri := core.Cfg.RedisConf.Index
	re := os.Getenv("REDIS_URL")
	if len(re) > 0 {
		ru = re
	}
	client := redis.NewClient(&redis.Options{
		Addr:     ru,
		Password: rp, //默认空密码
		DB:       ri, //使用默认数据库
	})
	pong, err := client.Ping().Result()
	if pong == "PONG" && err == nil {
		return client, err
	} else {
		logrus.Error("redis状态不可用:", ru, rp, ri, err)
	}

	return client, nil
}
func (core *Core) GetRedisLocalCli() (*redis.Client, error) {
	ru := core.Cfg.RedisConf.Url
	rp := core.Cfg.RedisConf.Password
	ri := core.Cfg.RedisConf.Index
	re := os.Getenv("REDIS_URL")
	if len(re) > 0 {
		ru = re
	}
	client := redis.NewClient(&redis.Options{
		Addr:     ru,
		Password: rp, //默认空密码
		DB:       ri, //使用默认数据库
	})
	pong, err := client.Ping().Result()
	if pong == "PONG" && err == nil {
		return client, err
	} else {
		logrus.Error("redis状态不可用:", ru, rp, ri, err)
	}

	return client, nil
}

// 这些应该是放到 texus 里实现的
func (core *Core) GetAllTickerInfo() (*rest.RESTAPIResult, error) {
	// GET / 获取所有产品行情信息
	rsp, err := core.RestInvoke("/api/v5/market/tickers?instType=SPOT", rest.GET)
	return rsp, err
}

// 这些跟 订单有关，都关掉
//
//	func (core *Core) GetBalances() (*rest.RESTAPIResult, error) {
//		// TODO 临时用了两个实现，restInvoke，复用原来的会有bug，不知道是谁的bug
//		rsp, err := core.RestInvoke2("/api/v5/account/balance", rest.GET, nil)
//		return rsp, err
//	}
//
//	func (core *Core) GetLivingOrderList() ([]*private.Order, error) {
//		// TODO 临时用了两个实现，restInvoke，复用原来的会有bug，不知道是谁的bug
//		params := make(map[string]interface{})
//		data, err := core.RestInvoke2("/api/v5/trade/orders-pending", rest.GET, &params)
//		odrsp := private.OrderResp{}
//		err = json.Unmarshal([]byte(data.Body), &odrsp)
//		str, _ := json.Marshal(odrsp)
//		fmt.Println("convert: err:", err, " body: ", data.Body, odrsp, " string:", string(str))
//		list, err := odrsp.Convert()
//		fmt.Println("loopLivingOrder response data:", str)
//		fmt.Println(utils.GetFuncName(), " 当前数据是 ", data.V5Response.Code, " list len:", len(list))
//		return list, err
//	}
// func (core *Core) LoopInstrumentList() error {
// 	for {
// 		time.Sleep(3 * time.Second)
// 		ctype := ws.SPOT
//
// 		redisCli := core.RedisLocalCli
// 		counts, err := redisCli.HLen("instruments|" + ctype + "|hash").Result()
// 		if err != nil {
// 			fmt.Println("err of hset to redis:", err)
// 		}
// 		if counts == 0 {
// 			continue
// 		}
// 		break
// 	}
// 	return nil
// }
// func (core *Core) SubscribeTicker(op string) error {
// 	mp := make(map[string]string)
//
// 	redisCli := core.RedisLocalCli
// 	ctype := ws.SPOT
// 	mp, err := redisCli.HGetAll("instruments|" + ctype + "|hash").Result()
// 	b, err := json.Marshal(mp)
// 	js, err := simple.NewJson(b)
// 	if err != nil {
// 		fmt.Println("err of unMarshalJson3:", js)
// 	}
// 	// fmt.Println("ticker js: ", js)
// 	instAry := js.MustMap()
// 	for k, v := range instAry {
// 		b = []byte(v.(string))
// 		_, err := simple.NewJson(b)
// 		if err != nil {
// 			fmt.Println("err of unMarshalJson4:", js)
// 		}
// 		time.Sleep(5 * time.Second)
// 		go func(instId string, op string) {
//
// 			redisCli := core.RedisLocalCli
// 			_, err = redisCli.SAdd("tickers|"+op+"|set", instId).Result()
// 			if err != nil {
// 				fmt.Println("err of unMarshalJson5:", js)
// 			}
// 		}(k, op)
// 	}
// 	return nil
// }

// 通过接口获取一个币种名下的某个时间范围内的Candle对象集合
// 按说这个应该放到 texus里实现
func (core *Core) v5PublicInvoke(subUrl string) (*CandleData, error) {
	restUrl, _ := core.Cfg.Config.Get("connect").Get("restBaseUrl").String()
	url := restUrl + subUrl
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	var result CandleData
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (core *Core) RestInvoke(subUrl string, method string) (*rest.RESTAPIResult, error) {
	restUrl, _ := core.Cfg.Config.Get("connect").Get("restBaseUrl").String()
	//ep, method, uri string, param *map[string]interface{}
	rest := rest.NewRESTAPI(restUrl, method, subUrl, nil)
	key, _ := core.Cfg.Config.Get("credentialReadOnly").Get("okAccessKey").String()
	secure, _ := core.Cfg.Config.Get("credentialReadOnly").Get("secretKey").String()
	pass, _ := core.Cfg.Config.Get("credentialReadOnly").Get("okAccessPassphrase").String()
	isDemo := false
	if core.Env == "demoEnv" {
		isDemo = true
	}
	rest.SetSimulate(isDemo).SetAPIKey(key, secure, pass)
	response, err := rest.Run(context.Background())
	if err != nil {
		logrus.Error("restInvoke1 err:", subUrl, err)
	}
	return response, err
}

// func (core *Core) RestInvoke2(subUrl string, method string, param *map[string]interface{}) (*rest.RESTAPIResult, error) {
// 	key, err1 := core.Cfg.Config.Get("credentialReadOnly").Get("okAccessKey").String()
// 	secret, err2 := core.Cfg.Config.Get("credentialReadOnly").Get("secretKey").String()
// 	pass, err3 := core.Cfg.Config.Get("credentialReadOnly").Get("okAccessPassphrase").String()
// 	userId, err4 := core.Cfg.Config.Get("connect").Get("userId").String()
// 	restUrl, err5 := core.Cfg.Config.Get("connect").Get("restBaseUrl").String()
// 	if err1 != nil || err2 != nil || err3 != nil || err4 != nil || err5 != nil {
// 		fmt.Println(err1, err2, err3, err4, err5)
// 	} else {
// 		fmt.Println("key:", key, secret, pass, "userId:", userId, "restUrl: ", restUrl)
// 	}
// 	// reqParam := make(map[string]interface{})
// 	// if param != nil {
// 	// reqParam = *param
// 	// }
// 	// rs := rest.NewRESTAPI(restUrl, method, subUrl, &reqParam)
// 	isDemo := false
// 	if core.Env == "demoEnv" {
// 		isDemo = true
// 	}
// 	// rs.SetSimulate(isDemo).SetAPIKey(key, secret, pass).SetUserId(userId)
// 	// response, err := rs.Run(context.Background())
// 	// if err != nil {
// 	// fmt.Println("restInvoke2 err:", subUrl, err)
// 	// }
// 	apikey := rest.APIKeyInfo{
// 		ApiKey:     key,
// 		SecKey:     secret,
// 		PassPhrase: pass,
// 	}
// 	cli := rest.NewRESTClient(restUrl, &apikey, isDemo)
// 	rsp, err := cli.Get(context.Background(), subUrl, param)
// 	if err != nil {
// 		return rsp, err
// 	}
// 	fmt.Println("response:", rsp, err)
// 	return rsp, err
// }

// 跟下单有关的都关掉，以后再说
// func (core *Core) RestPost(subUrl string, param *map[string]interface{}) (*rest.RESTAPIResult, error) {
// 	key, err1 := core.Cfg.Config.Get("credentialMutable").Get("okAccessKey").String()
// 	secret, err2 := core.Cfg.Config.Get("credentialMutable").Get("secretKey").String()
// 	pass, err3 := core.Cfg.Config.Get("credentialMutable").Get("okAccessPassphrase").String()
// 	userId, err4 := core.Cfg.Config.Get("connect").Get("userId").String()
// 	restUrl, err5 := core.Cfg.Config.Get("connect").Get("restBaseUrl").String()
// 	if err1 != nil || err2 != nil || err3 != nil || err4 != nil || err5 != nil {
// 		fmt.Println(err1, err2, err3, err4, err5)
// 	} else {
// 		fmt.Println("key:", key, secret, pass, "userId:", userId, "restUrl: ", restUrl)
// 	}
// 	// 请求的另一种方式
// 	apikey := rest.APIKeyInfo{
// 		ApiKey:     key,
// 		SecKey:     secret,
// 		PassPhrase: pass,
// 	}
// 	isDemo := false
// 	if core.Env == "demoEnv" {
// 		isDemo = true
// 	}
// 	cli := rest.NewRESTClient(restUrl, &apikey, isDemo)
// 	rsp, err := cli.Post(context.Background(), subUrl, param)
// 	if err != nil {
// 		return rsp, err
// 	}
// 	return rsp, err
// }

// 我当前持有的币，每分钟刷新
func (core *Core) GetMyFavorList() []string {
	redisCli := core.RedisLocalCli
	opt := redis.ZRangeBy{
		Min: "10",
		Max: "100000000000",
	}
	cary, _ := redisCli.ZRevRangeByScore("private|positions|sortedSet", opt).Result()
	cl := []string{}
	for _, v := range cary {
		cry := strings.Split(v, "|")[0] + "-USDT"
		cl = append(cl, cry)
	}
	return cary
}

// 得到交易量排行榜,订阅其中前N名的各维度k线,并merge进来我已经购买的币列表，这个列表是动态更新的
// 改了，不需要交易排行榜，我手动指定一个排行即可, tickersVol|sortedSet 改成 tickersList|sortedSet
func (core *Core) GetScoreList(count int) []string {

	// redisCli := core.RedisLocalCli
	myFocusList := core.Cfg.Config.Get("focusList").MustArray()
	logrus.Debug("curList: ", myFocusList)
	lst := []string{}
	for _, v := range myFocusList {
		lst = append(lst, v.(string))
	}
	return lst
}

// 跟订单有关，关掉
// func LoopBalances(cr *Core, mdura time.Duration) {
// 	//协程：动态维护topScore
// 	ticker := time.NewTicker(mdura)
// 	for {
// 		select {
// 		case <-ticker.C:
// 			//协程：循环执行rest请求candle
// 			fmt.Println("loopBalance: receive ccyChannel start")
// 			RestBalances(cr)
// 		}
// 	}
// }

// func LoopLivingOrders(cr *Core, mdura time.Duration) {
// 	//协程：动态维护topScore
// 	ticker := time.NewTicker(mdura)
// 	for {
// 		select {
// 		case <-ticker.C:
// 			//协程：循环执行rest请求candle
// 			fmt.Println("loopLivingOrder: receive ccyChannel start")
// 			RestLivingOrder(cr)
// 		}
// 	}
// }

// 跟订单有关，关掉
// func RestBalances(cr *Core) ([]*private.Ccy, error) {
// 	// fmt.Println("restBalance loopBalance loop start")
// 	ccyList := []*private.Ccy{}
// 	rsp, err := cr.GetBalances()
// 	if err != nil {
// 		fmt.Println("loopBalance err00: ", err)
// 	}
// 	fmt.Println("loopBalance balance rsp: ", rsp)
// 	if err != nil {
// 		fmt.Println("loopBalance err01: ", err)
// 		return ccyList, err
// 	}
// 	if len(rsp.Body) == 0 {
// 		fmt.Println("loopBalance err03: rsp body is null")
// 		return ccyList, err
// 	}
// 	js1, err := simple.NewJson([]byte(rsp.Body))
// 	if err != nil {
// 		fmt.Println("loopBalance err1: ", err)
// 	}
// 	itemList := js1.Get("data").GetIndex(0).Get("details").MustArray()
// 	// maxTickers是重点关注的topScore的coins的数量
// 	cli := cr.RedisLocalCli
// 	for _, v := range itemList {
// 		js, _ := json.Marshal(v)
// 		ccyResp := private.CcyResp{}
// 		err := json.Unmarshal(js, &ccyResp)
// 		if err != nil {
// 			fmt.Println("loopBalance err2: ", err)
// 		}
// 		ccy, err := ccyResp.Convert()
// 		ccyList = append(ccyList, ccy)
// 		if err != nil {
// 			fmt.Println("loopBalance err2: ", err)
// 		}
// 		z := redis.Z{
// 			Score:  ccy.EqUsd,
// 			Member: ccy.Ccy + "|position|key",
// 		}
// 		res, err := cli.ZAdd("private|positions|sortedSet", z).Result()
// 		if err != nil {
// 			fmt.Println("loopBalance err3: ", res, err)
// 		}
// 		res1, err := cli.Set(ccy.Ccy+"|position|key", js, 0).Result()
// 		if err != nil {
// 			fmt.Println("loopBalance err4: ", res1, err)
// 		}
// 		bjs, _ := json.Marshal(ccy)
// 		tsi := time.Now().Unix()
// 		tsii := tsi - tsi%600
// 		tss := strconv.FormatInt(tsii, 10)
// 		cli.Set(CCYPOSISTIONS_PUBLISH+"|ts:"+tss, 1, 10*time.Minute).Result()
// 		fmt.Println("ccy published: ", string(bjs))
// 		//TODO FIXME 50毫秒，每分钟上限是1200个订单，超过就无法遍历完成
// 		time.Sleep(50 * time.Millisecond)
// 		suffix := ""
// 		if cr.Env == "demoEnv" {
// 			suffix = "-demoEnv"
// 		}
// 		// TODO FIXME cli2
// 		cli.Publish(CCYPOSISTIONS_PUBLISH+suffix, string(bjs)).Result()
// 	}
// 	return ccyList, nil
// }

// func RestLivingOrder(cr *Core) ([]*private.Order, error) {
// 	// fmt.Println("restOrder loopOrder loop start")
// 	orderList := []*private.Order{}
// 	list, err := cr.GetLivingOrderList()
// 	if err != nil {
// 		fmt.Println("loopLivingOrder err00: ", err)
// 	}
// 	fmt.Println("loopLivingOrder response:", list)
// 	go func() {
// 		for _, v := range list {
// 			fmt.Println("order orderV:", v)
// 			time.Sleep(30 * time.Millisecond)
// 			cr.OrderChan <- v
// 		}
// 	}()
// 	return orderList, nil
// }

func (cr *Core) ProcessOrder(od *private.Order) error {
	// publish
	go func() {
		suffix := ""
		if cr.Env == "demoEnv" {
			suffix = "-demoEnv"
		}
		cn := ORDER_PUBLISH + suffix
		bj, _ := json.Marshal(od)

		// TODO FIXME cli2
		res, _ := cr.RedisLocalCli.Publish(cn, string(bj)).Result()
		logrus.Info("order publish res: ", res, " content: ", string(bj))
		rsch := ORDER_RESP_PUBLISH + suffix
		bj1, _ := json.Marshal(res)

		// TODO FIXME cli2
		res, _ = cr.RedisLocalCli.Publish(rsch, string(bj1)).Result()
	}()
	return nil
}

//  跟订单有关，关掉
// func (cr *Core) DispatchSubAction(action *SubAction) error {
// 	go func() {
// 		suffix := ""
// 		if cr.Env == "demoEnv" {
// 			suffix = "-demoEnv"
// 		}
// 		fmt.Println("action: ", action.ActionName, action.MetaInfo)
// 		res, err := cr.RestPostWrapper("/api/v5/trade/"+action.ActionName, action.MetaInfo)
// 		if err != nil {
// 			fmt.Println(utils.GetFuncName(), " actionRes 1:", err)
// 		}
// 		rsch := ORDER_RESP_PUBLISH + suffix
// 		bj1, _ := json.Marshal(res)
//
// 		// TODO FIXME cli2
// 		rs, _ := cr.RedisLocalCli.Publish(rsch, string(bj1)).Result()
// 		fmt.Println("action rs: ", rs)
// 	}()
// 	return nil
// }

// func (cr *Core) RestPostWrapper(url string, param map[string]interface{}) (rest.Okexv5APIResponse, error) {
// 	suffix := ""
// 	if cr.Env == "demoEnv" {
// 		suffix = "-demoEnv"
// 	}
// 	res, err := cr.RestPost(url, &param)
// 	fmt.Println("actionRes 2:", res.V5Response.Msg, res.V5Response.Data, err)
// 	bj, _ := json.Marshal(res)
//
// 	// TODO FIXME cli2
// 	cr.RedisLocalCli.Publish(ORDER_RESP_PUBLISH+suffix, string(bj))
// 	return res.V5Response, nil
// }

func (cr *Core) AddToGeneralCandleChnl(candle *Candle, channels []string) {
	redisCli := cr.RedisLocalCli
	ab, err := json.Marshal(candle)
	if err != nil {
		logrus.Error("candle marshal err: ", err)
	}
	logrus.Debug("AddToGeneralCandleChnl: ", string(ab), " channels: ", channels)
	for _, v := range channels {
		suffix := ""
		env := os.Getenv("GO_ENV")
		if env == "demoEnv" {
			suffix = "-demoEnv"
		}
		vd := v + suffix
		_, err := redisCli.Publish(vd, string(ab)).Result()
		if err != nil {
			logrus.Error("err of ma7|ma30 add to redis2:", err, candle.From)
		}
	}
}

// setName := "candle" + period + "|" + instId + "|sortedSet"
// count: 倒推多少个周期开始拿数据
// from: 倒推的起始时间点
// ctype: candle或者maX
func (core *Core) GetRangeMaXSortedSet(setName string, count int, from time.Time) (*MaXList, error) {
	mxl := &MaXList{Count: count}
	ary1 := strings.Split(setName, "|")
	ary2 := []string{}
	period := ""
	ary2 = strings.Split(ary1[1], "candle")
	period = ary2[1]

	dui, err := core.PeriodToMinutes(period)
	if err != nil {
		return mxl, err
	}
	fromt := from.UnixMilli()
	froms := strconv.FormatInt(fromt, 10)
	sti := fromt - dui*int64(count)*60*1000
	sts := strconv.FormatInt(sti, 10)
	opt := redis.ZRangeBy{
		Min:   sts,
		Max:   froms,
		Count: int64(count),
	}
	ary := []string{}
	logrus.Debug("ZRevRangeByScore ", " setName:", setName, " froms:", froms, " sts:", sts)
	dura, err := core.GetExpiration(period)
	if err != nil {
		return mxl, err
	}
	// fmt.Println("GetExpiration dura: ", dura, " period: ", period)
	ot := time.Now().Add(dura * -1)
	oti := ot.UnixMilli()
	// fmt.Println(fmt.Sprint("GetExpiration zRemRangeByScore ", setName, " ", 0, " ", strconv.FormatInt(oti, 10)))
	cli := core.RedisLocalCli
	cli.LTrim(setName, 0, oti)
	cunt, _ := cli.ZRemRangeByScore(setName, "0", strconv.FormatInt(oti, 10)).Result()
	if cunt > 0 {
		logrus.Warning("移出过期的引用数量：", "setName:", setName, " cunt:", cunt)
	}
	ary, err = cli.ZRevRangeByScore(setName, opt).Result()
	if err != nil {
		logrus.Warning("GetRangeMaXSortedSet ZRevRangeByScore err: ", " setName: ", setName, ", opt:", opt, ", res:", ary, ", err:", err)
		return mxl, err
	}
	keyAry, err := cli.MGet(ary...).Result()
	if err != nil {
		logrus.Warning("GetRangeMaXSortedSet mget err: ", "setName:", setName, ", max:", opt.Max, ", min:", opt.Min, ", res:", keyAry, ", err:", err)
		return mxl, err
	}
	for _, str := range keyAry {
		if str == nil {
			continue
		}
		mx := MaX{}
		json.Unmarshal([]byte(str.(string)), &mx)
		curData := mx.Data
		if len(curData) == 0 {
			logrus.Warning("curData is null", str)
			continue
		}

		ts := curData[0].(float64)
		tm := time.UnixMilli(int64(ts))
		if tm.Sub(from) > 0 {
			break
		}
		mxl.List = append(mxl.List, &mx)
	}
	mxl.Count = count
	return mxl, nil
}

// 根据周期的文本内容，返回这代表多少个分钟
// 设置sortedSet的过期时间

func (cr *Core) GetExpiration(per string) (time.Duration, error) {
	if len(per) == 0 {
		erstr := fmt.Sprint("period没有设置")
		logrus.Warn(erstr)
		err := errors.New(erstr)
		return 0, err
	}
	exp, err := cr.PeriodToMinutes(per)
	length, _ := cr.Cfg.Config.Get("sortedSet").Get("length").Int64()
	logrus.Info("length of sortedSet: ", length)
	dur := time.Duration(exp*length) * time.Minute
	return dur, err
}

func (cr *Core) PeriodToLastTime(period string, from time.Time) (time.Time, error) {
	candles := cr.Cfg.Config.Get("softCandleSegmentList").MustArray()
	minutes := int64(0)
	tms := ""

	for _, v := range candles {
		cs := CandleSegment{}
		sv, _ := json.Marshal(v)
		json.Unmarshal(sv, &cs)
		if cs.Seg == period {
			mns, err := cr.PeriodToMinutes(period)
			if err != nil {
				continue
			}
			minutes = mns
			tms = cs.StartTime
		} else {
		}

	}

	tm, err := time.ParseInLocation("2006-01-02 15:04.000", tms, time.Local)
	if err != nil {
		return from, err
	}
	// from.Unix() % minutes
	frmi := from.UnixMilli()
	frmi = frmi - (frmi)%(60*1000)
	tmi := tm.UnixMilli()
	tmi = tmi - (tmi)%(60*1000)
	oms := int64(0)
	for i := int64(0); true; i++ {
		if tmi+i*minutes*60*1000 > frmi {
			break
		} else {
			oms = tmi + i*minutes*60*1000
			continue
		}
	}
	// return from, nil
	// tm :=
	om := time.UnixMilli(oms)
	// fmt.Println("PeriodToLastTime: period: ", period, " lastTime:", om.Format("2006-01-02 15:04:05.000"))
	return om, nil
}

// setName := "candle" + period + "|" + instId + "|sortedSet"
// count: 倒推多少个周期开始拿数据
// from: 倒推的起始时间点
// ctype: candle或者maX
func (core *Core) GetRangeCandleSortedSet(setName string, count int, from time.Time) (*CandleList, error) {
	// 初始化返回结果
	cdl := &CandleList{Count: count}

	// 解析period
	ary := strings.Split(setName, "|")
	if len(ary) < 1 {
		return nil, fmt.Errorf("invalid setName format: %s", setName)
	}
	period := strings.TrimPrefix(ary[0], "candle")

	// 获取period对应的分钟数
	durationMinutes, err := core.PeriodToMinutes(period)
	if err != nil {
		return nil, fmt.Errorf("failed to get period minutes: %w", err)
	}

	// 计算时间范围
	fromTs := from.UnixMilli()
	nowTs := time.Now().UnixMilli()
	if fromTs > nowTs*2 {
		return nil, fmt.Errorf("invalid from time: %v", from)
	}

	// 计算查询的起始时间
	startTs := fromTs - durationMinutes*int64(count)*60*1000

	// 清理过期数据
	if err := core.cleanExpiredData(setName, period); err != nil {
		logrus.Warnf("Failed to clean expired data: %v", err)
	}

	// 从Redis获取数据
	cli := core.RedisLocalCli
	opt := redis.ZRangeBy{
		Min:   strconv.FormatInt(startTs, 10),
		Max:   strconv.FormatInt(fromTs, 10),
		Count: int64(count),
	}

	logrus.Debugf("Querying Redis: set=%s, start=%v, end=%v", setName, time.UnixMilli(startTs), from)

	// 获取键列表
	keys, err := cli.ZRevRangeByScore(setName, opt).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to query Redis: %w", err)
	}

	if len(keys) == 0 {
		logrus.Warnf("No data found for set: %s between %v and %v", setName, time.UnixMilli(startTs), from)
		return cdl, nil
	}

	// 批量获取数据
	values, err := cli.MGet(keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get values from Redis: %w", err)
	}

	// 解析数据
	for _, val := range values {
		if val == nil {
			continue
		}

		var candle Candle
		if err := json.Unmarshal([]byte(val.(string)), &candle); err != nil {
			logrus.Warnf("Failed to unmarshal candle data: %v", err)
			continue
		}

		// 检查时间是否在范围内
		candleTime := time.UnixMilli(ToInt64(candle.Data[0]))
		if candleTime.After(from) {
			break
		}

		cdl.List = append(cdl.List, &candle)
	}

	return cdl, nil
}

// cleanExpiredData 清理过期的数据
func (core *Core) cleanExpiredData(setName, period string) error {
	expiration, err := core.GetExpiration(period)
	if err != nil {
		return err
	}

	cli := core.RedisLocalCli
	expirationTime := time.Now().Add(-expiration)
	expirationTs := strconv.FormatInt(expirationTime.UnixMilli(), 10)

	// 清理过期数据
	if count, err := cli.ZRemRangeByScore(setName, "0", expirationTs).Result(); err != nil {
		return err
	} else if count > 0 {
		logrus.Infof("Cleaned %d expired records from %s", count, setName)
	}

	return nil
}

func (cr *Core) GetCoasterFromPlate(instID string, period string) (Coaster, error) {
	// cr.Mu.Lock()
	// defer cr.Mu.Unlock()
	pl, ok := cr.PlateMap[instID]
	if !ok {
		err := errors.New("instID period not found : " + instID + " " + period)
		return *new(Coaster), err
	}
	co := pl.CoasterMap["period"+period]

	return co, nil
}
