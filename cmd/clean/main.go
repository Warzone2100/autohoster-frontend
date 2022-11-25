package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx"
	"github.com/joho/godotenv"
	"github.com/natefinch/lumberjack"
)

var (
	appLog       = strings.Builder{}
	logsFilename = "cleaner.log"
)

// 0 15 1-31/2 * *

func main() {
	log.SetOutput(io.MultiWriter(&appLog, os.Stdout, &lumberjack.Logger{
		Filename:   "logs/" + logsFilename,
		MaxSize:    25,
		MaxAge:     31,
		MaxBackups: 0,
		LocalTime:  false,
		Compress:   true,
	}))
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	must(godotenv.Load())
	log.Println("Cleanup operation starting up at", time.Now().String())

	log.Println("Connecting to database...")
	db := noerr(pgx.Connect(noerr(pgx.ParseConnectionString(os.Getenv("DB")))))
	defer db.Close()

	log.Println("Fetching game ids that are older than 30 days...")
	var lowestID int
	must(db.QueryRow("SELECT max(id) FROM games WHERE timestarted < now() - '30 days'::interval").Scan(&lowestID))
	log.Println("Latest saving game id is", lowestID)

	log.Println("Clearing up graphs...")
	tag := noerr(db.Exec("DELETE FROM frames WHERE game < $1", lowestID))
	log.Println("Deleted", tag.RowsAffected(), "rows")

	log.Println("Aggregating old replays to dropout zone...")
	moved := processReplays(lowestID)
	log.Println("Moved", moved, "replays to dropout zone")

	log.Println("Uploading cleanup logs...")
	pushLogs()
}

func processReplays(lowestID int) (ret int) {
	basepath := getenvOr("REPLAY_STORAGE_PATH", "/home/max/replayStorage")
	// droppath := getenvOr("REPLAY_DROPOUT_PATH", "/home/max/replayDropout")
	replays := recursiveFindReplays(basepath, "")
	for fname, id := range replays {
		log.Printf("Found replay %d at [%s]", id, fname)
		if id < lowestID {
			log.Println("Found old replay", id, "path", fname, "removing...")
			// os.Remove(fname)
		}
	}
	return
}

func recursiveFindReplays(base string, encoded string) map[string]int {
	l := noerr(os.ReadDir(base))
	ret := map[string]int{}
	for _, v := range l {
		fp := path.Join(base, v.Name())
		fn := v.Name()
		if v.IsDir() {
			w := recursiveFindReplays(fp, fn+encoded)
			for i, j := range w {
				ret[i] = j
			}
		} else {
			if strings.HasSuffix(v.Name(), ".wzrp.zst") {
				rn := strings.TrimSuffix(v.Name(), ".wzrp.zst")
				i, err := strconv.ParseInt(rn+encoded, 32, 64)
				if err != nil {
					log.Println("Failed to parse integer from encoded id! [", fn+encoded, "]")
				}
				ret[fp] = int(i)
			} else {
				log.Println("Strange file found: ", fp)
			}
		}
	}
	return ret
}

func pushLogs() {
	return
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fw, err := w.CreateFormFile("file", logsFilename)
	if err != nil {
		log.Fatal(err)
	}
	s := appLog.String()
	_, err = fw.Write([]byte(s))
	if err != nil {
		log.Fatal(err)
	}
	fw, err = w.CreateFormField("payload_json")
	if err != nil {
		log.Fatal(err)
	}
	j, err := json.Marshal(map[string]interface{}{
		"content": "Logs of running cleanup at " + time.Now().String(),
	})
	if err != nil {
		log.Fatal(err)
	}
	_, err = fw.Write(j)
	if err != nil {
		log.Fatal(err)
	}
	client := http.Client{
		Timeout: 8 * time.Second,
	}
	w.Close()
	req, err := http.NewRequest("POST", getenvOr("DISCORD_WEBHOOK", ""), &b)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		log.Println(string(noerr(ioutil.ReadAll(res.Body))))
		log.Fatal(res)
	}
}

func getenvOr(key, fallback string) string {
	ret := os.Getenv(key)
	if ret == "" {
		ret = fallback
	}
	return ret
}

//lint:ignore U1000 must
func must(err error) {
	if err != nil {
		pc, filename, line, _ := runtime.Caller(1)
		log.Printf("Error: %s[%s:%d] %v", runtime.FuncForPC(pc).Name(), path.Base(filename), line, err)
		pushLogs()
	}
}

//lint:ignore U1000 must
func noerr[T any](t T, err error) T {
	if err != nil {
		pc, filename, line, _ := runtime.Caller(1)
		log.Printf("Error: %s[%s:%d] %v", runtime.FuncForPC(pc).Name(), path.Base(filename), line, err)
		pushLogs()
		os.Exit(1)
	}
	return t
}

//lint:ignore U1000 must
func noerr2[T any, T1 any](t T, t1 T1, err error) (T, T1) {
	if err != nil {
		pc, filename, line, _ := runtime.Caller(1)
		log.Printf("Error: %s[%s:%d] %v", runtime.FuncForPC(pc).Name(), path.Base(filename), line, err)
		pushLogs()
		os.Exit(1)
	}
	return t, t1
}
