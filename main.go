package main

import (
	"fmt"
	"github.com/r3labs/sse/v2"
	_ "github.com/r3labs/sse/v2"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"time"
)

type Song struct {
	Artist   string
	Title    string
	AudioUrl string
}

type Lobby struct {
	Name        string
	Slug        string
	Songs       []Song
	CurrentSong Song
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
		Name:  "Carnavalskrakers",
		Slug:  "carnavalskrakers",
		Songs: songs,
	}

	server := sse.New()
	go lobby.startLobby(server)

	mux := http.NewServeMux()
	mux.HandleFunc("/", indexHandler(lobby))
	mux.HandleFunc("/lobby", lobbyHandler(lobby))
	mux.HandleFunc("POST /guess", guessHandler(lobby, server))

	mux.HandleFunc("/events", server.ServeHTTP)

	fmt.Printf("[SERVER] starting lobby [%s] on :3000", lobby.Name)
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

func lobbyHandler(lobby *Lobby) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles("./templates/lobby.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data := struct {
			LobbyName string
			LobbySlug string
		}{
			LobbyName: lobby.Name,
			LobbySlug: lobby.Slug,
		}

		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func guessHandler(lobby *Lobby, server *sse.Server) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
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

		guess := r.FormValue("guess")

		if guess == lobby.CurrentSong.Artist || guess == lobby.CurrentSong.Title {
			server.Publish(lobby.Slug, &sse.Event{
				Data: []byte("Correct guess: " + guess),
			})
		}
	}
}

func (lobby *Lobby) startLobby(server *sse.Server) {
	server.CreateStream(lobby.Slug)

	lobby.CurrentSong = lobby.Songs[0]

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C
		randomSongIndex := rand.Intn(len(lobby.Songs))
		song := lobby.Songs[randomSongIndex]

		lobby.CurrentSong = song
		fmt.Printf("[LOBBY] song changed to [%s] - [%s]", song.Artist, song.Title)

		server.Publish(lobby.Slug, &sse.Event{
			Event: []byte("CurrentSong"),
			Data:  []byte("<audio controls autoplay src=\"" + song.AudioUrl + "\">"),
		})
	}
}
