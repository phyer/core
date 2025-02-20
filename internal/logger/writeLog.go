package logger

import (
	"bytes"
	// "fmt"
	"net/http"
	"os"

	// "github.com/phyer/core/internal/core"
	logrus "github.com/sirupsen/logrus"
)

type WriteLog struct {
	Content []byte
	Tag     string
	Id      string
}

func (wg *WriteLog) Process(cr *core.Core) error {
	go func() {
		reqBody := bytes.NewBuffer(wg.Content)
		cr.Env = os.Getenv("GO_ENV")
		cr.FluentBitUrl = os.Getenv("TEXUS_FluentBitUrl")
		fullUrl := "http://" + cr.FluentBitUrl + "/" + wg.Tag
		res, err := http.Post(fullUrl, "application/json", reqBody)

		logrus.Warn("requested, response:", fullUrl, string(wg.Content), res)
		if err != nil {
			logrus.Error(err)
		}
	}()
	return nil
}
