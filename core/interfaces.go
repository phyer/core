package core

import (
	"github.com/go-redis/redis"
	"time"
)

// 领域服务接口
type CandleManager interface {
	GetCandles(instID string, period string) ([]*Candle, error)
	SaveCandle(candle *Candle) error
}

// 基础设施接口
type RedisService interface {
	GetClient(options *redis.Options) (*redis.Client, error)
	Ping(client *redis.Client) error
}

type HTTPRequester interface {
	Get(url string) ([]byte, error)
	Post(url string, body []byte) ([]byte, error)
}

// 领域事件接口
type EventPublisher interface {
	Publish(topic string, message interface{}) error
	Subscribe(topic string, handler func(message []byte)) error
}
