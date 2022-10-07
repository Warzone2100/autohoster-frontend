package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/DataDog/zstd"
)

var replayStorageIdBase = 32

func getStorageReplayDir(gid int) string {
	ret := os.Getenv("REPLAY_STORAGE")
	if ret == "" {
		ret = "./replayStorage/"
	}
	if gid <= 0 {
		return ret
	}
	num := ""
	for _, v := range big.NewInt(int64(gid)).Text(replayStorageIdBase) {
		num = string(v) + num
	}
	for _, n := range num[0 : len(num)-1] {
		ret = path.Join(ret, string(n))
	}
	return ret
}

func getStorageReplayFilename(gid int) string {
	if gid < 0 {
		gid = -gid
	}
	num := ""
	for _, v := range big.NewInt(int64(gid)).Text(replayStorageIdBase) {
		num = string(v) + num
	}
	return string(num[len(num)-1:])
}

func getStorageReplayPath(gid int) string {
	return path.Join(getStorageReplayDir(gid), getStorageReplayFilename(gid)+".wzrp.zst")
}

var errReplayNotFound = errors.New("replay not found")

func findReplayByConfigFolder(p string) (string, error) {
	replaydir := path.Join(os.Getenv("MULTIHOSTER_GAMEDIRBASE"), p, "replay/multiplay/")
	files, err := ioutil.ReadDir(replaydir)
	if err != nil {
		return "", err
	}
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".wzrp") {
			h, err := os.Open(replaydir + "/" + f.Name())
			if err != nil {
				return "", err
			}
			var header [4]byte
			n, err := io.ReadFull(h, header[:])
			if err != nil {
				return "", err
			}
			h.Close()
			if n == 4 && string(header[:]) == "WZrp" {
				return replaydir + "/" + f.Name(), nil
			}
		}
	}
	return "", errReplayNotFound
}

func sendReplayToStorage(replaypath string, gid int) error {
	a, err := os.ReadFile(replaypath)
	if err != nil {
		return err
	}
	b, err := zstd.Compress(nil, a)
	if err != nil {
		return err
	}
	c := getStorageReplayDir(gid)
	log.Println("Storage dir: [", c, "]")
	err = os.MkdirAll(c, 0764)
	if err != nil {
		return err
	}
	d := getStorageReplayFilename(gid) + ".wzrp.zst"
	err = os.WriteFile(path.Join(c, d), b, 0664)
	if err != nil {
		return err
	}
	return os.Remove(replaypath)
}

func getReplayFromStorage(gid int) ([]byte, error) {
	fname := getStorageReplayPath(gid)
	a, err := os.ReadFile(fname)
	if errors.Is(err, os.ErrNotExist) {
		return nil, errReplayNotFound
	}
	if err != nil {
		return nil, err
	}
	return zstd.Decompress(nil, a)
}

func checkReplayExistsInStorage(gid int) bool {
	fname := getStorageReplayPath(gid)
	_, err := os.Stat(fname)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		log.Printf("Failed to stat by gid %d filename [%s]: %v", gid, fname, err)
		return false
	}
	return true
}

func replayMoveHandler(w http.ResponseWriter, r *http.Request) (int, interface{}) {
	if sessionGetUsername(r) != "Flex seal" {
		return 401, nil
	}
	rows, err := dbpool.Query(context.Background(), "SELECT id, gamedir FROM games WHERE finished = true AND gamedir != '-1'")
	if err != nil {
		log.Println("Error in gamedir request: ", err)
		return 500, nil
	}
	for rows.Next() {
		var gamedir string
		var gid int
		err = rows.Scan(&gid, &gamedir)
		if err != nil {
			log.Println("Failed to scan row:", err)
			return 500, nil
		}
		log.Println("Moving replay for gid [", gid, "] gamedir [", gamedir, "]")
		rpath, err := findReplayByConfigFolder(gamedir)
		if err == errReplayNotFound {
			log.Println("Replay gor gid", gid, "not found!")
			continue
		}
		if err != nil {
			log.Println("Error searching for replay file: ", err)
			return 500, nil
		}
		log.Println("Gamedir: [", rpath, "]")
		log.Println("Storage replay path: [", getStorageReplayPath(gid), "]")
		rcontent, err := os.ReadFile(rpath)
		if err != nil {
			log.Println("Failed to read replay: ", err)
			return 500, nil
		}
		err = sendReplayToStorage(rpath, gid)
		if err != nil {
			log.Println("Error moving to storage: ", err)
			return 500, nil
		}
		rcheck, err := getReplayFromStorage(gid)
		if err != nil {
			log.Println("Error getting from storage: ", err)
			return 500, nil
		}
		if !bytes.Equal(rcontent, rcheck) {
			log.Println("Comapre check failed!")
		}
	}
	return 200, nil
}
