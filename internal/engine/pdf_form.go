package engine

import (
	"encoding/json"
	"fmt"
)

// PDFFormField represents a fillable field in a PDF form rendered by Chrome's PDF viewer.
type PDFFormField struct {
	Name     string `json:"name"`
	Type     string `json:"type"`      // text, checkbox, radio, select, button
	Value    string `json:"value"`
	Required bool   `json:"required"`
	ReadOnly bool   `json:"read_only"`
	Page     int    `json:"page"`
}

// pdfFormDetectJS is the JavaScript that extracts form fields from Chrome's built-in PDF viewer.
// Chrome renders PDFs with an embedded PDF viewer that exposes form annotations via the plugin API.
const pdfFormDetectJS = `() => {
	return new Promise((resolve, reject) => {
		// Wait for PDF viewer to be ready
		const maxWait = 10000;
		const start = Date.now();

		function tryExtract() {
			const embed = document.querySelector('embed[type="application/x-google-chrome-pdf"]');
			if (!embed) {
				// Try iframe-based PDF rendering (pdf.js)
				const viewer = document.querySelector('#viewer');
				if (viewer) {
					return extractFromPDFJS(resolve);
				}

				if (Date.now() - start < maxWait) {
					setTimeout(tryExtract, 200);
					return;
				}

				resolve([]);
				return;
			}

			// Chrome's native PDF viewer — use postMessage API
			resolve([]);
		}

		function extractFromPDFJS(cb) {
			// pdf.js based viewer
			if (typeof window.PDFViewerApplication !== 'undefined') {
				const app = window.PDFViewerApplication;
				if (!app.pdfDocument) {
					setTimeout(() => extractFromPDFJS(cb), 200);
					return;
				}

				const fields = [];
				const numPages = app.pdfDocument.numPages;
				let processed = 0;

				for (let i = 1; i <= numPages; i++) {
					app.pdfDocument.getPage(i).then(page => {
						page.getAnnotations().then(annots => {
							for (const annot of annots) {
								if (annot.subtype === 'Widget') {
									fields.push({
										name: annot.fieldName || '',
										type: mapFieldType(annot.fieldType),
										value: annot.fieldValue || '',
										required: annot.required || false,
										read_only: annot.readOnly || false,
										page: i,
									});
								}
							}
							processed++;
							if (processed === numPages) {
								cb(fields);
							}
						});
					});
				}

				if (numPages === 0) cb([]);
				return;
			}

			cb([]);
		}

		function mapFieldType(ft) {
			switch (ft) {
				case 'Tx': return 'text';
				case 'Btn': return 'checkbox';
				case 'Ch': return 'select';
				case 'Sig': return 'signature';
				default: return ft || 'unknown';
			}
		}

		tryExtract();
	});
}`

// pdfFormFillJS fills form fields in a PDF rendered by Chrome's PDF viewer.
const pdfFormFillJS = `(fields) => {
	return new Promise((resolve, reject) => {
		// Try pdf.js viewer
		if (typeof window.PDFViewerApplication !== 'undefined') {
			const app = window.PDFViewerApplication;
			if (!app.pdfDocument) {
				reject(new Error('PDF document not loaded'));
				return;
			}

			const fieldMap = {};
			for (const [k, v] of Object.entries(fields)) {
				fieldMap[k] = v;
			}

			let filled = 0;
			const numPages = app.pdfDocument.numPages;
			let processed = 0;

			for (let i = 1; i <= numPages; i++) {
				app.pdfDocument.getPage(i).then(page => {
					page.getAnnotations().then(annots => {
						for (const annot of annots) {
							if (annot.subtype === 'Widget' && annot.fieldName in fieldMap) {
								// Find the annotation layer element
								const el = document.querySelector(
									'[data-annotation-id="' + annot.id + '"] input, ' +
									'[data-annotation-id="' + annot.id + '"] select, ' +
									'[data-annotation-id="' + annot.id + '"] textarea'
								);
								if (el) {
									el.value = fieldMap[annot.fieldName];
									el.dispatchEvent(new Event('input', {bubbles: true}));
									el.dispatchEvent(new Event('change', {bubbles: true}));
									filled++;
								}
							}
						}
						processed++;
						if (processed === numPages) {
							resolve({filled: filled, total: Object.keys(fieldMap).length});
						}
					});
				});
			}

			if (numPages === 0) resolve({filled: 0, total: Object.keys(fieldMap).length});
			return;
		}

		// For Chrome's native viewer, use keyboard simulation
		reject(new Error('PDF form filling requires pdf.js viewer; Chrome native viewer has limited form API'));
	});
}`

// PDFFormFields detects fillable form fields in a PDF rendered by the browser.
// The page must have already navigated to a PDF URL.
func (p *Page) PDFFormFields() ([]PDFFormField, error) {
	if p == nil || p.page == nil {
		return nil, fmt.Errorf("scout: pdf form fields: nil page")
	}

	result, err := p.Eval(pdfFormDetectJS)
	if err != nil {
		return nil, fmt.Errorf("scout: pdf form fields: %w", err)
	}

	raw, err := json.Marshal(result.Value)
	if err != nil {
		return nil, fmt.Errorf("scout: pdf form fields: marshal: %w", err)
	}

	var fields []PDFFormField
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("scout: pdf form fields: unmarshal: %w", err)
	}

	return fields, nil
}

// FillPDFForm fills form fields in a PDF rendered by the browser.
// The fields map keys are field names, values are the values to fill.
func (p *Page) FillPDFForm(fields map[string]string) error {
	if p == nil || p.page == nil {
		return fmt.Errorf("scout: fill pdf form: nil page")
	}

	if len(fields) == 0 {
		return nil
	}

	result, err := p.Eval(pdfFormFillJS, fields)
	if err != nil {
		return fmt.Errorf("scout: fill pdf form: %w", err)
	}

	var fillResult struct {
		Filled int `json:"filled"`
		Total  int `json:"total"`
	}

	raw, err := json.Marshal(result.Value)
	if err != nil {
		return fmt.Errorf("scout: fill pdf form: marshal result: %w", err)
	}

	if err := json.Unmarshal(raw, &fillResult); err != nil {
		return fmt.Errorf("scout: fill pdf form: unmarshal result: %w", err)
	}

	if fillResult.Filled == 0 && fillResult.Total > 0 {
		return fmt.Errorf("scout: fill pdf form: no fields were filled (0/%d)", fillResult.Total)
	}

	return nil
}
