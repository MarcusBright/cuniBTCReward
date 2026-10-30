## goctl
```bash
goctl api go -api cuniBTCReward.api -dir .
goctl api swagger -api cuniBTCReward.api -dir docs
```

## Profile line

`POST /api/v1/profileline`

```json
{"symbol":"b2uniBTC","range":"7D"}
```

`range` accepts exactly `24H`, `7D`, `30D`, or `All`. Windows end at the
request's current Unix second. Buckets are rolling intervals, not calendar days:
24 hourly buckets, 7 daily buckets, 30 daily buckets, or 53 buckets covering
365 days (52 weeks followed by one day). All boundaries use `[startTime, endTime)`.
The normal `code`, `msg`, `data` response envelope applies.

Inside `data`, fields are ordered `symbol`, `range`, `deposited`, `startTime`,
`endTime`, `data`. The outer `deposited` is the sum of the per-bucket deposits.
Only positive vault `Minted` events are new deposits; redemptions do not reduce
this total. Amounts are strings scaled from raw units by 10^-8.

Each bucket contains:

- `startTime`, `endTime`: inclusive/exclusive Unix seconds.
- `amountBase`: estimated base rewards, allocated from generated airdrop records
  in proportion to overlap with the earning period. Locked strategies earn over
  `[lockup_start, lockup_start + lockup_period)`; `b2uniBTC` earns over
  `[operate_start, lockup_start + lockup_period)`, matching distribution.
- `amountExtra`: original `extra_reward_dis` amounts by their accrual timestamps.
  Persisted `air_drop_records.amount_extra` is never added again.
- `rewards`: `amountBase + amountExtra`.
- `deposited`: new deposits within this bucket.
- `effectivePrincipal`: eligible principal weighted by seconds of participation.
  `b2uniBTC` uses the running signed balance across the vault and redeem router.
  Locked strategies use their balance strictly before `lockup_start` during the
  lockup period, excluding queued deposits, matching distribution's share rule.
- `apy`: compound annualized return,
  `(1 + rewards / effectivePrincipal)^(31536000 / bucketSeconds) - 1`.
  `0.05` means 5%. Returns `null` for zero principal or a result outside finite
  floating-point range; positive principal with zero rewards returns `0`.
- `baseRewardComplete`: false when an overlapping earning epoch has no generated
  airdrop records. Such epochs contribute no estimated base reward yet, so their
  APY reflects only currently known rewards. Distribution can revise historical
  base estimates and APYs. Root confirmation is not required for these estimates.

All amount calculations use Decimal; base allocation preserves epoch totals
using cumulative allocation at bucket boundaries, with precision to 24 decimal
places in raw units. APY alone uses floating-point logarithms/exponentiation.
The query uses a consistent read snapshot and a fixed number of database queries,
not one query per bucket. Extra rewards are scoped by symbol, since the existing
`extra_reward_dis` schema has no chain ID; symbols in that table must identify the
same chain as the configured strategy.
