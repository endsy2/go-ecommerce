Review the changes in the current branch against main.

Check specifically for:
- Layer violations (GORM outside repository, logic in handlers)
- Missing transactions around multi-write operations
- Race conditions on stock or balance
- N+1 queries
- Missing authorization checks
- Unvalidated input
- Money handled as float
- Errors swallowed or not wrapped

Be harsh. List problems by severity. Do not fix anything yet.