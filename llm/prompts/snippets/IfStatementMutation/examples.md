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


**Example 4**:

**Input Code Diff**:
```diff
--- original
+++ mutant
@@ -119,7 +119,8 @@
         IERC20 _token,
         uint256 _value
     ) public view returns (uint256) {
-        if (_token == stETH) {
+        /// IfStatementMutation(`_token == stETH` |==> `true`) of: `if (_token == stETH) {`
+        if (true) {
             // if stETH secondary Oracle is not set then return 1:1
             if (address(stETHSecondaryOracle) == address(0)) return _value;


```

**Input Function Context**:
```solidity
    function lookupTokenSecondaryAmountFromValue(
        IERC20 _token,
        uint256 _value
    ) public view returns (uint256) {
        /// IfStatementMutation(`_token == stETH` |==> `true`) of: `if (_token == stETH) {`
        if (true) {
            // if stETH secondary Oracle is not set then return 1:1
            if (address(stETHSecondaryOracle) == address(0)) return _value;

            // check the last price
```

###Desired_Output###

In the `lookupTokenSecondaryAmountFromValue(...)` function, the `if` statement condition: `_token == stETH` can be hardcoded to `true` without affecting the test suite. Consider adding tests for cases when the logic from the `if` branch of this statement is not executed.


**Example 5**:

**Input Code Diff**:
```diff
--- original
+++ mutant
@@ -99,7 +99,8 @@
         IERC20 _token,
         uint256 _balance
     ) public view returns (uint256) {
-        if (_token == stETH) {
+        /// IfStatementMutation(`_token == stETH` |==> `false`) of: `if (_token == stETH) {`
+        if (false) {
             // if stETH secondary Oracle is not set then return 1:1
             if (address(stETHSecondaryOracle) == address(0)) return _balance;


```

**Input Function Context**:
```solidity
    /**
     * @notice  calculate stETH value in terms of ETH through market rate~
     * @param   _balance  amount of stETH to convert in ETH
     * @return  uint256  stETH value in ETH through secondary exchange rate (DEX price)
     */
    function lookupTokenSecondaryValue(
        IERC20 _token,
        uint256 _balance
    ) public view returns (uint256) {
        /// IfStatementMutation(`_token == stETH` |==> `false`) of: `if (_token == stETH) {`
        if (false) {
            // if stETH secondary Oracle is not set then return 1:1
            if (address(stETHSecondaryOracle) == address(0)) return _balance;

            // check the last price
```

###Desired_Output###

In the `lookupTokenSecondaryValue(...)` function, the `if` statement condition: `_token == stETH` can be hardcoded to `false` without affecting the test suite. Consider adding test
cases for the logic executed in the `if` branch of this statement.


**Example 6**:

**Input Code Diff**:
```diff
--- original
+++ mutant
@@ -112,7 +112,8 @@
         // Route hook to voting module
         if (module != address(0)) {
             Hooks.Permissions memory hooks = BaseHook(module).getHookPermissions();
-            if (hooks.beforeVoteSucceeded) {
+            /// IfStatementMutation(`hooks.beforeVoteSucceeded` |==> `true`) of: `if (hooks.beforeVoteSucceeded) {`
+            if (true) {
                 (, voteSucceeded) = BaseHook(module).beforeVoteSucceeded(msg.sender, proposalId);
             }
         }

```

**Input Function Context**:
```solidity
    /// @inheritdoc IHooks
    function beforeVoteSucceeded(address, /* sender */ uint256 proposalId)
        external
        view
        override
        returns (bytes4, bool voteSucceeded)
    {
        voteSucceeded = false;
        uint8 proposalTypeId = _proposalTypeId[proposalId];
        _proposalTypeExists(proposalTypeId);

        address module = _proposalTypes[proposalTypeId].module;

        // Route hook to voting module
        if (module != address(0)) {
            Hooks.Permissions memory hooks = BaseHook(module).getHookPermissions();
            /// IfStatementMutation(`hooks.beforeVoteSucceeded` |==> `true`) of: `if (hooks.beforeVoteSucceeded) {`
            if (true) {
                (, voteSucceeded) = BaseHook(module).beforeVoteSucceeded(msg.sender, proposalId);
            }
        }

```

###Desired_Output###

In the `beforeVoteSucceeded(...)` function, the `if` statement condition: `hooks.beforeVoteSucceeded` can be hardcoded to `true` without affecting the test suite. Consider adding tests for cases when the logic from the `if` branch of this statement is not executed.

