The `DeleteExpressionMutation` replaces an expression with another one. For example
instead of calling function `deposit()`, the original code has been mutated to `assert(true)`, 
which is a no-op. Since the test suite passed with the modified code, it means that a change to the
business logic was NOT caught. Unit tests are specification of the business
logic. Any change to the code should be caught by the test suite.
