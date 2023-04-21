package main

import (
	"context"
	"encoding/json"
	"html/template"
	"io"
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
		if !allow_preset_request {
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

		_, derr = dbpool.Exec(context.Background(), "UPDATE presets SET last_requested = now() WHERE maphash = $1", r.PostFormValue("maphash"))
		if derr != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database error: " + cerr.Error()})
			return
		}
		// if tag.RowsAffected() != 1 {
		// 	basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database update error, rows affected " + string(tag)})
		// 	return
		// }
		gamever := r.PostFormValue("gamever")
		k, versraw := RequestVersions()
		vers := map[string]interface{}{}
		if k {
			if err := json.Unmarshal([]byte(versraw), &vers); err != nil {
				basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Json parse error: " + err.Error()})
				return
			}
		} else {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": versraw})
		}
		vershave := false
		for _, nextver := range vers["versions"].([]interface{}) {
			nextvers := nextver.(string)
			if nextvers == gamever {
				vershave = true
				break
			}
		}
		if !vershave {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Game version is not present"})
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
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Malformed alliances field: " + err.Error()})
			return
		}
		if alliancen > 0 {
			alliancen++
		}
		basen, err := strconv.Atoi(r.PostFormValue("base"))
		if err != nil {
			basicLayoutLookupRespond("error400", w, r, map[string]interface{}{})
			return
		}
		s, reqres := RequestHost(r.PostFormValue("maphash"),
			mapname, strconv.FormatInt(int64(alliancen), 10), strconv.FormatInt(int64(basen), 10),
			r.PostFormValue("scav"), strconv.FormatInt(int64(numplayers), 10), adminhash, roomname, mixmod, gamever, onlyregistered)
		log.Printf("Host request: [%s] [%s]", sessionManager.Get(r.Context(), "User.Username"), mapname)
		if s {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msggreen": true, "msg": reqres})
		} else {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Request error: " + reqres})
		}
		w.Header().Set("Refresh", "10; /lobby")
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

func wzlinkCheckHandler(w http.ResponseWriter, r *http.Request) {
	if !sessionManager.Exists(r.Context(), "User.Username") || sessionManager.Get(r.Context(), "UserAuthorized") != "True" {
		basicLayoutLookupRespond("noauth", w, r, map[string]interface{}{})
		return
	}
	var allow_profile_merge bool
	var profilenum int
	var confirmcode string
	derr := dbpool.QueryRow(context.Background(), `SELECT coalesce(wzprofile2, -1), coalesce(wzconfirmcode, ''), allow_profile_merge FROM users WHERE username = $1`,
		sessionManager.GetString(r.Context(), "User.Username")).Scan(&profilenum, &confirmcode, &allow_profile_merge)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Heeee?"})
			w.Header().Set("Refresh", "1; /logout")
			return
		}
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database error: " + derr.Error()})
		return
	}
	if confirmcode != "" && (allow_profile_merge || profilenum == -1) {
		var loghash, logname, logip string
		derr := dbpool.QueryRow(context.Background(), `SELECT hash, ip::text, name FROM chatlog WHERE msg = $1 LIMIT 1`,
			confirmcode).Scan(&loghash, &logip, &logname)
		if derr != nil && derr != pgx.ErrNoRows {
			log.Println(derr.Error())
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database error: " + derr.Error()})
			return
		}
		if derr != pgx.ErrNoRows {
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
			if profilenum == -1 {
				tag, derr := dbpool.Exec(context.Background(), `UPDATE users SET wzconfirmcode = '', wzprofile2 = $1 WHERE username = $2`,
					newwzid, sessionManager.GetString(r.Context(), "User.Username"))
				if derr != nil {
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
						"msg": template.HTML(`Profile already linked to an account. Contact Autohoster Administrator to resolve a conflict.
								<br>We created new link code for you: <code>` + newmsg + `</code>
								<br>Send it to any Autohoster room with profile you want to link selected.
								<br>Refresh this page after you sent the message.`),
					})
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
				if allow_profile_merge {
					if newwzid == profilenum {
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
							"msg": template.HTML("You tried to merge same profile into existing one. That does nothing. New code: <code>" + newmsg + "</code>"),
						})
						return
					}
					tag, derr := dbpool.Exec(context.Background(), `UPDATE games SET players = array_replace(players, $1, $2) WHERE $1 = ANY(players)`,
						newwzid, profilenum)
					if derr != nil {
						log.Println(derr.Error())
						basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database error: " + derr.Error()})
						return
					}
					tag2, derr2 := dbpool.Exec(context.Background(), `UPDATE users SET wzconfirmcode = '' WHERE username = $1`,
						sessionManager.GetString(r.Context(), "User.Username"))
					if derr2 != nil {
						log.Println(derr.Error())
						basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database error: " + derr.Error()})
						return
					}
					if tag2.RowsAffected() != 1 {
						log.Println(derr.Error())
						basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database update error, rows affected " + string(tag)})
						return
					}
					basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{
						"msg": template.HTML("We successfully merged new profile " + strconv.Itoa(newwzid) + " (" + logname + ") into " + strconv.Itoa(profilenum) + "<br>Games affected: " + strconv.Itoa(int(tag.RowsAffected()))),
					})
					return
				} else {
					basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{
						"msg": "You tried to merge new profile into existing one but it is not allowed.",
					})
					return
				}
			}
		}

	}
	if profilenum == -1 {
		if confirmcode == "" {
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
				"msg": template.HTML("We created new link code for you: <code>" + newmsg + "</code><br>Send it to any Autohoster room with profile you want to link selected.<br>Refresh this page after you sent the message."),
			})
		} else {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{
				"msg": template.HTML("Your link code is: <code>" + confirmcode + "</code><br>Send it to any Autohoster room with profile you want to link selected.<br>Refresh this page after you sent the message."),
			})
		}
	} else {
		if allow_profile_merge {
			if confirmcode == "" {
				newmsg := "confirm-" + generateRandomString(18)
				confirmcode = newmsg
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
			}
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{
				"msg": template.HTML("You are linked to profile " + strconv.Itoa(profilenum) + "<br>If you want to merge profiles into existing one your link code is: <code>" + confirmcode + "</code><br>Send it to any Autohoster room with profile you want to link selected.<br>Refresh this page after you sent the message."),
			})
		} else {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{
				"msg": template.HTML("Sorry, you can not merge any profiles. Contact us to learn more about profile linkage."),
			})
		}
	}
}

func wzlinkHandler(w http.ResponseWriter, r *http.Request) {
	if !sessionManager.Exists(r.Context(), "User.Username") || sessionManager.Get(r.Context(), "UserAuthorized") != "True" {
		basicLayoutLookupRespond("noauth", w, r, map[string]interface{}{})
		return
	}
	var allow_profile_merge bool
	var profilenum int
	var confirmcode string
	derr := dbpool.QueryRow(context.Background(), `SELECT coalesce(wzprofile2, -1), coalesce(wzconfirmcode, ''), allow_profile_merge FROM users WHERE username = $1`,
		sessionManager.GetString(r.Context(), "User.Username")).Scan(&profilenum, &confirmcode, &allow_profile_merge)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Heeee?"})
			w.Header().Set("Refresh", "1; /logout")
			return
		}
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database error: " + derr.Error()})
		return
	}
	basicLayoutLookupRespond("wzlink", w, r, map[string]interface{}{
		"AllowProfileMerge": allow_profile_merge,
	})

	// if confirmcode == "" && profilenum != -1 {
	// } else if confirmcode == "" && profilenum == -1 {
	// 	newmsg := "confirm-" + generateRandomString(18)
	// 	tag, derr := dbpool.Exec(context.Background(), `UPDATE users SET wzconfirmcode = $1 WHERE username = $2`,
	// 		newmsg, sessionManager.GetString(r.Context(), "User.Username"))
	// 	if derr != nil {
	// 		log.Println(derr.Error())
	// 		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database error: " + derr.Error()})
	// 		return
	// 	}
	// 	if tag.RowsAffected() != 1 {
	// 		log.Println(derr.Error())
	// 		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database update error, rows affected " + string(tag)})
	// 		return
	// 	}
	// 	basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{
	// 		"msg": "You are not linked to any profile. We generated a link key for you. Send \"" + newmsg + "\" to any autohoster room to link selected profile.",
	// 	})
	// } else if confirmcode != "" && profilenum == -1 {
	// 	var loghash string
	// 	var logname string
	// 	var logip string
	// 	derr := dbpool.QueryRow(context.Background(), `SELECT hash, ip::text, name FROM chatlog WHERE msg = $1 LIMIT 1`,
	// 		confirmcode).Scan(&loghash, &logip, &logname)
	// 	if derr != nil && derr != pgx.ErrNoRows {
	// 		log.Println(derr.Error())
	// 		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database error: " + derr.Error()})
	// 		return
	// 	}
	// 	if derr == pgx.ErrNoRows {
	// 		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{
	// 			"msg": "You are not linked to any profile. Send \"" + confirmcode + "\" to any autohoster room to link selected profile.",
	// 		})
	// 		return
	// 	}
	// 	var newwzid int
	// 	log.Printf("link [%s] [%s] [%s] [%s]", confirmcode, loghash, logname, logip)
	// 	derr = dbpool.QueryRow(context.Background(), `
	// 		INSERT INTO players as p (name, hash, asocip)
	// 		VALUES ($1::text, $2::text, ARRAY[$3::inet])
	// 		ON CONFLICT (hash) DO
	// 			UPDATE SET name = $1::text, asocip = array_sort_unique(p.asocip || ARRAY[$3::inet])
	// 		RETURNING id;`,
	// 		logname, loghash, logip).Scan(&newwzid)
	// 	if derr != nil {
	// 		log.Println(derr.Error())
	// 		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database error: " + derr.Error()})
	// 		return
	// 	}
	// 	tag, derr := dbpool.Exec(context.Background(), `UPDATE users SET wzconfirmcode = '', wzprofile2 = $1 WHERE username = $2`,
	// 		newwzid, sessionManager.GetString(r.Context(), "User.Username"))
	// 	if derr != nil {
	// 		log.Println(derr.Error())
	// 		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database error: " + derr.Error()})
	// 		return
	// 	}
	// 	if tag.RowsAffected() != 1 {
	// 		log.Println(derr.Error())
	// 		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database update error, rows affected " + string(tag)})
	// 		return
	// 	}
	// 	basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{
	// 		"msg": "We successfully linked your account to warzone profile (" + strconv.Itoa(newwzid) + ") " + logname + " [" + loghash + "]",
	// 	})
	// 	return
	// } else {
	// 	basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{
	// 		"msg": "id " + strconv.Itoa(profilenum) + " code [" + confirmcode + "]",
	// 	})
	// 	return
	// }
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
	var norequest_reason string
	derr := dbpool.QueryRow(context.Background(),
		`SELECT allow_host_request, allow_preset_request, norequest_reason FROM users WHERE username = $1`,
		sessionManager.GetString(r.Context(), "User.Username")).Scan(&allow_any, &allow_presets, &norequest_reason)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msg": "Unauthorized?!"})
			sessionManager.Destroy(r.Context())
		} else {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database query error: " + derr.Error()})
		}
		return
	}
	if !(allow_any || allow_presets) {
		basicLayoutLookupRespond("errornorequest", w, r, map[string]interface{}{"ForbiddenReason": norequest_reason})
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
	k, versraw := RequestVersions()
	vers := map[string]interface{}{}
	if k {
		if err := json.Unmarshal([]byte(versraw), &vers); err != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Json parse error: " + err.Error()})
			return
		}
	} else {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": versraw})
	}
	basicLayoutLookupRespond("multihoster-templates", w, r, map[string]interface{}{
		"MultihosterStatus": template.HTML(mhstatus),
		"Presets":           pres,
		"Versions":          vers,
		"RandomSelection":   rand.Intn(len(pres)),
	})
}

//lint:ignore U1000 for dedicated rooms page
func createdRoomsHandler(w http.ResponseWriter, r *http.Request) {
	if !sessionManager.Exists(r.Context(), "User.Username") || sessionManager.Get(r.Context(), "UserAuthorized") != "True" {
		basicLayoutLookupRespond("noauth", w, r, map[string]interface{}{})
		return
	}
	s, reqres := RequestHosters()
	var rooms []interface{}
	err := json.Unmarshal([]byte(reqres), &rooms)
	if err != nil {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "JSON error: " + err.Error()})
		return
	}
	if s {
		basicLayoutLookupRespond("rooms", w, r, map[string]interface{}{"Rooms": rooms})
	} else {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Request error: " + reqres})
	}
}

func MultihosterRequest(url string) (bool, string) {
	req, err := http.NewRequest("GET", os.Getenv("MULTIHOSTER_URLBASE")+url, nil)
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

func wzProfileRecoveryHandlerGET(w http.ResponseWriter, r *http.Request) {
	if !checkUserAuthorized(r) {
		basicLayoutLookupRespond("noauth", w, r, map[string]interface{}{})
		return
	}
	var profileID int
	var recoveryCode string
	var oldHash string
	var lastRecovery time.Time
	username := sessionGetUsername(r)
	log.Println("Recovery")
	log.Println("Username ", username)
	err := dbpool.QueryRow(context.Background(), `SELECT coalesce(wzprofile2, -1), coalesce((SELECT players.hash FROM players WHERE players.id = wzprofile2), ''), coalesce(profilerecoverycode, ''), lastprofilerecovery FROM users WHERE username = $1`,
		username).Scan(&profileID, &oldHash, &recoveryCode, &lastRecovery)
	if err != nil {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Error occured, please contact administratiors"})
		log.Println("Initial ", err)
		return
	}
	log.Println("Profile id ", profileID)
	log.Println("Old hash ", oldHash)
	if profileID <= -1 {
		w.Header().Add("Location", "/wzlinkcheck")
		w.WriteHeader(http.StatusFound)
		return
	}
	log.Println("Code ", recoveryCode)
	if recoveryCode == "" {
		recoveryCode = "recover-" + generateRandomString(20)
		log.Println("New code ", recoveryCode)
		tag, err := dbpool.Exec(context.Background(), `UPDATE users SET profilerecoverycode = $1 WHERE username = $2`, recoveryCode, username)
		if err != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Error occured, please contact administratiors"})
			log.Println("Update recovery code ", err)
			return
		}
		if !tag.Update() || tag.RowsAffected() != 1 {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Error occured, please contact administratiors"})
			log.Println("Update recovery code ", tag)
			return
		}
		basicLayoutLookupRespond("wzrecovery", w, r, map[string]interface{}{"RecoveryStatus": "code", "WzConfirmCode": recoveryCode})
		return
	}
	var newHash string
	err = dbpool.QueryRow(context.Background(), `SELECT hash FROM chatlog WHERE TRIM(msg) = $1 ORDER BY whensent ASC LIMIT 1`, recoveryCode).Scan(&newHash)
	if err != nil {
		if err == pgx.ErrNoRows {
			log.Println("Hash not found")
			basicLayoutLookupRespond("wzrecovery", w, r, map[string]interface{}{"RecoveryStatus": "code", "WzConfirmCode": recoveryCode})
			return
		}
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Error occured, please contact administratiors"})
		log.Println("Hash find ", err)
		return
	}
	log.Println("Found hash ", newHash)
	var collisionCheck int
	err = dbpool.QueryRow(context.Background(), `SELECT COUNT(*) FROM players WHERE hash = $1`, newHash).Scan(&collisionCheck)
	if err != nil {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Error occured, please contact administratiors"})
		log.Println("Collision check ", err)
		return
	}
	log.Println("Fresh check ", collisionCheck)
	if collisionCheck != 0 {
		recoveryCode = "recover-" + generateRandomString(20)
		log.Println("New code ", recoveryCode)
		tag, err := dbpool.Exec(context.Background(), `UPDATE users SET profilerecoverycode = $1 WHERE username = $2`, recoveryCode, username)
		if err != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Error occured, please contact administratiors"})
			log.Println("Update recovery code ", err)
			return
		}
		if !tag.Update() || tag.RowsAffected() != 1 {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Error occured, please contact administratiors"})
			log.Println("Update recovery code ", tag)
			return
		}
		basicLayoutLookupRespond("wzrecovery", w, r, map[string]interface{}{"RecoveryStatus": "collision", "WzConfirmCode": recoveryCode})
		return
	}
	tx, err := dbpool.Begin(context.Background())
	if err != nil {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Error occured, please contact administratiors"})
		log.Println("Begin ", err)
		return
	}
	tag, err := tx.Exec(context.Background(), `UPDATE players SET hash = $1 WHERE hash = $2;`, newHash, oldHash)
	if err != nil {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Error occured, please contact administratiors"})
		log.Println("Update last recovery date", err)
		err = tx.Rollback(context.Background())
		if err != nil {
			log.Println("Roll back ", err)
		}
		return
	}
	if !tag.Update() || tag.RowsAffected() != 1 {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Error occured, please contact administratiors"})
		log.Println("Update last recovery date", err)
		err = tx.Rollback(context.Background())
		if err != nil {
			log.Println("Roll back ", err)
		}
		return
	}
	tag, err = tx.Exec(context.Background(), `UPDATE users SET lastprofilerecovery = now(), profilerecoverycode = '' WHERE username = $1;`, username)
	if err != nil {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Error occured, please contact administratiors"})
		log.Println("Update last recovery date", err)
		err = tx.Rollback(context.Background())
		if err != nil {
			log.Println("Roll back ", err)
		}
		return
	}
	if !tag.Update() || tag.RowsAffected() != 1 {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Error occured, please contact administratiors"})
		log.Println("Update last recovery date", err)
		err = tx.Rollback(context.Background())
		if err != nil {
			log.Println("Roll back ", err)
		}
		return
	}
	err = tx.Commit(context.Background())
	if err != nil {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Error occured, please contact administratiors"})
		log.Println("Commit ", err)
		return
	}
	basicLayoutLookupRespond("wzrecovery", w, r, map[string]interface{}{"RecoveryStatus": "done", "OldHash": oldHash, "NewHash": newHash})
}
