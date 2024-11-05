package main

import (
	"bytes"
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
	"strings"
	"time"
	"unicode"
)

var SONG_DURATION = 30 * time.Second
var BREAK_DURATION = 5 * time.Second
var PLAYER_SESSION_DURATION = 10 * time.Minute

type Song struct {
	ID          string `json:"id"`
	Artist      string `json:"artist"`
	Title       string `json:"title"`
	ArtUrl      string `json:"artUrl"`
	AudioUrl    string `json:"audioUrl"`
	ExternalUrl string `json:"externalUrl"`
}

type Lobby struct {
	Name              string
	Slug              string
	Songs             []Song
	CurrentSong       Song
	CurrentPhaseEndAt time.Time
	RoundsPlayed      int
	Score             []*Score
	SessionId         string
	PlaylistId        string
}

type Score struct {
	Player Player
	Score  int
}

type Player struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	SessionExpireAt time.Time
}

func main() {
	lobby := &Lobby{
		Name:              "Pop Hits",
		Slug:              "pop-hits",
		PlaylistId:        "2dnGUVwVbvNEylkmmtisXU",
		CurrentPhaseEndAt: time.Now().Add(SONG_DURATION),
		RoundsPlayed:      0,
		SessionId:         generateRandomString(10),
	}

	lobby.Songs = GetSongs(lobby.PlaylistId)

	server := sse.New()
	server.BufferSize = 0
	server.AutoReplay = false
	go lobby.startLobby(server)

	mux := http.NewServeMux()
	mux.HandleFunc("/asset/", assetHandler())
	mux.HandleFunc("/", indexHandler(lobby))
	mux.HandleFunc("/login", loginHandler(lobby, server))
	mux.HandleFunc("/lobby", lobbyHandler(lobby, server))
	mux.HandleFunc("/players", playersHandler(lobby))
	mux.HandleFunc("POST /guess", guessHandler(lobby, server))

	mux.HandleFunc("/events", server.ServeHTTP)

	go lobby.startSessionExpiryTicker(server)

	fmt.Printf("[SERVER] starting lobby [%s] (%v songs)\n", lobby.Name, len(lobby.Songs))
	err := http.ListenAndServe("0.0.0.0:3000", mux)
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
			LobbySlug string
			AudioUrl  string
		}{
			LobbySlug: lobby.Slug,
			AudioUrl:  song.AudioUrl,
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
			ID:              generateRandomString(10),
			Name:            playerName,
			SessionExpireAt: time.Now().Add(PLAYER_SESSION_DURATION),
		}

		cookieValue, err := json.Marshal(player)
		if err != nil {

		}

		cookie := http.Cookie{
			Name:     lobby.SessionId,
			Value:    base64.StdEncoding.EncodeToString(cookieValue),
			Secure:   true,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   int(PLAYER_SESSION_DURATION.Seconds()),
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

		player := lobby.getPlayerFromRequest(w, r)

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

		player := lobby.getPlayerFromRequest(w, r)
		if player == nil {
			w.Header().Add("HX-Refresh", "true")
			return
		}

		tmpl, err := template.ParseFiles("./templates/guess-form.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tmpl.Execute(w, nil); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		guess := normalizeString(r.FormValue("guess"))

		var message string
		if guess == normalizeString(lobby.CurrentSong.Artist) || guess == normalizeString(lobby.CurrentSong.Title) {
			lobby.addScore(player.ID, 1)
			message = fmt.Sprintf("%s guessed correct!", player.Name)

			server.Publish(lobby.Slug, &sse.Event{
				Event: []byte("RefreshPlayers"),
				Data:  []byte("_"),
			})
		} else {
			message = fmt.Sprintf("%s guessed wrong!", player.Name)
		}

		chatMessage := guessAsChatMessage(message)

		server.Publish(lobby.Slug, &sse.Event{
			Event: []byte("Chat"),
			Data:  []byte(chatMessage),
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
			Data:  []byte("Guess!<audio autoplay src=\"" + song.AudioUrl + "\">"),
		})

		server.Publish(lobby.Slug, &sse.Event{
			Event: []byte("Timer"),
			Data:  []byte(fmt.Sprintf(lobby.secondsUntilNextPhase())),
		})

		breakTimer := time.After(SONG_DURATION)
		<-breakTimer

		lastSongChatMessage := song.asChatMessage()
		server.Publish(lobby.Slug, &sse.Event{
			Event: []byte("Chat"),
			Data:  []byte(lastSongChatMessage),
		})

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

func (lobby *Lobby) getPlayerFromRequest(w http.ResponseWriter, r *http.Request) *Player {
	var player *Player
	cookie, _ := r.Cookie(lobby.SessionId)

	if cookie != nil {
		decodedCookieValue, err := base64.StdEncoding.DecodeString(cookie.Value)
		if err != nil {
			//
		}

		err = json.Unmarshal(decodedCookieValue, &player)
		if err != nil {
			//
		}

		player.SessionExpireAt = time.Now().Add(PLAYER_SESSION_DURATION)

		newCookie := http.Cookie{
			Name:     lobby.SessionId,
			Value:    cookie.Value,
			Secure:   true,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   int((PLAYER_SESSION_DURATION).Seconds()),
		}

		http.SetCookie(w, &newCookie)
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

func (song Song) asChatMessage() string {
	tmpl, err := template.ParseFiles("./templates/song-chat-message.html")
	if err != nil {
		fmt.Println("[SERVER] error loading template")
		return ""
	}

	var message bytes.Buffer
	err = tmpl.Execute(&message, song)
	if err != nil {
		fmt.Println("[SERVER] error parsing template")
		return ""
	}

	return strings.ReplaceAll(message.String(), "\n", "")
}

func guessAsChatMessage(guess string) string {
	tmpl, err := template.ParseFiles("./templates/guess-chat-message.html")
	if err != nil {
		fmt.Println("[SERVER] error loading template")
		return ""
	}

	var message bytes.Buffer
	err = tmpl.Execute(&message, guess)
	if err != nil {
		fmt.Println("[SERVER] error parsing template")
		return ""
	}

	return message.String()
}

func assetHandler() http.HandlerFunc {
	fs := http.FileServer(http.Dir("./assets"))
	return func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/asset")
		fs.ServeHTTP(w, r)
	}
}

func normalizeString(input string) string {
	var stringBuilder strings.Builder

	for _, r := range strings.ToLower(input) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			stringBuilder.WriteRune(r)
		}
	}

	return stringBuilder.String()
}

func (lobby *Lobby) startSessionExpiryTicker(server *sse.Server) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		<-ticker.C
		fmt.Println("[SERVER] kicking inactive players")

		now := time.Now()
		var activeScores []*Score

		for _, score := range lobby.Score {
			if score.Player.SessionExpireAt.After(now) {
				activeScores = append(activeScores, score)
			} else {
				fmt.Printf("[SERVER] kicked inactive player [%s]\n", score.Player.Name)

				server.Publish(lobby.Slug, &sse.Event{
					Event: []byte("RefreshPlayers"),
					Data:  []byte("_"),
				})
			}
		}

		lobby.Score = activeScores
	}
}
