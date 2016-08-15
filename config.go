package main

import (
	"encoding/json"
	"io/ioutil"
	"time"
)

var Conf *Config = &Config{}

type Config struct {
	Name     string
	Realname string
	Server   string
	Refetch  time.Duration
	// string = channel name
	Channels map[string]*struct {
		Feeds []string // http list
		Ops   []string // ident list
	}
}

func init() {
	f, err := ioutil.ReadFile("config.json")
	Check(err)
	Check(json.Unmarshal(f, Conf))
}
