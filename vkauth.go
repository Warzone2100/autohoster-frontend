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

	"golang.org/x/oauth2"
	vkAuth "golang.org/x/oauth2/vk"
)

var VKRedirectUrl = "https://tacticalpepe.me/oauth/vk"

var vkOauthConfig = &oauth2.Config{
	RedirectURL:  VKRedirectUrl,
	ClientID:     os.Getenv("VKCLIENTID"),
	ClientSecret: os.Getenv("VKCLIENTSECRET"),
	Scopes:       []string{},
	Endpoint:     vkAuth.Endpoint,
}

func VKVerifyEnv() {
	vkOauthConfig.ClientID = os.Getenv("VKCLIENTID")
	vkOauthConfig.ClientSecret = os.Getenv("VKCLIENTSECRET")
}

func VKGetUrl(state string) string {
	return VKRedirectUrl
}

func VKGetUInfo(token string) map[string]interface{} {
	var client = &http.Client{Timeout: time.Second * 2}
	req, err := http.NewRequest("GET", "https://api.vk.com/method/users.get", nil)
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
	if !sessionManager.Exists(r.Context(), "UserAuthorized") || sessionManager.Get(r.Context(), "UserAuthorized") != "True" {
		log.Println("Not authorized")
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": 1, "msg": "Not authorized"})
		return
	}
	if !sessionManager.Exists(r.Context(), "User.Username") {
		log.Println("Not authorized (no username)")
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": 1, "msg": "Not authorized (no username)"})
		return
	}
	code := r.FormValue("code")
	if sessionManager.Get(r.Context(), "User.VK.State") != r.FormValue("state") {
		log.Println("Code missmatch")
		var st string
		st = sessionManager.GetString(r.Context(), "User.VK.State")
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": 1, "msg": "State missmatch " + st})
		return
	}
	token, err := vkOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		log.Println("Code exchange failed with error %s\n", err.Error())
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": 1, "msg": "Code exchange failed with error: " + err.Error()})
		return
	}
	if !token.Valid() {
		log.Println("Retreived invalid token")
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": 1, "msg": "Retreived invalid token"})
		return
	}
	tag, derr := dbpool.Exec(context.Background(), "UPDATE users SET vk_token = $1, vk_refresh = $2, vk_refresh_date = $3 WHERE username = $4", token.AccessToken, token.RefreshToken, token.Expiry, sessionManager.Get(r.Context(), "User.Username"))
	if derr != nil {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": 1, "msg": "Database call error: " + derr.Error()})
		return
	}
	if tag.RowsAffected() != 1 {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": 1, "msg": "Database insert error, rows affected " + string(tag)})
		return
	}
	log.Println("Got token")
	basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msggreen": 1, "msg": "VK linked."})
}
