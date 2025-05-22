**Example 1**:

**Input Code Diff**:
```diff
--- original
+++ mutant
@@ -604,7 +604,8 @@
     function validateProposedTx(bytes calldata proposedTx, uint8 proposalTypeId, bytes24 key) public view {
         Scope[] memory scopes = _assignedScopes[proposalTypeId][key];

-        for (uint8 i = 0; i < scopes.length; i++) {
+        /// SwapArgumentsOperatorMutation(`i < scopes.length` |==> `scopes.length < i`) of: `for (uint8 i = 0; i < scopes.length; i++) {`
+        for (uint8 i = 0; scopes.length < i; i++) {
             Scope memory validScope = scopes[i];
             if (validScope.selector != bytes4(proposedTx[:4])) revert Invalid4ByteSelector();
```

**Input Function Context**:
```solidity
     * @param idx the index of the assigned scope.
     */
    function deleteScope(uint8 proposalTypeId, bytes24 scopeKey, uint8 idx) external override onlyAdminOrTimelock {
        Scope[] storage scopeArr = _assignedScopes[proposalTypeId][scopeKey];

        scopeArr[idx] = scopeArr[scopeArr.length - 1];
        scopeArr.pop();

        emit ScopeDeleted(proposalTypeId, scopeKey);
    }

    /**
     * @notice Validates that a proposed transaction conforms to the scope defined in a given proposal type. Note: This
     *   version only supports functions that have for each parameter 32-byte abi encodings, please see the ABI
     *   specification to see which types are not supported. The types that are supported are as follows:
     *      - Uint
     *      - Address
     *      - Bytes32
     * @param proposedTx The calldata of the proposed transaction
     * @param proposalTypeId Id of the proposal type
     * @param key A type signature of a function and contract address that has a limit specified in a scope
     */
    function validateProposedTx(bytes calldata proposedTx, uint8 proposalTypeId, bytes24 key) public view {
        Scope[] memory scopes = _assignedScopes[proposalTypeId][key];

        /// SwapArgumentsOperatorMutation(`i < scopes.length` |==> `scopes.length < i`) of: `for (uint8 i = 0; i < scopes.length; i++) {`
        for (uint8 i = 0; scopes.length < i; i++) {
            Scope memory validScope = scopes[i];
            if (validScope.selector != bytes4(proposedTx[:4])) revert Invalid4ByteSelector();

            if (validScope.exists) {
```

###Desired_Output###

In the `validateProposedTx(...)` function, the loop condition `i < scopes.length` can be changed to `scopes.length < i` without affecting the test suite. Consider adding test cases that ensure the loop performs the correct number of iterations.


**Example 2**:

**Input Code Diff**:
```diff
--- original
+++ mutant
@@ -18,7 +18,8 @@
     uint160 internal constant BEFORE_QUORUM_CALCULATION_FLAG = 1 << 11;
     uint160 internal constant AFTER_QUORUM_CALCULATION_FLAG = 1 << 10;

-    uint160 internal constant BEFORE_VOTE_FLAG = 1 << 9;
+    /// SwapArgumentsOperatorMutation(`1 << 9` |==> `9 << 1`) of: `uint160 internal constant BEFORE_VOTE_FLAG = 1 << 9;`
+    uint160 internal constant BEFORE_VOTE_FLAG = 9 << 1;
     uint160 internal constant AFTER_VOTE_FLAG = 1 << 8;

     uint160 internal constant BEFORE_PROPOSE_FLAG = 1 << 7;

```

**Input Function Context**:
```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import {IHooks} from "src/interfaces/IHooks.sol";

/// Inspired in the https://github.com/Uniswap/v4-core/blob/main/src/libraries/Hooks.sol[Uniswap v4 implementation of hooks].
library Hooks {
    using Hooks for IHooks;

    uint160 internal constant ALL_HOOK_MASK = uint160((1 << 16) - 1);

    uint160 internal constant BEFORE_INITIALIZE_FLAG = 1 << 15;
    uint160 internal constant AFTER_INITIALIZE_FLAG = 1 << 14;

    uint160 internal constant BEFORE_VOTE_SUCCEEDED_FLAG = 1 << 13;
    uint160 internal constant AFTER_VOTE_SUCCEEDED_FLAG = 1 << 12;

    uint160 internal constant BEFORE_QUORUM_CALCULATION_FLAG = 1 << 11;
    uint160 internal constant AFTER_QUORUM_CALCULATION_FLAG = 1 << 10;

    /// SwapArgumentsOperatorMutation(`1 << 9` |==> `9 << 1`) of: `uint160 internal constant BEFORE_VOTE_FLAG = 1 << 9;`
    uint160 internal constant BEFORE_VOTE_FLAG = 9 << 1;
    uint160 internal constant AFTER_VOTE_FLAG = 1 << 8;

    uint160 internal constant BEFORE_PROPOSE_FLAG = 1 << 7;
    uint160 internal constant AFTER_PROPOSE_FLAG = 1 << 6;
```

###Desired_Output###

In the definition of the `BEFORE_VOTE_FLAG` constant, the operands in the expression `1 << 9` can be swapped to `9 << 1` without affecting the test suite.
Consider adding test cases that depend on the value of this constant.
