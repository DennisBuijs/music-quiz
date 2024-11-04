package main

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "strings"

    "github.com/joho/godotenv"
)

func GetSongs(playlistId string) []Song {
    if err := godotenv.Load(".env"); err != nil {
        log.Fatalf("Error loading .env file: %v", err)
    }

    clientID := os.Getenv("SPOTIFY_CLIENT_ID")
    clientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")

    accessToken, err := getAccessToken(clientID, clientSecret)
    if err != nil {
        log.Fatalf("Error getting access token: %v", err)
    }

    songs, err := getSongsFromPlaylist(accessToken, playlistId)
    if err != nil {
        log.Fatalf("Error fetching songs from playlist: %v", err)
    }

    return songs
}

func getAccessToken(clientID, clientSecret string) (string, error) {
    url := "https://accounts.spotify.com/api/token"
    payload := strings.NewReader(fmt.Sprintf("grant_type=client_credentials&client_id=%s&client_secret=%s", clientID, clientSecret))

    req, err := http.NewRequest(http.MethodPost, url, payload)
    if err != nil {
        return "", fmt.Errorf("failed to create request: %w", err)
    }
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

    client := http.DefaultClient
    res, err := client.Do(req)
    if err != nil {
        return "", fmt.Errorf("failed to send request: %w", err)
    }
    defer res.Body.Close()

    var result map[string]interface{}
    if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
        return "", fmt.Errorf("failed to decode response: %w", err)
    }

    token, ok := result["access_token"].(string)
    if !ok {
        return "", fmt.Errorf("access token not found in response")
    }

    return token, nil
}

func getSongsFromPlaylist(accessToken, playlistID string) ([]Song, error) {
    url := fmt.Sprintf("https://api.spotify.com/v1/playlists/%s?fields=tracks.items(track(id,name,href,artists(name),external_urls.spotify,album.images(url,width),preview_url)", playlistID)
    req, err := http.NewRequest(http.MethodGet, url, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }
    req.Header.Set("Authorization", "Bearer "+accessToken)

    client := http.DefaultClient
    res, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to send request: %w", err)
    }
    defer res.Body.Close()

    var response struct {
        Tracks struct {
            Items []struct {
                Track struct {
                    ID       string                                  `json:"id"`
                    Name     string                                  `json:"name"`
                    Preview  string                                  `json:"preview_url"`
                    Artists  []struct{ Name string }                 `json:"artists"`
                    Album    struct{ Images []struct{ URL string } } `json:"album"`
                    External struct{ Spotify string }                `json:"external_urls"`
                } `json:"track"`
            } `json:"items"`
        } `json:"tracks"`
    }

    if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    var songs []Song
    for _, item := range response.Tracks.Items {
        track := item.Track
        if track.Preview == "" || track.External.Spotify == "" {
            continue
        }

        artistName := ""
        if len(track.Artists) > 0 {
            artistName = track.Artists[0].Name
        }

        artUrl := ""
        if len(track.Album.Images) > 0 {
            artUrl = track.Album.Images[0].URL
        }

        song := Song{
            ID:          track.ID,
            Artist:      artistName,
            Title:       track.Name,
            ArtUrl:      artUrl,
            AudioUrl:    track.Preview,
            ExternalUrl: track.External.Spotify,
        }

        songs = append(songs, song)
    }

    return songs, nil
}
