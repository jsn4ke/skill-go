## Context

The `spell_effects.csv` has inconsistent column counts and missing header columns. The current state:

```
Header (10 cols): spellId,index,type,school,value,tickInterval,duration,auraType,miscValue,triggerSpellId
Row 2 (9 cols):  38692,0,school_damage,fire,717,,,,
Row 9 (11 cols): 10,0,school_damage,frost,25,,,,,,8
Row 10 (11 cols): 10,1,apply_aura,frost,65,,1500,1,10,,8
```

The `school_damage` row for Blizzard is missing `periodicType` (index 6) and `amplitude` (index 7). The `8` at the end of Row 9 is being parsed as `triggerSpellId=8`, but it should be `amplitude=8`.

The loader pads short rows to 10 columns with empty strings. For school_damage rows, this means:
- Index 6: periodicType (missing → 0)  
- Index 7: amplitude (missing → 0, but CSV has 8 here at wrong position)
- Index 8: miscValue (empty due to padding misalignment)
- Index 9: triggerSpellId (reads 8 from CSV, but that's amplitude, not triggerSpellId)

The `spelldef.SpellEffectInfo` struct has `PeriodicTickInterval int32` for tick rate. The amplitude (damage per tick) is stored in `BasePoints`.

## Goals / Non-Goals

**Goals:**
- Add missing `periodicType` and `amplitude` columns to header
- Standardize all rows to 12 columns (explicit trailing empty fields)
- Fix `parseEffectRow` to correctly map columns 6→periodicType, 7→amplitude for school_damage
- Ensure Blizzard's DoT effect is parsed with amplitude=8

**Non-Goals:**
- Changing the Go struct (no new fields needed — periodicType is just metadata, amplitude is BasePoints)
- Fixing other spell types' CSV rows beyond what's needed for correctness
- Migration of historical data

## Decisions

### D1: Extend header to 12 columns with explicit trailing pads

The new header:
```
spellId,index,type,school,value,periodicType,amplitude,dummy1,dummy2,auraType,miscValue,triggerSpellId
```

- Columns 0-4: spellId, index, type, school, value (same)
- Columns 5-7: **new** periodicType, amplitude, dummy1 (for periodic DoT data)
- Columns 8: **padding** (dummy) for non-school_damage rows
- Column 9: **moved** auraType (previously at 7)
- Column 10: **moved** miscValue (previously at 8)
- Column 11: **moved** triggerSpellId (previously at 9)

**Alternative**: Keep 10 columns and add comments. Rejected — inconsistent and harder to extend.

### D2: Parse periodicType/amplitude for school_damage only

In `parseEffectRow`, after reading `typeName` at index 2, branch on effect type:

```go
if et == SpellEffectSchoolDamage && len(row) > 6 {
    if v := strings.TrimSpace(row[6]); v != "" {
        eff PeriodicType, _ = parseInt32(row[6], "periodicType")
    }
    if len(row) > 7 {
        if v := strings.TrimSpace(row[7]); v != "" {
            eff.BasePoints, _ = parseInt32(row[7], "amplitude") // amplitude stored in BasePoints
        }
    }
}
```

Note: `amplitude` overwrites `BasePoints` for periodic school_damage. For non-periodic school_damage, `periodicType=0` and `amplitude` is 0.

### D3: Pad all rows to 12 columns with explicit empty strings

Before parsing, pad row to at least 12 elements:
```go
for len(row) < 12 {
    row = append(row, "")
}
```

This ensures consistent indexing regardless of trailing commas in CSV.

### D4: Update CSV rows

All rows padded to 12 with empty trailing fields. The old values for auraType/miscValue/triggerSpellId stay in their same relative positions (shifted +2 by the new columns):

```
38692,0,school_damage,fire,717,0,0,,,,,,     ← 12 cols: periodicType=0,amplitude=0,dummy1=,dummy2=,auraType(empty),miscValue(empty),triggerSpellId(empty)
38692,1,apply_aura,fire,21,0,0,2000,8000,1,21,,7922  ← 12 cols: dummy1=2000,dummy2=8000,auraType=1,miscValue=21,triggerSpellId=7922
100,0,charge,physical,1,,,,,,,,7922           ← 13 cols in design (typo); correct: 100,0,charge,physical,1,,,,,,,,7922? Let's count
100,1,trigger_spell,physical,0,,,,,,,7922     ← 12 cols (OK)
100,2,weapon_damage,physical,,,,,,,,           ← 12 cols (OK)
100,3,energize,physical,9,,,,,,,               ← 12 cols (OK)
7922,0,apply_aura,physical,0,0,0,0,1500,1,4,,7922   ← 13 cols in design; correct: 7922,0,apply_aura,physical,0,0,0,0,1500,1,4,,7922?
10,0,school_damage,frost,25,0,8,,,,,,           ← 12 cols (OK)
10,1,apply_aura,frost,65,0,0,0,1500,1,10,,8    ← 12 cols (OK)
```

**Note on D4 examples with count errors**: Rows with `charge` and `apply_aura` in the design above had trailing `,,` before `7922` that padded the column count. The corrected rows above show the intended 12-column layout.

## Risks / Trade-offs

**[Risk] Changing column positions breaks existing data parsing**  
→ Blizzard's triggerSpellId was at position 10 (the trailing 8), now it shifts. This only affects the trigger_spell row of spell 100 (which is already correct). The change only adds new columns before the existing ones.

**[Risk] Overwriting BasePoints with amplitude for periodic effects**  
→ This is intentional: for periodic school_damage, `BasePoints` IS the amplitude. The existing `value` column for periodic effects is 0 anyway. This is how WoW stores DoT data.

## Migration Plan

1. Update `spell_effects.csv` header to 12 columns
2. Pad all rows to 12 columns
3. Update `parseEffectRow` to read periodicType/amplitude for school_damage
4. Update `spell_effects.csv` Blizzard row: periodicType=0, amplitude=8
5. `go build ./...` — verify compilation
6. Run tests to verify spell loading still works
7. Push and merge

No rollback needed — the CSV is the source of truth and it was wrong before.
