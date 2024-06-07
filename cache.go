package main

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

var jobsCache = CreateCache[[]map[string]interface{}](10*time.Minute, true,
	func() ([]map[string]interface{}, error) {
		jobs := make([]map[string]interface{}, 0)
		files, err := os.ReadDir(TheConfig.Output)
		if err != nil {
			return jobs, err
		}
		for _, file := range files {
			job := populate(file.Name())
			if job != nil {
				jobs = append(jobs, job)
			}
		}
		return jobs, nil
	},
)

type Cache[T any] struct {
	Data          T
	Marshalled    string
	LastFetched   time.Time
	TTL           time.Duration
	FetchMethod   func() (T, error)
	EnableMarshal bool
	mutex         sync.RWMutex
}

type MapCache[T any] struct {
	mutex       sync.Mutex
	Data        map[string]T
	FetchMethod func(key string) (T, error)
	HasExpired  func(value T) bool
}

func (c *MapCache[T]) Get(key string) (T, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	_, ok := c.Data[key]
	if !ok || c.HasExpired(c.Data[key]) {
		log.Infof("Cache expired %s, fetching new data", key)
		data, err := c.FetchMethod(key)
		if err != nil {
			log.Errorf("Error fetching data: %v", err)
			return c.Data[key], err
		}
		c.Data[key] = data
	}
	return c.Data[key], nil
}

func (c *Cache[T]) Get() (T, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.LastFetched.Add(c.TTL).Before(time.Now()) {
		log.Info("Cache expired, fetching new data")
		data, err := c.FetchMethod()
		if err != nil {
			return c.Data, err
		}
		c.Data = data
		c.LastFetched = time.Now()
		if c.EnableMarshal {
			s, err := json.Marshal(c.Data)
			if err != nil {
				return c.Data, err
			}
			c.Marshalled = string(s)
		}
	}
	return c.Data, nil
}

func (c *Cache[T]) BypassGet() string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	go func() {
		_, err := c.Get()
		if err != nil {
			log.Errorf("Error fetching data: %v", err)
		}
	}()
	return c.Marshalled
}

func CreateCache[T any](ttl time.Duration, enabledMarshal bool, fetchMethod func() (T, error)) *Cache[T] {
	cache := &Cache[T]{
		TTL:           ttl,
		FetchMethod:   fetchMethod,
		EnableMarshal: enabledMarshal,
	}
	return cache
}

func CreateMapCache[T any](fetchMethod func(key string) (T, error),
	hasExpired func(value T) bool) *MapCache[T] {
	cache := &MapCache[T]{
		FetchMethod: fetchMethod,
		HasExpired:  hasExpired,
		Data:        make(map[string]T),
	}
	return cache
}
