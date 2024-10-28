package main

import (
	"fmt"
	"net/http"
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
}

func indexHandler(lobby Lobby) func(w http.ResponseWriter, r *http.Request) {
	song := lobby.Songs[0]
	fmt.Println(song.AudioUrl)

	return func(w http.ResponseWriter, r *http.Request) {
		html := fmt.Sprintf("<h1>Music Quiz</h1><audio controls src=\"%s\">", song.AudioUrl)
		_, err := w.Write([]byte(html))
		if err != nil {
			//
		}
	}
}
