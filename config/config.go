package config

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"time"
)

const (
	fileName = "config.json"

	//Seconds
	defaultTimeout = 60
	defaultDirName = "./"
)

type Config struct {
	UrlTemplate string        `json:"url_template"`
	Cameras     []string      `json:"camera_ids"`
	Timeout     time.Duration `json:"timeout"`
	CleanUp     bool          `json:"clean_up"`
	DirName     string        `json:"dir_name"`
}

func NewConfig() (*Config, error) {
	c := &Config{}
	if err := c.load(); err != nil {
		return nil, errors.New("Failed to load Config: " + err.Error())
	}
	return c, nil
}

func (c *Config) load() error {
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, c); err != nil {
		return err
	}
	return c.validate()
}

func (c *Config) validate() error {
	if c.UrlTemplate == "" {
		return errors.New("url_template is required")
	}
	if c.Cameras == nil || len(c.Cameras) == 0 {
		return errors.New("camera_ids is required")
	}
	if c.Timeout == 0 {
		c.Timeout = defaultTimeout
	}
	if c.DirName == "" {
		c.DirName = defaultDirName
	}
	return nil
}
