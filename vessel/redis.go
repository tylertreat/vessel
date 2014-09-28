package vessel

import (
	"encoding/json"
	"sync"

	"github.com/garyburd/redigo/redis"
)

type redisPersister struct {
	redis.Conn
	sync.RWMutex
}

func NewPersister() (Persister, error) {
	c, err := redis.Dial("tcp", ":6379")
	if err != nil {
		return nil, err
	}
	return &redisPersister{c, sync.RWMutex{}}, nil
}

func (r *redisPersister) Persist(id string, result *result) error {
	r.Lock()
	defer r.Unlock()

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = r.Do("SET", id, resultJSON)
	return err
}

func (r *redisPersister) Get(id string) (*result, error) {
	r.RLock()
	defer r.RUnlock()

	resultJSON, err := redis.String(r.Do("GET", id))
	if err != nil {
		return nil, err
	}

	var result result
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		return nil, err
	}

	return &result, nil
}
