package main

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/passforge/passforge/internal/core"
	"github.com/spf13/cobra"
)

// newRootCmd builds the full CLI command tree (mirrors main()).
func newRootCmd() *cobra.Command {
	var jsonOutput bool
	rootCmd := &cobra.Command{
		Use:   "passforge",
		Short: "Password generator, strength checker, and suggestion engine",
	}

	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output in JSON format")

	rootCmd.AddCommand(generateCmd(&jsonOutput))
	rootCmd.AddCommand(passphraseCmd(&jsonOutput))
	rootCmd.AddCommand(checkCmd(&jsonOutput))
	rootCmd.AddCommand(suggestCmd(&jsonOutput))
	rootCmd.AddCommand(rotateCmd(&jsonOutput))
	rootCmd.AddCommand(bulkCmd(&jsonOutput))

	return rootCmd
}

// captureOutput runs a cobra command with args and captures stdout.
func captureOutput(t *testing.T, cmd *cobra.Command, args []string) string {
	t.Helper()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	execErr := cmd.Execute()

	w.Close()
	os.Stdout = old

	var pipeBuf bytes.Buffer
	pipeBuf.ReadFrom(r)

	combined := buf.String() + pipeBuf.String()

	if execErr != nil {
		t.Fatalf("command %v failed: %v\noutput: %s", args, execErr, combined)
	}

	return combined
}

func TestRotateAlias_SSBD(t *testing.T) {
	rotateOut := captureOutput(t, newRootCmd(), []string{"rotate", "p@sSwor4", "--count", "3"})
	ssbdOut := captureOutput(t, newRootCmd(), []string{"ssbd", "p@sSwor4", "--count", "3"})

	rotateLines := strings.Split(strings.TrimSpace(rotateOut), "\n")
	ssbdLines := strings.Split(strings.TrimSpace(ssbdOut), "\n")

	if len(rotateLines) != 3 {
		t.Errorf("rotate: expected 3 lines, got %d: %q", len(rotateLines), rotateOut)
	}
	if len(ssbdLines) != 3 {
		t.Errorf("ssbd: expected 3 lines, got %d: %q", len(ssbdLines), ssbdOut)
	}

	for i, line := range ssbdLines {
		if !strings.Contains(line, ":") {
			t.Errorf("ssbd line %d missing colon-separated format: %q", i, line)
		}
	}
}

func TestRotateAlias_SSBD_JSON(t *testing.T) {
	out := captureOutput(t, newRootCmd(), []string{"ssbd", "--json", "p@sSwor4", "--count", "2"})

	if !strings.Contains(out, `"base"`) {
		t.Errorf("ssbd --json: expected 'base' key in output: %q", out)
	}
	if !strings.Contains(out, `"variants"`) {
		t.Errorf("ssbd --json: expected 'variants' key in output: %q", out)
	}
}

func TestRotateCmd_MinMaxLengthFlags(t *testing.T) {
	out := captureOutput(t, newRootCmd(), []string{
		"rotate", "p@sSwor4", "--count", "5", "--min-length", "8", "--max-length", "11",
	})

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d: %q", len(lines), out)
	}

	for i, line := range lines {
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) != 2 {
			continue
		}
		variant := parts[1]
		vLen := len([]rune(variant))
		if vLen < 8 || vLen > 11 {
			t.Errorf("line %d variant %q length %d outside [8, 11]", i, variant, vLen)
		}
	}
}

func TestRotateCmd_StrictLengthFlag(t *testing.T) {
	base := "p@sSwor4"
	baseLen := len([]rune(base))
	out := captureOutput(t, newRootCmd(), []string{
		"rotate", base, "--count", "5", "--strict-length",
	})

	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i, line := range lines {
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) != 2 {
			continue
		}
		variant := parts[1]
		if len([]rune(variant)) != baseLen {
			t.Errorf("line %d variant %q length %d != base %d with --strict-length", i, variant, len([]rune(variant)), baseLen)
		}
	}
}

func TestCheckCmd_ExitCodes(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetOut(new(bytes.Buffer)) // suppress output
	cmd.SetErr(new(bytes.Buffer))
	// "123" guarantees a score of under 10 (well below 40 threshold)
	cmd.SetArgs([]string{"check", "123"})
	err := cmd.Execute()
	if !errors.Is(err, core.ErrWeak) {
		t.Errorf("expected ErrWeak, got %v", err)
	}

	cmd = newRootCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"check", "SuperSecretPassword123!@#$"})
	err = cmd.Execute()
	if err != nil {
		t.Errorf("expected nil error for strong password, got %v", err)
	}
}
