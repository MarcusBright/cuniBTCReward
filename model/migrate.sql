USE `cunibtcreward`;

ALTER TABLE `air_drop_records`
    ADD COLUMN `shares_extra` DECIMAL(38, 0) NULL DEFAULT 0,
    ADD COLUMN `amount_extra` DECIMAL(38, 0) NULL DEFAULT 0,
    ADD COLUMN `amount_base` DECIMAL(38, 0) NULL DEFAULT 0;

UPDATE `air_drop_records`
SET `amount_base` = `amount`;

CREATE TABLE `extra_reward_dis` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `created_at` DATETIME(3) NULL,
    `updated_at` DATETIME(3) NULL,
    `deleted_at` DATETIME(3) NULL,
    `address` VARCHAR(128) NULL,
    `symbol` VARCHAR(128) NULL,
    `timestamp` BIGINT UNSIGNED NULL DEFAULT 0,
    `amount` DECIMAL(38, 0) NULL DEFAULT 0,
    `share` DECIMAL(38, 0) NULL DEFAULT 0,
    `last_deposit_time` BIGINT NULL DEFAULT 0,
    PRIMARY KEY (`id`),
    UNIQUE KEY `t_address_nonce` (`address`, `symbol`, `timestamp`),
    KEY `idx_extra_reward_dis_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `extra_reward_recs` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `created_at` DATETIME(3) NULL,
    `updated_at` DATETIME(3) NULL,
    `deleted_at` DATETIME(3) NULL,
    `symbol` VARCHAR(128) NULL,
    `timestamp` BIGINT UNSIGNED NULL DEFAULT 0,
    `total_amount` DECIMAL(38, 0) NULL DEFAULT 0,
    PRIMARY KEY (`id`),
    UNIQUE KEY `t_address_nonce` (`symbol`, `timestamp`),
    KEY `idx_extra_reward_recs_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;