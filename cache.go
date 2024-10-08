package main

import (
	"Sparkle/config"
	"Sparkle/discord"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

func populate(path string) *JobStripped {
	content, err := os.ReadFile(OutputJoin(path, JobFile))
	if err != nil {
		return nil
	}
	job := &JobStripped{}
	err = json.Unmarshal(content, job)
	if err != nil {
		return nil
	}
	fileSizes := make(map[string]int64)
	files, err := os.ReadDir(OutputJoin(path))
	if err != nil {
		return nil
	}
	for _, file := range files {
		stat, err := os.Stat(OutputJoin(path, file.Name()))
		if err == nil {
			fileSizes[file.Name()] = stat.Size()
			if time.Unix(job.JobModTime, 0).Before(stat.ModTime()) {
				job.JobModTime = stat.ModTime().Unix()
			}
		}
	}
	job.Files = fileSizes
	return job
}

var jobsCache = CreateCache[[]*JobStripped](15*time.Minute, true,
	func() ([]*JobStripped, error) {
		jobs := make([]*JobStripped, 0)
		files, err := os.ReadDir(config.TheConfig.Output)
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

func getTitleId(title string) string {
	parts := strings.Split(title, " - ")
	se := ""

	for i, part := range parts {
		matched, _ := regexp.MatchString(`S\d{2}E\d{2}`, part)
		if matched {
			se = part
			// seTitle = strings.Join(parts[i+1:], " - ")
			title = strings.Join(parts[:i], " - ")
			break
		}
	}

	titleId := regexp.MustCompile(`[^a-z0-9]`).ReplaceAllString(strings.ToLower(title), "")
	return titleId + se
}

type Cache[T any] struct {
	Data            T
	Marshalled      string
	LastFetched     time.Time
	TTL             time.Duration
	FetchMethod     func() (T, error)
	EnableMarshal   bool
	mutex           sync.RWMutex
	marshalledMutex sync.RWMutex
}

func (c *Cache[T]) Get(force bool) (T, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.LastFetched.Add(c.TTL).Before(time.Now()) || force {
		discord.Infof("Cache expired, fetching new data")
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
			c.marshalledMutex.Lock()
			defer c.marshalledMutex.Unlock()
			c.Marshalled = string(s)
		}
	}
	return c.Data, nil
}

func (c *Cache[T]) GetMarshalled() string {
	c.marshalledMutex.RLock()
	defer c.marshalledMutex.RUnlock()
	go func() {
		_, err := c.Get(false)
		if err != nil {
			discord.Errorf("Error fetching data: %v", err)
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
