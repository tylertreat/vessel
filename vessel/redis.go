package vessel

import (
	"encoding/json"
	"strconv"
	"sync"

	"github.com/garyburd/redigo/redis"
)

type redisPersister struct {
	conn redis.Conn
	mu   sync.RWMutex
}

func NewPersister() Persister {
	return &redisPersister{mu: sync.RWMutex{}}
}

func (r *redisPersister) Prepare() error {
	c, err := redis.Dial("tcp", ":6379")
	if err != nil {
		return err
	}
	r.conn = c
	return nil
}

func (r *redisPersister) SaveResult(id string, result *result) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = r.conn.Do("SET", id, resultJSON)
	return err
}

func (r *redisPersister) SaveMessage(channel string, message *message) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	resultJSON, err := json.Marshal(message)
	if err != nil {
		return err
	}
	_, err = r.conn.Do("ZADD", channel, message.Timestamp, resultJSON)
	return err
}

func (r *redisPersister) GetResult(id string) (*result, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	resultJSON, err := redis.String(r.conn.Do("GET", id))
	if err != nil {
		return nil, err
	}

	var result result
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (r *redisPersister) GetMessages(channel string, since int64) ([]*message, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	min := "(" + strconv.FormatInt(since, 10)
	max := "+inf"
	messages, err := redis.Strings(r.conn.Do("ZRANGEBYSCORE", channel, min, max))
	if err != nil {
		return nil, err
	}

	m := make([]*message, len(messages))
	for i, msg := range messages {
		if err := json.Unmarshal([]byte(msg), &m[i]); err != nil {
			return nil, err
		}
	}

	return m, nil
}
