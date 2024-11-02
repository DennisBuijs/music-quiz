package main

import (
	cryptoRand "crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/r3labs/sse/v2"
	_ "github.com/r3labs/sse/v2"
	"html/template"
	"log"
	"math"
	"math/rand"
	"net/http"
	"time"
)

var SONG_DURATION = 30 * time.Second
var BREAK_DURATION = 5 * time.Second

type Song struct {
	Artist   string
	Title    string
	AudioUrl string
}

type Lobby struct {
	Name              string
	Slug              string
	Songs             []Song
	CurrentSong       Song
	CurrentPhaseEndAt time.Time
	RoundsPlayed      int
	Score             []*Score
}

type Score struct {
	Player Player
	Score  int
}

type Player struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func main() {
	var songs []Song

	songs = append(songs, Song{
		"Guus Meeuwis",
		"Geef Mij Je Angst",
		"https://p.scdn.co/mp3-preview/0f8a68d9ca5fef269fe77cefd0087f0a6c390d10?cid=cfe923b2d660439caf2b557b21f31221",
	})

	songs = append(songs, Song{
		"Guus Meeuwis",
		"Brabant",
		"https://p.scdn.co/mp3-preview/d23338bd240c9cb8cc99f8c0f28eb6f4214b9e5b?cid=cfe923b2d660439caf2b557b21f31221",
	})

	songs = append(songs, Song{
		"Guus Meeuwig",
		"Per Spoor (Kedeng Kedeng)",
		"https://p.scdn.co/mp3-preview/a17e3e78be00473fe527e99cc1246e2244fa43cb?cid=cfe923b2d660439caf2b557b21f31221",
	})

	lobby := &Lobby{
		Name:              "Carnavalskrakers",
		Slug:              "carnavalskrakers",
		Songs:             songs,
		CurrentPhaseEndAt: time.Now().Add(SONG_DURATION),
		RoundsPlayed:      0,
	}

	server := sse.New()
	go lobby.startLobby(server)

	mux := http.NewServeMux()
	mux.HandleFunc("/", indexHandler(lobby))
	mux.HandleFunc("/login", loginHandler(lobby, server))
	mux.HandleFunc("/lobby", lobbyHandler(lobby, server))
	mux.HandleFunc("/players", playersHandler(lobby))
	mux.HandleFunc("POST /guess", guessHandler(lobby, server))

	mux.HandleFunc("/events", server.ServeHTTP)

	fmt.Printf("[SERVER] starting lobby [%s] on :3000\n", lobby.Name)
	err := http.ListenAndServe("localhost:3000", mux)
	if err != nil {
		log.Panic("[SERVER] could not start server")
	}
}

func indexHandler(lobby *Lobby) func(w http.ResponseWriter, r *http.Request) {
	song := lobby.Songs[0]

	return func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles("./templates/chrome.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data := struct {
			AudioUrl string
		}{
			AudioUrl: song.AudioUrl,
		}

		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func loginHandler(lobby *Lobby, server *sse.Server) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		playerName := r.FormValue("name")

		player := Player{
			ID:   generateRandomString(10),
			Name: playerName,
		}

		cookieValue, err := json.Marshal(player)
		if err != nil {

		}

		cookie := http.Cookie{
			Name:  "player",
			Value: base64.StdEncoding.EncodeToString(cookieValue),
		}

		lobby.addPlayer(player)

		server.Publish(lobby.Slug, &sse.Event{
			Event: []byte("Timer"),
			Data:  []byte(fmt.Sprintf(lobby.secondsUntilNextPhase())),
		})

		http.SetCookie(w, &cookie)
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func lobbyHandler(lobby *Lobby, server *sse.Server) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles("./templates/lobby.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		player := getPlayerFromRequest(r)

		data := struct {
			LobbyName string
			LobbySlug string
			Player    *Player
		}{
			LobbyName: lobby.Name,
			LobbySlug: lobby.Slug,
			Player:    player,
		}

		server.Publish(lobby.Slug, &sse.Event{
			Event: []byte("Timer"),
			Data:  []byte(fmt.Sprintf(lobby.secondsUntilNextPhase())),
		})

		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func playersHandler(lobby *Lobby) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles("./templates/players.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tmpl.Execute(w, lobby.Score); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func guessHandler(lobby *Lobby, server *sse.Server) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			return
		}

		player := getPlayerFromRequest(r)

		tmpl, err := template.ParseFiles("./templates/guess-form.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tmpl.Execute(w, nil); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		guess := r.FormValue("guess")

		var message string
		if guess == lobby.CurrentSong.Artist || guess == lobby.CurrentSong.Title {
			lobby.addScore(player.ID, 1)
			message = fmt.Sprintf("%s guessed correct!", player.Name)

			server.Publish(lobby.Slug, &sse.Event{
				Event: []byte("RefreshPlayers"),
				Data:  []byte(""),
			})
		} else {
			message = fmt.Sprintf("%s guessed wrong!", player.Name)
		}

		server.Publish(lobby.Slug, &sse.Event{
			Event: []byte("Chat"),
			Data:  []byte("<div class=\"chat-message\">" + message + "</div>"),
		})
	}
}

func (lobby *Lobby) startLobby(server *sse.Server) {
	server.CreateStream(lobby.Slug)

	lobby.CurrentSong = lobby.Songs[0]

	roundTimer := time.NewTicker(SONG_DURATION + BREAK_DURATION)
	defer roundTimer.Stop()

	for {
		<-roundTimer.C
		randomSongIndex := rand.Intn(len(lobby.Songs))
		song := lobby.Songs[randomSongIndex]

		lobby.CurrentSong = song
		lobby.CurrentPhaseEndAt = time.Now().Add(SONG_DURATION)
		fmt.Printf("[LOBBY] song changed to [%s] - [%s]\n", song.Artist, song.Title)

		server.Publish(lobby.Slug, &sse.Event{
			Event: []byte("CurrentSong"),
			Data:  []byte("Guess!<audio src=\"" + song.AudioUrl + "\">"),
		})

		server.Publish(lobby.Slug, &sse.Event{
			Event: []byte("Timer"),
			Data:  []byte(fmt.Sprintf(lobby.secondsUntilNextPhase())),
		})

		breakTimer := time.After(SONG_DURATION)
		<-breakTimer
		server.Publish(lobby.Slug, &sse.Event{
			Event: []byte("CurrentSong"),
			Data:  []byte("Break!"),
		})

		lobby.CurrentPhaseEndAt = time.Now().Add(BREAK_DURATION)
		server.Publish(lobby.Slug, &sse.Event{
			Event: []byte("Timer"),
			Data:  []byte(fmt.Sprintf(lobby.secondsUntilNextPhase())),
		})
	}
}

func (lobby *Lobby) secondsUntilNextPhase() string {
	seconds := math.Ceil(lobby.CurrentPhaseEndAt.Sub(time.Now()).Seconds())
	return fmt.Sprintf("%02d", int(seconds))
}

func (lobby *Lobby) addPlayer(player Player) {
	lobby.Score = append(lobby.Score, &Score{
		Player: player,
		Score:  0,
	})
}

func getPlayerFromRequest(r *http.Request) *Player {
	var player *Player
	cookie, _ := r.Cookie("player")

	if cookie != nil {
		decodedCookieValue, err := base64.StdEncoding.DecodeString(cookie.Value)
		if err != nil {
			//
		}

		err = json.Unmarshal(decodedCookieValue, &player)
		if err != nil {
			//
		}
	}

	return player
}

func generateRandomString(length int) string {
	b := make([]byte, length)
	if _, err := cryptoRand.Read(b); err != nil {
		panic(err)
	}

	return fmt.Sprintf("%X", b)
}

func (lobby *Lobby) addScore(playerId string, pointsAmount int) {
	for _, score := range lobby.Score {
		if score.Player.ID == playerId {
			score.Score += pointsAmount
			return
		}
	}
}
