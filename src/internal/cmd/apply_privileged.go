package cmd

import (
	"fmt"
	"os"
	"syscall"

	"github.com/hungthai1401/hostie/src/internal/apply"
	"github.com/hungthai1401/hostie/src/internal/core/etchosts"
	"github.com/spf13/cobra"
)

// applyPrivilegedCmd is a hidden subcommand used internally for privileged
// /etc/hosts write. Invoked by sudo from both the CLI (apply.ReexecWithSudo —
// indirect, via re-exec of the original argv) and the TUI (app.SudoApplyCmd
// via tea.ExecProcess).
//
// Contract (per design.md D12 + approach.md §8 threat model):
//
//   - --payload-path=<file>  : absolute path to a 0600 tempfile under $TMPDIR
//                              containing the rendered managed-block bytes.
//   - --owner-uid=<uid>      : the real (invoking) user's uid. The tempfile
//                              must be owned by this uid; if not, the command
//                              refuses and unlinks nothing (to preserve
//                              evidence of the mismatch).
//
// The receiver re-validates the file via apply.ValidatePayloadFile (regular
// file, 0600 perms, owned by invoking user) before writing /etc/hosts. The
// tempfile is unlinked on every exit path via defer.
var (
	applyPrivilegedPayloadPath string
	applyPrivilegedOwnerUID    int
)

var applyPrivilegedCmd = &cobra.Command{
	Use:    apply.APPLY_PRIVILEGED_CMD,
	Short:  "Internal command for privileged /etc/hosts write (do not call directly)",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		payloadPath := applyPrivilegedPayloadPath
		ownerUID := applyPrivilegedOwnerUID

		if payloadPath == "" {
			return fmt.Errorf("--payload-path is required")
		}
		if ownerUID <= 0 {
			return fmt.Errorf("--owner-uid must be > 0, got %d", ownerUID)
		}

		// Confirm the file's on-disk uid matches the asserted owner-uid
		// BEFORE handing off to ValidatePayloadFile. ValidatePayloadFile
		// allows either SUDO_UID or the current uid; this extra check
		// enforces the caller-asserted identity from the wire bead's
		// --owner-uid contract.
		info, err := os.Lstat(payloadPath)
		if err != nil {
			return fmt.Errorf("payload not accessible: %w", err)
		}
		if stat, ok := info.Sys().(*syscall.Stat_t); !ok {
			return fmt.Errorf("payload stat unavailable")
		} else if int(stat.Uid) != ownerUID {
			return fmt.Errorf("payload owner uid mismatch: file=%d asserted=%d",
				stat.Uid, ownerUID)
		}

		content, err := apply.ValidatePayloadFile(payloadPath)
		if err != nil {
			return fmt.Errorf("payload validation failed: %w", err)
		}

		// Ensure cleanup happens on all exit paths (success and failure
		// from the etchosts write below). The SIGINT/SIGTERM handler from
		// WritePayloadToTempfile (caller side) covers the signal path.
		defer func() {
			if err := os.Remove(payloadPath); err != nil && !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Warning: failed to remove payload file: %v\n", err)
			}
		}()

		// Threat-model §3.3: the privileged side re-derives the /etc/hosts
		// merge. The payload contains ONLY the rendered managed block
		// (asserted by ValidatePayloadFile above); we read /etc/hosts here
		// under root and call etchosts.ReplaceManagedBlock to splice the
		// payload between the BEGIN/END markers. This closes the TOCTOU
		// window between the unprivileged TUI's read and the privileged
		// write — any concurrent edit to the un-managed region of
		// /etc/hosts is preserved, and the unprivileged TUI never controls
		// the bytes outside its own managed block.
		etcContent, err := os.ReadFile(apply.ETC_HOSTS_PATH)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to read /etc/hosts: %w", err)
		}
		merged, err := etchosts.ReplaceManagedBlock(etcContent, content)
		if err != nil {
			return fmt.Errorf("failed to merge managed block: %w", err)
		}

		if err := etchosts.WriteEtcHosts(apply.ETC_HOSTS_PATH, string(merged)); err != nil {
			return fmt.Errorf("failed to write /etc/hosts: %w", err)
		}

		return nil
	},
}

func init() {
	applyPrivilegedCmd.Flags().StringVar(&applyPrivilegedPayloadPath,
		"payload-path", "", "absolute path to the 0600 payload tempfile under $TMPDIR")
	applyPrivilegedCmd.Flags().IntVar(&applyPrivilegedOwnerUID,
		"owner-uid", 0, "uid of the invoking user that owns the payload tempfile")
}
