package main

import (
	"fmt"
	"log"
	"net/http"
	"html/template"
)

type Song struct {
	Artist   string
	Title    string
	AudioUrl string
}

type Lobby struct {
	Name  string
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
		Songs: songs,
	}

	http.HandleFunc("/", indexHandler(lobby))

	fmt.Printf("[SERVER] starting lobby [%s] on :3000", lobby.Name)
	err := http.ListenAndServe("localhost:3000", nil)
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
			LobbyName string
			AudioUrl  string
		}{LobbyName: lobby.Name,
			AudioUrl: song.AudioUrl,
		}

		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}
