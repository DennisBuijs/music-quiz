package main

import (
    "fmt"
    "github.com/joho/godotenv"
    "io"
    "net/http"
    "strings"
    "log"
    "os"
)

func main() {
    err := godotenv.Load("../../.env")
    if err != nil {
        log.Fatal("Error loading .env file")
    }

    accessToken := getAccessToken(
        os.Getenv("SPOTIFY_CLIENT_ID"),
        os.Getenv("SPOTIFY_CLIENT_SECRET"),
    )

    fmt.Println(accessToken)
}

func getAccessToken(clientId string, clientSecret string) string {
    url := "https://accounts.spotify.com/api/token"
    method := "POST"

    s := fmt.Sprintf("grant_type=client_credentials&client_id=%s&client_secret=%s", clientId, clientSecret)
    payload := strings.NewReader(s)

    client := &http.Client{}
    req, err := http.NewRequest(method, url, payload)

    if err != nil {
        fmt.Println(err)
        return ""
    }
    req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

    res, err := client.Do(req)
    if err != nil {
        fmt.Println(err)
        return ""
    }
    defer res.Body.Close()

    body, err := io.ReadAll(res.Body)
    if err != nil {
        fmt.Println(err)
        return ""
    }

    return string(body)
}
