# 从零用 Go 写一个交易所 —— TDD 教程大纲

> 给一个**会 Java、刚学完 Go 基础**、跟 Anthony GG 的交易所课觉得吃力的人量身设计。
> 学习方式:**我给失败的测试 + 空壳签名,你写实现让它变绿,我审查纠错。**
> 主线:**不抄他的代码,自己从零重写一遍。** 推进方式:**A 方案 —— 分层 · 正确性优先。**

---

## 0. 为什么 Anthony GG 那套你看着费劲(不全是你的问题)

调研自他的仓库 `github.com/anthdm/crypto-exchange` 与 `go.mod`:

| 他的做法 | 为什么卡你 |
|---|---|
| 撮合 + Echo + go-ethereum + Ganache + 并发,全揉在一起 live coding | 同时消化 4 个难点,哪层红了都不知道 |
| 以太坊结算用 **Ganache**(已停止维护,他 repo 唯一 open issue 就是这个);`go-ethereum` 签名 API 也变了 | 你照着敲**必然报错** |
| 早期代码**不是线程安全**的,后面才补 `mutex`("fixed a lot of race conditions") | 跟着 live 写会撞 race panic |
| 价格用 `float64` | `0.1+0.2 != 0.3`,成交价飘、map key 对不上 |
| 用 OOP 思路写 Go | 你的 Java 直觉(到处 `new`、引用语义、`synchronized`)被反复咬 |

**本教程的策略:逐个拆开这些纠缠,每次只让你面对一个硬概念。**

---

## 1. 推进方式:A 方案(分层 · 正确性优先)

严格按依赖顺序自底向上,**撮合核心没绿之前不碰一行 goroutine / channel / HTTP**:

```
M0 领域类型 ──▶ M1 订单簿 ──▶ M2 撮合引擎(纯函数·单线程·确定性·测试全绿)
                                   │
                                   ▼
                       M3 并发(单写协程)+ HTTP/JSON API
                                   │
                                   ▼
                            M4 做市商 bot
```

**为什么这个顺序**:M2 撮合核心是纯函数、确定性的,所以 M3 的所有并发/API 课都只是"在已经绿了的测试背后做重构",永远不会变成调试噩梦——这跟 GG 那种全揉一起的 live build 正好相反。而且这正是 GG 的真实搭建顺序(他 EP1 就是撮合引擎),所以每个里程碑你都能拿你的干净版去 diff 他的代码。

---

## 2. 最终成品

单币对(ETH/USD)**纯内存限价撮合交易所**,约 1500–2500 行,全程测试先行。

逐模块产出可运行产物:打印盘口的 CLI(M1)→ stdin 成交 REPL(M2)→ 能 `curl` 的 JSON 交易所(M3)→ 持续报价的做市 bot(M4)。

**总工作量约 12–16 天**(按你节奏,不赶):M0 ~0.5–1 天 / M1 ~2–3 天 / M2 ~3–4 天 / M3 ~3–4 天 / M4 ~3–4 天。

---

## 3. 故意不做的(YAGNI,避开 GG 的坑)

- ❌ 真实区块链/以太坊结算(Ganache 已死)——如想要"结算"概念,用内存余额账本代替
- ❌ 数据库/持久化(簿就是纯内存,重启清空,这是**有意为之**)
- ❌ 登录/鉴权/用户体系(自成交策略只需一个 `OwnerID` 字段,不是鉴权系统)
- ❌ 多币对/撮合路由(锁死单币对 ETH/USD;一引擎一币种的横向扩展只在 M3 提一句)
- ❌ WebSocket 实时行情推送(`GET /book` 轮询足够看到效果)
- ❌ 订单簿性能版(红黑树/跳表/堆)——留作最后"接口不变只换内部"的重构练习
- ❌ Avellaneda-Stoikov 做市——留作 M4 可选 Stage 3,只讲直觉不堆数学

---

## 4. 每节课怎么上(TDD 闭环)

1. 我给你一个**失败的测试文件**(`xxx_test.go`)+ 一个**空壳函数签名** + 一句"这步的 aha 是什么"
2. 你写实现让它变绿,把代码贴回来
3. 我**审查**:对了讲清它为什么对、引出下一步;错了精准指出是哪个 Java→Go 陷阱咬了你
4. 每个模块结束有"**你现在能做 X 了**"通关点 + 一个能跑的产物

> 讲解用中文,代码/标识符/注释用英文(行业惯例,也方便对照 GG)。

---

## 5. 课程模块详表

### M0 · Go + TDD 脚手架与领域类型 　`估时 0.5–1 天`

**目标**:立起 Go module,吃透红/绿/重构循环和表驱动测试,定义全交易所赖以构建的值类型(`Side`/`Order`/`Trade`)——并在写任何逻辑前把"**钱是整数**""**错误是返回值**"立为铁律。

| 步骤 | 概念 | 我给的测试 | 你实现 | aha |
|---|---|---|---|---|
| 0.1 | 红/绿循环 + `go test` 机制 | 表驱动 `TestCrosses`:`crosses(buy, sell int64) bool`,用例 {买100卖100→true}{买101卖100→true}{买99卖100→false} | `go mod init exchange`;写 `matching/crosses.go` 里的 `crosses`,跑绿 | `go test` 极简:一个 `TestX(t *testing.T)` 函数,无框架无注解。而这一行 `buy >= sell` 就是整个撮合的心脏 |
| 0.2 | 值类型 + `iota` 枚举 | `TestSideString`:`Buy.String()=="Buy"` 等;构造 `Order{...}` 断言字段 | `type Side int` + `const (Buy Side = iota; Sell)` + `String()` 方法 + `Order` 结构体(`int64` 价/量 + `uint64` Seq) | 没有 enum class,`iota` 只是计数;结构体字面量就是整个对象,不需要构造仪式 |
| 0.3 | 钱是整数(铁律一) | 注释里展示 `0.1+0.2 != 0.3`,再 `TestOrderUsesInt` 断言 `int64` 运算精确 | 无新代码,确认 Price/Qty 是 `int64` | 出于 Java 习惯用 `float64/BigDecimal`,成交会悄悄飘、map key 对不上。tick/分用 `int64`,没商量 |
| 0.4 | 错误是返回值,不是异常 | `TestNewOrder` 表:{Qty:0→ErrInvalidQty}{限价 Price:0→ErrInvalidPrice}{合法→nil},用 `errors.Is` | `func NewOrder(...) (Order, error)` + 哨兵错误,校验后返回 | 没有 try/catch。返回 `(Order, error)`,调用方 `if err != nil`。`panic` 只留给程序员 bug,不是"订单被拒" |

**通关点**:你能搭 Go module、写表驱动测试、用结构体+`iota`+返错构造器建模,并能解释为什么价格是 `int64`。
**对应 GG**:他的 `Order`/`Side`。他用 `float64`,我们有意改 `int64`(M2 会指出他这选择正是 #1 隐蔽 bug 之源)。

---

### M1 · 订单簿数据结构 　`估时 2–3 天`

**目标**:建一个正确的单线程限价簿:价格档位=FIFO 队列、双边 `map`、O(1) 撤单(`locate` 索引)、最优买卖价——每个 Java→Go 集合陷阱都在它咬你的那一步现身。

| 步骤 | 概念 | 我给的测试 | 你实现 | aha |
|---|---|---|---|---|
| 1.1 | 单档位 FIFO | `TestLevelFIFO`:Add(A),Add(B);Peek==A,PopFront==A,再 Peek==B;空档 PopFront 返回 ok=false | `PriceLevel{Price int64; Orders []uint64}`;Add 追加、PopFront/Peek 取 `Orders[0]` | `[]uint64` 就是队列——尾部 append、读 index 0,不需要 LinkedList。存 ID(非 `*Order`)保证单一数据源 |
| 1.2 | 构造器 + nil-map 陷阱 | `TestNewBookWritable`:`NewBook()` 后 AddOrder 不 panic(注释里展示 `var b Book; b.bids[100]=...` 会 panic) | `Book` 三个 map;`NewBook() *Book` 把每个 map `make()` 出来 | 跟 Java HashMap 字段不同,Go 零值 map **可读但写就 panic**。每个 map 都得 `make()`,这是头号"为啥崩了"惊吓 |
| 1.3 | AddOrder + 指针 vs 值接收者 | `TestAddPreservesArrival`:在 100 加两个买单,断言 `bids[100].Orders==[first,second]`(故意给个值接收者版演示 append 消失) | AddOrder 把 Order 存进 orders,取或建 `*PriceLevel`,append ID,设 `locate[id]=price`;档位以 `*PriceLevel` 存在 map 里,方法用指针接收者 | 若 PriceLevel 以值存,append 改的是副本、会消失——**Java→Go 最大冲击**,因为 Java 对象永远是引用。`*PriceLevel`+指针接收者解决 |
| 1.4 | BestBid/BestAsk 不信任 map 顺序 | `TestBestPrice`:空簿→ok=false;买 100,102,101→BestBid==102;卖 105,103→BestAsk==103;再跑 50 次断言稳定 | 扫 map key 求 max(买)/min(卖),返回 `(price, ok)` | `range` map 给的是**随机顺序**;Java 来的人指望 TreeMap 那种有序会得到飘忽的"最优价"。必须显式求 min/max |
| 1.5 | CancelOrder + 切片删除别名 + 空档清理 | `TestCancel`:档位 [A,B,C];撤中间 B→[A,C] 有序;撤 A 再撤 C→档位从 map 删除、BestBid 不再看到该价;`TestCancelUnknown`→ErrNotFound | 用 locate 找价;`copy(s[i:], s[i+1:]); s[len-1]=0; s=s[:len-1]`;从 orders/locate 删;空档从 map 删 | 朴素 `append(s[:i], s[i+1:]...)` 会别名底层数组、破坏被持有的切片;忘删空档会留幽灵价、坏掉 BestBid 还泄漏内存 |
| 1.6 | ModifyOrder 与价格时间优先语义 | `TestModify` 表:减量→Qty 降、保持队首/队中位置;增量→重排到队**尾**;改价→移到新档队尾 | 纯减量则原地改 Qty;否则 cancel+re-add(新 Seq、档尾) | 交易所公平规则:只有减量能保住队位;任何可能插队的改动(改价/增量)都要付出时间优先——把市场公平规则写成代码 |

**通关点**:你能增/删/改/查一个正确的价格时间优先簿,并能按"它在哪步咬了你"解释每个 Java→Go 集合陷阱。
**产物**:能从小 `main()` 打印盘口深度的 CLI。
**对应 GG**:他的 `Limit`=我们的 `PriceLevel`;`Orderbook{AskLimits/BidLimits map[float64]*Limit, Orders map[int64]*Order}`=我们的 `Book{bids/asks + orders + locate}`。关键差异:我们用 `int64` 不是 `float64`,且多了个 O(1) `locate` 索引(他没有,所以他撤单是扫描)。

---

### M2 · 撮合引擎 　`估时 3–4 天`

**目标**:把静态簿变成确定性撮合引擎:一个 `Submit(Order) -> []Trade`,按价格时间优先撮合,处理部分成交、多档扫单、限价/市价剩余规则、自成交策略、校验——全单线程、彻底测试。

| 步骤 | 概念 | 我给的测试 | 你实现 | aha |
|---|---|---|---|---|
| 2.1 | 单笔全成交于 maker 价 | 挂 Sell(100,10);Submit Buy(限价≥100,10)→恰一笔 Trade{Price:100,Qty:10},成交后簿空 | Submit:找最优对手档,撮合队首,按 maker 价出 Trade,双方减量,移除已成 maker | 成交价是 100(挂单价),不是买家限价。"taker 价格改善"="成交按 maker 价",从循环里自然掉出来 |
| 2.2 | 双向部分成交 | taker 更大:Sell(100,10),Buy(100,15)→Trade 10,买方剩 5 挂到买 100。maker 更大:Sell(100,10),Buy(100,4)→Trade 4,卖方剩 6,不新挂 | `fill = min(taker.Remaining, maker.Remaining)`;双方减量;只有**限价 taker** 的余量挂上 | `min()` 就是部分成交的全部故事。谁的余量挂取决于谁量大;限价 taker 的余量变成挂单=簿在自愈 |
| 2.3 | 多档扫单(跨档价格优先) | 挂 Sell(100,5)、Sell(101,5);Buy(限价101,8)→先 Trade(100,5) 再 Trade(101,3),顺序固定;卖 101 剩 2 | 把撮合包进循环:吃掉最优档,空了就删、转下一档,直到无量或不再交叉 | 每轮都重新取最优档来保证价格优先;循环卫语(还有量 且 交叉)防止空侧 nil 解引用/死循环 |
| 2.4 | 档内 FIFO 时间优先 | 挂 SellA(100,5,seq1) 再 SellB(100,5,seq2);Buy(100,5)→只跟 A 成交,B 不动 | 从档位**队首**(最老 Seq)撮合 | 永远取队首,时间优先就免费了。用 Seq 计数器(非 `time.Now()`)让"谁先到"确定且不撞 |
| 2.5 | 不交叉的限价挂上;零成交 | 最优卖 100(或无卖);Buy(限价99,5)→零 Trade,买单挂 99、Seq 保留 | 无交叉量则跳过循环、直接挂限价 | 挂单就是"撮合没产出任何东西,于是 AddOrder";交叉检查是激进/被动之间唯一的门 |
| 2.6 | 市价单剩余规则 + 空簿 | 市价 Buy(5) 进空簿→零 Trade、余 5 **返回**为未成交、不挂;市价 Buy(10) vs 仅 Sell(100,3)→Trade(100,3)、报未成交 7、不挂 | 加 `Type` 限价/市价;市价永不挂——在结果里返回未成交余量 | 订单类型的差别就一件事:剩余怎么处理。显式 `Type` 字段让"无流动性"响亮地暴露,而不是悄悄挂成哨兵价 bug |
| 2.7 | 校验 + 自成交策略 + 守恒 | Submit Buy(Qty0)/限价 Buy(Price0)→拒绝、簿不变;自成交:X 挂 Sell(100,5),X 又 Submit Buy(100,5)→所选策略(撤新单:无 Trade、一边撤);属性测试:随机 submit→成交量守恒、不凭空增减;同输入两遍→同 `[]Trade` | 卫语返错;一个明确的自成交策略检查;(可选)属性/模糊测试 | 确定性是其他所有测试暗中依赖的属性——同序列跑两遍得到逐字节相同的成交,正是你"从没误用 map range"的证明 |

**通关点**:你能提交限价/市价单并得到正确、确定的成交(部分成交、多档扫单、FIFO、剩余处理),且属性测试证明量从不凭空增减。
**产物**:从 stdin 读单、打印成交+簿态的 REPL——你第一个端到端"交易所",还没 HTTP。
**对应 GG**:**这就是他的 EP1 撮合引擎**。他的 `PlaceMarketOrder` 走簿产出 `[]Match`=我们的 `Submit` 产出 `[]Trade`;他的 `Match`=我们的 `Trade`。差异:我们从一开始就单线程、确定性(他早期 commit **非**线程安全;他后来"加 mutex/修 race"那些 commit,补的正是我们 M3 用设计而非锁规避掉的并发)。

---

### M3 · 并发 + HTTP/JSON API 　`估时 3–4 天`

**目标**:用**单写 actor 模式**把引擎安全暴露到 HTTP:恰好一个 goroutine 独占簿,所有请求经 command channel + 每请求 reply channel,读返回不可变快照,优雅关停不漏 goroutine——以 `go test -race` 作验收门。**簿上不加 mutex。**

| 步骤 | 概念 | 我给的测试 | 你实现 | aha |
|---|---|---|---|---|
| 3.1 | 用 Command + Run 循环包住纯引擎 | `TestEngineRun`:goroutine 里启 `Run(ctx)`;发 `Command{Op:PlaceOrder, Order, Reply:make(chan Result,1)}`;断言 Result.Trades 符合 M2 预期 | `Command`/`Result` 结构体;`Engine{cmds chan Command}`;`Run(ctx)` 用 `select` 监听 ctx.Done() 和 cmds,分派给已有引擎方法 | 撮合代码原封不动——并发只是薄信封。一个 goroutine 抽干 channel,就是 Java 里一个单线程 worker 在消费 BlockingQueue |
| 3.2 | 队列之上的同步客户端方法 + 取消 | `TestPlaceContextCancelled`:用已取消 ctx 调 `Place(ctx, o)`→及时返回 ctx.Err()、不挂死;`TestPlaceRoundtrip`:正常 ctx→返回成交 | `Place(ctx, o)`:`make(chan Result,1)`;`select` 发 cmds vs ctx.Done();再 `select` 收 reply vs ctx.Done() | 同步调用=在每调用专属 channel 上"发了再等回"。buffered(1) 让引擎即使你放弃了也能回复并继续——簿不冻 |
| 3.3 | race detector 下的并发证明 | `TestConcurrent`:100 个 goroutine 向一个 Engine 交错 place/cancel;断言守恒、不 panic;CI 门:`go test -race` 必须绿 | 设计对的话无新代码——本测试证明单写不变式;红了说明你哪里加了共享状态 | `go test` 默认**什么 race 都抓不到**;`-race` 是另一种构建。绿的 `-race` 才是"一个 owner、无锁"真消除了数据竞争的证明 |
| 3.4 | 不可变快照读(无别名) | `TestSnapshotIsolation`:取 S1;再下单;取 S2;断言 S1 不变、S2 反映新单(不共享底层数组) | `book.snapshot()` 返回深拷贝的 `BookSnapshot` 值;GetSnapshot op 返回它 | 跨 goroutine 返回活簿会 race;深拷贝让 JSON 编码在 handler goroutine 上零锁运行。快照必须**拷贝**不是别名 |
| 3.5 | HTTP 适配 handler(薄、无状态) | `httptest` 集成:POST /orders→201+成交 JSON;GET /book→快照 JSON;DELETE /orders/{id}→200,二次删→404 | `net/http`(或 Echo)handler:解 JSON、调 `e.Place/Cancel/Snapshot(r.Context())`、编码 Result。无锁无共享态 | `net/http` 已给每请求一个 goroutine——你不用加 WaitGroup。所有串行化都在引擎 channel,不在 handler |
| 3.6 | 优雅关停 + goroutine 泄漏检查 | `TestMain` 包 `goleak.VerifyTestMain`;测试启服务、发流量、`srv.Shutdown(ctx)` 取消引擎 ctx,断言 Run 返回、零 goroutine 泄漏 | 把服务关停接到取消引擎 context;确保 Run 在 ctx.Done() 时返回 | 停引擎靠取消它的 context,绝不靠 close cmds(close 后再发会 panic)。泄漏的 goroutine=Java 里没归池的线程,goleak 抓得到 |

**通关点**:你能把引擎用 HTTP 服务、并发正确(靠设计)、可证无 race、返回一致快照、干净关停无泄漏。
**产物**:跑起来的 HTTP 服务:`POST /orders`、`DELETE /orders/{id}`、`GET /book`——能 `curl` 的活交易所。
**对应 GG**:他的 JSON API(Echo)+ 客户端 SDK=我们的 HTTP 层 + 薄客户端。关键:他的硬化 commit("加 mutex""修 race")是同一问题的**锁方案**;我们把 mutex 作为命名备选讲清,然后选单写 channel——你两种都懂,也看懂他为啥在他那共享态设计里需要 mutex |

---

### M4 · 做市商 bot 　`估时 3–4 天(可选 Stage 3 A-S 再加 ~1 天)`

**目标**:做市 bot 作为独立进程,仅通过一个小的**消费方定义接口**驱动交易所:报固定价差→演进到库存偏移(带硬仓位上限+自穿越保护),全在 `time.Ticker`+`select` 循环上对成交反应——`Quotes()` 纯逻辑保持可测,并发保持地道。

| 步骤 | 概念 | 我给的测试 | 你实现 | aha |
|---|---|---|---|---|
| 4.1 | 消费方定义接口 + 手写 fake | 给好的 `fakeExchange` 记录下/撤的单;`TestFakeImplementsExchange` 仅当它满足 `Exchange` 才编译过 | 在 **bot 这侧**定义 `Exchange interface{ PlaceLimit(...)(int64,error); Cancel(id)error; BestBid()(...); BestAsk()(...) }`;让 fake 隐式满足 | 你从不写 `implements`。在**消费处**定义小接口,任何有这些方法的类型自动满足。10 行 fake 顶替整个 mock 框架 |
| 4.2 | 纯固定价差报价 | `TestQuotesFixedSpread`:mid=100,spreadBps=20→bid==99.90,ask==100.10,差 0.20 | `Quotes(mid)`:`half=spreadBps/2/10000`;`bid=mid*(1-half)`;`ask=mid*(1+half)` | 策略是输入的**纯函数**——无交易所、无 IO。所以一行就能测,也让后面换策略只动一个签名 |
| 4.3 | Step() 下单,然后撤旧换新 | `TestStepPlacesTwo`:Step(100)→fake 上恰一买@bid+一卖@ask;`TestStepCancelsBeforeReplace`:两次 Step→第一对被撤,只剩第二对 | `Step(mid)`:撤上轮活单、算 Quotes、下新买+卖、记进 live map | 忘了"撤旧再换"会在多档留一扇全会成交的挂单——悄悄冲破仓位上限的路 |
| 4.4 | 库存追踪 via OnFill | `TestOnFill`:Buy 成交 5→inventory+=5;Sell 成交 5→inventory-=5 | `inventory` 字段;`OnFill(side, size)` 调整它 | bot 必须知道自己持有多少;inventory 是整个风险故事围绕的状态 |
| 4.5 | 库存偏移(Stage 2,同签名) | `TestSkew`:inventory>0→center<mid、ask 比 bid 更靠近 mid(倾向卖);inventory==0→与 Stage-1 对称报价一致;inventory<0→center>mid | `Quotes` 里:`skew=coef*inventory/maxInventory`;`center=mid*(1-skew)`;在 center 周围套价差 | 先偏移 **center** 再套价差就是整个库存模型——持多就倾向卖、把自己拉回平。纯函数重构,Stage-1 测试(inventory=0)保持绿 |
| 4.6 | 硬风险上限 + 自穿越保护 | `TestMaxInventory`:到 +maxInventory 时 Step 不发新买(只卖);`TestSelfCross`:会算出 bid≥ask 的参数→不发交叉单,断言 bid<ask | `|inventory|>=maxInventory` 时只报缩仓那侧;任何 bid>=ask 的报价 clamp/skip | 风险控制是**核心算法**不是装饰。naive bot 的经典死法:趋势市一直填一侧——上限和自穿越保护让它活命 |
| 4.7 | 驱动循环:ticker + select over fills | `TestLoop`(`go test -race` 下):喂合成 tick 与 fill;断言 Step/OnFill 被调、inventory 终态一致无 race | `for { select { case <-ticker.C: mm.Step(ref()); case f:=<-fills: mm.OnFill(f.Side,f.Size) } }`——循环保持薄,逻辑留在已测纯方法 | 用 channel+select(非 mutex)把成交传给报价循环。循环薄=要紧的都已单测过,循环只是管线,`-race` 证明它 |

**通关点**:你能跑一个做市 bot:双边报价、带偏移管理库存、守硬仓位上限、永不自穿越、并发循环对成交反应且可证无 race。
**产物**:持续围绕 mid 双边报价、按间隔刷新的 bot——在 `GET /book` 里看到活的双边流动性。
**对应 GG**:他的 `mm/` 包 + 模拟下单器。他的 bot=围绕参考价的固定 `PriceOffset`、`MinSpread` ~20bps、`OrderSize` 10、`MakeInterval` 刷新——正是我们 Stage 1。我们再往前加库存偏移(Stage 2)和命名的 A-S 选项(Stage 3,他没讲),并通过手写 fake 接口驱动,让策略可单测(他 live 写的版本不可)。

---

## 6. Java → Go 速查表(贯穿全程,在咬你的那步细讲)

| Java 直觉 | Go 做法 | 何处咬你 |
|---|---|---|
| 默认引用 | **默认拷贝**:`o2 := o1` 拷贝整个 Order;要改簿里的单必须持 `*Order`+指针接收者 | M1.3 |
| HashMap 字段可直接用 | 零值 map **可读、写就 panic**;每个 map 都 `make()` | M1.2 |
| TreeMap 有序 | map 永远无序;`range` 顺序随机,求最优价要显式 min/max | M1.4 |
| ArrayList 拷贝 | 切片是 `{ptr,len,cap}` 视图;reslice 会别名;删元素用 copy+截断+置零 | M1.5 |
| 异常 try/catch | `(T, error)` 返回值 + `errors.Is` + `%w`;`panic` 只给真正的不变式破坏 | M0.4, M2.7 |
| enum class | `type Side int` + `iota`;无方法重载,变体显式命名 | M0.2 |
| 继承 extends | 组合:`Type` 字段或嵌入;结构体+方法+小接口,不要类树 | 全程 |
| `implements` 声明 | 接口隐式满足,在**消费处**定义 1–2 方法的小接口 | M4.1 |
| `synchronized`/锁 | channel+select,"以通信共享内存";一个 goroutine 独占簿 | M3 |
| 单测靠 JUnit/Mockito | 内置 `testing` + 表驱动 `t.Run`;手写小 fake | M0.1, M4.1 |
| `go test` 就够了 | **必须 `go test -race`**,是另一种构建,当作硬门 | M3.3, M4.7 |
| `double`/`BigDecimal` | `int64` tick/分;float 破坏成交相等与 map key | M0.3(GG 用 float,我们有意分道) |

---

## 7. 进度追踪

- [x] M0 领域类型 　(0.1 crosses ▸ 0.2 Side/Order ▸ 0.3 int64 ▸ 0.4 NewOrder) ✅
- [x] M1 订单簿 　(1.1 FIFO ▸ 1.2 nil-map ▸ 1.3 指针接收者 ▸ 1.4 BestPrice ▸ 1.5 Cancel ▸ 1.6 Modify) ✅
- [x] M2 撮合引擎 　(2.1 全成 ▸ 2.2 部分 ▸ 2.3 多档 ▸ 2.4 FIFO ▸ 2.5 挂单 ▸ 2.6 市价 ▸ 2.7 校验/守恒) ✅
- [x] M3 并发+HTTP 　(3.1 Run ▸ 3.2 Place ▸ 3.3 -race ▸ 3.4 快照 ▸ 3.5 handler ▸ 3.6 关停) ✅
- [x] M4 做市 bot 　(4.1 接口 ▸ 4.2 价差 ▸ 4.3 Step ▸ 4.4 OnFill ▸ 4.5 偏移 ▸ 4.6 风控 ▸ 4.7 循环) ✅

**基础课程(M0-M4)已全部完成。进阶课程(M5-M9)见 `01-advanced-syllabus.md`。**

---

*本大纲由调研 Anthony GG 实际课程结构 + 业界订单簿/撮合/做市设计 + Java→Go 思维陷阱后综合而成,作为本项目随时回看的学习地图。*
