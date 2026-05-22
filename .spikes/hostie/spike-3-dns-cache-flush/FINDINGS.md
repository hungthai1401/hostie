# Spike 3: DNS Cache Flush Platform Detection

## Risk
**HIGH** — DNS cache flush requires platform-specific commands and sudo access. Must detect platform correctly and handle failures gracefully.

## Approach
Created a test script that:
1. Detects platform (darwin vs linux)
2. Attempts platform-specific DNS cache flush commands
3. Falls back through multiple Linux cache services (systemd-resolved, nscd, dnsmasq)

**macOS commands:**
```bash
sudo dscacheutil -flushcache
sudo killall -HUP mDNSResponder
```

**Linux commands (in order of preference):**
```bash
sudo systemctl restart systemd-resolved  # Modern systemd distros
sudo systemctl restart nscd              # Older distros with nscd
sudo systemctl restart dnsmasq           # Distros using dnsmasq
```

## Findings
**Platform detection works correctly** — The spike successfully:

✅ **Detects platform**: `process.platform` correctly identifies darwin vs linux  
✅ **Command selection works**: Correct commands chosen per platform  
✅ **Linux fallback logic**: Checks for systemd-resolved → nscd → dnsmasq in order  
❌ **Execution blocked by sudo TTY requirement**: Same issue as spike 2

## Verdict
**CONFIRMED with caveats** — Platform detection and command selection work correctly. Execution has same sudo constraints as spike 2.

**Implementation guidance for DNS cache flush:**

1. **Make DNS cache flush optional, not required:**
   - After writing `/etc/hosts`, attempt DNS cache flush
   - If flush fails (no sudo, wrong platform, service not found), **warn but don't fail**
   - Most applications will pick up the new `/etc/hosts` on next DNS lookup anyway

2. **Platform detection:**
   ```typescript
   const platform = process.platform;
   if (platform === "darwin") {
     // macOS commands
   } else if (platform === "linux") {
     // Linux fallback chain
   } else {
     // Unsupported platform, skip flush
   }
   ```

3. **Graceful degradation:**
   - Try flush, catch errors
   - If flush fails, print warning:
     ```
     Warning: Could not flush DNS cache. Changes to /etc/hosts may not take effect immediately.
     
     To flush manually:
       macOS: sudo dscacheutil -flushcache && sudo killall -HUP mDNSResponder
       Linux: sudo systemctl restart systemd-resolved
     ```

4. **User control:**
   - Add `--no-flush` flag to skip DNS cache flush entirely
   - Add `--flush` flag to make it explicit (default: attempt but don't fail)

5. **Exit codes:**
   - DNS cache flush failure should NOT cause non-zero exit
   - Only fail if `/etc/hosts` write itself fails

**Recommendation:** DNS cache flush is a nice-to-have, not a requirement. The `/etc/hosts` file is the source of truth; applications will eventually pick up changes. Make flush best-effort with clear warnings on failure.
