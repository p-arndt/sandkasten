package api

import (
	"encoding/json"
	"net/http"
)

const maxJSONBodyBytes int64 = 2 * 1024 * 1024

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	dec := json.NewDecoder(r.Body)
	return dec.Decode(dst)
}
