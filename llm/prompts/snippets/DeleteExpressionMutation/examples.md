**Example 1**:

**Input Code Diff**:
```diff
--- original
+++ mutant
@@ -131,7 +131,8 @@
             revert IGovernor.GovernorInvalidProposalLength(targets.length, calldatas.length, values.length);
         }

-        hooks.beforePropose(targets, values, calldatas, description);
+        /// DeleteExpressionMutation(`hooks.beforePropose(targets, values, calldatas, description)` |==> `assert(true)`) of: `hooks.beforePropose(targets, values, calldatas, description);`
+        assert(true);

         proposalId = super.propose(targets, values, calldatas, description);


```

**Input Function Context**:
```solidity
        admin = _newAdmin;
    }

    /**
     * @notice Set the manager address. Only the admin or timelock can call this function.
     * @param _newManager The new manager address.
     */
    function setManager(address _newManager) external onlyGovernance {
        emit ManagerSet(manager, _newManager);
        manager = _newManager;
    }

    /**
     * @inheritdoc Governor
     */
    function propose(
        address[] memory targets,
        uint256[] memory values,
        bytes[] memory calldatas,
        string memory description
    ) public virtual override returns (uint256 proposalId) {
        if (targets.length != values.length || targets.length != calldatas.length || targets.length == 0) {
            revert IGovernor.GovernorInvalidProposalLength(targets.length, calldatas.length, values.length);
        }

        /// DeleteExpressionMutation(`hooks.beforePropose(targets, values, calldatas, description)` |==> `assert(true)`) of: `hooks.beforePropose(targets, values, calldatas, description);`
        assert(true);

        proposalId = super.propose(targets, values, calldatas, description);

        hooks.afterPropose(proposalId, targets, values, calldatas, description);
```

###Desired_Output###

In the `propose(...)` function, the `hooks.beforePropose(...)` call can be replaced with `assert(true)` (which is effectively a no-op) without affecting the test suite. Consider adding test cases to check the expected effects of the original `hooks.beforePropose(...)` call.


**Example 2**:

**Input Code Diff**:
```diff
--- original
+++ mutant
@@ -584,7 +584,8 @@
     function deleteScope(uint8 proposalTypeId, bytes24 scopeKey, uint8 idx) external override onlyAdminOrTimelock {
         Scope[] storage scopeArr = _assignedScopes[proposalTypeId][scopeKey];

-        scopeArr[idx] = scopeArr[scopeArr.length - 1];
+        /// DeleteExpressionMutation(`scopeArr[idx] = scopeArr[scopeArr.length - 1]` |==> `assert(true)`) of: `scopeArr[idx] = scopeArr[scopeArr.length - 1];`
+        assert(true);
         scopeArr.pop();

         emit ScopeDeleted(proposalTypeId, scopeKey);
```

**Input Function Context**:
```solidity
        emit ProposalTypeSet(proposalTypeId, quorum, approvalThreshold, name, description, module);
    }

    /**
     * @notice Disables a scopes for all contract + function signatures.
     * @param proposalTypeId the proposal type ID that has the assigned scope.
     * @param scopeKey the contract and function signature representing the scope key
     * @param idx the index of the assigned scope.
     */
    function disableScope(uint8 proposalTypeId, bytes24 scopeKey, uint8 idx) external override onlyAdminOrTimelock {
        _assignedScopes[proposalTypeId][scopeKey][idx].exists = false;
        _scopeExists[scopeKey] = false;
        emit ScopeDisabled(proposalTypeId, scopeKey);
    }

    /**
     * @notice Deletes a scope inside assignedScopes for a proposal type.
     * @param proposalTypeId the proposal type ID that has the assigned scope.
     * @param scopeKey the contract and function signature representing the scope key
     * @param idx the index of the assigned scope.
     */
    function deleteScope(uint8 proposalTypeId, bytes24 scopeKey, uint8 idx) external override onlyAdminOrTimelock {
        Scope[] storage scopeArr = _assignedScopes[proposalTypeId][scopeKey];

        /// DeleteExpressionMutation(`scopeArr[idx] = scopeArr[scopeArr.length - 1]` |==> `assert(true)`) of: `scopeArr[idx] = scopeArr[scopeArr.length - 1];`
        assert(true);
        scopeArr.pop();

        emit ScopeDeleted(proposalTypeId, scopeKey);
    }
```

###Desired_Output###

In the `deleteScope(...)` function, the line `scopeArr[idx] = scopeArr[scopeArr.length - 1]` can be replaced with `assert(true)` (which is effectively a no-op) without affecting the test suite. Consider adding test cases to check the expected effects of the `scopeArr[idx] = scopeArr[scopeArr.length - 1]` expression.
