package finance

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kenma/okane/internal/domain"
	"github.com/kenma/okane/internal/money"
)

// ImportFormat identifies a supported statement format.
type ImportFormat string

const (
	ImportFormatCSV ImportFormat = "csv"
	ImportFormatOFX ImportFormat = "ofx"
)

// ImportRow is one parsed statement line.
type ImportRow struct {
	OccurredAt  time.Time
	Description string
	Merchant    *string
	Reference   *string
	ExternalID  *string
	Amount      money.Amount // signed: negative is an outflow
	Direction   domain.Direction
}

// ImportRowView is the JSON representation of a parsed row in a preview.
type ImportRowView struct {
	Date        string `json:"date"`
	Description string `json:"description"`
	Merchant    string `json:"merchant,omitempty"`
	Reference   string `json:"reference,omitempty"`
	ExternalID  string `json:"external_id,omitempty"`
	Amount      string `json:"amount"`
	Direction   string `json:"direction"`
	// Suggested category from the user's rules, shown in preview.
	CategoryID     *string `json:"category_id,omitempty"`
	CategoryName   *string `json:"category_name,omitempty"`
	CategorySource string  `json:"category_source,omitempty"`
	Duplicate      bool    `json:"duplicate"`
	Repeated       bool    `json:"repeated"`
}

// ImportPreview describes a parsed statement before it is stored.
type ImportPreview struct {
	Format    ImportFormat    `json:"format"`
	FileName  string          `json:"file_name"`
	Total     int             `json:"total"`
	Skipped   int             `json:"skipped"`
	Existing  int             `json:"existing"`
	Repeated  int             `json:"repeated"`
	DateFrom  string          `json:"date_from,omitempty"`
	DateTo    string          `json:"date_to,omitempty"`
	Rows      []ImportRowView `json:"rows"`
	Truncated bool            `json:"truncated"`
}

// ImportResult reports what a committed statement stored.
type ImportResult struct {
	Total     int `json:"total"`
	Imported  int `json:"imported"`
	Duplicate int `json:"duplicate"`
	Skipped   int `json:"skipped"`
}

const previewRowLimit = 200

// parseStatement detects the file format and parses it.
func parseStatement(fileName string, data []byte, loc *time.Location) (ImportFormat, []ImportRow, int, error) {
	data = decodeText(stripBOM(data))

	head := data
	if len(head) > 4096 {
		head = head[:4096]
	}
	upper := strings.ToUpper(string(head))
	lowerName := strings.ToLower(fileName)
	if strings.Contains(upper, "OFXHEADER") || strings.Contains(upper, "<OFX>") ||
		strings.Contains(upper, "<STMTTRN>") || strings.HasSuffix(lowerName, ".ofx") ||
		strings.HasSuffix(lowerName, ".qfx") {
		rows, skipped, err := parseOFX(data, loc)
		return ImportFormatOFX, rows, skipped, err
	}
	rows, skipped, err := parseCSV(data, loc)
	return ImportFormatCSV, rows, skipped, err
}

// stripBOM removes a UTF-8 byte order mark.
func stripBOM(data []byte) []byte {
	return bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
}

// decodeText keeps valid UTF-8 as is and falls back to Latin-1, which covers
// the windows-1252 exports used by most Colombian banks.
func decodeText(data []byte) []byte {
	if utf8.Valid(data) {
		return data
	}
	var buf strings.Builder
	buf.Grow(len(data))
	for _, b := range data {
		buf.WriteRune(rune(b))
	}
	return []byte(buf.String())
}

// --- CSV ---

type columnRole int

const (
	colIgnore columnRole = iota
	colDate
	colDescription
	colMerchant
	colAmount
	colDebit
	colCredit
	colType
	colReference
)

var headerKeywords = []struct {
	role     columnRole
	keywords []string
}{
	{colDate, []string{"fecha de la operacion", "fecha operacion", "fecha movimiento", "fecha de movimiento", "fecha transaccion", "fecha", "date"}},
	{colDescription, []string{"descripcion de la operacion", "descripcion del movimiento", "detalle de la transaccion", "descripcion", "detalle", "concepto", "description", "transaction"}},
	{colMerchant, []string{"comercio", "establecimiento", "merchant"}},
	{colReference, []string{"numero de documento", "numero documento", "documento referencia", "referencia", "documento", "comprobante", "reference"}},
	{colDebit, []string{"debito", "debitos", "cargo", "cargos", "debit", "debits"}},
	{colCredit, []string{"credito", "creditos", "abono", "abonos", "credit", "credits"}},
	{colAmount, []string{"valor transaccion", "valor de la operacion", "valor movimiento", "importe", "monto", "valor", "amount"}},
	{colType, []string{"tipo de movimiento", "tipo movimiento", "naturaleza", "movimiento", "tipo"}},
}

func normalizeHeader(raw string) string {
	s := foldAccents(strings.ToLower(strings.TrimSpace(raw)))
	s = strings.NewReplacer("_", " ", "-", " ", ".", " ", ":", " ").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

func mapColumns(header []string) []columnRole {
	roles := make([]columnRole, len(header))
	for i, cell := range header {
		norm := normalizeHeader(cell)
		if norm == "" {
			continue
		}
		best := colIgnore
		bestLen := 0
		for _, entry := range headerKeywords {
			for _, kw := range entry.keywords {
				if (norm == kw || strings.Contains(norm, kw)) && len(kw) > bestLen {
					best = entry.role
					bestLen = len(kw)
				}
			}
		}
		roles[i] = best
	}
	return roles
}

func parseCSV(data []byte, loc *time.Location) ([]ImportRow, int, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = sniffDelimiter(data)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, 0, fmt.Errorf("read csv: %w", err)
	}
	if len(records) == 0 {
		return nil, 0, nil
	}

	headerIndex := findHeaderRow(records)
	roles := mapColumns(records[headerIndex])
	if !containsRole(roles, colDate) ||
		(!containsRole(roles, colAmount) && !containsRole(roles, colDebit) && !containsRole(roles, colCredit)) {
		return nil, 0, Invalid("could not find date and amount columns in the file")
	}

	var (
		rows    []ImportRow
		skipped int
	)
	for _, record := range records[headerIndex+1:] {
		if isBlankRecord(record) {
			continue
		}
		row, ok := parseCSVRecord(record, roles, loc)
		if !ok {
			skipped++
			continue
		}
		rows = append(rows, row)
	}
	return rows, skipped, nil
}

func isBlankRecord(record []string) bool {
	for _, cell := range record {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

func containsRole(roles []columnRole, role columnRole) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

// findHeaderRow scans the first lines for the most likely header row.
func findHeaderRow(records [][]string) int {
	limit := len(records)
	if limit > 10 {
		limit = 10
	}
	bestIndex, bestScore := 0, 0
	for i := 0; i < limit; i++ {
		score := 0
		for _, role := range mapColumns(records[i]) {
			if role != colIgnore {
				score++
			}
		}
		if score > bestScore {
			bestIndex, bestScore = i, score
		}
	}
	return bestIndex
}

func parseCSVRecord(record []string, roles []columnRole, loc *time.Location) (ImportRow, bool) {
	var row ImportRow
	var haveDate, haveAmount bool
	var amountText, debitText, creditText, typeText string

	for i, role := range roles {
		if i >= len(record) {
			break
		}
		cell := strings.TrimSpace(record[i])
		if cell == "" {
			continue
		}
		switch role {
		case colDate:
			if !haveDate {
				if t, ok := parseDate(cell, loc); ok {
					row.OccurredAt = t
					haveDate = true
				}
			}
		case colDescription:
			if row.Description == "" {
				row.Description = cell
			}
		case colMerchant:
			if row.Merchant == nil {
				merchant := cell
				row.Merchant = &merchant
			}
		case colReference:
			if row.Reference == nil {
				reference := cell
				row.Reference = &reference
			}
		case colAmount:
			if amountText == "" {
				amountText = cell
			}
		case colDebit:
			if debitText == "" {
				debitText = cell
			}
		case colCredit:
			if creditText == "" {
				creditText = cell
			}
		case colType:
			if typeText == "" {
				typeText = cell
			}
		}
	}

	if !haveDate {
		return ImportRow{}, false
	}

	switch {
	case debitText != "" || creditText != "":
		if debitText != "" {
			parsed, err := parseAmount(debitText)
			if err != nil {
				return ImportRow{}, false
			}
			row.Amount = parsed.Abs().Neg()
		} else {
			parsed, err := parseAmount(creditText)
			if err != nil {
				return ImportRow{}, false
			}
			row.Amount = parsed.Abs()
		}
		haveAmount = true
	case amountText != "":
		parsed, err := parseAmount(amountText)
		if err != nil {
			return ImportRow{}, false
		}
		row.Amount = parsed
		haveAmount = true
	}
	if !haveAmount || row.Amount.IsZero() {
		return ImportRow{}, false
	}

	// A "tipo de movimiento" column can disambiguate unsigned amounts.
	if typeText != "" && amountText != "" && !row.Amount.IsNegative() && isDebitType(typeText) {
		row.Amount = row.Amount.Neg()
	}

	row.Direction = domain.DirectionOut
	if !row.Amount.IsNegative() {
		row.Direction = domain.DirectionIn
	}
	row.Description = truncateText(row.Description, 200)
	if row.Description == "" {
		if row.Merchant != nil {
			row.Description = truncateText(*row.Merchant, 200)
		} else {
			row.Description = "Imported transaction"
		}
	}
	return row, true
}

func isDebitType(text string) bool {
	norm := foldAccents(strings.ToLower(text))
	for _, word := range []string{"debito", "cargo", "compra", "pago", "retiro", "salida", "egreso"} {
		if strings.Contains(norm, word) {
			return true
		}
	}
	return false
}

func sniffDelimiter(data []byte) rune {
	sample := data
	if len(sample) > 8192 {
		sample = sample[:8192]
	}
	lines := strings.Split(string(sample), "\n")
	if len(lines) > 5 {
		lines = lines[:5]
	}
	best, bestCount := ',', 0
	for _, candidate := range []rune{';', ',', '\t', '|'} {
		count := 0
		for _, line := range lines {
			count += strings.Count(line, string(candidate))
		}
		if count > bestCount {
			best, bestCount = candidate, count
		}
	}
	return best
}

var numberCleaner = regexp.MustCompile(`[^0-9.,\-()]`)

// parseAmount understands "1.234,56", "1,234.56", "$ 350.50", "(350,50)" and
// "-350,50".
func parseAmount(raw string) (money.Amount, error) {
	clean := numberCleaner.ReplaceAllString(raw, "")
	negative := strings.Contains(clean, "-")
	if strings.HasPrefix(clean, "(") && strings.HasSuffix(clean, ")") {
		negative = true
	}
	clean = strings.NewReplacer("(", "", ")", "", "-", "").Replace(clean)
	if clean == "" {
		return money.Amount{}, fmt.Errorf("empty amount")
	}
	clean = normalizeDecimal(clean)
	dec, err := decimal.NewFromString(clean)
	if err != nil {
		return money.Amount{}, fmt.Errorf("parse amount %q: %w", raw, err)
	}
	if negative {
		dec = dec.Neg()
	}
	return money.FromDecimal(dec), nil
}

// normalizeDecimal converts a Spanish or English decimal string to one with a
// dot decimal separator and no thousands separators.
func normalizeDecimal(s string) string {
	lastComma := strings.LastIndex(s, ",")
	lastDot := strings.LastIndex(s, ".")
	switch {
	case lastComma == -1 && lastDot == -1:
		return s
	case lastComma > lastDot:
		// Comma is the decimal separator: 1.234,56
		s = strings.ReplaceAll(s, ".", "")
		return strings.Replace(s, ",", ".", 1)
	default:
		// Dot is the decimal separator: 1,234.56
		return strings.ReplaceAll(s, ",", "")
	}
}

var dateLayouts = []string{
	"2006-01-02", "2006/01/02", "20060102",
	"02/01/2006", "2/1/2006", "02-01-2006", "2-1-2006", "02.01.2006",
	"02/01/06", "2/1/06", "02-01-06",
	"02/01/2006 15:04:05", "2/1/2006 15:04", "02/01/2006 15:04",
	"2006-01-02 15:04:05", "2006-01-02 15:04",
}

// parseDate tries the supported layouts. Latin-American day-first formats win
// ties; when the first component is greater than 12 the day-first reading is
// the only valid one anyway.
func parseDate(raw string, loc *time.Location) (time.Time, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range dateLayouts {
		if t, err := time.ParseInLocation(layout, value, loc); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// --- OFX ---

const stmtTrnOpen = "<STMTTRN>"

// parseOFX extracts STMTTRN blocks from either OFX 1.x (SGML) or 2.x (XML).
func parseOFX(data []byte, loc *time.Location) ([]ImportRow, int, error) {
	content := string(data)
	upper := strings.ToUpper(content)

	var (
		rows    []ImportRow
		skipped int
	)
	searchFrom := 0
	for {
		start := strings.Index(upper[searchFrom:], stmtTrnOpen)
		if start < 0 {
			break
		}
		start += searchFrom + len(stmtTrnOpen)

		end := len(content) - start
		for _, marker := range []string{"</STMTTRN>", "<STMTTRN>", "</BANKTRANLIST>"} {
			if idx := strings.Index(upper[start:], marker); idx >= 0 && idx < end {
				end = idx
			}
		}
		block := content[start : start+end]

		amountRaw := ofxTag(block, "TRNAMT")
		dateRaw := ofxTag(block, "DTPOSTED")
		if dateRaw == "" {
			dateRaw = ofxTag(block, "DATE")
		}
		if amountRaw == "" || dateRaw == "" {
			skipped++
			searchFrom = start + end
			continue
		}
		amount, err := parseAmount(amountRaw)
		if err != nil || amount.IsZero() {
			skipped++
			searchFrom = start + end
			continue
		}
		occurred, ok := parseOFXDate(dateRaw, loc)
		if !ok {
			skipped++
			searchFrom = start + end
			continue
		}

		description := ofxTag(block, "NAME")
		if description == "" {
			description = ofxTag(block, "PAYEE")
		}
		memo := ofxTag(block, "MEMO")
		if description == "" {
			description = memo
		}
		if description == "" {
			description = "Imported transaction"
		}
		row := ImportRow{
			OccurredAt:  occurred,
			Description: truncateText(description, 200),
			Amount:      amount,
			Direction:   domain.DirectionIn,
		}
		if amount.IsNegative() {
			row.Direction = domain.DirectionOut
		}
		if fitid := ofxTag(block, "FITID"); fitid != "" {
			row.ExternalID = &fitid
		}
		if memo != "" && memo != description {
			row.Merchant = &memo
		}
		if check := ofxTag(block, "CHECKNUM"); check != "" {
			row.Reference = &check
		}
		rows = append(rows, row)
		searchFrom = start + end
	}
	if len(rows) == 0 {
		return nil, 0, Invalid("no transactions found in the OFX file")
	}
	return rows, skipped, nil
}

// ofxTag extracts a tag value from an OFX transaction block, with or without a
// closing tag (OFX 1.x is SGML).
func ofxTag(block, tag string) string {
	re := regexp.MustCompile(`(?is)<` + tag + `>([^<\r\n]*)`)
	match := re.FindStringSubmatch(block)
	if match == nil {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func parseOFXDate(raw string, loc *time.Location) (time.Time, bool) {
	value := strings.TrimSpace(raw)
	if idx := strings.IndexByte(value, '['); idx >= 0 {
		value = value[:idx]
	}
	value = strings.TrimSpace(value)
	for _, layout := range []string{"20060102150405", "200601021504", "20060102"} {
		if len(value) >= len(layout) {
			if t, err := time.ParseInLocation(layout, value[:len(layout)], loc); err == nil {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

// --- Deduplication ---

// dedupeHash builds a stable per-account fingerprint used to keep repeated
// imports of the same statement idempotent.
func dedupeHash(accountID uuid.UUID, row ImportRow) string {
	parts := []string{
		accountID.String(),
		row.OccurredAt.Format("2006-01-02"),
		row.Amount.String(),
		normalizeText(row.Description),
	}
	if row.ExternalID != nil {
		parts = append(parts, "id:"+strings.TrimSpace(*row.ExternalID))
	} else if row.Reference != nil {
		parts = append(parts, "ref:"+normalizeText(*row.Reference))
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

// hashRows computes dedupe hashes, appending an occurrence suffix when the
// same line appears more than once inside one statement.
func hashRows(accountID uuid.UUID, rows []ImportRow) []string {
	seen := map[string]int{}
	hashes := make([]string, len(rows))
	for i, row := range rows {
		base := dedupeHash(accountID, row)
		seen[base]++
		if seen[base] > 1 {
			hashes[i] = base + ":" + strconv.Itoa(seen[base])
		} else {
			hashes[i] = base
		}
	}
	return hashes
}

func normalizeText(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}

func truncateText(s string, max int) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

// foldAccents removes diacritics so accented headers still match keywords.
func foldAccents(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case 'á', 'à', 'ä', 'â', 'ã':
			r = 'a'
		case 'é', 'è', 'ë', 'ê':
			r = 'e'
		case 'í', 'ì', 'ï', 'î':
			r = 'i'
		case 'ó', 'ò', 'ö', 'ô', 'õ':
			r = 'o'
		case 'ú', 'ù', 'ü', 'û':
			r = 'u'
		case 'ñ':
			r = 'n'
		case 'ç':
			r = 'c'
		}
		b.WriteRune(r)
	}
	return b.String()
}
