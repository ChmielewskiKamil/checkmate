**Example 1**:

**Input Code Diff**:
```diff
--- original
+++ mutant
@@ -347,7 +347,8 @@
         address module = _proposalTypes[proposalTypeId].module;
 
         // Route hook to voting module
-        if (module != address(0)) {
+        /// IfStatementMutation(`module != address(0)` |==> `false`) of: `if (module != address(0)) {`
+        if (false) {
             Hooks.Permissions memory hooks = BaseHook(module).getHookPermissions();
             if (hooks.afterCancel) {
                 BaseHook(module).afterCancel(msg.sender, proposalId, targets, values, calldatas, descriptionHash);
```

**Input Function Context**:
```solidity
            (, proposalId) = BaseHook(module).beforeCancel(msg.sender, targets, values, calldatas, descriptionHash);
            }
        }
        return (this.beforeCancel.selector, proposalId);
    }

    function afterCancel(
        address,
        uint256 proposalId,
        address[] memory targets,
        uint256[] memory values,
        bytes[] memory calldatas,
        bytes32 descriptionHash
    ) external override returns (bytes4) {
        uint8 proposalTypeId = _proposalTypeId[proposalId];
        _proposalTypeExists(proposalTypeId);

        address module = _proposalTypes[proposalTypeId].module;

        /// IfStatementMutation(`module != address(0)` |==> `false`) of: `if (module != address(0)) {`
        if (false) {
            Hooks.Permissions memory hooks = BaseHook(module).getHookPermissions();
            if (hooks.afterCancel) {
                BaseHook(module).afterCancel(msg.sender, proposalId, targets, values, calldatas, descriptionHash);
            }
        }

        return this.afterCancel.selector;
    }

    /// @inheritdoc IHooks
```

###Desired_Output###

In the `afterCancel(...)` function, the `if` statement condition: `module != address(0)` can be
hardcoded to `false` without affecting the test suite. Consider adding test
cases for the logic executed in the `if` branch of this statement.


**Example 2**:

Code Diff:
```diff
--- original
+++ mutant
@@ -385,7 +385,8 @@
             );

             // The length of the result must be 64 bytes to return a bytes4 (padded to 32 bytes) and a uint256 (32 bytes) proposal ID value
-            if (result.length != 64) revert InvalidHookResponse();
+            /// IfStatementMutation(`result.length != 64` |==> `false`) of: `if (result.length != 64) revert InvalidHookResponse();`
+            if (false) revert InvalidHookResponse();

             // Extract the proposal ID from the result
             returnedProposalId = parseUint256(result);
```

Function Context:
```solidity
        uint256[] memory values,
        bytes[] memory calldatas,
        bytes32 descriptionHash
    ) internal noSelfCall(self) {
        if (self.hasPermission(AFTER_QUEUE_FLAG)) {
            self.callHook(
                abi.encodeCall(IHooks.afterQueue, (msg.sender, proposalId, targets, values, calldatas, descriptionHash))
            );
        }
    }

    /// @notice calls beforeCancel hook if permissioned and validates return value
    function beforeCancel(
        IHooks self,
        address[] memory targets,
        uint256[] memory values,
        bytes[] memory calldatas,
        bytes32 descriptionHash
    ) internal noSelfCall(self) returns (uint256 returnedProposalId) {
        if (self.hasPermission(BEFORE_CANCEL_FLAG)) {
            bytes memory result = self.callHook(
                abi.encodeCall(IHooks.beforeCancel, (msg.sender, targets, values, calldatas, descriptionHash))
            );

            // The length of the result must be 64 bytes to return a bytes4 (padded to 32 bytes) and a uint256 (32 bytes) proposal ID value
            /// IfStatementMutation(`result.length != 64` |==> `false`) of: `if (result.length != 64) revert InvalidHookResponse();`
            if (false) revert InvalidHookResponse();

            // Extract the proposal ID from the result
            returnedProposalId = parseUint256(result);
        }
```

###Desired_Output###

In the `beforeCancel(...)` function, the `if` statement condition: `result.length != 64` can be
hardcoded to `false` without affecting the test suite. Consider adding test
cases for the revert with the `InvalidHookResponse` error.
