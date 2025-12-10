-- Bank-in-a-Box Load Generator - Database Schema
-- Compatible with MariaDB 11.8+
--
-- Usage:
--   mysql -u root -p < schema.sql
--
-- For bulk loading, consider:
--   1. Run schema_no_indexes.sql first
--   2. Load data via LOAD DATA INFILE
--   3. Run schema_indexes.sql to create indexes

-- Create database if not exists
CREATE DATABASE IF NOT EXISTS bank
    CHARACTER SET utf8mb4
    COLLATE utf8mb4_unicode_ci;

USE bank;

-- ============================================
-- BRANCHES AND ATMS
-- ============================================

CREATE TABLE IF NOT EXISTS branches (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    branch_code VARCHAR(20) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,

    -- Type and status
    type ENUM('full', 'limited', 'atm_only', 'hq', 'regional') NOT NULL DEFAULT 'full',
    status ENUM('open', 'closed', 'renovation', 'relocating') NOT NULL DEFAULT 'open',

    -- Location
    address_line1 VARCHAR(255) NOT NULL,
    address_line2 VARCHAR(255),
    city VARCHAR(100) NOT NULL,
    state VARCHAR(100),
    postal_code VARCHAR(20),
    country CHAR(2) NOT NULL,  -- ISO 3166-1 alpha-2
    latitude DECIMAL(10, 8),
    longitude DECIMAL(11, 8),

    -- Timezone (IANA format, e.g., 'America/New_York')
    timezone VARCHAR(50) NOT NULL,

    -- Operating hours (HH:MM-HH:MM format, NULL if closed)
    monday_hours VARCHAR(20),
    tuesday_hours VARCHAR(20),
    wednesday_hours VARCHAR(20),
    thursday_hours VARCHAR(20),
    friday_hours VARCHAR(20),
    saturday_hours VARCHAR(20),
    sunday_hours VARCHAR(20),

    -- Contact
    phone VARCHAR(30),
    email VARCHAR(255),

    -- Capacity for load modeling
    customer_capacity INT DEFAULT 100,
    atm_count INT DEFAULT 2,

    -- Timestamps
    opened_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    closed_at TIMESTAMP NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS atms (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    atm_id VARCHAR(20) NOT NULL UNIQUE,  -- Displayed on machine

    -- Associated branch (can be NULL for standalone ATMs)
    branch_id BIGINT,

    -- Status
    status ENUM('online', 'offline', 'maintenance', 'out_of_cash') NOT NULL DEFAULT 'online',

    -- Location
    location_name VARCHAR(255),
    address_line1 VARCHAR(255) NOT NULL,
    city VARCHAR(100) NOT NULL,
    state VARCHAR(100),
    postal_code VARCHAR(20),
    country CHAR(2) NOT NULL,
    latitude DECIMAL(10, 8),
    longitude DECIMAL(11, 8),

    -- Timezone
    timezone VARCHAR(50) NOT NULL,

    -- Capabilities
    supports_deposit BOOLEAN DEFAULT FALSE,
    supports_transfer BOOLEAN DEFAULT FALSE,
    is_24_hours BOOLEAN DEFAULT TRUE,

    -- Load modeling
    avg_daily_transactions INT DEFAULT 50,

    -- Timestamps
    installed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB;

-- ============================================
-- CUSTOMERS
-- ============================================

CREATE TABLE IF NOT EXISTS customers (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,

    -- Personal Information
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    email VARCHAR(255) NOT NULL,
    phone VARCHAR(30),
    date_of_birth DATE,

    -- Address
    address_line1 VARCHAR(255),
    address_line2 VARCHAR(255),
    city VARCHAR(100),
    state VARCHAR(100),
    postal_code VARCHAR(20),
    country CHAR(2) NOT NULL,  -- ISO 3166-1 alpha-2

    -- Geographic/Timezone
    timezone VARCHAR(50) NOT NULL,
    home_branch_id BIGINT,

    -- Banking Profile
    segment ENUM('regular', 'premium', 'private', 'business', 'corporate') NOT NULL DEFAULT 'regular',
    status ENUM('active', 'inactive', 'suspended', 'closed') NOT NULL DEFAULT 'active',
    activity_score DECIMAL(3, 2) DEFAULT 0.50,  -- 0.00 to 1.00

    -- Authentication (hashed values)
    username VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    pin VARCHAR(255) NOT NULL,  -- Hashed PIN for ATM

    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB;

-- ============================================
-- ACCOUNTS
-- ============================================

CREATE TABLE IF NOT EXISTS accounts (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    account_number VARCHAR(30) NOT NULL UNIQUE,

    -- Owner
    customer_id BIGINT NOT NULL,

    -- Account details
    type ENUM('checking', 'savings', 'credit_card', 'loan', 'mortgage',
              'investment', 'business', 'merchant', 'payroll') NOT NULL,
    status ENUM('active', 'dormant', 'frozen', 'closed', 'pending') NOT NULL DEFAULT 'active',
    currency CHAR(3) NOT NULL DEFAULT 'USD',  -- ISO 4217

    -- Balance in cents (for precision)
    balance BIGINT NOT NULL DEFAULT 0,

    -- Limits in cents
    credit_limit BIGINT DEFAULT 0,
    overdraft_limit BIGINT DEFAULT 0,
    daily_withdraw_limit BIGINT DEFAULT 50000,   -- $500 default
    daily_transfer_limit BIGINT DEFAULT 500000,  -- $5000 default

    -- Interest rate in basis points (250 = 2.50%)
    interest_rate INT DEFAULT 0,

    -- Branch association
    branch_id BIGINT,

    -- Timestamps
    opened_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    closed_at TIMESTAMP NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB;

-- ============================================
-- BENEFICIARIES (External Payees)
-- ============================================

CREATE TABLE IF NOT EXISTS beneficiaries (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,

    -- Owner
    customer_id BIGINT NOT NULL,

    -- Beneficiary details
    nickname VARCHAR(100),
    name VARCHAR(255) NOT NULL,
    type ENUM('individual', 'business', 'utility', 'government') NOT NULL DEFAULT 'individual',
    status ENUM('pending', 'verified', 'blocked') NOT NULL DEFAULT 'verified',

    -- Bank details
    bank_name VARCHAR(255),
    bank_code VARCHAR(20),          -- SWIFT/BIC
    routing_number VARCHAR(20),     -- ABA (US)
    account_number VARCHAR(50),
    iban VARCHAR(50),

    -- Address
    address_line1 VARCHAR(255),
    address_line2 VARCHAR(255),
    city VARCHAR(100),
    state VARCHAR(100),
    postal_code VARCHAR(20),
    country CHAR(2),

    -- Payment details
    currency CHAR(3) DEFAULT 'USD',
    payment_method ENUM('ach', 'wire', 'internal') DEFAULT 'ach',
    account_reference VARCHAR(100),  -- Customer's account # with biller

    -- Usage tracking
    last_used_at TIMESTAMP NULL,
    transfer_count INT DEFAULT 0,

    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB;

-- ============================================
-- TRANSACTIONS
-- ============================================

CREATE TABLE IF NOT EXISTS transactions (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    reference_number VARCHAR(30) NOT NULL UNIQUE,

    -- Primary account
    account_id BIGINT NOT NULL,

    -- Counterparty (for internal transfers)
    counterparty_account_id BIGINT,

    -- External beneficiary
    beneficiary_id BIGINT,

    -- Transaction details
    type ENUM('deposit', 'salary', 'transfer_in', 'interest_credit', 'refund', 'cashback',
              'withdrawal', 'purchase', 'transfer_out', 'bill_payment', 'interest_debit',
              'fee', 'loan_payment', 'payroll_batch') NOT NULL,
    status ENUM('pending', 'completed', 'failed', 'reversed', 'declined') NOT NULL DEFAULT 'completed',
    channel ENUM('online', 'atm', 'branch', 'pos', 'ach', 'wire', 'internal') NOT NULL,

    -- Amount in cents (always positive)
    amount BIGINT NOT NULL,
    currency CHAR(3) NOT NULL DEFAULT 'USD',

    -- Running balance after transaction (in cents)
    balance_after BIGINT NOT NULL,

    -- Description
    description VARCHAR(500),
    metadata JSON,  -- Additional structured data

    -- Location context
    branch_id BIGINT,
    atm_id BIGINT,

    -- For double-entry bookkeeping (link debit/credit pair)
    linked_transaction_id BIGINT,

    -- Timing
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    posted_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    value_date DATE NOT NULL,

    -- Error info
    failure_reason VARCHAR(255)
) ENGINE=InnoDB;

-- ============================================
-- AUDIT LOG
-- ============================================

CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,

    -- When
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Who
    customer_id BIGINT,
    employee_id BIGINT,
    system_id VARCHAR(100),

    -- What
    action ENUM(
        -- Authentication
        'login_success', 'login_failed', 'logout', 'pin_success', 'pin_failed',
        'password_changed', 'account_locked',
        -- Transactions
        'transaction_initiated', 'transaction_completed', 'transaction_failed', 'transaction_declined',
        -- Account management
        'account_opened', 'account_closed', 'account_updated',
        'beneficiary_added', 'beneficiary_removed',
        -- Profile
        'profile_viewed', 'profile_updated', 'address_changed', 'contact_changed',
        -- Sessions
        'session_started', 'session_ended', 'session_timeout',
        -- Queries
        'balance_inquiry', 'statement_viewed', 'history_viewed'
    ) NOT NULL,
    outcome ENUM('success', 'failure', 'denied', 'error') NOT NULL,

    -- Where
    channel ENUM('online', 'atm', 'branch', 'mobile', 'phone', 'api', 'system') NOT NULL,
    branch_id BIGINT,
    atm_id BIGINT,
    ip_address VARCHAR(45),  -- IPv6 compatible
    user_agent VARCHAR(500),

    -- Which entity
    account_id BIGINT,
    transaction_id BIGINT,
    beneficiary_id BIGINT,

    -- Details
    description VARCHAR(500),
    failure_reason VARCHAR(255),
    metadata JSON,

    -- Session tracking
    session_id VARCHAR(100),

    -- Risk scoring
    risk_score DECIMAL(3, 2),  -- 0.00 to 1.00

    -- Request tracing
    request_id VARCHAR(100)
) ENGINE=InnoDB;

-- ============================================
-- INDEXES
-- Note: For bulk loading, create these AFTER data load
-- ============================================

-- Branches
CREATE INDEX idx_branches_country ON branches(country);
CREATE INDEX idx_branches_status ON branches(status);

-- ATMs
CREATE INDEX idx_atms_status ON atms(status);
CREATE INDEX idx_atms_country ON atms(country);

-- Customers
CREATE INDEX idx_customers_country ON customers(country);
CREATE INDEX idx_customers_segment ON customers(segment);
CREATE INDEX idx_customers_status ON customers(status);
CREATE INDEX idx_customers_email ON customers(email);

-- Accounts
CREATE INDEX idx_accounts_customer ON accounts(customer_id);
CREATE INDEX idx_accounts_type ON accounts(type);
CREATE INDEX idx_accounts_status ON accounts(status);
CREATE INDEX idx_accounts_branch ON accounts(branch_id);

-- Beneficiaries
CREATE INDEX idx_beneficiaries_customer ON beneficiaries(customer_id);
CREATE INDEX idx_beneficiaries_status ON beneficiaries(status);

-- Transactions (most important for query performance)
CREATE INDEX idx_transactions_account ON transactions(account_id);
CREATE INDEX idx_transactions_timestamp ON transactions(timestamp);
CREATE INDEX idx_transactions_account_timestamp ON transactions(account_id, timestamp);
CREATE INDEX idx_transactions_type ON transactions(type);
CREATE INDEX idx_transactions_status ON transactions(status);
CREATE INDEX idx_transactions_channel ON transactions(channel);
CREATE INDEX idx_transactions_value_date ON transactions(value_date);
CREATE INDEX idx_transactions_counterparty ON transactions(counterparty_account_id);

-- Audit logs
CREATE INDEX idx_audit_timestamp ON audit_logs(timestamp);
CREATE INDEX idx_audit_customer ON audit_logs(customer_id);
CREATE INDEX idx_audit_action ON audit_logs(action);
CREATE INDEX idx_audit_outcome ON audit_logs(outcome);
CREATE INDEX idx_audit_session ON audit_logs(session_id);
CREATE INDEX idx_audit_account ON audit_logs(account_id);
CREATE INDEX idx_audit_transaction ON audit_logs(transaction_id);
