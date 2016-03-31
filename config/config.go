// das2go/config - configuration sub-package for das2go
//
// Copyright (c) 2015-2016 - Valentin Kuznetsov <vkuznet AT gmail dot com>
//
package config

import (
	"encoding/json"
	"log"
	"os"
	"strings"
)

type Configuration struct {
	Uri string
}

// global config object
var _config Configuration

func ParseConfig() Configuration {
	var fname string
	for _, item := range os.Environ() {
		value := strings.Split(item, "=")
		if value[0] == "DAS_CONFIG" {
			fname = value[1]
			break
		}
	}
	if fname == "" {
		panic("DAS_CONFIG environment variable is not set")
	}
	log.Println("DAS_CONFIG", fname)
	file, _ := os.Open(fname)
	decoder := json.NewDecoder(file)
	conf := Configuration{}
	err := decoder.Decode(&conf)
	if err != nil {
		panic(err)
	}
	log.Println("DAS configuration", conf)
	return conf
}

func Uri() string {
	if _config.Uri == "" {
		_config = ParseConfig()
	}
	return _config.Uri
}