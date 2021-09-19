package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"

	"github.com/MakeNowJust/heredoc"
	"golang.org/x/crypto/bcrypt"
)

func respondWithUnauthorized(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusUnauthorized)
	basicLayoutLookupRespond(templateNotAuthorized, w, r, map[string]interface{}{})
}

func respondWithForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	basicLayoutLookupRespond(templateErrorForbidden, w, r, map[string]interface{}{})
}

func respondWithNotImplemented(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	basicLayoutLookupRespond(templatePlainMessage, w, r, map[string]interface{}{"msg": "Not implemented"})
}

func checkFormParse(w http.ResponseWriter, r *http.Request) bool {
	err := r.ParseForm()
	if err != nil {
		basicLayoutLookupRespond(templatePlainMessage, w, r, map[string]interface{}{"msgred": true, "msg": "Form parse error: " + err.Error()})
	}
	return err == nil
}

func checkRespondDatabaseErrorAny(w http.ResponseWriter, r *http.Request, derr error) bool {
	if derr != nil {
		basicLayoutLookupRespond(templatePlainMessage, w, r, map[string]interface{}{"msgred": true, "msg": "Database query error: " + derr.Error()})
	}
	return derr == nil
}

func checkRespondGenericErrorAny(w http.ResponseWriter, r *http.Request, derr error) bool {
	if derr != nil {
		basicLayoutLookupRespond(templatePlainMessage, w, r, map[string]interface{}{"msgred": true, "msg": "Error: " + derr.Error()})
	}
	return derr == nil
}

func myNotFoundHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.Header.Get("CF-Connecting-IP")
		geo := r.Header.Get("CF-IPCountry")
		ua := r.Header.Get("user-agent")
		log.Println("["+geo+" "+ip+"] 404", r.Method, r.URL.Path, "["+ua+"]")
		w.WriteHeader(http.StatusNotFound)
		basicLayoutLookupRespond("error404", w, r, map[string]interface{}{})
	})
}
func hashPassword(pwd string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	if err != nil {
		log.Println(err)
		return ""
	}
	return string(hash)
}
func comparePasswords(hashedPwd string, plainPwd string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPwd), []byte(plainPwd))
	if err != nil {
		log.Println(err)
		return false
	}
	return true
}
func generateRandomString(slen int) string {
	s := ""
	a := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	al := len(a)
	for i := 0; i < slen; i++ {
		s += string(a[rand.Intn(al-1)])
	}
	return s
}

var regexName = regexp.MustCompile(`^[a-zA-Z ]*$`)

func validateName(u string) bool {
	if len(u) < 3 || len(u) > 25 || !regexName.MatchString(u) {
		return false
	}
	return true
}
func validateUsername(u string) bool {
	if len(u) < 3 || len(u) > 25 {
		return false
	}
	return true
}
func validatePassword(u string) bool {
	if len(u) < 6 || len(u) > 25 {
		return false
	}
	return true
}

var regexEmail = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

func validateEmail(u string) bool {
	if len(u) < 3 || len(u) > 254 {
		return false
	}
	return regexEmail.MatchString(u)
}

func sendgridConfirmcode(email string, code string) bool {
	sendstr := heredoc.Docf(`
{
	"personalizations": [
		{
			"to": [
				{
					"email":"%s"
				}
			]
		}
	],
	"from": {
		"email": "no-reply@wz2100-autohost.net",
		"name": "Account Registration"
	},
	"subject": "Welcome to Warzone 2100 Autohoster website",
	"content": [
		{
			"type": "text/plain",
		 	"value": "Welcome to Warzone 2100 subdivision. To confirm your email address follow this link: https://wz2100-autohost.net/activate?code=%s"
		},
		{
			"type":"text/html",
			"value":"<html><h3>Welcome to Warzone 2100 subdivision.</h3><p>To confirm your email address follow link below.</p><p><a href=\"https://wz2100-autohost.net/activate?code=%s\">Activate account</a></p></html>"
		}
	]
}`, email, code, code)
	req, _ := http.NewRequest("POST", "https://api.sendgrid.com/v3/mail/send", bytes.NewBuffer([]byte(sendstr)))
	req.Header.Set("Authorization", "Bearer "+os.Getenv("SENDGRID_KEY"))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return false
	}
	defer resp.Body.Close()
	log.Println("response Status:", resp.Status)
	log.Println("response Headers:", resp.Header)
	body, _ := ioutil.ReadAll(resp.Body)
	log.Println("response Body:", string(body))
	return resp.Status == "200 Success" || resp.Status == "202 Accepted"
}

func sendgridRecoverRequest(email string, code string) bool {
	sendstr := heredoc.Docf(`
{
	"personalizations": [
		{
			"to": [
				{
					"email":"%s"
				}
			]
		}
	],
	"from": {
		"email": "no-reply@wz2100-autohost.net",
		"name": "Autohoster"
	},
	"subject": "Password recovery",
	"content": [
		{
			"type": "text/plain",
		 	"value": "Hello, to reset your password please follow this link: https://wz2100-autohost.net/recover?code=%s\nIf this was not you and you think someone is trying to gain access to your account please contact us."
		},
		{
			"type":"text/html",
			"value":"<html><h3>Password reset</h3><p>To reset your password follow link below.</p><p><a href=\"https://wz2100-autohost.net/recover?code=%s\">Set new password</a></p><p>If this was not you and you think someone is trying to gain access to your account please contact us.</p></html>"
		}
	]
}`, email, code, code)
	req, _ := http.NewRequest("POST", "https://api.sendgrid.com/v3/mail/send", bytes.NewBuffer([]byte(sendstr)))
	req.Header.Set("Authorization", "Bearer "+os.Getenv("SENDGRID_KEY"))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return false
	}
	defer resp.Body.Close()
	log.Println("response Status:", resp.Status)
	log.Println("response Headers:", resp.Header)
	body, _ := ioutil.ReadAll(resp.Body)
	log.Println("response Body:", string(body))
	return resp.Status == "200 Success" || resp.Status == "202 Accepted"
}
