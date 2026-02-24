package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/passforge/passforge/internal/core"
	"github.com/spf13/cobra"
)

var jsonOutput bool

func main() {
	rootCmd := &cobra.Command{
		Use:   "passforge",
		Short: "Password generator, strength checker, and suggestion engine",
		Long:  "PassForge generates strong passwords, scores their strength, and provides actionable suggestions to improve weak ones.",
	}

	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output in JSON format")

	rootCmd.AddCommand(generateCmd())
	rootCmd.AddCommand(passphraseCmd())
	rootCmd.AddCommand(checkCmd())
	rootCmd.AddCommand(suggestCmd())
	rootCmd.AddCommand(rotateCmd())
	rootCmd.AddCommand(bulkCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func generateCmd() *cobra.Command {
	cfg := core.DefaultGeneratorConfig()

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a random password",
		RunE: func(cmd *cobra.Command, args []string) error {
			pw, err := core.Generate(cfg)
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(map[string]any{"password": pw})
			}
			fmt.Println(pw)
			return nil
		},
	}

	cmd.Flags().IntVarP(&cfg.Length, "length", "l", cfg.Length, "password length")
	cmd.Flags().BoolVar(&cfg.Uppercase, "upper", cfg.Uppercase, "include uppercase letters")
	cmd.Flags().BoolVar(&cfg.Lowercase, "lower", cfg.Lowercase, "include lowercase letters")
	cmd.Flags().BoolVar(&cfg.Digits, "digits", cfg.Digits, "include digits")
	cmd.Flags().BoolVar(&cfg.Symbols, "symbols", cfg.Symbols, "include symbols")
	cmd.Flags().StringVar(&cfg.ExcludeChars, "exclude", "", "characters to exclude")

	return cmd
}

func passphraseCmd() *cobra.Command {
	cfg := core.DefaultPassphraseConfig()

	cmd := &cobra.Command{
		Use:   "passphrase",
		Short: "Generate a passphrase from the EFF wordlist",
		RunE: func(cmd *cobra.Command, args []string) error {
			pp, err := core.GeneratePassphrase(cfg)
			if err != nil {
				return err
			}
			if jsonOutput {
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

func checkCmd() *cobra.Command {
	var breachCheck bool

	cmd := &cobra.Command{
		Use:   "check [password]",
		Short: "Check the strength of a password",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			password := args[0]
			result := core.Score(password)

			if breachCheck {
				checker := core.NewHIBPChecker()
				breached, err := checker.IsBreached(password)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: breach check failed: %v\n", err)
				} else if breached {
					result.Breached = true
					result.Score = min(result.Score, core.BreachScoreCap)
					result.Label = core.LabelForScore(result.Score)
					result.Penalties = append(result.Penalties, "found in data breach")
					result.Suggestions = append(result.Suggestions, "This password appeared in a data breach — do not use it")
				}
			}

			if jsonOutput {
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

			// Exit code based on strength
			if result.Breached {
				os.Exit(2)
			}
			if result.Score < core.WeakThreshold {
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&breachCheck, "breach", false, "check against Have I Been Pwned")

	return cmd
}

func suggestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "suggest [password]",
		Short: "Get suggestions to improve a weak password",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			password := args[0]
			result := core.Score(password)

			if jsonOutput {
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

func rotateCmd() *cobra.Command {
	cfg := core.DefaultRotateConfig()

	cmd := &cobra.Command{
		Use:     "rotate [password]",
		Aliases: []string{"ssbd"},
		Short:   "Same Same But Different — generate rotation variants of a password",
		Long:    "Generate a sequence of password variants that cycle leet-speak substitutions, case positions, and symbol placements. Each variant looks different but stays recognizable. Built for forced password rotation policies.\n\nWith --min-length and --max-length, variants can grow or shrink by up to 3 characters via insertions, appends, prepends, or repeat-dropping.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			password := args[0]
			variants, err := core.RotateWithConfig(password, cfg)
			if err != nil {
				return err
			}

			if jsonOutput {
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

func bulkCmd() *cobra.Command {
	cfg := core.DefaultGeneratorConfig()
	var count int

	cmd := &cobra.Command{
		Use:   "bulk",
		Short: "Generate multiple passwords at once",
		RunE: func(cmd *cobra.Command, args []string) error {
			passwords := make([]string, 0, count)
			for i := 0; i < count; i++ {
				pw, err := core.Generate(cfg)
				if err != nil {
					return err
				}
				passwords = append(passwords, pw)
			}

			if jsonOutput {
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
	cmd.Flags().BoolVar(&cfg.Uppercase, "upper", cfg.Uppercase, "include uppercase letters")
	cmd.Flags().BoolVar(&cfg.Lowercase, "lower", cfg.Lowercase, "include lowercase letters")
	cmd.Flags().BoolVar(&cfg.Digits, "digits", cfg.Digits, "include digits")
	cmd.Flags().BoolVar(&cfg.Symbols, "symbols", cfg.Symbols, "include symbols")
	cmd.Flags().StringVar(&cfg.ExcludeChars, "exclude", "", "characters to exclude")

	return cmd
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
