// privilege_test.go — unit tests for apply.ValidatePayloadFile marker
// invariants added by hosts-cli-review-p1-sudo-merge-boundary-bre.
//
// The pre-existing perm/owner checks are covered indirectly via
// go/internal/cmd/apply_privileged_test.go. This file adds focused unit
// coverage for the NEW marker-invariant assertions:
//
//   - empty payload rejected
//   - missing markers rejected (no BEGIN, no END)
//   - duplicate BEGIN rejected
//   - duplicate END rejected
//   - END-before-BEGIN rejected
//   - bytes outside the managed block (preamble or suffix) rejected
//   - well-formed marker-only payload accepted

package apply

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writePayloadFile(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "payload")
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	require.NoError(t, os.Chmod(p, 0o600))
	return p
}

func TestValidatePayloadFile_Empty(t *testing.T) {
	p := writePayloadFile(t, "")
	_, err := ValidatePayloadFile(p)
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty")
}

func TestValidatePayloadFile_MissingMarkers(t *testing.T) {
	p := writePayloadFile(t, "# just a comment\n127.0.0.1 example.test\n")
	_, err := ValidatePayloadFile(p)
	require.Error(t, err)
	require.Contains(t, err.Error(), "managed block markers")
}

func TestValidatePayloadFile_OnlyBegin(t *testing.T) {
	p := writePayloadFile(t, "# BEGIN HOSTIE\n127.0.0.1 example.test\n")
	_, err := ValidatePayloadFile(p)
	require.Error(t, err)
	require.Contains(t, err.Error(), "marker validation failed")
}

func TestValidatePayloadFile_OnlyEnd(t *testing.T) {
	p := writePayloadFile(t, "127.0.0.1 example.test\n# END HOSTIE\n")
	_, err := ValidatePayloadFile(p)
	require.Error(t, err)
	require.Contains(t, err.Error(), "marker validation failed")
}

func TestValidatePayloadFile_DuplicateBegin(t *testing.T) {
	p := writePayloadFile(t, "# BEGIN HOSTIE\n# BEGIN HOSTIE\n# END HOSTIE\n")
	_, err := ValidatePayloadFile(p)
	require.Error(t, err)
	require.Contains(t, err.Error(), "marker validation failed")
}

func TestValidatePayloadFile_DuplicateEnd(t *testing.T) {
	p := writePayloadFile(t, "# BEGIN HOSTIE\n# END HOSTIE\n# END HOSTIE\n")
	_, err := ValidatePayloadFile(p)
	require.Error(t, err)
	require.Contains(t, err.Error(), "marker validation failed")
}

func TestValidatePayloadFile_EndBeforeBegin(t *testing.T) {
	p := writePayloadFile(t, "# END HOSTIE\n# BEGIN HOSTIE\n")
	_, err := ValidatePayloadFile(p)
	require.Error(t, err)
	require.Contains(t, err.Error(), "marker validation failed")
}

func TestValidatePayloadFile_PreambleBeforeBegin(t *testing.T) {
	p := writePayloadFile(t, "leading\n# BEGIN HOSTIE\n# END HOSTIE\n")
	_, err := ValidatePayloadFile(p)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ONLY the managed block")
}

func TestValidatePayloadFile_SuffixAfterEnd(t *testing.T) {
	p := writePayloadFile(t, "# BEGIN HOSTIE\n# END HOSTIE\ntrailing\n")
	_, err := ValidatePayloadFile(p)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ONLY the managed block")
}

func TestValidatePayloadFile_HappyPath(t *testing.T) {
	body := "# BEGIN HOSTIE\n127.0.0.1 example.test\n# END HOSTIE\n"
	p := writePayloadFile(t, body)
	got, err := ValidatePayloadFile(p)
	require.NoError(t, err)
	require.Equal(t, body, string(got))
}
