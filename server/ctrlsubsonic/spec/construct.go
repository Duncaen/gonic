package spec

import "senan.xyz/g/gonic/db"

func NewPlaylist(p *db.Playlist) *Playlist {
	return &Playlist{
		ID:       p.ID,
		Name:     p.Name,
		Comment:  p.Comment,
		Duration: "1",
		Public:   true,
		Created:  p.CreatedAt,
	}
}
