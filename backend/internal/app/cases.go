package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ctc/internal/engine"
	"github.com/jackc/pgx/v5"
)

type CaseSummary struct {
	ID                 string  `json:"id"`
	CandidateName      string  `json:"candidateName"`
	CandidateReference string  `json:"candidateReference"`
	OwnerName          string  `json:"ownerName"`
	CreatedAt          string  `json:"createdAt"`
	UpdatedAt          string  `json:"updatedAt"`
	LatestRevisionID   string  `json:"latestRevisionId"`
	RevisionCount      int     `json:"revisionCount"`
	Status             string  `json:"status"`
	Gross              string  `json:"gross"`
	AnnualCTC          *string `json:"annualCTC"`
}
type Revision struct {
	ID              string        `json:"id"`
	Number          int           `json:"number"`
	ConfigurationID string        `json:"configurationId"`
	Input           engine.Input  `json:"input"`
	Result          engine.Result `json:"result"`
	CreatedAt       string        `json:"createdAt"`
	CreatedBy       string        `json:"createdBy"`
}

const caseSQL = `SELECT c.id,p.name,p.candidate_reference,u.name,c.created_at,c.updated_at,
 last.id,(SELECT count(*) FROM ctc_revisions rr WHERE rr.case_id=c.id),last.status,last.gross::text,last.result_snapshot->'totals'->>'annualCTC'
 FROM ctc_cases c JOIN candidates p ON p.id=c.candidate_id JOIN users u ON u.id=c.owner_user_id
 JOIN LATERAL (SELECT * FROM ctc_revisions r WHERE r.case_id=c.id ORDER BY revision_number DESC LIMIT 1) last ON true`

func scanCase(row pgx.Row) (CaseSummary, error) {
	var c CaseSummary
	var created, updated time.Time
	e := row.Scan(&c.ID, &c.CandidateName, &c.CandidateReference, &c.OwnerName, &created, &updated, &c.LatestRevisionID, &c.RevisionCount, &c.Status, &c.Gross, &c.AnnualCTC)
	c.CreatedAt = created.Format(time.RFC3339)
	c.UpdatedAt = updated.Format(time.RFC3339)
	return c, e
}
func (a *App) caseFor(ctx context.Context, id string, u User) (CaseSummary, error) {
	return scanCase(a.DB.QueryRow(ctx, caseSQL+" WHERE c.id=$1 AND ($2<>'hr' OR c.owner_user_id=$3)", id, u.Role, u.ID))
}
func (a *App) listCases(w http.ResponseWriter, r *http.Request, s session) {
	limit, offset, ok := pagination(w, r)
	if !ok {
		return
	}
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(search) > 200 {
		fail(w, 400, "Search must be at most 200 characters")
		return
	}
	rows, e := a.DB.Query(r.Context(), caseSQL+" WHERE ($1<>'hr' OR c.owner_user_id=$2) AND ($3='' OR position(lower($3) in lower(p.name || ' ' || p.candidate_reference))>0) ORDER BY c.updated_at DESC,c.id LIMIT $4 OFFSET $5", s.User.Role, s.User.ID, search, limit, offset)
	if e != nil {
		a.dbError(w, e)
		return
	}
	defer rows.Close()
	out := []CaseSummary{}
	for rows.Next() {
		c, e := scanCase(rows)
		if e != nil {
			a.dbError(w, e)
			return
		}
		out = append(out, c)
	}
	if e = rows.Err(); e != nil {
		a.dbError(w, e)
		return
	}
	JSON(w, 200, out)
}
func scanRevision(row pgx.Row) (Revision, error) {
	var v Revision
	var input, result []byte
	var at time.Time
	e := row.Scan(&v.ID, &v.Number, &v.ConfigurationID, &input, &result, &at, &v.CreatedBy)
	if e != nil {
		return v, e
	}
	if e = json.Unmarshal(input, &v.Input); e != nil {
		return v, e
	}
	if e = json.Unmarshal(result, &v.Result); e != nil {
		return v, e
	}
	v.CreatedAt = at.Format(time.RFC3339)
	return v, nil
}

const revisionSQL = `SELECT r.id,r.revision_number,r.configuration_id,r.input_snapshot,r.result_snapshot,r.created_at,u.name FROM ctc_revisions r JOIN users u ON u.id=r.created_by`

func (a *App) getCase(w http.ResponseWriter, r *http.Request, s session) {
	limit, offset, ok := pagination(w, r)
	if !ok {
		return
	}
	c, e := a.caseFor(r.Context(), r.PathValue("id"), s.User)
	if errors.Is(e, pgx.ErrNoRows) {
		fail(w, 404, "CTC record not found")
		return
	}
	if e != nil {
		a.dbError(w, e)
		return
	}
	rows, e := a.DB.Query(r.Context(), revisionSQL+" WHERE r.case_id=$1 ORDER BY r.revision_number DESC LIMIT $2 OFFSET $3", c.ID, limit, offset)
	if e != nil {
		a.dbError(w, e)
		return
	}
	defer rows.Close()
	revisions := []Revision{}
	for rows.Next() {
		v, e := scanRevision(rows)
		if e != nil {
			a.dbError(w, e)
			return
		}
		revisions = append(revisions, v)
	}
	if e = rows.Err(); e != nil {
		a.dbError(w, e)
		return
	}
	JSON(w, 200, map[string]any{"case": c, "revisions": revisions})
}
func (a *App) revisionFor(ctx context.Context, id string, u User) (Revision, error) {
	return scanRevision(a.DB.QueryRow(ctx, revisionSQL+" JOIN ctc_cases c ON c.id=r.case_id WHERE r.id=$1 AND ($2<>'hr' OR c.owner_user_id=$3)", id, u.Role, u.ID))
}
func (a *App) saveCase(w http.ResponseWriter, r *http.Request, s session) {
	var in calculationRequest
	if !readJSON(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Input.CandidateName) == "" {
		fail(w, 400, "Candidate name is required to save a record")
		return
	}
	c, e := a.config(r.Context(), in.ConfigurationID)
	if errors.Is(e, pgx.ErrNoRows) {
		fail(w, 404, "Configuration not found")
		return
	}
	if e != nil {
		a.dbError(w, e)
		return
	}
	if !a.allowCalculation(w, c, in.Input, s.User) {
		return
	}
	result := engine.Calculate(c, in.Input)
	for _, issue := range result.Issues {
		if issue.Severity == "error" {
			fail(w, 422, "Resolve calculation errors before saving: "+issue.Message)
			return
		}
	}
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		a.dbError(w, e)
		return
	}
	defer tx.Rollback(r.Context())
	inputJSON, _ := json.Marshal(in.Input)
	resultJSON, _ := json.Marshal(result)
	caseID := in.CaseID
	if caseID == "" {
		candidateID := ID()
		caseID = ID()
		_, e = tx.Exec(r.Context(), "INSERT INTO candidates(id,candidate_reference,name,profile_snapshot) VALUES($1,$2,$3,$4)", candidateID, in.Input.CandidateReference, in.Input.CandidateName, inputJSON)
		if e == nil {
			_, e = tx.Exec(r.Context(), "INSERT INTO ctc_cases(id,candidate_id,owner_user_id) VALUES($1,$2,$3)", caseID, candidateID, s.User.ID)
		}
	} else {
		var owner string
		e = tx.QueryRow(r.Context(), "SELECT owner_user_id FROM ctc_cases WHERE id=$1 FOR UPDATE", caseID).Scan(&owner)
		if errors.Is(e, pgx.ErrNoRows) || (e == nil && s.User.Role == "hr" && owner != s.User.ID) {
			fail(w, 404, "CTC record not found")
			return
		}
		if e == nil {
			var latest string
			e = tx.QueryRow(r.Context(), "SELECT id FROM ctc_revisions WHERE case_id=$1 ORDER BY revision_number DESC LIMIT 1", caseID).Scan(&latest)
			if e == nil && ((!a.Demo && in.ExpectedRevisionID == "") || (in.ExpectedRevisionID != "" && in.ExpectedRevisionID != latest)) {
				fail(w, 409, "This record has changed. Reopen its latest revision before saving.")
				return
			}
		}
		if e == nil {
			_, e = tx.Exec(r.Context(), "UPDATE candidates SET candidate_reference=$2,name=$3,profile_snapshot=$4 WHERE id=(SELECT candidate_id FROM ctc_cases WHERE id=$1)", caseID, in.Input.CandidateReference, in.Input.CandidateName, inputJSON)
		}
	}
	if e != nil {
		a.dbError(w, e)
		return
	}
	var number int
	if e = tx.QueryRow(r.Context(), "SELECT COALESCE(max(revision_number),0)+1 FROM ctc_revisions WHERE case_id=$1", caseID).Scan(&number); e != nil {
		a.dbError(w, e)
		return
	}
	revisionID := ID()
	status := "incomplete"
	if result.Complete {
		status = "complete"
	}
	_, e = tx.Exec(r.Context(), `INSERT INTO ctc_revisions(id,case_id,revision_number,configuration_id,engine_version,grade_id,location_id,qualification_code,gross,basic,tax_regime,nps_opt_in,input_snapshot,result_snapshot,status,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`, revisionID, caseID, number, c.ID, engine.Version, in.Input.GradeID, in.Input.LocationID, in.Input.Qualification, in.Input.Gross, in.Input.Basic, in.Input.TaxRegime, in.Input.NPSOptIn, inputJSON, resultJSON, status, s.User.ID)
	if e != nil {
		a.dbError(w, e)
		return
	}
	for _, line := range result.Lines {
		_, e = tx.Exec(r.Context(), `INSERT INTO calculation_lines(revision_id,configuration_id,component_code,monthly_amount,annual_amount,amount,amount_period,applicability_status,formula_trace) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, revisionID, c.ID, line.Code, line.Monthly, line.Annual, line.Amount, nullable(line.Period), line.Status, line.Formula)
		if e != nil {
			a.dbError(w, e)
			return
		}
	}
	if _, e = tx.Exec(r.Context(), "UPDATE ctc_cases SET updated_at=now() WHERE id=$1", caseID); e != nil {
		a.dbError(w, e)
		return
	}
	details, _ := json.Marshal(map[string]any{"revisionId": revisionID, "revisionNumber": number, "configurationId": c.ID, "configurationVersion": c.Version, "engineVersion": engine.Version, "status": status})
	_, e = tx.Exec(r.Context(), "INSERT INTO audit_events(id,actor_id,action,entity_type,entity_id,details) VALUES($1,$2,'ctc.save','case',$3,$4)", ID(), s.User.ID, caseID, details)
	if e == nil {
		e = tx.Commit(r.Context())
	}
	if e != nil {
		a.dbError(w, e)
		return
	}
	JSON(w, 201, map[string]any{"caseId": caseID, "revisionId": revisionID, "result": result})
}

func pagination(w http.ResponseWriter, r *http.Request) (int, int, bool) {
	limit, offset := 100, 0
	var err error
	if s := r.URL.Query().Get("limit"); s != "" {
		limit, err = strconv.Atoi(s)
		if err != nil || limit < 1 || limit > 500 {
			fail(w, 400, "limit must be 1-500")
			return 0, 0, false
		}
	}
	if s := r.URL.Query().Get("offset"); s != "" {
		offset, err = strconv.Atoi(s)
		if err != nil || offset < 0 || offset > 1000000 {
			fail(w, 400, "offset must be 0-1000000")
			return 0, 0, false
		}
	}
	w.Header().Set("X-Page-Limit", strconv.Itoa(limit))
	w.Header().Set("X-Page-Offset", strconv.Itoa(offset))
	return limit, offset, true
}

func (a *App) assignCase(w http.ResponseWriter, r *http.Request, s session) {
	var in struct {
		OwnerUserID string `json:"ownerUserId"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		a.dbError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var owner string
	err = tx.QueryRow(r.Context(), "SELECT owner_user_id FROM ctc_cases WHERE id=$1 FOR UPDATE", r.PathValue("id")).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "CTC record not found")
		return
	}
	if err != nil {
		a.dbError(w, err)
		return
	}
	var role string
	err = tx.QueryRow(r.Context(), "SELECT role FROM users WHERE id=$1 AND active=true FOR SHARE", in.OwnerUserID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && role != "hr") {
		fail(w, 400, "Choose an active HR user")
		return
	}
	if err != nil {
		a.dbError(w, err)
		return
	}
	_, err = tx.Exec(r.Context(), "UPDATE ctc_cases SET owner_user_id=$2,updated_at=now() WHERE id=$1", r.PathValue("id"), in.OwnerUserID)
	if err == nil {
		err = auditTx(r.Context(), tx, s.User.ID, "case.assign", "case", r.PathValue("id"), map[string]string{"previousOwnerId": owner, "ownerUserId": in.OwnerUserID})
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		a.dbError(w, err)
		return
	}
	JSON(w, 200, map[string]bool{"ok": true})
}
