package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/x/term"
	"github.com/the-super-company/mihomoctl/internal/render"
)

// Mutation safety helpers shared by every mihomoctl mutation command per
// ADR-0014 v1.0 mutation safety matrix (canonical SHA 923240ba).
//
// Tiers: low (cache clear today), medium (proxy unfix / connections close <id> /
// proxy-providers update / rule-providers update / rules disable+enable /
// system upgrade ui / config set), high (config reload / config patch --file /
// service restart / connections close all / system upgrade core / geo update),
// debug-only (no CLI surface). Each tier's wire-format contract lives in this
// file so commands cannot diverge.

// riskTier names the mutation classification embedded in JSON success output.
// Keep these strings stable: external automation pipelines key off them.
const (
	riskLevelLow    = "low"
	riskLevelMedium = "medium"
	riskLevelHigh   = "high"
)

// errCodeMutationAborted is the ADR-0010 machine code surfaced when a
// medium- or high-tier mutation refuses to proceed because confirmation was
// declined or unavailable. The category is "usage" (sysexits 64) per
// ADR-0014 §4.2.
const errCodeMutationAborted = "mutation_aborted"

// dryRunMode classifies how a command's --dry-run is honored. ADR-0014 §4.3
// requires high-tier mutations to declare native or client_simulated; §4.1
// requires low-tier mutations to reject --dry-run with an actionable error.
type dryRunMode int

const (
	// dryRunUnsupported: the command does not honor --dry-run. Low-impact
	// surfaces such as cache clear declare this so callers cannot believe a
	// preview happened.
	dryRunUnsupported dryRunMode = iota
	// dryRunNative: the upstream controller exposes a real preview path
	// (e.g. configuration validation) that the CLI proxies. JSON output
	// must mark dry_run_mode: "native".
	dryRunNative
	// dryRunClientSimulated: the CLI computes the would-be diff locally
	// without touching the controller. JSON output must mark
	// dry_run_mode: "client_simulated".
	dryRunClientSimulated
)

// mutationAbortedError builds the standard usage-class error returned when a
// confirmation gate refuses to proceed. exit code 64 / category usage so
// scripts can distinguish a refusal from a controller-side failure.
func mutationAbortedError(target, reason string) error {
	return render.NewError(exitUsage,
		fmt.Sprintf("%s aborted: %s", target, reason),
		errCodeMutationAborted,
		map[string]any{"target": target, "reason": reason},
	)
}

// dryRunUnsupportedError is the low-tier rejection path declared by
// ADR-0014 §4.1. It is exit 64 / category usage and carries an actionable
// hint so callers know to drop the flag.
//
// action is a short verb-phrase describing what the command actually does
// when --dry-run is dropped ("flush the cache", "close the connection",
// "refetch the provider"). It must not be empty: a generic "to proceed"
// loses clarity for callers diagnosing scripts, and the per-command
// regression suite asserts the exact verb (Iris regression catch
// 04bd924 → next-HEAD).
func dryRunUnsupportedError(target, action string) error {
	if action == "" {
		// Programmer error — fail loud rather than silently emit a vague
		// "Drop --dry-run to proceed." style message.
		return fmt.Errorf("dryRunUnsupportedError: action verb-phrase required for %s", target)
	}
	return usage("%s does not support --dry-run; it is a low-impact mutation that runs immediately. Drop --dry-run to %s.", target, action)
}

// confirmInput is the side-channel a medium-tier confirmation reads from.
// Tests inject a strings.Reader; production wires os.Stdin.
type confirmInput struct {
	// reader is the stdin source for the y/N prompt.
	reader io.Reader
	// promptOut is where the prompt text is written. Use Stderr in
	// production so prompts do not pollute --json stdout.
	promptOut io.Writer
	// isTTY reports whether the input is interactive. When false, callers
	// must supply --yes or the helper returns mutationAborted.
	isTTY bool
}

// stdinConfirmInput returns a confirmInput sourced from os.Stdin / os.Stderr
// with TTY detection via charmbracelet/x/term so the medium-tier prompt path
// matches the existing connections watch / cache clear surfaces.
func stdinConfirmInput() confirmInput {
	return confirmInput{
		reader:    os.Stdin,
		promptOut: os.Stderr,
		isTTY:     term.IsTerminal(uintptr(os.Stdin.Fd())),
	}
}

// mediumConfirmOptions captures the per-command knobs that vary across
// medium-tier mutations.
type mediumConfirmOptions struct {
	// target is the user-facing label rendered in the prompt and any
	// abort/confirm error envelope ("proxy unfix Proxy", "connections close c-1234").
	target string
	// summary is one short sentence rendered above the y/N prompt so the
	// operator sees what is about to change. Required.
	summary string
	// yes is the value of the command's --yes flag.
	yes bool
}

// confirmMediumImpact implements the ADR-0014 §4.2 wire contract for every
// medium-tier mutation:
//
//   - --yes always wins (TTY or non-TTY).
//   - non-TTY without --yes returns mutationAborted (exit 64) so CI scripts
//     never block waiting on a prompt that no operator can answer.
//   - TTY without --yes prompts "Proceed? [y/N]:" and treats anything other
//     than a case-insensitive y/yes as decline.
//
// The helper does not log the secret material; callers must redact upstream
// before passing summary/target.
func confirmMediumImpact(in confirmInput, opts mediumConfirmOptions) error {
	if opts.target == "" {
		return fmt.Errorf("confirmMediumImpact: target required")
	}
	if opts.summary == "" {
		return fmt.Errorf("confirmMediumImpact: summary required")
	}
	if opts.yes {
		return nil
	}
	if !in.isTTY {
		return mutationAbortedError(opts.target,
			"non-interactive session requires --yes for medium-impact mutations (ADR-0014 §4.2)")
	}
	if in.reader == nil || in.promptOut == nil {
		return mutationAbortedError(opts.target,
			"interactive session has no usable stdin or stderr; pass --yes to bypass")
	}
	fmt.Fprintf(in.promptOut, "%s\nProceed? [y/N]: ", opts.summary)
	scanner := bufio.NewScanner(in.reader)
	if !scanner.Scan() {
		return mutationAbortedError(opts.target,
			"no response on stdin (EOF before answer); pass --yes for non-interactive use")
	}
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if answer == "y" || answer == "yes" {
		return nil
	}
	return mutationAbortedError(opts.target, "user declined confirmation")
}

// highConfirmOptions captures the per-command knobs that vary across
// high-tier mutations. ADR-0014 §4.3 requires literal-token confirmation
// (the operator must retype the resource name, not just answer "yes") so
// dangerous and ambiguous batch operations cannot be triggered by a
// confused keystroke.
type highConfirmOptions struct {
	// target is the user-facing label for prompt/error envelopes
	// ("connections close all", "config reload", "geo update").
	target string
	// summary is one short sentence rendered above the prompt so the
	// operator sees the blast radius. Required.
	summary string
	// expectedToken is the literal string the operator must retype. It
	// must be specific to the resource (e.g. the literal config name, the
	// word "all" for a fleet operation, the file path for a patch). The
	// generic word "yes" or "y" must not be used: ADR-0014 §4.3 requires
	// the token to disambiguate from medium-tier prompts.
	expectedToken string
	// yes is the value of the command's --yes flag. For high-tier
	// mutations --yes alone is not sufficient: a non-interactive session
	// must also supply --confirm <token>. yes alone bypasses only the
	// interactive prompt path; the token is still validated.
	yes bool
	// confirmToken is the value supplied via --confirm <token>. When the
	// session is non-interactive (or when yes is set), this must equal
	// expectedToken. When empty in a TTY without --yes, the helper
	// prompts the operator to retype the token interactively.
	confirmToken string
}

// confirmHighImpact implements the ADR-0014 §4.3 wire contract for every
// high-tier mutation:
//
//   - non-TTY: requires --confirm <token> matching expectedToken; --yes
//     alone is rejected with mutationAborted because automation that
//     intends a high-impact change must transmit the literal resource
//     name on the command line, not "yes".
//   - TTY without --yes and without --confirm: prompts the operator to
//     retype the token. Anything other than an exact match (case-
//     sensitive) declines.
//   - TTY with --yes: requires --confirm <token> matching expectedToken;
//     yes is treated as "I waived the prompt, but the token still has
//     to match".
//
// The helper does not log the secret material; callers must redact
// upstream before passing summary/target/expectedToken.
func confirmHighImpact(in confirmInput, opts highConfirmOptions) error {
	if opts.target == "" {
		return fmt.Errorf("confirmHighImpact: target required")
	}
	if opts.summary == "" {
		return fmt.Errorf("confirmHighImpact: summary required")
	}
	if opts.expectedToken == "" {
		return fmt.Errorf("confirmHighImpact: expectedToken required")
	}
	switch strings.ToLower(opts.expectedToken) {
	case "yes", "y":
		return fmt.Errorf("confirmHighImpact: expectedToken must be specific to the resource, not %q", opts.expectedToken)
	}

	// Non-interactive (CI / piped automation): --confirm <token> is
	// mandatory. --yes alone is rejected.
	if !in.isTTY {
		if opts.confirmToken == "" {
			return mutationAbortedError(opts.target,
				fmt.Sprintf("non-interactive high-impact mutation requires --confirm %q (ADR-0014 §4.3)", opts.expectedToken))
		}
		if opts.confirmToken != opts.expectedToken {
			return mutationAbortedError(opts.target,
				fmt.Sprintf("--confirm token %q does not match the expected literal %q", opts.confirmToken, opts.expectedToken))
		}
		return nil
	}

	// Interactive session.
	if opts.confirmToken != "" {
		// Operator passed --confirm explicitly; still validate to keep
		// behavior consistent with non-TTY callers.
		if opts.confirmToken != opts.expectedToken {
			return mutationAbortedError(opts.target,
				fmt.Sprintf("--confirm token %q does not match the expected literal %q", opts.confirmToken, opts.expectedToken))
		}
		return nil
	}
	if opts.yes {
		// --yes without --confirm in TTY: ADR-0014 §4.3 still requires
		// the token. Surface a specific message so the operator can fix
		// the invocation rather than guessing.
		return mutationAbortedError(opts.target,
			fmt.Sprintf("--yes alone is not enough for high-impact mutations; also pass --confirm %q (ADR-0014 §4.3)", opts.expectedToken))
	}
	if in.reader == nil || in.promptOut == nil {
		return mutationAbortedError(opts.target,
			"interactive session has no usable stdin or stderr; pass --confirm <token> to bypass the prompt")
	}

	fmt.Fprintf(in.promptOut, "%s\nThis is a HIGH-impact mutation. To proceed, retype: %s\n> ", opts.summary, opts.expectedToken)
	scanner := bufio.NewScanner(in.reader)
	if !scanner.Scan() {
		return mutationAbortedError(opts.target,
			"no response on stdin (EOF before token); pass --confirm <token> for non-interactive use")
	}
	typed := strings.TrimSpace(scanner.Text())
	if typed != opts.expectedToken {
		return mutationAbortedError(opts.target,
			fmt.Sprintf("typed token %q does not match the expected literal %q", typed, opts.expectedToken))
	}
	return nil
}

// riskInfo is the JSON shape mandated by ADR-0014 §5 for every successful
// mutation. Callers (especially CI scripts and audit pipelines) inspect
// risk.level to decide whether the action requires elevated approval logging.
//
// Levels are stable strings: "low", "medium", "high". Summary is a short
// human-language sentence that should not contain secret material.
type riskInfo struct {
	Level   string `json:"level"`
	Summary string `json:"summary,omitempty"`
}

func lowRisk(summary string) *riskInfo {
	return &riskInfo{Level: riskLevelLow, Summary: summary}
}

func mediumRisk(summary string) *riskInfo {
	return &riskInfo{Level: riskLevelMedium, Summary: summary}
}

func highRisk(summary string) *riskInfo {
	return &riskInfo{Level: riskLevelHigh, Summary: summary}
}
