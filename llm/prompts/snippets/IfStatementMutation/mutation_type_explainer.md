The `IfStatementMutation` modifies the `if` statement. For example
instead of doing `if (a > b)`, the original code has been mutated to for example 
`if (true)` or `if (b < a)`. 

Since the test suite passed with the modified code, it means that a change to the
business logic was NOT caught. Unit tests are specification of the business
logic. Any change to the code should be caught by the test suite.
