package main

import (
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dhowden/tag"
)

// StopProcessing error to stop processing too many files in hierarchy
type StopProcessing struct {
	s string
}

// TrackInfo contains all necessary info about a music track for processing
type TrackInfo struct {
	src                 string
	dst                 string
	Genre               string
	Composer            string
	Title               string
	Artist              string
	Album               string
	AlbumArtist         string
	DiscNum             int
	DiscTot             int
	Rating              int64
	TrackNum            int
	TrackTot            int
	Compilation         string
	AcoustidFingerprint string // acoustid_id, acoustid_fingerprint
	Performer           string
	IsClassical         bool
}

func (e *StopProcessing) Error() string {
	return e.s
}

func main() {
	// ottengo tutti i file e le cartelle
	var parsingFiles []TrackInfo
	//var baseDstPath = path.Join("Z:", "hdd2", "Music")
	var srcPath = path.Join("Z:", "hdd2", "_Music")
	filepath.Walk(srcPath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				f, err := os.Open(path)
				if err != nil {
					log.Fatal(err)
				} else {
					defer f.Close()
					m, errtag := tag.ReadFrom(f)
					if errtag != nil {
						log.Fatal(errtag)
					} else {
						var trackInfo TrackInfo
						trackInfo.src = path
						trackInfo.Genre = m.Genre()
						trackInfo.Title = m.Title()
						trackInfo.Artist = m.Artist()
						trackInfo.Album = m.Album()
						trackInfo.TrackNum, trackInfo.TrackTot = m.Track()
						trackInfo.AlbumArtist = m.AlbumArtist()
						trackInfo.DiscNum, trackInfo.DiscTot = m.Disc()
						tags := m.Raw()
						for name, value := range tags {
							name = strings.ToLower(name)
							if name == "rating" {
								trackInfo.Rating, _ = strconv.ParseInt(value.(string), 10, 0)
							}
							if name == "compilation" {
								trackInfo.Compilation = value.(string)
							}
							if name == "performer" {
								trackInfo.Performer = value.(string)
							}
							if name == "acoustid_fingerprint" {
								trackInfo.AcoustidFingerprint = value.(string)
							}
							if name == "is_classical" {
								trackInfo.IsClassical = (value == "1")
							}
							if name == "composer" {
								trackInfo.Composer = value.(string)
							}
						}
						if !trackInfo.IsClassical {
							classicalString := []string{"concert", "ballet", "classical", "symphon"}
							for _, substring := range classicalString {
								if strings.Index(strings.ToLower(trackInfo.Genre), substring) > 0 {
									trackInfo.IsClassical = true
								}
							}
						}
						parsingFiles = append(parsingFiles, trackInfo)
					}
				}
			}
			if len(parsingFiles) == 1000 {
				return &StopProcessing{"basta"}
			}
			return nil
		})
	// got 1000 files, start processing
	/*
		for _, track := range parsingFiles {
			var destDir string
			var destFile string
			if track.IsClassical {
				log.Println("Classica")
			} else {
				destDir = path.Join(baseDstPath, track.AlbumArtist, track.Album)
			}
			os.MkdirAll(destDir, os.FileMode(0777))
			if track.DiscNum > 0 {
				destFile = fmt.Sprintf("%d.", track.DiscNum)
			}
			if track.TrackNum > 0 {
				destFile += fmt.Sprintf("%d", track.TrackNum)
				if len(destFile) > 0 {
					destFile += "_"
				}
			}
			if len(track.Artist) > 0 && track.Artist != track.AlbumArtist {
				destFile += track.Artist
				if len(destFile) > 0 {
					destFile += "_"
				}
			}
			if len(track.Title) > 0 {
				destFile += track.Title
			}
			destFile += filepath.Ext(track.src)
			log.Println(path.Join(destDir, destFile))
		}
	*/
}
