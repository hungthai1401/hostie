# Spike 2: Sudo Re-exec in Non-Interactive Contexts

## Risk
**HIGH** — `core/apply.ts` must handle sudo re-exec correctly in both interactive terminals and non-interactive contexts (CI, cron, scripts).

## Approach
Created a test script that:
1. Checks if running as root (skip test if so)
2. Verifies sudo is available
3. Tests sudo access with `-v` flag
4. Attempts a write operation that requires sudo
5. Re-execs with sudo on EACCES

## Findings
**Partial success** — The spike revealed important constraints:

✅ **Sudo detection works**: Can detect EACCES and trigger re-exec  
✅ **Sudo availability check works**: `which sudo` succeeds  
❌ **Non-interactive context fails**: `sudo -v` requires a TTY for password prompts

Error encountered:
```
sudo: a terminal is required to read the password; either use the -S option to read from standard input or configure an askpass helper
sudo: a password is required
```

## Verdict
**CONFIRMED with caveats** — Mitigation works in interactive contexts, requires documentation for non-interactive use.

**Implementation guidance for `core/apply.ts`:**

1. **Interactive context (normal case):**
   - Detect EACCES when writing to `/etc/hosts`
   - Re-exec with `sudo bun <script> apply` (inherits stdin/stdout/stderr)
   - User sees sudo password prompt in terminal
   - Works correctly ✅

2. **Non-interactive context (CI, cron, scripts):**
   - Sudo requires either:
     - Passwordless sudo (via `/etc/sudoers` NOPASSWD directive)
     - Pre-authenticated sudo session (`sudo -v` before running)
     - SSH agent forwarding with sudo
   - Document this requirement in README
   - Provide clear error message if sudo fails in non-interactive context

3. **Error handling:**
   - Catch EACCES → attempt sudo re-exec
   - If sudo re-exec fails → exit with code 3 (permission error) and helpful message:
     ```
     Error: Cannot write to /etc/hosts (permission denied)
     
     In interactive terminals: Run 'hostie apply' and enter your password when prompted
     In CI/scripts: Configure passwordless sudo or run 'sudo -v' before hostie
     ```

4. **Detection strategy:**
   - Check `process.stdin.isTTY` to detect interactive vs non-interactive
   - In non-interactive contexts, provide actionable error message instead of failing silently

**Recommendation:** This is acceptable behavior. Most system tools (apt, yum, systemctl) have the same constraint. Document it clearly in README and error messages.
