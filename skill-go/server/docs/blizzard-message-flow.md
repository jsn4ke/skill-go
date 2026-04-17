# 暴风雪 (Blizzard) 消息流全链路追踪

> 以 spellId=10 (暴风雪) 为例，从 API 请求到前端渲染的完整消息流。

---

## 1. 数据定义

### 1.1 spells.csv

```csv
spellId,name,school,castTime,cooldown,gcd,manaCost,powerType,rangeYards,isChanneled,channelDuration,tickInterval,missileSpeed
10,暴风雪,frost,0,0,1500,320,,30,1,8000,1000,0
```

关键字段:
- `castTime=0` → 瞬发，Prepare 后立刻 Cast
- `isChanneled=1` → 引导法术
- `channelDuration=8000` → 引导持续 8 秒
- `tickInterval=1000` → 每 1 秒一次 tick
- `missileSpeed=0` → 无弹道，tick 时立即 hit

### 1.2 spell_effects.csv

```csv
spellId,index,type,school,value,periodicType,amplitude,dummy1,dummy2,auraType,miscValue,triggerSpellId,radius
10,0,school_damage,frost,8,0,8,0,0,0,0,0,8
10,1,apply_aura,frost,65,0,1500,0,0,1,10,8,
```

- **Effect 0**: `school_damage` — 每tick 8 点冰霜伤害，radius=8 (AoE)
- **Effect 1**: `apply_aura` — 施加减速 aura，duration=1500ms，auraType=1，radius=8

---

## 2. 完整消息流时序图

```
前端                    API Layer              GameLoop              Spell Engine           Effect Handlers        Aura Manager
 │                        │                       │                       │                       │                     │
 │  POST /api/cast        │                       │                       │                       │                     │
 │  {spellID:10}          │                       │                       │                       │                     │
 │───────────────────────▶│                       │                       │                       │                     │
 │                        │  Command{cast}         │                       │                       │                     │
 │                        │──────────────────────▶│                       │                       │                     │
 │                        │                       │  New()                 │                       │                     │
 │                        │                       │  ctx.Prepare()         │                       │                     │
 │                        │                       │   ├─ checkcast ✅      │                       │                     │
 │                        │                       │   ├─ mana: -320        │                       │                     │
 │                        │                       │   ├─ castTime=0        │                       │                     │
 │                        │                       │   │  → auto Cast()     │                       │                     │
 │                        │                       │   ▼                   │                       │                     │
 │                        │                       │  ctx.Cast()           │                       │                     │
 │                        │                       │   ├─ range recheck ✅  │                       │                     │
 │                        │                       │   ├─ start_gcd (1.5s) │                       │                     │
 │                        │                       │   ├─ launch effects    │                       │                     │
 │                        │                       │   │  → school_damage    │                       │                     │
 │                        │                       │   │    launch           │                       │                     │
 │                        │                       │   │  → apply_aura       │                       │                     │
 │                        │                       │   │    launch           │                       │                     │
 │                        │                       │   ├─ IsChanneled=true   │                       │                     │
 │                        │                       │   ▼                   │                       │                     │
 │                        │                       │  startChannel()       │                       │                     │
 │                        │                       │   State: Channeling    │                       │                     │
 │                        │                       │                       │                     │                     │
 │                        │                       │  startChannelTicker()  │                       │                     │
 │                        │                       │   (goroutine:         │                       │                     │
 │                        │                       │    ticker=1s          │                       │                     │
 │                        │                       │    timer=8s)           │                       │                     │
 │                        │                       │                       │                     │                     │
 │                        │  reply: channeling     │                       │                     │                     │
 │  ◀─────────────────────│  {channelDuration:8000} │                       │                     │
 │                        │                       │                       │                     │                     │
 │  processCastResult()   │                       │                       │                     │                     │
 │  → show channel bar    │                       │                       │                     │                     │
 │                        │                       │                       │                     │                     │
 │                        │                       │                       │                     │                     │
 │    ═══ 1秒后 ═══        │                       │                       │                     │                     │
 │                        │                       │                       │                     │                     │
 │                        │                       │  channel_tick #1      │                       │                     │
 │                        │                       │  re-resolve AoE targets                     │                     │
 │                        │                       │                       │                     │                     │
 │                        │                       │  ExecuteChannelTick() │                       │                     │
 │                        │                       │   ├─ effect[0]:       │                       │                     │
 │                        │                       │   │  school_damage hit │                       │                     │
 │                        │                       │   │  → ResolveSpellHit│                       │                     │
 │                        │                       │   │  → CalcSpellDamage                      │                     │
 │                        │                       │   │  → TakeDamage(-8) │                       │                     │
 │                        │                       │   │  → trace hit event│                       │                     │
 │                        │                       │   │                    │                     │                     │
 │                        │                       │   └─ effect[1]:       │                       │                     │
 │                        │                       │      apply_aura hit  │                       │                     │
 │                        │                       │      → trace apply   │                       │                     │
 │                        │                       │      → ApplyAura()  │──────▶ ApplyAura()      │
 │                        │                       │                       │                     │  ├─ refresh or    │
 │                        │                       │                       │                     │  │  apply          │
 │                        │                       │                       │                     │  ├─ addEffect     │
 │                        │                       │                       │                     │  └─ trace applied  │
 │                        │                       │                       │                     │       ▲            │
 │                        │                       │                       │                     │       │            │
 │                        │                       │  SSE: channel_tick     │                     │       │            │
 │                        │                       │  SSE: school_damage_hit│                    │       │            │
 │                        │                       │  SSE: apply_aura       │                     │       │            │
 │                        │                       │  SSE: aura.applied     │                     │       │            │
 │  ◀── SSE ─────────────│──── SSE ──────────────│                       │                     │       │            │
 │                        │                       │                       │                     │                     │
 │  handlePeriodicDamage │                       │                       │                     │                     │
 │  → update HP bar       │                       │                       │                     │                     │
 │  → show damage number  │                       │                       │                     │                     │
 │  spawnAuraRing()       │                       │                       │                     │                     │
 │                        │                       │                       │                     │                     │
 │                        │                       │                       │                     │                     │
 │    ═══ 2秒后 ═══        │                       │                       │                     │                     │
 │                        │                       │  (同上 tick #2)       │                     │                     │
 │    ...                  │                       │                       │                     │                     │
 │    ═══ 8秒后 ═══        │                       │                       │                     │                     │
 │                        │                       │                       │                     │                     │
 │                        │                       │  channel_elapsed      │                       │                     │
 │                        │                       │  FinishChannel()      │                       │                     │
 │                        │                       │  State: Finished      │                       │                     │
 │                        │                       │  pending = nil        │                       │                     │
 │                        │                       │                       │                     │                     │
 │                        │                       │  SSE: channel_elapsed │                       │                     │
 │  ◀── SSE ─────────────│──── SSE ──────────────│                       │                     │                     │
 │                        │                       │                       │                     │                     │
 │  exitChannelingState() │                       │                       │                     │                     │
 │  → hide channel bar    │                       │                       │                     │                     │
 │                        │                       │                       │                     │                     │
 │                        │                       │                       │                     │                     │
 │    ═══ 1.5秒后(减速过期)═│═════════════════════│                       │                     │                     │
 │                        │                       │                       │                     │                     │
 │                        │                       │  (auraTicker)         │                       │                     │
 │                        │                       │  aura expired         │                       │                     │
 │                        │                       │  → RemoveAura(mode=expired) ──────────────────▶│
 │                        │                       │                       │                     │  removeEffect()
 │                        │                       │                       │                     │  recalcSpeedMod()
 │                        │                       │                       │                     │  trace: removed
 │                        │                       │                       │                     │       ▲
 │                        │                       │                       │                     │       │
 │                        │                       │  SSE: aura.removed    │                       │                     │
 │  ◀── SSE ─────────────│──── SSE ──────────────│                       │                     │                     │
 │                        │                       │                       │                     │                     │
 │  reconcileUnits()      │                       │                       │                     │                     │
 │  → speedMod 恢复为 1.0  │                       │                       │                     │                     │
```

---

## 3. 每秒 tick 的详细事件序列

每个 tick 产生以下事件 (以 tick #1 为例):

```
1. span=spell,    event=channel_tick
   fields: {tick:1, totalTicks:8, spellID:10, targets:N}

2. span=effect_hit, event=hit  (effect[0] school_damage)
   fields: {effectIndex:0, effectType:1, target:"Target Dummy"}

3. span=combat, event=resist_roll  (如果目标有抗性)
   fields: {resistance:100, rollFactor:0, avgReduction:...}

4. span=combat, event=spell_roll
   fields: {hitSpell:100, roll:45.42, result:"hit", missChance:0}

5. span=combat, event=resist_reduction
   fields: {school:2, resistance:100, ...}

6. span=combat, event=damage_calc
   fields: {baseDamage:8, finalDamage:8, school:2, variance:1.0}

7. span=effect_hit, event=school_damage_hit
   fields: {target:"Target Dummy", damage:8, hp:14992, school:2, result:0}

8. span=proc, event=check (检查目标身上是否有可触发的 aura)

9. span=effect_hit, event=hit  (effect[1] apply_aura)
   fields: {effectIndex:1, effectType:3, target:"Target Dummy"}

10. span=effect_hit, event=apply_aura
    fields: {target:"Target Dummy", auraType:1, duration:1500}

11. span=aura, event=applied  (或 refreshed，如果已有此 aura)
    fields: {auraID:10, target:"Target Dummy", auraType:1, duration:1500, stacks:1}

12. span=proc, event=check ×2 (对新 aura 的 proc 检查)
```

---

## 4. 代码阅读路径

按顺序阅读以下文件和行号，可以完整理解暴风雪的执行路径：

### Step 1: API 入口
```
api/server.go:489  handleCast()          — POST /api/cast 路由
api/server.go:502  发送 Command{cast}   — 转发给 GameLoop
```

### Step 2: GameLoop 处理 cast 命令
```
api/game_loop.go:335   handleCast()              — 解析请求，创建 SpellContext
api/game_loop.go:439   spell.New(...)            — 创建 spell context
api/game_loop.go:448   ctx.Prepare()             — 验证 + 扣蓝 + 设置读条
api/game_loop.go:459   检测 StateChanneling      — 暴风雪是 instant channel (castTime=0)
api/game_loop.go:470   startChannelTicker(ctx)   — 启动 goroutine 定时器
api/game_loop.go:482   reply channeling          — 返回引导响应给前端
```

### Step 3: Prepare() → 自动 Cast()
```
spell/spell.go:111   Prepare()                  — 验证链 + 扣蓝
spell/spell.go:216   castTime=0 → 自动 Cast()
spell/spell.go:224   Cast()                     — 范围检查 + CD + Launch
spell/spell.go:324   IsChanneled → startChannel()
```

### Step 4: Launch phase (Cast 内部)
```
spell/spell.go:304   遍历 effects → GetLaunchHandler()
effect/effect.go:65  RegisterDefaults: school_damage → handleSchoolDamageLaunch
effect/effect.go:75  handleSchoolDamageLaunch()   — trace: school_damage_launch
(apply_aura 没有 launch handler，跳过)
```

### Step 5: startChannel() → Channeling 状态
```
spell/spell.go:518   startChannel()             — State: Channeling
(不做其他事，ticker 由 game loop 管理)
```

### Step 6: Channel ticker (goroutine)
```
api/game_loop.go:650  startChannelTicker()       — 启动 goroutine
api/game_loop.go:659  ticker = time.NewTicker(1000ms)
api/game_loop.go:664  timer = time.NewTimer(8000ms)
api/game_loop.go:669  case <-ticker.C → 发送 channel_tick 命令
api/game_loop.go:680  case <-timer.C  → 发送 channel_elapsed 命令
```

### Step 7: handleChannelTick — 每个 tick 的处理
```
api/game_loop.go:689  handleChannelTick()        — 处理 tick 命令
api/game_loop.go:694  re-resolve AoE targets    — 地面 AoE 重新选目标
api/game_loop.go:709  trace: channel_tick
api/game_loop.go:717  ctx.ExecuteChannelTick()   — 执行 hit phase
```

### Step 8: ExecuteChannelTick — 每个 tick 的 hit phase
```
spell/spell.go:716   ExecuteChannelTick()       — 遍历所有 effects + targets
spell/spell.go:734   GetHitHandler(SchoolDamage) → handler()
spell/spell.go:738   trace: effect_hit.hit (每个 target)
spell/spell.go:743   handler(ctx, eff, target)    — 调用具体 effect handler
```

### Step 9: SchoolDamage hit handler
```
effect/effect.go:82   handleSchoolDamageHit()      — 伤害处理
effect/effect.go:87   combat.ResolveSpellHit()     — 命中判定
effect/effect.go:98   combat.CalcSpellDamage()     — 伤害计算
effect/effect.go:105  target.TakeDamage(damage)    — 扣血
effect/effect.go:106  trace: school_damage_hit      — 伤害事件
```

### Step 10: ApplyAura hit handler
```
effect/effect.go:19   RegisterHit(ApplyAura)       — 注册在 effects.go
effects.go:19          apply_aura hit handler       — trace: apply_aura
effects.go:25          makeAuraHandler()             — 构造 aura 应用函数
(通过 AuraHandler 回调)
api/server.go:324      makeAuraHandler()            — 构造 Aura 对象并 ApplyAura
aura/manager.go:27     ApplyAura()                  — 应用/刷新/叠层 aura
aura/manager.go:80     trace: aura.applied           — aura 事件
```

### Step 11: Channel elapsed — 引导结束
```
api/game_loop.go:727  handleChannelElapsed()       — 8 秒到
api/game_loop.go:732  ctx.FinishChannel()           — State: Finished
spell/spell.go:759     FinishChannel()              — 仅设置 State=Finished
```

### Step 12: Aura 过期 (独立于 spell)
```
api/game_loop.go:261  auraTicker()                 — 每 200ms 检查 aura
api/game_loop.go:999  elapsed >= duration           — 检测过期
api/game_loop.go:1002 mgr.RemoveAura(mode=expired) — 移除 aura
aura/manager.go:107    trace: aura.removed           — 移除事件
```

### Step 13: SSE 推送到前端
```
trace/stream.go        StreamHub.Subscribe()       — SSE 订阅
trace/stream.go        Hub.Publish(event)           — 每个 trace.Event 都推送
```

### Step 14: 前端处理
```
web/app.js:110         SSE subscriber              — 订阅 /api/trace/stream
web/app.js:112         periodic_damage → handlePeriodicDamage()
web/app.js:116         aura.applied/removed/refreshed → reconcileUnits()
web/app.js:138         channel_elapsed → exitChannelingState()

web/timeline.js:       processEvents()            — VFX 处理
web/timeline.js:70      school_damage_hit           — 伤害数字
web/timeline.js:165     aura.applied                — aura 环形特效
web/timeline.js:83      isChanneledAoE check        — 跳过弹道 VFX
```

---

## 5. 关键设计点

### 5.1 Instant Channel 的特殊路径

暴风雪的 `castTime=0` 导致 Prepare() 内部自动调用 Cast()：

```
普通读条+引导 (读条施法):
  Prepare → [等3.5s] → Cast → startChannel → ticker

瞬发引导 (暴风雪):
  Prepare → Cast(auto) → startChannel → ticker
                  ↑
          castTime=0 时立刻调用
```

game_loop 在 `handleCast()` 的 L459 检测 `ctx.State == spell.StateChanneling` 来识别这个情况。

### 5.2 Tick 时的 AoE 目标重新选取

每次 tick 都会重新执行 targeting.Select()，这意味着：
- 引导期间走进 AoE 区域的新目标会被命中
- 离开区域的目标不再被命中
- 目标死亡后不会被命中 (handler 检查 target.IsAlive())

### 5.3 auraTick 独立于 channelTick

暴风雪的减速 aura 不是通过 channel tick 处理的：
- **伤害**: channel tick → ExecuteChannelTick → school_damage hit handler
- **减速**: channel tick → ExecuteChannelTick → apply_aura hit handler → ApplyAura()
- **aura 过期**: auraTicker (独立 200ms 定时器) 检测并移除

减速 aura 的 periodic 效果（如果有 DoT 类效果）由 auraTicker 的 periodic_damage 处理，不是 channel tick。

### 5.4 双 ticker 架构

```
startChannelTicker()  → channel_tick   (每 1s, 执行 hit phase)
auraTicker()           → aura_update    (每 200ms, 检查过期/DoT)
```

两个 goroutine 独立运行，都通过 `SendAsync()` 向 game loop 发送命令，确保串行执行。

---

## 6. 完整事件时间线

```
T=0ms      POST /api/cast {spellID:10}
           ├─ spell.prepare
           ├─ spell.mana_consumed {amount:320}
           ├─ spell.state_change {from:None, to:Preparing, castTime_ms:0}
           │  castTime=0 → auto Cast()
           ├─ spell.cast
           ├─ spell.state_change {from:Preparing, to:Launched}
           ├─ cooldown.start_gcd {duration_ms:1500}
           ├─ effect_launch.launch {effectIndex:0}  (school_damage)
           ├─ effect_launch.school_damage_launch {base:8, school:2}
           ├─ effect_launch.launch {effectIndex:1}  (apply_aura)
           ├─ spell.state_change {from:Launched, to:Channeling, duration:8s, interval:1s}
           └─ reply: channeling {channelDuration:8000}

T=1000ms   channel_tick #1
           ├─ spell.channel_tick {tick:1, totalTicks:8}
           ├─ effect_hit.hit {effectIndex:0, target:"Target Dummy"}
           ├─ combat.spell_roll {result:"hit", roll:45}
           ├─ combat.damage_calc {baseDamage:8, finalDamage:8}
           ├─ effect_hit.school_damage_hit {damage:8, hp:14992}
           ├─ effect_hit.hit {effectIndex:1, target:"Target Dummy"}
           ├─ effect_hit.apply_aura {auraType:1, duration:1500}
           ├─ aura.applied {auraID:10, duration:1500, stacks:1}
           └─ proc.check ×2

T=1200ms   aura_update (减速 aura 存在，未过期)

T=1500ms   GCD 结束 (无事件通知)

T=2000ms   channel_tick #2 (同上流程，叠加 damage)
           ├─ aura.refreshed {duration:1500} (刷新减速)

T=2500ms   aura_update

T=3000ms   channel_tick #3
           ...
T=3500ms   aura_update
T=4000ms   channel_tick #4
           ...
T=7000ms   channel_tick #7
           ...
T=8000ms   channel_tick #8
           ├─ spell.channel_tick {tick:8, totalTicks:8}
           └─ ... (最后一次 tick)

T=8000ms   channel_elapsed
           ├─ spell.channel_elapsed {total_ticks:8}
           └─ State: Finished

T=8200ms   aura_update
           └─ (减速 aura 过期，因为最后 refresh 在 T=7000ms, duration=1500ms)
           └─ aura.removed {auraID:10, mode:"expired"}
```
