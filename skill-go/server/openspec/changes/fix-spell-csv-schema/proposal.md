## Why

The `spell_effects.csv` has two structural problems:

1. `school_damage` rows have `periodicType` and `amplitude` columns (positions 6-7 in WoW's SPELLITE structure) that are absent from the header. The loader currently maps column 9 → `TriggerSpellID`, but for Blizzard's DoT row (value=8), this stores amplitude as TriggerSpellID instead of amplitude.

2. Trailing empty fields are omitted, making row column counts inconsistent (9, 10, or 11 columns). The loader pads with empty strings, which further misaligns field positions.

## What Changes

- Add `periodicType` and `amplitude` columns to `spell_effects.csv` header (standardizes to 12 columns for all rows, padding non-periodic effects with empty strings)
- Update `spell_effects.csv` rows to have exactly 12 columns (trailing empty fields explicit)
- Fix `parseEffectRow` in `spelldef/loader.go` to read `periodicType` at index 6 and `amplitude` at index 7 for `school_damage` effects
- Update `spell_effects.csv` for Blizzard DoT row: `periodicType=1` (periodic from aura), `amplitude=8`, trailing empty fields explicit
- Fix `spells.csv` Blizzard row: remove trailing garbage columns (already done)
- Update `spell-csv-loader` spec to document the full 12-column schema

## Capabilities

### Modified Capabilities

- `spell-csv-loader`: The schema definition for `spell_effects.csv` is incomplete (missing `periodicType`, `amplitude` columns) and the loader's field mapping for `school_damage` effects is incorrect. A delta spec will correct the schema and loader behavior.

## Impact

- `spelldef/loader.go`: `parseEffectRow` function — add `periodicType`/`amplitude` parsing
- `data/spell_effects.csv`: header update + all rows padded to 12 columns
- `data/spells.csv`: Blizzard row already fixed (9 columns, no trailing garbage)
- No API or runtime behavior changes (data correction only)
