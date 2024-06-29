package main

import (
	"context"
	"encoding/json"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
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

func hosterHandler(w http.ResponseWriter, r *http.Request) {
	if checkUserAuthorized(r) {
		basicLayoutLookupRespond("noauth", w, r, map[string]any{})
		return
	}
	if r.Method == "POST" {
		err := r.ParseForm()
		if err != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Form parse error: " + err.Error()})
			return
		}
		if !regexMaphash.MatchString(r.PostFormValue("maphash")) {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "maphash must match `^[a-zA-Z0-9-]*$`"})
			return
		}
		if !regexAlliances.MatchString(r.PostFormValue("alliances")) {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "alliances must match `^[0-2]$`"})
			return
		}
		if !regexLevelbase.MatchString(r.PostFormValue("base")) {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "base must match `^[0-2]$`"})
			return
		}
		if !regexScav.MatchString(r.PostFormValue("scav")) {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "scav must match `^[0-1]$`, got [" + r.PostFormValue("scav") + "]"})
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
			(SELECT coalesce(extract(epoch from last_host_request), 0) FROM accounts WHERE username = $2)::text as last_host_request,
			(SELECT allow_preset_request FROM accounts WHERE username = $2) as allow_preset_request,
			coalesce((SELECT hash FROM players WHERE id = (SELECT coalesce(wzprofile2, 0) FROM accounts WHERE username = $2)), '') as adminhash
		FROM presets
		WHERE maphash = $1 AND NOT ((SELECT id FROM accounts WHERE username = $2) = ANY(coalesce(disallowed_accounts, array[]::int[])))
		LIMIT 1`, r.PostFormValue("maphash"), sessionManager.GetString(r.Context(), "User.Username")).Scan(
			&mapname, &allowedalliances, &allowedbase, &allowedscav, &numplayers, &laststr, &allow_preset_request, &adminhash)
		if derr != nil {
			if derr == pgx.ErrNoRows {
				basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Selected map is not allowed."})
				w.Header().Set("Refresh", "3; /request")
				return
			}
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Database error: " + derr.Error()})
			return
		}
		if !allow_preset_request {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Sorry, you are not allowed to request games."})
			return
		}

		lastfloat, cerr := strconv.ParseFloat(laststr, 64)
		if cerr != nil {
			log.Println(cerr.Error())
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Database timestamp error: " + cerr.Error()})
			return
		}
		lastint := int64(lastfloat)
		if lastint-10800+300 > time.Now().Unix() && sessionManager.GetString(r.Context(), "User.Username") != "Flex seal" {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true,
				"msg": "Please wait before requesting another room. " + strconv.FormatInt(300-(time.Now().Unix()-(lastint-10800)), 10) + " seconds left."})
			return
		}

		tag, derr := dbpool.Exec(context.Background(), "UPDATE accounts SET last_host_request = now() WHERE username = $1", sessionManager.GetString(r.Context(), "User.Username"))
		if derr != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Database error: " + derr.Error()})
			return
		}
		if tag.RowsAffected() != 1 {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Database update error, rows affected " + string(tag)})
			return
		}

		_, derr = dbpool.Exec(context.Background(), "UPDATE presets SET last_requested = now() WHERE maphash = $1", r.PostFormValue("maphash"))
		if derr != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Database error: " + derr.Error()})
			return
		}
		// if tag.RowsAffected() != 1 {
		// 	basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Database update error, rows affected " + string(tag)})
		// 	return
		// }
		gamever := r.PostFormValue("gamever")
		k, versraw := RequestVersions()
		vers := map[string]any{}
		if k {
			if err := json.Unmarshal([]byte(versraw), &vers); err != nil {
				basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Json parse error: " + err.Error()})
				return
			}
		} else {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": versraw})
		}
		vershave := false
		for _, nextver := range vers["versions"].([]any) {
			nextvers := nextver.(string)
			if nextvers == gamever {
				vershave = true
				break
			}
		}
		if !vershave {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Game version is not present"})
			return
		}
		roomname := r.PostFormValue("roomname")
		if roomname == "" {
			roomname = "Autohoster"
		}
		mixmod := ""
		// if r.PostFormValue("AddSpecs") == "on" {
		// 	mixmod = "spec"
		// }
		if r.PostFormValue("AddBalance") == "on" {
			mixmod += "masterbal"
		}
		onlyregistered := "0"
		if r.PostFormValue("onlyregistered") == "on" {
			onlyregistered = "1"
		}
		alliancen, err := strconv.Atoi(r.PostFormValue("alliances"))
		if err != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Malformed alliances field: " + err.Error()})
			return
		}
		if alliancen > 0 {
			alliancen++
		}
		basen, err := strconv.Atoi(r.PostFormValue("base"))
		if err != nil {
			basicLayoutLookupRespond("error400", w, r, map[string]any{})
			return
		}
		s, reqres := RequestHost(r.PostFormValue("maphash"),
			mapname, strconv.FormatInt(int64(alliancen), 10), strconv.FormatInt(int64(basen), 10),
			r.PostFormValue("scav"), strconv.FormatInt(int64(numplayers), 10), adminhash, roomname, mixmod, gamever, onlyregistered)
		log.Printf("Host request: [%s] [%s]", sessionManager.Get(r.Context(), "User.Username"), mapname)
		if s {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msggreen": true, "msg": reqres})
		} else {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Request error: " + reqres})
		}
		w.Header().Set("Refresh", "10; /lobby")
	} else {
		s, reqres := RequestStatus()
		if s {
			basicLayoutLookupRespond("multihoster", w, r, map[string]any{
				"MultihosterStatus": template.HTML(reqres),
			})
		} else {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Request error: " + reqres})
		}
	}
}

func wzlinkCheckHandler(w http.ResponseWriter, r *http.Request) {
	if !checkUserAuthorized(r) {
		basicLayoutLookupRespond("noauth", w, r, map[string]any{})
		return
	}
	// blockedRegions := strings.Split(cfg.GetDString("", "requireDiscordLink", "regions"), " ")
	// if stringOneOf(r.Header.Get("CF-IPCountry"), blockedRegions...) {
	// 	basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Please contact administrator to link your profile."})
	// 	return
	// }
	var confirmcode string
	err := dbpool.QueryRow(r.Context(), `SELECT coalesce(wz_confirm_code, '') FROM accounts WHERE username = $1`, sessionGetUsername(r)).Scan(&confirmcode)
	if err != nil {
		if err == pgx.ErrNoRows {
			w.Header().Set("Refresh", "1; /logout")
			return
		}
		log.Printf("Error fetching wz_confirm_code: %s", err.Error())
		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Database error: " + err.Error()})
		return
	}
	if confirmcode == "" {
		confirmcode = "confirm-" + generateRandomString(18)
		_, err := dbpool.Exec(r.Context(), `update accounts set wz_confirm_code = $1 where username = $2`, confirmcode, sessionGetUsername(r))
		if err != nil {
			log.Printf("Error updating wz_confirm_code: %s", err.Error())
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Database error: " + err.Error()})
			return
		}
		basicLayoutLookupRespond("wzlinkcheck", w, r, map[string]any{"LinkStatus": "code", "WzConfirmCode": "/hostmsg " + confirmcode})
		return
	}
	var logname string
	var logkey []byte
	err = dbpool.QueryRow(context.Background(), `select name, pkey from chatlog where msg = $1 limit 1`,
		"/hostmsg "+confirmcode).Scan(&logname, &logkey)
	if err != nil {
		if err == pgx.ErrNoRows {
			basicLayoutLookupRespond("wzlinkcheck", w, r, map[string]any{"LinkStatus": "code", "WzConfirmCode": "/hostmsg " + confirmcode})
			return
		}
		log.Printf("Error selecting chatlog: %s", err.Error())
		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Database error: " + err.Error()})
		return
	}
	tag, err := dbpool.Exec(context.Background(), `
		insert into identities (name, pkey, hash, account)
		values ($1, $2, encode(sha256($2), 'hex'), $3)
		on conflict (hash) do update set account = $3 where identities.account is null and identities.pkey = $2`,
		logname, logkey, sessionGetUserID(r))
	if err != nil {
		log.Printf("Error inserting identity: %s", err.Error())
		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Database error: " + err.Error()})
		return
	}
	if tag.Update() && tag.RowsAffected() == 0 {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Tyou attempted to link already claimed identity, this is not allowed."})
		return
	}
	_, err = dbpool.Exec(context.Background(), `update accounts set wz_confirm_code = null, display_name = $1 where username = $2`, logname, sessionGetUsername(r))
	if err != nil {
		log.Printf("Error clearing confirm code: %s", err.Error())
		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Database error: " + err.Error()})
		return
	}
	basicLayoutLookupRespond("wzlinkcheck", w, r, map[string]any{"LinkStatus": "done", "PlayerKey": logkey, "PlayerName": logname})
}

func wzlinkHandler(w http.ResponseWriter, r *http.Request) {
	if !checkUserAuthorized(r) {
		basicLayoutLookupRespond("noauth", w, r, map[string]any{})
		return
	}
	idt := []struct {
		ID      int
		Name    string
		Pkey    []byte
		Hash    string
		Account int
	}{}
	var (
		ID      int
		Name    string
		Pkey    []byte
		Hash    string
		Account int
	)
	_, err := dbpool.QueryFunc(r.Context(), `select id, name, pkey, hash, account from identities where account = $1`, []any{sessionGetUserID(r)},
		[]any{&ID, &Name, &Pkey, &Hash, &Account}, func(qfr pgx.QueryFuncRow) error {
			idt = append(idt, struct {
				ID      int
				Name    string
				Pkey    []byte
				Hash    string
				Account int
			}{
				ID:      ID,
				Name:    Name,
				Pkey:    Pkey,
				Hash:    Hash,
				Account: Account,
			})
			return nil
		})
	if err != nil && err != pgx.ErrNoRows {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Database error: " + err.Error()})
		return
	}
	basicLayoutLookupRespond("wzlink", w, r, map[string]any{
		"Identities": idt,
	})
}

func hostRequestHandler(w http.ResponseWriter, r *http.Request) {
	if !sessionManager.Exists(r.Context(), "User.Username") || sessionManager.Get(r.Context(), "UserAuthorized") != "True" {
		basicLayoutLookupRespond("noauth", w, r, map[string]any{})
		return
	}
	s, mhstatus := RequestStatus()
	if !s {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msg": "Multihoster unavaliable"})
		return
	}
	var allow_any bool
	var allow_presets bool
	var norequest_reason string
	derr := dbpool.QueryRow(context.Background(),
		`SELECT allow_host_request, allow_preset_request, norequest_reason FROM accounts WHERE username = $1`,
		sessionManager.GetString(r.Context(), "User.Username")).Scan(&allow_any, &allow_presets, &norequest_reason)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msg": "Unauthorized?!"})
			sessionManager.Destroy(r.Context())
		} else {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Database query error: " + derr.Error()})
		}
		return
	}
	if !(allow_any || allow_presets) {
		basicLayoutLookupRespond("errornorequest", w, r, map[string]any{"ForbiddenReason": norequest_reason})
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
		disallowed_accounts
	FROM presets
	WHERE NOT ((SELECT id FROM accounts WHERE username = $1) = ANY(coalesce(disallowed_accounts, array[]::int[])))
	ORDER BY last_requested DESC
	`, sessionManager.GetString(r.Context(), "User.Username"))
	if derr != nil {
		if derr == pgx.ErrNoRows {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msg": "No maps avaliable"})
		} else {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Database query error: " + derr.Error()})
		}
		return
	}
	defer rows.Close()
	type RequestPrototype struct {
		NumID             int
		ID                int
		MapHash           string
		MapName           string
		Playercount       int
		LevelBase         int
		LevelAlliances    int
		LevelScav         int
		LastRequested     time.Time
		Forbiddenaccounts []int
	}
	var pres []RequestPrototype
	IID := 0
	for rows.Next() {
		var n RequestPrototype
		err := rows.Scan(&n.ID, &n.MapHash, &n.MapName, &n.Playercount, &n.LevelBase, &n.LevelAlliances, &n.LevelScav, &n.LastRequested, &n.Forbiddenaccounts)
		if err != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Database scan error: " + err.Error()})
			return
		}
		n.NumID = IID
		pres = append(pres, n)
		IID++
	}
	k, versraw := RequestVersions()
	vers := map[string]any{}
	if k {
		if err := json.Unmarshal([]byte(versraw), &vers); err != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Json parse error: " + err.Error()})
			return
		}
	} else {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": versraw})
	}
	basicLayoutLookupRespond("multihoster-templates", w, r, map[string]any{
		"MultihosterStatus": template.HTML(mhstatus),
		"Presets":           pres,
		"Versions":          vers,
		"RandomSelection":   rand.Intn(len(pres)),
	})
}

//lint:ignore U1000 for dedicated rooms page
func createdRoomsHandler(w http.ResponseWriter, r *http.Request) {
	if !sessionManager.Exists(r.Context(), "User.Username") || sessionManager.Get(r.Context(), "UserAuthorized") != "True" {
		basicLayoutLookupRespond("noauth", w, r, map[string]any{})
		return
	}
	s, reqres := RequestHosters()
	var rooms []any
	err := json.Unmarshal([]byte(reqres), &rooms)
	if err != nil {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "JSON error: " + err.Error()})
		return
	}
	if s {
		basicLayoutLookupRespond("rooms", w, r, map[string]any{"Rooms": rooms})
	} else {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Request error: " + reqres})
	}
}

func MultihosterRequest(url string) (bool, string) {
	req, err := http.NewRequest("GET", cfg.GetDSString("http://localhost:34206/", "multihoster", "urlBase")+url, nil)
	if err != nil {
		log.Print(err)
		return false, err.Error()
	}
	var netClient = &http.Client{
		Timeout: time.Second * 2,
	}
	resp, err := netClient.Do(req)
	if err != nil {
		log.Print(err)
		return false, err.Error()
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
		return false, err.Error()
	}
	bodyString := string(bodyBytes) + "\n"
	return true, bodyString
}

func RequestHosters() (bool, string) {
	return MultihosterRequest("hosters-online")
}

func RequestVersions() (bool, string) {
	return MultihosterRequest("wzversions")
}

func RequestStatus() (bool, string) {
	return MultihosterRequest("status")
}

func RequestHost(maphash, mapname, alliances, base, scav, players, admin, name, mods, ver, onlyregistered string) (bool, string) {
	req, err := http.NewRequest("GET", cfg.GetDSString("http://localhost:34206/", "multihoster", "urlBase")+"request-room", nil)
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
	q.Add("mod", mods)
	q.Add("version", ver)
	q.Add("onlyregistered", onlyregistered)
	req.URL.RawQuery = q.Encode()
	var netClient = &http.Client{
		Timeout: time.Second * 2,
	}
	resp, err := netClient.Do(req)
	if err != nil {
		log.Print(err)
		return false, "Error executing request"
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
		return false, "Error reading response"
	}
	bodyString := string(bodyBytes)
	return true, bodyString
}
