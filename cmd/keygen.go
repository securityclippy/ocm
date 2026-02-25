package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var keygenFlags struct {
	output string
	force  bool
}

var keygenCmd = &cobra.Command{
	Use:   "keygen",
	Short: "Generate a new master encryption key",
	Long: `Generate a cryptographically secure 256-bit master key for OCM.

The key is used to encrypt all stored credentials with AES-256-GCM.

Examples:
  # Print key to stdout (copy to OCM_MASTER_KEY env var)
  ocm keygen

  # Write key to a file
  ocm keygen -o /path/to/master.key

  # Overwrite existing key file
  ocm keygen -o /path/to/master.key --force

Security notes:
  - Store the key securely (e.g., secrets manager, encrypted vault)
  - Never commit the key to version control
  - Back up the key - without it, credentials cannot be decrypted
  - Set restrictive permissions on the key file (chmod 600)`,
	RunE: runKeygen,
}

func init() {
	keygenCmd.Flags().StringVarP(&keygenFlags.output, "output", "o", "", "Write key to file instead of stdout")
	keygenCmd.Flags().BoolVarP(&keygenFlags.force, "force", "f", false, "Overwrite existing key file")
}

func runKeygen(cmd *cobra.Command, args []string) error {
	// Generate 32 random bytes (256 bits)
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("failed to generate random key: %w", err)
	}

	keyHex := hex.EncodeToString(key)

	if keygenFlags.output == "" {
		// Print to stdout
		fmt.Println(keyHex)
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "To use this key:")
		fmt.Fprintln(os.Stderr, "  export OCM_MASTER_KEY="+keyHex)
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Or save to a file:")
		fmt.Fprintln(os.Stderr, "  ocm keygen -o ~/.ocm/master.key")
		return nil
	}

	// Write to file
	outputPath := keygenFlags.output
	if outputPath[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to expand home directory: %w", err)
		}
		outputPath = filepath.Join(home, outputPath[1:])
	}

	// Check if file exists
	if _, err := os.Stat(outputPath); err == nil && !keygenFlags.force {
		return fmt.Errorf("key file already exists: %s (use --force to overwrite)", outputPath)
	}

	// Create parent directories if needed
	if err := os.MkdirAll(filepath.Dir(outputPath), 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write key with restrictive permissions
	if err := os.WriteFile(outputPath, []byte(keyHex), 0600); err != nil {
		return fmt.Errorf("failed to write key file: %w", err)
	}

	fmt.Printf("Master key written to: %s\n", outputPath)
	fmt.Println("")
	fmt.Println("To use this key:")
	fmt.Printf("  ocm serve --master-key-file %s\n", keygenFlags.output)
	fmt.Println("")
	fmt.Println("Security reminder:")
	fmt.Println("  - Back up this key securely")
	fmt.Println("  - Never commit it to version control")
	fmt.Println("  - Without this key, credentials cannot be decrypted")

	return nil
}
