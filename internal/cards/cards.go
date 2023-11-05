package cards

import (
	"github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/paymentintent"
	"github.com/stripe/stripe-go/v72/paymentmethod"
)

type Card struct {
	Secret   string
	Key      string
	Currency string
}

type Transaction struct {
	TransactionStatusID int
	Amount              int
	Currency            string
	LastFour            string
	BankReturnCode      string
}

func (c *Card) Charge(currency string, amount int) (*stripe.PaymentIntent, string, error) {
	return c.CreatePaymentIntent(currency, amount)
}

func (c *Card) CreatePaymentIntent(currency string, amount int) (*stripe.PaymentIntent, string, error) {
	stripe.Key = c.Secret

	// create a payment intent
	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(int64(amount)),
		Currency: stripe.String(currency),
	}

	//params.AddMetadata("key", "value")

	pi, err := paymentintent.New(params)
	if err != nil {
		msg := ""
		if stripeErr, ok := err.(*stripe.Error); ok {
			msg = cardErrorMessage(stripeErr.Code)
		}
		return nil, msg, err
	}
	return pi, "", nil
}

func (c *Card) GetPaymentMethod(s string) (*stripe.PaymentMethod, error) {
	stripe.Key = c.Secret

	pm, err := paymentmethod.Get(s, nil)

	return pm, err
}

func (c *Card) RetrievePaymentIntent(id string) (*stripe.PaymentIntent, error) {
	stripe.Key = c.Secret

	pi, err := paymentintent.Get(id, nil)

	return pi, err
}

func cardErrorMessage(code stripe.ErrorCode) string {
	switch code {
	case stripe.ErrorCodeCardDeclined:
		return "Your card was declined"
	case stripe.ErrorCodeExpiredCard:
		return "Your card is expired"
	case stripe.ErrorCodeIncorrectCVC:
		return "Incorrect CVC code"
	case stripe.ErrorCodeIncorrectZip:
		return "Incorrect zip/postal code"
	case stripe.ErrorCodeAmountTooLarge:
		return "the amount is too large to charge to your card"
	case stripe.ErrorCodeAmountTooSmall:
		return "the amount is small large to charge to your card"
	case stripe.ErrorCodeBalanceInsufficient:
		return "Insufficient balance"
	case stripe.ErrorCodePostalCodeInvalid:
		return "Your postal code is invalid"
	default:
		return "Your card was declined"
	}
}
