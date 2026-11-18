package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"
	"time"

	"ctc/internal/engine"
	"github.com/chromedp/chromedp"
	"github.com/jackc/pgx/v5"
)

type docGroup struct {
	Name  string
	Lines []engine.Line
}
type docData struct {
	Title         string
	Mode          string
	Draft         bool
	IsOffer       bool
	Revision      Revision
	Configuration engine.Configuration
	Template      engine.OfferTemplate
	Groups        []docGroup
	GradeName     string
	LocationName  string
	Generated     string
}

var docTemplate = template.Must(template.New("document").Funcs(template.FuncMap{
	"money": func(v *string) string {
		if v == nil {
			return "Pending"
		}
		return "INR " + *v
	},
	"groupName": func(s string) string { return strings.Title(s) },
}).Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><title>{{.Title}}</title><style>
@page{size:A4;margin:16mm}*{box-sizing:border-box}body{font-family:"Noto Sans",Arial,sans-serif;font-size:10px;color:#203047;line-height:1.5}
.watermark{position:fixed;top:43%;left:4%;transform:rotate(-28deg);font-size:66px;font-weight:800;opacity:.07;z-index:20;pointer-events:none;white-space:nowrap}
.brand{font-size:22px;font-weight:700;color:#134e4a}.eyebrow{font-size:9px;letter-spacing:2px;color:#52756f;text-transform:uppercase}h1{font-size:24px;margin:8px 0}h2{break-after:avoid;page-break-after:avoid;font-size:13px;margin:18px 0 5px;color:#115e59}
.banner{padding:12px 14px;background:#fff5dc;border:1px solid #e6c47c;color:#79521a;margin:14px 0;font-weight:600}
.profile{display:grid;grid-template-columns:1fr 1fr;gap:6px;padding:12px;background:#f3f7f6;border-radius:6px}.label{color:#64748b}
table{width:100%;border-collapse:collapse;margin-top:5px}th{text-align:left;font-size:9px;background:#eaf2f0;color:#45625c}
td,th{padding:7px 8px;border-bottom:1px solid #e4e9ec}td.num,th.num{text-align:right;white-space:nowrap}tr{break-inside:avoid}
.subtle{font-size:8px;color:#64748b}.summary{padding:14px;background:#eaf5ef;border:1px solid #b8d6c5;margin-top:16px}.sumrow{display:flex;justify-content:space-between;padding:4px}
.total{font-weight:700;font-size:15px;border-top:1px solid #b8d6c5;margin-top:6px;padding-top:8px}.terms{white-space:pre-line;margin:16px 0}
.footer{border-top:1px solid #d7e0de;margin-top:24px;padding-top:8px;font-size:8px;color:#64748b}.pending{color:#a76a17}
</style></head><body>{{if .Draft}}<div class="watermark">DEMO / DRAFT</div>{{end}}
<div class="eyebrow">Compensation workspace</div><div class="brand">{{.Template.CompanyName}}</div><div class="subtle">{{.Template.Address}}</div>
<h1>{{.Title}}</h1><div class="subtle">Revision {{.Revision.Number}} &middot; {{.Revision.ID}} &middot; Configuration {{.Configuration.Version}} &middot; Engine {{.Revision.Result.EngineVersion}}</div>
{{if .Draft}}<div class="banner">DEMO / DRAFT &mdash; NOT AN EMPLOYMENT OFFER. Unresolved rules are shown as Pending. Known subtotal is not final CTC.</div>{{end}}
<div class="profile">
<div><span class="label">Candidate</span><br><strong>{{.Revision.Input.CandidateName}}</strong></div>
<div><span class="label">Candidate reference</span><br>{{.Revision.Input.CandidateReference}}</div>
<div><span class="label">Designation / Department</span><br>{{.Revision.Input.Designation}} / {{.Revision.Input.Department}}</div>
<div><span class="label">Proposed joining date</span><br>{{.Revision.Input.JoiningDate}}</div>
<div><span class="label">Grade / Location</span><br>{{.GradeName}} / {{.LocationName}}</div>
<div><span class="label">Tax regime / NPS election</span><br>{{.Revision.Input.TaxRegime}} / {{if .Revision.Input.NPSOptIn}}Opted in{{else}}Opted out{{end}}</div>
{{if .Revision.Input.CalculationDate}}<div><span class="label">Calculation effective date</span><br>{{.Revision.Input.CalculationDate}}</div>{{end}}
</div>
{{if .IsOffer}}<p>Dear {{.Revision.Input.CandidateName}},</p><div class="terms">{{.Template.Body}}</div><p>The compensation breakdown associated with this document is set out below.</p>{{end}}
{{range .Groups}}<h2>{{.Name}}</h2><table><thead><tr><th>Salary component</th><th>Period / status</th><th class="num">Monthly</th><th class="num">Annual CTC amount</th></tr></thead><tbody>
{{range .Lines}}<tr><td>{{.Name}}<div class="subtle">{{.Formula}}</div></td><td>{{.Period}}<br><span class="subtle">{{.Status}}</span></td><td class="num">{{if .Monthly}}{{money .Monthly}}{{else}}&mdash;{{end}}</td><td class="num {{if not .Annual}}pending{{end}}">{{money .Annual}}</td></tr>{{end}}
</tbody></table>{{end}}
<div class="summary">
<div class="sumrow"><span>Monthly Gross (includes Employer PF, NPS and Superannuation)</span><strong>INR {{.Revision.Result.Totals.Gross}}</strong></div>
<div class="sumrow"><span>Inclusion wages</span><strong>{{money .Revision.Result.Totals.Inclusion}}</strong></div>
<div class="sumrow"><span>Exclusion wages</span><strong>{{money .Revision.Result.Totals.Exclusion}}</strong></div>
<div class="sumrow"><span>Parity difference</span><strong>{{money .Revision.Result.Totals.Difference}}</strong></div>
<div class="sumrow total"><span>{{if .Revision.Result.Totals.AnnualCTC}}Annual CTC{{else}}Known annual subtotal &mdash; incomplete{{end}}</span><span>{{if .Revision.Result.Totals.AnnualCTC}}{{money .Revision.Result.Totals.AnnualCTC}}{{else}}INR {{.Revision.Result.Totals.KnownAnnualTotal}}{{end}}</span></div>
</div>
<p class="subtle">Annual values are CTC allocations under the selected configuration, not monthly take-home pay. Gratuity is shown as a CTC provision. Annual, one-time and scheduled payments are not assumed to be monthly cash. Tax computation is outside this application.</p>
{{if .Draft}}<h2>Items requiring confirmation</h2><ul>{{range .Revision.Result.Issues}}<li>{{.Message}}</li>{{end}}</ul>{{end}}
{{if .IsOffer}}<p>For {{.Template.CompanyName}},</p><p><strong>{{.Template.Signatory}}</strong></p>{{end}}
<div class="footer">Saved calculation: {{.Revision.CreatedAt}} &middot; Configuration: {{.Configuration.Name}} &middot; {{.Mode}}<br>Document generated from an immutable calculation revision. All amounts in INR.</div>
</body></html>`))

func renderDocument(v Revision, c engine.Configuration, t engine.OfferTemplate, kind, mode string) (string, error) {
	groups := []docGroup{}
	names := map[string]string{"inclusion": "1. Inclusion Wages", "exclusion": "2. Exclusion Wages", "statutory": "3. Statutory Payments", "policy": "4. Policy-Based Payments", "variable": "5. Variable Pay"}
	for _, g := range []string{"inclusion", "exclusion", "statutory", "policy", "variable"} {
		lines := []engine.Line{}
		for _, l := range v.Result.Lines {
			if l.Group == g {
				lines = append(lines, l)
			}
		}
		groups = append(groups, docGroup{names[g], lines})
	}
	title := "CTC Breakup Sheet"
	if kind == "offer" {
		title = "Offer Letter"
	}
	gradeName, locationName := v.Input.GradeID, v.Input.LocationID
	for _, g := range c.Data.Grades {
		if g.ID == v.Input.GradeID {
			gradeName = g.Name
		}
	}
	for _, l := range c.Data.Locations {
		if l.ID == v.Input.LocationID {
			locationName = l.Name
		}
	}
	var b bytes.Buffer
	e := docTemplate.Execute(&b, docData{Title: title, Mode: mode, Draft: mode != "final", IsOffer: kind == "offer", Revision: v, Configuration: c, Template: t, Groups: groups, GradeName: gradeName, LocationName: locationName})
	return b.String(), e
}
func (a *App) renderPDF(ctx context.Context, html string) ([]byte, error) {
	select {
	case a.pdfSlots <- struct{}{}:
		defer func() { <-a.pdfSlots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("document renderer is busy")
	}
	options := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	if path := os.Getenv("CHROME_PATH"); path != "" {
		options = append(options, chromedp.ExecPath(path))
	}
	options = append(options, chromedp.Flag("disable-dev-shm-usage", true))
	if a.Demo {
		options = append(options, chromedp.NoSandbox)
	}
	allocator, cancel := chromedp.NewExecAllocator(ctx, options...)
	defer cancel()
	browser, cancelBrowser := chromedp.NewContext(allocator)
	defer cancelBrowser()
	browser, cancelTimeout := context.WithTimeout(browser, 60*time.Second)
	defer cancelTimeout()
	url := "data:text/html;base64," + base64.StdEncoding.EncodeToString([]byte(html))
	if e := chromedp.Do(browser, chromedp.Navigate(url)); e != nil {
		return nil, e
	}
	return chromedp.Run(browser, chromedp.PrintToPDF(chromedp.PDFPrintBackground(), chromedp.PDFPreferCSSPageSize()))
}
func (a *App) document(w http.ResponseWriter, r *http.Request, s session) {
	kind, mode, format := r.PathValue("kind"), r.URL.Query().Get("mode"), r.URL.Query().Get("format")
	if mode == "" {
		mode = "draft"
	}
	if format == "" {
		format = "pdf"
	}
	if (kind != "ctc" && kind != "offer") || (mode != "draft" && mode != "final") || (format != "html" && format != "pdf") {
		fail(w, 400, "Choose ctc/offer, draft/final, and html/pdf")
		return
	}
	v, e := a.revisionFor(r.Context(), r.PathValue("id"), s.User)
	if errors.Is(e, pgx.ErrNoRows) {
		fail(w, 404, "Calculation revision not found")
		return
	}
	if e != nil {
		a.dbError(w, e)
		return
	}
	if !v.Result.CoreValid {
		fail(w, 422, "Document generation is blocked: core calculations and parity must be valid")
		return
	}
	if mode == "final" && !v.Result.CanFinalize {
		fail(w, 422, "Final generation requires a complete saved calculation, published client configuration, and approved template")
		return
	}
	if mode == "final" {
		if strings.TrimSpace(v.Input.CandidateName) == "" || strings.TrimSpace(v.Input.CandidateReference) == "" {
			fail(w, 422, "Candidate name and reference are required for final documents")
			return
		}
		if kind == "offer" {
			if _, e = time.Parse("2006-01-02", v.Input.JoiningDate); e != nil || strings.TrimSpace(v.Input.Designation) == "" {
				fail(w, 422, "A designation and valid joining date are required for a final offer")
				return
			}
		}
	}
	c, e := a.config(r.Context(), v.ConfigurationID)
	if e != nil {
		a.dbError(w, e)
		return
	}
	if !a.Demo && (c.Source != "client" || v.Input.CalculationDate == "") {
		fail(w, 422, "Create a production revision with a client configuration and explicit calculation effective date")
		return
	}
	if !a.Demo && mode == "final" {
		if errs := engine.ValidatePublication(c); len(errs) > 0 {
			fail(w, 422, "Final document requires complete approved production configuration")
			return
		}
	}
	var templateID string
	var templateJSON []byte
	e = a.DB.QueryRow(r.Context(), "SELECT id,template_snapshot FROM document_template_versions WHERE configuration_id=$1 AND document_type=$2", v.ConfigurationID, kind).Scan(&templateID, &templateJSON)
	if e != nil {
		a.dbError(w, e)
		return
	}
	var t engine.OfferTemplate
	if e = json.Unmarshal(templateJSON, &t); e != nil {
		a.dbError(w, e)
		return
	}
	html, e := renderDocument(v, c, t, kind, mode)
	if e != nil {
		fail(w, 500, "Document rendering failed")
		return
	}
	if format == "html" {
		if e = a.audit(r.Context(), s.User.ID, "document.preview", "revision", v.ID, map[string]string{"kind": kind, "mode": mode}); e != nil {
			a.dbError(w, e)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
		_, _ = w.Write([]byte(html))
		return
	}
	var content []byte
	var checksum string
	e = a.DB.QueryRow(r.Context(), "SELECT content,checksum FROM generated_documents WHERE revision_id=$1 AND document_type=$2 AND mode=$3", v.ID, kind, mode).Scan(&content, &checksum)
	if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		a.dbError(w, e)
		return
	}
	if e == nil && hash(string(content)) != checksum {
		fail(w, 500, "Stored document integrity check failed")
		return
	}
	if errors.Is(e, pgx.ErrNoRows) {
		content, e = a.renderPDF(r.Context(), html)
		if e != nil {
			fail(w, 503, "PDF renderer could not complete the document. Please retry.")
			return
		}
		tx, e := a.DB.Begin(r.Context())
		if e != nil {
			a.dbError(w, e)
			return
		}
		defer tx.Rollback(r.Context())
		id := ID()
		tag, e := tx.Exec(r.Context(), `INSERT INTO generated_documents(id,revision_id,template_version_id,document_type,mode,content,checksum,generated_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(revision_id,document_type,mode) DO NOTHING`, id, v.ID, templateID, kind, mode, content, hash(string(content)), s.User.ID)
		if e == nil && tag.RowsAffected() > 0 {
			details, _ := json.Marshal(map[string]any{"revisionId": v.ID, "templateVersionId": templateID, "type": kind, "mode": mode, "sha256": hash(string(content))})
			_, e = tx.Exec(r.Context(), "INSERT INTO audit_events(id,actor_id,action,entity_type,entity_id,details) VALUES($1,$2,'document.generate','document',$3,$4)", ID(), s.User.ID, id, details)
		} else if e == nil {
			e = tx.QueryRow(r.Context(), "SELECT content,checksum FROM generated_documents WHERE revision_id=$1 AND document_type=$2 AND mode=$3", v.ID, kind, mode).Scan(&content, &checksum)
			if e == nil && hash(string(content)) != checksum {
				e = fmt.Errorf("document integrity failure")
			}
		}
		if e == nil {
			e = tx.Commit(r.Context())
		}
		if e != nil {
			a.dbError(w, e)
			return
		}
	}
	if e = a.audit(r.Context(), s.User.ID, "document.download", "revision", v.ID, map[string]string{"kind": kind, "mode": mode, "sha256": hash(string(content))}); e != nil {
		a.dbError(w, e)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-%s-%s.pdf", kind, mode, v.ID))
	w.Header().Set("Content-Length", fmt.Sprint(len(content)))
	_, _ = w.Write(content)
}
