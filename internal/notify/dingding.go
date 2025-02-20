package notify

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	logrus "github.com/sirupsen/logrus"
)

const DingdingMsgType_Markdown = "markdown"

type DingdingMsg struct {
	RobotName  string
	Topic      string
	Ctype      string
	Content    string
	AtAll      bool
	UniqueCode string
}

func (dd *DingdingMsg) AddItemListGrp(title string, level int, list []string) error {
	pre := ""
	if level < 1 {
		err := errors.New("level is not allow " + strconv.FormatInt(int64(level), 10))
		return err
	}
	for i := level; i > 0; i-- {
		pre = pre + "#"
	}
	title = pre + " " + title
	dd.Content += "\n"
	dd.Content += title
	dd.Content += "\n"
	for _, v := range list {
		dd.Content += v
		dd.Content += "\n"
	}
	return nil
}

func MakeSign(baseUrl string, secure string, token string, tm time.Time) string {
	tsi := tm.UnixMilli()
	tsi = tsi - tsi%60000
	tss := strconv.FormatInt(tsi, 10)
	sign := tss + "\n" + secure
	sign = ComputeHmac256(secure, sign)
	sign = url.QueryEscape(sign)

	url := baseUrl + "?access_token=" + token + "&timestamp=" + tss + "&sign=" + sign
	return url
}

func (dd *DingdingMsg) MakeContent() []byte {
	ctn := map[string]interface{}{}
	ctn["msgtype"] = dd.Ctype
	if dd.Ctype == DingdingMsgType_Markdown {
		md := map[string]interface{}{}
		md["title"] = dd.Topic
		md["text"] = dd.Content
		md["isAtAll"] = dd.AtAll
		ctn[DingdingMsgType_Markdown] = md
	}
	btn, _ := json.Marshal(ctn)
	return btn
}

func ComputeHmac256(secret string, message string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func PostHeader(url string, msg []byte, headers map[string]string) (string, error) {
	client := &http.Client{}

	req, err := http.NewRequest("POST", url, strings.NewReader(string(msg)))
	if err != nil {
		return "", err
	}
	for key, header := range headers {
		req.Header.Set(key, header)
	}
	resp, err := client.Do(req)
	if err != nil {
		logrus.Warn("postHeader err: ", err)
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
func (dd *DingdingMsg) PostToRobot(rbt string, cr *Core) (string, error) {
	baseUrl, _ := cr.Cfg.Config.Get("dingding").Get("baseUrl").String()
	secret, _ := cr.Cfg.Config.Get("dingding").Get("robots").Get(rbt).Get("secret").String()
	token, _ := cr.Cfg.Config.Get("dingding").Get("robots").Get(rbt).Get("accessToken").String()
	cli := cr.RedisLocalCli

	if len(dd.UniqueCode) > 0 {
		unique := "ddPostUnique|" + dd.UniqueCode
		exists, _ := cli.Exists(unique).Result()
		if exists == 1 {
			err := errors.New("20分钟内已经投递过了，不再重复")
			return "", err
		}
		cli.Set(unique, 1, 20*time.Minute).Result()
	}
	nw := time.Now()
	url := MakeSign(baseUrl, secret, token, nw)

	ctn := dd.MakeContent()
	headers := make(map[string]string)
	headers["Content-Type"] = "application/json;charset=utf-8"
	res, err := PostHeader(url, ctn, headers)
	logrus.Warn("postToRobot res:", res, string(ctn))
	return res, err
}
