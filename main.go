package main

import (
	"fmt"
	"github.com/r3labs/sse/v2"
	"log"
	"net/http"
	"html/template"
	_ "github.com/r3labs/sse/v2"
)

type Song struct {
	Artist   string
	Title    string
	AudioUrl string
}

type Lobby struct {
	Name  string
	Slug  string
	Songs []Song
}

func main() {
	songs := []Song{}

	songs = append(songs, Song{
		"Piet Friet",
		"Broodje Frik",
		"https://p.scdn.co/mp3-preview/a4681d2ecf1a594e5ac3170379eaa5de85a3b017?cid=cfe923b2d660439caf2b557b21f31221",
	})

	songs = append(songs, Song{
		"Aart Appeltaart",
		"Suiker in de Thee",
		"https://p.scdn.co/mp3-preview/f15fb7b8651c5988c4e36bc4ccc82664cea64b2a?cid=cfe923b2d660439caf2b557b21f31221",
	})

	songs = append(songs, Song{
		"Fred Kroket",
		"Tien Uur Snacks",
		"https://p.scdn.co/mp3-preview/7f75e97880ceef1e88fc8097a568d765bc1f555c?cid=cfe923b2d660439caf2b557b21f31221",
	})

	lobby := Lobby{
		Name:  "Carnavalskrakers",
		Slug:  "carnavalskrakers",
		Songs: songs,
	}

	server := sse.New()
	lobby.startLobby(server)

	mux := http.NewServeMux()
	mux.HandleFunc("/", indexHandler(lobby))
	mux.HandleFunc("/lobby", lobbyHandler(lobby))
	mux.HandleFunc("/guess", guessHandler(lobby, server))

	mux.HandleFunc("/events", server.ServeHTTP)

	fmt.Printf("[SERVER] starting lobby [%s] on :3000", lobby.Name)
	err := http.ListenAndServe("localhost:3000", mux)
	if err != nil {
		log.Panic("[SERVER] could not start server")
	}
}

func indexHandler(lobby Lobby) func(w http.ResponseWriter, r *http.Request) {
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

func lobbyHandler(lobby Lobby) func(w http.ResponseWriter, r *http.Request) {
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

func guessHandler(lobby Lobby, server *sse.Server) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles("./templates/guess-form.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tmpl.Execute(w, nil); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		guess := r.FormValue("guess")
		fmt.Printf("[GUESS] %s", guess)

		server.Publish(lobby.Slug, &sse.Event{
			Data: []byte(guess),
		})
	}
}

func (lobby Lobby) startLobby(server *sse.Server) {
	server.CreateStream(lobby.Slug)
}
