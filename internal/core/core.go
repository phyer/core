package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis"
	// "github.com/phyer/texus/private"
	"github.com/phyer/core/analysis"
	"github.com/phyer/core/config"
	"github.com/phyer/core/models"
	"github.com/phyer/v5sdkgo/rest"
	logrus "github.com/sirupsen/logrus"
)

type Core struct {
	Env                  string
	Cfg                  *config.MyConfig
	RedisLocalCli        *redis.Client
	RedisRemoteCli       *redis.Client
	FluentBitUrl         string
	PlateMap             map[string]*models.Plate
	TrayMap              map[string]*models.Tray
	CoasterMd5SyncMap    sync.Map
	Mu                   *sync.Mutex
	Mu1                  *sync.Mutex
	Waity                *sync.WaitGroup
	CandlesProcessChan   chan *models.Candle
	MaXProcessChan       chan *MaX
	RsiProcessChan       chan *Rsi
	StockRsiProcessChan  chan *StockRsi
	TickerInforocessChan chan *TickerInfo
	CoasterChan          chan *CoasterInfo
	analysis.SeriesChan           chan *analysis.SeriesInfo  // to be init
	SegmentItemChan      chan *SegmentItem // to be init
	MakeMaXsChan         chan *Candle
	ShearForceGrpChan    chan *ShearForceGrp // to be init
	InvokeRestQueueChan  chan *RestQueue
	RedisLocal2Cli       *redis.Client
	RestQueueChan        chan *RestQueue
	RestQueue
	WriteLogChan chan *WriteLog
}

func (cre *coreCore) GetCandlesWithRest(instId string, kidx int, dura time.Duration, maxCandles int) error {
	ary := []string{}

	wsary := cre.Cfg.CandleDimentions
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
					coreRedisLocalCli.LPush("restQueue", js)
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
	coreEnv = os.Getenv("GO_ENV")
	gitBranch := os.Getenv("gitBranchName")
	commitID := os.Getenv("gitCommitID")

	logrus.Info("当前环境: ", coreEnv)
	logrus.Info("gitBranch: ", gitBranch)
	logrus.Info("gitCommitID: ", commitID)
	cfg := MyConfig{}
	cfg, _ = cfg.Init()
	coreCfg = &cfg
	cli, err := coreGetRedisLocalCli()
	coreRedisLocalCli = cli
	coreRestQueueChan = make(chan *RestQueue)
	coreWriteLogChan = make(chan *WriteLog)
	// 跟订单有关的都关掉
	// coreOrderChan = make(chan *private.Order)
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
	ru := coreCfg.RedisConf.Url
	rp := coreCfg.RedisConf.Password
	ri := coreCfg.RedisConf.Index
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
	ru := coreCfg.RedisConf.Url
	rp := coreCfg.RedisConf.Password
	ri := coreCfg.RedisConf.Index
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
	rsp, err := coreRestInvoke("/api/v5/market/tickers?instType=SPOT", rest.GET)
	return rsp, err
}

// 这些跟 订单有关，都关掉
//
//	func (core *Core) GetBalances() (*rest.RESTAPIResult, error) {
//		// TODO 临时用了两个实现，restInvoke，复用原来的会有bug，不知道是谁的bug
//		rsp, err := coreRestInvoke2("/api/v5/account/balance", rest.GET, nil)
//		return rsp, err
//	}
//
//	func (core *Core) GetLivingOrderList() ([]*private.Order, error) {
//		// TODO 临时用了两个实现，restInvoke，复用原来的会有bug，不知道是谁的bug
//		params := make(map[string]interface{})
//		data, err := coreRestInvoke2("/api/v5/trade/orders-pending", rest.GET, &params)
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
// 		redisCli := coreRedisLocalCli
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
// 	redisCli := coreRedisLocalCli
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
// 			redisCli := coreRedisLocalCli
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
	restUrl, _ := coreCfg.Config.Get("connect").Get("restBaseUrl").String()
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
	restUrl, _ := coreCfg.Config.Get("connect").Get("restBaseUrl").String()
	//ep, method, uri string, param *map[string]interface{}
	rest := rest.NewRESTAPI(restUrl, method, subUrl, nil)
	key, _ := coreCfg.Config.Get("credentialReadOnly").Get("okAccessKey").String()
	secure, _ := coreCfg.Config.Get("credentialReadOnly").Get("secretKey").String()
	pass, _ := coreCfg.Config.Get("credentialReadOnly").Get("okAccessPassphrase").String()
	isDemo := false
	if coreEnv == "demoEnv" {
		isDemo = true
	}
	rest.SetSimulate(isDemo).SetAPIKey(key, secure, pass)
	response, err := rest.Run(context.Background())
	if err != nil {
		logrus.Error("restInvoke1 err:", subUrl, err)
	}
	return response, err
}
// 保证同一个 period, keyName ，在一个周期里，SaveToSortSet只会被执行一次
func (core *core.Core) SaveUniKey(period string, keyName string, extt time.Duration, tsi int64) {

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

func (core *core.Core) findInSortSet(period string, keyName string, extt time.Duration, tsi int64) (bool, error) {
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
func (core *core.Core) SaveToSortSet(period string, keyName string, extt time.Duration, tsi int64) {
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
func (cr *core.Core) PeriodToMinutes(period string) (int64, error) {
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
func (core *core.Core) GetRangeKeyList(pattern string, from time.Time) ([]*simple.Json, error) {
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


// func (core *Core) RestInvoke2(subUrl string, method string, param *map[string]interface{}) (*rest.RESTAPIResult, error) {
// 	key, err1 := coreCfg.Config.Get("credentialReadOnly").Get("okAccessKey").String()
// 	secret, err2 := coreCfg.Config.Get("credentialReadOnly").Get("secretKey").String()
// 	pass, err3 := coreCfg.Config.Get("credentialReadOnly").Get("okAccessPassphrase").String()
// 	userId, err4 := coreCfg.Config.Get("connect").Get("userId").String()
// 	restUrl, err5 := coreCfg.Config.Get("connect").Get("restBaseUrl").String()
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
// 	if coreEnv == "demoEnv" {
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
// 	key, err1 := coreCfg.Config.Get("credentialMutable").Get("okAccessKey").String()
// 	secret, err2 := coreCfg.Config.Get("credentialMutable").Get("secretKey").String()
// 	pass, err3 := coreCfg.Config.Get("credentialMutable").Get("okAccessPassphrase").String()
// 	userId, err4 := coreCfg.Config.Get("connect").Get("userId").String()
// 	restUrl, err5 := coreCfg.Config.Get("connect").Get("restBaseUrl").String()
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
// 	if coreEnv == "demoEnv" {
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
	redisCli := coreRedisLocalCli
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

	// redisCli := coreRedisLocalCli
	myFocusList := coreCfg.Config.Get("focusList").MustArray()
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

	dui, err := corePeriodToMinutes(period)
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
	dura, err := coreGetExpiration(period)
	if err != nil {
		return mxl, err
	}
	// fmt.Println("GetExpiration dura: ", dura, " period: ", period)
	ot := time.Now().Add(dura * -1)
	oti := ot.UnixMilli()
	// fmt.Println(fmt.Sprint("GetExpiration zRemRangeByScore ", setName, " ", 0, " ", strconv.FormatInt(oti, 10)))
	cli := coreRedisLocalCli
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
	durationMinutes, err := corePeriodToMinutes(period)
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
	if err := corecleanExpiredData(setName, period); err != nil {
		logrus.Warnf("Failed to clean expired data: %v", err)
	}

	// 从Redis获取数据
	cli := coreRedisLocalCli
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
	expiration, err := coreGetExpiration(period)
	if err != nil {
		return err
	}

	cli := coreRedisLocalCli
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

func (core *Core) GetPixelanalysis.Series(instId string, period string) (analysis.Series, error) {
	srs := analysis.Series{}
	srName := instId + "|" + period + "|series"
	cli := coreRedisLocalCli
	srsStr, err := cli.Get(srName).Result()
	if err != nil {
		return *new(analysis.Series), err
	}
	err = json.Unmarshal([]byte(srsStr), &srs)
	if err != nil {
		return *new(analysis.Series), err
	}
	logrus.Info("sei:", srsStr)
	err = srs.Candleanalysis.Series.RecursiveBubbleS(srs.Candleanalysis.Series.Count, "asc")
	if err != nil {
		return *new(analysis.Series), err
	}
	// err = srs.Candleanalysis.Series.RecursiveBubbleX(srs.Candleanalysis.Series.Count, "asc")
	// if err != nil {
	// return nil, err
	// }
	err = srs.Ma7analysis.Series.RecursiveBubbleS(srs.Candleanalysis.Series.Count, "asc")
	if err != nil {
		return *new(analysis.Series), err
	}
	// err = srs.Ma7analysis.Series.RecursiveBubbleX(srs.Candleanalysis.Series.Count, "asc")
	// if err != nil {
	// return nil, err
	// }
	err = srs.Ma30analysis.Series.RecursiveBubbleS(srs.Candleanalysis.Series.Count, "asc")
	if err != nil {
		return *new(analysis.Series), err
	}
	// err = srs.Ma30analysis.Series.RecursiveBubbleX(srs.Candleanalysis.Series.Count, "asc")
	// if err != nil {
	// return nil, err
	// }
	return srs, nil
}
