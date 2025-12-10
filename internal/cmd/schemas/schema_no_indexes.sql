-- Bank-in-a-Box Load Generator - Schema WITHOUT Indexes
-- Use this for Phase 1 bulk loading, then run schema_indexes.sql after
--
-- Usage:
--   1. mysql -u root -p < schema_no_indexes.sql
--   2. LOAD DATA INFILE for each table
--   3. mysql -u root -p < schema_indexes.sql

CREATE DATABASE IF NOT EXISTS bank
    CHARACTER SET utf8mb4
    COLLATE utf8mb4_unicode_ci;

USE bank;

-- Drop existing tables (in reverse dependency order)
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS beneficiaries;
DROP TABLE IF EXISTS accounts;
DROP TABLE IF EXISTS customers;
DROP TABLE IF EXISTS atms;
DROP TABLE IF EXISTS branches;

-- Branches
CREATE TABLE branches (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    branch_code VARCHAR(20) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    type ENUM('full', 'limited', 'atm_only', 'hq', 'regional') NOT NULL DEFAULT 'full',
    status ENUM('open', 'closed', 'renovation', 'relocating') NOT NULL DEFAULT 'open',
    address_line1 VARCHAR(255) NOT NULL,
    address_line2 VARCHAR(255),
    city VARCHAR(100) NOT NULL,
    state VARCHAR(100),
    postal_code VARCHAR(20),
    country CHAR(2) NOT NULL,
    latitude DECIMAL(10, 8),
    longitude DECIMAL(11, 8),
    timezone VARCHAR(50) NOT NULL,
    monday_hours VARCHAR(20),
    tuesday_hours VARCHAR(20),
    wednesday_hours VARCHAR(20),
    thursday_hours VARCHAR(20),
    friday_hours VARCHAR(20),
    saturday_hours VARCHAR(20),
    sunday_hours VARCHAR(20),
    phone VARCHAR(30),
    email VARCHAR(255),
    customer_capacity INT DEFAULT 100,
    atm_count INT DEFAULT 2,
    opened_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    closed_at TIMESTAMP NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB;

-- ATMs
CREATE TABLE atms (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    atm_id VARCHAR(20) NOT NULL UNIQUE,
    branch_id BIGINT,
    status ENUM('online', 'offline', 'maintenance', 'out_of_cash') NOT NULL DEFAULT 'online',
    location_name VARCHAR(255),
    address_line1 VARCHAR(255) NOT NULL,
    city VARCHAR(100) NOT NULL,
    state VARCHAR(100),
    postal_code VARCHAR(20),
    country CHAR(2) NOT NULL,
    latitude DECIMAL(10, 8),
    longitude DECIMAL(11, 8),
    timezone VARCHAR(50) NOT NULL,
    supports_deposit BOOLEAN DEFAULT FALSE,
    supports_transfer BOOLEAN DEFAULT FALSE,
    is_24_hours BOOLEAN DEFAULT TRUE,
    avg_daily_transactions INT DEFAULT 50,
    installed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB;

-- Customers
CREATE TABLE customers (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    email VARCHAR(255) NOT NULL,
    phone VARCHAR(30),
    date_of_birth DATE,
    address_line1 VARCHAR(255),
    address_line2 VARCHAR(255),
    city VARCHAR(100),
    state VARCHAR(100),
    postal_code VARCHAR(20),
    country CHAR(2) NOT NULL,
    timezone VARCHAR(50) NOT NULL,
    home_branch_id BIGINT,
    segment ENUM('regular', 'premium', 'private', 'business', 'corporate') NOT NULL DEFAULT 'regular',
    status ENUM('active', 'inactive', 'suspended', 'closed') NOT NULL DEFAULT 'active',
    activity_score DECIMAL(3, 2) DEFAULT 0.50,
    username VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    pin VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB;

-- Accounts
CREATE TABLE accounts (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    account_number VARCHAR(30) NOT NULL UNIQUE,
    customer_id BIGINT NOT NULL,
    type ENUM('checking', 'savings', 'credit_card', 'loan', 'mortgage',
              'investment', 'business', 'merchant', 'payroll') NOT NULL,
    status ENUM('active', 'dormant', 'frozen', 'closed', 'pending') NOT NULL DEFAULT 'active',
    currency CHAR(3) NOT NULL DEFAULT 'USD',
    balance BIGINT NOT NULL DEFAULT 0,
    credit_limit BIGINT DEFAULT 0,
    overdraft_limit BIGINT DEFAULT 0,
    daily_withdraw_limit BIGINT DEFAULT 50000,
    daily_transfer_limit BIGINT DEFAULT 500000,
    interest_rate INT DEFAULT 0,
    branch_id BIGINT,
    opened_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    closed_at TIMESTAMP NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB;

-- Beneficiaries
CREATE TABLE beneficiaries (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    customer_id BIGINT NOT NULL,
    nickname VARCHAR(100),
    name VARCHAR(255) NOT NULL,
    type ENUM('individual', 'business', 'utility', 'government') NOT NULL DEFAULT 'individual',
    status ENUM('pending', 'verified', 'blocked') NOT NULL DEFAULT 'verified',
    bank_name VARCHAR(255),
    bank_code VARCHAR(20),
    routing_number VARCHAR(20),
    account_number VARCHAR(50),
    iban VARCHAR(50),
    address_line1 VARCHAR(255),
    address_line2 VARCHAR(255),
    city VARCHAR(100),
    state VARCHAR(100),
    postal_code VARCHAR(20),
    country CHAR(2),
    currency CHAR(3) DEFAULT 'USD',
    payment_method ENUM('ach', 'wire', 'internal') DEFAULT 'ach',
    account_reference VARCHAR(100),
    last_used_at TIMESTAMP NULL,
    transfer_count INT DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB;

-- Transactions (no indexes for fast bulk insert)
CREATE TABLE transactions (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    reference_number VARCHAR(30) NOT NULL UNIQUE,
    account_id BIGINT NOT NULL,
    counterparty_account_id BIGINT,
    beneficiary_id BIGINT,
    type ENUM('deposit', 'salary', 'transfer_in', 'interest_credit', 'refund', 'cashback',
              'withdrawal', 'purchase', 'transfer_out', 'bill_payment', 'interest_debit',
              'fee', 'loan_payment', 'payroll_batch') NOT NULL,
    status ENUM('pending', 'completed', 'failed', 'reversed', 'declined') NOT NULL DEFAULT 'completed',
    channel ENUM('online', 'atm', 'branch', 'pos', 'ach', 'wire', 'internal') NOT NULL,
    amount BIGINT NOT NULL,
    currency CHAR(3) NOT NULL DEFAULT 'USD',
    balance_after BIGINT NOT NULL,
    description VARCHAR(500),
    metadata JSON,
    branch_id BIGINT,
    atm_id BIGINT,
    linked_transaction_id BIGINT,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    posted_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    value_date DATE NOT NULL,
    failure_reason VARCHAR(255)
) ENGINE=InnoDB;

-- Audit logs (no indexes for fast bulk insert)
CREATE TABLE audit_logs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    customer_id BIGINT,
    employee_id BIGINT,
    system_id VARCHAR(100),
    action ENUM(
        'login_success', 'login_failed', 'logout', 'pin_success', 'pin_failed',
        'password_changed', 'account_locked',
        'transaction_initiated', 'transaction_completed', 'transaction_failed', 'transaction_declined',
        'account_opened', 'account_closed', 'account_updated',
        'beneficiary_added', 'beneficiary_removed',
        'profile_viewed', 'profile_updated', 'address_changed', 'contact_changed',
        'session_started', 'session_ended', 'session_timeout',
        'balance_inquiry', 'statement_viewed', 'history_viewed'
    ) NOT NULL,
    outcome ENUM('success', 'failure', 'denied', 'error') NOT NULL,
    channel ENUM('online', 'atm', 'branch', 'mobile', 'phone', 'api', 'system') NOT NULL,
    branch_id BIGINT,
    atm_id BIGINT,
    ip_address VARCHAR(45),
    user_agent VARCHAR(500),
    account_id BIGINT,
    transaction_id BIGINT,
    beneficiary_id BIGINT,
    description VARCHAR(500),
    failure_reason VARCHAR(255),
    metadata JSON,
    session_id VARCHAR(100),
    risk_score DECIMAL(3, 2),
    request_id VARCHAR(100)
) ENGINE=InnoDB;

-- Disable foreign key checks for bulk loading
-- SET FOREIGN_KEY_CHECKS = 0;
-- After loading, re-enable:
-- SET FOREIGN_KEY_CHECKS = 1;
