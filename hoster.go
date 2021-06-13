package main

import (
	"context"
	"html/template"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/jackc/pgx/v4"
	_ "github.com/jackc/pgx/v4/pgxpool"
)

var regexMaphash = regexp.MustCompile(`^[a-zA-Z0-9-]*$`)
var regexAlliances = regexp.MustCompile(`^[0-2]$`)
var regexLevelbase = regexp.MustCompile(`^[1-3]$`)
var regexScav = regexp.MustCompile(`^[0-1]$`)

// func ExecReq(r http.Request) (*http.Response, interface{}) {
// }

func hosterHandler(w http.ResponseWriter, r *http.Request) {
	if !sessionManager.Exists(r.Context(), "User.Username") || sessionManager.Get(r.Context(), "UserAuthorized") != "True" {
		basicLayoutLookupRespond("noauth", w, r, map[string]interface{}{})
		return
	}
	if r.Method == "POST" {
		err := r.ParseForm()

		if err != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Form parse error: " + err.Error()})
			return
		}
		if !regexMaphash.MatchString(r.PostFormValue("maphash")) {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "maphash must match `^[a-zA-Z0-9-]*$`"})
			return
		}
		if !regexAlliances.MatchString(r.PostFormValue("alliances")) {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "alliances must match `^[0-2]$`"})
			return
		}
		if !regexLevelbase.MatchString(r.PostFormValue("base")) {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "base must match `^[0-2]$`"})
			return
		}
		if !regexScav.MatchString(r.PostFormValue("scav")) {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "scav must match `^[0-1]$`, got [" + r.PostFormValue("scav") + "]"})
			return
		}
		var mapname string
		var allowedalliances int
		var allowedbase int
		var allowedscav int
		var numplayers int
		var laststr string
		var adminhash string
		var allow_preset_request bool

		derr := dbpool.QueryRow(context.Background(), `
		SELECT mapname, alliances, levelbase, scav, players, 
			(SELECT coalesce(extract(epoch from last_host_request), 0) FROM users WHERE username = $2)::text as last_host_request,
			(SELECT allow_preset_request FROM users WHERE username = $2) as allow_preset_request,
			coalesce((SELECT hash FROM players WHERE id = (SELECT coalesce(wzprofile2, 0) FROM users WHERE username = $2)), '') as adminhash
		FROM presets
		WHERE maphash = $1 AND NOT ((SELECT id FROM users WHERE username = $2) = ANY(coalesce(disallowed_users, array[]::int[])))
		LIMIT 1`, r.PostFormValue("maphash"), sessionManager.GetString(r.Context(), "User.Username")).Scan(
			&mapname, &allowedalliances, &allowedbase, &allowedscav, &numplayers, &laststr, &allow_preset_request, &adminhash)
		if derr != nil {
			if derr == pgx.ErrNoRows {
				basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Selected map is not allowed."})
				w.Header().Set("Refresh", "3; /request")
				return
			}
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database error: " + derr.Error()})
			return
		}

		if allow_preset_request {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Sorry, you are not allowed to request games."})
			return
		}

		lastfloat, cerr := strconv.ParseFloat(laststr, 64)
		if cerr != nil {
			log.Println(cerr.Error())
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database timestamp error: " + cerr.Error()})
			return
		}
		lastint := int64(lastfloat)
		if lastint-10800+300 > time.Now().Unix() && sessionManager.GetString(r.Context(), "User.Username") != "Flex seal" {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true,
				"msg": "Please wait before requesting another room. " + strconv.FormatInt(300-(time.Now().Unix()-(lastint-10800)), 10) + " seconds left."})
			return
		}

		tag, derr := dbpool.Exec(context.Background(), "UPDATE users SET last_host_request = now() WHERE username = $1", sessionManager.GetString(r.Context(), "User.Username"))
		if derr != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database error: " + cerr.Error()})
			return
		}
		if tag.RowsAffected() != 1 {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database update error, rows affected " + string(tag)})
			return
		}

		tag, derr = dbpool.Exec(context.Background(), "UPDATE presets SET last_requested = now() WHERE maphash = $1", r.PostFormValue("maphash"))
		if derr != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database error: " + cerr.Error()})
			return
		}
		if tag.RowsAffected() != 1 {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database update error, rows affected " + string(tag)})
			return
		}
		alliancen, err := strconv.Atoi(r.PostFormValue("alliances"))
		if alliancen > 0 {
			alliancen++
		}
		basen, err := strconv.Atoi(r.PostFormValue("base"))
		s, reqres := RequestHost(r.PostFormValue("maphash"),
			mapname, strconv.FormatInt(int64(alliancen), 10), strconv.FormatInt(int64(basen), 10),
			r.PostFormValue("scav"), strconv.FormatInt(int64(numplayers), 10), adminhash, "Autohoster")
		if s {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msggreen": true, "msg": reqres})
		} else {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Request error: " + reqres})
		}
		w.Header().Set("Refresh", "10; /created-rooms")
	} else {
		s, reqres := RequestStatus()
		if s {
			basicLayoutLookupRespond("multihoster", w, r, map[string]interface{}{
				"MultihosterStatus": template.HTML(reqres),
			})
		} else {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Request error: " + reqres})
		}
	}
}

func wzlinkHandler(w http.ResponseWriter, r *http.Request) {
	if !sessionManager.Exists(r.Context(), "User.Username") || sessionManager.Get(r.Context(), "UserAuthorized") != "True" {
		basicLayoutLookupRespond("noauth", w, r, map[string]interface{}{})
		return
	}
	var profilenum int
	var confirmcode string
	derr := dbpool.QueryRow(context.Background(), `SELECT coalesce(wzprofile2, -1), coalesce(wzconfirmcode, '') FROM users WHERE username = $1`,
		sessionManager.GetString(r.Context(), "User.Username")).Scan(&profilenum, &confirmcode)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Heeee?"})
			w.Header().Set("Refresh", "1; /logout")
			return
		}
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database error: " + derr.Error()})
		return
	}
	if confirmcode == "" && profilenum != -1 {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{
			"msgred": true, "msg": "Re-linking is not allowed. Linked to profile " + strconv.Itoa(profilenum),
		})
		return
	} else if confirmcode == "" && profilenum == -1 {
		newmsg := "confirm-" + generateRandomString(18)
		tag, derr := dbpool.Exec(context.Background(), `UPDATE users SET wzconfirmcode = $1 WHERE username = $2`,
			newmsg, sessionManager.GetString(r.Context(), "User.Username"))
		if derr != nil {
			log.Println(derr.Error())
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database error: " + derr.Error()})
			return
		}
		if tag.RowsAffected() != 1 {
			log.Println(derr.Error())
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database update error, rows affected " + string(tag)})
			return
		}
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{
			"msg": "You are not linked to any profile. We generated a link key for you. Send \"" + newmsg + "\" to any autohoster room to link selected profile.",
		})
	} else if confirmcode != "" && profilenum == -1 {
		var loghash string
		var logname string
		var logip string
		derr := dbpool.QueryRow(context.Background(), `SELECT hash, ip::text, name FROM chatlog WHERE msg = $1 LIMIT 1`,
			confirmcode).Scan(&loghash, &logip, &logname)
		if derr != nil && derr != pgx.ErrNoRows {
			log.Println(derr.Error())
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database error: " + derr.Error()})
			return
		}
		if derr == pgx.ErrNoRows {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{
				"msg": "You are not linked to any profile. Send \"" + confirmcode + "\" to any autohoster room to link selected profile.",
			})
			return
		}
		var newwzid int
		log.Printf("link [%s] [%s] [%s] [%s]", confirmcode, loghash, logname, logip)
		derr = dbpool.QueryRow(context.Background(), `
			INSERT INTO players as p (name, hash, asocip)
			VALUES ($1::text, $2::text, ARRAY[$3::inet])
			ON CONFLICT (hash) DO
				UPDATE SET name = $1::text, asocip = array_sort_unique(p.asocip || ARRAY[$3::inet])
			RETURNING id;`,
			logname, loghash, logip).Scan(&newwzid)
		if derr != nil {
			log.Println(derr.Error())
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database error: " + derr.Error()})
			return
		}
		tag, derr := dbpool.Exec(context.Background(), `UPDATE users SET wzconfirmcode = '', wzprofile2 = $1 WHERE username = $2`,
			newwzid, sessionManager.GetString(r.Context(), "User.Username"))
		if derr != nil {
			log.Println(derr.Error())
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database error: " + derr.Error()})
			return
		}
		if tag.RowsAffected() != 1 {
			log.Println(derr.Error())
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database update error, rows affected " + string(tag)})
			return
		}
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{
			"msg": "We successfully linked your account to warzone profile (" + strconv.Itoa(newwzid) + ") " + logname + " [" + loghash + "]",
		})
		return
	} else {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{
			"msg": "id " + strconv.Itoa(profilenum) + " code [" + confirmcode + "]",
		})
		return
	}
}

func hostRequestHandler(w http.ResponseWriter, r *http.Request) {
	if !sessionManager.Exists(r.Context(), "User.Username") || sessionManager.Get(r.Context(), "UserAuthorized") != "True" {
		basicLayoutLookupRespond("noauth", w, r, map[string]interface{}{})
		return
	}
	s, mhstatus := RequestStatus()
	if !s {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msg": "Multihoster unavaliable"})
		return
	}
	var allow_any bool
	var allow_presets bool
	derr := dbpool.QueryRow(context.Background(),
		`SELECT allow_host_request, allow_preset_request FROM users WHERE username = $1`,
		sessionManager.GetString(r.Context(), "User.Username")).Scan(&allow_any, &allow_presets)
	if !(allow_any || allow_presets) {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Sorry, you are not allowed to request games."})
		return
	}
	rows, derr := dbpool.Query(context.Background(), `
	SELECT
		id,
		maphash,
		mapname,
		players,
		levelbase,
		alliances,
		scav,
		last_requested,
		disallowed_users
	FROM presets
	WHERE NOT ((SELECT id FROM users WHERE username = $1) = ANY(coalesce(disallowed_users, array[]::int[])))
	ORDER BY last_requested DESC
	`, sessionManager.GetString(r.Context(), "User.Username"))
	if derr != nil {
		if derr == pgx.ErrNoRows {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msg": "No maps avaliable"})
		} else {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database query error: " + derr.Error()})
		}
		return
	}
	defer rows.Close()
	type RequestPrototype struct {
		NumID          int
		ID             int
		MapHash        string
		MapName        string
		Playercount    int
		LevelBase      int
		LevelAlliances int
		LevelScav      int
		LastRequested  time.Time
		ForbiddenUsers []int
	}
	var pres []RequestPrototype
	IID := 0
	for rows.Next() {
		var n RequestPrototype
		err := rows.Scan(&n.ID, &n.MapHash, &n.MapName, &n.Playercount, &n.LevelBase, &n.LevelAlliances, &n.LevelScav, &n.LastRequested, &n.ForbiddenUsers)
		if err != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database scan error: " + err.Error()})
			return
		}
		n.NumID = IID
		pres = append(pres, n)
		IID++
	}
	basicLayoutLookupRespond("multihoster-templates", w, r, map[string]interface{}{
		"MultihosterStatus": template.HTML(mhstatus),
		"Presets":           pres,
		"RandomSelection":   rand.Intn(len(pres)),
	})
}

func createdRoomsHandler(w http.ResponseWriter, r *http.Request) {
	if !sessionManager.Exists(r.Context(), "User.Username") || sessionManager.Get(r.Context(), "UserAuthorized") != "True" {
		basicLayoutLookupRespond("noauth", w, r, map[string]interface{}{})
		return
	}
	s, reqres := RequestHosters()
	if s {
		basicLayoutLookupRespond("multihoster", w, r, map[string]interface{}{"MultihosterStatus": reqres})
	} else {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Request error: " + reqres})
	}
}

func RequestHosters() (bool, string) {
	req, err := http.NewRequest("GET", os.Getenv("MULTIHOSTER_URLBASE")+"hosters-online", nil)
	if err != nil {
		log.Print(err)
		return false, "Error creating request"
	}
	var netClient = &http.Client{
		Timeout: time.Second * 2,
	}
	resp, err := netClient.Do(req)
	if err != nil {
		log.Print(err)
		return false, "Error executing request"
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
		return false, "Error reading response"
	}
	bodyString := string(bodyBytes)
	return true, bodyString
}

func RequestStatus() (bool, string) {
	req, err := http.NewRequest("GET", os.Getenv("MULTIHOSTER_URLBASE")+"status", nil)
	if err != nil {
		log.Print(err)
		return false, "Error creating request"
	}
	var netClient = &http.Client{
		Timeout: time.Second * 2,
	}
	resp, err := netClient.Do(req)
	if err != nil {
		log.Print(err)
		return false, "Error executing request"
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
		return false, "Error reading response"
	}
	bodyString := string(bodyBytes)
	return true, bodyString
}

func RequestHost(maphash, mapname, alliances, base, scav, players, admin, name string) (bool, string) {
	req, err := http.NewRequest("GET", os.Getenv("MULTIHOSTER_URLBASE")+"request-room", nil)
	if err != nil {
		log.Print(err)
		return false, "Error creating request"
	}
	q := req.URL.Query()
	q.Add("maphash", maphash)
	q.Add("mapname", mapname)
	q.Add("alliances", alliances)
	q.Add("base", base)
	q.Add("scav", scav)
	q.Add("maxplayers", players)
	q.Add("adminhash", admin)
	q.Add("roomname", name)
	req.URL.RawQuery = q.Encode()
	var netClient = &http.Client{
		Timeout: time.Second * 2,
	}
	resp, err := netClient.Do(req)
	if err != nil {
		log.Print(err)
		return false, "Error executing request"
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
		return false, "Error reading response"
	}
	bodyString := string(bodyBytes)
	return true, bodyString
}
