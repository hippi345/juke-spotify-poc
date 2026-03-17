package spotify

// CurrentlyPlaying represents the Spotify currently-playing response
type CurrentlyPlaying struct {
	ProgressMs int64   `json:"progress_ms"`
	IsPlaying  bool    `json:"is_playing"`
	Item       *Track  `json:"item"`
	Timestamp  int64   `json:"timestamp"`
}

// Track represents a Spotify track
type Track struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	URI        string   `json:"uri"`
	Artists    []Artist `json:"artists"`
	Album      Album    `json:"album"`
	DurationMs int      `json:"duration_ms"`
}

// Artist represents a Spotify artist
type Artist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Album represents a Spotify album
type Album struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Images []Image `json:"images"`
}

// Image represents a Spotify image
type Image struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// Playlist represents a Spotify playlist (simplified for listing)
type Playlist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URI  string `json:"uri"`
}

// PlaylistsResponse wraps the playlists list response
type PlaylistsResponse struct {
	Items []PlaylistItem `json:"items"`
	Next  *string        `json:"next"`
}

// PlaylistItem represents a playlist in the list
type PlaylistItem struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	URI   string `json:"uri"`
	Owner struct {
		ID string `json:"id"`
	} `json:"owner"`
}

// PlaylistTracksResponse wraps the playlist tracks response
type PlaylistTracksResponse struct {
	Items []PlaylistTrackItem `json:"items"`
	Next  *string             `json:"next"`
	Total int                 `json:"total"`
}

// PlaylistTrackItem wraps a track in a playlist
// New /items endpoint uses "item"; deprecated /tracks used "track"
type PlaylistTrackItem struct {
	Track *Track `json:"track"` // deprecated
	Item  *Track `json:"item"`  // new endpoint
}

// RecommendationsResponse wraps the recommendations response
type RecommendationsResponse struct {
	Tracks []Track `json:"tracks"`
}

// Device represents a Spotify Connect device
type Device struct {
	ID               string `json:"id"`
	IsActive         bool   `json:"is_active"`
	IsRestricted     bool   `json:"is_restricted"`
	Name             string `json:"name"`
	Type             string `json:"type"`
	VolumePercent    *int   `json:"volume_percent"`
	SupportsVolume   bool   `json:"supports_volume"`
}

// DevicesResponse wraps the devices list response
type DevicesResponse struct {
	Devices []Device `json:"devices"`
}
