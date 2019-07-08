package handler

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/jinzhu/gorm"

	"senan.xyz/g/gonic/model"
	"senan.xyz/g/gonic/server/subsonic"
)

func (c *Controller) GetArtists(w http.ResponseWriter, r *http.Request) {
	var artists []*model.Artist
	c.DB.
		Select("*, count(sub.id) as album_count").
		Joins(`
            LEFT JOIN albums sub
		    ON artists.id = sub.tag_artist_id
		`).
		Group("artists.id").
		Find(&artists)
	// [a-z#] -> 27
	indexMap := make(map[string]*subsonic.Index, 27)
	resp := make([]*subsonic.Index, 0, 27)
	for _, artist := range artists {
		i := lowerUDecOrHash(artist.IndexName())
		index, ok := indexMap[i]
		if !ok {
			index = &subsonic.Index{
				Name:    i,
				Artists: []*subsonic.Artist{},
			}
			indexMap[i] = index
			resp = append(resp, index)
		}
		index.Artists = append(index.Artists,
			newArtistByTags(artist))
	}
	sort.Slice(resp, func(i, j int) bool {
		return resp[i].Name < resp[j].Name
	})
	sub := subsonic.NewResponse()
	sub.Artists = &subsonic.Artists{
		List: resp,
	}
	respond(w, r, sub)
}

func (c *Controller) GetArtist(w http.ResponseWriter, r *http.Request) {
	id, err := getIntParam(r, "id")
	if err != nil {
		respondError(w, r, 10, "please provide an `id` parameter")
		return
	}
	artist := &model.Artist{}
	c.DB.
		Preload("Albums").
		First(artist, id)
	sub := subsonic.NewResponse()
	sub.Artist = newArtistByTags(artist)
	sub.Artist.Albums = make([]*subsonic.Album, len(artist.Albums))
	for i, album := range artist.Albums {
		sub.Artist.Albums[i] = newAlbumByTags(album, artist)
	}
	respond(w, r, sub)
}

func (c *Controller) GetAlbum(w http.ResponseWriter, r *http.Request) {
	id, err := getIntParam(r, "id")
	if err != nil {
		respondError(w, r, 10, "please provide an `id` parameter")
		return
	}
	album := &model.Album{}
	err = c.DB.
		Preload("TagArtist").
		Preload("Tracks", func(db *gorm.DB) *gorm.DB {
			return db.Order("tracks.tag_disc_number, tracks.tag_track_number")
		}).
		First(album, id).
		Error
	if gorm.IsRecordNotFoundError(err) {
		respondError(w, r, 10, "couldn't find an album with that id")
		return
	}
	sub := subsonic.NewResponse()
	sub.Album = newAlbumByTags(album, album.TagArtist)
	sub.Album.Tracks = make([]*subsonic.TrackChild, len(album.Tracks))
	for i, track := range album.Tracks {
		sub.Album.Tracks[i] = newTrackByTags(track, album)
	}
	respond(w, r, sub)
}

// changes to this function should be reflected in in _by_folder.go's
// getAlbumList() function
func (c *Controller) GetAlbumListTwo(w http.ResponseWriter, r *http.Request) {
	listType := getStrParam(r, "type")
	if listType == "" {
		respondError(w, r, 10, "please provide a `type` parameter")
		return
	}
	q := c.DB.DB
	switch listType {
	case "alphabeticalByArtist":
		q = q.Joins(`
			JOIN artists
			ON albums.tag_artist_id = artists.id`)
		q = q.Order("artists.name")
	case "alphabeticalByName":
		q = q.Order("tag_title")
	case "byYear":
		q = q.Where(
			"tag_year BETWEEN ? AND ?",
			getIntParamOr(r, "fromYear", 1800),
			getIntParamOr(r, "toYear", 2200))
		q = q.Order("tag_year")
	case "frequent":
		user := r.Context().Value(contextUserKey).(*model.User)
		q = q.Joins(`
			JOIN plays
			ON albums.id = plays.album_id AND plays.user_id = ?`,
			user.ID)
		q = q.Order("plays.count DESC")
	case "newest":
		q = q.Order("modified_at DESC")
	case "random":
		q = q.Order(gorm.Expr("random()"))
	case "recent":
		user := r.Context().Value(contextUserKey).(*model.User)
		q = q.Joins(`
			JOIN plays
			ON albums.id = plays.album_id AND plays.user_id = ?`,
			user.ID)
		q = q.Order("plays.time DESC")
	default:
		respondError(w, r, 10,
			"unknown value `%s` for parameter 'type'", listType)
		return
	}
	var albums []*model.Album
	q.
		Where("albums.tag_artist_id IS NOT NULL").
		Offset(getIntParamOr(r, "offset", 0)).
		Limit(getIntParamOr(r, "size", 10)).
		Preload("TagArtist").
		Find(&albums)
	sub := subsonic.NewResponse()
	sub.AlbumsTwo = &subsonic.Albums{
		List: make([]*subsonic.Album, len(albums)),
	}
	for i, album := range albums {
		sub.AlbumsTwo.List[i] = newAlbumByTags(album, album.TagArtist)
	}
	respond(w, r, sub)
}

func (c *Controller) SearchThree(w http.ResponseWriter, r *http.Request) {
	query := getStrParam(r, "query")
	if query == "" {
		respondError(w, r, 10, "please provide a `query` parameter")
		return
	}
	query = fmt.Sprintf("%%%s%%",
		strings.TrimSuffix(query, "*"))
	results := &subsonic.SearchResultThree{}
	//
	// search "artists"
	var artists []*model.Artist
	c.DB.
		Where(`
            name LIKE ? OR
            name_u_dec LIKE ?
		`, query, query).
		Offset(getIntParamOr(r, "artistOffset", 0)).
		Limit(getIntParamOr(r, "artistCount", 20)).
		Find(&artists)
	for _, a := range artists {
		results.Artists = append(results.Artists,
			newArtistByTags(a))
	}
	//
	// search "albums"
	var albums []*model.Album
	c.DB.
		Preload("TagArtist").
		Where(`
            tag_title LIKE ? OR
            tag_title_u_dec LIKE ?
		`, query, query).
		Offset(getIntParamOr(r, "albumOffset", 0)).
		Limit(getIntParamOr(r, "albumCount", 20)).
		Find(&albums)
	for _, a := range albums {
		results.Albums = append(results.Albums,
			newAlbumByTags(a, a.TagArtist))
	}
	//
	// search tracks
	var tracks []*model.Track
	c.DB.
		Preload("Album").
		Where(`
            tag_title LIKE ? OR
            tag_title_u_dec LIKE ?
		`, query, query).
		Offset(getIntParamOr(r, "songOffset", 0)).
		Limit(getIntParamOr(r, "songCount", 20)).
		Find(&tracks)
	for _, t := range tracks {
		results.Tracks = append(results.Tracks,
			newTrackByTags(t, t.Album))
	}
	sub := subsonic.NewResponse()
	sub.SearchResultThree = results
	respond(w, r, sub)
}
