package player

import (
	"github.com/fhs/gompd/v2/mpd"
)

// type Config struct {
// 	MusicDir    *string
// 	Port        *int
// 	MpdAddress  *string
// 	MpdPassword *string
// }

// // MpdPlayer struct to manage MPD player state
// type MpdPlayer struct {
// 	conn *mpd.Client
// }

// // NewMpdPlayer initializes a new MpdPlayer instance
// func NewMpdPlayer(conf *Config) (*MpdPlayer, error) {
// 	conn, err := mpd.DialAuthenticated("tcp", *conf.MpdAddress, *conf.MpdPassword)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &MpdPlayer{conn}, nil
// }

// internal/player/player.go
// package player

type MPDConfig struct {
	MusicDir    *string
	Track       *string
	Port        *int
	MpdAddress  *string
	MpdPassword *string
}

type Track struct {
	Name   string
	Artist string
	Album  string
}

// Player represents an MPD client
type Player struct {
	client *mpd.Client
}

// NewPlayer creates a new MPD client
func NewPlayer(conf *MPDConfig) (*Player, error) {
	conn, err := mpd.DialAuthenticated("tcp", *conf.MpdAddress, *conf.MpdPassword)
	if err != nil {
		return nil, err
	}
	return &Player{conn}, nil

}

// Close closes the MPD client connection
func (p *Player) Close() error {
	return p.client.Close()
}

// Clear MPD playlist
func (p *Player) Clear() error {
	return p.client.Clear()
}

// Play starts playing the current playlist
func (p *Player) Play() error {
	return p.client.Play(-1)
}

// Play starts playing the current playlist
func (p *Player) Seek(offset int) error {
	return p.client.Seek(0, offset)
}

// Stop stops playing the current playlist
func (p *Player) Stop() error {
	return p.client.Stop()
}

// Ping to daemon
func (p *Player) Ping() error {
	return p.client.Ping()
}

// Ping to daemon
func (p *Player) CurrentSong() (Track, error) {
	songAttr, err := p.client.CurrentSong()
	if err != nil {
		return Track{}, err
	}
	return Track{
		Name:   songAttr["Title"],
		Artist: songAttr["Artist"],
		Album:  songAttr["Album"],
	}, nil

}

// Shows the status of the MPD daemon
func (p *Player) Status() (mpd.Attrs, error) {
	status, err := p.client.Status()
	if err != nil {
		return nil, err
	}
	return status, nil
}

// TODO: Address the (likely) permission issues that lead to this hack
// func mpcAdd(path string) error {
// 	cmd := exec.Command("mpc", "add", path)
// 	output, err := cmd.CombinedOutput()

// 	if err != nil {
// 		return fmt.Errorf("error running mpc add: %v, output: %s", err, output)
// 	}

// 	return nil
// }

// AddToPlaylist adds a song to the current playlist
func (p *Player) AddToPlaylist(songPath string) error {
	// fmt.Println("Song Path:", songPath)
	// return mpcAdd(songPath)
	return p.client.Add(songPath)
	// return err
}

// ListPlaylist returns the current playlist
// func (p *Player) ListPlaylist() ([]string, error) {
// 	playlist, err := p.client.PlaylistInfo(-1, -1)
// 	if err != nil {
// 		return nil, err
// 	}

// 	var songs []string
// 	for _, entry := range playlist {
// 		songs = append(songs, entry.File)
// 	}

// 	return songs, nil
// }
