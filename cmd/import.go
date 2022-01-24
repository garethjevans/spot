package cmd

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"fmt"
	"github.com/dhowden/tag"
	"github.com/garethjevans/spot/config"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	//go:embed success.html
	successHTML string
	//go:embed error.html
	errorHTML string
)

type ImportCmd struct {
	Cmd       *cobra.Command
	Directory string
	Threshold int
}

func NewImportCommand() ImportCmd {
	cmd := ImportCmd{
		Cmd: &cobra.Command{
			Use:   "import",
			Short: "A brief description of your command",
			Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`},
	}
	cmd.Cmd.Run = cmd.Run

	cmd.Cmd.Flags().StringVarP(&cmd.Directory, "dir", "d", ".", "Directory to scan")
	cmd.Cmd.Flags().IntVarP(&cmd.Threshold, "threshold", "t", 5, "Number of tracks to filter on")

	return cmd
}

const RedirectURI = "http://localhost:1024/callback"

// importCmd represents the import command
func (i *ImportCmd) Run(cmd *cobra.Command, args []string) {

	artists := []string{}

	err := filepath.Walk(i.Directory,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				panic(err)
			}

			if strings.HasSuffix(path, ".mp3") || strings.HasSuffix(path, ".m4a") {
				f, err := os.Open(path)
				if err != nil {
					panic(err)
				}
				defer f.Close()

				m, err := tag.ReadFrom(f)
				if err != nil {
					fmt.Printf("[WARN] %s contained no tags\n", path)
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
		if count >= i.Threshold {
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
	var lastArtist = ""
	for {
		options := []spotify.RequestOption{spotify.Limit(50)}
		if lastArtist != "" {
			options = append(options, spotify.After(lastArtist))
		}
		followedArtistsResponse, err := client.CurrentUsersFollowedArtists(context.Background(), options...)
		if err != nil {
			panic(err)
		}

		fmt.Println("Requesting followed artists")

		for _, followed := range followedArtistsResponse.Artists {
			fmt.Printf("Already following '%s'\n", followed.Name)
			followedArtists = append(followedArtists, followed.Name)
			lastArtist = followed.ID.String()
		}

		if len(followedArtistsResponse.Artists) < 50 {
			break
		}
	}

	for artist, count := range grouped {
		if count >= i.Threshold {
			if !containsIgnoreCase(followedArtists, artist) {
				fmt.Printf("Searching for artist '%s'\n", artist)

				result, err := client.Search(context.Background(), artist, spotify.SearchTypeArtist)
				if err != nil {
					panic(err)
				}

				if result.Artists != nil {
					fmt.Printf("got %d results\n", len(result.Artists.Artists))
					if len(result.Artists.Artists) > 0 {

						if len(result.Artists.Artists) == 1 {
							// if there is only one result, lets go for that
							followArtist(client, result.Artists.Artists[0])
						} else {
							// loop through each, and check the name match (case insensitive) then follow that artist
							match := false
							for _, a := range result.Artists.Artists {
								if strings.EqualFold(artist, a.Name) {
									match = true
									fmt.Printf("Following '%s'...\n", a.Name)
									err = client.FollowArtist(context.Background(), a.ID)
									if err != nil {
										panic(err)
									}
									fmt.Println("OK")
									time.Sleep(500 * time.Millisecond)
								}
							}

							if !match {
								fmt.Println("[WARN] Unable to find a match")
								for _, a := range result.Artists.Artists {
									fmt.Printf("\talternatives '%s'\n", a.Name)
								}
							}
						}
					}
				}
			}
		}
	}
}

func followArtist(client *spotify.Client, artist spotify.FullArtist) {
	fmt.Printf("Following '%s'...\n", artist.Name)
	err := client.FollowArtist(context.Background(), artist.ID)
	if err != nil {
		panic(err)
	}
	fmt.Println("OK")
}

func init() {
	id, secret, err := config.Load()
	if err != nil {
		panic(err)
	}
	os.Setenv("SPOTIFY_ID", id)
	os.Setenv("SPOTIFY_SECRET", secret)
	rootCmd.AddCommand(NewImportCommand().Cmd)
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

func containsIgnoreCase(list []string, check string) bool {
	for _, l := range list {
		if strings.EqualFold(l, check) {
			return true
		}
	}
	return false
}
