package main

import (
	"archive/tar"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/DataDog/zstd"
	"github.com/natefinch/lumberjack"
	"golang.org/x/sys/unix"
)

var (
	appLog               = strings.Builder{}
	logsFilename         = "cleaner.log"
	instancesFolderPath  = flag.String("i", "/home/max/wz-multihoster/build/tmp/", "Multihoster instance folder")
	instancesDropoutPath = flag.String("o", "/home/max/mnt/bluelap/", "Where to drop out")
)

// 0 15 1-31/2 * *

func instDirNameToWeek(p string) int64 {
	nums := strings.TrimPrefix(p, "wz-")
	num, err := strconv.ParseInt(nums, 10, 64)
	must(err)
	return num / (7 * 24 * 60 * 60)
}

func instDirNameToDate(p string) time.Time {
	nums := strings.TrimPrefix(p, "wz-")
	num, err := strconv.ParseInt(nums, 10, 64)
	must(err)
	return time.Unix(num, 0)
}

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
	// must(godotenv.Load())
	log.Println("Cleanup operation starting up at", time.Now().String())

	instDir, err := os.ReadDir(*instancesFolderPath)
	must(err)
	log.Println(len(instDir))

	weeks := map[int64][]os.DirEntry{}

	for _, i := range instDir {
		if i.IsDir() && strings.HasPrefix(i.Name(), "wz-") {
			w := instDirNameToWeek(i.Name())
			ws, ok := weeks[w]
			if ok {
				ws = append(ws, i)
			} else {
				ws = []os.DirEntry{i}
			}
			weeks[w] = ws
		}
	}

	keys := make([]int64, 0, len(weeks))
	for k := range weeks {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	currentWeek := time.Now().Unix() / (7 * 24 * 60 * 60)

	var stat unix.Statfs_t
	wd, err := os.Getwd()
	must(err)

	for wi, w := range keys {
		if currentWeek-w < 2 {
			continue
		}
		instances := weeks[w]
		log.Printf("Packing week %3d offset %3d instances %3d", w, currentWeek-w, len(instances))

		unix.Statfs(wd, &stat)
		freeMegabytesLeft := (stat.Bavail * uint64(stat.Bsize)) / 1024 / 1024
		if freeMegabytesLeft < 250 {
			log.Println("Not enough space!", freeMegabytesLeft)
			break
		}

		weekfname := fmt.Sprintf("%v.tar.zst", w)
		f, err := os.OpenFile(path.Join(*instancesDropoutPath, weekfname), os.O_CREATE|os.O_WRONLY, os.FileMode(0764))
		must(err)
		zwr := zstd.NewWriterLevel(f, zstd.BestCompression)
		twr := tar.NewWriter(zwr)

		for i, j := range instances {
			unix.Statfs(wd, &stat)
			freeMegabytesLeft = (stat.Bavail * uint64(stat.Bsize)) / 1024 / 1024
			log.Printf("%4d/%-4d %5d/%-5d %4d %s %s", wi, len(keys), i, len(instances), freeMegabytesLeft, j.Name(), instDirNameToDate(j.Name()).String())
			filepath.Walk(path.Join(*instancesFolderPath, j.Name()), func(file string, fi os.FileInfo, err error) error {
				must(err)
				header, err := tar.FileInfoHeader(fi, file)
				must(err)
				header.Name = strings.TrimPrefix(filepath.ToSlash(file), *instancesFolderPath)
				// log.Printf("%s\n%#v", file, header) //, err)
				must(twr.WriteHeader(header))
				if !fi.IsDir() {
					data, err := os.Open(file)
					must(err)
					_, err = io.Copy(twr, data)
					must(err)
				}
				return nil
			})
		}

		// log.Println(w, "tar Flush")
		must(twr.Flush())
		// log.Println(w, "tar Close")
		must(twr.Close())
		// log.Println(w, "zst Flush")
		must(zwr.Flush())
		// log.Println(w, "zst Close")
		must(zwr.Close())
		// log.Println(w, "Sync")
		must(f.Sync())
		// log.Println(w, "Close")
		must(f.Close())

		for _, i := range instances {
			log.Println("Removing", path.Join(*instancesFolderPath, i.Name()))
			os.RemoveAll(path.Join(*instancesFolderPath, i.Name()))
		}
	}

	// log.Println("Connecting to database...")
	// db := noerr(pgx.Connect(noerr(pgx.ParseConnectionString(os.Getenv("DB")))))
	// defer db.Close()

	// log.Println("Fetching game ids that are older than 30 days...")
	// var lowestID int
	// must(db.QueryRow("SELECT max(id) FROM games WHERE timestarted < now() - '30 days'::interval").Scan(&lowestID))
	// log.Println("Latest saving game id is", lowestID)

	// log.Println("Clearing up graphs...")
	// tag := noerr(db.Exec("DELETE FROM frames WHERE game < $1", lowestID))
	// log.Println("Deleted", tag.RowsAffected(), "rows")

	// log.Println("Aggregating old replays to dropout zone...")
	// moved := processReplays(lowestID)
	// log.Println("Moved", moved, "replays to dropout zone")

	// log.Println("Uploading cleanup logs...")
	// pushLogs()
}

// func processReplays(lowestID int) (ret int) {
// 	basepath := getenvOr("REPLAY_STORAGE_PATH", "/home/max/replayStorage")
// 	// droppath := getenvOr("REPLAY_DROPOUT_PATH", "/home/max/replayDropout")
// 	replays := recursiveFindReplays(basepath, "")
// 	for fname, id := range replays {
// 		log.Printf("Found replay %d at [%s]", id, fname)
// 		if id < lowestID {
// 			log.Println("Found old replay", id, "path", fname, "removing...")
// 			// os.Remove(fname)
// 		}
// 	}
// 	return
// }

// func recursiveFindReplays(base string, encoded string) map[string]int {
// 	l := noerr(os.ReadDir(base))
// 	ret := map[string]int{}
// 	for _, v := range l {
// 		fp := path.Join(base, v.Name())
// 		fn := v.Name()
// 		if v.IsDir() {
// 			w := recursiveFindReplays(fp, fn+encoded)
// 			for i, j := range w {
// 				ret[i] = j
// 			}
// 		} else {
// 			if strings.HasSuffix(v.Name(), ".wzrp.zst") {
// 				rn := strings.TrimSuffix(v.Name(), ".wzrp.zst")
// 				i, err := strconv.ParseInt(rn+encoded, 32, 64)
// 				if err != nil {
// 					log.Println("Failed to parse integer from encoded id! [", fn+encoded, "]")
// 				}
// 				ret[fp] = int(i)
// 			} else {
// 				log.Println("Strange file found: ", fp)
// 			}
// 		}
// 	}
// 	return ret
// }

// func pushLogs() {
// 	return
// 	var b bytes.Buffer
// 	w := multipart.NewWriter(&b)
// 	fw, err := w.CreateFormFile("file", logsFilename)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	s := appLog.String()
// 	_, err = fw.Write([]byte(s))
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	fw, err = w.CreateFormField("payload_json")
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	j, err := json.Marshal(map[string]interface{}{
// 		"content": "Logs of running cleanup at " + time.Now().String(),
// 	})
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	_, err = fw.Write(j)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	client := http.Client{
// 		Timeout: 8 * time.Second,
// 	}
// 	w.Close()
// 	req, err := http.NewRequest("POST", getenvOr("DISCORD_WEBHOOK", ""), &b)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	req.Header.Set("Content-Type", w.FormDataContentType())
// 	res, err := client.Do(req)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	if res.StatusCode != http.StatusOK {
// 		log.Println(string(noerr(ioutil.ReadAll(res.Body))))
// 		log.Fatal(res)
// 	}
// }

// func getenvOr(key, fallback string) string {
// 	ret := os.Getenv(key)
// 	if ret == "" {
// 		ret = fallback
// 	}
// 	return ret
// }

//lint:ignore U1000 must
func must(err error) {
	if err != nil {
		pc, filename, line, _ := runtime.Caller(1)
		log.Fatalf("Error: %s[%s:%d] %v", runtime.FuncForPC(pc).Name(), path.Base(filename), line, err)
		// pushLogs()
	}
}

// //lint:ignore U1000 must
// func noerr[T any](t T, err error) T {
// 	if err != nil {
// 		pc, filename, line, _ := runtime.Caller(1)
// 		log.Printf("Error: %s[%s:%d] %v", runtime.FuncForPC(pc).Name(), path.Base(filename), line, err)
// 		pushLogs()
// 		os.Exit(1)
// 	}
// 	return t
// }

// //lint:ignore U1000 must
// func noerr2[T any, T1 any](t T, t1 T1, err error) (T, T1) {
// 	if err != nil {
// 		pc, filename, line, _ := runtime.Caller(1)
// 		log.Printf("Error: %s[%s:%d] %v", runtime.FuncForPC(pc).Name(), path.Base(filename), line, err)
// 		pushLogs()
// 		os.Exit(1)
// 	}
// 	return t, t1
// }
