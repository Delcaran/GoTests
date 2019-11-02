package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dhowden/tag"
)

type stopProcessing struct {
	s string
}

type classicalTrackInfo struct {
	composer     string
	movementNum  string // movement_num
	movementName string // movement_name | movement name
	conductor    string
}

type trackInfo struct {
	classicalInfo       classicalTrackInfo
	src                 string
	dst                 string
	genre               string
	title               string
	artist              string
	album               string
	albumArtist         string
	discNum             int
	discTot             int
	rating              int64
	trackNum            int
	trackTot            int
	compilation         string
	acoustidFingerprint string // acoustid_id, acoustid_fingerprint
	performer           string
	isClassical         bool
}

func (e *stopProcessing) Error() string {
	return e.s
}

func cleanString(stringa string) string {
	newstring := strings.TrimSpace(stringa)
	newstring = strings.ReplaceAll(stringa, "/", "")
	doublespace := (strings.Index(newstring, "  ") != -1)
	for doublespace {
		newstring = strings.ReplaceAll(newstring, "  ", " ")
		doublespace = (strings.Index(newstring, "  ") != -1)
	}
	maxNameLength := 240
	if len(newstring) > maxNameLength {
		newstring = newstring[:maxNameLength]
	}
	return newstring
}

func main() {
	// ottengo tutti i file e le cartelle
	var srcPath = path.Join("Z:", "hdd2", "_Music")
	filepath.Walk(srcPath,
		func(filepath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				f, err := os.Open(filepath)
				if err != nil {
					log.Fatal(err)
				} else {
					defer f.Close()
					m, errtag := tag.ReadFrom(f)
					if errtag != nil {
						log.Fatal(errtag)
					} else {
						var trackInfo trackInfo
						trackInfo.src = filepath
						trackInfo.genre = m.Genre()
						trackInfo.title = m.Title()
						trackInfo.artist = m.Artist()
						trackInfo.album = m.Album()
						trackInfo.trackNum, trackInfo.trackTot = m.Track()
						trackInfo.albumArtist = m.AlbumArtist()
						trackInfo.discNum, trackInfo.discTot = m.Disc()
						tags := m.Raw()
						for name, value := range tags {
							name = strings.ToLower(name)
							if name == "rating" {
								trackInfo.rating, _ = strconv.ParseInt(value.(string), 10, 0)
							}
							if name == "compilation" {
								trackInfo.compilation = value.(string)
							}
							if name == "performer" {
								trackInfo.performer = value.(string)
							}
							if name == "acoustid_fingerprint" {
								trackInfo.acoustidFingerprint = value.(string)
							}
							if name == "is_classical" {
								trackInfo.isClassical = (value == "1")
							}
							if name == "composer" {
								trackInfo.classicalInfo.composer = value.(string)
							}
							if name == "movement_num" {
								trackInfo.classicalInfo.movementNum = value.(string)
							}
							if name == "movement_name" {
								trackInfo.classicalInfo.movementName = value.(string)
							}
							if name == "conductor" {
								trackInfo.classicalInfo.conductor = value.(string)
							}
						}
						if !trackInfo.isClassical {
							classicalString := []string{"concert", "ballet", "classical", "symphon"}
							for _, substring := range classicalString {
								if strings.Index(strings.ToLower(trackInfo.genre), substring) > 0 {
									trackInfo.isClassical = true
								}
							}
						}
						// doing
						var baseDstPath = path.Join("Z:", "hdd2", "Music")
						var destDir string
						var destFile string
						if trackInfo.isClassical {
							destDir = baseDstPath
							if len(trackInfo.classicalInfo.composer) > 0 {
								destDir = path.Join(destDir, cleanString(trackInfo.classicalInfo.composer))
							}
							if len(trackInfo.album) > 0 {
								destDir = path.Join(destDir, cleanString(trackInfo.album))
							}
							if len(trackInfo.classicalInfo.conductor) > 0 {
								destDir = path.Join(destDir, cleanString(trackInfo.classicalInfo.conductor))
							}
							if len(trackInfo.albumArtist) > 0 {
								destDir = path.Join(destDir, cleanString(trackInfo.albumArtist))
							}
							if len(trackInfo.title) > 0 {
								destFile = cleanString(trackInfo.title)
							}
						} else {
							destDir = path.Join(baseDstPath, cleanString(trackInfo.albumArtist), cleanString(trackInfo.album))
							if trackInfo.discNum > 0 {
								destFile = fmt.Sprintf("%d.", trackInfo.discNum)
							}
							if trackInfo.trackNum > 0 {
								destFile += fmt.Sprintf("%d", trackInfo.trackNum)
								if len(destFile) > 0 {
									destFile += "_"
								}
							}
							if len(trackInfo.artist) > 0 && trackInfo.artist != trackInfo.albumArtist {
								destFile += cleanString(trackInfo.artist)
								if len(destFile) > 0 {
									destFile += "_"
								}
							}
							if len(trackInfo.title) > 0 {
								destFile += cleanString(trackInfo.title)
							}
						}
						destFile += path.Ext(trackInfo.src)
						destPath := path.Join(destDir, destFile)
						log.Printf("%s => %s \n", trackInfo.src, destPath)
						os.MkdirAll(destDir, os.FileMode(0777))
						os.Rename(trackInfo.src, destPath)
					}
				}
			}
			return nil
		})
}
