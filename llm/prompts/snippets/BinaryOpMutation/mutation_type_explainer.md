The `BinaryOpMutation` replaces a binary operator with another one. For example
instead of doing `a + b`, the original code has been mutated to `a - b`. Since
the test suite passed with the modified code, it means that a change to the
business logic was NOT caught. Unit tests are specification of the business
logic. Any change to the code should be caught by the test suite.
