package main

import (
	"encoding/json"
	"myapp/internal/cards"
	"myapp/internal/models"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

type stripePayload struct {
	Currency      string `json:"currency"`
	Amount        string `json:"amount"`
	PaymentMethod string `json:"payment_method"`
	Email         string `json:"email"`
	CardBrand     string `json:"card_brand"`
	ExpiryMonth   int    `json:"exp_month"`
	ExpiryYear    int    `json:"exp_year"`
	LastFour      string `json:"last_four"`
	Plan          string `json:"plan"`
	ProductID     string `json:"product_id"`
	FirstName     string `json:"first_name"`
	Lastname      string `json:"last_name"`
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

	app.infoLog.Println(data.FirstName, data.Email, data.LastFour, data.PaymentMethod, data.Plan)

	card := cards.Card{
		Secret:   app.config.stripe.secret,
		Key:      app.config.stripe.key,
		Currency: data.Currency,
	}

	txnMsg := "Transaction successful"

	stripeCustomer, msg, err := card.CreateCustomer(data.PaymentMethod, data.FirstName, data.Email)
	if err != nil {
		app.errorLog.Println(err)
		app.sendErrorResponse(w, msg)
		return
	}

	subscription, err := card.SubscribeToPlan(stripeCustomer, data.Plan, data.LastFour, "")
	if err != nil {
		app.errorLog.Println(err)
		txnMsg = "Error subscribing customer"
		app.sendErrorResponse(w, txnMsg)
		return
	}

	app.infoLog.Println("subscription id is ", subscription.ID)

	productID, _ := strconv.Atoi(data.ProductID)
	customerID, err := app.SaveCustomer(data.FirstName, data.Lastname, data.Email)
	if err != nil {
		app.errorLog.Println(err)
		return
	}

	amount, _ := strconv.Atoi(data.Amount)
	txn := models.Transaction{
		Amount:              amount,
		Currency:            "eur",
		LastFour:            data.LastFour,
		ExpiryMonth:         data.ExpiryMonth,
		ExpiryYear:          data.ExpiryYear,
		TransactionStatusID: 2,
	}

	txnID, err := app.SaveTransaction(txn)
	if err != nil {
		app.errorLog.Println(err)
		return
	}

	order := models.Order{
		WidgetID:      productID,
		TransactionID: txnID,
		CustomerID:    customerID,
		StatusID:      1,
		Quantity:      1,
		Amount:        amount,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	_, err = app.SaveOrder(order)

	resp := jsonResponse{
		OK:      true,
		Message: txnMsg,
	}

	app.sendResponse(w, resp)
}

func (app *application) SaveTransaction(txn models.Transaction) (int, error) {
	id, err := app.DB.InsertTransaction(txn)

	return id, err
}

func (app *application) SaveOrder(order models.Order) (int, error) {
	id, err := app.DB.InsertOrder(order)

	return id, err
}

func (app *application) SaveCustomer(firstName string, lastName string, email string) (int, error) {
	customer := models.Customer{
		FirstName: firstName,
		LastName:  lastName,
		Email:     email,
	}

	id, err := app.DB.InsertCustomer(customer)

	return id, err
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
