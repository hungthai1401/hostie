// Validators for hostie domain types.
//
// Ports v1 src/domain/validators.ts to Go. Returns error (nil = valid).
package domain

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// ValidateHostname checks RFC 952/1123 compliance.
//
// Rules (mirrors v1):
//   - 1..255 total chars
//   - no leading/trailing dot, no consecutive dots
//   - labels 1..63 chars
//   - each label: first/last char alnum, body alnum or hyphen
func ValidateHostname(hostname string) error {
	if hostname == "" {
		return fmt.Errorf("hostname cannot be empty")
	}
	if len(hostname) > 255 {
		return fmt.Errorf("hostname exceeds maximum length of 255 characters")
	}
	if strings.Contains(hostname, "..") {
		return fmt.Errorf("hostname cannot contain consecutive periods")
	}
	if strings.HasPrefix(hostname, ".") {
		return fmt.Errorf("hostname cannot start with a period")
	}
	if strings.HasSuffix(hostname, ".") {
		return fmt.Errorf("hostname cannot end with a period")
	}

	for _, label := range strings.Split(hostname, ".") {
		if label == "" {
			return fmt.Errorf("hostname labels cannot be empty")
		}
		if len(label) > 63 {
			return fmt.Errorf("hostname label %q exceeds maximum length of 63 characters", label)
		}
		if !isAlnum(label[0]) {
			return fmt.Errorf("hostname label %q must start with a letter or digit", label)
		}
		if !isAlnum(label[len(label)-1]) {
			return fmt.Errorf("hostname label %q must end with a letter or digit", label)
		}
		for i := 0; i < len(label); i++ {
			c := label[i]
			if !isAlnum(c) && c != '-' {
				return fmt.Errorf("hostname label %q contains invalid character %q", label, string(c))
			}
		}
	}
	return nil
}

func isAlnum(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// ValidateIPv4 checks dotted-decimal: 4 octets 0..255, no leading zeros, no whitespace.
func ValidateIPv4(ip string) error {
	if ip == "" {
		return fmt.Errorf("IPv4 address cannot be empty")
	}
	if ip != strings.TrimSpace(ip) {
		return fmt.Errorf("IPv4 address must not have leading or trailing whitespace")
	}
	octets := strings.Split(ip, ".")
	if len(octets) != 4 {
		return fmt.Errorf("IPv4 address must have exactly four octets")
	}
	for _, oct := range octets {
		if oct == "" {
			return fmt.Errorf("octet %q must be numeric", oct)
		}
		for i := 0; i < len(oct); i++ {
			if oct[i] < '0' || oct[i] > '9' {
				return fmt.Errorf("octet %q must be numeric", oct)
			}
		}
		v, err := strconv.Atoi(oct)
		if err != nil {
			return fmt.Errorf("octet %q must be numeric", oct)
		}
		if v < 0 || v > 255 {
			return fmt.Errorf("Octet %q must be between 0 and 255", oct)
		}
		if len(oct) > 1 && oct[0] == '0' {
			return fmt.Errorf("octet %q cannot have leading zeros", oct)
		}
	}
	return nil
}

// ValidateIPv6 checks IPv6 syntax (mirrors v1: rejects IPv4-mapped & dotted-quad embeds).
//
// We hand-roll the parser to match v1 semantics exactly rather than rely on
// net.ParseIP, which accepts IPv4-mapped forms ("::ffff:1.2.3.4") that v1 rejects.
func ValidateIPv6(ip string) error {
	if ip == "" {
		return fmt.Errorf("IPv6 address cannot be empty")
	}
	if strings.Contains(ip, ":::") {
		return fmt.Errorf("IPv6 address cannot have three or more consecutive colons")
	}
	if strings.Count(ip, "::") > 1 {
		return fmt.Errorf("IPv6 address can only have one double-colon compression")
	}

	var groups []string
	if strings.Contains(ip, "::") {
		parts := strings.SplitN(ip, "::", 2)
		var left, right []string
		if parts[0] != "" {
			left = strings.Split(parts[0], ":")
		}
		if parts[1] != "" {
			right = strings.Split(parts[1], ":")
		}
		// With `::` present, the elided run must cover ≥1 zero group,
		// so total explicit groups must be < 8.
		if len(left)+len(right) >= 8 {
			return fmt.Errorf("IPv6 address cannot have more than 8 groups")
		}
		groups = append(groups, left...)
		groups = append(groups, right...)
	} else {
		groups = strings.Split(ip, ":")
		if len(groups) != 8 {
			return fmt.Errorf("IPv6 address without compression must have exactly 8 groups")
		}
	}

	for _, g := range groups {
		if len(g) == 0 || len(g) > 4 {
			return fmt.Errorf("IPv6 group %q must be 1-4 hexadecimal digits", g)
		}
		for i := 0; i < len(g); i++ {
			c := g[i]
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return fmt.Errorf("IPv6 group %q must contain only hexadecimal digits", g)
			}
		}
	}

	// Sanity: net.ParseIP must also accept it (defensive parity).
	if parsed := net.ParseIP(ip); parsed == nil {
		return fmt.Errorf("invalid IPv6 address %q", ip)
	}
	return nil
}

// ValidateIP accepts either valid IPv4 or IPv6 (v1 parity).
func ValidateIP(ip string) error {
	if err := ValidateIPv4(ip); err == nil {
		return nil
	}
	if err := ValidateIPv6(ip); err == nil {
		return nil
	}
	return fmt.Errorf("invalid IP address (neither valid IPv4 nor IPv6)")
}

// ValidateComment checks that a comment does not contain newlines or control
// characters that could break out of the managed block or inject malicious content.
func ValidateComment(comment string) error {
	for i := 0; i < len(comment); i++ {
		c := comment[i]
		// Reject newline (\n, \r) and all ASCII control characters (0x00-0x1F, 0x7F)
		if c == '\n' || c == '\r' || c < 0x20 || c == 0x7F {
			return fmt.Errorf("comment contains invalid control character at position %d", i)
		}
	}
	return nil
}

// ValidateNoDuplicates returns the first duplicate hostname/alias collision
// across enabled entries. Case-insensitive (v1 parity). Disabled entries are
// ignored entirely.
func ValidateNoDuplicates(entries []Entry) error {
	seen := make(map[string]struct{})
	for _, e := range entries {
		if !e.Enabled {
			continue
		}
		h := strings.ToLower(e.Hostname)
		if _, dup := seen[h]; dup {
			return fmt.Errorf("duplicate hostname %q found in enabled entries", h)
		}
		seen[h] = struct{}{}
		for _, a := range e.Aliases {
			al := strings.ToLower(a)
			if _, dup := seen[al]; dup {
				return fmt.Errorf("duplicate hostname/alias %q found in enabled entries", al)
			}
			seen[al] = struct{}{}
		}
	}
	return nil
}
