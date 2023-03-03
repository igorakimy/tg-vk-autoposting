package main

import (
	"errors"
	"github.com/spf13/viper"
	"log"
)

type Config struct {
	App      App      `yaml:"app"`
	Services Services `yaml:"services"`
}

type App struct {
	RSS        RSS        `yaml:"rss"`
	Schedule   Schedule   `yaml:"schedule"`
	CustomPost CustomPost `yaml:"customPost"`
}

type RSS struct {
	parseTimeout int `yaml:"parseTimeout"`
}

type CustomPost struct {
	Message string `yaml:"message,omitempty"`
	Files   string `yaml:"files"`
}

type Services struct {
	Youtube   Youtube   `yaml:"youtube"`
	Telegram  Telegram  `yaml:"telegram"`
	Vkontakte Vkontakte `yaml:"vkontakte"`
}

type Youtube struct {
	ChannelID string `yaml:"channelId"`
}

type Telegram struct {
	Token     string `yaml:"token"`
	ChannelID string `yaml:"channelId"`
	PostTitle string `yaml:"postTitle"`
}

type Vkontakte struct {
	Token     string `yaml:"token"`
	GroupID   int    `yaml:"groupID"`
	PostTitle string `yaml:"postTitle"`
}

type Schedule struct {
	Every string `yaml:"every"`
	Day   int    `yaml:"day"`
	Time  string `yaml:"time"`
}

func LoadConfig(filename string) (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigFile(filename)
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AutomaticEnv()
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil, errors.New("config file not found")
		}
		return nil, err
	}
	return v, nil
}

func ParseConfig(v *viper.Viper) (*Config, error) {
	var c Config
	err := v.Unmarshal(&c)
	if err != nil {
		log.Printf("Unable to decode into struct, %v", err)
		return nil, err
	}
	return &c, nil
}
