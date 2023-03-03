package main

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"os"
)

type Database struct {
	conn *sqlx.DB
}

func NewDB(name string) (*Database, error) {
	f, err := os.Open(name)
	if os.IsNotExist(err) {
		f, err = os.Create(name)
		if err != nil {
			return nil, err
		}
	}

	conn, err := sqlx.Connect("sqlite3", f.Name())
	dbInst := &Database{
		conn: conn,
	}
	if err != nil {
		return nil, err
	}
	return dbInst, nil
}

func (db *Database) GetConn() *sqlx.DB {
	return db.conn
}

func (db *Database) CreatePostsTable() error {
	_, err := db.conn.Exec(createPostsTable)
	if err != nil {
		return err
	}
	return nil
}

func (db *Database) GetPostsByIds(ids ...string) ([]Post, error) {
	var posts []Post
	query, args, err := sqlx.In(getPostsByIDs, ids)
	query = db.conn.Rebind(query)
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		p := Post{}
		err := rows.Scan(
			&p.VideoID,
			&p.PublishedAt,
			&p.Title,
			&p.Description,
			&p.Status,
		)
		if err != nil {
			log.Println(err)
			continue
		}
		posts = append(posts, p)
	}
	return posts, nil
}

func (db *Database) CreateNewPost(post Post) (int, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return 0, err
	}
	res, err := tx.Exec(createNewPost,
		post.VideoID,
		post.PublishedAt,
		post.Title,
		post.Description,
		post.Status,
	)
	insertedID, _ := res.LastInsertId()
	err = tx.Commit()
	if err != nil {
		return 0, err
	}
	return int(insertedID), nil
}
