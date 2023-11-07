package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

func (app *application) readJson(w http.ResponseWriter, r *http.Request, data interface{}) error {
	maxBytes := 1048576

	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	dec := json.NewDecoder(r.Body)
	err := dec.Decode(data)

	if err != nil {
		return err
	}

	err = dec.Decode(&struct{}{})

	if err != io.EOF {
		return errors.New("Body must only have a single JSON value")
	}

	return nil
}

func (app *application) writeJson(data interface{}) ([]byte, error) {
	out, err := json.MarshalIndent(data, "", "\t")

	return out, err
}
