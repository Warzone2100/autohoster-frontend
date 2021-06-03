package main

import (
	"context"
	"encoding/json"
	_ "fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

var VKRedirectUrl = "https://tacticalpepe.me/oauth/vk"

func VKGetUrl(state string) string {
	return "https://oauth.vk.com/authorize?client_id=" + os.Getenv("VKCLIENTID") + "&display=popup&redirect_uri=" + VKRedirectUrl + "&scope=friends,offline&response_type=code&v=5.131&state=" + state
}

func VKGetUInfo(token string) map[string]interface{} {
	var client = &http.Client{Timeout: time.Second * 2}
	req, err := http.NewRequest(http.MethodGet, "https://api.vk.com/method/users.get", nil)
	if err != nil {
		log.Print(err)
		return map[string]interface{}{}
	}
	q := req.URL.Query()
	q.Add("fields", "photo_400_orig,city")
	q.Add("access_token", token)
	req.URL.RawQuery = q.Encode()
	client.Do(req)
	resp, err := client.Do(req)
	if err != nil {
		log.Print(err)
		return map[string]interface{}{}
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
		return map[string]interface{}{}
	}
	u := make(map[string]interface{})
	err = json.Unmarshal(bodyBytes, &u)
	if err != nil {
		log.Print(err)
		return map[string]interface{}{}
	}
	return u
}

func VKCallbackHandler(w http.ResponseWriter, r *http.Request) {
	if !sessionManager.Exists(r.Context(), "UserAuthorized") || sessionManager.Get(r.Context(), "UserAuthorized") != "True" || !sessionManager.Exists(r.Context(), "User.Username") {
		log.Println("Not authorized")
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": 1, "msg": "Not authorized"})
		return
	}
	if sessionManager.Get(r.Context(), "User.VK.State") != r.FormValue("state") {
		log.Println("Code missmatch")
		var st string
		st = sessionManager.GetString(r.Context(), "User.VK.State")
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": 1, "msg": "State missmatch " + st})
		return
	}

	var client = &http.Client{Timeout: time.Second * 3}
	req, err := http.NewRequest(http.MethodGet, "https://oauth.vk.com/access_token", nil)
	if err != nil {
		log.Print(err)
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": 1, "msg": "Error creating request."})
		return
	}
	q := req.URL.Query()
	q.Add("client_id", os.Getenv("VKCLIENTID"))
	q.Add("client_secret", os.Getenv("VKCLIENTSECRET"))
	q.Add("redirect_uri", "https://tacticalpepe.me/oauth/vk")
	q.Add("code", r.FormValue("code"))
	req.URL.RawQuery = q.Encode()
	log.Println(req.URL.RawQuery)
	client.Do(req)
	resp, err := client.Do(req)
	if err != nil {
		log.Print(err)
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": 1, "msg": "Can not send code exchange request."})
		return
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": 1, "msg": "Can not read vk response."})
		return
	}
	vk := make(map[string]interface{})
	if err = json.Unmarshal(bodyBytes, &vk); err != nil {
		log.Print(err)
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": 1, "msg": "Can not parse json."})
		return
	}
	if err, p := vk["error"]; p {
		log.Println(err)
		log.Println(vk["error_description"])
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": 1, "msg": "Code exchange failed"})
		return
	}
	token, p1 := vk["access_token"]
	uid, p2 := vk["user_id"]
	refresh_date, p3 := vk["expires_in"]
	if !p1 || !p2 || !p3 {
		log.Println("Map is wrong")
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": 1, "msg": "Wrong data recieved from vk."})
		return
	}
	if refresh_date != "0" {
		log.Println("offline scope is not offline")
	}
	tag, derr := dbpool.Exec(context.Background(), "UPDATE users SET vk_token = $1, vk_uid = $2 WHERE username = $4", token, uid, sessionManager.Get(r.Context(), "User.Username"))
	if derr != nil {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": 1, "msg": "Database call error: " + derr.Error()})
		return
	}
	if tag.RowsAffected() != 1 {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": 1, "msg": "Database update error, rows affected " + string(tag)})
		return
	}
	log.Println("Got token")
	basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msggreen": 1, "msg": "VK linked."})
}
