package config

import (
	// "fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	simple "github.com/bitly/go-simplejson"
	logrus "github.com/sirupsen/logrus"
)

type MyConfig struct {
	Env                    string `json:"env"`
	Config                 *simple.Json
	CandleDimentions       []string          `json:"candleDimentions"`
	RedisConf              *RedisConfig      `json:"redis"`
	CredentialReadOnlyConf *CredentialConfig `json:"credential"`
	CredentialMutableConf  *CredentialConfig `json:"credentialMutable"`
	ConnectConf            *ConnectConfig    `json:"connect"`
	// ThreadsConf    *ThreadsConfig    `json:"threads"`
}

type RedisConfig struct {
	Url      string `json:"url"`
	Password string `json:"password"`
	Index    int    `json:"index"`
}

type CredentialConfig struct {
	SecretKey          string `json:"secretKey"`
	BaseUrl            string `json:"baseUrl"`
	OkAccessKey        string `json:"okAccessKey"`
	OkAccessPassphrase string `json:"okAccessPassphrase"`
}

type ConnectConfig struct {
	LoginSubUrl      string `json:"loginSubUrl"`
	WsPrivateBaseUrl string `json:"wsPrivateBaseUrl"`
	WsPublicBaseUrl  string `json:"wsPublicBaseUrl"`
	RestBaseUrl      string `json:"restBaseUrl"`
}
type ThreadsConfig struct {
	MaxLenTickerStream int `json:"maxLenTickerStream"`
	MaxCandles         int `json:"maxCandles"`
	AsyncChannels      int `json:"asyncChannels"`
	MaxTickers         int `json:"maxTickers"`
	RestPeriod         int `json:"restPeriod"`
	WaitWs             int `json:"waitWs"`
}

func (cfg MyConfig) Init() (MyConfig, error) {
	env := os.Getenv("GO_ENV")
	arystr := os.Getenv("TUNAS_CANDLESDIMENTIONS")
	ary := strings.Split(arystr, "|")
	cfg.CandleDimentions = ary
	jsonStr, err := ioutil.ReadFile("/go/json/basicConfig.json")
	if err != nil {
		jsonStr, err = ioutil.ReadFile("configs/basicConfig.json")
		if err != nil {
			logrus.Error("err2:", err.Error())
			return cfg, err
		}
		cfg.Config, err = simple.NewJson([]byte(jsonStr))
		if err != nil {
			logrus.Error("err2:", err.Error())
			return cfg, err
		}
		cfg.Env = env
	}
	cfg.Config = cfg.Config.Get(env)

	ru, err := cfg.Config.Get("redis").Get("url").String()
	rp, _ := cfg.Config.Get("redis").Get("password").String()
	ri, _ := cfg.Config.Get("redis").Get("index").Int()
	redisConf := RedisConfig{
		Url:      ru,
		Password: rp,
		Index:    ri,
	}
	// fmt.Println("cfg: ", cfg)
	cfg.RedisConf = &redisConf
	ls, _ := cfg.Config.Get("connect").Get("loginSubUrl").String()
	wsPub, _ := cfg.Config.Get("connect").Get("wsPrivateBaseUrl").String()
	wsPri, _ := cfg.Config.Get("connect").Get("wsPublicBaseUrl").String()
	restBu, _ := cfg.Config.Get("connect").Get("restBaseUrl").String()
	connectConfig := ConnectConfig{
		LoginSubUrl:      ls,
		WsPublicBaseUrl:  wsPub,
		WsPrivateBaseUrl: wsPri,
		RestBaseUrl:      restBu,
	}
	cfg.ConnectConf = &connectConfig
	return cfg, nil
}

func (cfg *MyConfig) GetConfigJson(arr []string) *simple.Json {
	env := os.Getenv("GO_ENV")
	logrus.Info("env: ", env)
	cfg.Env = env

	json, err := ioutil.ReadFile("/go/json/basicConfig.json")

	if err != nil {
		json, err = ioutil.ReadFile("configs/basicConfig.json")
		if err != nil {
			log.Panic("read config error: ", err.Error())
		}
	}
	if err != nil {
		logrus.Error("read file err: ", err)
	}
	rjson, err := simple.NewJson(json)
	if err != nil {
		logrus.Error("newJson err: ", err)
	}
	for _, s := range arr {
		rjson = rjson.Get(s)
		// fmt.Println(s, ": ", rjson)
	}
	return rjson
}
