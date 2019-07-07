package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/dhowden/tag"
)

// StopProcessing error to stop processing too many files in hierarchy
type StopProcessing struct {
	s string
}

// TrackInfo contains all necessary info about a music track for processing
type TrackInfo struct {
	Genre               string
	Title               string
	Artist              string
	Album               string
	AlbumArtist         string
	DiscNum             int
	DiscTot             int
	Rating              int
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
	var parsingFiles []string
	filepath.Walk("Z:\\hdd2\\_Music",
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				parsingFiles = append(parsingFiles, path)
			}
			if len(parsingFiles) == 1000 {
				return &StopProcessing{"basta"}
			}
			return nil
		})

	// got 1000 files, start processing
	for _, path := range parsingFiles {
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
				trackInfo.Genre = m.Genre()
				trackInfo.Title = m.Title()
				trackInfo.Artist = m.Artist()
				trackInfo.Album = m.Album()
				trackInfo.AlbumArtist = m.AlbumArtist()
				trackInfo.DiscNum, trackInfo.DiscTot = m.Disc()
				tags := m.Raw()
				for name, value := range tags {
					if name == "rating" {
						trackInfo.Rating = value.(int)
					}
					if name == "compilation" {
						trackInfo.Compilation = value.(string)
					}
					if name == "performer" {
						trackInfo.Performer = value.(string)
					}
					if name == "acoustid_id" || name == "acoustid_fingerprint" {
						trackInfo.AcoustidFingerprint = value.(string)
					}
					if name == "is_classical" {
						trackInfo.IsClassical = value.(bool)
					}
				}
				/*
					tmpl, err := template.New("test").Parse("{{.Count}} items are made of {{.Material}}")
					if err != nil {
						panic(err)
					}
					err = tmpl.Execute(os.Stdout, trackInfo)
				*/
			}
		}
	}
}
