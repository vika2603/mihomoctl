package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// Tests below lock the v1.0 low-impact mutation contract for `cache clear`
// (ADR-0014 §4.1). The contract is:
//   - --dry-run is rejected with an actionable usage error (exit 64)
//   - --yes is accepted as a no-op for flag uniformity with higher-tier
//     mutations
//   - JSON success output carries `risk.level: "low"`

func TestCacheClearRejectsDryRunWithActionableMessage(t *testing.T) {
	for _, target := range []string{"fakeip", "dns", "all"} {
		target := target
		t.Run(target, func(t *testing.T) {
			err := run([]string{"cache", "clear", target, "--dry-run"}, &bytes.Buffer{})
			assertCLIError(t, err, exitUsage, "does not support --dry-run")
			assertCLIError(t, err, exitUsage, "low-impact mutation")
			// Lock the cache-specific verb-phrase so future helper
			// genericization cannot silently drift the wording (Iris
			// regression catch on PR #7 04bd924).
			assertCLIError(t, err, exitUsage, "Drop --dry-run to flush the cache")
		})
	}
}

func TestCacheClearYesFlagIsNoOpInTTYAndAutomation(t *testing.T) {
	srv := fakeMihomoWith(t, fakeOptions{onCacheFlush: func(path string) {}})

	// --yes alone (success path)
	var out bytes.Buffer
	if err := run([]string{"--endpoint", srv.URL, "cache", "clear", "dns", "--yes"}, &out); err != nil {
		t.Fatalf("cache clear dns --yes failed: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "flushed mihomo dns cache") {
		t.Fatalf("--yes human output unexpected: %q", got)
	}

	// --yes with --json must still emit risk.level=low
	out.Reset()
	if err := run([]string{"--endpoint", srv.URL, "cache", "clear", "dns", "--yes", "--json"}, &out); err != nil {
		t.Fatalf("cache clear dns --yes --json failed: %v", err)
	}
	var got cacheOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if got.Risk == nil || got.Risk.Level != "low" {
		t.Fatalf("expected risk.level=low; got %+v", got.Risk)
	}
}

func TestCacheClearJSONIncludesLowRiskLevel(t *testing.T) {
	srv := fakeMihomoWith(t, fakeOptions{onCacheFlush: func(path string) {}})

	cases := []struct {
		target string
	}{{"fakeip"}, {"dns"}, {"all"}}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.target, func(t *testing.T) {
			var out bytes.Buffer
			if err := run([]string{"--endpoint", srv.URL, "cache", "clear", tc.target, "--json"}, &out); err != nil {
				t.Fatalf("cache clear %s --json failed: %v", tc.target, err)
			}
			var got cacheOutput
			if err := json.Unmarshal(out.Bytes(), &got); err != nil {
				t.Fatalf("invalid JSON: %v\n%s", err, out.String())
			}
			if got.Risk == nil {
				t.Fatalf("missing risk envelope: %+v", got)
			}
			if got.Risk.Level != "low" {
				t.Fatalf("risk.level=%q, want %q", got.Risk.Level, "low")
			}
			if got.Risk.Summary == "" {
				t.Fatalf("risk.summary must be non-empty for low-impact contract: %+v", got.Risk)
			}
		})
	}
}

// Locked: a partial-failure error envelope (tempfail) does not include risk
// fields; risk is only emitted on success per ADR-0014 §5 (mutation succeeded
// → callers know what they actually changed).
func TestCacheClearPartialFailureDoesNotEmitRisk(t *testing.T) {
	partial := fakeMihomoWith(t, fakeOptions{dnsFlushCode: 500})
	var out, errOut bytes.Buffer
	code := Run([]string{"--endpoint", partial.URL, "--json", "cache", "clear", "all"}, &out, &errOut)
	if code != exitTempFail {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, exitTempFail, errOut.String())
	}
	if out.Len() != 0 {
		t.Fatalf("partial failure should not emit success stdout: %s", out.String())
	}
	if strings.Contains(errOut.String(), `"risk"`) {
		t.Fatalf("partial failure must not surface risk envelope: %s", errOut.String())
	}
}
