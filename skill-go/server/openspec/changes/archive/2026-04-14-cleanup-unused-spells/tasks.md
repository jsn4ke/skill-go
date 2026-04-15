## 1. CSV 数据清理

- [x] 1.1 从 `data/spells.csv` 删除冰霜新星行 (spellId=27088)
- [x] 1.2 从 `data/spells.csv` 删除奥术智慧行 (spellId=27126)
- [x] 1.3 从 `data/spell_effects.csv` 删除冰霜新星效果行 (spellId=27088, index=0 和 index=1)
- [x] 1.4 从 `data/spell_effects.csv` 删除奥术智慧效果行 (spellId=27126, index=0)

## 2. 测试更新

- [x] 2.1 更新 `spelldef/loader_test.go` 中引用了27088的测试用例，改用已实现的技能数据
- [x] 2.2 `go build ./...` 通过
- [x] 2.3 `go test ./...` 通过
