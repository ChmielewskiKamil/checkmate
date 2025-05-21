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

In the `propose(...)` function, the call to `hooks.beforePropose(...)` function
can be replaced with `assert(true)` (which is a no-op) without affecting the
test suite. Consider adding test cases to check the expected effects of this
removed expression.
