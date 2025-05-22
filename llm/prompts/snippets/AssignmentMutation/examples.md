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

        // Proposals can only be cancelled in any state other than Canceled, Expired, or Executed.
        _cancel(targets, values, calldatas, descriptionHash);

        hooks.afterCancel(proposalId, targets, values, calldatas, descriptionHash);
    }

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

