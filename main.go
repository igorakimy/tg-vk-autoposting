package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/SevereCloud/vksdk/v2/api"
	"github.com/SevereCloud/vksdk/v2/api/params"
	"github.com/SevereCloud/vksdk/v2/longpoll-bot"
	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/mmcdole/gofeed"
)

func main() {
	// Загрузка конфигурации
	cfgPath := "./config.yml"
	cfgFile, err := LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("Ошибка при загрузке файла конфигурации: %v", err)
	}
	cfg, err := ParseConfig(cfgFile)
	if err != nil {
		log.Fatalf("Ошибка при анализе файла конфигурации: %v", err)
	}
	log.Println("Конфигурация загружена")

	// Подключение к бд
	database, err := NewDB("database.db")
	if err != nil {
		log.Fatalf("Ошибка cоединенния с базой данных: %v", err)
	}
	log.Println("Соединение с БД установлено")
	conn := database.GetConn()

	// Создать таблицу публикаций
	err = database.CreatePostsTable()
	if err != nil {
		log.Fatalf("Ошибка создания таблицы: %v", err)
	}
	defer conn.Close()

	// Создать подключение к телеграм боту
	bot := createNewTelegramBot(cfg.Services.Telegram.Token)

	// Создать подключение к апи Вконтакте
	vk := api.NewVK(cfg.Services.Vkontakte.Token)

	// Создать канал, по которому будут приходить новые записи
	nChannel := make(chan []Post)
	defer close(nChannel)

	// Запустить Telegram бота
	go runRecoverableTask(func() { runTelegramBot(bot, cfg) })
	// Запустить приложение VK
	go runRecoverableTask(func() { runVkontakteApp(vk, cfg) })
	// Запустить парсинг RSS Youtube
	go runRecoverableTask(func() { runRssParser(cfg, database, nChannel) })
	// Запустить планировщик отправки сообщений VK
	go runRecoverableTask(func() { startScheduling(cfg, vk, sendFilesToVkontakte) })

	// Ожидаем получения новых записей, полученных по RSS
	for {
		select {
		case posts := <-nChannel:
			// Отправить посты о новых видео в Telegram
			for _, post := range posts {
				sendPostToTelegram(bot, cfg, &post)
			}
			// Отправить посты о новых видео в VK
			for _, post := range posts {
				sendPostToVkontakte(vk, cfg, &post)
				if len(posts) > 1 {
					time.Sleep(5 * time.Second)
				}
			}
		default:
		}
	}
}

// createNewTelegramBot создает и возвращает новый экземпляр tgbotapi.BotAPI.
func createNewTelegramBot(token string) *tgbotapi.BotAPI {
	bot, err := tgbotapi.NewBotAPI(token)
	defer func() {
		if err := recover(); err != nil {
			log.Println("Соединение с интернетом отсутствует. Переподключение...")
			time.Sleep(time.Second * 10)
			main()
		}
	}()
	if err != nil {
		log.Panicf("Ошибка подключения к телеграм: %v", err)
	}
	return bot
}

// runRssParser парсит RSS Youtube и отправляет новые записи о
// появившихся видео в соответствующий канал
func runRssParser(cfg *Config, db *Database, nChannel chan []Post) {
	// defer recoverApp()
	for {
		posts := make(map[string]Post)
		var postIDs []string
		fp := gofeed.NewParser()
		// Парсим RSS
		feed, err := fp.ParseURL(fmt.Sprintf(
			"https://www.youtube.com/feeds/videos.xml?channel_id=%s",
			cfg.Services.Youtube.ChannelID),
		)
		if err != nil {
			log.Panicf("Ошибка при попытке парсинга RSS: %v", err)
		}
		// Заполняем массив постов
		for _, item := range feed.Items {
			post := Post{
				VideoID:     item.Extensions["yt"]["videoId"][0].Value,
				Preview:     item.Extensions["media"]["group"][0].Children["thumbnail"][0].Attrs["url"],
				Title:       item.Title,
				Description: item.Extensions["media"]["group"][0].Children["description"][0].Value,
				PublishedAt: item.Published,
			}
			posts[post.VideoID] = post
			postIDs = append(postIDs, post.VideoID)
		}
		// Получить из бд записи по ID video из RSS
		postsList, err := db.GetPostsByIds(postIDs...)
		if err != nil {
			log.Panicf("Ошибка получения нескольких видео по ID: %v", err)
		}

		// Удалить те записи из результирующего списка, которые уже есть в бд
		for _, p := range postsList {
			if Contains(postIDs, p.VideoID) {
				delete(posts, p.VideoID)
				continue
			}
		}

		// Сформировать список добавляемых записей и сохранить их в бд
		var createdPosts []Post
		for _, post := range posts {
			_, err := db.CreateNewPost(post)
			if err != nil {
				log.Println(err)
			}
			createdPosts = append(createdPosts, post)
		}

		// Отправить сохраненные новые записи в канал
		nChannel <- createdPosts

		time.Sleep(time.Duration(cfg.App.RSS.parseTimeout) * time.Second)
	}

}

// runVkontakteApp запускает полинг приложения в VK
func runVkontakteApp(vk *api.VK, cfg *Config) {
	// defer recoverApp()
	lp, err := longpoll.NewLongPoll(vk, cfg.Services.Vkontakte.GroupID)
	if err != nil {
		log.Panicf("Ошибка поллинга Вконтакте: %v", err)
	}
	log.Println("Начат поллинг приложения Вконтакте")
	if err := lp.Run(); err != nil {
		log.Panicf("Поллинг Вконтакте остановлен: %v", err)
	}
}

// runTelegramBot запускает полинг Telegram бота
func runTelegramBot(bot *tgbotapi.BotAPI, cfg *Config) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)
	log.Println("Начат поллинг Телеграм бота")
	for range updates {
		continue
	}
}

// sendPostToTelegram отправляет пост в телеграм
func sendPostToTelegram(bot *tgbotapi.BotAPI, cfg *Config, post *Post) {
	msg := tgbotapi.NewMessageToChannel(
		cfg.Services.Telegram.ChannelID,
		fmt.Sprintf(
			"%s\n%s\n%s\n%s",
			cfg.Services.Telegram.PostTitle,
			fmt.Sprintf("https://youtu.be/%s", post.VideoID),
			post.Title,
			post.Description,
		),
	)
	if _, err := bot.Send(msg); err != nil {
		log.Panicf("Пост в Телеграм не был отправлен: %v", err)
	}
	log.Println("Пост в Телеграм отправлен")
}

// sendPostToVkontakte отправляет пост в VK
func sendPostToVkontakte(vk *api.VK, cfg *Config, post *Post) {
	b := params.NewWallPostBuilder()
	b.OwnerID(-cfg.Services.Vkontakte.GroupID)
	b.Message(fmt.Sprintf(
		"%s\n%s\n%s",
		cfg.Services.Vkontakte.PostTitle,
		post.Title,
		post.Description,
	))
	b.Attachments(fmt.Sprintf("https://youtu.be/%s", post.VideoID))
	_, err := vk.WallPost(b.Params)
	if err != nil {
		log.Panicf("Пост во Вконтакте не был отправлен: %v", err)
	}
	log.Println("Пост во Вконтакте отправлен")
}

func sendFilesToVkontakte(vk *api.VK, cfg *Config) {
	log.Println("Отправка пользовательского поста")
	var photos = make([]api.PhotosSaveWallPhotoResponse, 0)

	// Загружаем файлы на сервера VK
	if len(cfg.App.CustomPost.Files) > 0 {
		for _, file := range strings.Split(cfg.App.CustomPost.Files, ",") {
			f, err := os.Open(file)
			if err != nil {
				log.Fatalf("Файл не найден: %v", err)
			}
			photo, err := vk.UploadGroupWallPhoto(cfg.Services.Vkontakte.GroupID, f)
			if err != nil {
				log.Fatalf("Ошибка загрузки файла на сервер: %v", err)
			}
			photos = append(photos, photo)
		}
	}

	b := params.NewWallPostBuilder()
	var attachments []string
	b.OwnerID(-cfg.Services.Vkontakte.GroupID)

	if len(photos) > 0 {
		for _, photo := range photos {
			attachments = append(attachments, fmt.Sprintf("photo%d_%d", photo[0].OwnerID, photo[0].ID))
		}
		// Прикрепляем файлы к посту
		b.Attachments(strings.Join(attachments, ","))
	}

	// Прикрепляем сообщение, если оно имеется
	if cfg.App.CustomPost.Message != "" {
		b.Message(cfg.App.CustomPost.Message)
	}

	// Отправляем пост
	_, err := vk.WallPost(b.Params)
	if err != nil {
		log.Panicf("Пост во Вконтакте не был отправлен: %v", err)
	}
	log.Println("Пост во Вконтакте отправлен")
}

// runRecoverableTask выполняет функцию и в случае, если она запаникует,
// выполняет ее повторно, через определенный таймаут.
func runRecoverableTask(task func()) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Соединение с интернетом отсутствует. Переподключение...")
			time.Sleep(time.Second * 10)
			go runRecoverableTask(task)
		}
	}()
	task()
}
