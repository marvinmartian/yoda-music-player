package mp3

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/bogem/id3v2"
	"github.com/marvinmartian/yoda-music-player/internal/player"
	"github.com/tcolgate/mp3"
)

type mp3Data struct {
	EpisodeTitle string
	Author       string
	PodcastTitle string
}

func GetFramecount(track string) (float64, int) {
	t := 0.0
	frameCount := 0

	r, err := os.Open("/var/lib/mpd/music/" + track)
	if err != nil {
		fmt.Println(err)
		return 0, 0
	}
	defer r.Close()

	d := mp3.NewDecoder(r)
	var f mp3.Frame
	skipped := 0

	for {

		if err := d.Decode(&f, &skipped); err != nil {
			if err == io.EOF {
				break
			}
			fmt.Println(err)
			return 0, 0
		}
		// fmt.Println(f.Header().BitRate())
		t = t + f.Duration().Seconds()
		frameCount++
	}

	return t, frameCount
}

func ReadID3(filepath string, p *player.Player) mp3Data {
	// fmt.Println(filepath)
	// fmt.Println(p.CurrentSong())
	// os.Exit(1)
	tag, err := id3v2.Open("/var/lib/mpd/music/"+filepath, id3v2.Options{Parse: true})
	if err != nil {
		log.Fatal("Error while opening mp3 file: ", err)
	}
	defer tag.Close()

	// Create an instance of mp3Data and populate its fields from the ID3 tag
	data := mp3Data{
		EpisodeTitle: tag.Title(),
		Author:       tag.Artist(),
		PodcastTitle: tag.Album(),
	}

	// fmt.Println(tag.Artist())
	// fmt.Println(data)
	return data
}
