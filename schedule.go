package main

import (
	"log"
	"time"
	_ "time/tzdata"

	"github.com/SevereCloud/vksdk/v2/api"
	"github.com/go-co-op/gocron"
)

func startScheduling(cfg *Config, vk *api.VK, task interface{}) {
	location, _ := time.LoadLocation("Europe/Moscow")
	s := gocron.NewScheduler(location)

	if cfg.App.Schedule.Every == "day" {
		s = s.Every(1).Day()
	} else if cfg.App.Schedule.Every == "week" {
		day := cfg.App.Schedule.Day
		switch day {
		case 1:
			s = s.Every(1).Monday()
		case 2:
			s = s.Every(1).Tuesday()
		case 3:
			s = s.Every(1).Wednesday()
		case 4:
			s = s.Every(1).Thursday()
		case 5:
			s = s.Every(1).Friday()
		case 6:
			s = s.Every(1).Saturday()
		case 7:
			s = s.Every(1).Sunday()
		default:
			log.Fatalf("Неверный день недели: %v", day)
		}
	}

	_, err := s.At(cfg.App.Schedule.Time).Do(task, vk, cfg)
	if err != nil {
		log.Fatalf("Ошибка выполнения отложенной задачи: %v\n", err)
	}
	s.StartAsync()
}
