package server

import "net/http"

// Health is a common handler for REST service health checking.
// TODO: DO THIS BETTER - CHECK DB CONNECTION, ETC.
func Health(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}