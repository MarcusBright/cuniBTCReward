package logic

// pendingExtraRewardsCTE uses the same [operate_start, epoch end) window as
// DistributeRewardsNoLock. strat and top_epoches must already be scoped to the
// requested chain and symbol. Once distribution creates records for an epoch,
// its persisted amounts replace the accrued rewards, even before root submission.
const pendingExtraRewardsCTE = `pending_extra_rewards AS (
    SELECT e.contract, e.epoch, er.address,
           COALESCE(SUM(er.amount), 0) AS amount_extra,
           MAX(er.last_deposit_time) AS last_deposit_time
    FROM top_epoches e
    JOIN strat s ON s.vault = e.contract
    JOIN extra_reward_dis er ON er.symbol = s.symbol
        AND er.timestamp >= e.operate_start
        AND er.timestamp < e.lockup_start + e.lockup_period
        AND er.deleted_at IS NULL
    WHERE NOT EXISTS (
        SELECT 1 FROM air_drop_records ar
        WHERE ar.chain_id = e.chain_id AND ar.contract = s.airdrop
          AND ar.epoch = e.epoch AND ar.deleted_at IS NULL
    )
    GROUP BY e.contract, e.epoch, er.address
)`

// pendingUserActionsCTE includes addresses with only extra rewards as well as
// depositors. Both pagination queries use the same action set.
const pendingUserActionsCTE = `WITH strat AS (
    SELECT * FROM strategies
    WHERE chain_id = ? AND symbol = ? AND deleted_at IS NULL
), top_epoches AS (
    SELECT e.* FROM epoches e
    JOIN strat s ON s.vault = e.contract AND s.chain_id = e.chain_id
    WHERE e.epoch = ? AND e.deleted_at IS NULL
), ` + pendingExtraRewardsCTE + `,
transaction_actions AS (
    SELECT e.contract, e.epoch, t.address,
           COALESCE(SUM(CASE WHEN t.block_timestamp < e.lockup_start THEN t.amount ELSE 0 END), 0) AS deposited,
           COALESCE(SUM(CASE WHEN t.block_timestamp >= e.lockup_start THEN t.amount ELSE 0 END), 0) AS queued,
           MAX(t.block_timestamp) AS last_deposit_time
    FROM top_epoches e
    JOIN evm_transactions t ON t.chain_id = e.chain_id AND t.contract = e.contract
        AND t.block_timestamp < e.lockup_start + e.lockup_period
        AND t.deleted_at IS NULL AND t.amount > 0
    GROUP BY e.contract, e.epoch, t.address
), action_addresses AS (
    SELECT contract, epoch, address FROM transaction_actions
    UNION
    SELECT contract, epoch, address FROM pending_extra_rewards
), action AS (
    SELECT e.epoch, e.operate_start, e.operate_period, e.lockup_start, e.lockup_period,
           s.symbol, a.address,
           COALESCE(t.deposited, 0) AS deposited,
           COALESCE(t.queued, 0) AS queued,
           0 AS amount_base,
           COALESCE(er.amount_extra, 0) AS amount_extra,
           COALESCE(er.amount_extra, 0) AS rewards,
           GREATEST(COALESCE(t.last_deposit_time, 0), COALESCE(er.last_deposit_time, 0)) AS last_deposit_time
    FROM action_addresses a
    JOIN top_epoches e ON e.contract = a.contract AND e.epoch = a.epoch
    JOIN strat s ON s.vault = e.contract
    LEFT JOIN transaction_actions t ON t.contract = a.contract AND t.epoch = a.epoch AND t.address = a.address
    LEFT JOIN pending_extra_rewards er ON er.contract = a.contract AND er.epoch = a.epoch AND er.address = a.address
)`
