# 迷你区块浏览器 / ERC-20 监听服务 — 开发文档

> 创建时间：2025-06-11
> 本文档记录从 0 到 1 的完整开发计划、代码审查、实现细节与迭代记录。

---

## 一、现有代码问题分析与改进建议

### 1. `main.go` — 严重问题

**问题 1：重复逻辑。** `main.go` 手动做了 `client.LoadEthConfig()` + `ethclient.DialContext()` 的事情，但又没有使用 `internal/client/` 包。应统一使用 `client.NewEthClient()`。

**问题 2：`NewEventStore(100)` 未定义。** `main.go` 是 `package main`，但 `NewEventStore` 在 `package database` 中，且名称是小写 `list()`。直接调用会编译失败。

**问题 3：ABI 文件硬编码路径。** `"abi/MyERC20.abi.json"` 是相对路径，取决于工作目录，部署时容易出错。

### 2. `internal/client/client.go` — 基本可用，小问题

- 代码质量不错，结构清晰。
- **问题**：没有暴露 `*ethclient.Client` 的高级封装方法（如 `BlockByNumber`、`TransactionByHash` 等），后续 service 层不得不直接操作底层 client。
- **建议**：在 `EthClient` 上增加业务友好的查询方法，或将 `Client` 字段改为小写 `client` 并提供 getter，避免外部直接修改。

### 3. `internal/config/config.go` — 可有可无

- 当前仅是 `client.LoadEthConfig()` 的一层薄包装，意义不大。
- **建议**：要么在此处扩展为统一配置管理（含 HTTP 端口、缓冲区大小等），要么直接删掉此包让 `main.go` 直接调 `client.LoadEthConfig()`。

### 4. `internal/service/blockService.go` — 编译错误，需重写

**问题 1：尝试给 `*ethclient.Client` 添加方法。** Go 语言不允许跨包给外部类型添加方法，`func (c *ethclient.Client) BlockByNumber(...)` 会直接编译报错。

**问题 2：`queryBlockBy()` 函数签名不完整，编译失败。**

**建议**：完全重写此文件，定义自己的 `BlockService` 结构体，内部持有 `*client.EthClient`，在其上实现查询方法。

### 5. `internal/store/database.go` — 基本可用，需改进

**问题 1：包名 `database` 不准确。** 当前是纯内存实现，叫 `database` 有误导性。建议改为 `store` 或 `memory`。

**问题 2：`list()` 是小写。** 外部包无法调用。应改为 `List()` 或 `GetAll()`。

**问题 3：环形缓冲区效率低。** `s.events = s.events[1:]` 每次删除头部需要 O(n) 拷贝。对于高频事件场景，建议使用 `container/Ring` 或 channel + 固定大小 slice + cursor。

**问题 4：只存储了 `TransferEvent`。** 按需求还需存储区块信息、交易信息、地址余额等，需要扩展或新增 store。

### 6. 缺失部分

| 缺失项 | 说明 |
|---|---|
| `internal/api/` | 完全缺失，没有 HTTP 路由和 handler |
| 事件监听逻辑 | 没有 `SubscribeFilterLogs` / `SubscribeNewHead` 代码 |
| 交易/回执查询 | 未实现 |
| 地址余额查询 | 未实现 |
| 优雅退出 | 仅声明了 `cancel()`，没有信号监听 |
| 配置文件 | 没有 `.env.example` 或 `config.yaml` |
| `go.sum` 异常 | 包含 `github.com/ProjectZKM/Ziren` 依赖，疑似从其他项目复制而来 |

---

## 二、项目目标

构建一个迷你区块浏览器 + ERC-20 监听服务，能够：

1. 按**区块号/哈希**查询区块
2. 按**哈希**查询交易与回执
3. 按**地址**查询最近交易与原生代币余额
4. 后台持续监听新区块
5. 后台持续监听指定 ERC-20 合约的 `Transfer` 事件
6. 数据存储在内存中（环形缓冲区），通过 RESTful API 暴露

---

## 三、目标目录结构

```
MINI/
├── main.go                          # 程序入口
├── go.mod / go.sum
├── .env.example                     # 环境变量示例
├── abi/
│   └── MyERC20.abi.json            # ERC-20 ABI（已有）
├── internal/
│   ├── client/
│   │   └── client.go               # 以太坊客户端封装
│   ├── config/
│   │   └── config.go               # 统一配置管理
│   ├── service/
│   │   ├── block_service.go        # 区块查询服务
│   │   ├── tx_service.go           # 交易/回执查询服务
│   │   ├── address_service.go      # 地址查询服务
│   │   └── event_service.go        # 事件监听服务
│   ├── api/
│   │   ├── server.go               # HTTP 服务 + 路由注册
│   │   ├── block_handler.go        # 区块相关 handler
│   │   ├── tx_handler.go           # 交易相关 handler
│   │   ├── address_handler.go      # 地址相关 handler
│   │   └── event_handler.go        # 事件查询 handler
│   └── store/
│       ├── store.go                # 统一存储接口定义
│       ├── memory_store.go         # 内存存储实现
│       └── model.go                # 数据模型定义
```

---

## 四、分步实现计划

### 第 1 步：统一配置管理 — `internal/config/config.go`

**目标**：集中管理所有配置项，支持环境变量 + 默认值。

```go
package config

type Config struct {
    RPCURL       string // ETH_WS_URL 或 ETH_RPC_URL
    ContractAddr string // ERC20_CONTRACT
    HTTPPort     string // HTTP_PORT，默认 ":8080"
    StoreLimit   int    // 环形缓冲区大小，默认 500
}

func Load() *Config { ... }
```

**要点**：
- `RPCURL`：优先 `ETH_WS_URL`（用于订阅），回退 `ETH_RPC_URL`（用于查询）
- `HTTPPort`：从 `HTTP_PORT` 环境变量读取，默认 `:8080`
- `StoreLimit`：从 `STORE_LIMIT` 读取，默认 500
- 如果必要字段为空，直接 `log.Fatal`

---

### 第 2 步：重构客户端封装 — `internal/client/client.go`

**目标**：保留现有结构，增加便利方法。

```go
type EthClient struct {
    Client       *ethclient.Client
    ContractAddr common.Address
    Ctx          context.Context
    Cancel       context.CancelFunc
    ABI          abi.ABI                     // 新增：解析好的 ABI
}

func NewEthClient(cfg *config.Config) (*EthClient, error) {
    // 1. context
    // 2. dial
    // 3. 加载 ABI 文件（路径可配置或相对路径）
    // 4. 返回
}
```

**新增便利方法**：
- `BlockByNumber(num *big.Int) (*types.Block, error)`
- `BlockByHash(hash common.Hash) (*types.Block, error)`
- `TxByHash(hash common.Hash) (*types.Transaction, bool, error)`
- `TxReceipt(hash common.Hash) (*types.Receipt, error)`
- `BalanceAt(addr common.Address, blockNum *big.Int) (*big.Int, error)`
- `SubscribeNewHead(ch chan<- *types.Header) (ethereum.Subscription, error)`
- `FilterLogs(q ethereum.FilterQuery) ([]types.Log, error)`
- `SubscribeFilterLogs(q ethereum.FilterQuery, ch chan<- types.Log) (ethereum.Subscription, error)`

---

### 第 3 步：数据模型与存储层 — `internal/store/`

#### 3a. `model.go` — 数据结构定义

```go
package store

import (
    "math/big"
    "time"
    "github.com/ethereum/go-ethereum/common"
)

type BlockInfo struct {
    Number       uint64      `json:"number"`
    Hash         string      `json:"hash"`
    ParentHash   string      `json:"parentHash"`
    Timestamp    uint64      `json:"timestamp"`
    Miner        string      `json:"miner"`
    GasUsed      uint64      `json:"gasUsed"`
    GasLimit     uint64      `json:"gasLimit"`
    TxCount      int         `json:"txCount"`
    TxHashes     []string    `json:"txHashes,omitempty"`
}

type TxInfo struct {
    Hash        string `json:"hash"`
    BlockNumber uint64 `json:"blockNumber"`
    From        string `json:"from"`
    To          string `json:"to,omitempty"`
    Value       string `json:"value"`        // wei 的字符串表示
    GasPrice    string `json:"gasPrice"`
    GasUsed     uint64 `json:"gasUsed"`
    Status      uint64 `json:"status"`       // 1=成功, 0=失败
    Input       string `json:"input"`
    Timestamp   uint64 `json:"timestamp"`
}

type TransferEvent struct {
    BlockNumber uint64    `json:"blockNumber"`
    TxHash      string    `json:"txHash"`
    From        string    `json:"from"`
    To          string    `json:"to"`
    Value       string    `json:"value"`
    LogIndex    uint      `json:"logIndex"`
    Timestamp   time.Time `json:"timestamp"`
}
```

#### 3b. `store.go` — 统一存储接口

```go
type Store interface {
    // 区块
    SaveBlock(block BlockInfo)
    GetBlockByNumber(num uint64) (*BlockInfo, bool)
    GetBlockByHash(hash string) (*BlockInfo, bool)
    LatestBlockNumber() uint64

    // 交易
    SaveTx(tx TxInfo)
    GetTx(hash string) (*TxInfo, bool)

    // Transfer 事件
    AddTransfer(event TransferEvent)
    ListTransfers(from, to string, limit int) []TransferEvent

    // 地址相关
    SaveAddressTx(addr string, txHash string)
    GetAddressTxs(addr string, limit int) []string
}
```

#### 3c. `memory_store.go` — 内存实现

使用 `sync.RWMutex` + `map` + 固定大小 slice 实现环形缓冲区：

```go
type MemoryStore struct {
    mu          sync.RWMutex
    blocks      map[uint64]*BlockInfo   // blockNumber -> BlockInfo
    blockHashes map[string]uint64       // blockHash -> blockNumber
    blockOrder  []uint64                // 按插入顺序，用于淘汰旧数据
    txs         map[string]*TxInfo      // txHash -> TxInfo
    transfers   []TransferEvent         // 环形缓冲区
    transferIdx int                     // 环形游标
    addrTxs     map[string][]string     // addr -> []txHash（最近 N 笔）
    limit       int
}
```

**淘汰策略**：当存储数量超过 `limit` 时，按 FIFO 淘汰最早的记录。

---

### 第 4 步：Service 层 — `internal/service/`

#### 4a. `block_service.go`

```go
type BlockService struct {
    client *client.EthClient
    store  store.Store
}

func NewBlockService(c *client.EthClient, s store.Store) *BlockService

// QueryByNumber 按区块号查询（先查内存，miss 则链上查并缓存）
func (s *BlockService) QueryByNumber(num uint64) (*store.BlockInfo, error)

// QueryByHash 按哈希查询
func (s *BlockService) QueryByHash(hash string) (*store.BlockInfo, error)
```

**核心逻辑**：
1. 先查 `store.GetBlockByNumber/Hash`
2. 如果 miss，调用 `client.BlockByNumber/Hash` 从链上查
3. 将 `*types.Block` 转换为 `BlockInfo` 并存入 store
4. 返回结果

#### 4b. `tx_service.go`

```go
type TxService struct {
    client *client.EthClient
    store  store.Store
}

func NewTxService(c *client.EthClient, s store.Store) *TxService

// QueryTx 查询交易详情
func (s *TxService) QueryTx(hash string) (*store.TxInfo, error)

// QueryReceipt 查询交易回执
func (s *TxService) QueryReceipt(hash string) (*store.TxInfo, error)
```

#### 4c. `address_service.go`

```go
type AddressService struct {
    client *client.EthClient
    store  store.Store
}

func NewAddressService(c *client.EthClient, s store.Store) *AddressService

// GetBalance 查询原生 ETH 余额
func (s *AddressService) GetBalance(addr string) (string, error)

// GetRecentTxs 查询最近交易列表
func (s *AddressService) GetRecentTxs(addr string, limit int) ([]*store.TxInfo, error)
```

#### 4d. `event_service.go` — 核心监听逻辑

```go
type EventService struct {
    client *client.EthClient
    store  store.Store
}

func NewEventService(c *client.EthClient, s store.Store) *EventService

// WatchNewBlocks 订阅新区块，存入 store
func (s *EventService) WatchNewBlocks(ctx context.Context) error

// WatchTransfers 订阅 ERC-20 Transfer 事件，存入 store
func (s *EventService) WatchTransfers(ctx context.Context) error
```

**WatchNewBlocks 核心流程**：
```
1. 调用 client.SubscribeNewHead(headerCh)
2. for 循环 select:
   case header := <-headerCh:
       - 用 client.BlockByNumber(header.Number) 获取完整区块
       - 转换为 BlockInfo，保存到 store
       - 遍历区块中的交易，转换为 TxInfo，保存到 store
       - 记录地址与交易的映射关系
   case err := <-sub.Err():
       - log 错误，尝试重新连接
   case <-ctx.Done():
       - 退出
```

**WatchTransfers 核心流程**：
```
1. 构建 FilterQuery:
   - Addresses: [contractAddr]
   - Topics: [TransferEventID]
2. 调用 client.SubscribeFilterLogs(query, logCh)
3. for 循环 select:
   case vLog := <-logCh:
       - 用 ABI.Unpack("Transfer", vLog.Data) 解析事件
       - 构造 TransferEvent，保存到 store
   case err := <-sub.Err():
       - log 错误，尝试重连
   case <-ctx.Done():
       - 退出
```

---

### 第 5 步：API 层 — `internal/api/`

#### 5a. `server.go`

```go
type Server struct {
    router     *mux.Router  // 或 net/http 默认路由
    blockSvc   *service.BlockService
    txSvc      *service.TxService
    addrSvc    *service.AddressService
    eventSvc   *service.EventService
}

func NewServer(...) *Server
func (s *Server) RegisterRoutes()
func (s *Server) Start(addr string) error
```

#### 5b. 路由设计

| 方法 | 路径 | 功能 |
|---|---|---|
| `GET` | `/api/block/number/:num` | 按区块号查询区块 |
| `GET` | `/api/block/:hash` | 按哈希查询区块 |
| `GET` | `/api/tx/:hash` | 查询交易详情 |
| `GET` | `/api/receipt/:hash` | 查询交易回执 |
| `GET` | `/api/address/:addr/balance` | 查询地址余额 |
| `GET` | `/api/address/:addr/txs` | 查询地址最近交易 |
| `GET` | `/api/events/transfers` | 查询已捕获的 Transfer 事件 |

#### 5c. Handler 实现模式

每个 handler 遵循统一模式：

```go
func (s *Server) handleGetBlockByNumber(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    num, err := strconv.ParseUint(vars["num"], 10, 64)
    if err != nil { writeJSON(w, 400, errResp("invalid block number")); return }

    block, err := s.blockSvc.QueryByNumber(num)
    if err != nil { writeJSON(w, 500, errResp(err.Error())); return }
    if block == nil { writeJSON(w, 404, errResp("block not found")); return }

    writeJSON(w, 200, block)
}
```

使用标准 `net/http` + 手动路由解析即可，无需引入第三方框架。

---

### 第 6 步：重构 `main.go` — 组装一切

```go
func main() {
    // 1. 加载配置
    cfg := config.Load()

    // 2. 初始化以太坊客户端
    ethClient, err := client.NewEthClient(cfg)
    if err != nil { log.Fatal(err) }
    defer ethClient.Close()

    // 3. 初始化内存存储
    memStore := store.NewMemoryStore(cfg.StoreLimit)

    // 4. 初始化 Service 层
    blockSvc  := service.NewBlockService(ethClient, memStore)
    txSvc     := service.NewTxService(ethClient, memStore)
    addrSvc   := service.NewAddressService(ethClient, memStore)
    eventSvc  := service.NewEventService(ethClient, memStore)

    // 5. 启动后台监听（goroutine）
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    go eventSvc.WatchNewBlocks(ctx)
    go eventSvc.WatchTransfers(ctx)

    // 6. 启动 HTTP 服务
    srv := api.NewServer(blockSvc, txSvc, addrSvc, eventSvc)
    log.Printf("HTTP server listening on %s", cfg.HTTPPort)
    if err := srv.Start(cfg.HTTPPort); err != nil {
        log.Fatal(err)
    }

    // 7. 优雅退出（监听 SIGINT / SIGTERM）
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh
    log.Println("shutting down...")
    cancel()
}
```

---

### 第 7 步：创建 `.env.example`

```env
# WebSocket URL（用于订阅事件，优先级高于 HTTP）
ETH_WS_URL=ws://127.0.0.1:8546

# HTTP RPC URL（用于查询，回退选项）
ETH_RPC_URL=http://127.0.0.1:8545

# 要监听的 ERC-20 合约地址
ERC20_CONTRACT=0xYourContractAddress

# HTTP 服务端口
HTTP_PORT=:8080

# 内存存储缓冲区大小
STORE_LIMIT=500
```

---

## 五、实现顺序

按依赖关系从底向上：

```
第 1 步  config        ← 无依赖
第 2 步  client        ← 依赖 config
第 3 步  store/model   ← 无依赖
第 4 步  service       ← 依赖 client + store
第 5 步  api           ← 依赖 service
第 6 步  main.go       ← 依赖以上所有
第 7 步  .env.example  ← 配置文件
```

每完成一步都可以 `go build` 确认编译通过，最后一步 `main.go` 完成后即可运行。

---

## 六、关键技术要点提醒

1. **WebSocket vs HTTP RPC**：订阅功能（`SubscribeNewHead`、`SubscribeFilterLogs`）需要 WebSocket 连接。纯 HTTP RPC 无法订阅。所以 `ETH_WS_URL` 优先，`ETH_RPC_URL` 回退用于查询。

2. **Transfer 事件解析**：`Transfer(address indexed from, address indexed to, uint256 value)` 中 `from` 和 `to` 是 indexed，存在 `vLog.Topics[1]` 和 `vLog.Topics[2]` 中；`value` 是非 indexed，存在 `vLog.Data` 中，需要用 ABI 解码。

3. **重连机制**：WebSocket 连接可能断开，`sub.Err()` 通道收到错误后应实现指数退避重连。

4. **大数处理**：所有 `*big.Int` 类型在 JSON 返回时转为**十进制字符串**，避免精度丢失。

5. **线程安全**：store 层所有方法必须是并发安全的（`sync.RWMutex`），因为后台 goroutine 写入、HTTP goroutine 读取。

---

## 七、迭代记录

> 后续对话中的关键决策、代码变更、问题修复等内容追加在此处。

### [2025-06-11] 初始版本

- 完成代码审查与架构设计
- 确定目录结构与分步实现计划
- 识别现有代码 6 大类问题
