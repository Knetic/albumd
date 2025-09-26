package main

import (
	"albumd"
	"flag"
	"os"
)

func main() {

	settings := parseSettings()

	server := albumd.Server{
		AlbumPath:     settings.AlbumPath,
		ThumbPath:     settings.ThumbPath,
		Salt:          []byte(settings.Salt),
		AdminUsername: settings.AdminUsername,
		AdminPassword: settings.AdminPassword,
	}

	server.Run()
}

type RunSettings struct {
	AlbumPath     string
	ThumbPath     string
	Salt          string
	AdminUsername string
	AdminPassword string
}

func parseSettings() RunSettings {
	var ret RunSettings

	flag.StringVar(&ret.AlbumPath, "path", "./albums", "Path to the album directory")
	flag.StringVar(&ret.ThumbPath, "thumbs", "thumbs", "Path to the thumbnail directory")
	flag.StringVar(&ret.Salt, "salt", "aaa", "Salt used for hashing album names")

	ret.AdminUsername = os.Getenv("ALBUMD_USERNAME")
	ret.AdminPassword = os.Getenv("ALBUMD_PASSWORD")
	ret.Salt = os.Getenv("ALBUMD_SALT")

	flag.Parse()
	return ret
}
