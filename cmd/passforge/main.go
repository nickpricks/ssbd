package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/passforge/passforge/internal/core"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func main() {
	var jsonOutput bool

	rootCmd := &cobra.Command{
		Use:           "passforge",
		Short:         "Password generator, strength checker, and suggestion engine",
		Long:          "PassForge generates strong passwords, scores their strength, and provides actionable suggestions to improve weak ones.",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output in JSON format")

	rootCmd.AddCommand(generateCmd(&jsonOutput))
	rootCmd.AddCommand(passphraseCmd(&jsonOutput))
	rootCmd.AddCommand(checkCmd(&jsonOutput))
	rootCmd.AddCommand(suggestCmd(&jsonOutput))
	rootCmd.AddCommand(rotateCmd(&jsonOutput))
	rootCmd.AddCommand(bulkCmd(&jsonOutput))

	if err := rootCmd.Execute(); err != nil {
		if errors.Is(err, core.ErrBreached) {
			os.Exit(2)
		} else if errors.Is(err, core.ErrWeak) {
			os.Exit(1)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(3) // Operational error
		}
	}
}

func getPassword(args []string) (string, error) {
	if len(args) == 1 && args[0] != "-" {
		return args[0], nil
	}

	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Piped from stdin
		bytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf(core.MsgErrReadingStdin, err)
		}
		return strings.TrimSpace(string(bytes)), nil
	}

	// Prompt interactively with hidden echo
	fmt.Fprintf(os.Stderr, "Enter password: ")
	bytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", fmt.Errorf(core.MsgErrReadingPassword, err)
	}
	fmt.Fprintln(os.Stderr)
	return string(bytes), nil
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func generateCmd(jsonOutput *bool) *cobra.Command {
	cfg := core.DefaultGeneratorConfig()
	var noUpper, noLower, noDigits, noSymbols bool

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a random password",
		RunE: func(cmd *cobra.Command, args []string) error {
			if noUpper {
				cfg.Uppercase = false
			}
			if noLower {
				cfg.Lowercase = false
			}
			if noDigits {
				cfg.Digits = false
			}
			if noSymbols {
				cfg.Symbols = false
			}

			if err := cfg.Validate(); err != nil {
				return err
			}

			pw, err := core.Generate(cfg)
			if err != nil {
				return err
			}
			if *jsonOutput {
				return printJSON(map[string]any{"password": pw})
			}
			fmt.Println(pw)
			return nil
		},
	}

	cmd.Flags().IntVarP(&cfg.Length, "length", "l", cfg.Length, "password length")
	cmd.Flags().BoolVar(&noUpper, "no-upper", false, "exclude uppercase letters")
	cmd.Flags().BoolVar(&noLower, "no-lower", false, "exclude lowercase letters")
	cmd.Flags().BoolVar(&noDigits, "no-digits", false, "exclude digits")
	cmd.Flags().BoolVar(&noSymbols, "no-symbols", false, "exclude symbols")
	cmd.Flags().StringVar(&cfg.ExcludeChars, "exclude", "", "characters to exclude")

	return cmd
}

func passphraseCmd(jsonOutput *bool) *cobra.Command {
	cfg := core.DefaultPassphraseConfig()

	cmd := &cobra.Command{
		Use:   "passphrase",
		Short: "Generate a passphrase from the EFF wordlist",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cfg.Validate(); err != nil {
				return err
			}

			pp, err := core.GeneratePassphrase(cfg)
			if err != nil {
				return err
			}
			if *jsonOutput {
				return printJSON(map[string]any{"passphrase": pp})
			}
			fmt.Println(pp)
			return nil
		},
	}

	cmd.Flags().IntVarP(&cfg.Words, "words", "w", cfg.Words, "number of words")
	cmd.Flags().StringVarP(&cfg.Separator, "separator", "s", cfg.Separator, "word separator")
	cmd.Flags().BoolVar(&cfg.Capitalize, "capitalize", cfg.Capitalize, "capitalize first letter of each word")
	cmd.Flags().BoolVar(&cfg.AddNumber, "number", cfg.AddNumber, "append a random digit to a random word")

	return cmd
}

func checkCmd(jsonOutput *bool) *cobra.Command {
	var breachCheck bool
	var breachWarnOnly bool

	cmd := &cobra.Command{
		Use:   "check [password|-]",
		Short: "Check the strength of a password",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			password, err := getPassword(args)
			if err != nil {
				return err
			}

			result := core.Score(password)

			if breachCheck {
				checker := core.NewHIBPChecker()
				breached, err := checker.IsBreached(password)
				if err != nil {
					if breachWarnOnly {
						fmt.Fprintf(os.Stderr, core.MsgWarnBreachFailed, err)
					} else {
						return fmt.Errorf(core.MsgErrBreachInconclusive, err)
					}
				} else if breached {
					result.MarkBreached()
				}
			}

			if *jsonOutput {
				return printJSON(result)
			}

			fmt.Printf("Score: %d/100 — %s\n", result.Score, result.Label)
			fmt.Printf("Entropy: %.1f bits\n", result.Entropy)
			if len(result.Penalties) > 0 {
				fmt.Printf("Issues: %s\n", strings.Join(result.Penalties, "; "))
			}
			if len(result.Suggestions) > 0 {
				fmt.Println("Suggestions:")
				for _, s := range result.Suggestions {
					fmt.Printf("  - %s\n", s)
				}
			}
			if result.Breached {
				fmt.Println("BREACHED: This password has appeared in a known data breach!")
			}

			if result.Breached {
				return core.ErrBreached
			}
			if result.Score < core.WeakThreshold {
				return core.ErrWeak
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&breachCheck, "breach", false, "check against Have I Been Pwned")
	cmd.Flags().BoolVar(&breachWarnOnly, "breach-warn-only", false, "do not fail if breach check is inconclusive")

	return cmd
}

func suggestCmd(jsonOutput *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "suggest [password|-]",
		Short: "Get suggestions to improve a weak password",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			password, err := getPassword(args)
			if err != nil {
				return err
			}

			result := core.Score(password)

			if *jsonOutput {
				return printJSON(map[string]any{
					"score":       result.Score,
					"label":       result.Label,
					"suggestions": result.Suggestions,
				})
			}

			fmt.Printf("Score: %d/100 — %s\n", result.Score, result.Label)
			if len(result.Suggestions) > 0 {
				fmt.Println("Suggestions:")
				for _, s := range result.Suggestions {
					fmt.Printf("  - %s\n", s)
				}
			} else {
				fmt.Println("No suggestions — this password looks good!")
			}
			return nil
		},
	}

	return cmd
}

func rotateCmd(jsonOutput *bool) *cobra.Command {
	cfg := core.DefaultRotateConfig()

	cmd := &cobra.Command{
		Use:     "rotate [password|-]",
		Aliases: []string{"ssbd"},
		Short:   "Same Same But Different — generate rotation variants of a password",
		Long:    "Generate a sequence of password variants that cycle leet-speak substitutions, case positions, and symbol placements. Each variant looks different but stays recognizable. Built for forced password rotation policies.\n\nWith --min-length and --max-length, variants can grow or shrink by up to 3 characters via insertions, appends, prepends, or repeat-dropping.",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cfg.Validate(); err != nil {
				return err
			}

			password, err := getPassword(args)
			if err != nil {
				return err
			}

			variants, err := core.RotateWithConfig(password, cfg)
			if err != nil {
				return err
			}

			if *jsonOutput {
				return printJSON(map[string]any{
					"base":     password,
					"variants": variants,
				})
			}

			for i, v := range variants {
				fmt.Printf("%d: %s\n", i+1, v)
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&cfg.Count, "count", "n", cfg.Count, "number of variants to generate")
	cmd.Flags().IntVar(&cfg.MinLength, "min-length", 0, "minimum variant length (default: same as input)")
	cmd.Flags().IntVar(&cfg.MaxLength, "max-length", 0, "maximum variant length (default: same as input)")
	cmd.Flags().BoolVar(&cfg.StrictLength, "strict-length", false, "force all variants to match input length exactly")

	return cmd
}

func bulkCmd(jsonOutput *bool) *cobra.Command {
	cfg := core.DefaultGeneratorConfig()
	var count int
	var noUpper, noLower, noDigits, noSymbols bool

	cmd := &cobra.Command{
		Use:   "bulk",
		Short: "Generate multiple passwords at once",
		RunE: func(cmd *cobra.Command, args []string) error {
			if noUpper {
				cfg.Uppercase = false
			}
			if noLower {
				cfg.Lowercase = false
			}
			if noDigits {
				cfg.Digits = false
			}
			if noSymbols {
				cfg.Symbols = false
			}

			if err := cfg.Validate(); err != nil {
				return err
			}

			passwords := make([]string, 0, count)
			for i := 0; i < count; i++ {
				pw, err := core.Generate(cfg)
				if err != nil {
					return err
				}
				passwords = append(passwords, pw)
			}

			if *jsonOutput {
				return printJSON(map[string]any{"passwords": passwords})
			}

			for _, pw := range passwords {
				fmt.Println(pw)
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&count, "count", "n", core.DefaultBulkCount, "number of passwords to generate")
	cmd.Flags().IntVarP(&cfg.Length, "length", "l", cfg.Length, "password length")
	cmd.Flags().BoolVar(&noUpper, "no-upper", false, "exclude uppercase letters")
	cmd.Flags().BoolVar(&noLower, "no-lower", false, "exclude lowercase letters")
	cmd.Flags().BoolVar(&noDigits, "no-digits", false, "exclude digits")
	cmd.Flags().BoolVar(&noSymbols, "no-symbols", false, "exclude symbols")
	cmd.Flags().StringVar(&cfg.ExcludeChars, "exclude", "", "characters to exclude")

	return cmd
}
