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

**Input Code Diff**:
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

**Input Function Context**:
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


**Example 3**:

**Input Code Diff**:
```diff
--- original
+++ mutant
@@ -256,7 +256,8 @@

         // Ensure there is no TVL for the specified token in the Operator Delegators
         for (uint i = 0; i < operatorDelegatorTokenTVLs.length; ) {
-            if (operatorDelegatorTokenTVLs[i][collateralTokenIndex] > 0) {
+            /// IfStatementMutation(`operatorDelegatorTokenTVLs[i][collateralTokenIndex] > 0` |==> `false`) of: `if (operatorDelegatorTokenTVLs[i][collateralTokenIndex] > 0) {`
+            if (false) {
                 revert InvalidTVL();
             }
             unchecked {

```

**Input Function Context**:
```solidity
        // Verify the token has 18 decimal precision - pricing calculations will be off otherwise
        if (IERC20Metadata(address(_newCollateralToken)).decimals() != 18)
            revert InvalidTokenDecimals(
                18,
                IERC20Metadata(address(_newCollateralToken)).decimals()
            );

        // Add it to the list
        collateralTokens.push(_newCollateralToken);

        emit CollateralTokenAdded(_newCollateralToken);
    }

    /// @dev Allows restake manager to remove a collateral token
    function removeCollateralToken(
        IERC20 _collateralTokenToRemove
    ) external onlyRestakeManagerAdmin {
        // Get the token index - will revert if not found
        uint256 collateralTokenIndex = getCollateralTokenIndex(_collateralTokenToRemove);

        // Get the token TVLs of the ODs
        (uint256[][] memory operatorDelegatorTokenTVLs, , ) = calculateTVLs();

        // Ensure there is no TVL for the specified token in the Operator Delegators
        for (uint i = 0; i < operatorDelegatorTokenTVLs.length; ) {
            /// IfStatementMutation(`operatorDelegatorTokenTVLs[i][collateralTokenIndex] > 0` |==> `false`) of: `if (operatorDelegatorTokenTVLs[i][collateralTokenIndex] > 0) {`
            if (false) {
                revert InvalidTVL();
            }
            unchecked {
                ++i;
```

###Desired_Output###

In the `removeCollateralToken(...)` function, the `if` statement condition: `operatorDelegatorTokenTVLs[i][collateralTokenIndex] > 0` can be
hardcoded to `false` without affecting the test suite. Consider adding test
cases for the revert with the `InvalidTVL` error.
