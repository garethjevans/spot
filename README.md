# Spot - follow artists from your local files

A simple cli to follow the artists that are "popular" in your local files (on your harddrive).

## How it works

  * Searches for all files named `*.mp3`/`*.m4a` recursively from the specified directory.
  * Extracts the `Artist` from the ID tag
  * Counts the number of tracks for each artist.
  * If the number of tracks is greater than the threshold (default: 5), search for the artist & follow the artist on your spotify account.

Sometimes more than one match for an artists can be found, if the name of the artist matches the name of the spotify artists (case insensitive 
match), then the artist is followed. Yes, this sometimes follows too many artists with the same name but hey.... submit a PR to fix it.

## Installation

As this is not an offical app, you'll need to register a new app under the spotify developer portal. Add the client id/secret to:

```
cat ~/.spot/config.yaml
clientId: id
clientSecret: secret
```

Then run:

`make build && ./build/spot import --dir <directory-to-scan>`

Simples.

