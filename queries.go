package main

var (
	createPostsTable = `
		CREATE TABLE IF NOT EXISTS posts(
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		video_id VARCHAR(50),
		published_at TIMESTAMP,
		title VARCHAR(255),
		description TEXT,
		status BOOL)`

	getPostsByIDs = `SELECT video_id, published_at, title, description, status FROM posts WHERE video_id IN (?)`

	createNewPost = `
		INSERT INTO posts(video_id, published_at, title, description, status)
		VALUES (?, ?, ?, ?, ?)`
)
