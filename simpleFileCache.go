package main

import (
	"bytes"
	"image"
	"image/png"
	"io/fs"
	"log"
	"os"
	"path"

	mapsdatabase "github.com/maxsupermanhd/go-wz/maps-database"
)

func mapsdbGetTerrain(hash string) (image.Image, error) {
	p := path.Join(cfg.GetDString("./mapcache/terrain/", "cache", "terrain"), hash+".png")
	b, err := os.ReadFile(p)
	if err == nil {
		return png.Decode(bytes.NewBuffer(b))
	}
	if _, ok := err.(*fs.PathError); !ok {
		return nil, err
	}
	log.Printf("Fetching terrain image %v", hash)
	out, err := mapsdatabase.FetchMapTerrain(hash)
	if err != nil {
		return nil, err
	}
	ob := bytes.NewBufferString("")
	err = png.Encode(ob, out)
	if err != nil {
		return nil, err
	}
	dperm := fs.FileMode(cfg.GetDInt(493, "dirPerms"))
	err = os.MkdirAll(path.Dir(p), dperm)
	if err != nil {
		return nil, err
	}
	fperm := fs.FileMode(cfg.GetDInt(420, "filePerms"))
	err = os.WriteFile(p, ob.Bytes(), fperm)
	if err != nil {
		return nil, err
	}
	return out, err
}

func mapsdbGetBlob(hash string) ([]byte, error) {
	p := path.Join(cfg.GetDString("./mapcache/blob/", "cache", "blob"), hash+".wz")
	b, err := os.ReadFile(p)
	if err == nil {
		return b, nil
	}
	if _, ok := err.(*fs.PathError); !ok {
		return nil, err
	}
	log.Printf("Fetching map blob %v", hash)
	out, err := mapsdatabase.FetchMapBlob(hash)
	if err != nil {
		return nil, err
	}
	dperm := fs.FileMode(cfg.GetDInt(493, "dirPerms"))
	err = os.MkdirAll(path.Dir(p), dperm)
	if err != nil {
		return nil, err
	}
	fperm := fs.FileMode(cfg.GetDInt(420, "filePerms"))
	err = os.WriteFile(p, out, fperm)
	if err != nil {
		return nil, err
	}
	return out, err
}
