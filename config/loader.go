package config

import (
	"github.com/ghodss/yaml"
	"io/ioutil"
	"os"
	"path"
)

type Config struct {
	ClientID     string `yaml:"clientId"`
	ClientSecret string `yaml:"clientSecret"`
}

func Load() (string, string, error) {
	p := path.Join(HomeDir(), ".spot", "config.yaml")
	config := Config{}
	b, err := ioutil.ReadFile(p)
	if err != nil {
		return "", "", err
	}
	err = yaml.Unmarshal(b, &config)
	if err != nil {
		return "", "", err
	}
	return config.ClientID, config.ClientSecret, nil
}

func HomeDir() string {
	return os.Getenv("HOME")
}
