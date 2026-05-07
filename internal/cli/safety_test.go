package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/the-super-company/mihomoctl/internal/render"
)

// Tests below lock the ADR-0014 §4.2 medium-tier confirmation wire contract
// and the §4.1 dry-run rejection contract, plus the JSON shape of riskInfo.
// Every mutation command in v1.0 should call into these helpers; if these
// tests change, the commands' behavior changes too, so reviewers see it
// surface here.

func TestConfirmMediumImpactRejectsNonTTYWithoutYes(t *testing.T) {
	in := confirmInput{
		reader:    strings.NewReader(""),
		promptOut: &bytes.Buffer{},
		isTTY:     false,
	}
	err := confirmMediumImpact(in, mediumConfirmOptions{
		target:  "proxy unfix HK",
		summary: "Unfix the HK proxy group fixed selection.",
	})
	if err == nil {
		t.Fatalf("expected mutation_aborted error, got nil")
	}
	assertRenderError(t, err, exitUsage, errCodeMutationAborted)
	if !strings.Contains(err.Error(), "non-interactive") {
		t.Fatalf("error should reference non-interactive constraint: %v", err)
	}
}

func TestConfirmMediumImpactYesShortCircuitsTTYAndNonTTY(t *testing.T) {
	for _, isTTY := range []bool{true, false} {
		isTTY := isTTY
		t.Run(label(isTTY), func(t *testing.T) {
			promptOut := &bytes.Buffer{}
			in := confirmInput{
				reader:    strings.NewReader("n\n"), // would decline if read
				promptOut: promptOut,
				isTTY:     isTTY,
			}
			err := confirmMediumImpact(in, mediumConfirmOptions{
				target:  "connections close c-1234",
				summary: "Close connection c-1234 to example.com:443.",
				yes:     true,
			})
			if err != nil {
				t.Fatalf("--yes must short-circuit: %v", err)
			}
			if promptOut.Len() != 0 {
				t.Fatalf("--yes must not write a prompt: %q", promptOut.String())
			}
		})
	}
}

func TestConfirmMediumImpactTTYAcceptsYAndYesCaseInsensitive(t *testing.T) {
	for _, answer := range []string{"y", "Y", "yes", "YES", "Yes  "} {
		answer := answer
		t.Run(answer, func(t *testing.T) {
			in := confirmInput{
				reader:    strings.NewReader(answer + "\n"),
				promptOut: &bytes.Buffer{},
				isTTY:     true,
			}
			err := confirmMediumImpact(in, mediumConfirmOptions{
				target:  "rule disable r-1",
				summary: "Disable rule r-1.",
			})
			if err != nil {
				t.Fatalf("answer %q must accept: %v", answer, err)
			}
		})
	}
}

func TestConfirmMediumImpactTTYDeclinesAnythingElse(t *testing.T) {
	for _, answer := range []string{"", "n", "no", "x", "later", "  "} {
		answer := answer
		t.Run("answer="+answer, func(t *testing.T) {
			in := confirmInput{
				reader:    strings.NewReader(answer + "\n"),
				promptOut: &bytes.Buffer{},
				isTTY:     true,
			}
			err := confirmMediumImpact(in, mediumConfirmOptions{
				target:  "rule enable r-2",
				summary: "Enable rule r-2.",
			})
			if err == nil {
				t.Fatalf("answer %q must decline", answer)
			}
			assertRenderError(t, err, exitUsage, errCodeMutationAborted)
		})
	}
}

func TestConfirmMediumImpactTTYEOFAborts(t *testing.T) {
	in := confirmInput{
		reader:    strings.NewReader(""), // immediate EOF
		promptOut: &bytes.Buffer{},
		isTTY:     true,
	}
	err := confirmMediumImpact(in, mediumConfirmOptions{
		target:  "proxy-providers update upstream",
		summary: "Refetch upstream provider.",
	})
	if err == nil {
		t.Fatalf("EOF must produce mutation_aborted")
	}
	assertRenderError(t, err, exitUsage, errCodeMutationAborted)
	if !strings.Contains(err.Error(), "no response on stdin") {
		t.Fatalf("EOF reason should be specific: %v", err)
	}
}

func TestConfirmMediumImpactRejectsEmptyMetadata(t *testing.T) {
	cases := []mediumConfirmOptions{
		{target: "", summary: "x"},
		{target: "x", summary: ""},
	}
	for _, opts := range cases {
		opts := opts
		t.Run(opts.target+"|"+opts.summary, func(t *testing.T) {
			err := confirmMediumImpact(confirmInput{isTTY: true, reader: strings.NewReader("y\n"), promptOut: &bytes.Buffer{}}, opts)
			if err == nil {
				t.Fatalf("missing metadata must produce a programmer error, not silent success")
			}
		})
	}
}

func TestDryRunUnsupportedErrorIsUsageCategory(t *testing.T) {
	err := dryRunUnsupportedError("cache clear dns")
	assertRenderError(t, err, exitUsage, "")
	if !strings.Contains(err.Error(), "low-impact mutation") {
		t.Fatalf("dry-run reject must explain why: %v", err)
	}
	if !strings.Contains(err.Error(), "Drop --dry-run") {
		t.Fatalf("dry-run reject must give actionable next step: %v", err)
	}
}

func TestRiskHelpersReturnStableLevels(t *testing.T) {
	if r := lowRisk("s"); r.Level != riskLevelLow {
		t.Fatalf("lowRisk level = %q, want %q", r.Level, riskLevelLow)
	}
	if r := mediumRisk("s"); r.Level != riskLevelMedium {
		t.Fatalf("mediumRisk level = %q, want %q", r.Level, riskLevelMedium)
	}
	if r := highRisk("s"); r.Level != riskLevelHigh {
		t.Fatalf("highRisk level = %q, want %q", r.Level, riskLevelHigh)
	}
}

func TestPromptRendersSummaryAndProceedQuestion(t *testing.T) {
	promptOut := &bytes.Buffer{}
	in := confirmInput{
		reader:    strings.NewReader("y\n"),
		promptOut: promptOut,
		isTTY:     true,
	}
	if err := confirmMediumImpact(in, mediumConfirmOptions{
		target:  "rules disable r-99",
		summary: "Disable rule r-99 until controller restart.",
	}); err != nil {
		t.Fatalf("y answer rejected: %v", err)
	}
	got := promptOut.String()
	if !strings.Contains(got, "Disable rule r-99 until controller restart.") {
		t.Fatalf("summary missing: %q", got)
	}
	if !strings.Contains(got, "Proceed? [y/N]:") {
		t.Fatalf("prompt question missing: %q", got)
	}
}

// assertRenderError asserts that err is a render.Error with the given exit
// code and (optionally) machine code. If wantCode is empty, only the exit
// code is checked.
func assertRenderError(t *testing.T, err error, wantExit int, wantCode string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var re *render.Error
	if !errors.As(err, &re) {
		t.Fatalf("error is not *render.Error: %T %v", err, err)
	}
	if re.Code != wantExit {
		t.Fatalf("exit code = %d, want %d (err=%v)", re.Code, wantExit, err)
	}
	if wantCode != "" && re.ErrorCode != wantCode {
		t.Fatalf("machine code = %q, want %q (err=%v)", re.ErrorCode, wantCode, err)
	}
}

func label(isTTY bool) string {
	if isTTY {
		return "tty"
	}
	return "non-tty"
}
