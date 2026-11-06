package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// A Spotify app in Development Mode links five people at most, and its owner needs Premium; Extended Quota is only
// for organizations with 250,000 monthly users. Since February 2026 a playlist's tracks are its items, each item's
// track its item, and only those of playlists the person owns or collaborates on.
func init() {
	Register(&Connector{
		ID: "spotify", Name: "Spotify",
		Info: Info{By: "Spotify", Description: "Read the music you saved, played and put in playlists, and the artists you follow and play most, never change them", Category: CatMedia, Icon: IconMusic,
			Scopes: []string{"Read your saved tracks and playlists", "Read what you played lately and play most", "Read the artists you follow"}},
		API: "https://api.spotify.com/v1",
		Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.spotify.com/authorize", TokenURL: "https://accounts.spotify.com/api/token",
			AuthStyle: oauth2.AuthStyleInHeader},
		Scopes: []string{"user-read-private", "user-library-read", "playlist-read-private", "playlist-read-collaborative",
			"user-read-recently-played", "user-top-read", "user-follow-read"},
		Who: &Who{Req: Req{URL: "/me"}, Names: []Path{"display_name", "id"}},
		Actions: map[string]*Action{
			"search": Call[spotifyFind, map[string]any](Plan{
				Doc:   "Find tracks, artists, albums or playlists on Spotify by words: { tracks: { items }, artists: { items }, ... }, 10 of each at most",
				Clamp: map[string][2]int{"limit": {5, 10}},
				Enum:  map[string][]string{"type": {"track", "artist", "album", "playlist"}},
				Steps: []Step{{Req: Req{URL: "/search", Query: Q{"q": "{query}", "type": "{type|track}", "limit": "{limit}", "offset": "{offset}"}}}},
			}),
			"playing": Call[struct{}, map[string]any](Plan{
				Doc:   "What you are playing now, if anything: { is_playing, progress_ms, item: { name, artists, album, external_urls } }",
				Steps: []Step{{Req: Req{URL: "/me/player/currently-playing"}}},
			}),
		},
		Sync: []*Sync{
			{Name: "saved", Gap: time.Second, Req: Req{URL: "/me/tracks?limit=50"}, Items: "items", Key: []Reads{{Of: "{track/id}"}}, Updated: "added_at",
				Page: spotifyNext, Change: NewestFirst{}, Set: Obj{"list": "Liked Songs"},
				Doc: "Every track you saved to Liked Songs, newest first, 50 a page, by track id: listed whole the first time and once a week, " +
					"which drops those you unsaved, and in between only those saved since.",
				To: spotifyTracks},
			{Name: "played", Gap: time.Second, Every: 30 * time.Minute, Keep: true, Req: Req{URL: "/me/player/recently-played?limit=50"}, Items: "items",
				Key: []Reads{{Of: "{played_at}"}}, Updated: "played_at", Set: Obj{"list": "Played"},
				Doc: "Each play, by when it was played: Spotify tells only the last 50, so Archive asks every 30 minutes and keeps every play it heard of.",
				To:  spotifyTracks},
			{Name: "top_tracks", Gap: time.Second, Every: 24 * time.Hour, Req: Req{URL: "/me/top/tracks?time_range=medium_term&limit=50"}, Items: "items",
				Key: []Reads{{Of: "{id}"}}, Set: Obj{"list": "Top tracks"},
				Doc: "The 50 tracks you played most in about the last six months, best first, every day.",
				To:  spotifyTracks},
			{Name: "playlists", Gap: time.Second, Every: 6 * time.Hour, Req: Req{URL: "/me/playlists?limit=50"}, Items: "items", Key: []Reads{{Of: "{id}"}}, Page: spotifyNext,
				Doc: "Every playlist you made, follow or collaborate on, every six hours, 50 a page.",
				To: Map{Table: "music_playlist",
					Cols: map[string]SQL{"title": "data->>'name'", "description": "data->>'description'", "owner_name": "data#>>'{owner,display_name}'",
						"public": "(data->>'public')::boolean", "collaborative": "(data->>'collaborative')::boolean",
						"tracks": "archive.sync_num(coalesce(data#>>'{items,total}', data#>>'{tracks,total}'))", "url": "data#>>'{external_urls,spotify}'", "version": "data->>'snapshot_id'"}}},
			{Name: "playlist_items", Gap: time.Second, Every: 6 * time.Hour, Parts: spotifyPlaylists, Pass: []int{403, 404},
				Req: Req{URL: "/playlists/{part}/items?limit=50"}, Items: "items", Key: []Reads{{Of: "{part}/{item/id;track/id}/{added_at}"}}, Updated: "added_at", Page: spotifyNext,
				Set: Obj{"list": "{in/name}", "playlist": "{part}"},
				Doc: "The tracks and episodes of each playlist you own or collaborate on, every six hours, by <playlist>/<track>/<when added>; " +
					"Spotify shows no one else's playlists' items to an app like Archive's.",
				To: spotifyTracks},
			{Name: "artists", Gap: time.Second, Every: 24 * time.Hour, Req: Req{URL: "/me/following?type=artist&limit=50"}, Items: "artists/items", Key: []Reads{{Of: "{id}"}},
				Page: Cursor{At: "artists/cursors/after", Then: &Req{URL: "/me/following?type=artist&limit=50", Query: Q{"after": "{page}"}}}, Set: Obj{"list": "Following"},
				Doc: "Every artist you follow, every day.",
				To:  spotifyArtists},
			{Name: "top_artists", Gap: time.Second, Every: 24 * time.Hour, Req: Req{URL: "/me/top/artists?time_range=medium_term&limit=50"}, Items: "items", Key: []Reads{{Of: "{id}"}},
				Set: Obj{"list": "Top artists"},
				Doc: "The 50 artists you played most in about the last six months, best first, every day.",
				To:  spotifyArtists},
		},
	})
}

var (
	spotifyNext      = NextURL{At: "next"}
	spotifyPlaylists = &Parts{Req: Req{URL: "/me/playlists?limit=50"}, Items: "items", Page: spotifyNext, Vals: Obj{"name": "{name}"}}
	spotifyTrack     = "coalesce(data->'item', data->'track', data)"
	spotifyTracks    = Map{Table: "music_track",
		Cols: map[string]SQL{"list": "data->>'list'", "title": SQL(spotifyTrack + "->>'name'"),
			"artists":     SQL("(SELECT string_agg(a->>'name', ', ') FROM archive.sync_list(" + spotifyTrack + "->'artists') a)"),
			"album":       SQL(spotifyTrack + "#>>'{album,name}'"),
			"duration_ms": SQL("archive.sync_num(" + spotifyTrack + "->>'duration_ms')"),
			"url":         SQL(spotifyTrack + "#>>'{external_urls,spotify}'"),
			"track_id":    SQL(spotifyTrack + "->>'id'"),
			"rank":        "CASE WHEN data->>'list' = 'Top tracks' THEN i END",
			"added_at":    "archive.sync_at(data->>'added_at')", "played_at": "archive.sync_at(data->>'played_at')"},
		Refs: map[string]SQL{"playlist": "data->>'playlist'"}}
	spotifyArtists = Map{Table: "music_artist",
		Cols: map[string]SQL{"list": "data->>'list'", "title": "data->>'name'", "genres": `(SELECT string_agg(g#>>'{}', ', ') FROM archive.sync_list(data->'genres') g)`,
			"url": "data#>>'{external_urls,spotify}'", "artist_id": "data->>'id'", "rank": "CASE WHEN data->>'list' = 'Top artists' THEN i END"}}
)

type spotifyFind struct {
	Query  string `json:"query" jsonschema:"Words, or Spotify's filters like artist:Radiohead year:1997"`
	Type   string `json:"type,omitempty" jsonschema:"What to find; track when left out"`
	Limit  int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 10; 5 when left out"`
	Offset int    `json:"offset,omitempty" jsonschema:"How many to skip, for more"`
}
