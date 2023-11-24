package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"myapp/internal/cards"
	"myapp/internal/encryption"
	"myapp/internal/models"
	"myapp/internal/urlsigner"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
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
		PaymentIntent:       subscription.ID,
		PaymentMethod:       data.PaymentMethod,
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
	user, err := app.authenticateToken(r)
	if err != nil {
		app.sendUnauthorized(w)
		return
	}

	var response response

	response.Message = fmt.Sprintf("authenticated user %s", user.Email)
	app.sendOK(w, response)

}

func (app *application) authenticateToken(r *http.Request) (*models.User, error) {
	authorizationHeader := r.Header.Get("Authorization")
	if authorizationHeader == "" {
		return nil, errors.New("no authorization header received")
	}

	headerParts := strings.Split(authorizationHeader, " ")
	if len(headerParts) != 2 || headerParts[0] != "Bearer" {
		return nil, errors.New("no authorization header received")
	}

	token := headerParts[1]
	if len(token) != 26 {
		return nil, errors.New("authentication token wrong size")
	}

	user, err := app.DB.GetUserForToken(token)
	if err != nil {
		return nil, errors.New("no matching user found")
	}

	return user, nil
}

func (app *application) VirtualTerminalPaymentSucceeded(w http.ResponseWriter, r *http.Request) {
	var txnData struct {
		PaymentAmount   int    `json:"amount"`
		PaymentCurrency string `json:"currency"`
		Firstname       string `json:"first_name"`
		LastName        string `json:"last_name"`
		Email           string `json:"email"`
		PaymentIntent   string `json:"payment_intent"`
		PaymentMethod   string `json:"payment_method"`
		BankReturnCode  string `json:"bank_return_code"`
		ExpiryMonth     int    `json:"expiry_month"`
		ExpiryYear      int    `json:"expiry_year"`
		LastFour        string `json:"last_four"`
	}

	err := app.readJson(w, r, &txnData)
	if err != nil {
		app.sendBadRequest(w, err.Error())
		return
	}

	card := cards.Card{
		Secret: app.config.stripe.secret,
		Key:    app.config.stripe.key,
	}

	pi, err := card.RetrievePaymentIntent(txnData.PaymentIntent)
	if err != nil {
		app.sendBadRequest(w, err.Error())
		return
	}

	pm, err := card.GetPaymentMethod(txnData.PaymentMethod)
	if err != nil {
		app.sendBadRequest(w, err.Error())
		return
	}

	txnData.LastFour = pm.Card.Last4
	txnData.ExpiryMonth = int(pm.Card.ExpMonth)
	txnData.ExpiryYear = int(pm.Card.ExpYear)

	txn := models.Transaction{
		Amount:              txnData.PaymentAmount,
		Currency:            txnData.PaymentCurrency,
		LastFour:            txnData.LastFour,
		ExpiryMonth:         txnData.ExpiryMonth,
		ExpiryYear:          txnData.ExpiryYear,
		PaymentIntent:       txnData.PaymentIntent,
		PaymentMethod:       txnData.PaymentMethod,
		BankReturnCode:      pi.Charges.Data[0].ID,
		TransactionStatusID: 2,
	}

	_, err = app.SaveTransaction(txn)
	if err != nil {
		app.sendBadRequest(w, err.Error())
		return
	}

	response := response{
		Message: "Virtual terminal payment succeeded",
		Content: txn,
	}
	app.sendOK(w, response)
}

func (app *application) SendPasswordResetEmail(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Email string `json:"email"`
	}

	err := app.readJson(w, r, &payload)
	if err != nil {
		app.sendBadRequest(w, err.Error())
		return
	}

	// verify email exists
	_, err = app.DB.GetUserByEmail(payload.Email)
	if err != nil {
		response := response{
			Error:   true,
			Message: "no matching email found on our system",
		}
		app.sendResponse(w, http.StatusAccepted, response)
		return
	}

	link := fmt.Sprintf("%s/reset-password?email=%s", app.config.frontend, payload.Email)

	sign := urlsigner.Signer{
		Secret: []byte(app.config.secretKey),
	}

	signedLink := sign.GenerateTokenFromString(link)

	var data struct {
		Link string
	}

	data.Link = signedLink

	// send email
	err = app.SendMail("info@widgets.com", payload.Email, "Password Reset Request", "password-reset", data)
	if err != nil {
		app.errorLog.Println(err)
		app.sendBadRequest(w, err.Error())
		return
	}

	response := response{
		Message: "Email sent",
	}
	app.sendOK(w, response)
}

func (app *application) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		EncryptedEmail string `json:"encrypted_email"`
		Password       string `json:"password"`
	}

	err := app.readJson(w, r, &payload)
	if err != nil {
		app.sendBadRequest(w, err.Error())
		return
	}

	encryptor := encryption.Encryption{
		Key: []byte(app.config.secretKey),
	}

	email, err := encryptor.Decrypt(payload.EncryptedEmail)
	if err != nil {
		app.sendBadRequest(w, err.Error())
		return
	}

	user, err := app.DB.GetUserByEmail(email)
	if err != nil {
		app.sendBadRequest(w, err.Error())
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(payload.Password), 12)
	if err != nil {
		app.sendBadRequest(w, err.Error())
		return
	}

	err = app.DB.UpdatePasswordForUser(user, string(newHash))
	if err != nil {
		app.sendBadRequest(w, err.Error())
		return
	}

	response := response{
		Message: "password changed",
	}
	app.sendOK(w, response)
}

func (app *application) AllSales(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		PageSize int `json:"page_size"`
		Page     int `json:"page"`
	}

	err := app.readJson(w, r, &payload)
	if err != nil {
		app.sendBadRequest(w, err.Error())
		return
	}

	allSales, lastPage, totalRecords, err := app.DB.GetAllOrdersPaginated(payload.PageSize, payload.Page)
	if err != nil {
		app.sendBadRequest(w, err.Error())
		return
	}

	var content struct {
		CurrentPage  int             `json:"current_page"`
		PageSize     int             `json:"page_size"`
		LastPage     int             `json:"last_page"`
		Totalrecords int             `json:"total_records"`
		Orders       []*models.Order `json:"orders"`
	}

	content.CurrentPage = payload.Page
	content.PageSize = payload.PageSize
	content.LastPage = lastPage
	content.Totalrecords = totalRecords
	content.Orders = allSales

	response := response{
		Content: content,
	}

	app.sendOK(w, response)
}

func (app *application) AllSubscriptions(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		PageSize int `json:"page_size"`
		Page     int `json:"page"`
	}

	err := app.readJson(w, r, &payload)
	if err != nil {
		app.sendBadRequest(w, err.Error())
		return
	}

	allSubscriptions, lastPage, totalRecords, err := app.DB.GetAllSubscriptionsPaginated(payload.PageSize, payload.Page)
	if err != nil {
		app.sendBadRequest(w, err.Error())
		return
	}

	var content struct {
		CurrentPage  int             `json:"current_page"`
		PageSize     int             `json:"page_size"`
		LastPage     int             `json:"last_page"`
		Totalrecords int             `json:"total_records"`
		Orders       []*models.Order `json:"orders"`
	}

	content.CurrentPage = payload.Page
	content.PageSize = payload.PageSize
	content.LastPage = lastPage
	content.Totalrecords = totalRecords
	content.Orders = allSubscriptions

	response := response{
		Content: content,
	}

	app.sendOK(w, response)
}

func (app *application) GetSale(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	orderID, _ := strconv.Atoi(id)

	sale, err := app.DB.GetOrderById(orderID)
	if err != nil {
		app.sendBadRequest(w, err.Error())
		return
	}

	response := response{
		Content: sale,
	}

	app.sendOK(w, response)
}

func (app *application) RefundCharge(w http.ResponseWriter, r *http.Request) {
	var chargeToRefund struct {
		ID            int    `json:"id"`
		PaymentIntent string `json:"pi"`
		Amount        int    `json:"amount"`
		Currency      string `json:"currency"`
	}

	err := app.readJson(w, r, &chargeToRefund)
	if err != nil {
		app.sendBadRequest(w, err.Error())
		return
	}

	// validate

	card := cards.Card{
		Secret:   app.config.stripe.secret,
		Key:      app.config.stripe.key,
		Currency: chargeToRefund.Currency,
	}

	err = card.Refund(chargeToRefund.PaymentIntent, chargeToRefund.Amount)
	if err != nil {
		app.sendBadRequest(w, err.Error())
		return
	}

	// update status in db
	err = app.DB.UpdateOrderStatus(chargeToRefund.ID, 2)
	if err != nil {
		app.sendBadRequest(w, errors.New("the charge was refunded, but the database could not be updated").Error())
		return
	}

	response := response{
		Message: "Charge refunded",
	}
	app.sendOK(w, response)

}

func (app *application) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	var subToCancel struct {
		ID            int    `json:"id"`
		PaymentIntent string `json:"pi"`
		Currency      string `json:"currency"`
	}

	err := app.readJson(w, r, &subToCancel)
	if err != nil {
		app.sendBadRequest(w, err.Error())
		return
	}

	// validate

	card := cards.Card{
		Secret:   app.config.stripe.secret,
		Key:      app.config.stripe.key,
		Currency: subToCancel.Currency,
	}

	err = card.CancelSubscription(subToCancel.PaymentIntent)
	if err != nil {
		app.sendBadRequest(w, err.Error())
		return
	}

	// update status in db
	err = app.DB.UpdateOrderStatus(subToCancel.ID, 3)
	if err != nil {
		app.sendBadRequest(w, errors.New("the subscription was cancelled, but the database could not be updated").Error())
		return
	}

	response := response{
		Message: "Subscription cancelled",
	}
	app.sendOK(w, response)

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
