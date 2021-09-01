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

	discord "github.com/ravener/discord-oauth2"
	"golang.org/x/oauth2"
)

var DiscordRedirectUrl = "https://wz2100-autohost.net/oauth/discord"

var discordOauthConfig = &oauth2.Config{
	RedirectURL:  DiscordRedirectUrl,
	ClientID:     os.Getenv("DISCORDCLIENTID"),
	ClientSecret: os.Getenv("DISCORDCLIENTSECRET"),
	Scopes: []string{
		"connections", "identify", "guilds", "email"},
	Endpoint: discord.Endpoint,
}

type DiscordUser struct {
	ID            string `json:"id"`
	Avatar        string `json:"avatar"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
}

func DiscordVerifyEnv() {
	discordOauthConfig.ClientID = os.Getenv("DISCORDCLIENTID")
	discordOauthConfig.ClientSecret = os.Getenv("DISCORDCLIENTSECRET")
}

func DiscordGetUrl(state string) string {
	return discordOauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func DiscordGetUInfo(token *oauth2.Token) map[string]interface{} {
	res, err := discordOauthConfig.Client(context.Background(), token).Get("https://discordapp.com/api/users/@me")
	if err != nil {
		log.Println("Unauthorized, resetting discord")
		token.AccessToken = ""
		token.RefreshToken = ""
		token.Expiry = time.Now()
		return map[string]interface{}{"DiscordError": "Error"}
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return map[string]interface{}{"DiscordError": err.Error()}
	}
	jsonMap := make(map[string]interface{})
	err = json.Unmarshal([]byte(body), &jsonMap)
	if err != nil {
		log.Println(err.Error())
	}
	return jsonMap
}

func DiscordCallbackHandler(w http.ResponseWriter, r *http.Request) {
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
	if sessionManager.Get(r.Context(), "User.Discord.State") != r.FormValue("state") {
		log.Println("Code missmatch")
		var st string
		st = sessionManager.GetString(r.Context(), "User.Discord.State")
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": 1, "msg": "State missmatch " + st})
		return
	}
	token, err := discordOauthConfig.Exchange(context.Background(), code)
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
	tag, derr := dbpool.Exec(context.Background(), "UPDATE users SET discord_token = $1, discord_refresh = $2, discord_refresh_date = $3 WHERE username = $4", token.AccessToken, token.RefreshToken, token.Expiry, sessionManager.Get(r.Context(), "User.Username"))
	if derr != nil {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": 1, "msg": "Database call error: " + derr.Error()})
		return
	}
	if tag.RowsAffected() != 1 {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": 1, "msg": "Database insert error, rows affected " + string(tag)})
		return
	}
	log.Println("Got token")
	basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msggreen": 1, "msg": "Discord linked."})
}
