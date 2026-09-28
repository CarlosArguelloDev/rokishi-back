package document

import (
	_ "embed"
	"fmt"
	"strconv"
	"strings"

	"github.com/signintech/gopdf"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"rokishi-back/internal/models"
)

const (
	pageWidth = 612.0
	margin    = 48.0
	contentW  = pageWidth - margin*2
	fontBody  = "RokishiRegular"
	fontBold  = "RokishiBold"
)

var (
	//go:embed assets/rokishi-logo.png
	logoBytes []byte
)

type QuotePDFGenerator struct{}

func NewQuotePDFGenerator() *QuotePDFGenerator { return &QuotePDFGenerator{} }

func (g *QuotePDFGenerator) Generate(quote models.Quote, options models.QuotePDFOptions) ([]byte, error) {
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeLetter})
	if err := pdf.AddTTFFontData(fontBody, goregular.TTF); err != nil {
		return nil, err
	}
	if err := pdf.AddTTFFontData(fontBold, gobold.TTF); err != nil {
		return nil, err
	}
	logo, err := gopdf.ImageHolderByBytes(logoBytes)
	if err != nil {
		return nil, err
	}

	doc := quoteDocument{pdf: pdf, quote: quote, options: options, logo: logo}
	if err := doc.render(); err != nil {
		return nil, err
	}
	return pdf.GetBytesPdfReturnErr()
}

type quoteDocument struct {
	pdf     *gopdf.GoPdf
	quote   models.Quote
	options models.QuotePDFOptions
	logo    gopdf.ImageHolder
	page    int
}

func (d *quoteDocument) render() error {
	d.addPage(false)
	if err := d.drawHeader(); err != nil {
		return err
	}
	informationBottom, err := d.drawInformation()
	if err != nil {
		return err
	}

	y := informationBottom + 28
	if err := d.sectionTitle("DETALLE DE LA COTIZACIÓN", y); err != nil {
		return err
	}
	y += 24
	if err := d.drawTableHeader(y); err != nil {
		return err
	}
	y += 28
	for index, concept := range d.quote.Concepts {
		description := conceptDescription(concept, index)
		lines, err := d.wrap(description, 264)
		if err != nil {
			return err
		}
		rowHeight := maxFloat(36, float64(len(lines))*11+14)
		if y+rowHeight > 650 {
			d.drawFooter()
			d.addPage(true)
			y = 92
			if err := d.drawTableHeader(y); err != nil {
				return err
			}
			y += 28
		}
		if err := d.drawConceptRow(y, rowHeight, lines, concept, index%2 == 1); err != nil {
			return err
		}
		y += rowHeight
	}

	if y > 500 {
		d.drawFooter()
		d.addPage(true)
		y = 92
		if err := d.sectionTitle("RESUMEN DE LA COTIZACIÓN", y); err != nil {
			return err
		}
		y += 28
	}
	if err := d.drawTotals(y); err != nil {
		return err
	}
	y += 116
	if err := d.drawNotes(&y); err != nil {
		return err
	}
	if y+75 > 730 {
		d.drawFooter()
		d.addPage(true)
		y = 100
	}
	if err := d.drawSignature(y + 24); err != nil {
		return err
	}
	d.drawFooter()
	return nil
}

func (d *quoteDocument) addPage(continuation bool) {
	d.pdf.AddPage()
	d.page++
	if continuation {
		d.pdf.SetStrokeColor(85, 72, 221)
		d.pdf.SetLineWidth(2)
		d.pdf.Line(margin, 55, pageWidth-margin, 55)
		_ = d.setFont(fontBold, 10, 85, 72, 221)
		d.pdf.SetXY(margin, 38)
		_ = d.cell(280, 14, "Rokishi3D - Cotización", gopdf.Left|gopdf.Middle)
		_ = d.setFont(fontBody, 9, 102, 112, 133)
		d.pdf.SetXY(pageWidth-margin-170, 38)
		_ = d.cell(170, 14, folio(d.quote.ID), gopdf.Right|gopdf.Middle)
	}
}

func (d *quoteDocument) drawHeader() error {
	if err := d.pdf.ImageByHolder(d.logo, margin, 39, &gopdf.Rect{W: 50, H: 50}); err != nil {
		return err
	}
	if err := d.setFont(fontBold, 18, 34, 37, 48); err != nil {
		return err
	}
	d.pdf.SetXY(106, 42)
	if err := d.cell(230, 22, "Rokishi3D", gopdf.Left|gopdf.Middle); err != nil {
		return err
	}
	if err := d.setFont(fontBody, 9, 102, 112, 133); err != nil {
		return err
	}
	d.pdf.SetXY(106, 66)
	if err := d.cell(230, 12, "Impresión 3D", gopdf.Left|gopdf.Middle); err != nil {
		return err
	}
	d.pdf.SetXY(106, 80)
	if err := d.cell(250, 12, "contacto@rokishi3d.com | 427", gopdf.Left|gopdf.Middle); err != nil {
		return err
	}

	if err := d.setFont(fontBold, 22, 85, 72, 221); err != nil {
		return err
	}
	d.pdf.SetXY(352, 40)
	if err := d.cell(212, 25, "COTIZACIÓN", gopdf.Right|gopdf.Middle); err != nil {
		return err
	}
	if err := d.setFont(fontBold, 10, 34, 37, 48); err != nil {
		return err
	}
	d.pdf.SetXY(352, 68)
	if err := d.cell(212, 13, "N.º "+folio(d.quote.ID), gopdf.Right|gopdf.Middle); err != nil {
		return err
	}
	if err := d.setFont(fontBody, 9, 102, 112, 133); err != nil {
		return err
	}
	d.pdf.SetXY(352, 83)
	return d.cell(212, 13, "Fecha: "+d.quote.CreationDate.Format("02/01/2006")+"   |   Vigencia: "+strconv.Itoa(d.options.ValidityDays)+" días", gopdf.Right|gopdf.Middle)
}

func (d *quoteDocument) drawInformation() (float64, error) {
	const y = 118.0
	customer, err := d.prepareInfoLines(customerLines(d.quote), true)
	if err != nil {
		return 0, err
	}
	payment, err := d.prepareInfoLines([]string{
		"Anticipo: " + d.options.Deposit,
		"Saldo: " + d.options.Balance,
		"Forma de pago: " + d.options.PaymentMethod,
	}, false)
	if err != nil {
		return 0, err
	}
	lineCount := len(customer)
	if len(payment) > lineCount {
		lineCount = len(payment)
	}
	height := maxFloat(92, 42+float64(lineCount)*13)
	d.pdf.SetFillColor(247, 247, 251)
	d.pdf.RectFromUpperLeftWithStyle(margin, y, 250, height, "F")
	d.pdf.RectFromUpperLeftWithStyle(314, y, 250, height, "F")
	if err := d.infoBlock(margin+14, y+12, "COTIZADO PARA", customer); err != nil {
		return 0, err
	}
	if err := d.infoBlock(328, y+12, "CONDICIONES DE PAGO", payment); err != nil {
		return 0, err
	}
	return y + height, nil
}

type infoLine struct {
	text string
	bold bool
}

func (d *quoteDocument) prepareInfoLines(values []string, firstBold bool) ([]infoLine, error) {
	result := make([]infoLine, 0, len(values))
	for index, value := range values {
		bold := firstBold && index == 0
		font := fontBody
		if bold {
			font = fontBold
		}
		if err := d.setFont(font, 8.5, 67, 71, 85); err != nil {
			return nil, err
		}
		wrapped, err := d.wrap(value, 222)
		if err != nil {
			return nil, err
		}
		for _, line := range wrapped {
			result = append(result, infoLine{text: line, bold: bold})
		}
	}
	return result, nil
}

func (d *quoteDocument) infoBlock(x, y float64, heading string, lines []infoLine) error {
	if err := d.setFont(fontBold, 8, 85, 72, 221); err != nil {
		return err
	}
	d.pdf.SetXY(x, y)
	if err := d.cell(222, 12, heading, gopdf.Left|gopdf.Middle); err != nil {
		return err
	}
	for index, line := range lines {
		font := fontBody
		color := [3]uint8{67, 71, 85}
		if line.bold {
			font = fontBold
			color = [3]uint8{34, 37, 48}
		}
		if err := d.setFont(font, 8.5, color[0], color[1], color[2]); err != nil {
			return err
		}
		d.pdf.SetXY(x, y+18+float64(index)*13)
		if err := d.cell(222, 12, line.text, gopdf.Left|gopdf.Middle); err != nil {
			return err
		}
	}
	return nil
}

func (d *quoteDocument) sectionTitle(title string, y float64) error {
	if err := d.setFont(fontBold, 10, 85, 72, 221); err != nil {
		return err
	}
	d.pdf.SetXY(margin, y)
	return d.cell(contentW, 14, title, gopdf.Left|gopdf.Middle)
}

func (d *quoteDocument) drawTableHeader(y float64) error {
	d.pdf.SetFillColor(85, 72, 221)
	d.pdf.RectFromUpperLeftWithStyle(margin, y, contentW, 28, "F")
	if err := d.setFont(fontBold, 8, 255, 255, 255); err != nil {
		return err
	}
	headers := []struct {
		x, w  float64
		text  string
		align int
	}{
		{margin + 10, 270, "DESCRIPCIÓN", gopdf.Left | gopdf.Middle},
		{328, 46, "CANT.", gopdf.Center | gopdf.Middle},
		{382, 78, "PRECIO UNIT.", gopdf.Right | gopdf.Middle},
		{468, 86, "IMPORTE", gopdf.Right | gopdf.Middle},
	}
	for _, header := range headers {
		d.pdf.SetXY(header.x, y)
		if err := d.cell(header.w, 28, header.text, header.align); err != nil {
			return err
		}
	}
	return nil
}

func (d *quoteDocument) drawConceptRow(y, height float64, description []string, concept models.QuoteConcept, shaded bool) error {
	if shaded {
		d.pdf.SetFillColor(247, 247, 251)
	} else {
		d.pdf.SetFillColor(255, 255, 255)
	}
	d.pdf.RectFromUpperLeftWithStyle(margin, y, contentW, height, "F")
	d.pdf.SetStrokeColor(226, 228, 234)
	d.pdf.SetLineWidth(0.5)
	d.pdf.Line(margin, y+height, pageWidth-margin, y+height)
	if err := d.setFont(fontBody, 8.5, 52, 55, 65); err != nil {
		return err
	}
	for index, line := range description {
		d.pdf.SetXY(margin+10, y+7+float64(index)*11)
		if err := d.cell(264, 11, line, gopdf.Left|gopdf.Middle); err != nil {
			return err
		}
	}
	d.pdf.SetXY(328, y)
	if err := d.cell(46, height, strconv.FormatInt(concept.PieceCount, 10), gopdf.Center|gopdf.Middle); err != nil {
		return err
	}
	d.pdf.SetXY(382, y)
	if err := d.cell(78, height, formatMoney(concept.SuggestedPricePerPiece), gopdf.Right|gopdf.Middle); err != nil {
		return err
	}
	d.pdf.SetXY(468, y)
	return d.cell(86, height, formatMoney(concept.SuggestedPrice), gopdf.Right|gopdf.Middle)
}

func (d *quoteDocument) drawTotals(y float64) error {
	rows := []struct {
		label string
		value string
		total bool
	}{
		{"Subtotal", formatMoney(d.quote.TotalSuggestedPrice), false},
		{"Descuento " + percentageLabel(d.options.DiscountBasisPoints), "- " + formatMoney(d.options.DiscountAmount), false},
		{"IVA " + percentageLabel(d.options.TaxBasisPoints), formatMoney(d.options.TaxAmount), false},
		{"TOTAL", formatMoney(d.options.TotalAfterDiscountTax), true},
	}
	for index, row := range rows {
		rowY := y + float64(index)*27
		if row.total {
			d.pdf.SetFillColor(238, 240, 255)
			d.pdf.RectFromUpperLeftWithStyle(314, rowY, 250, 27, "F")
		}
		font := fontBody
		size := 9.0
		color := [3]uint8{67, 71, 85}
		if row.total {
			font, size, color = fontBold, 11, [3]uint8{85, 72, 221}
		}
		if err := d.setFont(font, size, color[0], color[1], color[2]); err != nil {
			return err
		}
		d.pdf.SetXY(326, rowY)
		if err := d.cell(126, 27, row.label, gopdf.Left|gopdf.Middle); err != nil {
			return err
		}
		d.pdf.SetXY(456, rowY)
		if err := d.cell(96, 27, row.value, gopdf.Right|gopdf.Middle); err != nil {
			return err
		}
	}
	return nil
}

func (d *quoteDocument) drawNotes(y *float64) error {
	if err := d.sectionTitle("NOTAS Y TÉRMINOS", *y); err != nil {
		return err
	}
	*y += 22
	paragraphs := []string{
		fmt.Sprintf("Vigencia: %d días a partir de la fecha de emisión. Tiempo estimado de producción: %s. El descuento, si aplica, se resta del subtotal antes de calcular el IVA. Los cambios de diseño o trabajos adicionales se cotizan por separado.", d.options.ValidityDays, d.options.ProductionTime),
	}
	if d.quote.Notes != nil && strings.TrimSpace(*d.quote.Notes) != "" {
		paragraphs = append(paragraphs, "Notas de la cotización: "+strings.TrimSpace(*d.quote.Notes))
	}
	if d.options.Specifications != "" {
		paragraphs = append(paragraphs, "Especificaciones adicionales: "+d.options.Specifications)
	}
	for _, paragraph := range paragraphs {
		if err := d.setFont(fontBody, 8.5, 67, 71, 85); err != nil {
			return err
		}
		lines, err := d.wrap(paragraph, contentW)
		if err != nil {
			return err
		}
		for _, line := range lines {
			if *y+12 > 720 {
				d.drawFooter()
				d.addPage(true)
				*y = 90
				if err := d.sectionTitle("NOTAS Y TÉRMINOS (CONT.)", *y); err != nil {
					return err
				}
				*y += 22
				if err := d.setFont(fontBody, 8.5, 67, 71, 85); err != nil {
					return err
				}
			}
			d.pdf.SetXY(margin, *y)
			if err := d.cell(contentW, 12, line, gopdf.Left|gopdf.Middle); err != nil {
				return err
			}
			*y += 12
		}
		*y += 6
	}
	return nil
}

func (d *quoteDocument) drawSignature(y float64) error {
	d.pdf.SetStrokeColor(102, 112, 133)
	d.pdf.SetLineWidth(0.6)
	d.pdf.Line(margin, y, margin+210, y)
	if err := d.setFont(fontBody, 8, 102, 112, 133); err != nil {
		return err
	}
	d.pdf.SetXY(margin, y+7)
	return d.cell(210, 12, "Nombre y firma de aceptación", gopdf.Center|gopdf.Middle)
}

func (d *quoteDocument) drawFooter() {
	d.pdf.SetStrokeColor(226, 228, 234)
	d.pdf.SetLineWidth(0.5)
	d.pdf.Line(margin, 754, pageWidth-margin, 754)
	_ = d.setFont(fontBody, 7.5, 102, 112, 133)
	d.pdf.SetXY(margin, 760)
	_ = d.cell(260, 10, "Rokishi3D | "+folio(d.quote.ID), gopdf.Left|gopdf.Middle)
	d.pdf.SetXY(pageWidth-margin-80, 760)
	_ = d.cell(80, 10, "Página "+strconv.Itoa(d.page), gopdf.Right|gopdf.Middle)
}

func (d *quoteDocument) setFont(font string, size float64, r, g, b uint8) error {
	if err := d.pdf.SetFont(font, "", size); err != nil {
		return err
	}
	d.pdf.SetTextColor(r, g, b)
	return nil
}

func (d *quoteDocument) cell(width, height float64, text string, align int) error {
	return d.pdf.CellWithOption(&gopdf.Rect{W: width, H: height}, text, gopdf.CellOption{Align: align})
}

func (d *quoteDocument) wrap(text string, width float64) ([]string, error) {
	paragraphs := strings.Split(strings.ReplaceAll(text, "\r", ""), "\n")
	lines := make([]string, 0)
	for _, paragraph := range paragraphs {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		current := ""
		for _, word := range words {
			wordParts, err := d.splitWord(word, width)
			if err != nil {
				return nil, err
			}
			for partIndex, part := range wordParts {
				if partIndex > 0 {
					if current != "" {
						lines = append(lines, current)
					}
					current = part
					continue
				}
				candidate := part
				if current != "" {
					candidate = current + " " + part
				}
				measured, err := d.pdf.MeasureTextWidth(candidate)
				if err != nil {
					return nil, err
				}
				if measured <= width {
					current = candidate
					continue
				}
				if current != "" {
					lines = append(lines, current)
				}
				current = part
			}
		}
		if current != "" {
			lines = append(lines, current)
		}
	}
	return lines, nil
}

func (d *quoteDocument) splitWord(word string, width float64) ([]string, error) {
	measured, err := d.pdf.MeasureTextWidth(word)
	if err != nil {
		return nil, err
	}
	if measured <= width {
		return []string{word}, nil
	}
	parts := make([]string, 0, 2)
	current := ""
	for _, character := range []rune(word) {
		candidate := current + string(character)
		measured, err := d.pdf.MeasureTextWidth(candidate)
		if err != nil {
			return nil, err
		}
		if measured <= width {
			current = candidate
			continue
		}
		if current != "" {
			parts = append(parts, current)
		}
		current = string(character)
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts, nil
}

func customerLines(quote models.Quote) []string {
	lines := []string{quote.CustomerName}
	if quote.CustomerEmail != nil && strings.TrimSpace(*quote.CustomerEmail) != "" {
		lines = append(lines, strings.TrimSpace(*quote.CustomerEmail))
	}
	if quote.CustomerPhone != nil && strings.TrimSpace(*quote.CustomerPhone) != "" {
		lines = append(lines, strings.TrimSpace(*quote.CustomerPhone))
	}
	if len(lines) == 1 {
		lines = append(lines, "Datos de contacto no registrados")
	}
	return lines
}

func conceptDescription(concept models.QuoteConcept, index int) string {
	title := "Concepto " + strconv.Itoa(index+1)
	if concept.Description != nil && strings.TrimSpace(*concept.Description) != "" {
		title = strings.TrimSpace(*concept.Description)
	}
	return fmt.Sprintf("%s | %s | %s g | %d min", title, concept.MaterialName, concept.MaterialGrams.String(), concept.DurationMinutes)
}

func folio(id int64) string { return fmt.Sprintf("COT-%06d", id) }

func percentageLabel(basisPoints int64) string {
	value := float64(basisPoints) / 100
	return "[" + strconv.FormatFloat(value, 'f', -1, 64) + " %]"
}

func formatMoney(value models.Money) string {
	cents := int64(value)
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	whole := strconv.FormatInt(cents/100, 10)
	for index := len(whole) - 3; index > 0; index -= 3 {
		whole = whole[:index] + "," + whole[index:]
	}
	return sign + "$" + whole + "." + fmt.Sprintf("%02d", cents%100)
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
