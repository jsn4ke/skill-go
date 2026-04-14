## 1. Root cause investigation

- [x] 1.1 Add debug logging to `handleAuraUpdate` to verify: aura exists on target, PeriodicTimer > 0, tick calculation, damage target identity
- [x] 1.2 Add debug logging to `makeAuraHandler` to verify aura is created with correct fields (BaseAmount, PeriodicTimer, Duration)
- [x] 1.3 Start server, cast Fireball, check logs to identify where the DoT pipeline breaks

## 2. Fix

- [x] 2.1 Fix identified root cause: `main.go` hardcoded `"server/data"` path, fails when running `go run .` from `server/` dir. Added auto-detection: try `data/` first, fallback to `server/data/`
- [x] 2.2 Verify fix: `go test ./...` passes
- [x] 2.3 Verify fix: manual test — cast Fireball, observe DoT ticks in timeline and HP change
