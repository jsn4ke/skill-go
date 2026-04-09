# Spell Flow Tracing 使用指南

## 概述

`server/trace` 包提供结构化的法术流程追踪。每次施法生成唯一 FlowID，贯穿 spell → checkcast → effect → cooldown → aura → proc → targeting → script 全链路。替代了各包中散落的 `log.Printf` 调用。

## 输出格式

每行一个事件，格式如下：

```
[flow-00001] 17:28:10.031 spell.prepare | spell=1001(Fireball) targetCount=1 state=None
[flow-00001] 17:28:10.058 checkcast.passed | spell=1001(Fireball)
[flow-00001] 17:28:10.058 spell.state_change | spell=1001(Fireball) castTime_ms=0 from=None to=Preparing
[flow-00001] 17:28:10.059 spell.cast | spell=1001(Fireball) state=Preparing
[flow-00001] 17:28:10.059 cooldown.add_cooldown | spell=1001() duration_ms=6000 category=0
[flow-00001] 17:28:10.059 effect_hit.school_damage_hit | spell=1001(Fireball) target=Target Dummy damage=100 hp=9900
[flow-00001] 17:28:10.060 proc.triggered | spell=1001(Fireball) target=Target Dummy auraSpell=5003
[flow-00001] 17:28:10.060 spell.finish | spell=1001(Fireball) cancelled=false
```

### 字段含义

| 部分 | 说明 | 示例 |
|------|------|------|
| `[flow-XXXXX]` | FlowID，同一法术所有事件共享 | `flow-00001` |
| `17:28:10.031` | 时间戳（HH:MM:SS.mmm） | `17:28:10.031` |
| `spell.prepare` | Span.Event — 子系统 + 事件名 | `checkcast.failed` |
| `spell=1001(Fireball)` | 法术 ID 和名称 | `spell=1001(Fireball)` |
| `key=value` | 附加字段（因事件而异） | `damage=100`, `hp=9900` |

### Span（子系统）一览

| Span | 常量 | 触发时机 |
|------|------|----------|
| `spell` | `SpanSpell` | 生命周期：prepare, cast, cancel, finish, state_change, mana_consumed, mana_refunded |
| `checkcast` | `SpanCheckCast` | 验证链：passed, failed, recheck_passed, recheck_failed |
| `effect_launch` | `SpanEffectLaunch` | 效果发射：launch, school_damage_launch, heal_launch, energize_launch, weapon_damage_launch |
| `effect_hit` | `SpanEffectHit` | 效果命中：hit, school_damage_hit, heal_hit, energize_hit, weapon_damage_hit, apply_aura |
| `cooldown` | `SpanCooldown` | 冷却：add_cooldown, start_gcd, consume_charge |
| `aura` | `SpanAura` | 光环：applied, removed, refreshed, stacked, replacing |
| `proc` | `SpanProc` | 触发器：check, triggered |
| `targeting` | `SpanTargeting` | 目标选择：selected |
| `script` | `SpanScript` | 脚本钩子：hook_fired |

### Event（事件名）一览

| Span | Event | Fields | 说明 |
|------|-------|-------|------|
| spell | `prepare` | state, targetCount | 施法开始 |
| spell | `prepare_failed` | reason, error_code | 施法失败（reason: caster_dead / checkcast / script_prevented / no_mana） |
| spell | `cast` | state | 施法执行 |
| spell | `cast_failed` | reason | 施法执行阶段失败 |
| spell | `cancel` | state | 中断施法 |
| spell | `cancel_ignored` | state | 中断被忽略（不可中断状态） |
| spell | `finish` | cancelled | 施法完成 |
| spell | `state_change` | from, to, reason, castTime_ms, multiplier | 状态转换 |
| spell | `mana_consumed` | amount, remaining | 消耗法力 |
| spell | `mana_refunded` | amount, remaining | 退还法力 |
| spell | `delayed_hit_path` | delay_ms, targetCount | 进入延迟命中路径 |
| spell | `delayed_hit_arrived` | target, effectIndex | 延迟命中到达 |
| spell | `delayed_hit_skipped` | cancelled, targetAlive | 延迟命中被跳过 |
| spell | `all_delayed_hits_processed` | — | 所有延迟命中处理完毕 |
| checkcast | `passed` | — | 全部验证通过 |
| checkcast | `failed` | reason | 验证失败（reason: not_ready / out_of_range / too_close / silenced / disarmed / wrong_shapeshift / wrong_area / mounted） |
| checkcast | `recheck_passed` | — | 发射时距离复查通过 |
| checkcast | `recheck_failed` | target | 发射时距离复查失败 |
| effect_launch | `school_damage_launch` | base, school | 法术伤害发射 |
| effect_launch | `heal_launch` | base | 治疗发射 |
| effect_launch | `energize_launch` | amount, powerType | 能量恢复发射 |
| effect_launch | `weapon_damage_launch` | basePoints, weaponPercent | 武器伤害发射 |
| effect_launch | `trigger_spell_launch` | triggerSpellID | 触发法术发射 |
| effect_hit | `school_damage_hit` | target, damage, school, hp | 法术伤害命中 |
| effect_hit | `heal_hit` | target, amount, hp | 治疗命中 |
| effect_hit | `energize_hit` | target, amount, mana | 能量恢复命中 |
| effect_hit | `weapon_damage_hit` | target, totalDamage, basePoints, weaponDamage, weaponPercent, hp | 武器伤害命中 |
| effect_hit | `apply_aura` | target, auraType, duration | 应用光环 |
| effect_hit | `trigger_spell_hit` | target, triggerSpellID | 触发法术命中 |
| cooldown | `add_cooldown` | duration_ms, category | 添加冷却 |
| cooldown | `start_gcd` | category, duration_ms | 触发 GCD |
| cooldown | `consume_charge` | success, remaining | 消耗充能 |
| aura | `applied` | target, auraID, auraType, duration, stacks | 光环应用 |
| aura | `removed` | target, auraID, mode | 光环移除 |
| aura | `refreshed` | target, auraID | 光环刷新 |
| aura | `stacked` | target, auraID, stacks | 光环叠加 |
| aura | `replacing` | target, auraID, oldCaster, newCaster | 光环替换 |
| proc | `check` | target, auraID, procEvent, remaining | Proc 检查 |
| proc | `triggered` | target, auraSpell | Proc 触发 |
| targeting | `selected` | category, count | 目标选择完成 |
| script | `hook_fired` | hook, handlerCount | 脚本钩子触发 |
| spell | `effect_prevented` | effectIndex | 脚本阻止了效果 |
| spell | `channel_tick` | tick | 引导 tick |
| spell | `channel_stopped` | reason, total_ticks | 引导停止 |
| spell | `channel_elapsed` | total_ticks | 引导超时 |
| spell | `channel_finished` | — | 引导结束 |
| spell | `empower_stage_changed` | from, to, elapsed | 充能阶段变化 |
| spell | `empower_released` | stage, elapsed | 释放充能 |
| spell | `empower_release_failed` | reason, elapsed, min | 充能释放失败 |
| spell | `empower_release_ignored` | — | 非充能状态释放被忽略 |

---

## 典型流程事件序列

### 正常瞬发法术

```
[flow-00001] spell.prepare | ...
[flow-00001] checkcast.passed | ...
[flow-00001] spell.state_change | from=None to=Preparing
[flow-00001] spell.cast | state=Preparing
[flow-00001] spell.state_change | from=Preparing to=Launched
[flow-00001] effect_launch.launch | ...
[flow-00001] effect_hit.hit | target=Target
[flow-00001] spell.finish | cancelled=false
```

### 施法失败（沉默）

```
[flow-00002] spell.prepare | ...
[flow-00002] checkcast.failed | reason=silenced
[flow-00002] spell.prepare_failed | reason=checkcast error_code=2
```

### 施法失败（冷却中）

```
[flow-00003] spell.prepare | ...
[flow-00003] checkcast.failed | reason=not_ready
[flow-00003] spell.prepare_failed | reason=checkcast error_code=1
```

### 施法失败（超出射程）

```
[flow-00004] spell.prepare | ...
[flow-00004] checkcast.failed | reason=out_of_range target=Target dist=25.0 max=10.0
[flow-00004] spell.prepare_failed | reason=checkcast error_code=3
```

### 施法中断

```
[flow-00005] spell.prepare | ...
[flow-00005] checkcast.passed | ...
[flow-00005] spell.state_change | from=None to=Preparing castTime_ms=3000
[flow-00005] spell.mana_consumed | amount=50 remaining=150
[flow-00005] spell.cancel | state=Preparing
[flow-00005] spell.mana_refunded | amount=50 remaining=200
[flow-00005] spell.state_change | from=Preparing to=Finished reason=cancelled
```

### 延迟命中

```
[flow-00006] spell.prepare | ...
[flow-00006] checkcast.passed | ...
[flow-00006] spell.state_change | to=Preparing
[flow-00006] spell.cast | state=Preparing
[flow-00006] spell.state_change | to=Launched
[flow-00006] spell.delayed_hit_path | delay_ms=50 targetCount=1
[flow-00006] effect_launch.launch | ...
[flow-00006] spell.delayed_hit_arrived | target=Target effectIndex=0
[flow-00006] effect_hit.hit | target=Target
[flow-00006] spell.all_delayed_hits_processed | ...
[flow-00006] spell.finish | cancelled=false
```

### 命中触发 Proc

```
[flow-00007] spell.prepare | ...
[flow-00007] checkcast.passed | ...
[flow-00007] ... (正常施法流程) ...
[flow-00007] effect_hit.hit | target=Target
[flow-00007] proc.check | target=Target auraID=5003 procEvent=0 remaining=2
[flow-00007] proc.triggered | target=Target auraSpell=5003
[flow-00007] spell.finish | cancelled=false
```

---

## API 参考

### 生产代码：在业务包中记录事件

```go
import "skill-go/server/trace"

// 在任何函数中接收 *trace.Trace 参数，记录事件
t.Event(trace.SpanSpell, "prepare", spellID, spellName, map[string]interface{}{
    "state":       s.State.String(),
    "targetCount": len(s.Targets),
})

// t 为 nil 时自动跳过（向后兼容），不会 panic
```

### StdoutSink：默认输出到控制台

```go
// NewTrace() 自动附加 StdoutSink
t := trace.NewTrace()  // 事件同时写入 stdout 和内存
```

### 自定义 Sink

```go
// 实现 TraceSink 接口
type MySink struct{}

func (s *MySink) Write(e trace.FlowEvent) {
    // 自定义处理：写入文件、发送到远程等
}

// 创建 Trace 时使用自定义 Sink（不输出 stdout）
t := trace.NewTraceWithSinks(&MySink{})
```

### 多 Sink 组合

```go
t := trace.NewTrace()                // 自带 StdoutSink
recorder := trace.NewFlowRecorder()   // 用于测试
t.AddSink(recorder)                   // 同时输出到 recorder
```

---

## 测试中使用 FlowRecorder

### 基本模式

```go
func TestFlow_NormalCast(t *testing.T) {
    recorder := trace.NewFlowRecorder()
    tr := trace.NewTraceWithSinks(recorder)

    // ... 使用 tr 进行施法 ...

    // 断言事件存在
    if !recorder.HasEvent(trace.SpanSpell, "prepare") {
        t.Error("missing prepare event")
    }
    if !recorder.HasEvent(trace.SpanCheckCast, "passed") {
        t.Error("missing checkcast passed event")
    }

    // 不应该出现的事件
    if recorder.HasEvent(trace.SpanSpell, "cast_failed") {
        t.Error("unexpected cast_failed")
    }
}
```

### 按事件名过滤

```go
// 获取所有 prepare_failed 事件
failed := recorder.ByEvent("prepare_failed")
if len(failed) != 1 {
    t.Errorf("expected 1 prepare_failed, got %d", len(failed))
}
```

### 按 Span 过滤

```go
// 获取所有 effect_hit 事件
hits := recorder.BySpan(trace.SpanEffectHit)
for _, e := range hits {
    // 检查具体内容
}
```

### 统计数量

```go
// 统计某个 span 的事件数
count := recorder.Count(trace.SpanProc, "")  // 所有 proc 事件
hitCount := recorder.Count(trace.SpanEffectHit, "hit")  // effect_hit.hit
totalCount := recorder.Count("", "")  // 全部事件
```

### 检查失败原因

```go
// 检查是否有 "silenced" 原因的失败
foundSilenced := false
for _, e := range recorder.ByEvent("failed") {
    if reason, ok := e.Fields["reason"]; ok && reason == "silenced" {
        foundSilenced = true
        break
    }
}
```

### 清空重用

```go
recorder.Reset()  // 清空已捕获的事件
// recorder 可继续使用
```

### 在 SpellContext 中替换 Trace

```go
sc := spell.New(info.ID, info, caster, targets)
recorder := trace.NewFlowRecorder()
sc.Trace = trace.NewTraceWithSinks(recorder)
// 现在 sc.Prepare() 的所有事件都会被 recorder 捕获
```
