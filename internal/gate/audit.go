package gate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// AuditSpec identifies the spec of a run.
type AuditSpec struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// AuditCheck is one check in the audit record.
type AuditCheck struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Ms     int64  `json:"ms"`
}

// AuditRecord is one line of .factory/audit.jsonl.
type AuditRecord struct {
	RunID       string        `json:"run_id"`
	TS          string        `json:"ts"`
	Kind        string        `json:"kind"` // gate_run or approval
	RequestedBy string        `json:"requested_by,omitempty"`
	ApprovedBy  string        `json:"approved_by,omitempty"`
	Agents      []AgentRecord `json:"agents,omitempty"`
	Spec        *AuditSpec    `json:"spec,omitempty"`
	Base        string        `json:"base,omitempty"`
	Head        string        `json:"head,omitempty"`
	Attempt     int           `json:"attempt,omitempty"`
	Files       []string      `json:"files,omitempty"`
	Checks      []AuditCheck  `json:"checks,omitempty"`
	Verdict     string        `json:"verdict,omitempty"`
	Mode        string        `json:"mode,omitempty"`
	Source      string        `json:"source,omitempty"`
	GateVersion string        `json:"gate_version,omitempty"`
}

// AuditFromEvidence builds the gate_run record of a run.
func AuditFromEvidence(ev *Evidence, now time.Time) AuditRecord {
	p := ev.Provenance
	r := AuditRecord{
		RunID: ev.RunID, TS: now.UTC().Format(time.RFC3339), Kind: "gate_run",
		RequestedBy: p.RequestedBy, Agents: p.Agents, Base: p.Base, Head: p.Head,
		Attempt: p.Attempt, Verdict: ev.Verdict, Mode: p.Mode, Source: p.Source, GateVersion: p.GateVersion,
		Files: []string{},
	}
	if p.SpecPath != "" {
		r.Spec = &AuditSpec{Path: p.SpecPath, SHA256: p.SpecSHA256}
	}
	for _, f := range ev.Files {
		r.Files = append(r.Files, f.Path)
	}
	for _, c := range ev.Checks {
		r.Checks = append(r.Checks, AuditCheck{ID: c.ID, Status: c.Status, Ms: c.Ms})
	}
	return r
}

// AppendAudit appends one record to the audit file.
func AppendAudit(path string, r AuditRecord) error {
	line, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return fmt.Errorf("audit: %w", err)
	}
	return f.Close()
}
