**Example 1**

**Input Code Diff**:
```diff
--- original
+++ mutant
@@ -279,7 +279,8 @@
         override(Governor, GovernorVotesQuorumFraction)
         returns (uint256 _quorum)
     {
-        _quorum = hooks.beforeQuorumCalculation(proposalId);
+        /// AssignmentMutation(`hooks.beforeQuorumCalculation(proposalId)` |==> `0`) of: `_quorum = hooks.beforeQuorumCalculation(proposalId);`
+        _quorum = 0;

         if (_quorum == 0) {
             uint256 snapshot = proposalSnapshot(proposalId);
```

**Input Function Context**:
```solidity
    /**
     * @notice Max value of `quorum` and `approvalThreshold` in `ProposalType`
     */
    function quorumDenominator() public pure override returns (uint256) {
        return 10_000;
    }

    /**
     * @notice Returns the quorum for a `proposalId`, in terms of number of votes: `supply * numerator / denominator`.
     * @dev Supply is calculated at the proposal snapshot timepoint.
     * @dev Quorum value is derived from `ProposalTypes` in the `Middleware` and can be changed using the `beforeQuorumCalculation` hook.
     */
    function quorum(uint256 proposalId)
        public
        view
        override(Governor, GovernorVotesQuorumFraction)
        returns (uint256 _quorum)
    {
        /// AssignmentMutation(`hooks.beforeQuorumCalculation(proposalId)` |==> `0`) of: `_quorum = hooks.beforeQuorumCalculation(proposalId);`
        _quorum = 0;

        if (_quorum == 0) {
            uint256 snapshot = proposalSnapshot(proposalId);
            _quorum = (token().getPastTotalSupply(snapshot) * quorumNumerator(snapshot)) / quorumDenominator();
```

###Desired_Output###

In the `quorum(...)` function, the assignment `_quorum = hooks.beforeQuorumCalculation(proposalId);` can be changed to `_quorum = 0;` with no effect on the test suite. Consider adding test cases that depend on the value of `_quorum`.


**Example 2**

**Input Code Diff**:
```diff
--- original
+++ mutant
@@ -210,7 +210,8 @@
             if (result.length != 64) revert InvalidHookResponse();

             // Extract the boolean from the result and convert to uint8 with 2 meaning true and 1 meaning false
-            returnedVoteSucceeded = parseBool(result) ? 2 : 1;
+            /// AssignmentMutation(`parseBool(result) ? 2 : 1` |==> `1`) of: `returnedVoteSucceeded = parseBool(result) ? 2 : 1;`
+            returnedVoteSucceeded = 1;
         }
     }


```

**Input Function Context**:
```solidity
    /// @notice calls afterInitialize hook if permissioned and validates return value
    function afterInitialize(IHooks self) internal noSelfCall(self) {
        if (self.hasPermission(AFTER_INITIALIZE_FLAG)) {
            self.callHook(abi.encodeCall(IHooks.afterInitialize, (msg.sender)));
        }
    }

    /// @notice calls beforeVoteSucceeded hook if permissioned and validates return value
    function beforeVoteSucceeded(IHooks self, uint256 proposalId)
        internal
        view
        noSelfCall(self)
        returns (uint8 returnedVoteSucceeded)
    {
        if (self.hasPermission(BEFORE_VOTE_SUCCEEDED_FLAG)) {
            bytes memory result =
                self.staticCallHook(abi.encodeCall(IHooks.beforeVoteSucceeded, (msg.sender, proposalId)));

            // The length of the result must be 64 bytes to return a bytes4 (padded to 32 bytes) and a boolean (padded to 32 bytes) value
            if (result.length != 64) revert InvalidHookResponse();

            // Extract the boolean from the result and convert to uint8 with 2 meaning true and 1 meaning false
            /// AssignmentMutation(`parseBool(result) ? 2 : 1` |==> `1`) of: `returnedVoteSucceeded = parseBool(result) ? 2 : 1;`
            returnedVoteSucceeded = 1;
        }
    }

    /// @notice calls afterVoteSucceeded hook if permissioned and validates return value
```

###Desired_Output###

In the `beforeVoteSucceeded(...)` function, the assignment `returnedVoteSucceeded = parseBool(result) ? 2 : 1;` can be changed to `returnedVoteSucceeded = 1;` with no effect on the test suite. Consider adding test cases that depend on the value of `returnedVoteSucceeded`.

