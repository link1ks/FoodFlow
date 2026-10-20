package main

// Local engineering knowledge is data, never executable instructions. Immutable
// revisions are authoritative; a disposable index must match their digest.
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

type Failure struct {
	ID                 string    `json:"id"`
	Revision           int       `json:"revision"`
	Task               string    `json:"task"`
	Category           string    `json:"category"`
	ErrorCode          string    `json:"error_code,omitempty"`
	ApplicableVersions []string  `json:"applicable_versions,omitempty"`
	Business           []string  `json:"business"`
	Tests              []string  `json:"tests"`
	CodeVersion        string    `json:"code_version"`
	ModelVersion       string    `json:"model_version"`
	Environment        string    `json:"environment"`
	Conditions         string    `json:"conditions"`
	Symptom            string    `json:"symptom"`
	Reproduction       string    `json:"reproduction"`
	RootCause          string    `json:"root_cause"`
	Verified           bool      `json:"verified"`
	Fix                string    `json:"fix"`
	Scope              string    `json:"scope"`
	Status             string    `json:"status"`
	Severity           int       `json:"severity"`
	Owner              string    `json:"owner"`
	Evidence           []string  `json:"evidence"`
	Regression         []string  `json:"regression"`
	PageEvidence       []string  `json:"page_evidence"`
	Replaces           string    `json:"replaces,omitempty"`
	RecurrenceOf       string    `json:"recurrence_of,omitempty"`
	Conflicts          []string  `json:"conflicts,omitempty"`
	RedactionReviewed  bool      `json:"redaction_reviewed"`
	RecordedAt         time.Time `json:"recorded_at"`
}

func hashJSON(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func validateFailure(root string, f Failure) error {
	if !taskName.MatchString(f.ID) || !taskName.MatchString(f.Task) || f.Revision < 1 ||
		!strings.Contains("|environment|code|data|model|unknown|", "|"+f.Category+"|") ||
		!strings.Contains("|open|resolved|obsolete|deleted|", "|"+f.Status+"|") ||
		len(f.Business) == 0 || len(f.Tests) == 0 || f.CodeVersion == "" || f.ModelVersion == "" ||
		f.Environment == "" || f.Conditions == "" || f.Symptom == "" || f.Reproduction == "" ||
		f.Scope == "" || f.Owner == "" || f.Severity < 1 || f.Severity > 5 || !f.RedactionReviewed {
		return fmt.Errorf("failure requires identity, classification, context, provenance and reviewed redaction")
	}
	if f.Verified && f.RootCause == "" {
		return fmt.Errorf("verified root cause is missing")
	}
	if f.Status == "resolved" && (!f.Verified || f.Fix == "" || len(f.Regression) == 0 || len(f.PageEvidence) == 0) {
		return fmt.Errorf("resolved failure requires verified cause, fix, regression and page evidence")
	}
	if f.Status == "resolved" && f.Category == "unknown" {
		return fmt.Errorf("resolved failure needs reviewed category")
	}
	raw, _ := json.Marshal(f)
	lower := strings.ToLower(string(raw))
	for _, forbidden := range []string{"bearer ", "api_key", "password=", "authorization:", "-----begin", "sk-"} {
		if strings.Contains(lower, forbidden) {
			return fmt.Errorf("potential credential in knowledge; redact before ingestion")
		}
	}
	if len(f.Evidence) == 0 {
		return fmt.Errorf("original evidence required")
	}
	for _, path := range append(append(append([]string{}, f.Evidence...), f.Regression...), f.PageEvidence...) {
		p, err := repositoryFile(root, path)
		if err != nil {
			return err
		}
		if !(strings.HasPrefix(path, ".cache/harness/") || strings.HasPrefix(path, "docs/evidence/")) {
			return fmt.Errorf("knowledge evidence must be reviewed harness/evidence artifacts")
		}
		if info, e := os.Stat(p); e != nil || info.IsDir() {
			return fmt.Errorf("missing evidence")
		}
	}
	return nil
}

func knowledgeRecords(root string) ([]Failure, error) {
	files, err := filepath.Glob(filepath.Join(root, ".cache/harness/experience/records/*.json"))
	if err != nil {
		return nil, err
	}
	current := map[string]Failure{}
	for _, path := range files {
		var f Failure
		raw, e := os.ReadFile(path)
		if e != nil {
			return nil, e
		}
		if e = json.Unmarshal(raw, &f); e != nil {
			return nil, e
		}
		if prev, ok := current[f.ID]; !ok || prev.Revision < f.Revision {
			current[f.ID] = f
		}
	}
	result := []Failure{}
	for _, f := range current {
		result = append(result, f)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func ingestFailure(root string, f Failure) error {
	if err := validateFailure(root, f); err != nil {
		return err
	}
	records, err := knowledgeRecords(root)
	if err != nil {
		return err
	}
	revision := 0
	recurrenceFound := f.RecurrenceOf == ""
	for _, old := range records {
		if old.ID == f.RecurrenceOf && old.Status == "resolved" && old.Verified && old.ID != f.ID {
			recurrenceFound = true
		}
		if old.ID == f.ID {
			revision = old.Revision
		}
		if old.ID != f.ID && old.Status != "deleted" && old.Symptom == f.Symptom && old.Conditions == f.Conditions {
			return fmt.Errorf("duplicate failure; update existing ID")
		}
	}
	if !recurrenceFound {
		return fmt.Errorf("recurrence must reference a verified resolved failure")
	}
	if f.Revision != revision+1 {
		return fmt.Errorf("revision must extend existing history")
	}
	f.RecordedAt = time.Now().UTC()
	dir := filepath.Join(root, ".cache/harness/experience/records")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err = writeExclusive(filepath.Join(dir, fmt.Sprintf("%s-%06d.json", f.ID, f.Revision)), f); err != nil {
		return err
	}
	// A rebuild can be retried without resending a model call. Queries reject an
	// old index even if this operation crashes between revision and index writes.
	return rebuildExperience(root)
}

// Explicit local domain-concept vectors provide bounded semantic matching,
// alongside lexical overlap. This is not a pretrained general embedding model.
var concepts = [][]string{
	{"duplicate", "replay", "idempotency", "重复", "重放", "幂等"},
	{"quantity", "decimal", "precision", "数量", "小数", "精度"},
	{"speech", "audio", "microphone", "voice", "语音", "录音", "麦克风"},
	{"image", "photo", "vision", "图片", "照片", "视觉", "拍照"},
	{"menu", "recipe", "plan", "菜单", "菜谱", "配方"},
	{"timeout", "latency", "slow", "超时", "延迟", "缓慢"},
	{"permission", "authorization", "权限", "越权", "授权"},
	{"cache", "redis", "缓存"},
	{"database", "transaction", "数据库", "事务"},
	{"environment", "docker", "infrastructure", "环境", "容器"},
}

const retrievalVersion = "concept-vector-v1+lexical-v1"

func lexical(s string) map[string]float64 {
	out := map[string]float64{}
	for _, word := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		out[word]++
		r := []rune(word)
		for i := 0; i+1 < len(r); i++ {
			if r[i] > 127 {
				out[string(r[i:i+2])]++
			}
		}
	}
	return out
}
func semantic(s string) map[string]float64 {
	out := map[string]float64{}
	s = strings.ToLower(s)
	for i, words := range concepts {
		for _, w := range words {
			if strings.Contains(s, w) {
				out[fmt.Sprint(i)] = 1
				break
			}
		}
	}
	return out
}
func cosine(a, b map[string]float64) float64 {
	var dot, x, y float64
	for k, v := range a {
		dot += v * b[k]
		x += v * v
	}
	for _, v := range b {
		y += v * v
	}
	if x*y == 0 {
		return 0
	}
	return dot / math.Sqrt(x*y)
}

type ExperienceIndex struct {
	Version string    `json:"version"`
	Digest  string    `json:"digest"`
	Records []Failure `json:"records"`
}

func rebuildExperience(root string) error {
	records, err := knowledgeRecords(root)
	if err != nil {
		return err
	}
	active := []Failure{}
	replaced := map[string]bool{}
	for _, f := range records {
		if f.Status != "deleted" && f.Status != "obsolete" && f.Replaces != "" {
			replaced[f.Replaces] = true
		}
	}
	for _, f := range records {
		if f.Status != "deleted" && f.Status != "obsolete" && !replaced[f.ID] {
			active = append(active, f)
		}
	}
	return saveJSON(filepath.Join(root, ".cache/harness/experience/index.json"), ExperienceIndex{retrievalVersion, hashJSON(records), active})
}

type Retrieval struct {
	ID              string   `json:"id"`
	Score           float64  `json:"score"`
	Verified        bool     `json:"verified"`
	Cause           string   `json:"historical_cause"`
	Suggestion      string   `json:"historical_fix"`
	Scope           string   `json:"scope"`
	Citations       []string `json:"citations"`
	VersionMismatch bool     `json:"version_mismatch"`
	Conflicts       []string `json:"conflicts,omitempty"`
}
type QueryRecord struct {
	Task              string      `json:"task"`
	Stage             string      `json:"stage"`
	Query             string      `json:"query"`
	Business          string      `json:"business"`
	CodeVersion       string      `json:"code_version"`
	Category          string      `json:"category,omitempty"`
	ErrorCode         string      `json:"error_code,omitempty"`
	ApplicableVersion string      `json:"applicable_version,omitempty"`
	Test              string      `json:"test,omitempty"`
	VerifiedOnly      bool        `json:"verified_only"`
	RecordedAt        time.Time   `json:"recorded_at"`
	IndexDigest       string      `json:"index_digest"`
	RetrievalVersion  string      `json:"retrieval_version"`
	Results           []Retrieval `json:"results"`
	DurationMS        int64       `json:"duration_ms"`
	Disposition       string      `json:"disposition"`
}

func queryExperience(root string, q QueryRecord) (QueryRecord, error) {
	start := time.Now()
	for _, marker := range []string{"bearer ", "password=", "authorization:", "-----begin", "sk-"} {
		if strings.Contains(strings.ToLower(q.Query+" "+q.Disposition), marker) {
			return q, fmt.Errorf("redact sensitive query before retrieval or generation")
		}
	}
	records, err := knowledgeRecords(root)
	if err != nil {
		return q, err
	}
	var index ExperienceIndex
	if err = readLocalJSON(root, ".cache/harness/experience/index.json", &index); err != nil {
		return q, err
	}
	if index.Digest != hashJSON(records) || index.Version != retrievalVersion {
		return q, fmt.Errorf("stale experience index; rebuild required")
	}
	q.Results = []Retrieval{}
	for _, f := range index.Records {
		if q.Business != "" && !contains(f.Business, q.Business) || q.Category != "" && q.Category != f.Category ||
			q.Test != "" && !contains(f.Tests, q.Test) || q.VerifiedOnly && (!f.Verified || f.Status != "resolved") {
			continue
		}
		if q.ErrorCode != "" && q.ErrorCode != f.ErrorCode {
			continue
		}
		if q.ApplicableVersion != "" && !contains(f.ApplicableVersions, q.ApplicableVersion) {
			continue
		}
		text := f.Symptom + " " + f.Conditions + " " + f.RootCause + " " + f.Fix
		score := 0.55*cosine(lexical(q.Query), lexical(text)) + 0.45*cosine(semantic(q.Query), semantic(text))
		if score < 0.15 {
			continue
		}
		for _, p := range f.Evidence {
			path, e := repositoryFile(root, p)
			if e != nil {
				return q, e
			}
			if _, e = os.Stat(path); e != nil {
				return q, fmt.Errorf("retrieved source evidence unavailable")
			}
		}
		cause, fix := f.RootCause, f.Fix
		repairVerified := f.Verified && f.Status == "resolved"
		if !repairVerified {
			cause = "Unverified hypothesis: " + cause
			fix = "Requires diagnosis; no verified repair"
		}
		q.Results = append(q.Results, Retrieval{f.ID, score, repairVerified, cause, fix, f.Scope, f.Evidence, f.CodeVersion != q.CodeVersion, f.Conflicts})
	}
	sort.Slice(q.Results, func(i, j int) bool {
		if q.Results[i].Score == q.Results[j].Score {
			return q.Results[i].ID < q.Results[j].ID
		}
		return q.Results[i].Score > q.Results[j].Score
	})
	if len(q.Results) > 5 {
		q.Results = q.Results[:5]
	}
	q.IndexDigest = index.Digest
	q.RetrievalVersion = index.Version
	q.RecordedAt = time.Now().UTC()
	q.DurationMS = time.Since(start).Milliseconds()
	return q, nil
}
func contains(a []string, b string) bool {
	for _, v := range a {
		if v == b {
			return true
		}
	}
	return false
}

func captureFailures(root string, r Evidence) error {
	if r.Task == "" {
		return nil
	}
	for _, check := range r.Checks {
		if check.Passed {
			continue
		}
		// Raw subprocess output is intentionally not copied into RAG.
		f := Failure{ID: "run-" + r.RunID[:16] + "-" + fmt.Sprintf("%x", sha256.Sum256([]byte(check.Name)))[:8],
			Revision: 1, Task: r.Task, Category: "unknown", Business: []string{"HARNESS"}, Tests: []string{"harness#" + check.Name},
			CodeVersion: r.SourceDigest, ModelVersion: "not-applicable", Environment: hashJSON(r.Toolchain), Conditions: "harness mode " + r.Mode + " run " + r.RunID,
			Symptom: "Check " + check.Name + " failed; classification requires review", Reproduction: "Repeat named check with same source and environment",
			Scope: "untriaged; no verified cause", Status: "open", Severity: 3, Owner: "task-owner",
			Evidence: []string{".cache/harness/runs/" + r.RunID + ".json"}, RedactionReviewed: true}
		if err := ingestFailure(root, f); err != nil {
			return err
		}
		q, err := queryExperience(root, QueryRecord{Task: r.Task, Stage: "failure", Query: check.Name, Business: "HARNESS", CodeVersion: r.SourceDigest, Disposition: "Automatic retrieval; root cause and adoption require review"})
		if err != nil {
			return err
		}
		if err = saveJSON(filepath.Join(root, ".cache/harness/experience/queries", runID()+".json"), q); err != nil {
			return err
		}
	}
	return nil
}

func experienceCommand(root, action, path, out string) error {
	switch action {
	case "evaluate":
		return evaluateRAG(root, out, true)
	case "index":
		return rebuildExperience(root)
	case "record":
		var f Failure
		if err := readLocalJSON(root, path, &f); err != nil {
			return err
		}
		return ingestFailure(root, f)
	case "query":
		var q QueryRecord
		if err := readLocalJSON(root, path, &q); err != nil {
			return err
		}
		if !taskName.MatchString(q.Task) || q.Query == "" || q.Disposition == "" || !contains([]string{"start", "failure", "repair"}, q.Stage) {
			return fmt.Errorf("query requires task, stage and disposition")
		}
		q, err := queryExperience(root, q)
		if err != nil {
			return err
		}
		if err = saveJSON(filepath.Join(root, ".cache/harness/experience/queries", runID()+".json"), q); err != nil {
			return err
		}
		return saveJSON(out, q)
	case "metrics":
		m, err := learningMetrics(root)
		if err != nil {
			return err
		}
		return saveJSON(out, m)
	default:
		return fmt.Errorf("experience action: index, record, query or metrics")
	}
}

// Metrics retain all runs, including failed runs. No task association is inferred
// from timestamps: old unlinked reports remain unknown.
func learningMetrics(root string) (any, error) {
	files, _ := filepath.Glob(filepath.Join(root, ".cache/harness/runs/*.json"))
	byTask := map[string][]Evidence{}
	groups := map[string]map[string]bool{}
	total, unknown, duration := 0, 0, int64(0)
	for _, p := range files {
		var e Evidence
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &e); err != nil {
			return nil, err
		}
		total++
		if e.Task == "" {
			unknown++
		} else {
			byTask[e.Task] = append(byTask[e.Task], e)
		}
		env := hashJSON(e.Toolchain)
		for _, c := range e.Checks {
			duration += c.DurationMS
		}
		for _, t := range e.Tests {
			key := e.SourceDigest + "|" + env + "|" + t.Project + "|" + t.ID
			if groups[key] == nil {
				groups[key] = map[string]bool{}
			}
			groups[key][t.Status] = true
		}
	}
	firstPass, rework, eligible := 0, 0, 0
	for _, runs := range byTask {
		sort.Slice(runs, func(i, j int) bool { return runs[i].StartedAt.Before(runs[j].StartedAt) })
		seenMode := map[string]bool{}
		firstOK := true
		for _, r := range runs {
			if r.Mode != "full" && r.Mode != "acceptance" {
				continue
			}
			ok := len(r.Checks) > 0
			for _, c := range r.Checks {
				ok = ok && c.Passed
			}
			if !seenMode[r.Mode] {
				firstOK = firstOK && ok
				seenMode[r.Mode] = true
			} else {
				rework++
			}
		}
		if seenMode["full"] && seenMode["acceptance"] {
			eligible++
			if firstOK {
				firstPass++
			}
		}
	}
	inconsistent := 0
	for _, s := range groups {
		if s["pass"] && s["fail"] {
			inconsistent++
		}
	}
	records, err := knowledgeRecords(root)
	if err != nil {
		return nil, err
	}
	priority := append([]Failure{}, records...)
	recurrences, resolved := 0, 0
	for _, f := range records {
		if f.RecurrenceOf != "" {
			recurrences++
		}
		if f.Status == "resolved" {
			resolved++
		}
	}
	sort.Slice(priority, func(i, j int) bool { return priority[i].Severity > priority[j].Severity })
	type item struct {
		ID       string `json:"id"`
		Severity int    `json:"severity"`
		Owner    string `json:"owner"`
		Status   string `json:"status"`
	}
	top := []item{}
	for _, f := range priority {
		if f.Status == "open" {
			top = append(top, item{f.ID, f.Severity, f.Owner, f.Status})
		}
	}
	evaluationFiles, _ := filepath.Glob(filepath.Join(root, ".cache/harness/evaluations/*.json"))
	evaluations := []map[string]any{}
	for _, p := range evaluationFiles {
		raw, e := os.ReadFile(p)
		if e != nil {
			return nil, e
		}
		var v map[string]any
		if e = json.Unmarshal(raw, &v); e != nil {
			return nil, e
		}
		evaluations = append(evaluations, map[string]any{"domain": v["domain"], "samples": v["samples"], "attempted": v["attempted"], "correct": v["correct"],
			"service_failures": v["service_failures"], "p95_ms": v["p95_ms"], "model": v["model"], "source_digest": v["source_digest"], "corpus_digest": v["corpus_digest"], "cost_status": v["cost_status"], "cost": v["cost"]})
	}
	rag, err := ragHistory(root)
	if err != nil {
		return nil, err
	}
	return map[string]any{"version": 1, "recorded_at": time.Now().UTC(), "runs": total, "unlinked_runs": unknown,
		"first_pass": map[string]int{"numerator": firstPass, "denominator": eligible}, "additional_full_acceptance_runs": rework,
		"inconsistent_test_groups": inconsistent, "observed_test_groups": len(groups), "verification_duration_ms": duration,
		"recurrence":           map[string]int{"linked_recurrences": recurrences, "recorded_failures": len(records), "resolved_failures": resolved},
		"recurrence_note":      "Linked recurrence / recorded failures is observed recurrence share, not a production incident rate. Missing historical links remain unknown.",
		"business_evaluations": evaluations,
		"rag":                  rag,
		"improvement_backlog":  top, "review_period": "weekly; manual review, no scheduler installed",
		"interpretation": "First pass requires both full and acceptance first attempts. Additional runs are a rework proxy, not human edits. Inconsistent statuses are candidates for investigation, not proven flaky tests. Old unlinked records are unknown."}, nil
}
