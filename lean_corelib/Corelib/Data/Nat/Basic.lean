import Init.Data.Nat.Basic
import Corelib.Data.Nat.Notation

open Nat

-- Used in Test.lean:6
theorem succ_pred_one (n : ℕ) : n ≠ 0 ∧ n ≠ 1 → succ (pred n) = n := by
  intro hn
  cases n with
  | zero => exact absurd rfl (And.left hn)
  | succ n => rw [Nat.pred_succ]
