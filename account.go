package main

import (
	"context"
	"log"
	"net/http"

	"github.com/jackc/pgx/v4"
)

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		err := r.ParseForm()
		if err != nil {
			basicLayoutLookupRespond("login", w, r, map[string]interface{}{"LoginError": true, "LoginDetailedError": "Database query error: " + err.Error()})
			return
		}
		log.Printf("Login attempt: [%s]", r.PostFormValue("username"))
		if !validateUsername(r.PostFormValue("username")) || !validatePassword(r.PostFormValue("password")) {
			basicLayoutLookupRespond("login", w, r, map[string]interface{}{"LoginError": true})
			return
		}
		var passdb string
		var iddb int
		derr := dbpool.QueryRow(context.Background(), "SELECT password, id FROM users WHERE username = $1", r.PostFormValue("username")).Scan(&passdb, &iddb)
		if derr != nil {
			if derr == pgx.ErrNoRows {
				basicLayoutLookupRespond("login", w, r, map[string]interface{}{"LoginError": true})
				log.Printf("No such user")
			} else {
				basicLayoutLookupRespond("login", w, r, map[string]interface{}{"LoginError": true, "LoginDetailedError": "Database query error: " + derr.Error()})
				log.Printf("DB err: " + derr.Error())
			}
			return
		}
		if comparePasswords(passdb, r.PostFormValue("password")) {
			sessionManager.Put(r.Context(), "User.Username", r.PostFormValue("username"))
			sessionManager.Put(r.Context(), "User.Id", iddb)
			sessionManager.Put(r.Context(), "UserAuthorized", "True")
			w.Header().Set("Refresh", "1; /account")
			basicLayoutLookupRespond("login", w, r, map[string]interface{}{"LoginComplete": true, "User": map[string]interface{}{"Username": r.PostFormValue("username")}})
			log.Printf("Log in success: [%s]", r.PostFormValue("username"))
		} else {
			basicLayoutLookupRespond("login", w, r, map[string]interface{}{"LoginError": true})
		}
	} else {
		if r.Header.Get("CF-Visitor") != "{\"scheme\":\"https\"}" {
			basicLayoutLookupRespond("login", w, r, map[string]interface{}{"WarningUnsafe": true})
		} else {
			basicLayoutLookupRespond("login", w, r, map[string]interface{}{})
		}
	}
}
func accountHandler(w http.ResponseWriter, r *http.Request) {
	basicLayoutLookupRespond("account", w, r, map[string]interface{}{})
}
func logoutHandler(w http.ResponseWriter, r *http.Request) {
	sessionManager.Destroy(r.Context())
	w.Header().Set("Refresh", "2; /login")
	basicLayoutLookupRespond("logout", w, r, map[string]interface{}{})
}
func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		err := r.ParseForm()
		if err != nil {
			log.Printf("Error reading form: %v\n", err)
			return
		}
		type LastAttemptS struct {
			Fname string
			Lname string
			Username string
			Password string
			Email string
		}
		la := LastAttemptS{r.PostFormValue("fname"), r.PostFormValue("lname"), r.PostFormValue("username"), r.PostFormValue("password"), r.PostFormValue("email")}
		if !validateUsername(r.PostFormValue("username")) {
			basicLayoutLookupRespond("register", w, r, map[string]interface{}{"RegisterErrorMsg": "Username length must be between 3 and 25", "LastAttempt": la})
			return
		}
		if !validatePassword(r.PostFormValue("password")) {
			basicLayoutLookupRespond("register", w, r, map[string]interface{}{"RegisterErrorMsg": "Password length must be between 6 and 25", "LastAttempt": la})
			return
		}
		if !validateName(r.PostFormValue("fname")) || !validateName(r.PostFormValue("lname")) {
			basicLayoutLookupRespond("register", w, r, map[string]interface{}{"RegisterErrorMsg": "Name length must be between 3 and 25 and can only contain a-z, A-Z characters and space", "LastAttempt": la})
			return
		}
		if r.PostFormValue("password") != r.PostFormValue("confirm-password") {
			basicLayoutLookupRespond("register", w, r, map[string]interface{}{"RegisterErrorMsg": "Passwords are not matching", "LastAttempt": la})
			return
		}
		if !validateEmail(r.PostFormValue("email")) {
			basicLayoutLookupRespond("register", w, r, map[string]interface{}{"RegisterErrorMsg": "Email is not valid", "LastAttempt": la})
			return
		}
		requname := r.PostFormValue("username")
		requpass := hashPassword(r.PostFormValue("password"))
		reqemail := r.PostFormValue("email")
		reqfname := r.PostFormValue("fname")
		reqlname := r.PostFormValue("lname")
		reqemailcode := generateRandomString(50)

		log.Printf("Register attempt: [%s] [%s] [%s]", requname, reqemail)

		dberr := func(e error) {
			basicLayoutLookupRespond("register", w, r, map[string]interface{}{"RegisterErrorMsg": "Database call error: " + e.Error(), "LastAttempt": la})
		}

		numUsername := 0
		numUsernameErr := dbpool.QueryRow(context.Background(), "SELECT COUNT(*) FROM users WHERE username = $1", requname).Scan(&numUsername)
		if numUsernameErr != nil {
			dberr(numUsernameErr)
			return
		}
		if numUsername != 0 {
			basicLayoutLookupRespond("register", w, r, map[string]interface{}{"RegisterErrorMsg": "Username is already taken!", "LastAttempt": la})
			return
		}

		numEmail := 0
		numEmailErr := dbpool.QueryRow(context.Background(), "SELECT COUNT(*) FROM users WHERE email = $1", reqemail).Scan(&numEmail)
		if numEmailErr != nil {
			dberr(numEmailErr)
			return
		}
		if numEmail != 0 {
			basicLayoutLookupRespond("register", w, r, map[string]interface{}{"RegisterErrorMsg": "Email is already taken!", "LastAttempt": la})
			return
		}

		if sendgridConfirmcode(reqemail, reqemailcode) == false {
			basicLayoutLookupRespond("register", w, r, map[string]interface{}{"RegisterErrorMsg": "Can't verify email. Contact administrator.", "LastAttempt": la})
			return
		}

		tag, derr := dbpool.Exec(context.Background(), "INSERT INTO users (username, password, fname, lname, email, emailconfirmcode) VALUES($1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING", requname, requpass, reqfname, reqlname, reqemail, reqemailcode)
		if derr != nil {
			basicLayoutLookupRespond("register", w, r, map[string]interface{}{"RegisterErrorMsg": "Database call error: " + derr.Error(), "LastAttempt": la})
			return
		}
		if tag.RowsAffected() != 1 {
			basicLayoutLookupRespond("register", w, r, map[string]interface{}{"RegisterErrorMsg": "Database insert error, rows affected " + string(tag), "LastAttempt": la})
			return
		}
		basicLayoutLookupRespond("register", w, r, map[string]interface{}{"SuccessRegister": true})
		log.Printf("Register attempt success: [%s] [%s] [%s]", requname, reqemail)
	} else {
		if r.Header.Get("CF-Visitor") != "{\"scheme\":\"https\"}" {
			basicLayoutLookupRespond("register", w, r, map[string]interface{}{"WarningUnsafe": true})
		} else {
			basicLayoutLookupRespond("register", w, r, map[string]interface{}{})
		}
	}
}
func emailconfHandler(w http.ResponseWriter, r *http.Request) {
	keys, ok := r.URL.Query()["code"]
	if !ok || len(keys[0]) < 1 {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msg": "Confirm code does not exist", "msgred": true})
		return
	}
	key := keys[0]
	tag, derr := dbpool.Exec(context.Background(), "UPDATE users SET email_confirmed = now(), emailconfirmcode = '' WHERE emailconfirmcode = $1", key)
	if derr != nil {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msg": "Database call error: " + derr.Error(), "msgred": true})
		return
	}
	if tag.RowsAffected() != 1 {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msg": "Confirm code does not exist", "msgred": true})
		return
	}
	basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msg": "Email confirmed.", "msggreen": true})
}
