package main

import (
	"net/http"

	"github.com/centrifugal/centrifuge"
	"github.com/google/uuid"
)

func auth(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		context := r.Context()
		credentials := &centrifuge.Credentials{UserID: uuid.NewString()}
		newContext := centrifuge.SetCredentials(context, credentials)
		r = r.WithContext(newContext)
		h.ServeHTTP(w, r)
	})
}
