# User Stories (Customer & Bank Staff)

Each user story has corresponding tests in the [`internal/simulator/userstory/`](../internal/simulator/userstory/) package that validate the load generation workers perform these actions correctly.

## Retail & Business Customers

- As a retail customer, I want to log in to online banking and see my balances quickly so that I can confirm funds before acting.
  - Acceptance: credentials validated; failed login logged; balance query returns per-account balances within expected latency.
  - **Test:** [`TestUS_RetailCustomer_LoginAndViewBalances`](../internal/simulator/userstory/retail_test.go#L35)

- As a retail customer, I want to view recent transactions (e.g., last 30 days or last 10 items) so that I can verify activity.
  - Acceptance: history is ordered by timestamp; shows amount, type, memo, channel/ATM/branch; works for multiple accounts.
  - **Test:** [`TestUS_RetailCustomer_ViewTransactionHistory`](../internal/simulator/userstory/retail_test.go#L113)

- As a retail customer, I want to transfer funds between my own accounts or to saved beneficiaries so that I can pay bills or move money.
  - Acceptance: source/destination selection; amount validation and insufficient-funds handling; paired debit/credit entries for internal transfers; audit entry recorded.
  - **Test:** [`TestUS_RetailCustomer_FundsTransfer`](../internal/simulator/userstory/retail_test.go#L169)

- As a retail customer, I want to withdraw cash at an ATM after a quick balance check so that I know how much I can take out.
  - Acceptance: ATM session includes balance inquiry; withdrawal debits account and notes ATM ID; receipt/memo captured; failures (insufficient funds/out-of-service) are logged.
  - **Test:** [`TestUS_RetailCustomer_ATMWithdrawal`](../internal/simulator/userstory/retail_test.go#L271)

- As a retail customer, I want to deposit cash/checks via ATM or branch so that my balance increases and is immediately visible.
  - Acceptance: deposit credits account with channel metadata; updated balance visible on next inquiry; audit entry recorded.
  - **Test:** [`TestUS_RetailCustomer_ATMDeposit`](../internal/simulator/userstory/retail_test.go#L337)

- As a retail customer, I want to add a new payee/beneficiary so that I can send external payments later.
  - Acceptance: beneficiary details validated; saved to my profile; subsequent transfers can target the new beneficiary.
  - **Test:** *Coverage via beneficiary error classification in [`TestUS_Customer_FailureOutcomes`](../internal/simulator/userstory/retail_test.go#L398)*

- As a business customer, I want to receive many small incoming payments (merchant flows) and periodically sweep funds to another account so that my operating balance stays controlled.
  - Acceptance: bulk incoming credits recorded with realistic memos; sweeps post as transfers with paired entries; activity visible in history and balances.
  - **Test:** [`TestUS_BusinessCustomer_MerchantFlows`](../internal/simulator/userstory/business_test.go#L31)

- As a payroll operator (business customer), I want to run end-of-month salary batches to many employees so that payroll completes on schedule.
  - Acceptance: batch creates many outgoing credits to employee accounts; spike handled without dropping transactions; audit trail links batch reference to each payment.
  - **Test:** [`TestUS_PayrollOperator_BatchPayroll`](../internal/simulator/userstory/business_test.go#L92)

- As a customer in any country/time zone, I want activity to occur during my local daytime (8:00–16:00) so that usage patterns feel natural.
  - Acceptance: session likelihood weighted to local hours; weekend/weekday differences applied; intraday peaks (morning check, lunch ATM, end-of-day rush) present.
  - **Test:** [`TestUS_Customer_TimezoneAwareness`](../internal/simulator/userstory/timezone_test.go#L28)

- As a customer, I want clear outcomes for failed actions (wrong password, insufficient funds, invalid account) so that I understand what happened.
  - Acceptance: no ledger changes on failed attempts; descriptive error surfaced; failure recorded in audit logs with reason.
  - **Test:** [`TestUS_Customer_FailureOutcomes`](../internal/simulator/userstory/retail_test.go#L398)

## Bank Staff (Ops, Compliance, Support)

- As an operations analyst, I want every customer action (success or failure) captured in audit logs with who/what/when/where/outcome so that I can trace events.
  - Acceptance: audit includes customer/account/transaction IDs, channel (online/ATM/branch), location/IP or ATM/branch ID, timestamp, and status; no action lacks an entry.
  - **Test:** [`TestUS_OpsAnalyst_AuditTrail`](../internal/simulator/userstory/staff_test.go#L35)

- As a fraud/compliance investigator, I want to review high-risk behaviors (repeated failed logins, bursts, large transfers) so that I can detect anomalies.
  - Acceptance: flagged events identifiable from audit/transaction data; reason codes stored; ability to correlate customer, session time, and channel.
  - **Test:** [`TestUS_FraudInvestigator_HighRiskBehaviors`](../internal/simulator/userstory/staff_test.go#L125)

- As a branch/ATM manager, I want transactions to tag the originating branch/ATM so that I can monitor device usage and outages.
  - Acceptance: ATM/branch ID present on withdrawals/deposits; outages or out-of-service events logged; traffic shows daily lunch spikes.
  - **Test:** [`TestUS_BranchATMManager_DeviceTracking`](../internal/simulator/userstory/staff_test.go#L189)

- As a customer support agent, I want to replay a customer session timeline (logins, balance checks, transfers, failures) so that I can resolve disputes.
  - Acceptance: ordered audit/transaction view per customer; includes memos/descriptions; failed attempts visible alongside successful ones.
  - **Test:** [`TestUS_SupportAgent_SessionTimeline`](../internal/simulator/userstory/staff_test.go#L233)

- As a performance/SRE engineer, I want visibility into bursts (payroll days, random spikes) and sustained load so that I can assess capacity.
  - Acceptance: load patterns reflect weekday/weekend and monthly peaks; metrics show TPS/ops counts over time; seeded runs are reproducible for comparison.
  - **Test:** [`TestUS_SREEngineer_LoadPatterns`](../internal/simulator/userstory/staff_test.go#L293)
