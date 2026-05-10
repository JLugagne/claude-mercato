package commands

import (
	"strings"
	"testing"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain/service"
)

func TestDoctorCmd_Clean(t *testing.T) {
	svc := mockServices()
	out, err := runCmd(t, svc, "doctor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "everything looks healthy") {
		t.Errorf("expected healthy line, got: %s", out)
	}
}

func TestDoctorCmd_FindingsOutput(t *testing.T) {
	svc := mockServices()
	svc.Doctor = &stubDoctor{
		doctorFn: func(opts service.DoctorOpts) (service.DoctorReport, error) {
			return service.DoctorReport{
				StaleLocations: []service.DoctorLocation{
					{Market: "m", Profile: "p", Location: "/dead"},
				},
				ModifiedFiles: []service.DoctorFile{
					{Market: "m", Profile: "p", Location: "/proj", Path: ".claude/agents/x.md"},
				},
			}, nil
		},
	}
	out, err := runCmd(t, svc, "doctor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"Stale install locations", "/dead", "Locally-modified files", "x.md"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got: %s", want, out)
		}
	}
}

func TestDoctorCmd_JSON(t *testing.T) {
	svc := mockServices()
	svc.Doctor = &stubDoctor{
		doctorFn: func(opts service.DoctorOpts) (service.DoctorReport, error) {
			return service.DoctorReport{
				LocallyDeleted: []service.DoctorFile{
					{Market: "m", Profile: "p", Location: "/proj", Path: ".claude/agents/x.md"},
				},
			}, nil
		},
	}
	out, err := runCmd(t, svc, "doctor", "--json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mustValidJSON(t, out)
	if !strings.Contains(out, "locally_deleted") {
		t.Errorf("expected locally_deleted key in JSON, got: %s", out)
	}
}

func TestDoctorCmd_MarketFilterPropagated(t *testing.T) {
	var captured service.DoctorOpts
	svc := mockServices()
	svc.Doctor = &stubDoctor{
		doctorFn: func(opts service.DoctorOpts) (service.DoctorReport, error) {
			captured = opts
			return service.DoctorReport{}, nil
		},
	}
	if _, err := runCmd(t, svc, "doctor", "--market", "mkt"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.Market != "mkt" {
		t.Errorf("expected --market to be propagated, got %q", captured.Market)
	}
}
