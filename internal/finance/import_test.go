package finance

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/kenma/okane/internal/money"
)

func TestParseCSVSpanish(t *testing.T) {
	data := []byte("Fecha;Descripción;Valor\n" +
		"01/09/2026;Supermercado Éxito;-350500,50\n" +
		"02/09/2026;Pago de nómina;2500000,00\n")

	format, rows, skipped, err := parseStatement("movimientos.csv", data, time.UTC)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if format != ImportFormatCSV {
		t.Fatalf("format = %q, want csv", format)
	}
	if skipped != 0 || len(rows) != 2 {
		t.Fatalf("skipped=%d rows=%d, want 0/2", skipped, len(rows))
	}
	first := rows[0]
	if first.Description != "Supermercado Éxito" {
		t.Fatalf("description = %q", first.Description)
	}
	if got := first.Amount.String(); got != "-350500.5" {
		t.Fatalf("amount = %q, want -350500.5", got)
	}
	if first.Direction != "out" {
		t.Fatalf("direction = %q, want out", first.Direction)
	}
	if got := first.OccurredAt.Format("2006-01-02"); got != "2026-09-01" {
		t.Fatalf("date = %q", got)
	}
	if got := rows[1].Amount.String(); got != "2500000" || rows[1].Direction != "in" {
		t.Fatalf("second row = %s %s", rows[1].Amount, rows[1].Direction)
	}
}

func TestParseCSVDebitCreditColumns(t *testing.T) {
	data := []byte("Date,Description,Debit,Credit\n" +
		"2026-09-01,Coffee,15000,\n" +
		"2026-09-02,Salary,,2500000\n")

	_, rows, _, err := parseStatement("statement.csv", data, time.UTC)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if got := rows[0].Amount.String(); got != "-15000" {
		t.Fatalf("debit amount = %q, want -15000", got)
	}
	if got := rows[1].Amount.String(); got != "2500000" {
		t.Fatalf("credit amount = %q, want 2500000", got)
	}
}

func TestParseCSVUnsignedWithTypeColumn(t *testing.T) {
	data := []byte("Fecha,Descripcion,Valor,Tipo\n" +
		"2026-09-03,Restaurante,85000,Compra\n" +
		"2026-09-04,Transferencia recibida,100000,Abono\n")

	_, rows, _, err := parseStatement("cuenta.csv", data, time.UTC)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := rows[0].Amount.String(); got != "-85000" {
		t.Fatalf("purchase amount = %q, want -85000", got)
	}
	if got := rows[1].Amount.String(); got != "100000" {
		t.Fatalf("credit amount = %q, want 100000", got)
	}
}

func TestParseCSVMissingColumns(t *testing.T) {
	data := []byte("Alpha,Beta\none,two\n")
	if _, _, _, err := parseStatement("bad.csv", data, time.UTC); err == nil {
		t.Fatal("expected error for a file without date/amount columns")
	}
}

func TestParseOFX(t *testing.T) {
	data := []byte(`OFXHEADER:100
DATA:OFXSGML
VERSION:102
SECURITY:NONE
ENCODING:USASCII

<OFX>
<SIGNONMSGSRSV1><SONRS><STATUS><CODE>0</CODE></STATUS></SONRS></SIGNONMSGSRSV1>
<BANKMSGSRSV1><STMTTRNRS><STMTRS>
<BANKTRANLIST>
<STMTTRN><TRNTYPE>DEBIT</TRNTYPE><DTPOSTED>20260901120000[-5:EST]</DTPOSTED><TRNAMT>-350500.50</TRNAMT><FITID>20260901001</FITID><NAME>Supermercado Exito</NAME><MEMO>Compra POS</MEMO></STMTTRN>
<STMTTRN><TRNTYPE>CREDIT</TRNTYPE><DTPOSTED>20260902120000</DTPOSTED><TRNAMT>2500000.00</TRNAMT><FITID>20260902001</FITID><NAME>Pago nomina</NAME></STMTTRN>
</BANKTRANLIST>
</STMTRS></STMTTRNRS></BANKMSGSRSV1>
</OFX>
`)

	format, rows, _, err := parseStatement("extracto.ofx", data, time.UTC)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if format != ImportFormatOFX {
		t.Fatalf("format = %q, want ofx", format)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	first := rows[0]
	if got := first.Amount.String(); got != "-350500.5" {
		t.Fatalf("amount = %q", got)
	}
	if first.ExternalID == nil || *first.ExternalID != "20260901001" {
		t.Fatalf("external id = %v", first.ExternalID)
	}
	if first.Merchant == nil || *first.Merchant != "Compra POS" {
		t.Fatalf("merchant = %v", first.Merchant)
	}
	if got := first.OccurredAt.Format("2006-01-02"); got != "2026-09-01" {
		t.Fatalf("date = %q", got)
	}
}

func TestHashRowsRepeatedAndStable(t *testing.T) {
	account := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	row := ImportRow{
		OccurredAt:  time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		Description: "Coffee Shop",
		Amount:      money.New("-15000"),
		Direction:   "out",
	}

	rows := []ImportRow{row, row, row}
	hashes := hashRows(account, rows)
	if len(hashes) != 3 {
		t.Fatalf("hashes = %d", len(hashes))
	}
	if hashes[0] == hashes[1] || hashes[1] == hashes[2] {
		t.Fatalf("unexpected repetition handling: %v", hashes)
	}
	if !strings.HasSuffix(hashes[2], ":3") {
		t.Fatalf("expected :3 suffix, got %q", hashes[2])
	}
}
