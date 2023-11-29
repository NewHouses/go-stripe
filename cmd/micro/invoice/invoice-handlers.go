package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/phpdave11/gofpdf"
	"github.com/phpdave11/gofpdf/contrib/gofpdi"
)

type Order struct {
	ID        int       `json:"id"`
	Quantity  int       `json:"quantity"`
	Amount    int       `json:"amount"`
	Product   string    `json:"product"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type response struct {
	Error   bool        `json:"error"`
	Message string      `json:"message"`
	Content interface{} `json:"content"`
}

func (app *application) CreateAndSendInvoice(w http.ResponseWriter, r *http.Request) {
	// receive json
	var order Order

	err := app.readJson(w, r, &order)
	if err != nil {
		app.sendBadRequest(w, err.Error())
		return
	}

	// order.ID = 100
	// order.Email = "me@here.com"
	// order.FirstName = "Serxio"
	// order.LastName = "Simons"
	// order.Quantity = 1
	// order.Amount = 1000
	// order.Product = "Widget"
	// order.CreatedAt = time.Now()

	// generate a pdf invoice
	err = app.createInvoicePDF(order)
	if err != nil {
		app.sendBadRequest(w, err.Error())
		return
	}

	// create mail attachment
	attachments := []string{
		fmt.Sprintf("./invoices/%d.pdf", order.ID),
	}

	// send mail with attachment
	err = app.SendMail("info@widgets.com", order.Email, "Your invoice", "invoice", attachments, nil)
	if err != nil {
		app.sendBadRequest(w, err.Error())
		return
	}

	// send response

	var response response
	response.Message = fmt.Sprintf("invoice %d.pdf created and sent to %s", order.ID, order.Email)

	app.sendOK(w, response)
}

func (app *application) createInvoicePDF(order Order) error {
	pdf := gofpdf.New("P", "mm", "Letter", "")
	pdf.SetMargins(10, 13, 10)
	pdf.SetAutoPageBreak(true, 0)

	importer := gofpdi.NewImporter()

	t := importer.ImportPage(pdf, "./pdf-templates/invoice.pdf", 1, "/MediaBox")

	pdf.AddPage()
	importer.UseImportedTemplate(pdf, t, 0, 0, 215.9, 0)

	// write info
	pdf.SetY(50)
	pdf.SetX(10)
	pdf.SetFont("Times", "", 11)
	pdf.CellFormat(97, 8, fmt.Sprintf("Attention: %s %s", order.FirstName, order.LastName), "", 0, "L", false, 0, "")
	pdf.Ln(5)
	pdf.CellFormat(97, 8, order.Email, "", 0, "L", false, 0, "")
	pdf.Ln(5)
	pdf.CellFormat(97, 8, order.CreatedAt.Format("2006-01-02"), "", 0, "L", false, 0, "")

	pdf.SetX(58)
	pdf.SetY(93)
	pdf.CellFormat(155, 8, order.Product, "", 0, "L", false, 0, "")
	pdf.SetX(166)
	pdf.CellFormat(20, 8, fmt.Sprintf("%d", order.Quantity), "", 0, "C", false, 0, "")

	pdf.SetX(185)

	pdf.CellFormat(20, 8, fmt.Sprintf("%.2f eur", float32(order.Amount/100.0)), "", 0, "R", false, 0, "")

	invoicePath := fmt.Sprintf("./invoices/%d.pdf", order.ID)
	err := pdf.OutputFileAndClose(invoicePath)

	return err
}

func (app *application) sendBadRequest(w http.ResponseWriter, errorMessage string) error {
	var payload response

	payload.Error = true
	payload.Message = errorMessage

	return app.sendResponse(w, http.StatusBadRequest, payload)
}

func (app *application) sendOK(w http.ResponseWriter, payload response) error {
	payload.Error = false

	return app.sendResponse(w, http.StatusOK, payload)
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
