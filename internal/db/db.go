// internal/db/db.go
package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3" // Assuming you are using SQLite as an example
)

// DB represents the database connection
type DB struct {
	conn *sql.DB
}

type TrackInfo struct {
	Id       int64
	RFID     string
	Track    string
	Offset   int64
	Location string
}

// NewDB creates a new database connection
func NewDB(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	return &DB{conn: conn}, nil
}

// Close closes the database connection
func (d *DB) Close() error {
	return d.conn.Close()
}

// CreateTable creates a sample table in the database
func (d *DB) CreateTable() error {
	_, err := d.conn.Exec(`
		CREATE TABLE IF NOT EXISTS tracks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			rfid INTEGER NOT NULL,
			track TEXT NOT NULL,
			offset INTEGER,
			url TEXT,
			location TEXT
		);
	`)
	return err
}

// InsertUser inserts a new user into the database
func (d *DB) InsertTrackURL(rfid int64, track string, offset int, url string, location string) error {
	_, err := d.conn.Exec("INSERT INTO tracks (rfid,track, offset, url, location) VALUES (?,?,?,?,?)", rfid, track, offset, url, location)
	return err
}

func (d *DB) GetTrackByRFID(rfid string) (*TrackInfo, error) {
	query := "SELECT id, rfid, track, offset, location FROM tracks WHERE rfid = ?"
	var trackInfo TrackInfo

	err := d.conn.QueryRow(query, rfid).Scan(&trackInfo.Id, &trackInfo.RFID, &trackInfo.Track, &trackInfo.Offset, &trackInfo.Location)
	if err != nil {
		return nil, err
	}

	return &trackInfo, nil
}

// GetAllUsers retrieves all users from the database
func (d *DB) GetAllTracks() ([]TrackInfo, error) {
	rows, err := d.conn.Query("SELECT id,rfid,track,offset,location FROM tracks")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []TrackInfo
	for rows.Next() {
		var trackInfo TrackInfo
		if err := rows.Scan(&trackInfo.Id, &trackInfo.RFID, &trackInfo.Track, &trackInfo.Offset, &trackInfo.Location); err != nil {
			return nil, err
		}
		tracks = append(tracks, trackInfo)
	}

	return tracks, nil
}
