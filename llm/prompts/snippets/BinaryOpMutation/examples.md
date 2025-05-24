**Example 1**:

**Input Code Diff**:
```diff
--- original
+++ mutant
@@ -374,7 +374,8 @@
         _proposals[proposalId] = helper.hash(targets, values, calldatas, 0, salt);
         _timelock.scheduleBatch(targets, values, calldatas, 0, salt, delay);

-        return block.timestamp + deadlie;
+        /// BinaryOpMutation(`+` |==> `*`) of: `return block.timestamp + deadline;`
+        return block.timestamp + deadline;
     }
```

**Input Function Context**:
```solidity
    function _setAdmin(address _newAdmin) internal {
        emit AdminSet(admin, _newAdmin);
        admin = _newAdmin;
    }

    function _setManager(address _newManager) internal {
        emit ManagerSet(manager, _newManager);
        manager = _newManager;
    }

    // @notice See Governor.sol replicates the logic to handle modified calldata from hooks
    function _queueOperations(
        uint256 proposalId,
        address[] memory targets,
        uint256[] memory values,
        bytes[] memory calldatas,
        bytes32 descriptionHash
    ) internal virtual override returns (uint48) {
        uint256 delay = _timelock.getMinDelay();

        bytes32 salt = _timelockSalt(descriptionHash);
         _proposals[proposalId] = helper.hash(targets, values, calldatas, 0, salt);
         _timelock.scheduleBatch(targets, values, calldatas, 0, salt, delay);

        /// BinaryOpMutation(`+` |==> `*`) of: `return block.timestamp + deadline;`
        return block.timestamp + deadline;
    }

    // @notice See Governor.sol replicates the logic to handle modified calldata from hooks
    function _executeOperations(
```

###Desired_Output###

In the `_queueOperations(...)` function, the addition in the return statement `block.timestamp + deadline` can
be changed to multiplication without affecting the test suite. Consider adding
test cases for the expected return value of this function.


**Example 2**:

**Input Code Diff**:
```diff
--- original
+++ mutant
@@ -275,7 +275,8 @@
      */
     function fillEthWithdrawBuffer() external payable nonReentrant onlyDepositQueue {
         uint256 queueFilled = _checkAndFillWithdrawQueue(IS_NATIVE, msg.value);
-        emit EthBufferFilled(msg.value - queueFilled);
+        /// BinaryOpMutation(`-` |==> `*`) of: `emit EthBufferFilled(msg.value - queueFilled);`
+        emit EthBufferFilled(msg.value*queueFilled);
     }

     /**

```

**Input Function Context**:
```solidity

        // return total deficit
        return bufferDeficit + queueDeficit;
    }

    /**
     * @notice  fill Eth WithdrawBuffer from RestakeManager deposits
     * @dev     permissioned call (onlyDepositQueue)
     */
    function fillEthWithdrawBuffer() external payable nonReentrant onlyDepositQueue {
        uint256 queueFilled = _checkAndFillWithdrawQueue(IS_NATIVE, msg.value);
        /// BinaryOpMutation(`-` |==> `*`) of: `emit EthBufferFilled(msg.value - queueFilled);`
        emit EthBufferFilled(msg.value*queueFilled);
    }

    /**
     * @notice  Fill ERC20 token withdraw buffer from RestakeManager deposits
```

###Desired_Output###

In the `fillEthWithdrawBuffer(...)` function, the subtraction in the `EthBufferFilled(msg.value - queueFilled)`
event emission can be changed to multiplication without affecting the test
suite. Consider adding test cases for the expected value emitted by this event.


**Example 3**:

**Input Code Diff**:
```diff
--- original
+++ mutant
@@ -105,7 +105,8 @@

             // check the last price
             (, int256 price, , uint256 timestamp, ) = stETHSecondaryOracle.latestRoundData();
-            if (timestamp < block.timestamp - MAX_TIME_WINDOW) revert OraclePriceExpired();
+            /// BinaryOpMutation(`-` |==> `/`) of: `if (timestamp < block.timestamp - MAX_TIME_WINDOW) revert OraclePriceExpired();`
+            if (timestamp < block.timestamp/MAX_TIME_WINDOW) revert OraclePriceExpired();
             if (price <= 0) revert InvalidOraclePrice();

             // Price is times 10**18 ensure value amount is scaled

```

**Input Function Context**:
```solidity
        AggregatorV3Interface _oracleAddress
    ) external nonReentrant onlyOracleAdmin {
        // Verify that the pricing of the oracle is 18 decimals - pricing calculations will be off otherwise
        if (_oracleAddress.decimals() != 18)
            revert InvalidTokenDecimals(18, _oracleAddress.decimals());

        stETHSecondaryOracle = _oracleAddress;
        emit StETHSecondaryOracleUpdated(_oracleAddress);
    }

    /**
     * @notice  calculate stETH value in terms of ETH through market rate~
     * @param   _balance  amount of stETH to convert in ETH
     * @return  uint256  stETH value in ETH through secondary exchange rate (DEX price)
     */
    function lookupTokenSecondaryValue(
        IERC20 _token,
        uint256 _balance
    ) public view returns (uint256) {
        if (_token == stETH) {
            // if stETH secondary Oracle is not set then return 1:1
            if (address(stETHSecondaryOracle) == address(0)) return _balance;

            // check the last price
            (, int256 price, , uint256 timestamp, ) = stETHSecondaryOracle.latestRoundData();
            /// BinaryOpMutation(`-` |==> `/`) of: `if (timestamp < block.timestamp - MAX_TIME_WINDOW) revert OraclePriceExpired();`
            if (timestamp < block.timestamp/MAX_TIME_WINDOW) revert OraclePriceExpired();
            if (price <= 0) revert InvalidOraclePrice();

            // Price is times 10**18 ensure value amount is scaled
            return (uint256(price) * _balance) / SCALE_FACTOR;
```

###Desired_Output###

In the `lookupTokenSecondaryValue(...)` function, the subtraction in the condition `if (timestamp < block.timestamp - MAX_TIME_WINDOW)` can be changed to division without affecting the test suite. Consider adding test cases for the expected behavior of the `OraclePriceExpired` revert condition.


**Example 4**:

**Input Code Diff**:
```diff
--- original
+++ mutant
@@ -31,7 +31,8 @@
     uint160 internal constant AFTER_QUEUE_FLAG = 1 << 2;

     uint160 internal constant BEFORE_EXECUTE_FLAG = 1 << 1;
-    uint160 internal constant AFTER_EXECUTE_FLAG = 1 << 0;
+    /// BinaryOpMutation(`<<` |==> `*`) of: `uint160 internal constant AFTER_EXECUTE_FLAG = 1 << 0;`
+    uint160 internal constant AFTER_EXECUTE_FLAG = 1*0;

     struct Permissions {
         bool beforeInitialize;

```

**Input Function Context**:
```solidity
    uint160 internal constant ALL_HOOK_MASK = uint160((1 << 16) - 1);

    uint160 internal constant BEFORE_INITIALIZE_FLAG = 1 << 15;
    uint160 internal constant AFTER_INITIALIZE_FLAG = 1 << 14;

    uint160 internal constant BEFORE_VOTE_SUCCEEDED_FLAG = 1 << 13;
    uint160 internal constant AFTER_VOTE_SUCCEEDED_FLAG = 1 << 12;

    uint160 internal constant BEFORE_QUORUM_CALCULATION_FLAG = 1 << 11;
    uint160 internal constant AFTER_QUORUM_CALCULATION_FLAG = 1 << 10;

    uint160 internal constant BEFORE_VOTE_FLAG = 1 << 9;
    uint160 internal constant AFTER_VOTE_FLAG = 1 << 8;

    uint160 internal constant BEFORE_PROPOSE_FLAG = 1 << 7;
    uint160 internal constant AFTER_PROPOSE_FLAG = 1 << 6;

    uint160 internal constant BEFORE_CANCEL_FLAG = 1 << 5;
    uint160 internal constant AFTER_CANCEL_FLAG = 1 << 4;

    uint160 internal constant BEFORE_QUEUE_FLAG = 1 << 3;
    uint160 internal constant AFTER_QUEUE_FLAG = 1 << 2;

    uint160 internal constant BEFORE_EXECUTE_FLAG = 1 << 1;
    /// BinaryOpMutation(`<<` |==> `*`) of: `uint160 internal constant AFTER_EXECUTE_FLAG = 1 << 0;`
    uint160 internal constant AFTER_EXECUTE_FLAG = 1*0;

    struct Permissions {
        bool beforeInitialize;
        bool afterInitialize;
```

###Desired_Output###

In the definition of the `AFTER_EXECUTE_FLAG` constant, the bitwise left shift operation can be changed to multiplication without affecting the test suite. Consider adding test cases that depend on the value of this constant.


**Example 5**:

**Input Code Diff**:
```diff
--- original
+++ mutant
@@ -91,7 +91,8 @@
         pure
     {
         compare(
-            bytes32(bytes2(param[param.length - 2:param.length])),
+            /// BinaryOpMutation(`-` |==> `/`) of: `bytes32(bytes2(param[param.length - 2:param.length])),`
+            bytes32(bytes2(param[param.length/2:param.length])),
             bytes32(bytes2(scopedParam[scopedParam.length - 2:scopedParam.length])),
             comparison
         );
```

**Input Function Context**:
```solidity
    /**
     * @dev Conforms the uint16 type to the necessary size considerations prior to comparison
     */
    function validate_uint16(bytes calldata param, bytes calldata scopedParam, IMiddleware.Comparators comparison)
        internal
        pure
    {
        compare(
            /// BinaryOpMutation(`-` |==> `/`) of: `bytes32(bytes2(param[param.length - 2:param.length])),`
            bytes32(bytes2(param[param.length/2:param.length])),
            bytes32(bytes2(scopedParam[scopedParam.length - 2:scopedParam.length])),
            comparison
        );
    }
```

###Desired_Output###

In the `validate_uint16(...)` function, the subtraction in the slicing operation `bytes2(param[param.length - 2:param.length])` can be changed to division without affecting the test suite. Consider adding test cases that depend on the expected result of this slicing operation.
