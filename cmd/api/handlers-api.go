package main

import (
	"encoding/json"
	"fmt"
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

type response struct {
	Error   bool        `json:"error"`
	Message string      `json:"message"`
	Content interface{} `json:"content"`
}

func (app *application) GetPaymentIntent(w http.ResponseWriter, r *http.Request) {
	var payload stripePayload

	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		app.sendBadRequest(w, err.Error())
		return
	}

	amount, err := strconv.Atoi(payload.Amount)
	if err != nil {
		app.sendBadRequest(w, err.Error())
		return
	}

	card := cards.Card{
		Secret:   app.config.stripe.secret,
		Key:      app.config.stripe.key,
		Currency: payload.Currency,
	}

	pi, msg, err := card.Charge(payload.Currency, amount)
	if err != nil {
		app.sendBadRequest(w, msg)
		return
	}

	var response response
	response.Message = msg
	response.Content = pi

	app.sendOK(w, response)
}

func (app *application) GetWidgetById(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	widgetID, _ := strconv.Atoi(id)

	widget, err := app.DB.GetWidget(widgetID)
	if err != nil {
		app.errorLog.Println(err)
		return
	}

	var response response
	response.Content = widget

	app.sendOK(w, response)
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
		app.sendBadRequest(w, msg)
		return
	}

	subscription, err := card.SubscribeToPlan(stripeCustomer, data.Plan, data.LastFour, "")
	if err != nil {
		app.errorLog.Println(err)
		txnMsg = "Error subscribing customer"
		app.sendBadRequest(w, txnMsg)
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

	var response response
	response.Message = txnMsg

	app.sendOK(w, response)
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

func (app *application) CreateAuthToken(w http.ResponseWriter, r *http.Request) {
	var userInput struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := app.readJson(w, r, &userInput)
	if err != nil {
		app.sendBadRequest(w, err.Error())
		return
	}

	app.infoLog.Println(userInput.Email)

	user, err := app.DB.GetUserByEmail(userInput.Email)
	if err != nil {
		app.errorLog.Println(err)
		app.sendUnauthorized(w)
		return
	}
	app.infoLog.Println(user)

	validPassword, err := app.passwordMatches(user.Password, userInput.Password)
	if err != nil {
		app.errorLog.Println(err)
		app.sendUnauthorized(w)
		return
	}

	if !validPassword {
		app.sendUnauthorized(w)
		return
	}

	token, err := models.GenerateToken(user.ID, 24*time.Hour, models.ScopeAuthentication)
	if err != nil {
		app.sendBadRequest(w, err.Error())
		return
	}

	err = app.DB.InsertToken(token, user)

	var payload struct {
		Token *models.Token `json:"authentication_token"`
	}
	payload.Token = token

	var response response
	response.Message = fmt.Sprintf("token for %s created", userInput.Email)
	response.Content = payload

	app.sendOK(w, response)
}

func (app *application) CheckAuthenticated(w http.ResponseWriter, r *http.Request) {
	app.sendUnauthorized(w)
}

func (app *application) sendOK(w http.ResponseWriter, payload response) error {
	payload.Error = false

	return app.sendResponse(w, http.StatusOK, payload)
}

func (app *application) sendBadRequest(w http.ResponseWriter, errorMessage string) error {
	var payload response

	payload.Error = true
	payload.Message = errorMessage

	return app.sendResponse(w, http.StatusBadRequest, payload)
}

func (app *application) sendUnauthorized(w http.ResponseWriter) error {
	var payload response

	payload.Error = true
	payload.Message = "invalid authentication credentials"

	return app.sendResponse(w, http.StatusUnauthorized, payload)
}

func (app *application) sendResponse(w http.ResponseWriter, statusID int, payload any, headers ...http.Header) error {
	out, err := app.writeJson(payload)
	if err != nil {
		app.errorLog.Println(err)
		return err
	}

	if len(headers) > 0 {
		for k, v := range headers[0] {
			w.Header()[k] = v
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusID)
	w.Write(out)
	return nil
}
