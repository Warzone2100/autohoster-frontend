package main

import (
	"net/http"
)

const (
	keyUserUsername       = "User.Username"
	keyUserID             = "User.Id"
	keyUserAuthorized     = "UserAuthorized"
	valUserAuthorizedTrue = "True"
)

func sessionGetUsername(r *http.Request) string {
	return sessionManager.GetString(r.Context(), keyUserUsername)
}

func sessionGetUserID(r *http.Request) int {
	return sessionManager.GetInt(r.Context(), keyUserID)
}

func checkUserAuthorized(r *http.Request) bool {
	return !(!sessionManager.Exists(r.Context(), keyUserUsername) || sessionManager.Get(r.Context(), keyUserAuthorized) != valUserAuthorizedTrue)
}
