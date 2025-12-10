# Architecture Documentation

## Overview

The Bank-in-a-Box Load Generator is designed as a two-phase system:

1. **Phase 1 (Data Generation)**: Generates realistic historical banking data as CSV files
2. **Phase 2 (Live Simulation)**: Runs concurrent customer sessions against a database

## Directory Structure

```
load-generator/
├── cmd/loadgen/           # Application entry point
│   └── main.go
├── internal/
│   ├── cmd/               # CLI commands (Cobra)
│   │   ├── root.go        # Root command, global flags
│   │   ├── generate.go    # Phase 1: data generation
│   │   ├── simulate.go    # Phase 2: live simulation
│   │   ├── schema.go      # Database schema output
│   │   └── version.go     # Version information
│   ├── config/            # Configuration structures
│   │   └── config.go      # Config structs, defaults, validation
│   ├── data/              # Embedded reference data
│   │   ├── loader.go      # Data loading and lookup
│   │   ├── names/         # First/last names by region
│   │   └── addresses/     # Countries, cities, postal codes
│   ├── database/          # Database layer
│   │   ├── pool.go        # Connection pool management
│   │   ├── queries.go     # Parameterized SQL queries
│   │   └── schema*.sql    # Database schema files
│   ├── generator/         # Phase 1: Data generators
│   │   ├── orchestrator.go    # Coordinates all generators
│   │   ├── customer.go        # Customer generation
│   │   ├── account.go         # Account generation
│   │   ├── transaction.go     # Transaction generation
│   │   ├── branch.go          # Branch/ATM generation
│   │   ├── business.go        # Business entity generation
│   │   ├── beneficiary.go     # Beneficiary generation
│   │   ├── audit.go           # Audit log generation
│   │   ├── csvwriter.go       # Streaming CSV output
│   │   ├── progress.go        # Progress bar display
│   │   └── patterns/          # Transaction patterns
│   │       ├── daily.go       # Intraday patterns
│   │       ├── weekly.go      # Weekday/weekend patterns
│   │       ├── monthly.go     # Payroll spikes
│   │       └── distribution.go # Pareto distribution
│   ├── models/            # Data models
│   │   ├── customer.go
│   │   ├── account.go
│   │   ├── transaction.go
│   │   ├── branch.go
│   │   ├── beneficiary.go
│   │   └── audit.go
│   ├── simulator/         # Phase 2: Live simulation
│   │   ├── session.go         # Session manager
│   │   ├── state.go           # Customer state machine
│   │   ├── activity.go        # Activity probability calculator
│   │   ├── scheduler.go       # Session scheduling
│   │   ├── timezone.go        # Timezone management
│   │   ├── metrics.go         # Real-time metrics
│   │   ├── audit.go           # Live audit trail writing
│   │   ├── errors.go          # Error simulation
│   │   ├── loadcontrol.go     # Ramp-up/ramp-down
│   │   └── burst/             # Burst simulation
│   │       ├── types.go       # Burst manager
│   │       ├── lunch.go       # Lunch-time ATM burst
│   │       ├── payroll.go     # End-of-month payroll surge
│   │       └── random.go      # Random spike generator
│   └── utils/             # Utilities
│       ├── random.go          # Deterministic PRNG
│       └── money.go           # Fixed-point money type
├── configs/               # Example configuration files
├── data/                  # Reference data (JSON)
│   ├── names/
│   └── addresses/
├── scripts/               # Helper scripts
└── docs/                  # Documentation
```

## Key Design Decisions

### 1. Deterministic Randomness

All random generation uses a seedable PRNG (`internal/utils/random.go`). This enables:
- Reproducible data generation for consistent benchmarks
- Identical workloads across different database comparisons
- Debugging by replaying specific scenarios

```go
rng := utils.NewRandom(seed)
value := rng.Intn(100)  // Same seed = same sequence
```

### 2. Fixed-Point Money

All monetary values use `int64` cents (`internal/utils/money.go`) to avoid floating-point precision issues:
- Balance of $100.50 stored as `10050`
- Supports 40+ currencies with proper decimal places
- No rounding errors in transaction calculations

### 3. Streaming CSV Generation

The CSV writer (`internal/generator/csvwriter.go`) uses buffered streaming:
- Memory-efficient for large datasets
- Writes directly to disk, not in-memory
- Progress reporting during generation

### 4. Timezone-Aware Scheduling

The simulator respects customer timezones (`internal/simulator/timezone.go`):
- Each customer has an assigned timezone
- Activity probability calculated based on local time
- Peak activity during 8 AM - 4 PM local time
- Creates realistic "follow the sun" load patterns

### 5. Session State Machine

Customer sessions follow realistic workflows (`internal/simulator/state.go`):

```
ATM Session:
  authenticate → checkBalance → [withdraw|deposit] → end

Online Banking:
  login → viewAccounts → [viewHistory|transfer|billPay]* → logout

Business Session:
  authenticate → reviewAccounts → [batchPayroll|sweep|vendorPayment] → logout
```

## Data Flow

### Phase 1: Generation

```
                    ┌─────────────────────────────────────────────┐
                    │              Orchestrator                    │
                    │   (coordinates all generators)               │
                    └─────────────────────────────────────────────┘
                                        │
         ┌──────────────────────────────┼──────────────────────────────┐
         ▼                              ▼                              ▼
┌─────────────────┐           ┌─────────────────┐           ┌─────────────────┐
│ Entity Generator │           │ Transaction Gen │           │  Audit Gen      │
│ (customers,      │           │ (patterns,      │           │ (login events,  │
│  accounts, etc.) │           │  amounts, types)│           │  txn events)    │
└────────┬────────┘           └────────┬────────┘           └────────┬────────┘
         │                              │                              │
         ▼                              ▼                              ▼
┌─────────────────┐           ┌─────────────────┐           ┌─────────────────┐
│  CSV Writer     │           │  CSV Writer     │           │  CSV Writer     │
│  (streaming)    │           │  (streaming)    │           │  (streaming)    │
└────────┬────────┘           └────────┬────────┘           └────────┬────────┘
         │                              │                              │
         ▼                              ▼                              ▼
    customers.csv              transactions.csv               audit_logs.csv
```

### Phase 2: Simulation

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Session Manager                                    │
│   • Manages goroutine pool                                                  │
│   • Coordinates with Scheduler                                              │
│   • Handles graceful shutdown                                               │
└─────────────────────────────────────────────────────────────────────────────┘
                                        │
                    ┌───────────────────┼───────────────────┐
                    ▼                   ▼                   ▼
           ┌────────────────┐  ┌────────────────┐  ┌────────────────┐
           │  Scheduler     │  │  Burst Manager │  │  Load Control  │
           │  • Weighted    │  │  • Lunch burst │  │  • Ramp up     │
           │    customer    │  │  • Payroll     │  │  • Ramp down   │
           │    selection   │  │  • Random      │  │                │
           └────────────────┘  └────────────────┘  └────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Customer Sessions (goroutines)                       │
│   ┌──────────┐  ┌──────────┐  ┌──────────┐         ┌──────────┐            │
│   │ Session  │  │ Session  │  │ Session  │   ...   │ Session  │            │
│   │ (ATM)    │  │ (Online) │  │(Business)│         │   (N)    │            │
│   └────┬─────┘  └────┬─────┘  └────┬─────┘         └────┬─────┘            │
└────────┼─────────────┼─────────────┼─────────────────────┼──────────────────┘
         │             │             │                     │
         ▼             ▼             ▼                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Database Pool                                      │
│   • Connection pooling                                                       │
│   • Prepared statements                                                      │
│   • Metrics collection                                                       │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Realistic Patterns

### Transaction Volume Distribution

Uses Pareto (80/20) distribution:
- 20% of accounts generate 80% of transactions
- Implemented via activity scores assigned per customer

### Temporal Patterns

```
Daily:    ██████████░░░░████████░░░░░░░░
          6AM       12PM       6PM      12AM
          (peak)    (lunch)   (decline) (overnight)

Weekly:   Mon  Tue  Wed  Thu  Fri  Sat  Sun
          ████ ████ ████ ████ ████ ██░░ ░░░░
          (full activity)      (reduced)

Monthly:  Day 1-24: Normal activity
          Day 25-31: Payroll spike (3x multiplier)
```

### Customer Segments

| Segment | Activity | Transaction Size | Typical Operations |
|---------|----------|------------------|-------------------|
| Regular | Medium | Small-Medium | ATM, bills, transfers |
| Premium | High | Medium-Large | Investments, transfers |
| Business | Very High | Large | Payroll, sweeps, vendor |

## Extensibility

### Adding New Transaction Types

1. Add type to `models/transaction.go`
2. Add generation logic to `generator/transaction.go`
3. Add execution query to `database/queries.go`
4. Add session action to `simulator/state.go`

### Adding New Patterns

1. Create new pattern in `generator/patterns/`
2. Integrate in `generator/transaction.go`
3. Add corresponding burst type in `simulator/burst/`

### Adding New Session Types

1. Define workflow in `simulator/state.go`
2. Add ratio to config
3. Update `simulator/activity.go` for selection

## Metrics

The simulator tracks:

| Metric | Description |
|--------|-------------|
| TPS | Transactions per second (rolling 10s window) |
| Error Rate | Percentage of failed operations |
| Latency | p50, p95, p99 response times |
| Per-Operation | Breakdown by operation type |
| Active Sessions | Currently active session count |
| Burst Status | Current burst multiplier |

## Error Handling

### Simulated Errors

- **Failed Login**: Configurable rate of authentication failures
- **Insufficient Funds**: Declined transactions when balance too low
- **Timeouts**: Simulated network/database timeouts with retry

### Real Errors

- Connection failures trigger graceful reconnection
- Session errors logged but don't crash other sessions
- Graceful shutdown drains all pending operations
