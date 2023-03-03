package main

type Post struct {
	VideoID     string `json:"video_id"`
	Preview     string `json:"preview"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      bool   `json:"status"`
	PublishedAt string `json:"published_at"`
}
