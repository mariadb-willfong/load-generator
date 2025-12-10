-- Bank-in-a-Box Load Generator - Indexes Only
-- Run this AFTER bulk data loading to create indexes
--
-- Usage:
--   mysql -u root -p bank < schema_indexes.sql

USE bank;

-- ============================================
-- CREATE INDEXES
-- ============================================

-- Branches
CREATE INDEX idx_branches_country ON branches(country);
CREATE INDEX idx_branches_status ON branches(status);

-- ATMs
CREATE INDEX idx_atms_status ON atms(status);
CREATE INDEX idx_atms_country ON atms(country);
CREATE INDEX idx_atms_branch ON atms(branch_id);

-- Customers
CREATE INDEX idx_customers_country ON customers(country);
CREATE INDEX idx_customers_segment ON customers(segment);
CREATE INDEX idx_customers_status ON customers(status);
CREATE INDEX idx_customers_email ON customers(email);
CREATE INDEX idx_customers_branch ON customers(home_branch_id);

-- Accounts
CREATE INDEX idx_accounts_customer ON accounts(customer_id);
CREATE INDEX idx_accounts_type ON accounts(type);
CREATE INDEX idx_accounts_status ON accounts(status);
CREATE INDEX idx_accounts_branch ON accounts(branch_id);
CREATE INDEX idx_accounts_currency ON accounts(currency);

-- Beneficiaries
CREATE INDEX idx_beneficiaries_customer ON beneficiaries(customer_id);
CREATE INDEX idx_beneficiaries_status ON beneficiaries(status);
CREATE INDEX idx_beneficiaries_type ON beneficiaries(type);

-- Transactions (critical for performance)
CREATE INDEX idx_transactions_account ON transactions(account_id);
CREATE INDEX idx_transactions_timestamp ON transactions(timestamp);
CREATE INDEX idx_transactions_account_timestamp ON transactions(account_id, timestamp);
CREATE INDEX idx_transactions_type ON transactions(type);
CREATE INDEX idx_transactions_status ON transactions(status);
CREATE INDEX idx_transactions_channel ON transactions(channel);
CREATE INDEX idx_transactions_value_date ON transactions(value_date);
CREATE INDEX idx_transactions_counterparty ON transactions(counterparty_account_id);
CREATE INDEX idx_transactions_beneficiary ON transactions(beneficiary_id);
CREATE INDEX idx_transactions_branch ON transactions(branch_id);
CREATE INDEX idx_transactions_atm ON transactions(atm_id);

-- Audit logs (for compliance and debugging)
CREATE INDEX idx_audit_timestamp ON audit_logs(timestamp);
CREATE INDEX idx_audit_customer ON audit_logs(customer_id);
CREATE INDEX idx_audit_action ON audit_logs(action);
CREATE INDEX idx_audit_outcome ON audit_logs(outcome);
CREATE INDEX idx_audit_channel ON audit_logs(channel);
CREATE INDEX idx_audit_session ON audit_logs(session_id);
CREATE INDEX idx_audit_account ON audit_logs(account_id);
CREATE INDEX idx_audit_transaction ON audit_logs(transaction_id);
CREATE INDEX idx_audit_customer_timestamp ON audit_logs(customer_id, timestamp);

-- ============================================
-- ANALYZE TABLES (update statistics)
-- ============================================

ANALYZE TABLE branches;
ANALYZE TABLE atms;
ANALYZE TABLE customers;
ANALYZE TABLE accounts;
ANALYZE TABLE beneficiaries;
ANALYZE TABLE transactions;
ANALYZE TABLE audit_logs;
