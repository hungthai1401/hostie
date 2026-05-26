// Table-driven validator tests; ports v1 src/domain/__tests__/validators.test.ts.
//
// Case counts measured from v1 (rg/grep on call sites in validators.test.ts):
//
//	validateIPv4(...)         : 11 cases
//	validateIPv6(...)         : 12 cases
//	validateIP(...)           :  7 cases
//	validateNoDuplicates(...) : 10 cases
//	validateHostname          :  0 cases in v1 test file (function exported but
//	                            untested in v1) — we add 31 cases here to
//	                            cover the spec the bead requires.
//
// Total: ≥60 named subtests (verified by `go test -run Validate -v | grep -c "=== RUN"`).
package domain

import (
	"strings"
	"testing"
)

func TestValidateHostname(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
		errSub  string // optional substring expected in error
	}{
		// --- valid (RFC 952/1123) ---
		{"valid_simple", "example.com", false, ""},
		{"valid_subdomain", "www.example.com", false, ""},
		{"valid_single_label", "localhost", false, ""},
		{"valid_numeric_label", "123.example.com", false, ""},
		{"valid_hyphen_middle", "my-host.example.com", false, ""},
		{"valid_uppercase", "EXAMPLE.COM", false, ""},
		{"valid_mixed_case", "Example.Com", false, ""},
		{"valid_digits_only_label", "1.2.3", false, ""},
		{"valid_label_63_chars", strings.Repeat("a", 63) + ".com", false, ""},
		{"valid_total_255_chars", strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 63), false, ""},

		// --- invalid: empty / length ---
		{"empty_string", "", true, "empty"},
		{"too_long_256", strings.Repeat("a", 256), true, "maximum length of 255"},
		{"label_too_long_64", strings.Repeat("a", 64) + ".com", true, "63"},

		// --- invalid: dots ---
		{"consecutive_dots", "foo..bar", true, "consecutive"},
		{"leading_dot", ".example.com", true, "start with a period"},
		{"trailing_dot", "example.com.", true, "end with a period"},
		{"only_dot", ".", true, "start with a period"},

		// --- invalid: labels ---
		{"label_starts_with_hyphen", "-foo.com", true, "start with a letter or digit"},
		{"label_ends_with_hyphen", "foo-.com", true, "end with a letter or digit"},
		{"middle_label_starts_with_hyphen", "foo.-bar.com", true, "start with a letter or digit"},
		{"middle_label_ends_with_hyphen", "foo.bar-.com", true, "end with a letter or digit"},

		// --- invalid: characters ---
		{"underscore", "foo_bar.com", true, "invalid character"},
		{"space", "foo bar.com", true, "invalid character"},
		{"asterisk", "*.com", true, "start with a letter or digit"},
		{"slash", "foo/bar.com", true, "invalid character"},
		{"at_sign", "foo@bar.com", true, "invalid character"},
		{"colon", "foo:bar.com", true, "invalid character"},
		{"plus", "foo+bar.com", true, "invalid character"},
		{"exclam", "foo!bar.com", true, "invalid character"},
		{"unicode", "fooé.com", true, ""},
		{"tab", "foo\tbar.com", true, "invalid character"},
		{"newline", "foo\nbar.com", true, "invalid character"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateHostname(tc.input)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %q, got nil", tc.input)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error for %q, got %v", tc.input, err)
			}
			if tc.wantErr && tc.errSub != "" && !strings.Contains(err.Error(), tc.errSub) {
				t.Fatalf("error %q missing substring %q", err.Error(), tc.errSub)
			}
		})
	}
}

func TestValidateIPv4(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
		errSub  string
	}{
		// --- valid (v1 parity) ---
		{"loopback", "127.0.0.1", false, ""},
		{"private_192", "192.168.1.1", false, ""},
		{"private_10", "10.0.0.1", false, ""},
		{"all_zeros", "0.0.0.0", false, ""},
		{"all_255", "255.255.255.255", false, ""},

		// --- invalid (v1 parity) ---
		{"octet_over_255", "256.1.1.1", true, "Octet"},
		{"too_few_octets", "1.1.1", true, "four octets"},
		{"too_many_octets", "1.1.1.1.1", true, "four octets"},
		{"empty", "", true, ""},
		{"alpha_octet", "192.168.a.1", true, "numeric"},
		{"negative_octet", "192.168.-1.1", true, ""},

		// --- additional cases for full coverage ---
		{"leading_zero", "01.2.3.4", true, "leading zeros"},
		{"leading_zero_triple", "001.2.3.4", true, "leading zeros"},
		{"trailing_whitespace", "1.2.3.4 ", true, "whitespace"},
		{"leading_whitespace", " 1.2.3.4", true, "whitespace"},
		{"empty_octet", "1..2.3", true, ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateIPv4(tc.input)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %q, got nil", tc.input)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error for %q, got %v", tc.input, err)
			}
			if tc.wantErr && tc.errSub != "" && !strings.Contains(err.Error(), tc.errSub) {
				t.Fatalf("error %q missing substring %q", err.Error(), tc.errSub)
			}
		})
	}
}

func TestValidateIPv6(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
		errSub  string
	}{
		// --- valid (v1 parity) ---
		{"loopback_compressed", "::1", false, ""},
		{"docs_compressed", "2001:db8::1", false, ""},
		{"link_local", "fe80::1", false, ""},
		{"full_expansion", "2001:0db8:0000:0000:0000:0000:0000:0001", false, ""},
		{"all_zeros", "::", false, ""},
		{"mid_compression", "2001:db8:85a3::8a2e:370:7334", false, ""},

		// --- invalid (v1 parity) ---
		{"triple_colon", ":::1", true, "consecutive colons"},
		{"invalid_hex_xyz", "2001:db8::xyz", true, "hexadecimal"},
		{"empty", "", true, ""},
		{"too_many_groups", "1:2:3:4:5:6:7:8:9", true, "groups"},
		{"group_5_hex", "12345::1", true, "4 hexadecimal"},
		{"multiple_compression", "2001::db8::1", true, "one double-colon"},

		// --- IPv4-mapped and dotted-quad embeds (must reject per v1 parity) ---
		{"ipv4_mapped_full", "::ffff:192.168.1.1", true, "dotted-decimal"},
		{"ipv4_mapped_short", "::ffff:1.2.3.4", true, "dotted-decimal"},
		{"dotted_quad_embed", "::1.2.3.4", true, "dotted-decimal"},
		{"dotted_quad_full", "2001:db8::192.168.1.1", true, "dotted-decimal"},

		// --- additional coverage ---
		{"empty_group_mid", "1:2::3:4:5:6:7:8", true, "groups"},
		{"too_few_groups_no_compress", "1:2:3:4:5:6:7", true, "exactly 8 groups"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateIPv6(tc.input)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %q, got nil", tc.input)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error for %q, got %v", tc.input, err)
			}
			if tc.wantErr && tc.errSub != "" && !strings.Contains(err.Error(), tc.errSub) {
				t.Fatalf("error %q missing substring %q", err.Error(), tc.errSub)
			}
		})
	}
}

func TestValidateIP(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"ipv4_loopback", "127.0.0.1", false},
		{"ipv4_private", "192.168.1.1", false},
		{"ipv6_loopback", "::1", false},
		{"ipv6_docs", "2001:db8::1", false},
		{"invalid_octet", "256.1.1.1", true},
		{"empty", "", true},
		{"not_an_ip", "not-an-ip", true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateIP(tc.input)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %q, got nil", tc.input)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error for %q, got %v", tc.input, err)
			}
		})
	}
}

func TestValidateComment(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
		errSub  string
	}{
		// Valid comments
		{"empty", "", false, ""},
		{"simple", "production server", false, ""},
		{"with_punctuation", "server-01 (backup)", false, ""},
		{"unicode", "测试服务器", false, ""},
		{"spaces", "   leading and trailing   ", false, ""},
		
		// Invalid: newlines
		{"newline_lf", "line1\nline2", true, "control character"},
		{"newline_cr", "line1\rline2", true, "control character"},
		{"newline_crlf", "line1\r\nline2", true, "control character"},
		
		// Invalid: control characters
		{"null_byte", "test\x00injection", true, "control character"},
		{"tab", "test\tinjection", true, "control character"},
		{"bell", "test\x07injection", true, "control character"},
		{"escape", "test\x1binjection", true, "control character"},
		{"delete", "test\x7finjection", true, "control character"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateComment(tc.input)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %q, got nil", tc.input)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error for %q, got %v", tc.input, err)
			}
			if tc.wantErr && tc.errSub != "" && !strings.Contains(err.Error(), tc.errSub) {
				t.Fatalf("expected error containing %q, got %v", tc.errSub, err)
			}
		})
	}
}

func TestValidateNoDuplicates(t *testing.T) {
	cases := []struct {
		name    string
		entries []Entry
		wantErr bool
		errSub  string
	}{
		{
			name: "unique_hostnames",
			entries: []Entry{
				{ID: "1", IP: "127.0.0.1", Hostname: "example.com", Aliases: nil, Enabled: true},
				{ID: "2", IP: "127.0.0.2", Hostname: "test.com", Aliases: nil, Enabled: true},
			},
			wantErr: false,
		},
		{
			name: "duplicate_hostname_enabled",
			entries: []Entry{
				{ID: "1", IP: "127.0.0.1", Hostname: "example.com", Enabled: true},
				{ID: "2", IP: "127.0.0.2", Hostname: "example.com", Enabled: true},
			},
			wantErr: true,
			errSub:  "example.com",
		},
		{
			name: "duplicate_hostname_case_insensitive",
			entries: []Entry{
				{ID: "1", IP: "127.0.0.1", Hostname: "example.com", Enabled: true},
				{ID: "2", IP: "127.0.0.2", Hostname: "EXAMPLE.COM", Enabled: true},
			},
			wantErr: true,
			errSub:  "example.com",
		},
		{
			name: "duplicate_alias",
			entries: []Entry{
				{ID: "1", IP: "127.0.0.1", Hostname: "example.com", Aliases: []string{"alias1.com"}, Enabled: true},
				{ID: "2", IP: "127.0.0.2", Hostname: "test.com", Aliases: []string{"alias1.com"}, Enabled: true},
			},
			wantErr: true,
			errSub:  "alias1.com",
		},
		{
			name: "hostname_conflicts_with_alias",
			entries: []Entry{
				{ID: "1", IP: "127.0.0.1", Hostname: "example.com", Enabled: true},
				{ID: "2", IP: "127.0.0.2", Hostname: "test.com", Aliases: []string{"example.com"}, Enabled: true},
			},
			wantErr: true,
			errSub:  "example.com",
		},
		{
			name: "allow_duplicates_one_disabled",
			entries: []Entry{
				{ID: "1", IP: "127.0.0.1", Hostname: "example.com", Enabled: true},
				{ID: "2", IP: "127.0.0.2", Hostname: "example.com", Enabled: false},
			},
			wantErr: false,
		},
		{
			name: "allow_duplicates_both_disabled",
			entries: []Entry{
				{ID: "1", IP: "127.0.0.1", Hostname: "example.com", Enabled: false},
				{ID: "2", IP: "127.0.0.2", Hostname: "example.com", Enabled: false},
			},
			wantErr: false,
		},
		{
			name:    "empty_slice",
			entries: nil,
			wantErr: false,
		},
		{
			name: "single_entry",
			entries: []Entry{
				{ID: "1", IP: "127.0.0.1", Hostname: "example.com", Enabled: true},
			},
			wantErr: false,
		},
		{
			name: "multiple_aliases_conflict",
			entries: []Entry{
				{ID: "1", IP: "127.0.0.1", Hostname: "example.com", Aliases: []string{"alias1.com", "alias2.com"}, Enabled: true},
				{ID: "2", IP: "127.0.0.2", Hostname: "test.com", Aliases: []string{"alias3.com", "alias2.com"}, Enabled: true},
			},
			wantErr: true,
			errSub:  "alias2.com",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateNoDuplicates(tc.entries)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tc.wantErr && tc.errSub != "" && !strings.Contains(err.Error(), tc.errSub) {
				t.Fatalf("error %q missing substring %q", err.Error(), tc.errSub)
			}
		})
	}
}
