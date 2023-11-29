package main

import "net/http"

func (app *application) failedValidation(w http.ResponseWriter, r *http.Request, errors map[string]string) {
	var payload struct {
		Error   bool              `json:"error"`
		Message string            `json:"message"`
		Errors  map[string]string `json:"errors"`
	}

	payload.Error = true
	payload.Message = "failed validation"
	payload.Errors = errors
	app.sendResponse(w, http.StatusUnprocessableEntity, payload)
}
