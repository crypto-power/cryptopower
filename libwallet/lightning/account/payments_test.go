package account

import (
	"testing"

	"github.com/breez/breez/data"
)

func TestOnlyDescription(t *testing.T) {
	invoiceMemo := &data.InvoiceMemo{
		Description: "test",
	}
	description := formatTextMemo(invoiceMemo)
	if description != "test" {
		t.Fatalf("description should be test, instead got %v", description)
	}

	invoiceMemo = &data.InvoiceMemo{}
	parseTextMemo(description, invoiceMemo)
	if invoiceMemo.Description != "test" {
		t.Fatalf("invoiceMemo.Description should be test, instead got %v", invoiceMemo.Description)
	}
}

func TestWithPayeeData(t *testing.T) {
	invoiceMemo := &data.InvoiceMemo{
		Description:   "test",
		PayeeName:     "payee",
		PayeeImageURL: "payee_image",
	}
	description := formatTextMemo(invoiceMemo)
	invoiceMemo = &data.InvoiceMemo{}
	parseTextMemo(description, invoiceMemo)
	if invoiceMemo.Description != "test" {
		t.Fatalf("invoiceMemo.Description should be test, instead got %v", invoiceMemo.Description)
	}
	if invoiceMemo.PayeeName != "payee" {
		t.Fatalf("invoiceMemo.PayeeName should be payee, instead got %v", invoiceMemo.PayeeName)
	}
	if invoiceMemo.PayeeImageURL != "payee_image" {
		t.Fatalf("invoiceMemo.PayeeImageURL should be payee_image, instead got %v", invoiceMemo.PayeeImageURL)
	}
}

func TestWithFullCustomData(t *testing.T) {
	invoiceMemo := &data.InvoiceMemo{
		Description:   "test",
		PayeeName:     "payee",
		PayeeImageURL: "payee_image",
		PayerName:     "payer",
		PayerImageURL: "payer_image",
	}
	description := formatTextMemo(invoiceMemo)
	invoiceMemo = &data.InvoiceMemo{}
	parseTextMemo(description, invoiceMemo)
	if invoiceMemo.Description != "test" {
		t.Fatalf("invoiceMemo.Description should be test, instead got %v", invoiceMemo.Description)
	}
	if invoiceMemo.PayeeName != "payee" {
		t.Fatalf("invoiceMemo.PayeeName should be payee, instead got %v", invoiceMemo.PayeeName)
	}
	if invoiceMemo.PayeeImageURL != "payee_image" {
		t.Fatalf("invoiceMemo.PayeeImageURL should be payee_image, instead got %v", invoiceMemo.PayeeImageURL)
	}
	if invoiceMemo.PayerName != "payer" {
		t.Fatalf("invoiceMemo.PayerName should be payer, instead got %v", invoiceMemo.PayerName)
	}
	if invoiceMemo.PayerImageURL != "payer_image" {
		t.Fatalf("invoiceMemo.PayerImageURL should be payer_image, instead got %v", invoiceMemo.PayerImageURL)
	}
}
