package core

import (
	// "crypto/sha256"
	// "encoding/hex"
	// "encoding/json"
	// "errors"
	// "fmt"
	// "math/rand"
	// "os"
	// "strconv"
	// "strings"
	"time"
	// simple "github.com/bitly/go-simplejson"
	// "github.com/go-redis/redis"
	// "github.com/phyer/texus/utils"
	// logrus "github.com/sirupsen/logrus"
)

type Rsi struct {
	Id         string `json:"_id"`
	core       *Core
	InstID     string    `json:"instID"`
	Period     string    `json:"period"`
	Timestamp  time.Time `json:"timeStamp"`
	LastUpdate time.Time `json:"lastUpdate"`
	RsiVol     float64   `json:"rsiVol"`
	Confirm    bool      `json:"confirm"`
}
type RsiList struct {
	Count          int    `json:"count,number"`
	LastUpdateTime int64  `json:"lastUpdateTime"`
	UpdateNickName string `json:"updateNickName"`
	List           []*Rsi `json:"list"`
}
