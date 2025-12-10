-- Bank-in-a-Box Load Generator - Data Loading Script
--
-- Usage:
--   1. Run schema_no_indexes.sql first to create tables
--   2. Update the path below to point to your CSV files
--   3. Run this script: mysql -u root -p bank < load_data.sql
--   4. Run schema_indexes.sql to add indexes
--
-- Note: Adjust the file paths to match your output directory

-- Disable foreign key checks for faster loading
SET FOREIGN_KEY_CHECKS = 0;
SET UNIQUE_CHECKS = 0;
SET AUTOCOMMIT = 0;

-- ============================================
-- LOAD BRANCHES
-- ============================================
LOAD DATA LOCAL INFILE 'branches.csv'
INTO TABLE branches
FIELDS TERMINATED BY ','
ENCLOSED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, branch_code, name, type, status, address_line1, @address_line2, city, @state,
 @postal_code, country, @latitude, @longitude, timezone, @monday_hours, @tuesday_hours,
 @wednesday_hours, @thursday_hours, @friday_hours, @saturday_hours, @sunday_hours,
 @phone, @email, customer_capacity, atm_count, opened_at, @closed_at, updated_at)
SET
    address_line2 = NULLIF(@address_line2, ''),
    state = NULLIF(@state, ''),
    postal_code = NULLIF(@postal_code, ''),
    latitude = NULLIF(@latitude, ''),
    longitude = NULLIF(@longitude, ''),
    monday_hours = NULLIF(@monday_hours, ''),
    tuesday_hours = NULLIF(@tuesday_hours, ''),
    wednesday_hours = NULLIF(@wednesday_hours, ''),
    thursday_hours = NULLIF(@thursday_hours, ''),
    friday_hours = NULLIF(@friday_hours, ''),
    saturday_hours = NULLIF(@saturday_hours, ''),
    sunday_hours = NULLIF(@sunday_hours, ''),
    phone = NULLIF(@phone, ''),
    email = NULLIF(@email, ''),
    closed_at = NULLIF(@closed_at, '');

SELECT 'Branches loaded' AS status, COUNT(*) AS count FROM branches;

-- ============================================
-- LOAD ATMS
-- ============================================
LOAD DATA LOCAL INFILE 'atms.csv'
INTO TABLE atms
FIELDS TERMINATED BY ','
ENCLOSED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, atm_id, @branch_id, status, @location_name, address_line1, city, @state,
 @postal_code, country, @latitude, @longitude, timezone, supports_deposit,
 supports_transfer, is_24_hours, avg_daily_transactions, installed_at, updated_at)
SET
    branch_id = NULLIF(@branch_id, ''),
    location_name = NULLIF(@location_name, ''),
    state = NULLIF(@state, ''),
    postal_code = NULLIF(@postal_code, ''),
    latitude = NULLIF(@latitude, ''),
    longitude = NULLIF(@longitude, '');

SELECT 'ATMs loaded' AS status, COUNT(*) AS count FROM atms;

-- ============================================
-- LOAD CUSTOMERS
-- ============================================
LOAD DATA LOCAL INFILE 'customers.csv'
INTO TABLE customers
FIELDS TERMINATED BY ','
ENCLOSED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, first_name, last_name, email, @phone, @date_of_birth, @address_line1, @address_line2,
 @city, @state, @postal_code, country, timezone, @home_branch_id, segment, status,
 activity_score, username, password_hash, pin, created_at, updated_at)
SET
    phone = NULLIF(@phone, ''),
    date_of_birth = NULLIF(@date_of_birth, ''),
    address_line1 = NULLIF(@address_line1, ''),
    address_line2 = NULLIF(@address_line2, ''),
    city = NULLIF(@city, ''),
    state = NULLIF(@state, ''),
    postal_code = NULLIF(@postal_code, ''),
    home_branch_id = NULLIF(@home_branch_id, '');

SELECT 'Customers loaded' AS status, COUNT(*) AS count FROM customers;

-- ============================================
-- LOAD ACCOUNTS
-- ============================================
LOAD DATA LOCAL INFILE 'accounts.csv'
INTO TABLE accounts
FIELDS TERMINATED BY ','
ENCLOSED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, account_number, customer_id, type, status, currency, balance, credit_limit,
 overdraft_limit, daily_withdraw_limit, daily_transfer_limit, interest_rate,
 @branch_id, opened_at, @closed_at, updated_at)
SET
    branch_id = NULLIF(@branch_id, ''),
    closed_at = NULLIF(@closed_at, '');

SELECT 'Accounts loaded' AS status, COUNT(*) AS count FROM accounts;

-- ============================================
-- LOAD BENEFICIARIES
-- ============================================
LOAD DATA LOCAL INFILE 'beneficiaries.csv'
INTO TABLE beneficiaries
FIELDS TERMINATED BY ','
ENCLOSED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, customer_id, @nickname, name, type, status, @bank_name, @bank_code, @routing_number,
 @account_number, @iban, @address_line1, @address_line2, @city, @state, @postal_code,
 @country, currency, payment_method, @account_reference, @last_used_at, transfer_count,
 created_at, updated_at)
SET
    nickname = NULLIF(@nickname, ''),
    bank_name = NULLIF(@bank_name, ''),
    bank_code = NULLIF(@bank_code, ''),
    routing_number = NULLIF(@routing_number, ''),
    account_number = NULLIF(@account_number, ''),
    iban = NULLIF(@iban, ''),
    address_line1 = NULLIF(@address_line1, ''),
    address_line2 = NULLIF(@address_line2, ''),
    city = NULLIF(@city, ''),
    state = NULLIF(@state, ''),
    postal_code = NULLIF(@postal_code, ''),
    country = NULLIF(@country, ''),
    account_reference = NULLIF(@account_reference, ''),
    last_used_at = NULLIF(@last_used_at, '');

SELECT 'Beneficiaries loaded' AS status, COUNT(*) AS count FROM beneficiaries;

-- ============================================
-- LOAD TRANSACTIONS
-- ============================================
LOAD DATA LOCAL INFILE 'transactions.csv'
INTO TABLE transactions
FIELDS TERMINATED BY ','
ENCLOSED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, reference_number, account_id, @counterparty_account_id, @beneficiary_id,
 type, status, channel, amount, currency, balance_after, description, @metadata,
 @branch_id, @atm_id, @linked_transaction_id, timestamp, posted_at, value_date,
 @failure_reason)
SET
    counterparty_account_id = NULLIF(@counterparty_account_id, ''),
    beneficiary_id = NULLIF(@beneficiary_id, ''),
    metadata = NULLIF(@metadata, ''),
    branch_id = NULLIF(@branch_id, ''),
    atm_id = NULLIF(@atm_id, ''),
    linked_transaction_id = NULLIF(@linked_transaction_id, ''),
    failure_reason = NULLIF(@failure_reason, '');

SELECT 'Transactions loaded' AS status, COUNT(*) AS count FROM transactions;

-- ============================================
-- LOAD AUDIT LOGS
-- ============================================
LOAD DATA LOCAL INFILE 'audit_logs.csv'
INTO TABLE audit_logs
FIELDS TERMINATED BY ','
ENCLOSED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, timestamp, @customer_id, @employee_id, @system_id, action, outcome, channel,
 @branch_id, @atm_id, @ip_address, @user_agent, @account_id, @transaction_id,
 @beneficiary_id, @description, @failure_reason, @metadata, @session_id, @risk_score,
 @request_id)
SET
    customer_id = NULLIF(@customer_id, ''),
    employee_id = NULLIF(@employee_id, ''),
    system_id = NULLIF(@system_id, ''),
    branch_id = NULLIF(@branch_id, ''),
    atm_id = NULLIF(@atm_id, ''),
    ip_address = NULLIF(@ip_address, ''),
    user_agent = NULLIF(@user_agent, ''),
    account_id = NULLIF(@account_id, ''),
    transaction_id = NULLIF(@transaction_id, ''),
    beneficiary_id = NULLIF(@beneficiary_id, ''),
    description = NULLIF(@description, ''),
    failure_reason = NULLIF(@failure_reason, ''),
    metadata = NULLIF(@metadata, ''),
    session_id = NULLIF(@session_id, ''),
    risk_score = NULLIF(@risk_score, ''),
    request_id = NULLIF(@request_id, '');

SELECT 'Audit logs loaded' AS status, COUNT(*) AS count FROM audit_logs;

-- ============================================
-- FINALIZE
-- ============================================
COMMIT;
SET UNIQUE_CHECKS = 1;
SET FOREIGN_KEY_CHECKS = 1;

-- Summary
SELECT 'Loading complete!' AS status;
SELECT
    (SELECT COUNT(*) FROM branches) AS branches,
    (SELECT COUNT(*) FROM atms) AS atms,
    (SELECT COUNT(*) FROM customers) AS customers,
    (SELECT COUNT(*) FROM accounts) AS accounts,
    (SELECT COUNT(*) FROM beneficiaries) AS beneficiaries,
    (SELECT COUNT(*) FROM transactions) AS transactions,
    (SELECT COUNT(*) FROM audit_logs) AS audit_logs;
