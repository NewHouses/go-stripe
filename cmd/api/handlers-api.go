package main

import (
	"encoding/json"
	"myapp/internal/cards"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type stripePayload struct {
	Currency      string `json:"currency"`
	Amount        string `json:"amount"`
	PaymentMethod string `json:"payment_method"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	LastFour      string `json:"last_four"`
	Plan          string `json:"plan"`
}

type jsonResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Content string `json:"content,omitempty"`
	ID      int    `json:"id,omitempty"`
}

func (app *application) GetPaymentIntent(w http.ResponseWriter, r *http.Request) {
	var payload stripePayload

	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		app.errorLog.Println(err)
		app.sendErrorResponse(w, err.Error())
		return
	}

	amount, err := strconv.Atoi(payload.Amount)
	if err != nil {
		app.errorLog.Println(err)
		app.sendErrorResponse(w, err.Error())
		return
	}

	card := cards.Card{
		Secret:   app.config.stripe.secret,
		Key:      app.config.stripe.key,
		Currency: payload.Currency,
	}

	pi, msg, err := card.Charge(payload.Currency, amount)
	if err != nil {
		app.sendErrorResponse(w, msg)
		return
	}

	app.sendResponse(w, pi)
}

func (app *application) GetWidgetById(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	widgetID, _ := strconv.Atoi(id)

	widget, err := app.DB.GetWidget(widgetID)
	if err != nil {
		app.errorLog.Println(err)
		return
	}

	app.sendResponse(w, widget)
}

func (app *application) CreateCustomerAndSubscribeToPlan(w http.ResponseWriter, r *http.Request) {
	var data stripePayload
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		app.errorLog.Println(err)
		return
	}

	app.infoLog.Println(data.Name, data.Email, data.LastFour, data.PaymentMethod, data.Plan)

	card := cards.Card{
		Secret:   app.config.stripe.secret,
		Key:      app.config.stripe.key,
		Currency: data.Currency,
	}

	stripeCustomer, msg, err := card.CreateCustomer(data.PaymentMethod, data.Name, data.Email)
	if err != nil {
		app.errorLog.Println(err)
		return
	}

	subscriptionId, err := card.SubscribeToPlan(stripeCustomer, data.Plan, data.LastFour, "")
	if err != nil {
		app.errorLog.Println(err)
		return
	}

	app.infoLog.Println("subscription id is ", subscriptionId)

	resp := jsonResponse{
		OK:      true,
		Message: msg,
	}
	app.sendResponse(w, resp)
}

func (app *application) sendResponse(w http.ResponseWriter, j any) {
	out, err := json.MarshalIndent(j, "", "	  ")
	if err != nil {
		app.errorLog.Println(err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
}

func (app *application) sendErrorResponse(w http.ResponseWriter, errorMessage string) {
	j := jsonResponse{
		OK:      false,
		Message: errorMessage,
	}

	app.sendResponse(w, j)
}
