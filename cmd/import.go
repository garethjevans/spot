package cmd

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"fmt"
	"github.com/dhowden/tag"
	"github.com/pkg/browser"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	//go:embed success.html
	successHTML string
	//go:embed error.html
	errorHTML string
)

const RedirectURI = "http://localhost:1024/callback"

// importCmd represents the import command
var importCmd = &cobra.Command{
	Use:   "import",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		path := "/Users/garethevans/Music/Music/Media.localized/"

		artists := []string{}

		err := filepath.Walk(path,
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				if strings.HasSuffix(path, ".mp3") || strings.HasSuffix(path, ".m4a") {
					// fmt.Println(path, info.Size())
					f, err := os.Open(path)
					if err != nil {
						log.Fatal(err)
					}
					defer f.Close()

					m, err := tag.ReadFrom(f)
					if err != nil {
						//log.Fatal(err)
					} else {
						if "" != m.Artist() {
							artists = append(artists, m.Artist())
						}
					}
				}

				return nil
			})
		if err != nil {
			log.Println(err)
		}

		grouped := groupBy(artists)

		for artist, count := range grouped {
			if count >= 5 {
				fmt.Println("Artist:", artist, "=>", "Count:", count)
			}
		}

		// the redirect URL must be an exact match of a URL you've registered for your application
		// scopes determine which permissions the user is prompted to authorize
		auth := spotifyauth.New(spotifyauth.WithRedirectURL(RedirectURI), spotifyauth.WithScopes(
			spotifyauth.ScopeUserReadPrivate,
			spotifyauth.ScopeUserFollowRead,
			spotifyauth.ScopeUserFollowModify),
		)

		// get the user to this URL - how you do that is up to you
		// you should specify a unique state string to identify the session
		state, err := generateRandomState()
		if err != nil {
			panic(err)
		}
		url := auth.AuthURL(state)

		fmt.Println("Logging in to spotify, if your browser doesn't open, please navigate to the following URL: \n" + url)

		// the user will eventually be redirected back to your redirect URL
		// typically you'll have a handler set up like the following:
		// 3. Your app redirects the user to the authorization URI
		if err := browser.OpenURL(url); err != nil {
			panic(err)
		}

		client, err := listenForCode(state, auth)
		if err != nil {
			panic(err)
		}

		followedArtists := []string{}

		followedArtistsResponse, err := client.CurrentUsersFollowedArtists(context.Background(), spotify.Limit(50))
		if err != nil {
			panic(err)
		}

		fmt.Println("Requesting followed artists")

		for _, followed := range followedArtistsResponse.Artists {
			fmt.Printf("Already following '%s'\n", followed.Name)
			followedArtists = append(followedArtists, followed.Name)
		}

		for artist, count := range grouped {
			if count >= 5 {
				if !contains(followedArtists, artist) {
					fmt.Printf("Searching for artist '%s'\n", artist)

					result, err := client.Search(context.Background(), artist, spotify.SearchTypeArtist)
					if err != nil {
						panic(err)
					}

					if result.Artists != nil {
						fmt.Printf("got %d results\n", len(result.Artists.Artists))
						if len(result.Artists.Artists) > 0 {
							artistId := result.Artists.Artists[0].ID
							artistName := result.Artists.Artists[0].Name
							if artist == artistName {
								fmt.Printf("Following '%s'...\n", artist)
								err = client.FollowArtist(context.Background(), artistId)
								if err != nil {
									panic(err)
								}
								fmt.Println("OK")
								time.Sleep(1 * time.Second)
							}
						}
					}
				}
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(importCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// importCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// importCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func groupBy(slice []string) map[string]int {
	keys := make(map[string]int)
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = 1
		} else {
			keys[entry]++
		}
	}
	return keys
}

func generateRandomState() (string, error) {
	buf := make([]byte, 7)
	_, err := rand.Read(buf)
	if err != nil {
		return "", err
	}

	state := hex.EncodeToString(buf)
	return state, nil
}

func listenForCode(state string, auth *spotifyauth.Authenticator) (client *spotify.Client, err error) {
	server := &http.Server{Addr: ":1024"}

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {

		// use the same state string here that you used to generate the URL
		token, err := auth.Token(r.Context(), state, r)
		if err != nil {
			fmt.Fprintln(w, errorHTML)
			return
		}
		// create a client using the specified token
		client = spotify.New(auth.Client(r.Context(), token))
		fmt.Fprintln(w, successHTML)

		// Use a separate thread so browser doesn't show a "No Connection" message
		go func() {
			_ = server.Shutdown(context.Background())
		}()
	})

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		return nil, err
	}

	return
}

func contains(list []string, check string) bool {
	for _, l := range list {
		if l == check {
			return true
		}
	}
	return false
}
