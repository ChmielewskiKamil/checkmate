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
