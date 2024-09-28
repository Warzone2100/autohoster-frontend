package main

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"io/fs"
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
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	out, err := mapsdatabase.FetchMapTerrain(hash)
	if err != nil {
		return nil, err
	}
	ob := bytes.NewBufferString("")
	err = png.Encode(ob, out)
	if err != nil {
		return nil, err
	}
	dperm := fs.FileMode(cfg.GetDInt(755, "dirPerms"))
	err = os.MkdirAll(path.Base(p), dperm)
	if err != nil {
		return nil, err
	}
	fperm := fs.FileMode(cfg.GetDInt(644, "filePerms"))
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
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	out, err := mapsdatabase.FetchMapBlob(hash)
	if err != nil {
		return nil, err
	}
	dperm := fs.FileMode(cfg.GetDInt(755, "dirPerms"))
	err = os.MkdirAll(path.Base(p), dperm)
	if err != nil {
		return nil, err
	}
	fperm := fs.FileMode(cfg.GetDInt(644, "filePerms"))
	err = os.WriteFile(p, out, fperm)
	if err != nil {
		return nil, err
	}
	return out, err
}
