## ADDED Requirements

### Requirement: warrior-charge SHALL teleport caster to target and stun

当施法者对目标施放冲锋（spellId=100）时：
- 施法者 SHALL 瞬移至目标面前（目标面朝方向偏移 1 码）
- 目标 SHALL 被施加 1.5 秒眩晕效果（通过 trigger_spell 触发 spell=7922 Charge Stun → `UnitStateStunned`）
- 眩晕到期后 SHALL 自动移除控制效果
- 前端 SHALL 实时更新施法者的 3D 位置
