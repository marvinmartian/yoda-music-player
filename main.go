package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"text/template"
	"time"

	"github.com/marvinmartian/yoda-music-player/internal/db"
	"github.com/marvinmartian/yoda-music-player/internal/player"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	mu             sync.Mutex
	lastPlayedID   string
	lastStartTime  time.Time
	timeout        = 3500 * time.Millisecond
	timeoutTimer   *time.Timer
	jsonData       map[string]map[string]interface{}
	isPlaying      bool
	canStartTrack  bool = true
	canPlayTimer   *time.Timer
	canPlayTimeout = 5 * time.Second
)

var (
	// Define a Prometheus CounterVec to track the number of plays for each track ID and name.
	playsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "track_plays_total",
			Help: "Total number of plays for each track ID.",
		},
		[]string{"track_id", "track_name", "track_artist"},
	)

	// Define a Prometheus Counter for a podcast plays.
	podcastCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "podcast_plays",
			Help: "A counter for the number of times a particular podcast is played.",
		},
		[]string{"episode_title"},
	)

	// Define a Prometheus Counter for tracking the number of errors.
	playErrorsCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "play_errors_total",
			Help: "Total number of play errors.",
		},
	)

	trackDuration = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "track_play_duration",
			Help: "Track the total amount of time played in seconds",
		},
		[]string{"track_id"},
	)
)

var trackdb *db.DB

type PostData struct {
	ID string `json:"id"`
}

type Track struct {
	Duration  float64 // Duration of the track in seconds (you can change the data type based on your requirements).
	TrackName string  // Name of the track.
}

// Message represents the structure of the JSON message
type SocketMessage struct {
	Text string `json:"text"`
}

var tracks = make(map[string]Track)

var mp3JsonPath string

var tmpl *template.Template

func init() {
	// Register the Prometheus metrics.
	prometheus.MustRegister(
		playsCounter,
		podcastCounter,
		playErrorsCounter,
		trackDuration,
	)

	flag.StringVar(&mp3JsonPath, "mp3File", "../mp3.json", "Path to the MP3 JSON file")
	flag.Parse()

	tmpl = template.Must(template.ParseGlob("templates/*.tmpl"))
	// tmpl = template.Must(template.ParseFiles("templates/index.html", "templates/navigation.html", "templates/footer.html"))
}

// Function to set the start time
func setStartTime() time.Time {
	startTime := time.Now()
	// fmt.Println("Start time set:", startTime)
	return startTime
}

// Function to get the duration since the start time
// func durationSinceStart(startTime time.Time, playedDuration float64) time.Duration {
// 	// Add the played duration to the original startTime
// 	newStartTime := startTime.Add(time.Duration(playedDuration * float64(time.Second)))

// 	// Calculate the duration since the updated startTime
// 	return time.Since(newStartTime)
// }

func durationSinceStart(startTime time.Time) time.Duration {
	return time.Since(startTime)
}

func playMP3(player *player.Player, filePath string, offset int, currentID string) {
	stopMP3(player)
	if isPlaying && currentID == lastPlayedID {
		fmt.Println("This track is already playing.")
		return
	} else if isPlaying {
		fmt.Println("Music is already playing, but starting new track.")
		stopMP3(player)
	}

	isPlaying = true

	player.Clear()
	player.AddToPlaylist(filePath)

	playErr := player.Play()
	if playErr != nil {
		fmt.Println("mpd Play Error")
		fmt.Println(playErr)
	}

	if offset > 0 {
		// time.Sleep(1 * time.Millisecond)
		seekErr := player.Seek(offset)
		if seekErr != nil {
			fmt.Println(seekErr)
		}
	}
}

func stopMP3(player *player.Player) {
	if isPlaying {
		// If music is playing, stop it
		player.Stop()
		player.Clear()
		isPlaying = false
		lastPlayedID = "0"
	}
}

func updateTrackPlayInfo(track string, duration float64) {
	// fmt.Printf("Update Track Play Info\n")

	// Add/Update the duration of the existing track
	tracks[track] = Track{Duration: duration, TrackName: track}
	// fmt.Printf("Track %s updated with new duration: %f seconds\n", track, duration)
}

func playHandler(player *player.Player) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST requests are allowed", http.StatusMethodNotAllowed)
			return
		}

		// Decode the request body into a PostData struct
		var postData PostData
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&postData)
		if err != nil {
			http.Error(w, "Failed to decode request body", http.StatusInternalServerError)
			return
		}

		currentID := postData.ID
		// fmt.Println("Received a POST request to /play with data:", currentID)

		mu.Lock()
		defer mu.Unlock()

		// Check if something is playing now
		if isPlaying {
			// playStats, _ := player.Status()
			if currentID != lastPlayedID {
				// If something is already playing, and it's not the same as the incoming ID
				fmt.Printf("Stopping the previous track (ID: %s) and starting the new track (ID: %s)\n", lastPlayedID, currentID)
				stopMP3(player)
				lastPlayedID = currentID
			} else {
				// If the same ID is requested again
				// fmt.Printf("Received the same ID again (ID: %s). Current track remains unchanged.\n", currentID)

				durationSince := durationSinceStart(lastStartTime)
				// fmt.Println("durationSinceStart:", durationSince.Seconds())
				updateTrackPlayInfo(currentID, durationSince.Seconds())
				trackDuration.WithLabelValues(currentID).Add(durationSince.Seconds())
				// Reset the timer
				if timeoutTimer != nil {
					timeoutTimer.Stop()
				}
				// Start or reset the timer
				if canPlayTimer != nil {
					// fmt.Println("reset canPlayTimer")
					canPlayTimer.Reset(canPlayTimeout)
				}

			}
		} else {

			// Start or reset the timer
			if canPlayTimer != nil {
				// fmt.Println("reset canPlayTimer")
				canPlayTimer.Reset(canPlayTimeout)
			} else {
				canPlayTimer = time.AfterFunc(canPlayTimeout, func() {
					mu.Lock()
					defer mu.Unlock()

					// Allow playing again
					fmt.Println("canPlayTimer timeout reached. Allowing play again")
					canStartTrack = true
					stopMP3(player)
				})
			}

			if canStartTrack {

				// If nothing is playing, start playing the song
				fmt.Printf("Starting to play the track (ID: %s)\n", currentID)
				lastPlayedID = currentID

				// Start or reset the timer
				if timeoutTimer != nil {
					timeoutTimer.Stop()
				}
				timeoutTimer = time.AfterFunc(timeout, func() {
					mu.Lock()
					defer mu.Unlock()

					// Stop playing if the timeout is reached
					fmt.Println("Timeout reached. Stopping play...")
					stopMP3(player)
				})

				// Check if the currentID exists in the JSON data
				if data, ok := jsonData[currentID]; ok {
					filePath, _ := data["file"].(string)
					offset, ok := data["offset"].(float64)
					if ok {
						if isPlaying {
							fmt.Println("")
						}
						trackPath := filePath
						// id3_info := mp3.ReadID3(trackPath, player)
						lastStartTime = setStartTime()
						go func() {
							// Call the function with the track path

							// duration := 2342.32
							// count := 232
							// duration, count := getFramecount(trackPath)

							// Print the results
							// fmt.Printf("Duration=%.2f seconds, Frame count=%d\n", duration, count)

							padded_offset := 0

							// if allow_play_resume {
							// 	playInfo := getTrackPlayInfo(currentID)
							// 	// fmt.Printf("getTrackPlayInfo.Duration: %f \n", playInfo.Duration)
							// 	frames_per_second := count / int(duration)
							// 	// fmt.Println(frames_per_second)
							// 	padded_offset = frames_per_second * int(playInfo.Duration)
							// 	// fmt.Println(padded_offset)

							// }

							playMP3(player, trackPath, int(offset)+padded_offset, currentID)

							currentSong, _ := player.CurrentSong()
							// fmt.Println(currentSong.Name)
							// fmt.Println(currentSong.Album)
							// fmt.Println(currentSong.Artist)

							// duration := playStats["duration"]
							// fmt.Println("-- Duration:", playStats["duration"])

							playsCounter.WithLabelValues(currentID, currentSong.Album, currentSong.Artist).Inc()
							podcastCounter.WithLabelValues(currentSong.Name).Inc()
							canStartTrack = false
						}()
						// duration, frames := getFramecount(trackPath)
						// fmt.Printf("frames: %d - duration %f seconds", frames, duration)
						// Increment the playsCounter metric for the current track ID.

					} else {
						fmt.Println("Offset field not found in JSON for ID:", currentID)
					}
				} else {
					fmt.Println("ID not found in JSON:", currentID)
				}
			}
		}

		fmt.Fprintln(w, "Data received and printed to console") // Respond to the client
	}
}

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log the incoming request
		// fmt.Printf("Received request: %s %s\n", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func statusHandler(player *player.Player) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// status, err := player.Status()
		// if err != nil {
		// 	fmt.Println(err)
		// }
		track := "Sample Track"
		volume := 65

		data := struct {
			TrackName string
			Volume    int
		}{
			TrackName: track,
			Volume:    volume,
		}

		templateContent := `
        <div class="track-volume-display" hx-get="/v1/status"
    hx-trigger="load delay:1s"
    hx-swap="outerHTML">
            <span>Current Track: {{ .TrackName}}</span>
            <span class="volume">Volume: {{ .Volume}}%</span>
        </div>
		`

		tmpl, err := template.New("playPage").Parse(templateContent)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = tmpl.Execute(w, data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func serveRoot(w http.ResponseWriter, r *http.Request) {

	the_tracks, err := trackdb.GetAllTracks()
	if err != nil {
		fmt.Println(err)
	}

	tmpl.ExecuteTemplate(w, "index.tmpl", the_tracks)
}

func handleNewTrack(w http.ResponseWriter, r *http.Request) {
	// time.Sleep(1 * time.Second)
	trackName := r.PostFormValue("title")
	offset := int64(20)
	// duration := r.PostFormValue("director")
	// htmlStr := fmt.Sprintf("<li class='list-group-item bg-primary text-white'>%s - %s</li>", title, director)
	// tmpl, _ := template.New("t").Parse(htmlStr)
	tmpl := template.Must(template.ParseFiles("templates/index.html"))
	tmpl.ExecuteTemplate(w, "film-list-element", db.TrackInfo{Track: trackName, Offset: offset})
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	// Load JSON data from the "mp3.json" file
	jsonDataFile, err := os.ReadFile(mp3JsonPath)
	if err != nil {
		fmt.Println("Error reading JSON file:", err)
		return
	}

	if err := json.Unmarshal(jsonDataFile, &jsonData); err != nil {
		fmt.Println("Error parsing JSON:", err)
		return
	}

	// Create a router
	router := http.NewServeMux()

	mpdAddress := "localhost:6600"
	mpdPassword := ""
	mpdConfig := player.MPDConfig{}
	mpdConfig.MpdAddress = &mpdAddress
	mpdConfig.MpdPassword = &mpdPassword

	var mpdPlayer *player.Player
	var mpdError error
	mpdPlayer, mpdError = player.NewPlayer(&mpdConfig)
	if mpdError != nil {
		fmt.Println("MPD Player Error:", err)
	}
	// Defer the call to mpdPlayer.Stop() to ensure it's executed on exit
	defer func() {
		fmt.Println("Exiting application")
		stopMP3(mpdPlayer)
	}()

	// Start a goroutine to ping the MPD server every 5 seconds
	// go func() {
	// 	var delay time.Duration
	// 	maxDelay := 1 * time.Minute // Set your maximum delay as needed
	// 	for {
	// 		// Ping the MPD server
	// 		if err := mpdPlayer.Ping(); err != nil {
	// 			fmt.Println("Error pinging MPD server:", err)

	// 			// Attempt to re-connect
	// 			// mpdPlayer, mpdError = player.NewPlayer(&mpdConfig)
	// 			// if mpdError != nil {
	// 			// 	fmt.Println("Error re-connecting..", err)
	// 			// }

	// 			// Exponential backoff: double the delay each time, with a maximum limit
	// 			delay *= 2
	// 			if delay == 0 {
	// 				delay = 1 * time.Second // Start with a small delay
	// 			} else if delay > maxDelay {
	// 				delay = maxDelay
	// 			}
	// 			time.Sleep(delay)
	// 			continue
	// 		}
	// 		// Reset delay on successful ping
	// 		delay = 0
	// 	}
	// }()
	var dberr error
	trackdb, dberr = db.NewDB("test.db")
	if dberr != nil {
		panic(dberr)
	}
	trackdb.CreateTable()

	// Default Route to show all available tracks
	router.HandleFunc("/", serveRoot)
	router.HandleFunc("/add-track/", handleNewTrack)

	// Define the route and handler for /play
	router.HandleFunc("/play", playHandler(mpdPlayer))
	router.HandleFunc("/v1/status", statusHandler(mpdPlayer))

	// Define a new route for Prometheus metrics
	router.Handle("/metrics", promhttp.Handler())

	// Create a handler chain with the request logger
	chain := http.Handler(logRequest(router))

	// Start the web server on port 3001
	fmt.Println("Listening on port 3001...")
	// Create a channel to listen for interrupt signals (Ctrl+C)
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	// Start the server in a goroutine
	go func() {
		if err := http.ListenAndServe(":3001", chain); err != nil {
			fmt.Println("Error:", err)
		}
	}()

	// Wait for an interrupt signal
	<-stopChan
	// err = http.ListenAndServe(":3001", chain)
	// if err != nil {
	// 	fmt.Println("Error:", err)
	// }

}
